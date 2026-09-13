# 交接文档（cf-connect）

> 写给接手的新会话。所有数据均为**实测**，来源是本轮会话的工具输出。
> 不确定的地方明确标注"未实测"。**本文档已取代上一版交接文档。**
>
> **新会话先读**：本文 → `docs/DINGTALK-E2E.md`（钉钉联调验收报告，含 7 个缺陷的根因与证据）。

- **仓库**：`E:\my_idea_workspace\cf-connect`
- **分支**：`trim-dingtalk-codefree-o`（**未合并回 main，未推送**）
- **HEAD**：`fe7873a`（`docs: rewrite HANDOFF.md for the v1.0.0 handoff`）
- **工作树**：**不干净 —— 有未提交改动**（见 §3，这是本轮唯一待收尾的代码改动）
- **版本号**：`Makefile` 的 `VERSION := v1.0.0`
- **git tag**：只有 `baseline-upstream-3a6534d`；**v1.0.0 仍未打 tag**
- **上游基线 tag**：`baseline-upstream-3a6534d`（= 上游 cc-connect `3a6534d`）

---

## 1. 这个项目是什么

**钉钉 ↔ 本地 CodeFree-O 的单通道网关**：钉钉消息 → 子进程调用本地 agent → 结果回推钉钉。

```
钉钉 ──(Stream 长连接，无需公网 IP)──▶ cf-connect ──(子进程 + NDJSON)──▶ codefree-o / opencode
```

| | 现状 |
|---|---|
| 平台 | **只有钉钉**（`platform/dingtalk`） |
| 智能体 | **只有 opencode 适配器**，同时驱动 codefree-o（`agent/opencode`） |
| module | `github.com/ItsQifan/cf-connect` |
| 配置目录 | `~/.cf-connect`（会话状态、cron、timer、workspace 绑定、api.sock） |

---

## 2. 本轮（新会话）做了什么 —— 起因是一个 `/dir` 报错

### 现象

用户已配好 `admin_from`，但在钉钉里发 `/dir` 仍回：

```
🔒 Command /dir requires admin privilege. Set admin_from in config to authorize users.
```

### 根因：**改的不是被读取的那份配置文件**

配置解析优先级（`cmd/cf-connect/main.go:1440` `resolveConfigPath`）：

```
--config  →  ./config.toml（当前工作目录）  →  ~/.cf-connect/config.toml
```

`config.example.toml` **不在候选列表里，永远不会被读取**。用户先后改了两份无效的文件：

| 用户改的 | 为什么无效 |
|---|---|
| `D:\...\config.example.toml` | 模板文件，程序从不读 |
| `C:\Users\qifan\.cf-connect\config.toml` | 被同目录优先级更高的 `./config.toml` 遮蔽（当时 exe 的 cwd 在 D:） |

而真正生效的 `D:\software\cf-connect-v1.0.0-windows-amd64\config.toml` 里 `admin_from` 一直是**注释状态**。

### 证据链（可复现）

1. 进程路径 = `D:\software\cf-connect-v1.0.0-windows-amd64\cf-connect.exe`（前台运行，无计划任务、无 `daemon.json`）
2. 逐目录探针（`cf-connect config path`）：

   | cwd | 解析结果 |
   |---|---|
   | `D:\software\cf-connect-v1.0.0-windows-amd64` | `config.toml` |
   | `E:\my_idea_workspace` | `C:\Users\qifan\.cf-connect\config.toml` |

3. **锁文件旁证**：`.config.toml.lock` 与配置同目录，内容即持锁 PID
   - `D:\...\.config.toml.lock` = 活着的进程 PID
   - `C:\Users\qifan\.cf-connect\.config.toml.lock` = 早已退出的旧 PID
4. 语言旁证：生效那份 `language = "en"`，用户收到的正是英文提示

判定逻辑本身没问题：`core/engine.go:1259` `isAdmin` 对空值 **fail-closed**，`main.go:496` 把 `proj.AdminFrom` 接进去。

### 已做的处置

**(a) 本机部署**（工作区外，已提权写入并**用项目自带 loader 验证解析结果**）：

`D:\software\cf-connect-v1.0.0-windows-amd64\config.toml`
```
line 141: admin_from = "01140255060421424301"   ← 原本是注释
line 226: allow_from = "01140255060421424301"   ← 原本是注释（等于机器人对所有人开放）
```

