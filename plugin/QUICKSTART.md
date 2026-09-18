# CF-Connect 四步上手（钉钉 ↔ CodeFree）

> 配套插件：`cf-connect-dingtalk` v0.1.1 ｜ 网关：`cf-connect.exe` v0.1.1

## 第 0 步：解压

把插件包解压到**英文路径**（例如 `D:\tools\cf-connect-dingtalk\`），
然后（可选）把 `bin\` 加入 PATH：

```powershell
powershell -ExecutionPolicy Bypass -File .\install.ps1
```

确认两个前置：

```powershell
codefree-o --version     # 宿主 CLI
cf-connect --version     # 网关（没加 PATH 就用 .\bin\cf-connect.exe --version）
```

---

## 第 1 步：建钉钉企业内部应用（拿凭证）

1. 打开 [钉钉开放平台](https://open-dev.dingtalk.com/) → **应用开发 → 企业内部应用 → 创建应用**
2. 记下 **AppKey** 和 **AppSecret**（后面要填进 `config.toml` 的 `client_id` / `client_secret`）
3. 左侧 **机器人** → 打开机器人配置：
   - **消息接收模式：必须选 `Stream` 模式**（这样本机不需要公网 IP、不需要内网穿透）
   - 机器人名称随意，例如 `CodeFree 助手`
4. 左侧 **权限管理**，至少勾选：
   - 企业内机器人发送消息（`qyapi_robot_sendmsg`）
   - 机器人接收消息相关权限（Stream 模式所需）
5. **发布**应用（版本管理与发布 → 发布）。未发布时消息进不来。
6. 把机器人加到自己的单聊：钉钉 → 搜索机器人名称 → 发一条 `/whoami`，
   机器人会回复你的 **userId** —— 把它填到 `allow_from`（强烈建议，防止他人误用）。

> 可选（更好的体验）：**卡片平台 → 新建「AI 卡片」模板**，放一个绑定变量
> `content` 的 Markdown 组件并发布，把模板 ID 填到 `card_template_id`。
> 不配也能用，只是回复会在整轮结束后一次性到达，看不到流式过程。

---

## 第 2 步：生成 config.toml（**放在 `~/.cf-connect`**）

配置文件固定在 **`%USERPROFILE%\.cf-connect\config.toml`** —— 和会话历史 / 日志 / `api.sock` 同一个目录。
一开始它**不存在**，也不用手工复制样例：**指定 `--config` 跑一次 exe，它会把配置写出来然后自己退出**。

```powershell
cd /d "解压目录\package\bin"
cf-connect.exe --config "%USERPROFILE%\.cf-connect\config.toml"   # 首次运行：写出配置，然后退出
notepad "%USERPROFILE%\.cf-connect\config.toml"
```

> 为什么放这儿：配置和运行时数据在一个目录，排障只看一处；而且**插件目录名带版本号**，
> 配置放在 exe 旁边的话，每次升级都会连同旧目录一起被删掉（凭证也就没了）。
> ⚠️ **`--config` 建议显式带上**：查找顺序是 `--config` > 当前目录 `./config.toml` > `~/.cf-connect/config.toml`，
> 当前目录里恰好有一份 `config.toml` 时会把它顶掉。
> 装上本插件后，这一步 agent 会自动帮你做（skill 里就是这么写的）；只有自己手动装时才需要手敲。
> ⚠️ 也别手工 `copy config.example.toml`：exe 写的是它**自己内嵌**的那份样例，手工复制容易版本对不上。

只需要改这 4 处：

```toml
[projects.agent.options]
work_dir = "D:\\你的项目目录"        # ① 必改：必须是这台机器上真实存在的目录
cmd = "C:\\nvm4w\\nodejs\\node_modules\\@srdcloud\\codefree-o\\bin\\codefree-o.exe"
                                   # ② 宿主 CLI 的绝对路径 —— 别写 "codefree-o"，怎么取见下方「② cmd 怎么取」

