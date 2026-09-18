# CF-Connect 快速上手

把 **CodeFree-O** 从"必须坐在电脑前用"变成"在钉钉里随时用"。

整个过程 **3 步**，只需要一个压缩包 + 你自己的钉钉应用凭证。
**不需要** Go、Node、Python、Java 或 npm。

---

## 0. 你需要准备什么

| 项目 | 说明 |
|---|---|
| 本压缩包 | `cf-connect-vX-windows-amd64.zip` |
| 解压目录 | **建议英文路径**，例如 `D:\tools\cf-connect\`（避免中文/空格引发的第三方 CLI 解析问题） |
| 钉钉账号 | 需要能创建「企业内部应用」的权限 |
| CodeFree-O CLI | 已安装并**已登录**（`codefree-o --version` 能正常输出） |

---

## 1. 解压（可选加入 PATH）

1. 把压缩包解压到英文目录，例如 `D:\tools\cf-connect\`
2. 解压后目录里应该有：

```
cf-connect.exe          主程序（Web 管理界面已内嵌，无需 Node）
config.example.toml     配置模板
QUICKSTART.md           本文件
install.ps1             可选：把当前目录加入用户 PATH
README.md
```

3. （可选）以**普通用户身份**打开 PowerShell，执行：

```powershell
cd D:\tools\cf-connect
powershell -ExecutionPolicy Bypass -File .\install.ps1
```

之后**新开的**终端里就能直接敲 `cf-connect` 了。不加 PATH 也可以，用完整路径调用即可。

> **SmartScreen / 杀毒软件提示？**
> 未签名的 exe 首次运行可能被 Windows 拦截。
> 处理方式：右键 `cf-connect.exe` → 属性 → 勾选「解除锁定」→ 确定；
> 或在 SmartScreen 弹窗里点「更多信息」→「仍要运行」。
> 若公司有统一签名/白名单流程，请走该流程。

---

## 2. 创建钉钉应用（Stream 模式，不需要公网 IP）

> 每个使用者创建**自己的**应用，凭证互不影响。

1. 打开 [钉钉开放平台](https://open-dev.dingtalk.com/) → 登录
2. **应用开发 → 企业内部应用 → 钉钉应用 → 创建应用**
   - 应用名称：随意，例如 `CodeFree 助手`
   - 应用类型：**企业内部应用**
3. 进入应用 → **凭证与基础信息**，记下两个值：
   - `Client ID`（即 AppKey）
   - `Client Secret`（即 AppSecret）
4. **添加应用能力 → 机器人**
   - 机器人名称随意
   - **消息接收模式：选择「Stream 模式」** ← 关键，选这个就不需要公网 IP、不需要配回调地址
   - 发布/保存
5. （可选，想要**流式打字机效果**才需要）
   钉钉开放平台 → **卡片平台** → 新建一个 **AI 卡片**模板，
   在卡片里放一个 **Markdown** 组件，并把它的内容绑定到一个变量，变量名填 `content`；
   发布后记下模板 ID（形如 `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.schema`），
   填到配置的 `card_template_id`。
   不配也能用，但**没有流式**：整轮跑完才一次性发出答案（通常等十几秒）。

6. **把自己的钉钉 userid 填进 `allow_from`**（推荐）
   先按第 3 步跑起来，在钉钉里对机器人发 `/whoami`，它会回你的 userid。
   然后把该 ID 填进 `config.toml` 里 **`[projects.platforms.options]` 段下的**
   `allow_from`，避免别人误用你的额度。
   ⚠️ 不要填在 `[[projects]]` 下——那里会被静默忽略，等于没设。

---

## 3. 写配置并运行

```powershell
cd D:\tools\cf-connect
# 首次运行写出样例配置（用户目录，和会话/日志同处），然后退出
.\cf-connect.exe --config "$env:USERPROFILE\.cf-connect\config.toml"
notepad "$env:USERPROFILE\.cf-connect\config.toml"
```

> 配置固定放在 `%USERPROFILE%\.cf-connect\config.toml`（不在解压目录里）。
> 查找顺序是 `--config` > 当前目录 `./config.toml` > `~/.cf-connect/config.toml`，
> 所以生成/校验/装服务时都显式带 `--config`，免得当前目录里恰好有一份 `config.toml` 把它顶掉。

最少只需要改这几处：

```toml
[[projects]]
name = "my-project"
# 授权特权命令（/dir、/shell、/show、/diff、/web、/restart、/upgrade）：
# 注意它写在 [[projects]] 下，位置与下方的 allow_from 正好相反。
# 不填 = 特权命令对所有人一律拒绝，钉钉里会回 "requires admin privilege"。
# admin_from = "你的 userid"

