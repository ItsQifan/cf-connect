#!/usr/bin/env node
/**
 * 校验打包出来的 .tgz 是否符合 codefree-o 的插件规范（比赛平台校验口径）。
 *
 * 断言（来自 codefree-o 1.7.0 二进制中的校验函数 ul()）：
 *   ① 解包后根下存在 package/                （npm pack 布局）
 *   ② package/.codefree-plugin/plugin.json 存在且是 JSON 对象
 *   ③ plugin.json.agentCompatibility 是数组，含 "codefree"、不含 "opencode-npm"
 * 附加断言（本仓库自己加的保险）：
 *   ④ 根级 .codefree-plugin/plugin.json 也存在（比赛文案写的是 /.codefree-plugin/plugin.json）
 *   ⑤ 三处版本号一致：plugin.json = package.json = tgz 文件名
 *   ⑥ commands / skills 目录非空（否则装上去什么也看不到）
 *   ⑦ bin/cf-connect(.exe) 是否存在（缺失只警告，不失败）
 *
 * 用法：
 *   node scripts/verify-pack.mjs                 # 自动找 dist/*.tgz
 *   node scripts/verify-pack.mjs path/to/x.tgz
 */

import { existsSync, mkdtempSync, readFileSync, readdirSync, rmSync, statSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { basename, dirname, join, relative, resolve } from "node:path";
import { tmpdir } from "node:os";
import { fileURLToPath } from "node:url";

const HERE = dirname(fileURLToPath(import.meta.url));
const ROOT = resolve(HERE, "..");

let failures = 0;
let warnings = 0;

function ok(msg) {
  console.log(`  ✅ ${msg}`);
}
function bad(msg) {
  failures++;
  console.log(`  ❌ ${msg}`);
}
function warn(msg) {
  warnings++;
  console.log(`  ⚠️  ${msg}`);
}
function section(title) {
  console.log(`\n${title}`);
}

function pickTgz() {
  const arg = process.argv[2];
  if (arg) return resolve(arg);
  const dist = join(ROOT, "dist");
  if (!existsSync(dist)) throw new Error(`no dist/ directory and no path given`);
  const found = readdirSync(dist).filter((f) => f.endsWith(".tgz"));
  if (found.length === 0) throw new Error(`no .tgz in ${dist}; run: node scripts/pack.mjs`);
  if (found.length > 1) {
    console.log(`  (dist/ 里有多个 tgz，选最新的：${found.sort().reverse()[0]})`);
    return join(dist, found.sort().reverse()[0]);
  }
  return join(dist, found[0]);
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

// ---------------------------------------------------------------------------

const tgz = pickTgz();
if (!existsSync(tgz)) {
  console.error(`tgz not found: ${tgz}`);
  process.exit(1);
}

console.log(`\n校验插件包：${tgz}`);
console.log(`包大小：${human(statSync(tgz).size)}`);

const work = mkdtempSync(join(tmpdir(), "cf-connect-verify-"));
try {
  // stdio: "inherit" 而不是管道，既能让 tar 的错误直接可见，也避免在受限环境里
  // 因管道句柄被拒而 EPERM。注意：不要加 -q —— Windows 自带的 bsdtar 把 -q 当成
  // "只解出名为 q 的成员"，会静默解出 0 个文件（退出码仍是 0）。
  const tar = spawnSync("tar", ["-xzf", tgz, "-C", work], {
    stdio: ["ignore", "inherit", "inherit"],
  });
  if (tar.error) {
    console.error(`解包失败：${tar.error.message}`);
    process.exit(1);
  }
  if (tar.status !== 0) {
    console.error(`解包失败：tar 退出码 ${tar.status}`);
    process.exit(1);
  }

  section("解包后的完整文件清单");
  const all = walk(work);
  for (const f of all) console.log(`     ${f.path}  (${human(f.size)})`);
  console.log(`     —— 共 ${all.length} 个文件`);

  section("① npm pack 布局：package/ 目录");
  const packageDir = join(work, "package");
  existsSync(packageDir) && statSync(packageDir).isDirectory()
    ? ok("package/ 存在")
    : bad("package/ 不存在 —— codefree-o 会报 `package directory not found in extracted plugin`");

  let manifest = null;
  const manifestPath = join(packageDir, ".codefree-plugin", "plugin.json");

  section("② package/.codefree-plugin/plugin.json");
  if (!existsSync(manifestPath)) {
    bad("缺失 —— 会报 `.codefree-plugin/plugin.json not found`");
  } else {
    try {
      const parsed = JSON.parse(readFileSync(manifestPath, "utf8"));
      if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
        bad("不是 JSON 对象（不能是数组）—— 会报 `is not a JSON object`");
      } else {
        manifest = parsed;
        ok("存在，且是 JSON 对象");
      }
    } catch (error) {
      bad(`解析失败：${error.message} —— 会报 failed to parse .codefree-plugin/plugin.json`);
    }
  }

  section("③ agentCompatibility 声明");
  if (!manifest) {
    bad("跳过：manifest 不可用");
  } else {
    const compat = manifest.agentCompatibility;
    if (!Array.isArray(compat)) {
      bad("agentCompatibility 不是数组 —— 会报 `does not declare codefree compatibility`");
    } else {
      compat.includes("codefree")
        ? ok(`包含 "codefree"（${JSON.stringify(compat)}）`)
        : bad('不包含 "codefree" —— 会报 `does not declare codefree compatibility`');
      compat.includes("opencode-npm")
        ? bad('包含 "opencode-npm" —— 会被判定为不兼容 codefree')
        : ok('不含 "opencode-npm"');
    }
  }

  section("④ 根级 .codefree-plugin/plugin.json（保险副本）");
  existsSync(join(work, ".codefree-plugin", "plugin.json"))
    ? ok("存在")
    : bad("缺失 —— 建议补一份，兼容按 /.codefree-plugin/plugin.json 校验的口径");

  section("⑤ 版本号三处一致");
  if (manifest) {
    const version = String(manifest.version ?? "");
    const pkgPath = join(packageDir, "package.json");
    let pkgVersion = null;
    if (existsSync(pkgPath)) {
      try {
        pkgVersion = String(JSON.parse(readFileSync(pkgPath, "utf8")).version ?? "");
      } catch {
        bad("package.json 解析失败");
      }
    } else {
      bad("package/package.json 缺失");
    }
    // 文件形如 cf-connect-dingtalk-plugin-<版本>[-<平台>].tgz，平台后缀可有可无
    const nameMatch = basename(tgz).match(
      /^cf-connect-dingtalk-plugin-(\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.]+)?)(?:-([a-z0-9]+-[a-z0-9_]+))?\.tgz$/,
    );
    const fileVersion = nameMatch ? nameMatch[1] : null;
    const filePlatform = nameMatch?.[2] ?? null;

    version ? ok(`plugin.json version = ${version}`) : bad("plugin.json 没有 version");
    pkgVersion === version
      ? ok(`package.json version = ${pkgVersion}`)
      : bad(`package.json version = ${pkgVersion}，与 plugin.json 的 ${version} 不一致`);
    fileVersion === version
      ? ok(`文件名版本 = ${fileVersion}`)
      : bad(`文件名版本 = ${fileVersion}，与 plugin.json 的 ${version} 不一致`);
    filePlatform
      ? ok(`文件名带平台标识 = ${filePlatform}`)
      : warn("文件名没有平台标识（形如 …-windows-amd64.tgz）——多平台分发时容易拿错包");
  }

  section("⑥ 注入内容：skills / commands");
  const skillsDir = join(packageDir, "skills");
  if (!existsSync(skillsDir)) {
    bad("package/skills 缺失 —— 顶层目录会被自动注入为 skill，没有它就只是个空壳插件");
  } else {
    const skillFiles = walk(skillsDir).filter((f) => f.path.endsWith("SKILL.md"));
    skillFiles.length > 0
      ? ok(`skills：${skillFiles.map((f) => f.path).join(", ")}`)
      : bad("skills/ 下没有 SKILL.md");
  }
  const cmdDir = join(packageDir, "commands");
  if (!existsSync(cmdDir)) {
    bad("package/commands 缺失 —— manifest.commands 指向的目录不存在");
  } else {
    const md = walk(cmdDir).filter((f) => f.path.endsWith(".md"));
    md.length > 0
      ? ok(`commands：${md.map((f) => "/" + basename(f.path, ".md")).join(", ")}`)
      : bad("commands/ 下没有 .md 文件");
  }

  section("⑦ 内置 cf-connect 二进制（可选）");
  const binFound = [`bin/cf-connect.exe`, `bin/cf-connect`].filter((p) => existsSync(join(packageDir, p)));
  if (binFound.length > 0) {
    for (const b of binFound) ok(`${b}（${human(statSync(join(packageDir, b)).size)}）`);
  } else {
    warn("包内没有 cf-connect 二进制 —— 用户需要另行准备可执行文件（见 README「安装前置」）");
  }

  section("⑧ install.ps1 的编码：Windows PowerShell 5.1 必须能解析");
  // 用户机器上默认的 powershell.exe 就是 5.1，它不探测 UTF-8：
  // 无 BOM 的 .ps1 会按系统 ANSI 代码页（中文 Windows = GBK）解码，
  // 中文注释字节错位后吞掉引号 → `字符串缺少终止符: '`，脚本根本跑不起来。
  //
  // 会分发出去的 .ps1 有三个来源，都要盯：
  //   ① 插件包里的 package/install.ps1（本 tgz，硬要求）
  //   ② 插件源码 plugin/install.ps1（下一次打包的输入）
  //   ③ 仓库根 install.ps1（standalone 发行 zip 通过 Makefile 的 DIST_FILES 带上它）
  const ps1Targets = [
    { label: "package/install.ps1", file: join(packageDir, "install.ps1"), required: true },
    { label: "plugin/install.ps1", file: join(ROOT, "install.ps1"), required: false },
    { label: "<repo>/install.ps1", file: join(ROOT, "..", "install.ps1"), required: false },
  ];
  for (const { label, file, required } of ps1Targets) {
    if (!existsSync(file)) {
      if (required) bad(`${label} 缺失`);
      else warn(`${label} 不存在（跳过）`);
      continue;
    }
    const head = readFileSync(file).subarray(0, 3);
    head.length === 3 && head[0] === 0xef && head[1] === 0xbb && head[2] === 0xbf
      ? ok(`${label} 带 UTF-8 BOM`)
      : bad(
          `${label} 缺 UTF-8 BOM —— PowerShell 5.1 会按 GBK 解码，中文注释吞掉引号后报「字符串缺少终止符」`,
        );
  }
  const packagedPs1 = join(packageDir, "install.ps1");
  if (process.platform !== "win32") {
    warn("非 Windows：跳过 PowerShell 5.1 语法校验");
  } else if (existsSync(packagedPs1)) {
    const quoted = packagedPs1.replace(/'/g, "''");
    const probe = spawnSync(
      "powershell",
      [
        "-NoProfile",
        "-Command",
        `$e=$null; [void][System.Management.Automation.Language.Parser]::ParseFile('${quoted}',[ref]$null,[ref]$e); if ($e) { $e | ForEach-Object { $_.Extent.StartLineNumber.ToString() + ': ' + $_.Message }; exit 1 }`,
      ],
      { encoding: "utf8" },
    );
    if (probe.error) {
      warn(`跳过 PowerShell 5.1 语法校验：${probe.error.message}`);
    } else if (probe.status === 0) {
      ok("Windows PowerShell 5.1 解析通过（用户机器上默认就是它）");
    } else {
      bad(`Windows PowerShell 5.1 解析失败：${(probe.stdout || probe.stderr || "").trim()}`);
    }
  }
} finally {
  rmSync(work, { recursive: true, force: true });
}

console.log("");
if (failures === 0) {
  console.log(`\n🎉 校验通过（${warnings} 条警告）—— 这个 tgz 可以上传到比赛平台。\n`);
  process.exit(0);
} else {
  console.log(`\n💥 校验失败：${failures} 项不通过，${warnings} 条警告。\n`);
  process.exit(1);
}
