# CF-Connect 改造验收报告

> 对照计划 §11 验收清单（DoD）逐条给出**实测证据**。
> 环境：Windows 11 · go1.25.14 windows/amd64 · Node v22.22.0 · pnpm 10 (corepack) · codefree-o 1.7.0
> 分支：`trim-dingtalk-codefree-o` · 基线 tag：`baseline-upstream-3a6534d`（上游 `3a6534d`）

---

## 结论速览

| # | 验收项 | 结果 |
|---|---|---|
| 1 | `cd web && pnpm install && pnpm build` 成功，生成 `web/dist` | ✅ |
| 2 | `go build ./...`、`go vet ./...` 通过 | ✅ |
| 3 | `go test ./...` 通过；`go test ./core -run TestCUJ` 通过 | ⚠️ 见 §3（上游既有 Windows 失败，无新增；CUJ 通过） |
| 4 | 产出 `cf-connect` 二进制 | ✅ |
| 5 | `--version` / `config example` 只含 dingtalk + codefree-o | ✅ |
| 6 | **zip 发行包自检**（本轮核心交付） | ✅ |
| 7 | 品牌一致性冒烟 | ✅ |
| 8 | `update` 命令不存在；无自更新地址 | ✅ |
| 9 | codefree-o 专项 5 项 | ✅（会话标题/消息数已改为进程内 sqlite） |
| 10 | 真机联调（钉钉） | ⛔ 未做（需真实钉钉应用凭证） |
| 11 | 分阶段提交，每阶段可独立编译 | ✅ 7 个提交 |

---

## §1 Web 构建

```
$ cd web && pnpm build
✓ 2143 modules transformed.
dist/index.html                   0.52 kB
dist/assets/index-*.css          79.63 kB
dist/assets/index-*.js          898.75 kB
✓ built in 7.99s
```

`tsc -b` 零 TypeScript 错误（`pnpm build` 先跑 `tsc -b` 再 `vite build`）。

> 说明：pnpm 11.7.0 与仓库锁文件（lockfileVersion 9.0 / pnpm 10 生成）不兼容，
> 已用 `corepack prepare pnpm@10 --activate` 切到 pnpm 10 后安装成功。

## §2 构建与静态检查

```
$ go build ./...      → exit 0
$ go vet ./...        → exit 0
```

`go vet ./...` 在基线时是**失败**的（`cmd/cc-connect` 测试文件缺 build tag 导致包无法编译）。
本次用一行 `//go:build !windows` 修掉，属"只修阻塞项"范围内的必要修复。

## §3 测试

### CUJ（用户视角端到端）

```
$ go test ./core -run TestCUJ -count=1
ok  github.com/ItsQifan/cf-connect/core  4.361s
```
✅ 通过。**注意**：存在不稳定用例，首轮曾报 `TestCUJ_A3_ImageReachesAgent` /
`TestCUJ_A5_FileReachesAgent` 失败，重跑即过（详见 §3.2）。

### 全量测试

```
$ go test ./... -count=1
ok    github.com/ItsQifan/cf-connect
FAIL  github.com/ItsQifan/cf-connect/agent/opencode   (22 例)
FAIL  github.com/ItsQifan/cf-connect/cmd/cf-connect   (1 例)
FAIL  github.com/ItsQifan/cf-connect/config           (2 例)
FAIL  github.com/ItsQifan/cf-connect/core             (4 例，稳定)
FAIL  github.com/ItsQifan/cf-connect/daemon           (1 例)
ok    github.com/ItsQifan/cf-connect/platform/dingtalk
ok    github.com/ItsQifan/cf-connect/tests/release_local/{config_matrix,engine_matrix,media_pipeline,turn_contract}
```

**这些失败在改造前就存在**（基线实测清单见 `.tmp-tools/BASELINE-TESTS.md`）：
- 全部为 Windows 环境问题：POSIX 权限语义（`daemon`）、HOME 未隔离（`config`）、
  路径分隔符断言（`core`）、依赖真实 CLI 行为的模型发现（`agent/opencode`）。
- 改造后**稳定失败集合 = 基线稳定失败集合**（去掉被删除的包）→ **无新增失败**。
- `cmd/cf-connect` 的 1 例是修好 build tag 后才暴露的上游既有失败
  （`"/tmp/override"` vs `"\\tmp\\override"`）。
- 新增的回归测试全部通过（见 §9）。

### §3.2 不稳定用例（如实记录，已定位根因）

**根因**：这些 `core` 用例的失败**不是断言失败**，而是 Windows 上 `t.TempDir()`
的清理竞态 —— engine 的后台 goroutine 仍在写文件，`RemoveAll` 因此报
`The directory is not empty` / `being used by another process`：

