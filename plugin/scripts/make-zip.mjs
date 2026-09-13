#!/usr/bin/env node
/**
 * 在 .tgz 之外再产出一份 .zip（分发 / 评审用）。
 *
 * 比赛平台只收 .tgz（见 README §2.2），但 zip 在 Windows 上双击就能看，
 * 方便同事和评审直接翻阅内容、也可以手动重新打包。两种格式内容完全一致，
 * zip 里的 .tgz 就是同一份字节。
 *
 * 只用 Node 标准库写 zip（zlib.deflateRawSync + 手写 central directory），
 * 不依赖 tar/Compress-Archive，跨平台行为一致（Windows 的 Compress-Archive
 * 会对中文路径和编码挑刺，能不用就不用）。
 *
 * 用法：
 *   node scripts/make-zip.mjs              # 先跑 pack.mjs，再用最新 tgz 打 zip
 *   node scripts/make-zip.mjs --no-repack  # 直接用 dist/ 里已有的 tgz
 */

import { copyFileSync, existsSync, mkdirSync, readdirSync, readFileSync, rmSync, statSync, writeFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { deflateRawSync } from "node:zlib";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const HERE = dirname(fileURLToPath(import.meta.url));
const PLUGIN_DIR = resolve(HERE, "..");
const REPO_ROOT = resolve(PLUGIN_DIR, "..");
const DIST = join(PLUGIN_DIR, "dist");
const STAGE = join(PLUGIN_DIR, "build", "zip-stage");

const repack = !process.argv.includes("--no-repack");

function run(cmd, args) {
  const res = spawnSync(cmd, args, { stdio: ["ignore", "inherit", "inherit"] });
  if (res.error) throw res.error;
  if (res.status !== 0) throw new Error(`${cmd} ${args.join(" ")} -> exit ${res.status}`);
}

// ---------------------------------------------------------------------------
// 极简 zip 写入器
// ---------------------------------------------------------------------------

const CRC_TABLE = (() => {
  const table = new Int32Array(256);
  for (let n = 0; n < 256; n++) {
    let c = n;
    for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    table[n] = c;
  }
  return table;
})();

function crc32(buf) {
  let c = 0 ^ -1;
  for (let i = 0; i < buf.length; i++) c = (c >>> 8) ^ CRC_TABLE[(c ^ buf[i]) & 0xff];
  return (c ^ -1) >>> 0;
}

function walk(dir, base = dir) {
  const files = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) files.push(...walk(full, base));
    else if (entry.isFile()) files.push({ full, name: relative(base, full).split("\\").join("/") });
  }
  return files.sort((a, b) => a.name.localeCompare(b.name));
}

function zipDirectory(dir, outPath) {
  const files = walk(dir);
  const locals = [];
  const central = [];
  let offset = 0;

  for (const file of files) {
    const data = readFileSync(file.full);
    const crc = crc32(data);
    const deflated = deflateRawSync(data, { level: 9 });
    const useDeflate = deflated.length < data.length;
    const body = useDeflate ? deflated : data;
    const method = useDeflate ? 8 : 0;
    const nameBuf = Buffer.from(file.name, "utf8");

    const local = Buffer.alloc(30);
    local.writeUInt32LE(0x04034b50, 0);
    local.writeUInt16LE(20, 4); // version needed
    local.writeUInt16LE(0x0800, 6); // UTF-8 flag
    local.writeUInt16LE(method, 8);
    local.writeUInt16LE(0, 10); // time
    local.writeUInt16LE(0x21, 12); // date: 1980-01-01
    local.writeUInt32LE(crc, 14);
    local.writeUInt32LE(body.length, 18);
    local.writeUInt32LE(data.length, 22);
    local.writeUInt16LE(nameBuf.length, 26);
    local.writeUInt16LE(0, 28);
    locals.push(local, nameBuf, body);

    const cd = Buffer.alloc(46);
    cd.writeUInt32LE(0x02014b50, 0);
    cd.writeUInt16LE(20, 4); // version made by
    cd.writeUInt16LE(20, 6); // version needed
    cd.writeUInt16LE(0x0800, 8);
    cd.writeUInt16LE(method, 10);
    cd.writeUInt16LE(0, 12);
    cd.writeUInt16LE(0x21, 14);
    cd.writeUInt32LE(crc, 16);
    cd.writeUInt32LE(body.length, 20);
    cd.writeUInt32LE(data.length, 24);
    cd.writeUInt16LE(nameBuf.length, 28);
    cd.writeUInt16LE(0, 30); // extra
    cd.writeUInt16LE(0, 32); // comment
    cd.writeUInt16LE(0, 34); // disk
    cd.writeUInt16LE(0, 36); // internal attrs
    cd.writeUInt32LE(0, 38); // external attrs
    cd.writeUInt32LE(offset, 42);
    central.push(cd, nameBuf);

    offset += local.length + nameBuf.length + body.length;
  }

  const centralBuf = Buffer.concat(central);
  const end = Buffer.alloc(22);
  end.writeUInt32LE(0x06054b50, 0);
  end.writeUInt16LE(0, 4);
  end.writeUInt16LE(0, 6);
  end.writeUInt16LE(files.length, 8);
  end.writeUInt16LE(files.length, 10);
  end.writeUInt32LE(centralBuf.length, 12);
  end.writeUInt32LE(offset, 16);
  end.writeUInt16LE(0, 20);

  writeFileSync(outPath, Buffer.concat([...locals, centralBuf, end]));
  return files.length;
}

// ---------------------------------------------------------------------------

if (repack) {
  console.log("== 先重新打包 tgz ==");
  run(process.execPath, [join(HERE, "pack.mjs")]);
}

if (!existsSync(DIST)) throw new Error(`no dist/: ${DIST}`);
const tgzs = readdirSync(DIST).filter((f) => f.endsWith(".tgz")).sort().reverse();
if (!tgzs.length) throw new Error("no .tgz in dist/; run: node scripts/pack.mjs");
const tgzName = tgzs[0];
const tgzPath = join(DIST, tgzName);
const version = (tgzName.match(/-(\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?)\.tgz$/) || [])[1] ?? "0.0.0";

const zipName = `cf-connect-dingtalk-plugin-${version}.zip`;
const zipPath = join(DIST, zipName);

rmSync(STAGE, { recursive: true, force: true });
mkdirSync(STAGE, { recursive: true });

// 解包 tgz → zip-stage/package（同时带出根级 .codefree-plugin 保险副本）
run("tar", ["-xzf", tgzPath, "-C", STAGE]);

// 顶层再放一份样例与说明，方便不翻 package/ 也能上手
for (const [name, from] of [
  ["config.example.toml", REPO_ROOT],
  ["QUICKSTART.md", PLUGIN_DIR],
  ["install.ps1", PLUGIN_DIR],
  ["README.md", PLUGIN_DIR],
]) {
  const src = join(from, name);
  if (existsSync(src)) copyFileSync(src, join(STAGE, name));
}

// 把 tgz 本身也放进去：zip 解出来后可以直接拿去比赛平台上传
copyFileSync(tgzPath, join(STAGE, tgzName));

const count = zipDirectory(STAGE, zipPath);
const size = statSync(zipPath).size;
console.log(`\n✅ ${zipName}`);
console.log(`   输出：${zipPath}`);
console.log(`   大小：${(size / 1024 / 1024).toFixed(2)} MB（${count} 个条目）`);
console.log(`   内含：package/、${tgzName}、config.example.toml、QUICKSTART.md、install.ps1、README.md`);