[projects.platforms.options]
client_id = "你的 AppKey"           # ③
client_secret = "你的 AppSecret"    # ④
# allow_from = "你的 userId"        # 第 4 步拿到 userId 后再填
```

> ⚠️ **① 千万别留着占位符**。`work_dir` 指向不存在的目录时，服务能起来，
> 但你在钉钉里发第一句话会报
> `Error: opencodeSession: start: chdir <路径>: The system cannot find the file specified.`
> 目录不存在就先新建一个。已经踩到了也可以不改配置，直接在钉钉里发 `/dir <绝对路径>`。

> ⚠️【为保证对话私密性，不建议公用机器人。请自己创建测试企业和机器人来进行使用】

---

## 第 3 步：跑起来（装成后台服务）

配置填好后，**装成后台服务**——这是唯一"真正启动"的方式（关掉终端也一直在跑）：

```powershell
# --config 指向用户目录里的配置；daemon install 拿它所在目录当服务的工作目录
cf-connect daemon install --config "%USERPROFILE%\.cf-connect\config.toml"
cf-connect daemon status       # 期望 running
cf-connect send -m "bridge ping"   # 钉钉里应该收到（send 走 socket，不需要 --config）
```

- 关服务：`cf-connect daemon stop`；彻底移除服务：`cf-connect daemon uninstall`
- ⚠️ **前台方式（`cd 到 bin` 直接跑 `cf-connect.exe`）只用于临时看日志**：
 日志里会打印 `stream connected`，但那只是这次命令调用还活着，**命令一结束进程就没了，等于没启动**
- ⚠️ 后台服务**不继承你终端的 PATH**，所以配置里的 `cmd` 必须写 codefree-o 的**绝对路径**

**② `cmd` 怎么取**（别用 `(Get-Command codefree-o).Source`）：
npm 全局装的 codefree-o 有三份 shim（无扩展名的 `codefree-o`、`codefree-o.cmd`、`codefree-o.ps1`），
`Get-Command codefree-o` 多半给你 **`.ps1`** —— 那是给交互式 shell 用的，cf-connect 内部用
Go 的 `exec` 拉不起来。写成 `.ps1` 的症状很有迷惑性：**服务正常、日志正常，只有钉钉里第一句话**
报 `"..." CLI not found in PATH`。

```powershell
$shim   = (Get-Command codefree-o.cmd -ErrorAction SilentlyContinue).Source
$native = Join-Path (Split-Path $shim) 'node_modules\@srdcloud\codefree-o\bin\codefree-o.exe'
if (Test-Path -LiteralPath $native -PathType Leaf) { $native } else { $shim }   # ★ 原样填进 cmd
```

```
帮我看下 D:\workspace_idea\demo 这个项目的编译错误并修掉
```

---

## 第 4 步：回填 allow_from / admin_from（**必须等第 3 步跑起来之后**）

⚠️ **顺序不能颠倒**：userId 只能在钉钉里对机器人发 `/whoami` 得到，而机器人上线的前提是
**你已经填好 `client_id` / `client_secret` 并且服务已经 running**（第 3 步）。
没起来之前发 `/whoami` 不会有任何回复 —— 先回去把第 3 步做完。

启动后日志里通常有两条 WARN（`allow_from is not set` / `admin_from is not set`），
这是"暂时没锁白名单"的正常提示，**不影响你现在去取 userId**：

```powershell
# 现在可以做了：在钉钉里对机器人发 /whoami，拿到自己的 userId
```

```toml
[[projects]]
admin_from = "你的 userId"                # 谁能用 /dir /show /shell /restart（留空=全部关闭）

[projects.agent.options]
work_dir = "D:\\你的项目目录"              # agent 实际改代码的目录（必填、必须真实存在）

