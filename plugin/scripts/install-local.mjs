#!/usr/bin/env node
/**
 * 把打包好的插件装到本机 codefree-o 的 srdplugins 目录，用于真机验证。
 *
 *   ~/.codefree-o/.config/srdplugins/<插件名>@<版本>/package/...
 *
 * 这正是 codefree-o 的插件加载器扫描的路径（见 README「真机验证」）。
 *
 * 用法：
 *   node scripts/install-local.mjs                    # 用 dist/ 里最新的 tgz
 *   node scripts/install-local.mjs path/to/x.tgz
 *   node scripts/install-local.mjs --remove           # 卸载
 */

import { cpSync, existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { basename, dirname, join, resolve } from "node:path";
import { homedir, tmpdir } from "node:os";
import { fileURLToPath } from "node:url";

const HERE = dirname(fileURLToPath(import.meta.url));
const ROOT = resolve(HERE, "..");
const PLUGIN_ROOT = join(homedir(), ".codefree-o", ".config", "srdplugins");

const argv = process.argv.slice(2);
const remove = argv.includes("--remove");

function findTgz() {
  const arg = argv.find((a) => !a.startsWith("--"));
  if (arg) return resolve(arg);
  const dist = join(ROOT, "dist");
  if (!existsSync(dist)) throw new Error("no dist/; run: node scripts/pack.mjs");
  const found = readdirSync(dist).filter((f) => f.endsWith(".tgz")).sort().reverse();
  if (!found.length) throw new Error("no .tgz in dist/; run: node scripts/pack.mjs");
  return join(dist, found[0]);
}

if (remove) {
  if (!existsSync(PLUGIN_ROOT)) {
    console.log(`nothing to remove (${PLUGIN_ROOT} does not exist)`);
    process.exit(0);
  }
  let removed = 0;
  for (const entry of readdirSync(PLUGIN_ROOT, { withFileTypes: true })) {
    if (!entry.isDirectory() || !entry.name.startsWith("cf-connect-dingtalk@")) continue;
    rmSync(join(PLUGIN_ROOT, entry.name), { recursive: true, force: true });
    console.log(`removed ${entry.name}`);
    removed++;
  }
  console.log(removed ? "卸载完成，重启 codefree-o 生效。" : "没有找到已安装的 cf-connect-dingtalk。");
  process.exit(0);
}

const tgz = findTgz();
if (!existsSync(tgz)) throw new Error(`tgz not found: ${tgz}`);

const work = mkdtempSync(join(tmpdir(), "cf-connect-install-"));
try {
  // 不要加 -q：Windows 自带的 bsdtar 把 -q 当成"只解出名为 q 的成员"，
  // 会静默解出 0 个文件（退出码仍是 0）。
  const tar = spawnSync("tar", ["-xzf", tgz, "-C", work], {
    stdio: ["ignore", "inherit", "inherit"],
  });
  if (tar.error) throw tar.error;
  if (tar.status !== 0) throw new Error(`tar failed with exit code ${tar.status}`);

  const manifestPath = join(work, "package", ".codefree-plugin", "plugin.json");
  if (!existsSync(manifestPath)) throw new Error("tgz has no package/.codefree-plugin/plugin.json");
  const manifest = JSON.parse(readFileSync(manifestPath, "utf8"));
  const name = String(manifest.name || "").trim();
  const version = String(manifest.version || "").trim();
  if (!name || !version) throw new Error("manifest is missing name or version");

  const target = join(PLUGIN_ROOT, `${name}@${version}`);
  rmSync(target, { recursive: true, force: true });
  mkdirSync(target, { recursive: true });
  cpSync(join(work, "package"), join(target, "package"), { recursive: true });
  console.log(`installed: ${target}`);
  console.log(`verify   : codefree-o debug skill    # dingtalk-bridge 应该出现在列表里`);
  console.log(`remove   : node scripts/install-local.mjs --remove`);
} finally {
  rmSync(work, { recursive: true, force: true });
}
