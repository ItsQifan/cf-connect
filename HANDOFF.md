# 交接文档（CF-Connect 改造）

> 写给接手的新会话/新同事。所有数据均为**实测**，来源是本轮会话的工具输出。
> 有不确定的地方会明确标注，不编造。

- **仓库**：`E:\my_idea_workspace\cf-connect`
- **分支**：`trim-dingtalk-codefree-o`（**未合并回 main，未推送**）
- **HEAD**：`80b243d`
- **上游基线 tag**：`baseline-upstream-3a6534d`（= 上游 cc-connect `3a6534d`）
- **工作树**：干净（`git status` 无输出；`dist/` 被 gitignore，属预期）
- **改造计划原文**：`E:\my_idea_workspace\workspace_obsidian\codefree-connect-plan.md`
- **本轮核心交付**：`dist/cf-connect-v1.5.1-cf.1-windows-amd64.zip`（6.43 MB）

---

## 1. 这个项目现在是什么

**钉钉 ↔ 本地 CodeFree-O 的单通道网关**：钉钉消息 → 子进程调用本地 agent → NDJSON 流式结果回推钉钉。

```
钉钉 ──(Stream 长连接，无需公网 IP)──▶ cf-connect ──(子进程 + NDJSON)──▶ codefree-o / opencode
```

| | 本轮之后 |
|---|---|
| 平台 | **只有钉钉**（`platform/dingtalk`，20 → 1） |
| 智能体 | **只有 opencode 适配器**，同时驱动 codefree-o（`agent/opencode`，17 → 1） |
| CLI / 二进制 | `cf-connect`（`cmd/cf-connect`） |
| module | `github.com/ItsQifan/cf-connect`（remote 走 `githubproxy.cc` 代理） |
| 配置目录 | `~/.cf-connect` |
| 版本号 | `Makefile` 的 `VERSION := v1.5.1-cf.1` |
| 依赖 | 直接依赖 9 个，`go.sum` 133 行（原 372 行） |

---

## 2. 环境事实（新会话先读这段）

### 工具链（已装好，无需重装）

| 工具 | 位置 / 版本 | 备注 |
|---|---|---|
| Go | `%LOCALAPPDATA%\Programs\go-toolchain\go`，**go1.25.14 windows/amd64** | 已写入用户级 `GOROOT` / `GOPATH` / 用户 `PATH` |
| Node | v22.22.0 | |
| pnpm | 11.7.0，但**必须切 pnpm 10** | 见下 |
| codefree-o | `C:\nvm4w\nodejs\codefree-o.ps1`，**1.7.0** | 已授权登录（会话中用户完成过 OAuth） |
| opencode | `C:\nvm4w\nodejs\opencode.ps1` | 回归测试用它，也在 PATH |

**关于 Go 的 PATH**：用户级 PATH 已写入，但**只有新开的终端**才继承。
当前/旧会话里请用仓库自带的包装脚本：

```powershell
cd E:\my_idea_workspace\cf-connect
.\scripts\go-dev.cmd version      # 自动探测 GOROOT，默认 GOPROXY=https://goproxy.cn,direct
.\scripts\go-dev.cmd build ./...
.\scripts\go-dev.cmd test ./...
```

覆盖代理：`set GOPROXY=direct` 后再调用即可。

**关于 pnpm**：仓库锁文件是 `lockfileVersion: '9.0'`（pnpm 10 生成），
用 pnpm 11 安装会失败。已执行过：

```powershell
corepack prepare pnpm@10 --activate
```

### 出网受限（重要）

本机直连 `go.dev` / `goproxy.cn` 等在 PowerShell 里 TLS 失败。
**Node 的 fetch 可以正常走代理**，这是本轮能装上 Go 的原因。

- 可用：`node` + `fetch`（需 `HTTPS_PROXY=http://127.0.0.1:7897`）
- 系统代理已开（`ProxyEnable=1` → `127.0.0.1:7897`）
- 若需下载大文件，参考思路：写个 `.mjs` 用 Node fetch 下载

---

## 3. 已完成的工作（9 个提交）

