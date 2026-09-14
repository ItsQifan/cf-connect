---
name: cf-connect
description: 把 CodeFree 接到钉钉上的桥接工具 cf-connect 的使用与运维。当用户说"装 cf-connect / 配置钉钉机器人 / 我在钉钉里跟你说话 / 手机钉钉派活 / 测一下连通性 / 卸载 cf-connect / 换个工作目录 / 为什么钉钉没反应"时使用。负责：读懂 release 目录 → 落 PATH → 生成并填写 config.toml（缺钉钉凭证时引导用户去开放平台获取）→ 起服务 → 端到端验证 → 卸载/升级/排障。不负责：帮用户注册钉钉应用本体、替用户保管或外传 AppSecret。
---

# cf-connect（用户机器上的运维手册）

## 0. 先明白它是什么、不是什么

```
用户在钉钉里发消息 ──Stream 长连接──▶ cf-connect.exe ──子进程 + NDJSON──▶ codefree-o
        ▲                                    │
        └──────── 流式卡片 / 结果回推 ────────┘
```

- **是**：一个本机网关。它把钉钉消息变成对**本机 codefree-o** 的一次调用，再把结果推回钉钉。
- **不是**：不是模型、不是 MCP server、不提供算力。模型额度走用户自己的 codefree-o 登录。
- **不需要**：Go / Node / Python / Java / 公网 IP / 域名 / 内网穿透。走钉钉 Stream 模式，只出网、不监听公网。
- **它和你（agent）的关系**：当用户在**钉钉里**说话时，那侧是另一个 cf-connect 拉起来的会话；当用户在你这个终端里说话时，你能通过 `cf-connect send` 把消息推到钉钉。两边是**同一个 codefree-o 会话库**，用 `/list` + `/switch` 可以互相接管（见 §7）。

## 1. 你要做的事与你不该做的事

| 你能自动做 | 必须问用户 | 永远不要做 |
|---|---|---|
| 找 release 目录、读版本、落 PATH | **钉钉 AppKey / AppSecret**（只能用户自己去开放平台拿） | ❌ 替用户注册钉钉应用（需要登录钉钉后台） |
| 写 `config.toml`、填 `work_dir`、`cmd` | **用哪个目录当 `work_dir`**（写代码的位置，必须用户确认） | ❌ 把 AppSecret 打进日志、提交进 git、发到任何对话里 |
| 检测 codefree-o 是否已安装/登录 | 是否要设 `allow_from`（谁能跟机器人说话） | ❌ 在用户没确认的情况下用 `mode = "yolo"` |
| 起停服务、跑连通性验证 | 是否要常驻（后台服务）还是前台跑 | ❌ 改用户的全局 PATH 之外的系统设置、装驱动 |
| 生成交接摘要、推送消息 | 是否需要 AI 卡片流式（要用户去卡片平台建模板） | ❌ 声称"验证通过"却没真的收到过一条钉钉消息 |

**铁律**：`client_secret` 只能来自用户输入或已有的 `config.toml`，你只写进配置文件、只回显脱敏形式（`0qec…d88Z`）。

## 2. 先探测现状（任何操作前都先跑这一步）

```powershell
# 1) 有没有装、装在哪
Get-Command cf-connect -ErrorAction SilentlyContinue | Select-Object Source
@("$env:USERPROFILE\.cf-connect\cf-connect.exe","D:\software\cf-connect-v0.1.0-windows-amd64\cf-connect.exe",
  "D:\tools\cf-connect\cf-connect.exe") | ForEach-Object { if (Test-Path $_) { "FOUND: $_" } }

# 2) 有没有配置
cf-connect config path                 # 打印实际生效的 config.toml 路径
Test-Path "$env:USERPROFILE\.cf-connect\config.toml"
Test-Path .\config.toml                # ⚠️ 当前目录下的 config.toml 优先级更高

# 3) 宿主在不在
codefree-o --version

# 4) 服务状态（是否常驻）
cf-connect daemon status
Test-Path "$env:USERPROFILE\.cf-connect\run\api.sock"   # 存在 = 有实例在跑
```

配置查找顺序（`main.go:1440`）：**`--config` 参数 > 当前目录 `config.toml` > `~/.cf-connect/config.toml`**。
两处都没有时，`cf-connect` 会在**自己启动的那一刻**写一份带占位符的样例进去 ——
所以"配置不存在"这件事**只有真正去启动它才会被解决**；插件装完但没人启动时，
`~/.cf-connect/config.toml` 就是不存在（处理方式见 §3 的 A5）。

