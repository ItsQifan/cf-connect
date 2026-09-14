#!/usr/bin/env node
/**
 * 打包 cf-connect-dingtalk 插件为比赛平台要求的 .tgz。
 *
 * 产出布局（npm pack 布局，这是 codefree-o 校验函数要求的）：
 *
 *   package/
 *   ├── .codefree-plugin/plugin.json     ← 校验目标
 *   ├── package.json
 *   ├── index.js
 *   ├── skills/dingtalk-bridge/SKILL.md
 *   ├── commands/*.md
 *   ├── bin/cf-connect.exe               ← 存在才打入
 *   ├── config.example.toml
 *   └── README.md
 *
 * 用法：
 *   node scripts/pack.mjs                 # 默认输出到 dist/
 *   node scripts/pack.mjs --no-binary     # 不打入 cf-connect 二进制
 *   node scripts/pack.mjs --out D:\x      # 指定输出目录
 */

import { spawnSync } from "node:child_process";
import {
  closeSync,
  cpSync,
  existsSync,
  mkdirSync,
  openSync,
  readFileSync,
  readdirSync,
  readSync,
  rmSync,
  statSync,
  writeFileSync,
} from "node:fs";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const HERE = dirname(fileURLToPath(import.meta.url));
const PLUGIN_DIR = resolve(HERE, ".."); // plugin/
const REPO_ROOT = resolve(PLUGIN_DIR, ".."); // 仓库根

const argv = process.argv.slice(2);
const withBinary = !argv.includes("--no-binary");
const outFlag = argv.indexOf("--out");
const OUT_DIR = outFlag >= 0 && argv[outFlag + 1] ? resolve(argv[outFlag + 1]) : join(PLUGIN_DIR, "dist");

const SKIP_NAMES = new Set(["node_modules", "dist", ".git", ".DS_Store", "Thumbs.db", "build", "scripts"]);

function readJson(path) {
  return JSON.parse(readFileSync(path, "utf8"));
}

function copyTree(from, to) {
  mkdirSync(to, { recursive: true });
  for (const entry of readdirSync(from, { withFileTypes: true })) {
    if (SKIP_NAMES.has(entry.name)) continue;
    const src = join(from, entry.name);
    const dst = join(to, entry.name);
    if (entry.isDirectory()) {
      copyTree(src, dst);
    } else if (entry.isFile()) {
      mkdirSync(dirname(dst), { recursive: true });
      cpSync(src, dst);
    }
  }
}

function walk(dir, base = dir) {
  const out = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) out.push(...walk(full, base));
    else out.push({ path: relative(base, full).split("\\").join("/"), size: statSync(full).size });
  }
  return out.sort((a, b) => a.path.localeCompare(b.path));
}

