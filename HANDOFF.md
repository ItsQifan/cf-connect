# 交接文档（cf-connect）

> 写给接手的新会话。所有数据均为**实测**，来源是本轮会话的工具输出。
> 不确定的地方会明确标注"未实测"。**本文档已取代上一版交接文档**（上一版写的是裁剪阶段，
> 结论已过期）。
>
> **新会话先读**：本文 → `docs/DINGTALK-E2E.md`（钉钉联调验收报告，含 7 个缺陷的根因与证据）。

- **仓库**：`E:\my_idea_workspace\cf-connect`
- **分支**：`trim-dingtalk-codefree-o`（**未合并回 main，未推送**）
- **本轮最后一个功能提交**：`72c2be8`
  （本文档自身的提交不含代码改动，用 `git log -1 --oneline` 看当前 HEAD）
- **工作树**：干净（`git status` 无输出）
- **版本号**：`Makefile` 的 `VERSION := v1.0.0`
- **本轮交付**：`dist/cf-connect-v1.0.0-windows-amd64.zip`（6,748,480 字节，
  SHA256 `52efdd4b06a1a9298bea1280641c1072ed49ec0cf05ff01b5572f6a138d038bb`）
- **上游基线 tag**：`baseline-upstream-3a6534d`（= 上游 cc-connect `3a6534d`）
- **git tag**：只有 `baseline-upstream-3a6534d`；**v1.0.0 还没打 tag**

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
| module | `github.com/ItsQifan/cf-connect`（remote 走 `githubproxy.cc` 代理） |
| 配置目录 | `~/.cf-connect`（会话状态、cron、timer、workspace 绑定、api.sock） |
| 依赖 | 直接依赖 9 个，`go.sum` 133 行 |

### 当前阶段结论

**功能已可用，且完成真机验收。** 联调结论（`docs/DINGTALK-E2E.md`）：
Stream 收发、真实 AI 回复、多轮上下文、工具调用、**AI 卡片流式**、
会话标题/消息数、白名单拦截、`~/.cf-connect/` 生成 —— 全部通过。

**剩下的是发布流程与未覆盖场景**，不是阻塞性缺陷。见 §7。

---

## 2. 环境事实（先读这段，能省很多时间）

### 工具链（已装好，无需重装）

| 工具 | 位置 / 版本 | 备注 |
|---|---|---|
| Go | `%LOCALAPPDATA%\Programs\go-toolchain\go`，go1.25.14 windows/amd64 | 用户级 PATH 已写入，但**只有新终端**继承 |
| Node | v22.22.0 | |
| pnpm | **10.34.5**（已 `corepack prepare pnpm@10 --activate`） | 锁文件是 `lockfileVersion: '9.0'`，pnpm 11 会失败 |
| codefree-o | `C:\nvm4w\nodejs\codefree-o.cmd` → `node_modules\@srdcloud\codefree-o\bin\codefree-o.exe`，1.7.0 | 已登录（userId 311476） |
| opencode | `C:\nvm4w\nodejs\opencode.ps1` | |

当前会话里用仓库自带包装脚本（自动探测 GOROOT）：

```powershell
cd E:\my_idea_workspace\cf-connect
.\scripts\go-dev.cmd build ./...
.\scripts\go-dev.cmd test ./core/ -run TestCUJ
```

### 沙箱限制（本轮踩了 4 次，务必先设好再干活）

本轮的 DSH 文件策略是 `workspace-write`，**工作区外一律拒绝写入**。表现和绕法：