> ⚠️ `cf-connect daemon uninstall` **会真的执行卸载**，加 `--help` 也会执行。别拿它当"看看用法"。
> ⚠️ **Windows 上 `cf-connect doctor` 不支持**（会直接说 not supported）。别把它当验证手段，用 §5 的验证流程。

## 3. 场景 A：「安装 cf-connect」

用户可能只说一句"帮我装 cf-connect"、"把 codefree 接到钉钉"。照下面走，**每完成一步就把结果汇报给用户**，不要闷头做完。

### A1. 找到 release 目录

按顺序找，找不到就**停下来问用户**（不要自己去下载，本机可能没有外网）：

```powershell
Get-ChildItem "D:\software" -Directory -ErrorAction SilentlyContinue | Where-Object { $_.Name -like "cf-connect-*" }
Get-ChildItem "$env:USERPROFILE\Downloads","$env:USERPROFILE\Desktop" -ErrorAction SilentlyContinue | Where-Object { $_.Name -like "cf-connect-*.zip" }
```

找到的目录里应该有：`cf-connect.exe`、`config.example.toml`、`QUICKSTART.md`、`install.ps1`、`README.md`。
**建议把它放到英文路径**（如 `D:\tools\cf-connect\`），中文/空格路径会让第三方 CLI 出问题。

找不到就告诉用户：「请把 cf-connect 的发行 zip 解压到 `D:\tools\cf-connect\`，或告诉我 zip 在哪」，然后停住。

### A2. 落 PATH（自动）

```powershell
powershell -ExecutionPolicy Bypass -File <release目录>\install.ps1
```

- 只写 `HKCU`，不需要管理员权限；`-Uninstall` 可撤销
- **只对新开终端生效**——本会话内一律用绝对路径，别假设 `cf-connect` 现在就能直接敲
- 如果用户不想动 PATH，就跳过，后续所有命令用 `<release目录>\cf-connect.exe`

### A3. 认识两个目录（别搞混）

| 目录 | 是什么 | 里面有什么 |
|---|---|---|
| **解压目录**（如 `D:\tools\cf-connect-dingtalk\`） | 用户解压插件 tgz 得到的位置 | `package\`（含 `bin\cf-connect.exe`、`config.example.toml`、skill、README）、<br>`config.example.toml`、`QUICKSTART.md`、`install.ps1` |
| `%USERPROFILE%\.cf-connect\` | **运行时目录**，cf-connect 自己用 | `config.toml`（要你建）、`run\api.sock`、会话历史、日志 |

**配置放运行时目录，样例在解压目录**：`<解压目录>\package\config.example.toml`
→ 复制成 `%USERPROFILE%\.cf-connect\config.toml`。这是 A5 要做的事。

验证：

```powershell
& <release目录>\cf-connect.exe --version      # 必须打印版本号，如 cf-connect v0.1.0
```

### A4. 确认宿主（codefree-o）

```powershell
codefree-o --version
```

- 没有/报错 → 先解决这一条，cf-connect 离开它就是个空壳。提示用户安装并登录 CodeFree。
- 记下它的**绝对路径**（如果不在 PATH 里，后面 `cmd` 必须写绝对路径）：

```powershell
(Get-Command codefree-o -ErrorAction SilentlyContinue).Source
# 常见位置：C:\nvm4w\nodejs\node_modules\@srdcloud\codefree-o\bin\codefree-o.exe
```

### A5. 先建出 config.toml，再填（**这一步不能跳**）

解压/安装完之后 `~/.cf-connect/` 里**很可能根本没有 `config.toml`** ——
`cf-connect` 只在自己第一次启动时才生成，而那时它已经因为缺钉钉凭证起不来了。
所以**由你（agent）先把文件建出来**，它是填写的唯一依据：

```powershell
$cfgDir = "$env:USERPROFILE\.cf-connect"
New-Item -ItemType Directory -Force $cfgDir | Out-Null

