# 钉钉真机联调报告（HANDOFF §9 A）

> 本文所有结论均为**本轮实测**，来源是终端输出与钉钉客户端现象。
> 未验证的项目在 §5 明确列出，不做推断。

- **日期**：2026-09-12
- **仓库**：`E:\my_idea_workspace\cf-connect`
- **分支 / HEAD**：`trim-dingtalk-codefree-o` / `774a2f6`（联调开始时的 HEAD）
- **二进制**：本轮从源码构建，`go build -o dist\cf-connect.exe ./cmd/cf-connect` → exit 0
- **运行目录**：`tmp\dingtalk-smoke\`（在 `.gitignore` 内，含真实 `client_secret`，勿提交）
- **钉钉应用**：`client_id = dingomm29pzoocbgqcvl`（企业内部应用，机器人消息接收模式 = Stream）
- **本地 agent**：codefree-o 1.7.0（`C:\nvm4w\nodejs\codefree-o.cmd`，已登录 userId 311476）

**结论**：核心链路**全部打通**——Stream 收发、真实 AI 回复、多轮上下文、
AI 卡片流式、会话标题/消息数、白名单拦截、数据目录生成。
同时发现 **1 个高优先级配置缺陷**和 **4 个文档/日志缺陷**（§4）。

**修复状态**：F1 / F2 / F3 / F4 / F5 已在本分支修复（§7），F6 仅记录未动。

---

## 1. 联调前的凭证自检（建议固化成流程）

不要等程序跑起来看日志判断凭证对不对，直接探一次取 token 接口最省时间：

```powershell
node -e "fetch('https://api.dingtalk.com/v1.0/oauth2/accessToken',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({appKey:process.argv[1],appSecret:process.argv[2]})}).then(async r=>console.log(r.status, await r.text()))" dingomm29pzoocbgqcvl <client_secret>
```

- 成功：`200 {"expireIn":7200,"accessToken":"..."}`
- 失败：`400 {"code":"invalidClientIdOrSecret", ...}`

本轮第一次探测返回 400，原因是 client_secret 末位字符与实际不符
（控制台掩码显示 `0qec7Y6cw3****1p5c5Nd88Z`，拿到的是 `...Nd88C`）。
**教训**：secret 是 64 字符串，肉眼核对首尾各 10 位，别无脑复制。

网络：本机系统代理指向 `127.0.0.1:7897`，但 `api.dingtalk.com` **直连可用**，
cf-connect 无需设置 `HTTPS_PROXY` 即可收发（实测全程未设代理）。

---

## 2. 验收清单逐条结果

| # | 项目 | 结果 | 证据 |
|---|---|---|---|
| 1 | 消息收发（入站） | ✅ | 日志 `dingtalk: message received` + `message received`，用户可见回复 |
| 2 | 真实 AI 回复 | ✅ | `turn complete ... response_len=148 turn_duration=20.8s` |
| 3 | 多轮上下文继承 | ✅ | 第二问 `resume=true --session ses_f6bcdd0aff...`，模型正确复述第一问 |
| 4 | 工具调用 | ✅ | `tools=1`；工作目录里真实生成 `hello.txt`（2 字节 `hi`） |
| 5 | AI 卡片流式 | ✅ | `createAndDeliver` 200 且 `deliverResults[0].success=true`；`/v1.0/card/streaming` 200 ×2（更新 + finalize）；用户可见逐步刷新 |
| 6 | 会话列表标题/消息数（补丁 2） | ✅ | 钉钉 `/list` 显示标题与消息数；另跑 `TestRealCodefreeDB_ReadsTitlesAndMessageCounts` → PASS（27 个会话） |
| 7 | 白名单 `allow_from` 拦截 | ✅ | 换成假 ID 后用户收到拒绝，日志 `dingtalk: message from unauthorized user` |
| 8 | 表情反馈 | ✅ | 默认 `reaction_emoji = "🤔Thinking"`，用户确认消息上出现表情 |
| 9 | `~/.cf-connect/` 生成 | ✅ | 见 §3 |
| 10 | **权限确认（`mode = "default"`）** | ❌ **功能不存在** | 见 §4 F2 |

---

## 3. `~/.cf-connect/` 实际生成内容

```
C:\Users\qifan\.cf-connect\
├── crons\                                                  （cron 调度器）
├── timers\                                                 （timer 调度器）
├── run\api.sock                                            （内部 API，供 cf-connect send 用）
├── sessions\dingtalk-smoke_7bc1bcbf.json                   （会话状态，1.2 KB）
├── projects\dingtalk-smoke-ec4c82d3aa4a3063.opencode-models.json
├── daemon.json
└── dir_history.json
```

`sessions\dingtalk-smoke_7bc1bcbf.json` 内容与钉钉侧行为一致：
`s1` 记录了 `past_agent_session_ids`，`s2/s3` 为 `/new` 新建，
`user_meta` 记录了发送者昵称 `周传祥`，`active_session` 指向当前会话。

---

## 4. 发现的问题

### F1（高，安全）`config.example.toml` 把 `allow_from` 放在错误的层级 → 静默失效

**现象**：按 `config.example.toml` 的写法把 `allow_from` 写在 `[[projects]]` 下，
程序**不报错、不告警**，但白名单完全不生效——所有人可用。

**根因**：
- `config.ProjectConfig` 根本没有 `allow_from` 字段（只有 `AdminFrom string \`toml:"admin_from"\``，`config/config.go:512`），
  TOML 解码时未知键被静默丢弃。