[projects.platforms.options]
allow_from = "你的 userId"                # 谁能跟机器人对话（不填=所有人都能用你的额度）
```

```powershell
cf-connect daemon restart      # 让配置生效
```

---

## 自检清单

| 检查 | 命令 | 期望 |
|---|---|---|
| 网关在跑 | `cf-connect daemon status` + `dir %USERPROFILE%\.cf-connect\run` | `running`，且目录里有 `api.sock`（⚠️ Windows 上 `cf-connect doctor` 不支持，别用它） |
| 能发消息到钉钉 | `cf-connect send -m "bridge ping"` | 钉钉里收到 `bridge ping` |
| 插件已注入 | `codefree-o debug skill` | 列表里有 `cf-connect-setup` 和 `dingtalk-bridge` |
| 我的 userId | 钉钉里发 `/whoami` | 回复一串 userId（填进上面三个键） |

## 懒人模式：让 codefree-o 自己装

装上本插件后，用户机器上的 codefree-o 会获得 `cf-connect-setup` skill，
上面这一整套（找解压目录 → 落 PATH → **生成 `~/.cf-connect\config.toml`** →
**`daemon install` 装成后台服务** → 引导拿钉钉凭证 → 拿到 userId 后回填两个白名单 → 验证）
都可以直接在 codefree-o 对话框里一句话交给 agent：

```
安装 cf-connect
测一下钉钉连通性
卸载 cf-connect
```

agent 会自动做完能自动做的部分，只在**必须用户提供**的地方停下来问你
（钉钉 AppKey/AppSecret、`work_dir`，以及**等服务跑起来之后**再要你发 `/whoami` 把 userId 给它）。

## 常见问题

| 现象 | 原因 | 处置 |
|---|---|---|
| 钉钉发消息没反应 | 应用没发布 / 不是 Stream 模式 / 机器人没启用 | 回第 1 步 3、5 项 |
| `socket not found` | 网关没启动 —— 多半是只用了前台方式跑（命令一结束进程就没了） | `cf-connect daemon install --config "%USERPROFILE%\.cf-connect\config.toml"`（装过就 `daemon restart`） |
| 日志有 `stream connected` 但钉钉没反应 | 前台跑的后遗症：进程随后被结束 | 改用后台服务启动（同上） |
| 跑 `cf-connect.exe` 一下就退出，提示 `Created default config at ...<路径>` | 正常：当时没有 `config.toml`，它写完就退出 | 路径应为 `%USERPROFILE%\.cf-connect\config.toml`；写到别处多半是当前目录有一份 `config.toml` 被优先命中 |
| `unknown flag: --dangerously-skip-permissions` | yolo 模式给老版本发的标志不对 | `permission_flag = "--auto"`（默认值） |
| 回复慢、没有过程 | 没配 AI 卡片流式 | 配 `card_template_id`，见第 1 步可选段 |
| 会话标题/消息数空 | 数据目录没识别对 | 配 `data_dir` / `db_file`（见 config.toml 高级段） |
| SmartScreen 拦截 exe | 未签名 | 「更多信息 → 仍要运行」 |
| 敲 `cf-connect` 说"无法将"cf-connect"项识别为 cmdlet、函数、脚本文件或可运行程序的名称" | 包里的 `bin\` 不在**用户 PATH** 里（或者那条指向的是已被删除的旧目录）；也可能只是终端是改 PATH 之前开的 | 重新跑 `install.ps1`，然后**新开**终端；`where cf-connect` 只看当前进程，判断不了注册表 |
| 跑 `install.ps1` 报 `字符串缺少终止符: '`（ParserError） | 老包里的 .ps1 是 UTF-8 **无 BOM** + 中文注释，PowerShell 5.1 按 GBK 解码把引号吞了 | 换 `pwsh -File .\install.ps1`（PowerShell 7），或手工把 `bin\` 加进用户 PATH；新版打包已默认补 BOM |
| 服务 `running`、日志也 `stream connected`，但钉钉里第一句话报 `"..." CLI not found in PATH` | `cmd` 写了裸名字 `codefree-o` 或写成 `.ps1` | 按第 2 步「② `cmd` 怎么取」换成 `.exe` / `.cmd` 绝对路径，再 `cf-connect daemon restart` |

## 安全提醒

- 钉钉 `client_secret` 是**每人一套**的本机凭据，就放在 **`%USERPROFILE%\.cf-connect\config.toml`**；
  不要提交到 git、不要发群里、也不要放进任何项目目录。它不在插件目录里，所以换版本不会丢。
- **不建议用公用机器人**：为保证对话私密性，请自己创建测试企业和机器人来进行使用。
- 建议 `allow_from` 填自己的 userId；`/whoami` 可以查到。
- 建议 `work_dir` 指向专用工作目录，别指到家目录或系统盘根目录。
- 演示时用 `mode = "default"`；`yolo` 会跳过 CLI 的权限提示。
- 把 `<work_dir>/.cf-connect/` 加进项目的 `.gitignore`。