验证输出（`config.Load` 实跑）：
```
language="zh" log.level="info" projects=1
project "my-project" admin_from="01140255060421424301"
  agent type="opencode" work_dir=D:\workspace_idea mode=yolo cmd=codefree-o
  platform "dingtalk" allow_from=01140255060421424301
```

**(b) 产品侧文档修复（任务 A，未提交）**：见 §3。

### ⚠️ 还没生效 —— 需要重启

当前进程 PID 800 启动于 `17:00:50`，而 `config.toml` 的 mtime 是 `17:00:59`，**晚于进程启动**。也就是说运行中的实例加载的是更早的版本。**必须重启 cf-connect 才能确认 `/dir` 真的通了。**

`admin_from` 虽有热重载路径（`main.go:1715`），但它只挂在 Web 管理 API 上（`main.go:891`），而配置里 `[management]` 是注释掉的 —— 所以只能重启。

**验证成功的标志**：启动日志里这条 WARN 应该消失
```
level=WARN msg="admin_from is not set — privileged commands (/shell, /show, /dir, /restart, /upgrade) are blocked..."
```

---

## 3. 未提交的改动（**新会话第一件事：决定提交还是继续改**）

```
 M QUICKSTART.md         +12 -1
 M config.example.toml   +9  -1
 M embed_test.go         +42
?? quickstart_test.go     +45（新文件）
```

### 改了什么

**`QUICKSTART.md`（随 zip 分发的唯一指南）** —— 补 `admin_from`，三处：
1. §3 配置样例：在 `[[projects]]` 下加 `admin_from` 注释，点明"位置与下方的 `allow_from` 正好相反"
2. §4 排障表：新增一行，直接对着报错 `requires admin privilege` 给出解法
3. §4 命令表：补 `/dir`（标注特权命令）、`/whoami` 补 `admin_from`、表下加七个特权命令的说明

**`config.example.toml`** —— 修掉一处**事实错误**。原文：
```
# Optional: who may run admin commands (defaults to allow_from).
```
**这是错的。** 已查证 `admin_from` 只从 `proj.AdminFrom` 逐字取值，**没有任何 `allow_from` 回退**：

| 位置 | 代码 |
|---|---|
| `main.go:496` / `main.go:1715` | `engine.SetAdminFrom(proj.AdminFrom)` |
| `management.go:767` | 同上 |
| `core/engine.go:1258` | 注释明写 *"Unlike AllowList, empty adminFrom means deny-all (fail-closed)"* |

空值 = 对所有人拒绝，正是用户遇到的报错。已改为准确描述（列七个特权命令 + fail-closed + `"*"` 语义）。

**新增回归测试**（`AGENTS.md` 要求"修复前 FAIL、修复后 PASS"，已实测）：
- `embed_test.go` → `TestConfigExampleTOML_AdminFromIsProjectLevel`
- `quickstart_test.go`（新文件）→ `TestQuickstartDocumentsAdminFrom`

**实测证据**（把两份文档 `git checkout` 回 HEAD、保留新测试）：
```
--- FAIL: TestConfigExampleTOML_AdminFromIsProjectLevel
    config.example.toml claims admin_from defaults to allow_from; an unset
    admin_from is fail-closed (denies every privileged command) ...
--- FAIL: TestQuickstartDocumentsAdminFrom
    QUICKSTART.md does not mention "admin_from" ...
    QUICKSTART.md does not mention "requires admin privilege" ...
    QUICKSTART.md does not mention "/dir" ...
```
恢复后两个都 PASS。

**约束提醒**：`embed_test.go:115-141` 有一条硬规则 —— 在 `[[projects]]` 段内，**任何注释行去掉 `#` 后都不能以 `allow_from` 开头**。新写模板时别踩。

---

## 4. 关键结论（**别重新推翻**）

