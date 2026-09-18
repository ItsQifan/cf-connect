/**
 * CF-Connect 钉钉桥接插件 (cf-connect-dingtalk)
 * ---------------------------------------------
 * 这个 JS 入口做三件小事，桥接主逻辑（钉钉 Stream 长连接、会话路由、卡片流式）
 * 全部由 Go 二进制 cf-connect 负责：
 *
 *   1. config  hook : 启动时确保 cf-connect 网关在跑（不在就 spawn 一个）
 *   2. event   hook : 一轮对话结束时，把结果回推到发起人的钉钉会话（幂等，失败只告警）
 *   3. tool         : 注册 notify_dingtalk 工具，让 agent 能主动往钉钉推消息
 *
 * 说明：codefree-o 的插件加载器只从 srdplugins 注入 skills / commands / agents /
 * mcpServers；本文件是给宿主 plugin 数组或未来加载器扩展用的入口，逻辑刻意保持
 * 零外部依赖、全量 try/catch，任何失败都不影响 codefree-o 本身。
 *
 * 环境变量（全部可选，见 README 的「配置项」）：
 *   CF_CONNECT_BIN        cf-connect 可执行文件绝对路径
 *   CF_CONNECT_CONFIG     config.toml 路径
 *   CF_CONNECT_DATA_DIR   数据目录（默认 ~/.cf-connect）
 *   CF_CONNECT_NOTIFY     event 自动回推开关，默认 "1"；设 "0" 关闭
 *   CF_CONNECT_DEFAULT_TO 没有会话上下文时的默认钉钉推送目标（userid 或会话 key）
 */