- `allow_from` 是**平台级**选项：`platform/dingtalk/dingtalk.go:101` 读的是 `opts["allow_from"]`，
  即必须写在 `[projects.platforms.options]` 下。

**实测对照**（同一份配置，只改位置）：

| 位置 | 启动日志 | 实际效果 |
|---|---|---|
| `[[projects]]` | `level=WARN msg="allow_from is not set — all users are permitted"` | 假 ID 也放行 |
| `[projects.platforms.options]` | 无告警 | 假 ID 被拒：`dingtalk: message from unauthorized user` |

**影响面**：`config.example.toml` 是随发行 zip 分发、并内嵌进二进制的模板；
`QUICKSTART.md` §2 第 6 步也照此引导。用户以为锁了门，其实没有。
考虑到 codefree-o 带 `~/.codefree-o/.config/codefree.json` 里配置的 MCP（本机实测含一个指向内网
MySQL 的 `mysql_scchain_db`，明文账号口令），这个"以为锁了其实没锁"的组合风险是真实的。

**建议修法**：把 `allow_from` 移到 `[projects.platforms.options]`，并在
`QUICKSTART.md` / `docs/dingtalk.md` 里点明层级；另加一条
`TestConfigExampleTOML_DingTalkAllowFromIsPlatformLevel` 回归测试。
**额外加固**：仅改文档不够——手改配置的用户仍然会踩。
已在 `config.load()` 里加了启动期检测，命中时打 WARN（实测输出）：

```
WARN config: project "dingtalk-smoke" sets allow_from at the [[projects]] level,
where it is silently ignored; it is a per-platform option —
move it under [projects.platforms.options] or the bot will accept every user
```

### F2（高，文档与产品能力不符）`mode = "default"` 并不拦截工具调用，"权限确认"不存在

**文档怎么说的**：
- `config.example.toml`：`"default" - the agent asks before risky operations (recommended to start)`
- `QUICKSTART.md`：`mode = "default"  # 先用 default，确认没问题再考虑 yolo`
- `QUICKSTART.md` 排障表：`一直卡在"等待权限确认" → 正常行为。要么在钉钉里点确认，要么改成 yolo`
- `HANDOFF.md` §9 A：把"权限确认（`mode = "default"`）"列为待验证项

**实际是什么**：
- `agent/opencode/session.go:516`：`RespondPermission` 是 **no-op**（注释：`OpenCode handles permissions internally`）。
- 适配器从不向引擎发"权限请求"事件；唯一的权限相关路径是**事后**把 CLI 的拒绝原因当文本回推
  （`session.go:362-378`，由 `TestHandleToolUsePermissionDeniedEmitsEventText` 覆盖）。