| 结论 | 依据 |
|---|---|
| **配置优先级：`--config` → `./config.toml` → `~/.cf-connect/config.toml`；`config.example.toml` 永不读取** | `cmd/cf-connect/main.go:1440`；逐目录探针实测 |
| **`allow_from` 是平台级**，必须写在 `[projects.platforms.options]`；项目级会被 TOML 解码器**静默丢弃** | `config.ProjectConfig` 无此字段；`platform/dingtalk/dingtalk.go:101` |
| **`admin_from` 是项目级**（`[[projects]]` 下），且**空值 = 对所有人拒绝**（fail-closed，无 `allow_from` 回退） | `config/config.go:512`；`core/engine.go:1258`；`main.go:496` |
| **特权命令共 7 个**：`/shell` `/show` `/dir` `/restart` `/upgrade` `/web` `/diff`；另有 `/commands addexec`、`/cron addexec` 两个子命令 | `core/engine.go:1211` `privilegedCommands`、`:1235` `isPrivilegedCommandInvocation` |
| **`admin_from` 可热重载，但只挂在 Web 管理 API 上**；`[management]` 未启用时只能重启 | `main.go:891` → `:1715` |
| **cf-connect 没有交互式权限确认**，`mode` 只决定是否追加跳过权限的 flag | `agent/opencode/session.go:517` `RespondPermission` 是 no-op |
| **钉钉唯一的流式通道是 AI 卡片**，开关只有 `card_template_id`；`card_mode` 是飞书遗留，`[stream_preview]` 需平台实现 `MessageUpdater`（钉钉没有） | `core/engine.go:4990`；`core/progress_compact.go:311` |
| **用户可见文字用 `core.AgentDisplayName()`，注册/配置/会话归属用 `Agent.Name()`** | `core/registry.go`；`Name()` 必须保持 `"opencode"` |
| **首跑模板 = 内嵌的 `config.example.toml`**（`embed.go` `go:embed`），`cf-connect config example` 原样输出 | `cmd/cf-connect/main.go` `bootstrapConfig` |
| **`docs/` 不进 zip** —— 只有 `scripts/release-windows.ps1:63` 列的那 4 个额外文件 + exe | 实测 zip 内容见 §7 |
| `CC_*` 环境变量与 `cc_connect_*` 内部标识**保持不改名** | 对外 hook 契约 + bridge 协议/缓存键 |

---

## 5. 环境事实

### 工具链（已装好，无需重装）

| 工具 | 位置 / 版本 |
|---|---|
| Go | `%LOCALAPPDATA%\Programs\go-toolchain\go`，go1.25.14 windows/amd64（用户级 PATH 已写入，但**只有新终端**继承） |
| Node / pnpm | v22.22.0 / **10.34.5**（锁文件 `lockfileVersion: '9.0'`，pnpm 11 会失败） |
| codefree-o | `C:\nvm4w\nodejs\codefree-o.cmd`，1.7.0，已登录（userId 311476） |
| opencode | `C:\nvm4w\nodejs\opencode.ps1` |

仓库自带包装脚本（自动探测 GOROOT）：
```powershell
cd E:\my_idea_workspace\cf-connect
.\scripts\go-dev.cmd build ./...
.\scripts\go-dev.cmd test ./core/ -run TestCUJ
```

### 沙箱限制（本轮又踩了 2 次）

DSH 文件策略 `workspace-write`，**工作区外写入一律拒绝**：

| 操作 | 现象 | 处理 |
|---|---|---|
| `go build` / `go test` | `open C:\Users\...\go-build\...: Access is denied` | 把 `GOCACHE` 指到工作区内（见下），无需提权 |
| 写 `D:\software\...\config.toml` | `[sandbox: file access denied under workspace-write mode]` | **提权一次**（`danger-full-access`） |
| 发布脚本的 `go build` | 写 `...\pkg\mod\cache\...` 被拒 | 需要**提权一次** |
| `cd web; pnpm build` | `Error: spawn EPERM`（esbuild 用管道 stdio） | 需要**提权一次** |
| 运行 `cf-connect.exe` | `mkdir C:\Users\...\.cf-connect\crons: Access is denied` | 需要**提权一次** |
| 写注册表 | `Access to the registry key ... is denied` | 本轮仍未绕过 |

**推荐固定环境**：
```powershell
$env:GOCACHE="E:\my_idea_workspace\cf-connect\tmp\gocache"
$env:GOTMPDIR="E:\my_idea_workspace\cf-connect\tmp\gotmp"
$env:GOTELEMETRY="off"     # 否则每步都刷 telemetry 写权限报错（噪音，不影响结果）
$env:GOROOT = Join-Path $env:LOCALAPPDATA 'Programs\go-toolchain\go'
$env:PATH = "$env:GOROOT\bin;$env:PATH"
```