if (-not (Test-Path "$cfgDir\config.toml")) {
  # 按优先级找“插件包自带的样例”：先找解压目录，再找已安装的 srdplugins
  $cands = @(
    "<解压目录>\package\config.example.toml"      # 解压 tgz 得到的目录
    "<解压目录>\config.example.toml"              # 有些包样例在根级
    "$env:USERPROFILE\.codefree-o\.config\srdplugins\cf-connect-dingtalk@*\package\config.example.toml"
  )
  $tpl = $cands | ForEach-Object { Get-Item $_ -ErrorAction SilentlyContinue } |
         Select-Object -First 1 -ExpandProperty FullName
  if (-not $tpl) { $tpl = "<release目录>\config.example.toml" }   # 最后回退到 cf-connect 发行目录

  if (Test-Path $tpl) {
    Copy-Item $tpl "$cfgDir\config.toml" -Force
    Write-Host "已从样例创建配置：$cfgDir\config.toml（来源：$tpl）"
  } else {
    Write-Host "!! 找不到 config.example.toml，无法自动创建配置"
  }
} else {
  Write-Host "配置已存在：$cfgDir\config.toml（保留原文件，不覆盖）"
}

# 三闸门：文件在、能解析、路径一致 —— 全过再往下走
Test-Path        "$cfgDir\config.toml"
& <cf-connect> config format --config "$cfgDir\config.toml"
& <cf-connect> config path --config "$cfgDir\config.toml"     # 必须回显同一个路径
```

> ⚠️ 别指望 `cf-connect.exe` 自己生成：它只在"当前目录没有 config.toml 且
> `~/.cf-connect/config.toml` 也不存在"时，于启动那一刻写一份带占位符的样例。
> 你要做的是**在启动之前**就把文件准备好，否则用户下次运行还是起不来。
> 已经存在 `config.toml` 时**不要覆盖**（用户可能填过凭证了）。

然后**逐项向用户确认/索要**，一次问清楚，别反复打扰：

| 要填的 | 谁来定 | 怎么问 / 怎么自动定 |
|---|---|---|
| `work_dir` | **问用户** | 「你希望我在哪个目录里改代码？给绝对路径。建议专用目录，别用家目录或 C 盘根目录。」填完后**必须过 A5.1 的实例校验** |
| `cmd` | 自动 | 探测到的 codefree-o 路径；在 PATH 里就写 `codefree-o`，否则写绝对路径（**常驻服务推荐绝对路径**，因为服务不继承终端 PATH） |
| `mode` | 建议 `default` | 「演示或首次使用建议 `default`；`yolo` 会跳过 CLI 的权限提示，确认了再开」 |
| `client_id` / `client_secret` | **只能用户给** | 见 §4 的引导话术 |
| `allow_from` | **强烈建议填** | 「把机器人锁给你自己，防止别人用你的额度。你在钉钉里发 `/whoami` 就能看到自己的 userId」 |
| `admin_from` | 可选 | 留空 = **所有人都不能用**管理命令（fail-closed），不是"继承 allow_from"。要用 `/shell`、`/dir` 这类命令就得填 userId |
| `card_template_id` | 可选 | 想要"边跑边看过程"才需要，见 §4 第 2 部分 |
| `language` | 自动 | 用户说中文就 `zh` |

#### A5.1 定稿前必须做的实例校验（**漏掉这一条 = 钉钉里第一条消息必炸**）

样例里的 `work_dir = "D:\\工作目录"`、`cmd = "codefree-o"` 是**占位符**。
如果直接照抄，服务能起来，但用户在钉钉发第一句话会收到：

```
Error: opencodeSession: start: chdir D:\工作目录: The system cannot find the file specified.
```

（原因：`agent/opencode/session.go` 把 `cmd.Dir` 设成 `work_dir`，目录不存在 exec 就失败；
cf-connect 启动时**不校验** `work_dir`，所以是延迟到第一条消息才报错。）

把配置里**所有"路径型"和"命令型"的值逐个实例校验**，全部通过才算定稿：

```powershell
# work_dir：必须存在，且是目录。不存在就直接建（别留给用户自己去踩）
$wd = "<用户给的工作目录>"
if (-not (Test-Path $wd -PathType Container)) {
  New-Item -ItemType Directory -Force $wd | Out-Null
  Write-Host "已创建工作目录：$wd"
}
Test-Path $wd -PathType Container          # 必须 True

# cmd：必须能解析到，否则先解决宿主再定稿
(Get-Command codefree-o -ErrorAction SilentlyContinue).Source