| 操作 | 现象 | 处理 |
|---|---|---|
| `go build` / `go test` | `open C:\Users\...\go-build\...: Access is denied` | **把 GOCACHE 指到工作区内**（见下），无需提权 |
| 发布脚本里的 `go build` | `go: writing stat cache: ...\go\pkg\mod\cache\...: Access is denied` + 脚本中断 | 需要**提权一次**（`danger-full-access`）；Go 必须写工作区外的模块 stat 缓存 |
| `cd web; pnpm build` | `Error: spawn EPERM`（esbuild 用管道 stdio 启子进程） | 需要**提权一次** |
| 运行 `cf-connect.exe` | `mkdir C:\Users\...\.cf-connect\crons: Access is denied` | 需要**提权一次**（默认数据目录在工作区外） |
| 写注册表（如测 `install.ps1`） | `Access to the registry key ... is denied` | 本轮未能绕过，相关验证只能靠代码审查 |

**推荐固定环境（放在每个 pwsh 调用里）**：

```powershell
$env:GOCACHE="E:\my_idea_workspace\cf-connect\tmp\gocache"
$env:GOTMPDIR="E:\my_idea_workspace\cf-connect\tmp\gotmp"
$env:GOROOT = Join-Path $env:LOCALAPPDATA 'Programs\go-toolchain\go'
$env:PATH = "$env:GOROOT\bin;$env:PATH"     # 发布脚本直接调 `go`，必须上 PATH
```

> `tmp\gocache` 已存在（约 275 MB，gitignore 覆盖）。嫌大可删，Go 会重建。

### 出网

- 系统代理 `127.0.0.1:7897`（`ProxyEnable=1`）。
- **`api.dingtalk.com` 直连可用**，cf-connect 全程**未设** `HTTPS_PROXY` 就收发正常。
- PowerShell 里直连 `go.dev` / `goproxy.cn` TLS 失败；**Node 的 fetch 可走代理**，
  下载大文件参考：写个 `.mjs` 用 Node fetch。

---

## 3. 本轮做了什么（3 个提交）

```
72c2be8 docs: fix the shipped README's false claims and dead links
551b142 docs: drop the stale doctor self-check step from INSTALL.md
31c3bd4 fix: correct user-facing agent labels and repair trim leftovers
774a2f6 docs: add a handoff document for the next session   ← 上一轮
80b243d chore: add the repo's own verification scripts
```

`31c3bd4` 是主体：联调 + 修 7 个缺陷 + 15 个回归测试。详见 `docs/DINGTALK-E2E.md` §7/§8。

### 修了什么（每条都有回归测试，且都验证过"修复前 FAIL、修复后 PASS"）

| 编号 | 问题 | 严重度 |
|---|---|---|
| F1 | `config.example.toml` 把 `allow_from` 写在 `[[projects]]` 下 → **被静默丢弃**，机器人对所有人开放；且 `ProjectConfig` 根本没这个字段。已移到平台层 + 启动时打 WARN | **高（安全）** |
| F2 | 文档声称 `mode="default"` 会"逐次确认"、QUICKSTART 让人"在钉钉里点确认" → **该流程不存在**（`RespondPermission` 是空实现，无头 `run` 直接执行工具） | **高（文档与能力不符）** |
| F3 | QUICKSTART / INSTALL 让 Windows 用户跑 `cf-connect doctor` → 只打印 `not supported on Windows` | 中 |
| F4 | 未配 `card_template_id` 时每轮打 WARN；新增 `core.ErrStreamingCardUnavailable` 降为 Debug | 中 |
| F5 | `card_mode` / `[stream_preview]` 对钉钉完全无效，模板里却有注释说它是"钉钉 AI 卡片开关" | 低 |
| F6 | `agent_id` 解析了但从未使用（死配置） | 低，**未修** |
| F7.1 | `/list` `/status` `/start` `/skills` 和进度卡片显示注册名 `opencode` 而非品牌 `CodeFree-O`。新增 `core.AgentDisplayName()` | **高（用户直接可见）** |
| F7.2 | `bootstrapConfig` 首跑模板写的是**已删除**的 `claudecode`+`feishu` → 全新用户拿到永远起不来的配置。改为写内嵌的 `config.example.toml`（单一真源） | **高** |
| F7.3 | `[projects.references]` 允许列表只剩已删除适配器 → 功能**永久失效**，且填对的值会**启动报错** | 中 |
| F7.4 | core 里残留硬编码 telegram 分支（`AGENTS.md` 明令禁止） | 低 |
| F7.5 | 打包前审出：`README.md`（zip 门面）有 `default（逐次确认）` 等 6 处问题 + 6 个死链；`install.ps1` 会把 `REG_EXPAND_SZ` 的 PATH 降级成 `REG_SZ` | 中 |