> `tmp\gocache` 已存在（约 275 MB，gitignore 覆盖）。嫌大可删，Go 会重建。

### 出网

- 系统代理 `127.0.0.1:7897`（`ProxyEnable=1`）
- **`api.dingtalk.com` 直连可用**，cf-connect 全程**未设** `HTTPS_PROXY` 就收发正常
- PowerShell 直连 `go.dev` / `goproxy.cn` TLS 失败；Node 的 fetch 可走代理

---

## 6. 测试现状

### 稳定通过

```
.\scripts\go-dev.cmd build ./...          → exit 0
.\scripts\go-dev.cmd vet .                → exit 0
.\scripts\go-dev.cmd test .               → ok（含本轮新增的 2 个测试）
python scripts\check_doc_links.py         → 46 链接，0 死链
python scripts\check_brand.py             → 0 违规
```

### Windows 上 `go test ./...` **不是全绿**，且全是基线既有问题（**无新增失败**）

| 包 | 例数 | 根因 |
|---|---|---|
| `agent/opencode` | 22 | 依赖真实 CLI 行为（基线） |
| `cmd/cf-connect` | 1 | `TestParseDaemonInstallArgs_WorkDirOverridesConfig`，路径分隔符（基线） |
| `config` | 2 | `TestLoad_DefaultsDataDir`（HOME 未隔离）、`TestLoad_ResolvesEnvPlaceholders`（分隔符）（基线） |
| `core` | 4 | 4 个 Windows 分隔符用例（基线） |
| `daemon` | 2 | `TestSchtasksInstall_TightensExistingScriptFrom0644`（基线）+ `TestMetaSaveLoad`（**沙箱导致**，写真实 `~/.cf-connect/daemon.json`） |

- **flaky**：`core` 的 `TestCUJ_A3_ImageReachesAgent` / `A5_FileReachesAgent` 偶发失败，报的是 `TempDir RemoveAll cleanup: ... directory is not empty`，**不是断言失败**。单独重跑即过。
- 完整基线清单：`docs/BASELINE-TESTS.md`。
- 本轮实测：`go test .`、`go test ./cmd/cf-connect/`、`go test ./config/` 的结果与上表逐条一致。

---

## 7. 发布产物状态

```
dist\cf-connect-v1.0.0-windows-amd64.zip   6,748,480 B
  SHA256 52efdd4b06a1a9298bea1280641c1072ed49ec0cf05ff01b5572f6a138d038bb
dist\cf-connect.exe                        23,487,488 B
dist\checksums.txt                         与实测 SHA256 一致
```

**zip 内容（实测，只有 5 个文件）**：
```
cf-connect.exe        16,745,472
config.example.toml   12,601
install.ps1            3,335
QUICKSTART.md          9,495
README.md              6,843
```

> ⚠️ **这个 zip 是旧的，不含本轮 QUICKSTART 修复**。实测确认包内 `QUICKSTART.md`
> 里 `admin_from` 与 `requires admin privilege` 均为 `False`。
> 要让 zip 用户拿到修复，**必须重新打包**（§9）。

---

## 8. 本机部署现状（`D:\software\cf-connect-v1.0.0-windows-amd64\`）

| 文件 | 说明 |
|---|---|
| `cf-connect.exe` | 16,745,472 B，= zip 内那份 |
| `config.toml` | **生效的配置**，已被本轮 + 用户编辑 |
| `config.example.toml` | **用户自己维护的中文版**（7,747 B），**不是仓库内嵌的英文版**（12,601 B） |
| `QUICKSTART.md` / `README.md` / `install.ps1` | 与 zip 一致（旧版） |

**用户自本轮起改动的项**（`config.Load` 实测确认）：

```
language = "zh"            ← 用户改（原为 "en"）
mode = "yolo"              ← 用户改（原为 "default"，现在会追加 --auto）
work_dir = "D:\\workspace_idea"
cmd = "codefree-o"
admin_from = "01140255060421424301"   ← 本轮补
allow_from = "01140255060421424301"   ← 本轮补
```

**一处小瑕疵（无害）**：`config.toml` 第 50 行有一个多余的 `language = "zh"`，
它落在 `[log]` 表里（因第 37 行 `[log]` 之后没有新表头），解析成 `log.language`
被静默忽略。真正的顶层值是第 35 行那个。**不影响行为**，想清爽可删第 50 行。