```
80b243d chore: add the repo's own verification scripts
bbbaef6 docs: add the acceptance report and the pre-trim baseline test record
3c193f1 test: add real-CLI acceptance tests for the codefree-o contract
bf8531e build: drop unused dependencies, add the Windows release script (stages 10, 6.5)
638e27d docs: rewrite for the zip distribution model (stage 8)
14e7f87 refactor!: remove self-update; converge web UI to DingTalk + CodeFree-O
8748cf2 refactor!: rename brand to cf-connect (stage 7)
3e96fd7 refactor!: drop non-opencode agents; add codefree-o compatibility layer
a4fd2a0 refactor!: trim to DingTalk-only gateway (stage 1 + core/config cleanup)

3a6534d (baseline-upstream-3a6534d) 上游基线
```

对应计划的阶段：1 → a4fd2a0，2+2.5 → 3e96fd7，7 → 8748cf2，5+6 → 14e7f87，
8 → 638e27d，10+6.5 → bf8531e，验收测试 → 3c193f1，验收记录 → bbbaef6，脚本 → 80b243d。

**回滚**：`git reset --hard baseline-upstream-3a6534d`（会丢掉全部改造），
或按提交逐个 revert。

### codefree-o 兼容层（三处补丁 + 一处计划外增强）

| # | 补丁 | 文件 | 要点 |
|---|---|---|---|
| 1 | 权限 flag 可配置 | `session.go` / `opencode.go` | 原硬编码 `--dangerously-skip-permissions`（两个 CLI 都不支持）。新增 `permission_flag` 选项，默认 `--auto`；`none`/`off`/`-` 表示不追加；仅 `yolo` 模式生效 |
| 2 | 数据目录按品牌识别 | `sessiondb.go`（新增） | 原固定读 `~/.local/share/opencode/opencode.db`。现按优先级：显式 `db_file`/`data_dir` → `$XDG_DATA_HOME`（仅 opencode）→ 品牌默认。codefree-o → `~/.codefree-o/.local/share/codefree.db` |
| 3 | 全局记忆文件 + 别名 | `opencode.go` | `GlobalMemoryFile()` 支持 `global_memory_file` 选项，并按品牌探测 `~/.codefree-o/.config/{OPENCODE,AGENTS}.md`；注册 `codefree-o` 别名；实现 `AgentDoctorInfo` |
| **计划外** | **进程内 sqlite** | `sessiondb.go` | 计划 §9.3 列为"可选"，**已改为必做**（理由见 §4） |

### 其它

- **自更新已删除**：`cmd/cf-connect/update.go`、`core/updater.go`、`core/updater_test.go`；
  `/upgrade` 改为提示手动升级（新 i18n key `MsgUpgradeRemoved`，5 语言）
- **全量改名**：`cc-connect` → `cf-connect`（大小写保留）；**`CC_*` 环境变量与
  `cc_connect_*` 内部标识保持不变**（按计划）
- **web 收敛**：`platformMeta.ts` 只留 dingtalk；agent 选择只留 opencode + codefree-o；
  删 `PlatformSetupQR.tsx` + `api/setup.ts`（QR 平台已不存在）；
  `provider-presets.json` 24 → 16 条（只留 opencode 段）
- **文档重写**：`README.md` / `README.zh-CN.md` / `INSTALL.md` / `QUICKSTART.md`（新增）/
  `RELEASE.md`（新增）/ `config.example.toml`（2135 → 237 行）/ 删 20 个已删平台的 guide

### 新增的守卫测试

| 测试 | 文件 | 作用 |
|---|---|---|
| `TestBuildRunArgs_YoloPermissionFlagIsConfigurable` 等 | `agent/opencode/codefree_regression_test.go` | 补丁 1/2/3 的回归覆盖 |
| `TestRealCodefreeDB_ReadsTitlesAndMessageCounts` | `agent/opencode/codefree_real_db_test.go` | 对真实 codefree.db 读标题/消息数（`-tags codefree_integration`） |
| `TestRealCLI_*` | `agent/opencode/codefree_cli_integration_test.go` | 真实 CLI 验收（`-tags codefree_integration`） |
| `TestConfigExampleTOML_*` | `embed_test.go` | 配置模板可解析 + 不含已删适配器 |
| `TestDocsHaveNoDanglingLinks` 等 | `docs_links_test.go` | 文档死链（曾揪出 5 条） |

---

## 4. 关键决策与理由（别重新推翻）