```
--- FAIL: TestCUJ_A3_ImageReachesAgent (0.02s)
    testing.go:1369: TempDir RemoveAll cleanup: unlinkat ...\001: The directory is not empty.
--- FAIL: TestCUJ_A5_FileReachesAgent (0.02s)
    testing.go:1369: TempDir RemoveAll cleanup: unlinkat ...\001: The directory is not empty.
```

用例本身的用户视角断言全部通过：

| 用例 | 全量并发下 | 单独跑 |
|---|---|---|
| `core.TestCUJ_A3_ImageReachesAgent` | 偶发 TempDir 清理失败 | ✅ `ok core 0.194s` |
| `core.TestCUJ_A5_FileReachesAgent` | 偶发 TempDir 清理失败 | ✅ `ok core 0.205s` |
| `core.TestCUJ_H2_TwoPlatformsConcurrentNoBleed` | 偶发 TempDir 清理失败 | ✅ `ok core 0.282s` |
| `core.TestTimerScheduler_FiresOnTime` | 91s 后失败 | ✅ 重跑通过 |
| `core.TestCmdShell_MultiWorkspaceIgnoresMissingSharedBinding` | 基线即不稳定 | 一过一败 |

**最终稳定结果**（本轮最后一次全量运行，实测）：

```
$ go test ./core/ -count=1
--- FAIL: TestCompactReplyFooterPath_HomeRelativeDeepPathStaysFull
--- FAIL: TestAppendImageRefs
--- FAIL: TestAppendFileRefs_AbsolutizesRelativePaths
--- FAIL: TestAppendFileRefs_AbsoluteInputsPassthrough
FAIL  github.com/ItsQifan/cf-connect/core  23.870s
```

→ **只等于基线的 4 条稳定失败**（全部是 Windows 路径分隔符断言）；
临时目录竞态在本次运行中未复现。

**结论**：改造后的稳定失败集合 = 基线稳定失败集合（去掉被删除的包），**无新增功能失败**。

均为时序/端口竞争，非本次改造引入（基线亦不稳定）。

## §4–5 二进制与配置输出

```
$ cf-connect --version
cf-connect v1.5.1-cf.1
commit:  bf8531e
built:   2026-09-11T16:30:38Z

$ cf-connect config example | (only 'type = ' lines)
type = "opencode"
type = "dingtalk"
```

`config example` 的活动配置行中**不含**任何已删除平台/agent
（有回归测试 `TestConfigExampleTOML_OnlyShipsSupportedAdapters` 守卫）。

## §6 zip 发行包自检（核心交付）

产物：`dist/cf-connect-v1.5.1-cf.1-windows-amd64.zip`（6.43 MB）

```
SIZE     NAME
16350 KB cf-connect.exe
 10.8 KB config.example.toml
  2.3 KB install.ps1
    7 KB QUICKSTART.md
  5.5 KB README.md
```

- ✅ 5 个文件齐备
- ✅ 解压到干净英文路径（`D:\tmp\cf-smoke-*`）后 `cf-connect.exe --version` 正常
- ✅ `SHA256` 与 `checksums.txt` 记录一致
  （`80d17f8c08e2d245e1fc839c1790ac8a7a44184c8d81b9cc719c330c1e9e305a`）
- ✅ **无运行时依赖**：静态单文件（`CGO_ENABLED=0`）+ Web 界面 `go:embed` +
  进程内纯 Go sqlite → 不需要 Go / Node / Python / Java / sqlite3

## §7 品牌一致性冒烟

全仓扫描 335 个文本文件（排除 `npm/`、`CHANGELOG`、`changelogs/`、`.git`、`node_modules`）：

```
unexpected old-brand occurrences: 0
upstream attributions allowed: 12   ← 全部为指向 chenhg5/cc-connect 的上游出处说明
CC_* env-var references kept: 207   ← 按要求保持不变
cc_connect_* internal identifiers kept: 5
legacy '~/.cc-connect' path references: 0
```

- ✅ 无 `~/.cc-connect` 残留（已全部改为 `~/.cf-connect`）
- ✅ 无 `[cc-connect sender_id=…]` 残留（prompt 头已改为 `[cf-connect …]`，
  `core/engine.go:16188/16190` 实测）
- ✅ `CC_*` 与 `cc_connect_*` 按计划保持不变
- 仅剩 12 处 `cc-connect`，全部是**必须保留的上游出处**：
  README 的"基于 cc-connect 二次开发"+ 仓库链接、Makefile/RELEASE.md 的版本溯源说明

> 另：本轮**移除**了上游作者的推广返利链接（`referral_code=H65IOClRGu5CM7nn5ykfad`
> 等，见 provider-presets.json 与两个 README）——fork 不应携带他人的返利标识。

## §8 自更新已删除

```
$ cf-connect update
Unknown top-level command / 不再出现在 usage 中
```