> **陷阱**：用户的 `config.example.toml` 是中文版、仓库的是英文版。这两份**内容不同步**，
> 且**都不是生效文件**。别再往 `.example` 里填值。

---

## 9. 下一步可选任务

### A0. 【最优先】重启并确认 `/dir` 通了

```powershell
cd D:\software\cf-connect-v1.0.0-windows-amd64
# 停掉 PID 800（控制台 Ctrl+C 或关窗口），然后
.\cf-connect.exe
```
确认 §2 那条 `admin_from is not set` WARN 消失，再在钉钉里发 `/dir`。
若仍失败：发 `/whoami` 核对返回的 User ID 是否就是 `01140255060421424301`。

### A1. 提交本轮改动

```
 M QUICKSTART.md   M config.example.toml   M embed_test.go   ?? quickstart_test.go
```
建议 commit message（沿用仓库 Conventional Commits 风格）：
```
docs: document admin_from and fix its false allow_from default

QUICKSTART.md is the only guide shipped in the release zip (docs/ is not
packaged), and it documented allow_from but never admin_from — while every
privileged command (/dir, /shell, /show, /diff, /web, /restart, /upgrade)
is fail-closed when admin_from is unset. Following the quickstart exactly
produced "Command /dir requires admin privilege" with no documented remedy
anywhere in the zip.

Also correct config.example.toml, which claimed admin_from "defaults to
allow_from". It does not: SetAdminFrom takes proj.AdminFrom verbatim and
an empty value denies every privileged command.

Regression tests: TestQuickstartDocumentsAdminFrom,
TestConfigExampleTOML_AdminFromIsProjectLevel.
```

### A2. 重新打包（让 zip 带上文档修复）

```powershell
# 1) 先建 web（web/dist 会被 go:embed 进二进制）。沙箱下需提权一次（esbuild spawn EPERM）
cd web; corepack pnpm build; cd ..
# 2) 设好 §5 的环境，然后
& .\scripts\release-windows.ps1 -Version v1.0.0
```
产出 `dist\cf-connect-<version>-windows-amd64.zip` + `dist\checksums.txt`。
**改版本号只需改 `Makefile` 的 `VERSION` 一处。**

> `pwsh` 在这台机器上**不在 PATH**（只有 Windows PowerShell），用 `& .\scripts\...ps1` 调用。
> 打包后自检见 `RELEASE.md` §3（**该文件被 gitignore，是本地 runbook**）。

### A3. 发布收尾

```powershell
git tag v1.0.0 && git push origin trim-dingtalk-codefree-o --tags
# 在 ItsQifan/cf-connect 的 Release 上传 dist/*.zip + checksums.txt
```
⚠️ README 里的文档链接已改成**绝对地址指向 `main`**，合并回 main 前指向旧内容，合并后自动正确。

### B. 【本轮新发现，建议做】配置遮蔽是静默的

`./config.toml` 与 `~/.cf-connect/config.toml` 同时存在时，后者被忽略且**无任何提示**。
本轮用户就是这么踩的（他改的那份 `~/.cf-connect/config.toml` 被静默无视）。
启动日志有 `config loaded path=...`（`main.go:320`），但前台一闪而过。

**建议**：`resolveConfigPath` 返回时若发现另一候选也存在，打一条 WARN 点名两个路径。

### C. 【本轮新发现，建议做】报错不说加载的是哪份配置

`core/i18n.go:3396` 现在是
`Command %s requires admin privilege. Set admin_from in config to authorize users.`
—— `in config` 指哪份？建议带上实际生效路径（`config.ConfigPath` 全局已有）。
同理 `[projects]` 下没有 `admin_from` 时启动那条 WARN 也可直接打印文件名。
**B + C 能把半小时的排查变成 5 秒。**

### D. 安全项（**需要用户决策**）

`allow_from` 在本轮之前一直是**关闭**状态，意味着那段时间**任何人都能对机器人说话**。
按 `docs/DINGTALK-E2E.md` / 上一版交接 §10.3：本机 codefree-o 挂着指向**内网 MySQL**
的 MCP（明文账号口令）—— 谁能对机器人说话，谁就能驱动一个能访问那个库的 agent。
**建议与用户确认是否需要轮换 `client_secret`。**