# 别留占位符：确认这两个值都不是样例原样
Select-String -Path "$cfgDir\config.toml" -Pattern '^\s*(work_dir|cmd)\s*='
```

判定：

| 检查项 | 不通过时怎么办 |
|---|---|
| `work_dir` 不存在 | **默认直接创建**；用户明确说不要就再问他要别的目录 |
| `work_dir` 指向家目录 / 系统盘根 / 网络共享 | 提醒风险（agent 会在这个目录里读写、跑命令），建议换成专用项目目录 |
| `cmd` 解析不到 | 先装/修好 codefree-o，或用绝对路径；**不要**带着一个跑不起来的 `cmd` 定稿 |
| 配置里还留着 `D:\工作目录` 这类占位符 | 说明没真正改过，回到上表重新填 |

#### A5.2 回显

回显给用户时，`client_secret` 一律脱敏：**前 4 位 + `…` + 后 4 位**。
并明确告诉他「`work_dir` 已设为 `X`，在钉钉里可以用 `/dir <绝对路径>` 随时切换」。

## 4. 引导用户拿钉钉凭证（你无法代替的一步）

用户说"你帮我配一下钉钉"时，直接把下面这段给他（别自己瞎猜后台路径）：

> **请在钉钉开放平台按这几步操作，拿到两个值发我：**
> 1. 打开 https://open-dev.dingtalk.com/ → 应用开发 → **企业内部应用** → 创建应用
> 2. 在「凭证与基础信息」里复制 **AppKey** 和 **AppSecret**（AppKey = 我配置里的 `client_id`）
> 3. 左侧「机器人」→ 打开机器人配置 → **消息接收模式必须选 `Stream`**（这样你本机不用公网 IP）
> 4. 左侧「权限管理」→ 勾选机器人发送消息、接收消息相关权限
> 5. **右上角「版本管理与发布」→ 发布应用**（没发布消息进不来）
> 6. 钉钉里搜到你的机器人 → 发一条 `/whoami` → 把它回复的 **userId** 也发我（我用来做 `allow_from` 白名单）

拿到后写进配置：

```toml
[projects.platforms.options]
client_id = "<AppKey>"
client_secret = "<AppSecret>"
allow_from = "<用户的userId>"
```

（可选，想要流式过程）再引导一次：

> 想看到"边想边做"的实时过程吗？需要在钉钉开放平台 → **卡片平台** → 新建「AI 卡片」模板，里头放一个绑定变量 `content` 的 Markdown 组件，发布后把**模板 ID** 发我。不配也能用，只是回复会在整轮结束后一次性到达。

如果用户拿不到（无权限、公司流程慢），**不要卡住**：告诉他可以先跳过，cf-connect 起不来是正常的，凭证到位后再继续。

## 5. 场景 B：「测一下连通性」

按顺序跑，**每一步都给出判断和下一步动作**，不要只看命令退出码：

```powershell
$cc = "<cf-connect 绝对路径或命令名>"

# ① 二进制与版本
& $cc --version

# ② 配置能否解析（会回显规整后的文件）
& $cc config format --config "$env:USERPROFILE\.cf-connect\config.toml"

# ③ 宿主可用
codefree-o --version

# ④ 服务是否在跑（有实例会创建这个 socket 文件）
Get-Process cf-connect -ErrorAction SilentlyContinue | Select-Object Id,ProcessName
Test-Path "$env:USERPROFILE\.cf-connect\run\api.sock"