- 钉钉侧因此**没有任何"允许/拒绝"按钮**，用户根本无法"点确认"。

**实测**：`mode = "default"` 下，一条"创建 hello.txt"的写操作**没有弹任何确认**，
直接在工作目录生成了文件（`work\hello.txt`，2 字节）。也就是说 `default` 与 `yolo`
在无头（`run --format json`）场景下行为一致，`default` 不提供文档承诺的保护。

**建议修法**：改文档，别改承诺——把 `default` 描述为"不追加跳过权限的 flag，
权限由 CLI 自身配置决定"，删掉"点确认"的排障条目，并说明
"要做工具级管控请用 CLI 自己的 `permission` 配置 / 或容器隔离"。
（若确实要做交互式权限，那是新功能，不是本轮范围。）

### F3（中，文档）`cf-connect doctor` 在 Windows 上不做任何事

`QUICKSTART.md` §4 让 Windows 用户 `.\cf-connect.exe doctor` 做自检；
实际输出：

```
doctor command is not supported on Windows
```

`cmd/cf-connect/doctor_runas_windows.go:8` 就是这么写的，而且即便在 Unix 上，
`doctor` 目前也只是 `doctor user-isolation`（`doctor_runas.go:22`），
并不检查 agent CLI / 数据目录 / 配置合法性。

**建议修法**：QUICKSTART 删掉该步骤，或改成指向真实存在的检查手段
（`--version`、日志、`/whoami`）。

### F4（中，行为）未配 `card_template_id` 时每轮都打 WARN

```
level=WARN msg="streaming card creation failed, falling back to normal messages" error="dingtalk: card_template_id not configured"
```

不配卡片模板是**官方支持的默认用法**（`config.example.toml` 明确写了"不配也能用"），
却每轮告警一次，会把用户引向"我是不是配错了"。建议在
`platform/dingtalk/dingtalk.go:1085` 处把这种"未配置"情形降级为 Debug，
只在真实 API 失败时才 WARN。

### F5（低，配置）`display.card_mode` 对钉钉无效，注释却说是钉钉 AI 卡片开关

`config.example.toml`：`# card_mode = "legacy"  # legacy | rich  (rich = DingTalk AI Card streaming)`

实际 `card_mode` 是上游飞书 Card 2.0 的遗留开关
（`config/config.go:196` 注释 `"rich" (Card 2.0 Feishu)`；`core/engine.go:5167` 只用于
`RichCardSupporter`）。钉钉的 AI 卡片走的是另一条路——实现
`core.StreamingCardPlatform`，唯一开关是 `card_template_id`，与 `card_mode` 无关
（`core/engine.go:4989` 无条件尝试创建）。

同理 `[stream_preview]` 对钉钉也无效：钉钉没有实现 `MessageUpdater`，
`core/progress_compact.go:310-318` 会直接 `progress writer disabled: unsupported style`。
`config.example.toml` 里"Disabled by default because DingTalk AI Cards already stream natively"
的说法反了因果——不是默认关，是这条路对钉钉不存在。

**建议修法**：钉钉专属的模板里删掉这两个块，或把注释改成"仅飞书 / 对钉钉无效"。

### F6（低，死配置）`agent_id` 解析了但从没被使用

`platform/dingtalk/dingtalk.go:131-138` 解析 `agent_id`、存进结构体字段（第 160 行、第 76 行），
全文件再无任何读取点。配置里写它没有任何效果，也没有文档说明。

---

## 5. 未覆盖 / 不在本轮范围

1. **群聊**（`share_session_in_channel`、群内 @ 机器人、卡片 `IM_GROUP` 投递）——本轮只测了单聊。
2. **附件**（图片 / 文件上行下发、`max_attachment_size_mb`）。
3. **`/stop` 打断、`/model` 切换、cron / timer 定时任务**的端到端行为。
4. **卡片内的思考/工具步骤渲染**——本轮卡片只出现了最终答案，
   因为模型一次吐完，未构造"多步 + 长思考"的场景去验证 `thinking_messages` /
   `tool_messages` 在卡片里的呈现。