### E. 核实两条存疑项（沿用上一版）

1. `QUICKSTART.md` §6 说"会话状态/附件会写在 `<work_dir>\.cf-connect\`"。
   **实测未发现该目录**（会话状态实际在 `~/.cf-connect\`）。**很可能是错的，请核实后修掉。**
2. `install.ps1` 的 `REG_EXPAND_SZ` 修复**没有真机验证**（注册表写入被沙箱拒绝）。

### F. 补钉钉未覆盖场景

优先建议：**群聊**（`share_session_in_channel`、群内 @、卡片 `IM_GROUP` 投递）与**附件**。
其余未覆盖：`/stop` 打断、卡片降级熔断（403/429/5xx）、daemon 模式、cron/timer 端到端、
卡片内思考/工具步骤渲染。

### G. 其它可选优化

- F6：`agent_id` 解析了但从未使用（死配置），要么删要么接上。
- 修 `tests/integration/engine_platform_test.go` 的上游既有编译错误（`sess.Send` 参数个数不符），
  仅在 `-tags integration` 下可见。
- 给 flaky 的 CUJ 用例加 `t.Cleanup` 等待，消除 TempDir 竞态。
- 把 `make release-all` 做成 Windows 可用（目前是 POSIX shell 循环）。
- npm 发布（等账号/scope）：`npm/` 包名改 `@itsqifan/cf-connect`，里面仍有 30 处 `cc-connect` 字面量。

---

## 10. 关键文件速查

| 文件 | 作用 |
|---|---|
| `docs/DINGTALK-E2E.md` | **钉钉联调验收报告**（逐条验收 + 7 个发现 + 复现 + 修复记录） |
| `docs/BASELINE-TESTS.md` | 裁剪前的基线失败清单（"无新增失败"的依据） |
| `HANDOFF.md` | 本文 |
| `RELEASE.md` | 发行/构建/打包自检/分发（**gitignored，本地**） |
| `QUICKSTART.md` | 3 步上手 + 排障（随 zip 分发，**离线自足、无相对链接**） |
| `config.example.toml` | 完整配置模板（内嵌进二进制，`cf-connect config example` 输出它） |
| `cmd/cf-connect/main.go` | `resolveConfigPath`(:1440)、`SetAdminFrom` 接线(:496/:1715)、`reloadConfig`(:1580) |
| `core/engine.go` | `isAdmin`(:1259)、`privilegedCommands`(:1211)、`SetAdminFrom`(:1197) |
| `core/registry.go` | `AgentDisplayName()` —— 用户可见名字的解析 |
| `agent/opencode/session.go` | `buildRunArgs`（yolo flag 注入点）、`RespondPermission`（no-op） |
| `platform/dingtalk/card.go` | AI 卡片（createAndDeliver + streaming） |
| `scripts/release-windows.ps1` | 打包脚本（`:63` 列出进 zip 的额外文件） |
| `scripts/go-dev.cmd` / `check_brand.py` / `check_doc_links.py` | 工具 |
| `tmp/dingtalk-smoke/config.toml` | 联调用的**可用配置**（含真实 secret），可直接抄 |

---

## 11. 钉钉联调资源

| 项 | 值 |
|---|---|
| 应用 | 企业内部应用，`client_id = dingomm29pzoocbgqcvl`，机器人消息接收模式 = **Stream** |
| Client Secret | **不写在这里**。在 `tmp\dingtalk-smoke\config.toml`（gitignore 覆盖）。⚠️ 见 §12 |
| 用户钉钉 userid | `01140255060421424301` |
| 已验证的 AI 卡片模板 | `6b172a70-7dd1-44c1-a02d-654c664715f3.schema`，模板内 Markdown 组件的变量名必须是 `content` |
| 联调工作目录 | `tmp\dingtalk-smoke\` |

**凭证预检**（比跑起来看日志快得多）：
```powershell
node -e "fetch('https://api.dingtalk.com/v1.0/oauth2/accessToken',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({appKey:process.argv[1],appSecret:process.argv[2]})}).then(async r=>console.log(r.status, await r.text()))" dingomm29pzoocbgqcvl <client_secret>
```
成功 `200 {"expireIn":7200,...}`；失败 `400 {"code":"invalidClientIdOrSecret"}`。
**教训**：secret 是 64 字符，肉眼核对首尾各 10 位。

---

## 12. 安全注意

1. **`tmp\dingtalk-smoke\config.toml` 和 `D:\software\...\config.toml` 里都有真实 Client Secret。**
   `tmp/` 被 gitignore 覆盖不会进库；但**别打包整个仓库发人**。
2. 全仓库（除 tmp/）已确认**不含明文 secret**（grep 过）。
3. codefree-o 的全局配置 `~/.codefree-o/.config/codefree.json` 里挂了一个指向**内网 MySQL**
   的 MCP（明文账号口令）。**`allow_from` 必须设** —— 这正是上一轮 F1 被定为高优先级的原因，
   也是本轮 §9-D 建议轮换 secret 的原因。

---

## 13. 踩过的坑（省得重踩）

1. **改配置前先确认哪份生效** —— `cf-connect config path`（在目标 cwd 下跑）或看 `.config.toml.lock`。
   **`config.example.toml` 永远不是生效文件。** 本轮用户连踩两次。
2. **别把 `.go` 文件放进 `tmp/`** —— 会被 `go test ./...` 当成一个包编译，报
   `FAIL .../tmp [build failed]`。临时诊断程序用完**立刻删掉整个目录**。
3. **`.cmd` 包装脚本里不能出现 `|`** —— 会被 cmd.exe 当管道。传正则给 `go test -run` 时避开。
4. **仓库是 CRLF 检出** —— `gofmt -l` 会把 **152 个文件**（含没碰过的）全列出来。
   要判断真实格式问题，先把内容规范化成 LF 再跑 gofmt。**别直接 `gofmt -w`**。
5. **`check_doc_links.py` 会扫描仓库内解压出来的 zip 目录**，把包内文档的链接报成死链。
   跑之前先清掉解压目录。
6. **PowerShell 里 `"..." > file` 是 UTF-16** —— 会把 TOML/JSON 写成 UTF-16，解析器报
   "files cannot contain NULL bytes"。用
   `[System.IO.File]::WriteAllText($p, $t, (New-Object System.Text.UTF8Encoding($false)))`。
7. **`Set-Content -Encoding utf8` 会加 BOM** —— 提交信息开头带 BOM 会污染 `git log`；
   写提交信息用上面的 UTF8Encoding($false)，再 `git commit -F`。
8. **控制台显示中文会乱码** —— PowerShell 输出 CJK 显示成乱码，但文件本身是好的。
   判断 CJK 用正则 `[\u4e00-\u9fff]`，别用肉眼。
9. **codefree-o 未登录时会以 OAuth 提示阻塞并超时**（约 190s），不是适配器 bug；
   `TestRealCLI` 已识别并 skip。
10. **codefree-o 在进程结束后仍短暂持有 work_dir 句柄** —— Windows 上 `t.TempDir()` 清理会报
    sharing violation；集成测试用 `newWorkDir()` 重试规避。
11. **别在 `core/` 里写平台/agent 名** —— `AGENTS.md` 有硬性约定。`Agent.Name()` 也不该
    直接进用户可见文案（用 `AgentDisplayName()`）。
12. **`go-dev.cmd` 会给每条命令刷 telemetry 写权限报错**（`AppData\Roaming\go\telemetry\...`）——
    是**噪音**，不影响退出码。设 `$env:GOTELEMETRY="off"` 可消除。

---

## 14. 一分钟上手（给新会话的最短路径）

```powershell
cd E:\my_idea_workspace\cf-connect
git log --oneline -3          # 确认在 fe7873a
git status --short            # 应有 4 项未提交改动（§3）
$env:GOCACHE="E:\my_idea_workspace\cf-connect\tmp\gocache"
$env:GOTMPDIR="E:\my_idea_workspace\cf-connect\tmp\gotmp"
$env:GOTELEMETRY="off"
.\scripts\go-dev.cmd build ./...                  # 应 exit 0
.\scripts\go-dev.cmd test .                       # 应 ok（含 2 个新回归测试）
python scripts\check_brand.py                     # 应 0 违规
python scripts\check_doc_links.py                 # 应 0 死链
```

然后决定做 §9 里的哪一项。**若用户还在问 `/dir`：先做 §9-A0（重启）。**