# ⑤ 真正发一条到钉钉（用户必须在钉钉里收到，才算通）
& $cc send -m "cf-connect 连通性测试 $(Get-Date -Format 'HH:mm:ss')"
```

判定表（照着回给用户）：

| 现象 / 报错 | 原因 | 下一步 |
|---|---|---|
| 钉钉里回 `Error: opencodeSession: start: chdir <路径>: The system cannot find the file specified.` | **`work_dir` 指向了不存在的目录**（多半是照抄了样例里的占位符） | 配置里改成真实目录后 `daemon restart`；或直接在钉钉发 `/dir <绝对路径>`（命令不拉 agent，所以还能用）；预防见 §3 的 A5.1 |
| ① 报 command not found | 没装或没在 PATH | 用绝对路径；或跑 `install.ps1` 后**新开终端** |
| ② 报 `Config file not found: ...\.cf-connect\config.toml` | 装完还没建配置（**最常见**） | 回 §3 的 A5：从 `package\config.example.toml` 复制到 `~/.cf-connect/config.toml`，再填 4 处 |
| ② 报解析错误 | 配置格式坏了 | 对着 `config.example.toml` 修，或重新生成 |
| ③ 失败 | 宿主缺失/未登录 | `codefree-o --version` 修复；必要时 `codefree-o auth` |
| ④ 没有 cf-connect 进程 | 服务没起 | `& $cc daemon install ; & $cc daemon start` |
| ⑤ 报 `cf-connect is not running (socket not found)` | 同上，服务没起 | 先解决 ④ |
| ⑤ 报凭证错误 | `client_id`/`client_secret` 不对 | 回 §4 重新核对（注意别贴明文） |
| ⑤ 返回成功但钉钉没收到 | 应用没发布 / 不是 Stream 模式 / 机器人没启用 / `allow_from` 没包含自己 | 逐条核 §4 的 3、4、5 步 |
| 全绿但用户说"我发消息它不回" | 会话侧问题 | 让用户在当前会话发 `/status`；或看 `~/.cf-connect` 下的日志 |
| 验证时**故意发一句话**回来了 `chdir ... cannot find the file` | `work_dir` 不存在（照抄了占位符） | 改配置或 `/dir <真实路径>`；预防见 §3 的 A5.1。**收到这条就说明前面几步全过了，只差目录** |

**验证成功的定义只有一个：用户在钉钉里真的收到并回了一条消息。** 命令返回 0 不算。

## 6. 场景 C：「卸载 cf-connect」

**先确认用户要卸到什么程度**，再动手（一句话问清）：

| 级别 | 动作 | 会丢什么 |
|---|---|---|
| 只停服务 | `& $cc daemon stop` | 不丢 |
| 删服务（保留数据与配置） | `& $cc daemon uninstall` | 不丢 |
| 从 PATH 移除 | `powershell -File <release目录>\install.ps1 -Uninstall` | 不丢 |
| 删数据目录 | 删除 `%USERPROFILE%\.cf-connect` | **会丢会话历史、定时任务、web token** |
| 删程序目录 | 删除 release 目录 | 不丢数据 |

标准顺序：

```powershell
& $cc daemon stop
& $cc daemon uninstall
powershell -ExecutionPolicy Bypass -File <release目录>\install.ps1 -Uninstall
# 只有用户明确说"全清"才做下面这步
Remove-Item -Recurse -Force "$env:USERPROFILE\.cf-connect"
```

> ⚠️ 卸载前提醒一句：**钉钉那边不会自动解绑**。如果用户以后还要用，钉钉应用留着别删；应用本身是否删除由用户去开放平台决定。
> ⚠️ 别顺手删 `%USERPROFILE%\.codefree-o`——那是 CodeFree 自己的目录，跟 cf-connect 无关。

## 7. 场景 D：用户说"我在钉钉里跟你说话"、"手机上接着弄"

两边共用同一个 `codefree.db` 会话库，所以可以互相接管：

| 用户想做的事 | 做法 |
|---|---|
| 在钉钉里继续终端那个会话 | 钉钉里发 `/list` 看列表 → `/switch <会话ID前缀或描述关键字>` |
| 把终端的结果推到钉钉 | `cf-connect send --stdin`（长文本用 `--stdin`，避免引号/换行被 shell 吃掉） |
| 查自己在钉钉里的身份 | 钉钉里发 `/whoami` |
| 换工作目录 | 钉钉里 `/dir` 相关命令，或改 `config.toml` 的 `work_dir` 后重启服务 |

**用 `/switch` 的两个要点**（务必转告用户）：

1. **优先用会话 ID 或描述关键字，不要用序号**（`/switch 2` 里的 2 是列表第 2 项，列表一变就错位）
2. **切换前确认终端那侧的会话已经退出**：同一时间两边写一个会话，消息可能交错、历史可能被覆盖（这是当前版本的已知边界，0.1.0 未做互斥）

推送消息示例：

```powershell
cf-connect send --stdin <<'EOF'
**构建完成** ✅
- 用例：128 passed / 0 failed
- 产物：dist/cf-connect-v0.1.0-windows-amd64.zip
EOF
```

## 8. 场景 E：升级 cf-connect

自更新功能已移除，升级 = **换 exe 覆盖 + 重启**：

```powershell
& $cc daemon stop
Copy-Item <新版本目录>\cf-connect.exe <原安装目录>\cf-connect.exe -Force
& $cc daemon start
& $cc --version                     # 确认新版本
& $cc send -m "upgrade check"
```

升级不影响：`~/.cf-connect`（会话历史）、`config.toml`（凭证与 work_dir）。
**唯一要注意**：新版本如果加了配置项，用户的 `config.toml` 不会自动获得——拿新版本目录里的 `config.example.toml` 比一比，缺什么补什么。

## 9. 模糊指令 → 动作对照（用户会怎么说）

| 用户可能说 | 你要做什么 |
|---|---|
| "装 cf-connect" / "把 codefree 接到钉钉" | §2 探测 → §3 安装 → §4 索要凭证 → §5 验证 |
| "测试连通性" / "通不通" / "钉钉没反应" | §5 全流程 + 判定表 |
| "卸载" / "不要了" / "删掉" | §6，先问清卸到哪一级 |
| "换个工作目录" | 改 `config.toml` 的 `work_dir` → `daemon restart` → `cf-connect send -m` 验证 |
| "让 codefree 只在某个项目里干活" | 确认 `work_dir`，并建议把 `<work_dir>/.cf-connect/` 加进 `.gitignore` |
| "把结果发到钉钉" | §7 的 `cf-connect send`；没有活跃会话就说明原因，别反复重试 |
| "手机上能接着弄吗" | §7 的 `/list` + `/switch`，并提醒两个要点 |
| "能不能不改代码只看过程" | 引导配 `card_template_id`（§4 可选段） |
| "担心安全" | 见 §10 安全清单 |

## 10. 安全清单（配完主动跟用户过一遍）

- `client_secret` 是**每人一套**的本机凭据：只放 `~/.cf-connect/config.toml`（别放项目目录），不提交 git、不发群里
- `allow_from` 填用户自己的 userId（钉钉发 `/whoami` 可查），否则任何能搜到机器人的同事都能用其额度
- `admin_from` 留空 = 管理命令对所有人关闭（fail-closed）；要开就填明确的 userId
- `work_dir` 指向**专用工作目录**，不要指到家目录或系统盘根
- 首次演示建议 `mode = "default"`；`yolo` 会跳过 CLI 的权限提示（cf-connect 本身没有交互式权限确认，别把它当成安全门）
- 把 `<work_dir>/.cf-connect/` 加进项目的 `.gitignore`（媒体/附件会落在那里）
- Windows 未签名 exe 可能触发 SmartScreen → 「更多信息 → 仍要运行」；企业内可统一签名

## 11. 排障速查

| 症状 | 原因 | 处置 |
|---|---|---|
| `socket not found` / `cf-connect is not running` | 服务没起 | `daemon start`；前台调试就直接 `cf-connect` 看日志 |
| 钉钉里发消息没反应 | 应用没发布 / 不是 Stream 模式 / 机器人没启用 / `allow_from` 不含自己 | 逐条核 §4 |
| `unknown flag: --dangerously-skip-permissions` | `yolo` 给老版本 CLI 发了不支持的标志 | 配 `permission_flag = "--auto"`（默认值） |
| 回复慢、看不到过程 | 没配 AI 卡片流式 | 配 `card_template_id` |
| 会话标题/消息数空 | 数据目录没识别对 | 配 `data_dir` / `db_file`（见 config 里「高级」段） |
| 服务起不来、报配置错误 | `config.toml` 语法/键位不对 | `config format` 校验；`allow_from` 必须在 `[projects.platforms.options]` 下，写到 `[[projects]]` 里会被**静默丢弃**，机器人就对所有人开放 |
| 中文/空格路径下行为异常 | 第三方 CLI 解析问题 | 换到英文路径（如 `D:\tools\cf-connect\`） |
| `cf-connect doctor` 报 not supported | Windows 不支持该命令 | 用 §5 的验证流程替代 |

## 12. 你可以顺手用的 cf-connect 能力（用户问起时别答"不知道"）

```
cf-connect send -m "..."                发消息到钉钉活跃会话（--stdin 读长文本）
cf-connect sessions list|show <id>      浏览会话历史
cf-connect agent-sid                    打印当前会话的 agent 会话 ID（可对接管/续聊）
cf-connect cron add/list/exec/del       定时任务
cf-connect timer add/list/info/del      一次性提醒
cf-connect relay send --to <项目> ...   跨项目转发并等回复
cf-connect provider add/list/remove     管理 API provider
cf-connect web                          内嵌 Web 管理界面
cf-connect config example|format|path   配置：打印样例 / 规整 / 查路径
cf-connect daemon install|start|stop|restart|status|logs|uninstall
```

配置里可调的关键项（都在 `config.toml` 注释里）：`work_dir`、`cmd`、`mode`、`model`、`agent`、`allow_from`、`admin_from`、`card_template_id`、`language`、`[display]`、`[cron]`、`[management]`。