| 决策 | 理由（实测） |
|---|---|
| **sqlite 改为进程内必做** | 本机**没有 `sqlite3` CLI**（`Get-Command sqlite3` 为空），而同事机器大概率同样。不做 → 会话标题/消息数永远为空。`modernc.org/sqlite` 本来就是依赖，零新增成本 |
| 只修阻塞项，不追全绿 | 用户明确选择。基线在 Windows 上本来就不是全绿（见 §5） |
| `CC_*` / `cc_connect_*` 不改名 | 计划已定：`CC_HOOK_*` 是对外 hook 契约，`cc_connect_*` 是 bridge 协议/缓存键，改名破坏面大且用户无感 |
| `npm/` 不动 | 计划已定：推迟到 npm 账号/scope 就绪 |
| 移除上游推广返利链接 | `provider-presets.json` 与两个 README 里的 `referral_code=H65IOClRGu5CM7nn5ykfad` 是**上游作者的**返利标识，fork 不应携带 |
| 版本号 `v1.5.1-cf.1` | `<上游版本>-cf.<fork修订>`，便于追溯到裁剪自哪个上游 commit |
| `/upgrade` 保留但只回提示 | 删掉命令会让用户看到"未知命令"；保留可让旧的 `disabled_commands` 配置仍能解析 |

---

## 5. 测试现状（务必先读懂这段）

### 稳定结果

```
$ .\scripts\go-dev.cmd build ./...     → exit 0
$ .\scripts\go-dev.cmd vet ./...       → exit 0     （基线时是失败的，见下）
$ .\scripts\go-dev.cmd test ./core/ -run TestCUJ   → ok
```

### Windows 上 `go test ./...` **不是全绿**，且这是**基线既有**问题

最终稳定失败集合 = 基线稳定失败集合（去掉被删除的包）→ **无新增功能失败**。
完整清单与逐条根因见 **`docs/BASELINE-TESTS.md`**。摘要：

| 包 | 例数 | 根因（全部 Windows 环境性） |
|---|---|---|
| `agent/opencode` | 22 | 模型发现依赖真实 CLI 行为 |
| `cmd/cf-connect` | 1 | `"/tmp/override"` vs `"\\tmp\\override"` 路径分隔符断言 |
| `config` | 2 | HOME 未隔离（读到真实用户目录） |
| `core` | 4 | Windows 路径分隔符断言 |
| `daemon` | 1 | Windows 无 POSIX 0644 语义 |

**唯一被我们修掉的基线失败**：`cmd/cc-connect` 的测试文件缺 `//go:build !windows`
导致整包无法编译（这会让 `go vet ./...` 失败）。修掉后该包才暴露出上面那 1 例。

### 不稳定（flaky）用例 —— 根因已定位

`core` 里几条 CUJ 用例在全量并发跑时**偶发**失败，但**不是断言失败**，是 Windows 上
`t.TempDir()` 的清理竞态（engine 后台 goroutine 仍在写文件）：

```
--- FAIL: TestCUJ_A3_ImageReachesAgent (0.02s)
    testing.go:1369: TempDir RemoveAll cleanup: unlinkat ...: The directory is not empty.
```

- `TestCUJ_A3_ImageReachesAgent` / `A5_FileReachesAgent` / `H2_TwoPlatformsConcurrentNoBleed`
  → 单独跑全过（`ok core`，各 0.2s 左右）
- `TestTimerScheduler_FiresOnTime` → 偶发 91s 后失败，重跑通过
- `TestCmdShell_MultiWorkspaceIgnoresMissingSharedBinding` → 基线即不稳定

**若新会话看到这些失败，先单独重跑确认，不要当成回归。**

---

## 6. 三个验收命令（DoD）

```powershell
# 品牌一致性冒烟（0 = 干净）
python scripts\check_brand.py

# 文档死链（0 = 干净）
python scripts\check_doc_links.py

# 真实 CLI 验收（需要 codefree-o 已登录；未登录会自动 skip 而不是失败）
.\scripts\go-dev.cmd test -tags codefree_integration ./agent/opencode/ -run TestRealCLI -v -timeout 1500s
```

本轮最后一次实测结果：