---

## 4. 关键结论（**别重新推翻**）

| 结论 | 依据 |
|---|---|
| **`allow_from` 是平台级**，必须写在 `[projects.platforms.options]`；项目级会被 TOML 解码器**静默丢弃** | `config.ProjectConfig` 无此字段；`platform/dingtalk/dingtalk.go:101` 读 `opts["allow_from"]`。实测对照过 |
| **`admin_from` 是项目级**（`[[projects]]` 下） | `config/config.go:512` `toml:"admin_from"` |
| **cf-connect 没有交互式权限确认**，`mode` 只决定是否追加跳过权限的 flag | `agent/opencode/session.go:517` `RespondPermission` 是 no-op；实测写操作无提示直接执行 |
| **钉钉唯一的流式通道是 AI 卡片**，开关只有 `card_template_id`；`card_mode` 是飞书 Card 2.0 遗留，`[stream_preview]` 需要平台实现 `MessageUpdater`（钉钉没有） | `core/engine.go:4990` 无条件尝试；`core/progress_compact.go:311` 打 `unsupported style` |
| **用户可见文字用 `core.AgentDisplayName()`，注册/配置/预设/会话归属用 `Agent.Name()`** | `core/registry.go`；`Name()` 必须保持 `"opencode"`（`CreateAgent` 和会话归属靠它） |
| **`cmd` 的 basename 决定品牌**（含 `codefree`/`code-free`/`code_free` → codefree-o，读 `~/.codefree-o/.local/share/codefree.db`；否则 opencode） | `agent/opencode/sessiondb.go:117` |
| **首跑模板 = 内嵌的 `config.example.toml`**（不再手抄），回归测试用**注册表**断言，任何裁剪忘了改模板都会失败 | `cmd/cf-connect/main.go` `bootstrapConfig` |
| **`[projects.references]` 允许列表 = `opencode` / `dingtalk`** | `config/config.go` + `core/reference_render.go`，两处必须同步 |
| **切换工作目录用 `/dir`（单项目）或 `/workspace`（`mode="multi-workspace"` + `base_dir`）**，改动持久化在项目状态里，**不用改配置**；`/dir` 是特权命令，需要 `admin_from` | `core/engine.go:8481`（cmdDir）/`:8365`（dirApply）/`:1211`（privilegedCommands）；`cmd/cf-connect/main.go:421-432`（多工作区接线）。**真机未实测** |
| `CC_*` 环境变量与 `cc_connect_*` 内部标识**保持不改名** | 对外 hook 契约 + bridge 协议/缓存键 |
| `npm/` 不动 | 推迟到 npm 账号/scope 就绪 |

---

## 5. 测试现状（**务必先读懂这段，否则会白追失败**）

### 稳定通过

```
.\scripts\go-dev.cmd build ./...            → exit 0
.\scripts\go-dev.cmd vet ./...              → exit 0
python scripts\check_brand.py               → 0 违规
python scripts\check_doc_links.py           → 0 死链
go test ./platform/dingtalk/ . ./tests/release_local/...  → ok
```

### Windows 上 `go test ./...` **不是全绿**，且基本是基线既有问题

本轮最后一次全量结果（与基线逐条比对过，**无新增失败**）：

