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
5. （可选，用 AI 卡片流式回显才需要）
   **卡片平台** 里新建一个卡片模板，记下模板 ID 和内容字段名，
   对应配置里的 `card_template_id` / `card_template_key`。
   不配也能用：回复会以普通 markdown 消息发出。

6. **把自己的钉钉 userid 填进 `allow_from`**（推荐）
   先按第 3 步跑起来，在钉钉里对机器人发 `/whoami`，它会回你的 userid。
   然后把该 ID 填进 `config.toml` 的 `allow_from`，避免别人误用你的额度。

---

## 3. 写配置并运行

```powershell
cd D:\tools\cf-connect
copy config.example.toml config.toml
notepad config.toml
```

最少只需要改这几处：

```toml
[[projects]]
name = "my-project"

[projects.agent]
type = "opencode"

[projects.agent.options]
work_dir = "E:\\work\\my-project"    # 你希望它改代码的目录
cmd = "codefree-o"                   # 或填 codefree-o.exe 的绝对路径
mode = "default"                     # 先用 default，确认没问题再考虑 yolo

[[projects.platforms]]
type = "dingtalk"

[projects.platforms.options]
client_id = "第 2 步拿到的 AppKey"
client_secret = "第 2 步拿到的 AppSecret"
```

> **强烈建议 `cmd` 填绝对路径。** 后台服务（daemon）通常不继承你当前终端的 PATH。
> 用 `where codefree-o` 可以查到路径。

### 前台运行（先这样验证）

```powershell
.\cf-connect.exe
```

控制台出现启动日志后，去钉钉里给机器人发一条消息试试，例如：

```
你好，用一句话介绍你自己
```

能在钉钉里收到流式回显就说明通了。

### 常驻运行（验证通过后再装）

```powershell
.\cf-connect.exe daemon install    # 安装并启动后台服务
.\cf-connect.exe daemon status     # 查看状态
.\cf-connect.exe daemon logs -f    # 跟踪日志
```

其它常用命令：

```powershell
.\cf-connect.exe --version
.\cf-connect.exe doctor            # 自检：CLI、数据目录、权限
.\cf-connect.exe daemon stop
.\cf-connect.exe daemon uninstall
```

---

## 4. 排障

### 先跑自检

```powershell
.\cf-connect.exe doctor
```

它会检查 agent CLI 是否存在、数据目录是否可写、配置是否合法。

### 常见问题

| 现象 | 原因 / 处理 |
|---|---|
| 启动报 `"codefree-o" CLI not found in PATH` | `cmd` 没配或路径不对。用 `where codefree-o` 查绝对路径填进去 |
| 钉钉里发消息没有任何反应 | ① 机器人消息接收模式不是 **Stream** ② `client_id`/`client_secret` 填错 ③ 应用没发布 ④ 看日志：`cf-connect daemon logs -n 100` |
| 回复里没有会话标题 / 消息数 | 你的 CodeFree-O 数据目录不是默认位置。用 `codefree-o debug paths` 看 `data` 路径，把 `data_dir` 填进配置 |
| 会话列表是空的 | 该 `work_dir` 下还没跑过会话，先在钉钉里让机器人干点活 |
| 一直卡在"等待权限确认" | 正常行为（`mode = "default"`）。要么在钉钉里点确认，要么改成 `mode = "yolo"` |
| `yolo` 模式报 unknown flag | 你的 CLI 版本不认 `--auto`。在配置里设 `permission_flag = "--dangerously-skip-permissions"`（老版 opencode），或 `"none"` |
| 别人也能用我的机器人 | 填 `allow_from = "你的userid"`（`/whoami` 可查） |
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
| `/whoami` | 查看自己的 userid（用来配 `allow_from`） |
| `/new` | 开一个新会话 |
| `/list` | 列出会话 |
| `/switch <id>` | 切到某个会话 |
| `/history` | 看当前会话历史 |
| `/model` | 查看/切换模型 |
| `/stop` | 打断当前这一轮 |
| `/help` | 全部命令 |

---

## 5. 升级

自更新功能已移除，升级 = **换新压缩包**：

```powershell
cf-connect daemon stop
# 用新的 zip 覆盖解压到同一目录（config.toml 不会被覆盖）
cf-connect daemon start
cf-connect --version      # 确认版本
```

> 覆盖前建议先备份 `config.toml` 和 `~/.cf-connect/`。

---

## 6. 安全提醒

- `client_secret` 是凭证，不要提交到 git、不要发群里
- `work_dir` 建议指向一个你愿意让 agent 改动的目录，不要直接指向整个用户目录
- 首次使用建议 `mode = "default"`，确认行为符合预期后再考虑 `yolo`
- 把 `<你的 work_dir>\.cf-connect\` 加入项目的 `.gitignore`（会话状态/附件会写在这里）