```
check_brand.py     : scanned 344 files, unexpected old-brand occurrences: 0
                     CC_* 引用 207 处、cc_connect_* 5 处 —— 均按计划保持不变
check_doc_links.py : checked 49 links, dead: 0
TestRealCLI        : 6/6 PASS
                     （codefree-o 与 opencode 默认模式均可起会话；
                       yolo 两者都不报 unknown flag；
                       doctor 报 binary="codefree-o" display="CodeFree-O"）
```

---

## 7. 重新打包（改了代码之后）

```powershell
cd web; pnpm build; cd ..          # 必须先构建，web/dist 会被 go:embed 进二进制
.\scripts\release-windows.ps1      # 默认读 Makefile 的 VERSION
# 或指定版本： .\scripts\release-windows.ps1 -Version v1.5.1-cf.2
```

产出 `dist\cf-connect-<version>-windows-amd64.zip` + `dist\checksums.txt`。
打包后自检步骤见 `RELEASE.md` §3。

> `make release-all` 是类 Unix 用的（POSIX shell 循环），Windows 上用上面的 ps1。

**改版本号只需改 `Makefile` 的 `VERSION` 一处**，二进制 / 包名 / `--version` 都从这里取。

---

## 8. 未完成 / 已知限制（**新会话请从 §9 挑活**）

1. **钉钉真机联调未做** —— 需要真实钉钉企业内部应用凭证（`client_id`/`client_secret`）。
   `platform/dingtalk` 单测通过（`ok platform/dingtalk`），但端到端收发消息、
   AI 卡片流式、权限确认**均未验证**。这是交付前最大的一块空白。
2. **`npm/` 未改写、未打包、未发布** —— 计划明确推迟。里面仍有 30 处 `cc-connect` 字面量。
3. **未推送到远程** —— 9 个提交都在本地 `trim-dingtalk-codefree-o` 分支；`main` 未动。
4. **`tests/integration/engine_platform_test.go` 有上游既有编译错误**
   （`sess.Send` 参数个数不符），仅在 `-tags integration` 下可见，未修。该文件本轮未改动。
5. **4 个不稳定用例**（见 §5），根因是测试的 TempDir 清理方式，不是产品代码。
6. **git 提交历史里 `dist/` 是 gitignore 的** —— 发行包不入库，只作为本地/内网分发产物。
7. **本轮删除了上游 20 个平台 guide** —— 若将来要恢复某个平台，文档需一并恢复。

---

## 9. 下一步可选任务

### A. 钉钉真机联调（优先级最高，交付阻塞项）

```powershell
cd D:\tmp\cf-smoke          # 或解压 zip 的任意目录
copy config.example.toml config.toml
notepad config.toml         # 填 work_dir / cmd / client_id / client_secret
.\cf-connect.exe            # 前台运行，看日志
# 钉钉里发消息 → 应该收到流式回显
# 发 /whoami → 拿到 userid → 填进 allow_from
```

验证清单：消息收发、流式回显、AI 卡片（若配了 `card_template_id`）、
权限确认（`mode = "default"`）、会话列表标题/消息数（补丁 2 的最终验证）、
`~/.cf-connect/` 正确生成。

### B. 推送与发版

```powershell
git push -u origin trim-dingtalk-codefree-o
# 打 tag（与 Makefile 的 VERSION 同名）
git tag v1.5.1-cf.1 && git push origin v1.5.1-cf.1
# 在 ItsQifan/cf-connect 的 Release 上传 dist/*.zip + checksums.txt
```

### C. 计划 §13「阶段 11」（比赛若报第 1 类才需要）

- **Skill（最轻）**：`dingtalk-bridge` skill 放进 `~/.codefree-o/.config/skills`，
  教 codefree-o 何时用 `cf-connect send` 回推、如何查会话、如何定时提醒
- **MCP server（较重）**：Go 官方 SDK（`modelcontextprotocol/go-sdk`）写 stdio server，
  暴露 `notify_dingtalk` / `ask_dingtalk_user` / `list_sessions` / `schedule_reminder`
- **案例材料**：架构图、DEMO 视频、可复制配置、踩坑清单、提效数据

### D. npm 发布（等账号/scope 就绪）

改 `npm/` 包名为 `@itsqifan/cf-connect` + 资产逻辑 + `npm pack` + `npm publish --access public`。

### E. 可选优化