| 包 | 例数 | 根因 |
|---|---|---|
| `agent/opencode` | 22 | 模型发现依赖真实 CLI 行为（基线） |
| `cmd/cf-connect` | 1 | `TestParseDaemonInstallArgs_WorkDirOverridesConfig`，路径分隔符（基线） |
| `config` | 2 | `TestLoad_DefaultsDataDir`（HOME 未隔离）、`TestLoad_ResolvesEnvPlaceholders`（分隔符）（基线） |
| `core` | 4 | `TestCompactReplyFooterPath_HomeRelativeDeepPathStaysFull`、`TestAppendImageRefs`、`TestAppendFileRefs_AbsolutizesRelativePaths`、`TestAppendFileRefs_AbsoluteInputsPassthrough`（均 Windows 分隔符，基线） |
| `daemon` | 2 | `TestSchtasksInstall_TightensExistingScriptFrom0644`（基线，POSIX 权限语义）+ **`TestMetaSaveLoad`（沙箱导致）** |

- **`daemon.TestMetaSaveLoad` 是沙箱造成的**：它直接写真实 `~/.cf-connect/daemon.json`，
  没有隔离 HOME。工作区外不可写时必然 `Access is denied`，**不是代码缺陷**。
- **flaky**：`core` 的 `TestCUJ_A3_ImageReachesAgent` / `A5_FileReachesAgent` 偶发失败，
  报的是 `TempDir RemoveAll cleanup: ... directory is not empty`，**不是断言失败**。
  单独重跑即过。看到别当回归。
- 完整基线清单：`docs/BASELINE-TESTS.md`。

---

## 6. 钉钉联调资源（本轮用过，可复用）

| 项 | 值 |
|---|---|
| 应用 | 企业内部应用，`client_id = dingomm29pzoocbgqcvl`，机器人消息接收模式 = **Stream** |
| Client Secret | **不写在这里**。在 `tmp\dingtalk-smoke\config.toml`（gitignore 覆盖）。⚠️ 见 §10 |
| 我的钉钉 userid | `01140255060421424301` |
| 已验证的 AI 卡片模板 | `6b172a70-7dd1-44c1-a02d-654c664715f3.schema`，模板内 Markdown 组件的变量名必须是 `content` |
| 联调工作目录 | `tmp\dingtalk-smoke\`（含 exe 副本、config.toml、work/） |

**凭证预检**（比跑起来看日志快得多）：

```powershell
node -e "fetch('https://api.dingtalk.com/v1.0/oauth2/accessToken',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({appKey:process.argv[1],appSecret:process.argv[2]})}).then(async r=>console.log(r.status, await r.text()))" dingomm29pzoocbgqcvl <client_secret>
```

成功 `200 {"expireIn":7200,...}`；失败 `400 {"code":"invalidClientIdOrSecret"}`。
**教训**：secret 是 64 字符，肉眼核对首尾各 10 位（本轮第一次就错在末位 C/Z）。

**未覆盖**（要做就得自己造场景）：群聊、附件（图片/文件）、`/stop` 打断、
卡片降级熔断（403/429/5xx）、daemon 模式、cron/timer 端到端、
卡片内的思考/工具步骤渲染（本轮模型一次吐完，没构造多步长思考场景）。

---

## 7. 下一步可选任务

### A. 发布收尾（最直接）

```powershell
git tag v1.0.0 && git push origin trim-dingtalk-codefree-o --tags
# 在 ItsQifan/cf-connect 的 Release 上传 dist/*.zip + checksums.txt
```

⚠️ README 里的文档链接已改成**绝对地址指向 `main`**（为了让 zip 用户也能点开）。
合并回 main 之前，这些链接指向的是**旧内容**（main 上的 INSTALL.md 还带 doctor 那段）。
合并后自动正确。

### B. 补钉钉未覆盖场景（见 §6 列表）

优先建议：**群聊**（`share_session_in_channel`、群内 @、卡片 `IM_GROUP` 投递）与
**附件**——这两块是用户最可能踩到的。

### C. 核实两条存疑项

1. `QUICKSTART.md` §6 说"会话状态/附件会写在 `<work_dir>\.cf-connect\`，记得 gitignore"。
   **实测未发现该目录**（我的 work_dir 里只有 agent 建的 `hello.txt`，
   会话状态实际在 `~/.cf-connect\`）。**很可能是错的，请核实后修掉。**
2. `install.ps1` 的 `REG_EXPAND_SZ` 修复**没有真机验证**（注册表写入被沙箱拒绝）。
   要验证需在一台 User PATH 含 `%VAR%` 的机器上跑 `install.ps1` 再 `-Uninstall`，
   确认类型仍是 `ExpandString`。

### D. npm 发布（等账号/scope）

改 `npm/` 包名为 `@itsqifan/cf-connect` + 资产逻辑 + `npm pack` + `npm publish --access public`。
里面仍有 30 处 `cc-connect` 字面量。

### E. 可选优化

- 修 `tests/integration/engine_platform_test.go` 的上游既有编译错误
  （`sess.Send` 参数个数不符），仅在 `-tags integration` 下可见。
- 给 flaky 的 CUJ 用例加 `t.Cleanup` 等待，消除 TempDir 竞态。
- F6：`agent_id` 死配置，要么删要么接上。
- 把 `make release-all` 也做成 Windows 可用（目前是 POSIX shell 循环）。

---

## 8. 重新打包（改了代码之后）

```powershell
# 1) 先建 web（web/dist 会被 go:embed 进二进制）。沙箱下需要提权一次（esbuild spawn EPERM）
cd web; corepack pnpm build; cd ..