5. **卡片降级路径**（`activateCardDegrade` 30 分钟熔断）——未构造 403/429/5xx。
6. **多用户 / 多项目并发**。
7. **daemon 模式**（`daemon install` + schtasks）——本轮全程前台运行。

---

## 6. 复现步骤

```powershell
cd E:\my_idea_workspace\cf-connect
.\scripts\go-dev.cmd build -o dist\cf-connect.exe ./cmd/cf-connect

$smoke = "E:\my_idea_workspace\cf-connect\tmp\dingtalk-smoke"
New-Item -ItemType Directory -Force -Path "$smoke\work" | Out-Null
Copy-Item dist\cf-connect.exe $smoke -Force
# 在 $smoke\config.toml 里填 client_id / client_secret
# （allow_from 必须写在 [projects.platforms.options] 下，见 F1）

cd $smoke
.\cf-connect.exe --config .\config.toml      # 前台跑，看日志
```

钉钉侧依次发：`/whoami` → `用一句话介绍你自己` → `我上个问题问的什么？` →
`列出当前工作目录下的所有文件` → `/new` → `/list`。

AI 卡片需要额外两步（本轮已做）：
1. 钉钉开放平台 → 卡片平台 → 新建 **AI 卡片**模板，模板里放一个 Markdown 组件并绑定变量 `content`；
2. 把模板 ID（形如 `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.schema`）填进
   `card_template_id`，重启即可。本轮实测模板：`6b172a70-7dd1-44c1-a02d-654c664715f3.schema`。

> 跑测试时注意：本仓库的 `go test ./...` 需要 `GOCACHE` 可写。
> 若 `%LOCALAPPDATA%\go-build` 不可写，可临时指定
> `GOCACHE=<repo>\tmp\gocache`（`tmp/` 已被 gitignore）。
> **不要把 `.go` 文件放进 `tmp/`**——它会被 `./...` 当成一个包去编译。

---

## 7. 修复内容（本轮）

| 编号 | 改动 | 文件 |
|---|---|---|
| F1 | `allow_from` 挪到 `[projects.platforms.options]`，并加"必须写在这一层"的显式警告；启动时检测到项目级 `allow_from` 会打 WARN，不再静默丢弃 | `config.example.toml`、`QUICKSTART.md`、`config/config.go`（新增 `misplacedKeyWarning` + `warnMisplacedKeys`） |
| F2 | 改写 `mode = "default"` 的说明：它只决定追加哪个 flag，不产生钉钉里的允许/拒绝按钮；删掉"在钉钉里点确认"的排障条目 | `config.example.toml`、`QUICKSTART.md` |
| F3 | QUICKSTART 删掉 `doctor` 步骤，改成"看日志" | `QUICKSTART.md` |
| F4 | 新增 `core.ErrStreamingCardUnavailable` 哨兵错误；钉钉未配 `card_template_id` 时包装它，引擎据此降为 Debug，真实 API 失败仍 WARN | `core/interfaces.go`、`core/engine.go`（2 处）、`platform/dingtalk/dingtalk.go` |
| F5 | 从模板里删除对钉钉无效的 `card_mode` / `[stream_preview]`，改为说明性注释 | `config.example.toml` |
| F6 | **未修**，仅记录：`agent_id` 是死配置 | — |
| F7 | `/list` `/status` `/skills` 标题显示注册名 `opencode` 而非品牌 `CodeFree-O`；`bootstrapConfig` 首跑模板写的是已删除的 `claudecode`+`feishu`；`[projects.references]` 允许列表只剩已删除适配器导致功能永久失效；core 里残留硬编码 telegram 分支 | 见 §8 |

新增回归测试（均已验证"修复前 FAIL、修复后 PASS"）：