- 修 §8 第 4 条那个上游既有编译错误（顺手，1 行参数）
- 给 flaky CUJ 用例加 `t.Cleanup` 等待，消除 TempDir 竞态
- 把 `make release-all` 也做成 Windows 可用（目前需 POSIX shell）

---

## 10. 关键文件速查

| 文件 | 作用 |
|---|---|
| `docs/ACCEPTANCE.md` | **对照计划 §11 的逐条验收报告**（含未做项、证据、命令输出） |
| `docs/BASELINE-TESTS.md` | 裁剪前的基线失败清单（证明"无新增失败"的依据） |
| `RELEASE.md` | 发行/构建/打包自检/内网分发/升级/发版检查清单 |
| `QUICKSTART.md` | 3 步上手 + 排障表（随 zip 分发） |
| `config.example.toml` | 237 行完整配置（已内嵌进二进制，`cf-connect config example` 输出它） |
| `agent/opencode/sessiondb.go` | 进程内 sqlite + 品牌感知的 DB 路径解析（补丁 2 + 增强） |
| `agent/opencode/opencode.go` | 补丁 1（`resolvePermissionFlag`）+ 补丁 3（`GlobalMemoryFile` / 别名 / `AgentDoctorInfo`） |
| `agent/opencode/session.go` | `buildRunArgs`（yolo flag 注入点） |
| `scripts/release-windows.ps1` | Windows 打包 |
| `scripts/check_brand.py` | 品牌一致性冒烟（DoD §7） |
| `scripts/check_doc_links.py` | 文档死链检查 |
| `scripts/go-dev.cmd` | Go 工具链包装（探测 GOROOT，默认 goproxy.cn） |
| `docs/dingtalk.md` / `docs/usage.md` | 钉钉适配器细节 / 命令用法 |

---

## 11. 踩过的坑（省得重踩）

1. **`.tmp-tools/` 已删除** —— 本轮所有临时脚本已清掉；有用内容已迁到
   `docs/`（记录）与 `scripts/`（工具）。
2. **PowerShell 里 `"..." > file` 是 UTF-16** —— 会把 TOML/JSON 写成 UTF-16，
   解析器报 "files cannot contain NULL bytes"。用
   `[System.IO.File]::WriteAllText($p, $t, (New-Object System.Text.UTF8Encoding($false)))`。
3. **`Set-Content -Encoding utf8` 会加 BOM** —— 提交信息开头带 BOM 会污染
   `git log`；写提交信息用上面的 UTF8Encoding($false)。
4. **`.cmd` 包装脚本里不能出现 `|`** —— 会被 cmd.exe 当管道解释。
   传正则给 `go test -run` 时避开 `|`（用单个测试名或 `-run 'Codefree'` 这类）。
5. **控制台显示中文会乱码** —— PowerShell 输出 CJK 显示成 `����`，但文件本身是好的。
   判断 CJK 用正则 `[\u4e00-\u9fff]`，别用肉眼。
6. **`scripts/` 原本被 gitignore** —— 已加 `!scripts/` / `!scripts/**` 否定规则。
7. **`git mv`/`Rename-Item` 目录会被正在运行的测试二进制锁住** —— 先 `job_kill`
   或等测试跑完。
8. **codefree-o 未登录时会以 OAuth 提示阻塞并超时**（约 190s），
   不是适配器 bug；`TestRealCLI` 已识别该情况并 skip。
9. **codefree-o 在进程结束后仍短暂持有 work_dir 句柄** —— Windows 上
   `t.TempDir()` 清理会报 sharing violation；集成测试用 `newWorkDir()` 重试规避。
10. **别在 `core/` 里写平台名** —— `AGENTS.md` 有硬性约定，本轮已清掉
    `"telegram"` / `"slack"` 硬编码分支。

---

## 12. 一分钟上手（给新会话的最短路径）

```powershell
cd E:\my_idea_workspace\cf-connect
git log --oneline -3                        # 确认在 80b243d
.\scripts\go-dev.cmd build ./...            # 应 exit 0
.\scripts\go-dev.cmd test ./core/ -run TestCUJ   # 应 ok
python scripts\check_brand.py               # 应 0 违规
Get-ChildItem dist                          # 应有 zip + checksums.txt
```

然后读 **`docs/ACCEPTANCE.md`**（全貌）→ 决定做 §9 里的哪一项。