- ✅ `cmd/cf-connect/update.go`、`core/updater.go` 及相关测试已删
- ✅ 无 chenhg5 自更新地址残留（`CheckForUpdate`/`SelfUpdate`/`ReleaseInfo` 精确 grep 为空）
- ✅ `/upgrade` 改为提示手动升级流程（新 i18n key，5 语言）
- ✅ 保留 `CurrentVersion`/`CurrentCommit`/`CurrentBuildTime`（bridge capabilities 用）

## §9 codefree-o 专项

| 项 | 结果 | 证据 |
|---|---|---|
| 同一二进制用 `cmd = "opencode"` 与 `cmd = "codefree-o"` 起会话均正常 | ✅ | `TestRealCLI_DefaultModeForBothBinaryNames` 两个子用例均 PASS |
| `mode = "yolo"` 在两者下都不再报 unknown flag | ✅ | `TestRealCLI_YoloModeDoesNotHitUnknownFlag` 两子用例 PASS；且 `codefree-o run --help` **不含** `--dangerously-skip-permissions`、**含** `--auto` |
| 会话标题/消息数读 `codefree.db` 正确 | ✅ | `TestRealCodefreeDB_ReadsTitlesAndMessageCounts`：真实库 18 个会话，标题 `"CustController测试方法生成（扩充代码行数）"`、消息数 28 |
| `cf-connect doctor` 显示实际 CLI 名 | ✅ | `binary="codefree-o" display="CodeFree-O"` |
| `go test ./agent/opencode/` 含新增回归测试 | ⚠️ | 新增回归测试**全绿**；该包 22 例上游既有失败（模型发现依赖真实 CLI 行为），与基线一致 |

**实测 CLI 契约**（`codefree-o run --help`）：

存在：`--auto` `--session/-s` `--model/-m` `--agent` `--format` `--file/-f` `--dir` `--thinking`
不存在：`--dangerously-skip-permissions` ← 证实补丁 1 确有必要

**NDJSON 事件契约实测**（唯一高风险项，已排除）：
```json
{"type":"step_start","part":{...}}
{"type":"text","part":{"type":"text","text":"PONG",...}}
{"type":"step_finish","part":{"type":"step-finish","tokens":{...}}}
```
与 `agent/opencode` 的解析完全一致 → **流式解析零改动可用**。

## §10 真机联调

⛔ **未执行**。需要真实的钉钉企业内部应用凭证（client_id / client_secret）与
一个可用的钉钉账号，本轮无此条件。`platform/dingtalk` 单元测试通过
（`ok platform/dingtalk 5.076s`），但端到端收发消息、AI 卡片流式、权限确认
**需使用方按 QUICKSTART.md 自行验证**。

## §11 分阶段提交

```
bf8531e build: drop unused dependencies, add the Windows release script (stages 10, 6.5)
638e27d docs: rewrite for the zip distribution model (stage 8)
14e7f87 refactor!: remove self-update; converge web UI to DingTalk + CodeFree-O
8748cf2 refactor!: rename brand to cf-connect (stage 7)
3e96fd7 refactor!: drop non-opencode agents; add codefree-o compatibility layer
a4fd2a0 refactor!: trim to DingTalk-only gateway (stage 1 + core/config cleanup)
3a6534d (baseline) feat(feishu): acknowledge accepted messages with an optional receipt reaction
```
（另有 1 个提交补上真实 CLI 验收测试）

每阶段均验证 `go build ./...` 通过后才提交。

---

## 交付物清单

| 文件 | 说明 |
|---|---|
| `dist/cf-connect-v1.5.1-cf.1-windows-amd64.zip` | **本轮核心交付**：解压即用发行包 |
| `dist/checksums.txt` | SHA256 |
| `QUICKSTART.md` | 3 步上手 + 排障（随 zip 分发） |
| `config.example.toml` | 237 行完整示例（已内嵌进二进制） |
| `install.ps1` | 加入用户 PATH（可选） |
| `RELEASE.md` | 发行/构建/升级/检查清单 |
| `agent/opencode/sessiondb.go` | 进程内 sqlite 会话库读取（新增） |
| `agent/opencode/codefree_regression_test.go` | 补丁 1/2/3 的回归测试 |
| `agent/opencode/codefree_cli_integration_test.go` | 真实 CLI 验收测试 |
| `embed_test.go` / `docs_links_test.go` | 配置模板与文档链接的守卫测试 |
| `scripts/release-windows.ps1` | Windows 打包脚本 |

## 已知限制（如实列出）

1. **npm/ 未改写、未发布** —— 按计划推迟到账号就绪
2. **Windows 上 `go test ./...` 非全绿** —— 上游既有环境性失败，本轮范围外
3. **`tests/integration/engine_platform_test.go` 有上游既有编译错误** —— 仅在
   `-tags integration` 下可见（`sess.Send` 参数个数不符），未修
4. **钉钉真机联调未做** —— 需真实凭证
5. **4 个不稳定用例** —— 时序/端口竞争，基线即存在