function human(bytes) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(2)} MB`;
}

// 从 PE 头读出 .exe 的真实目标架构。返回 null 表示不是 PE（或读不出来），
// 调用方回退到打包机架构。
function detectBinaryTarget(file) {
  try {
    const head = Buffer.alloc(64);
    const fd = openSync(file, "r");
    let read;
    try {
      read = readSync(fd, head, 0, 64, 0);
    } finally {
      closeSync(fd);
    }
    if (read < 64 || head[0] !== 0x4d || head[1] !== 0x5a) return null; // "MZ"
    const peOffset = head.readUInt32LE(0x3c);
    if (peOffset <= 0 || peOffset > 1 << 20) return null;
    const sig = Buffer.alloc(6);
    const fd2 = openSync(file, "r");
    let read2;
    try {
      read2 = readSync(fd2, sig, 0, 6, peOffset);
    } finally {
      closeSync(fd2);
    }
    if (read2 < 6) return null;
    if (sig.toString("latin1", 0, 4) !== "PE\0\0") return null;
    const machine = sig.readUInt16LE(4);
    const arch = machine === 0x8664 ? "amd64" : machine === 0xaa64 ? "arm64" : null;
    return arch ? { os: "windows", arch } : null;
  } catch {
    return null;
  }
}

// ---------------------------------------------------------------------------

const manifestPath = join(PLUGIN_DIR, ".codefree-plugin", "plugin.json");
const pkgPath = join(PLUGIN_DIR, "package.json");
if (!existsSync(manifestPath)) throw new Error(`missing ${manifestPath}`);
if (!existsSync(pkgPath)) throw new Error(`missing ${pkgPath}`);

const manifest = readJson(manifestPath);
const pkg = readJson(pkgPath);
const version = String(manifest.version ?? "");

if (!version) throw new Error(".codefree-plugin/plugin.json has no version");
if (String(pkg.version) !== version) {
  throw new Error(`version mismatch: plugin.json=${version} package.json=${pkg.version}`);
}
if (!Array.isArray(manifest.agentCompatibility) || !manifest.agentCompatibility.includes("codefree")) {
  throw new Error('agentCompatibility must be an array containing "codefree"');
}
if (manifest.agentCompatibility.includes("opencode-npm")) {
  throw new Error('agentCompatibility must NOT contain "opencode-npm"');
}

const stageRoot = join(PLUGIN_DIR, "build", "stage");
const packageDir = join(stageRoot, "package");
rmSync(stageRoot, { recursive: true, force: true });
mkdirSync(packageDir, { recursive: true });

// 1. 插件本体
for (const name of [".codefree-plugin", "skills", "commands", "index.js", "package.json", "README.md"]) {
  const src = join(PLUGIN_DIR, name);
  if (!existsSync(src)) {
    if (name === "README.md") continue;
    throw new Error(`missing required plugin entry: ${name}`);
  }
  const dst = join(packageDir, name);
  if (statSync(src).isDirectory()) copyTree(src, dst);
  else cpSync(src, dst);
}

// 2. 根级 plugin.json 的备用副本（比赛文案写的是 /.codefree-plugin/plugin.json）
const rootManifestDir = join(stageRoot, ".codefree-plugin");
mkdirSync(rootManifestDir, { recursive: true });
cpSync(manifestPath, join(rootManifestDir, "plugin.json"));

// 3. 用户配置样例 + 安装脚本。
//    config.example.toml 的唯一真源在仓库根（`go:embed` 也是读它，有测试盯着），
//    以免同一份样例出现两个会漂移的副本；install.ps1 / QUICKSTART.md 则用插件自己的。
const rootExtras = [
  { name: "config.example.toml", from: REPO_ROOT },
  { name: "install.ps1", from: PLUGIN_DIR },
  { name: "QUICKSTART.md", from: PLUGIN_DIR },
];
for (const { name, from } of rootExtras) {
  const src = join(from, name);
  if (!existsSync(src)) {
    console.log(`  ! 跳过 ${name}（没有这个文件：${src}）`);
    continue;
  }
  cpSync(src, join(stageRoot, name));
  cpSync(src, join(packageDir, name));
  console.log(`  · 收入 ${name}`);
}
if (!existsSync(join(stageRoot, "config.example.toml"))) {
  throw new Error(`config.example.toml not staged from ${join(REPO_ROOT, "config.example.toml")}`);
}

// 4. cf-connect 二进制（可选）
//    同时判断它到底是为哪个平台/架构编的 —— 包名要带平台标识，就得按二进制的
//    真实架构命名，不能按打包机的架构（交叉编译时会错）。
const binDir = join(packageDir, "bin");
let detected = null; // { os, arch }
if (withBinary) {
  const candidates =
    process.platform === "win32"
      ? ["bin/cf-connect.exe", "bin/cf-connect"]
      : ["bin/cf-connect", "bin/cf-connect.exe"];
  let copied = [];
  for (const rel of candidates) {
    const src = join(PLUGIN_DIR, rel);
    if (!existsSync(src)) continue;
    mkdirSync(binDir, { recursive: true });
    const dest = join(binDir, rel.slice(4));
    cpSync(src, dest);
    copied.push(rel);
    if (!detected) detected = detectBinaryTarget(dest);
  }
  if (copied.length === 0) {
    console.log("  ! bin/ 里没有 cf-connect 可执行文件 —— 打包会缺少二进制。");
    console.log("    放一个到 plugin/bin/ 后重跑，或参考 README「安装前置」一节。");
  }
} else {
  console.log("  · --no-binary：跳过二进制");
}

// 平台标识：优先按二进制真实架构，其次按打包机
const targetOs = detected?.os ?? (process.platform === "win32" ? "windows" : process.platform);
const targetArch = detected?.arch ?? (process.arch === "x64" ? "amd64" : process.arch);
const platformTag = `${targetOs}-${targetArch}`;
if (detected) {
  console.log(`  · 二进制目标平台：${platformTag}（由 PE 头识别）`);
} else if (withBinary) {
  console.log(`  · 二进制目标平台：${platformTag}（按打包机推断，未能读取 PE 头）`);
}

// 5. 打成 tgz（npm pack 布局：tar 根下就是 package/），包名带平台标识
mkdirSync(OUT_DIR, { recursive: true });
const tgzName = `${manifest.name}-plugin-${version}-${platformTag}.tgz`;
const tgzPath = join(OUT_DIR, tgzName);
rmSync(tgzPath, { force: true });

// 写一份构建信息，便于事后追溯
// stdio: "inherit" 而不是默认的管道：tar/git 的输出直接进终端，也避免在受限
// 环境里因为管道句柄被拒而 EPERM。
const sha = spawnSync("git", ["rev-parse", "--short", "HEAD"], {
  cwd: REPO_ROOT,
  encoding: "utf8",
  stdio: ["ignore", "pipe", "ignore"],
});
writeFileSync(
  join(packageDir, ".codefree-plugin", "build-info.json"),
  JSON.stringify(
    {
      name: manifest.name,
      version,
      builtAt: new Date().toISOString(),
      node: process.version,
      platform: `${process.platform}-${process.arch}`,
      gitCommit: (sha.stdout || "").trim() || "unknown",
      withBinary,
    },
    null,
    2,
  ) + "\n",
);

const tar = spawnSync(
  "tar",
  ["-czf", tgzPath, "-C", stageRoot, "package", ...(existsSync(rootManifestDir) ? [".codefree-plugin"] : [])],
  {
    stdio: ["ignore", "inherit", "inherit"],
  },
);
if (tar.error) throw tar.error;
if (tar.status !== 0) throw new Error(`tar failed with exit code ${tar.status}`);

const files = walk(packageDir);
const total = files.reduce((sum, f) => sum + f.size, 0);

console.log(`\n✅ ${tgzName}`);
console.log(`   输出：${tgzPath}`);
console.log(`   大小：${human(statSync(tgzPath).size)}（解包后 ${files.length} 个文件 / ${human(total)}）`);
console.log(`   版本：${version}  （plugin.json = package.json = 文件名）\n`);
console.log("   package/ 内容：");
for (const f of files) console.log(`     ${f.path}  (${human(f.size)})`);
console.log("\n下一步：node scripts/verify-pack.mjs \"" + tgzPath + "\"");