| 测试 | 文件 |
|---|---|
| `TestConfigExampleTOML_AllowFromIsPlatformLevel` | `embed_test.go` |
| `TestConfigExampleTOML_DoesNotAdvertiseDeadDingTalkSwitches` | `embed_test.go` |
| `TestMisplacedKeyWarning_DetectsProjectLevelAllowFrom` | `config/config_test.go` |
| `TestMisplacedKeyWarning_DroppedByDecoder` | `config/config_test.go` |
| `TestCreateStreamingCard_UnconfiguredWrapsSentinel` | `platform/dingtalk/dingtalk_test.go` |
| `TestCreateStreamingCard_ConfiguredIsNotUnavailable` | `platform/dingtalk/dingtalk_test.go` |
| `TestAgentDisplayName_PrefersCLIDisplayName` | `core/agent_display_name_test.go` |
| `TestCmdList_TitleUsesAgentDisplayName` | `core/agent_display_name_test.go` |
| `TestCmdList_PagedTitleUsesAgentDisplayName` | `core/agent_display_name_test.go` |
| `TestRenderListCard_TitleUsesAgentDisplayName` | `core/agent_display_name_test.go` |
| `TestCmdSkills_TitleUsesAgentDisplayName` | `core/agent_display_name_test.go` |
| `TestNormalizeProgressAgentLabel_CodefreeBrand` | `core/agent_display_name_test.go` |
| `TestReferenceRender_ScopesCoverTheForkAdapters` | `core/reference_render_test.go` |
| `TestBootstrapConfig_OnlyNamesCompiledInAdapters` | `cmd/cf-connect/main_test.go` |

测试结果：`platform/dingtalk` / 根包 / `tests/release_local/*` 全绿；
`config` 只剩基线那 2 例；`core` 只剩基线 4 例（`TestCUJ_A3/A5` 为已知 flaky）；
`agent/opencode` 22 例、`cmd/cf-connect` 1 例与基线一致（见 `docs/BASELINE-TESTS.md`）。

---

## 8. F7：用户可见文字里的"写死"（第二轮）

联调后用户实测反馈：`/list` 回的是 **`**opencode 会话列表** (N)`**，应该是 CodeFree-O。
顺着这条线排查，发现同一根因（裁剪时漏改的硬编码）还有 4 处。

### F7.1 标题用 `Agent.Name()`（注册名）而不是品牌名 —— 用户报的那个

- `Agent.Name()` 是**注册键**，用于 `CreateAgent()` 和会话归属标记，必须保持 `"opencode"`
  稳定；但它驱动的是 codefree-o，拿来当展示名就错了。
- 受影响：`/list`（文本 + 分页 + 卡片，4 处）、`/status`（文本 + 卡片）、`/start` 欢迎语、
  `/skills`（2 处）、进度卡片里的 agent 标签。

**修法**：core 新增 `AgentDisplayName(agent)`，优先取已有的
`AgentDoctorInfo.CLIDisplayName()`（适配器早已按品牌返回 `CodeFree-O`/`OpenCode`，
`cf-connect doctor` 一直在用），取不到再回退 `Agent.Name()`。所有用户可见处改用它；
注册、配置键、provider 预设查找、会话归属一律继续用 `Agent.Name()`。
`normalizeProgressAgentLabel` 补上 `codefree-o`/`codefree`/`codefreeo` → `CodeFree-O`。

实测修复前后（同一个测试的输出）：

```
修复前： **opencode 会话列表** (1)
修复后： **CodeFree-O 会话列表** (1)      # 钉钉真机已确认
```

### F7.2 `bootstrapConfig` 首跑模板写的是**已删除**的适配器（后果最严重）

`cf-connect` 在 `config.toml` 不存在时会自动写一份模板。这份模板是**手抄**的，
裁剪时没人更新，仍然是：

```toml
type = "claudecode"     # 本分支已删除，注册表里只有 codefree-o / opencode
type = "feishu"         # 本分支已删除，注册表里只有 dingtalk
```

也就是说：一个**全新用户**、没有 config.toml、直接运行 `cf-connect`，
拿到的是永远起不来的配置（`unknown agent "claudecode"`）。
QUICKSTART 让人 `copy config.example.toml` 所以绕过了它，但这条路径是坏的。