import { spawn, spawnSync } from "node:child_process";
import { existsSync, mkdirSync, appendFileSync } from "node:fs";
import { homedir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const PLUGIN_ID = "cf-connect-dingtalk";
const PLUGIN_VERSION = "0.1.1";
const IS_WINDOWS = process.platform === "win32";

const HERE = dirname(fileURLToPath(import.meta.url));

// ---------------------------------------------------------------------------
// 小工具
// ---------------------------------------------------------------------------

const logFile = join(homedir(), ".cf-connect", "plugin.log");

function log(level, message, extra) {
  const line =
    `${new Date().toISOString()} [${level}] ${PLUGIN_ID} ${message}` +
    (extra === undefined ? "" : ` ${safeJson(extra)}`);
  try {
    mkdirSync(dirname(logFile), { recursive: true });
    appendFileSync(logFile, line + "\n");
  } catch {
    /* 日志写不进去也不能影响流程 */
  }
  if (level === "error") console.warn(line);
}

function safeJson(value) {
  try {
    const text = JSON.stringify(value);
    return text === undefined ? String(value) : text.length > 500 ? text.slice(0, 500) + "…" : text;
  } catch {
    return String(value);
  }
}

function envFlag(name, fallback) {
  const raw = process.env[name];
  if (raw === undefined || raw === "") return fallback;
  return !["0", "false", "no", "off"].includes(String(raw).trim().toLowerCase());
}

function dataDir() {
  return process.env.CF_CONNECT_DATA_DIR?.trim() || join(homedir(), ".cf-connect");
}

function binaryCandidates() {
  const fromEnv = process.env.CF_CONNECT_BIN?.trim();
  const names = IS_WINDOWS ? ["cf-connect.exe", "cf-connect"] : ["cf-connect"];
  const candidates = [];
  if (fromEnv) candidates.push(fromEnv);
  for (const name of names) {
    candidates.push(join(HERE, "bin", name)); // 插件包内置
    candidates.push(join(dataDir(), name));
    candidates.push(join(homedir(), ".cf-connect", "bin", name));
  }
  if (IS_WINDOWS) {
    candidates.push("D:\\software\\cf-connect-v0.1.1-windows-amd64\\cf-connect.exe");
    candidates.push("D:\\software\\cf-connect-v0.1.0-windows-amd64\\cf-connect.exe");
    candidates.push("D:\\software\\cf-connect-v1.0.0-windows-amd64\\cf-connect.exe");
  }
  return candidates;
}

let cachedBinary;

function resolveBinary() {
  if (cachedBinary !== undefined) return cachedBinary;
  for (const candidate of binaryCandidates()) {
    try {
      if (candidate && existsSync(candidate)) {
        cachedBinary = candidate;
        return cachedBinary;
      }
    } catch {
      /* 忽略无法访问的候选路径 */
    }
  }
  // 最后退回 PATH 查找
  const probe = spawnSync(IS_WINDOWS ? "where" : "which", ["cf-connect"], {
    stdio: ["ignore", "pipe", "ignore"],
    encoding: "utf8",
  });
  if (probe.status === 0 && probe.stdout) {
    const first = probe.stdout.split(/\r?\n/).map((s) => s.trim()).filter(Boolean)[0];
    if (first) {
      cachedBinary = first;
      return cachedBinary;
    }
  }
  cachedBinary = null;
  return cachedBinary;
}

function socketPath() {
  return join(dataDir(), "run", "api.sock");
}

function bridgeIsRunning() {
  return existsSync(socketPath());
}

let spawnAttempted = false;

function ensureBridgeRunning() {
  if (bridgeIsRunning()) return { ok: true, started: false };
  const bin = resolveBinary();
  if (!bin) {
    log("warn", "skip autostart: cf-connect binary not found");
    return { ok: false, error: "binary not found" };
  }
  if (spawnAttempted) return { ok: false, error: "autostart already attempted" };
  spawnAttempted = true;

  const args = [];
  const configPath = process.env.CF_CONNECT_CONFIG?.trim();
  if (configPath) args.push("--config", configPath);
  args.push("--data-dir", dataDir());
  args.push("daemon", "start");

  try {
    const child = spawn(bin, args, {
      detached: true,
      stdio: "ignore",
      windowsHide: true,
      env: { ...process.env, CF_CONNECT_DATA_DIR: dataDir() },
    });
    child.unref();
    log("info", "cf-connect daemon start requested", { bin, args });
    return { ok: true, started: true, bin };
  } catch (error) {
    log("error", "failed to start cf-connect daemon", String(error));
    return { ok: false, error: String(error) };
  }
}

// ---------------------------------------------------------------------------
// 结果提取 + 回推
// ---------------------------------------------------------------------------

const seenTurns = new Map(); // sessionID -> 最近一次已推送的 messageID
const PUSHED_LIMIT = 500;
const pushedOnce = new Set(); // sessionID:messageID，防重复推送

function markPushed(key) {
  if (pushedOnce.size > PUSHED_LIMIT) pushedOnce.clear();
  pushedOnce.add(key);
}

function textOfParts(parts) {
  if (!Array.isArray(parts)) return "";
  const chunks = [];
  for (const part of parts) {
    if (!part || typeof part !== "object") continue;
    if (part.type === "text" && typeof part.text === "string") chunks.push(part.text);
  }
  return chunks.join("\n").trim();
}

async function summarizeLastAssistantText(client, sessionID, limit = 1500) {
  if (!client?.session?.messages) return "";
  const response = await client.session.messages({ path: { id: sessionID } });
  const list = response?.data ?? response;
  if (!Array.isArray(list)) return "";
  for (let i = list.length - 1; i >= 0; i--) {
    const entry = list[i];
    if (!entry || typeof entry !== "object") continue;
    const info = entry.info ?? entry;
    if (info?.role !== "assistant") continue;
    const text = textOfParts(entry.parts ?? []);
    if (text) return text.length > limit ? text.slice(0, limit) + "\n…(已截断)" : text;
  }
  return "";
}

function pushToDingtalk({ text, sessionKey, project, title }) {
  const message = text?.trim();
  if (!message) return { ok: false, error: "empty message" };
  const args = ["send"];
  const target = sessionKey || process.env.CF_CONNECT_SESSION_KEY?.trim();
  if (target) args.push("--session", target);
  const proj = project || process.env.CF_CONNECT_PROJECT?.trim();
  if (proj) args.push("--project", proj);
  args.push("--stdin");
  const bin = resolveBinary();
  if (!bin) return { ok: false, error: "cf-connect binary not found" };
  const result = spawnSync(bin, args, {
    input: message,
    encoding: "utf8",
    timeout: 20000,
    windowsHide: true,
    env: { ...process.env, CF_CONNECT_DATA_DIR: dataDir() },
  });
  if (result.error) return { ok: false, error: String(result.error.message || result.error) };
  if (result.status !== 0) {
    return { ok: false, error: (result.stderr || result.stdout || `exit ${result.status}`).trim() };
  }
  log("info", "pushed turn result to DingTalk", { title, sessionKey: Boolean(target), chars: message.length });
  return { ok: true };
}

// ---------------------------------------------------------------------------
// 插件主体
// ---------------------------------------------------------------------------

export const CFConnectDingTalk = async (ctx) => {
  log("info", `loaded v${PLUGIN_VERSION}`, { directory: ctx?.directory, worktree: ctx?.worktree });

  return {
    config: async () => {
      const status = ensureBridgeRunning();
      if (!status.ok) log("warn", "cf-connect not running", status.error);
    },

    event: async ({ event }) => {
      try {
        if (!envFlag("CF_CONNECT_NOTIFY", true)) return;
        if (event?.type !== "session.idle") return;
        const sessionID = event.properties?.sessionID;
        if (!sessionID) return;
        if (!bridgeIsRunning()) return; // 网关没跑，说明结果本来就在本机，不用回推

        const text = await summarizeLastAssistantText(ctx?.client, sessionID);
        if (!text) return;

        const key = `${sessionID}:${text.length}:${text.slice(0, 64)}`;
        if (pushedOnce.has(key)) return;
        markPushed(key);

        const result = pushToDingtalk({
          text,
          title: `codefree-o session ${sessionID}`,
        });
        if (!result.ok) log("warn", "push to DingTalk failed", result.error);
      } catch (error) {
        // 永远不会把异常抛回宿主
        log("warn", "event hook failed", String(error));
      }
    },

    tool: {
      notify_dingtalk: {
        description:
          "把一条消息推送到钉钉（通过本机 cf-connect 网关）。用于长时间任务的关键节点汇报、需要用户决策时的提醒。",
        args: {
          message: {
            type: "string",
            description: "要推送到钉钉的 Markdown 文本",
          },
          session: {
            type: "string",
            description: "可选的 cf-connect 会话 key；不填则推给当前活跃会话",
          },
          project: {
            type: "string",
            description: "可选的 cf-connect 项目名；不填则用唯一项目",
          },
        },
        async execute(args) {
          const text = typeof args?.message === "string" ? args.message : "";
          if (!text.trim()) return "未推送：message 为空";
          const result = pushToDingtalk({
            text,
            sessionKey: args?.session,
            project: args?.project,
          });
          return result.ok ? "已推送到钉钉" : `推送失败：${result.error}`;
        },
      },
    },
  };
};

export default CFConnectDingTalk;
export const id = PLUGIN_ID;