# 2) 设置环境（见 §2），然后打包
& .\scripts\release-windows.ps1 -Version v1.0.0
#   不给 -Version 就读 Makefile 的 VERSION
```

产出 `dist\cf-connect-<version>-windows-amd64.zip` + `dist\checksums.txt`。
**改版本号只需改 `Makefile` 的 `VERSION` 一处**。

> `pwsh` 在这台机器上**不在 PATH**（只有 Windows PowerShell），用 `& .\scripts\...ps1` 调用。
> `make release-all` 是类 Unix 用的。

打包后自检见 `RELEASE.md` §3（**该文件被 gitignore，是本地 runbook**）。

---

## 9. 关键文件速查

| 文件 | 作用 |
|---|---|
| `docs/DINGTALK-E2E.md` | **钉钉联调验收报告**（逐条验收 + 7 个发现 + 复现 + 修复记录） |
| `docs/BASELINE-TESTS.md` | 裁剪前的基线失败清单（"无新增失败"的依据） |
| `HANDOFF.md` | 本文 |
| `RELEASE.md` | 发行/构建/打包自检/分发（**gitignored，本地**） |
| `QUICKSTART.md` | 3 步上手 + 排障（随 zip 分发，**离线自足、无相对链接**） |
| `config.example.toml` | 完整配置模板（内嵌进二进制，`cf-connect config example` 输出它） |
| `core/registry.go` | `AgentDisplayName()`——用户可见名字的解析 |
| `agent/opencode/sessiondb.go` | 品牌感知的 DB 路径解析（补丁 2 + 进程内 sqlite） |
| `agent/opencode/opencode.go` | `permission_flag` 解析、`GlobalMemoryFile`、别名、`AgentDoctorInfo` |
| `agent/opencode/session.go` | `buildRunArgs`（yolo flag 注入点）、`RespondPermission`（no-op） |
| `platform/dingtalk/card.go` | AI 卡片（createAndDeliver + streaming） |
| `scripts/release-windows.ps1` / `go-dev.cmd` / `check_brand.py` / `check_doc_links.py` | 工具 |
| `tmp/dingtalk-smoke/config.toml` | 联调用的**可用配置**（含真实 secret），可直接抄 |

---

## 10. 安全注意

1. **`tmp\dingtalk-smoke\config.toml` 里有真实的 Client Secret。**
   `tmp/` 被 gitignore 覆盖，所以不会进库；但**别打包整个仓库发人**。
   新会话若要重跑联调，直接抄这份配置（改 `work_dir` 即可）。
2. 本轮已确认**全仓库（除 tmp/）不含明文 secret**（grep 过）。
3. codefree-o 的全局配置 `~/.codefree-o/.config/codefree.json` 里挂了一个指向**内网 MySQL**
   的 MCP（明文账号口令）。这意味着**谁能对机器人说话，谁就能驱动一个能访问那个库的 agent**。
   `allow_from` 必须设——这正是 F1 被定为高优先级的原因。

---

## 11. 踩过的坑（省得重踩）

1. **别把 `.go` 文件放进 `tmp/`** —— 它会被 `go test ./...` 当成一个包去编译，
   报 `FAIL .../tmp [build failed]`。我用 `.bak` 后缀备份就没这问题。
2. **`.cmd` 包装脚本里不能出现 `|`** —— 会被 cmd.exe 当管道。
   传正则给 `go test -run` 时避开（用单个测试名或公共子串）。本轮又踩了一次。
3. **仓库是 CRLF 检出** —— `gofmt -l` 会把 **152 个文件**（含我没碰过的）全列出来。
   要判断真实格式问题，先把内容规范化成 LF 再跑 gofmt。**别直接 `gofmt -w`**。
4. **`check_doc_links.py` 会扫描仓库内解压出来的 zip 目录**，把包内文档的链接报成死链。
   跑之前先清掉解压目录。
5. **PowerShell 里 `"..." > file` 是 UTF-16** —— 会把 TOML/JSON 写成 UTF-16，
   解析器报 "files cannot contain NULL bytes"。用
   `[System.IO.File]::WriteAllText($p, $t, (New-Object System.Text.UTF8Encoding($false)))`。
6. **`Set-Content -Encoding utf8` 会加 BOM** —— 提交信息开头带 BOM 会污染 `git log`；
   写提交信息用上面的 UTF8Encoding($false)，再 `git commit -F`。
7. **控制台显示中文会乱码** —— PowerShell 输出 CJK 显示成乱码，但文件本身是好的。
   判断 CJK 用正则 `[\u4e00-\u9fff]`，别用肉眼。
8. **codefree-o 未登录时会以 OAuth 提示阻塞并超时**（约 190s），不是适配器 bug；
   `TestRealCLI` 已识别并 skip。
9. **codefree-o 在进程结束后仍短暂持有 work_dir 句柄** —— Windows 上
   `t.TempDir()` 清理会报 sharing violation；集成测试用 `newWorkDir()` 重试规避。
10. **别在 `core/` 里写平台/agent 名** —— `AGENTS.md` 有硬性约定。
    本轮又清掉一处残留（F7.4）；新写代码时注意，`Agent.Name()` 也不该直接进用户可见文案。

---

## 12. 一分钟上手（给新会话的最短路径）

```powershell
cd E:\my_idea_workspace\cf-connect
git log --oneline -3                        # 确认在 72c2be8（或其后仅多一个 docs: handoff 提交）
$env:GOCACHE="E:\my_idea_workspace\cf-connect\tmp\gocache"   # 沙箱下必须
$env:GOTMPDIR="E:\my_idea_workspace\cf-connect\tmp\gotmp"
.\scripts\go-dev.cmd build ./...            # 应 exit 0
.\scripts\go-dev.cmd test ./core/ -run TestCUJ   # 应 ok（偶发 flaky，重跑即可）
python scripts\check_brand.py               # 应 0 违规
Get-ChildItem dist                          # 应有 v1.0.0 的 zip + checksums.txt
```

然后读 **`docs/DINGTALK-E2E.md`**（联调全貌与 7 个缺陷）→ 决定做 §7 里的哪一项。