**修法**：改为直接写内嵌的 `config.example.toml`（`embed.go` 已 go:embed），
从此**只有一个真源**，不会再漂移。
回归测试 `TestBootstrapConfig_OnlyNamesCompiledInAdapters` 用**注册表**（编译期真相）
而不是硬编码列表来断言，所以以后任何一次裁剪忘了改模板都会在这里失败。
修复前实测报错：

```
bootstrap selects agent type "claudecode", which is not compiled in; registered: [codefree-o opencode]
bootstrap selects platform type "feishu", which is not compiled in; registered: [dingtalk]
```

同一处还有一条过期提示：`cf-connect init` 这个命令**根本不存在**，
已改为真实可用的 `cf-connect config example > config.toml`。

### F7.3 `[projects.references]` 被裁剪搞成**永久失效**（且配置直接报错）

`docs/usage.zh-CN.md` 记录了本地引用重渲染功能，并推荐
`normalize_agents = ["all"]` / `render_platforms = ["all"]`。
但两处允许列表都只剩上游名字：

| 位置 | 原值 | 后果 |
|---|---|---|
| `config/config.go` `supportedReferenceAgents/Platforms` | `codex`/`claudecode`、`feishu`/`weixin` | 用户填 `"opencode"` 或 `"dingtalk"` 会**启动报错** `has unsupported value` |
| `core/reference_render.go` `supportedReferenceNormalizeAgents/RenderPlatforms` | 同上 | `"all"` 展开成 `[codex,claudecode]`/`[feishu,weixin]`，而运行时 agent 是 `opencode`、平台是 `dingtalk`，**永远匹配不上** |

两头堵死：写对了报错，写 `"all"` 静默无效。

**修法**：两处都改为 `opencode` / `dingtalk`，文档同步；
`core/reference_render_test.go` 里 30 多处测试数据一并更新（它们原本也用上游名字）。
回归测试 `TestReferenceRender_ScopesCoverTheForkAdapters` 断言文档推荐的
`["all"]/["all"]` 确实能对 `opencode+dingtalk` 生效——
修复前实测：`validate() unexpected error: ... normalize_agents has unsupported value "opencode"`。

### F7.4 core 里残留硬编码 telegram 分支（`AGENTS.md` 明令禁止）

`displayCommandForPlatform()` 里写着 `if !strings.EqualFold(platformName, "telegram")`，
而 Telegram 早已删除；`/skills` 的文本路径是它唯一的调用点，对钉钉恒为直通。

`HANDOFF.md` 声称"已清掉 telegram/slack 硬编码分支"——这条漏了
（隔壁 `TestMenuCommandsForPlatform_PassesThroughWithoutMenuLimit` 的注释还专门写了
"with Telegram gone…"，说明是同一批裁剪的遗漏）。

**修法**：删掉 `displayCommandForPlatform` / `sanitizeTelegramDisplayCommand`
及其对应的过期测试 `TestCmdSkills_UsesTelegramSafeNamesOnTelegramPlatform`
（该测试断言的正是已删平台的行为）。

### F7.5 未改但已记录

- `core/progress_compact.go` 的标签映射仍留着 `codex`/`claudecode`/`gemini`/`cursor`/
  `qoder`/`iflow`/`pi` 等已删品牌的 case。它们是**无害的兜底映射**（本分支不可能命中），
  删除纯属清理，未动。
- `core/i18n.go` 的 `MsgReasoningDefault` 文案含 "using Codex default"。
  opencode 适配器未实现 `ReasoningEffortSwitcher`，`/reasoning` 会回
  `MsgReasoningNotSupported`，所以该文案**当前不可达**，未改。
- `cmd/cf-connect/provider.go` 与 `core/management.go` 里的 `"claude"`/`"codex"`
  是 **cc-switch 数据库的 `app_type` 字段值**（外部 schema），不是 cf-connect 的适配器名，
  属于正确用法，未改。