[projects.agent]
type = "opencode"

[projects.agent.options]
work_dir = "E:\\work\\my-project"    # 你希望它改代码的目录
cmd = "C:\\nvm4w\\nodejs\\node_modules\\@srdcloud\\codefree-o\\bin\\codefree-o.exe"
                                     # 宿主 CLI 的绝对路径（取法见下方说明）
mode = "default"                     # 只决定追加哪个权限 flag，不是"安全模式"，见下方说明

[[projects.platforms]]
type = "dingtalk"

[projects.platforms.options]
client_id = "第 2 步拿到的 AppKey"
client_secret = "第 2 步拿到的 AppSecret"
# 建议填上，否则谁都能用你的机器人（注意必须写在 [projects.platforms.options] 里）
# allow_from = "你的 userid"
```

> ⚠️ **`mode = "default"` 不是安全开关。** 它只是不追加"跳过权限"的 flag，
> 裁决权在 CLI 自己手上；而无人值守调用（`run`）下 codefree-o / opencode 会**直接执行工具**，
> 不会询问。cf-connect 里也没有"权限确认"这个环节——适配器的 `RespondPermission`
> 是空实现，钉钉里不会出现允许/拒绝按钮。
> 需要真正的工具管控，请配置 CLI 自身的权限规则，或把 agent 放进沙箱/容器里跑。
>
> ⚠️ **`allow_from` 必须写在 `[projects.platforms.options]` 下。** 写在 `[[projects]]`
> 下不是"不生效"，而是会被 TOML 解码器**静默丢弃**（既不报错也不告警），
> 结果就是机器人对所有人开放。

> **`cmd` 必须填绝对路径。** 后台服务（daemon）不继承你当前终端的 PATH。
>
> ⚠️ 别直接抄 `where codefree-o` 或 `(Get-Command codefree-o).Source`：npm 装法的 codefree-o
> 同时有 `.ps1` / `.cmd` / 无扩展名三份 shim，这两条命令可能给你 **`.ps1`**，
> 而 cf-connect 内部用 Go 的 `exec` **拉不起 `.ps1`**。症状很有迷惑性：服务正常、日志正常，
> 只有钉钉里第一句话报 `"..." CLI not found in PATH`。正确取法：
> `$shim = (Get-Command codefree-o.cmd).Source`，再取
> `Join-Path (Split-Path $shim) 'node_modules\@srdcloud\codefree-o\bin\codefree-o.exe'`
> （存在就用它，否则用 `$shim`）。

### 前台运行（先这样验证）

```powershell
.\cf-connect.exe
```

控制台出现启动日志后，去钉钉里给机器人发一条消息试试，例如：

```
你好，用一句话介绍你自己
```

能在钉钉里收到回复就说明链路通了。
（**默认不会"流式"**：没有配 `card_template_id` 时，整轮跑完才把答案一次性发出来，
通常要等十几秒。想要逐字刷新的卡片效果，见第 2 步的第 5 点。）

### 常驻运行（验证通过后再装）

```powershell
.\cf-connect.exe daemon install    # 安装并启动后台服务
.\cf-connect.exe daemon status     # 查看状态
.\cf-connect.exe daemon logs -f    # 跟踪日志
```

其它常用命令：

```powershell
.\cf-connect.exe --version
.\cf-connect.exe daemon stop
.\cf-connect.exe daemon uninstall
```

> `cf-connect doctor` 不要用：在 Windows 上它只打印
> `doctor command is not supported on Windows`，什么都不检查。

---

## 4. 排障

### 先看日志，别跑 doctor

```powershell
# 前台运行：日志直接打在终端上
# daemon 运行：
cf-connect daemon logs -n 100
```

启动正常的标志是这几行（本项目实测输出）：

```
level=INFO msg="dingtalk: stream connected" client_id=dingxxxxxxxx
level=INFO msg="platform ready" project=... platform=dingtalk
level=INFO msg="cf-connect is running" projects=1
```

### 常见问题

| 现象 | 原因 / 处理 |
|---|---|
| 启动报 `"codefree-o" CLI not found in PATH` | `cmd` 没配或路径不对。用 `where codefree-o` 查绝对路径填进去 |
| 钉钉里发消息没有任何反应 | ① 机器人消息接收模式不是 **Stream** ② `client_id`/`client_secret` 填错 ③ 应用没发布 ④ 看日志：`cf-connect daemon logs -n 100` |
| 启动日志报 `invalidClientIdOrSecret` | 凭证不对，通常是 Client Secret 复制漏/多了字符。它是 64 位，核对首尾各 10 位 |
| 回复里没有会话标题 / 消息数 | 你的 CodeFree-O 数据目录不是默认位置。用 `codefree-o debug paths` 看 `data` 路径，把 `data_dir` 填进配置 |
| 会话列表是空的 | 该 `work_dir` 下还没跑过会话，先在钉钉里让机器人干点活 |
| 回复要等很久，且日志有 `streaming card creation failed` | 正常：没配 `card_template_id`，只能等整轮结束。配了卡片模板才有流式 |
| 日志出现 `allow_from is not set — all users are permitted` | 你把它写在 `[[projects]]` 下了。挪到 `[projects.platforms.options]` |
| 发 `/dir`（或 `/shell`、`/show`）回 `requires admin privilege` | 没配 `admin_from`。它和 `allow_from` 位置**相反**：要写在 **`[[projects]]`** 段下（不是 `[projects.platforms.options]`），改完重启 |
| `yolo` 模式报 unknown flag | 你的 CLI 版本不认 `--auto`。在配置里设 `permission_flag = "--dangerously-skip-permissions"`（老版 opencode），或 `"none"` |
| 别人也能用我的机器人 | 在 **`[projects.platforms.options]`** 下填 `allow_from = "你的userid"`（`/whoami` 可查） |
| 机器人不询问就改了文件 | 预期行为。cf-connect 没有交互式权限确认，`mode = "default"` 也不拦；要管控请配 CLI 自己的权限或上沙箱 |
| 想换模型 | `codefree-o models` 看可用列表，填 `model = "provider/model"` |
| 端口/服务冲突 | `daemon status` → `daemon stop` → 检查是否有第二个实例：启动时加 `--force` |

### 日志位置

| 运行方式 | 日志 |
|---|---|
| 前台 | 直接输出到终端 |
| daemon | 由服务管理器写入日志文件，用 `cf-connect daemon logs -f` 查看 |

### 钉钉里的常用命令

| 命令 | 作用 |
|---|---|
| `/whoami` | 查看自己的 userid（用来配 `allow_from` / `admin_from`） |
| `/new` | 开一个新会话 |
| `/list` | 列出会话 |
| `/switch <id>` | 切到某个会话 |
| `/history` | 看当前会话历史 |
| `/model` | 查看/切换模型 |
| `/dir [路径]` | 查看/切换 agent 工作目录（**特权命令**，需 `admin_from`） |
| `/stop` | 打断当前这一轮 |
| `/help` | 全部命令 |

> `/dir`、`/shell`、`/show`、`/diff`、`/web`、`/restart`、`/upgrade` 都是**特权命令**，
> 必须先在 `config.toml` 的 **`[[projects]]`** 段下配好 `admin_from`，
> 否则一律被拒绝（钉钉里回 `requires admin privilege`）。

---

## 5. 升级

自更新功能已移除，升级 = **换新压缩包**：

```powershell
cf-connect daemon stop
# 用新的 zip 覆盖解压到同一目录（配置在 %USERPROFILE%\.cf-connect，不受影响）
cf-connect daemon start
cf-connect --version      # 确认版本
```

> 配置和会话数据都在 `~/.cf-connect/` 里，覆盖压缩包不会碰它们；要保险可以整体备份这个目录。

---

## 6. 安全提醒

- `client_secret` 是凭证，不要提交到 git、不要发群里
- `work_dir` 建议指向一个你愿意让 agent 改动的目录，不要直接指向整个用户目录
- 首次使用建议 `mode = "default"`，确认行为符合预期后再考虑 `yolo`
- 把 `<你的 work_dir>\.cf-connect\` 加入项目的 `.gitignore`（会话状态/附件会写在这里）
