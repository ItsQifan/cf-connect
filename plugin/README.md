# CF-Connect 钉钉桥接插件（cf-connect-dingtalk v0.1.0）

## 这是什么

连接 **CodeFree-O 与钉钉**的插件：在手机钉钉里派活，本机 codefree-o 干活，结果流式回推钉钉。

```
钉钉 App ──Stream 长连接──▶ cf-connect.exe ──CLI + NDJSON──▶ codefree-o
   ▲                              │
   └──── 流式卡片 / 完成通知 ◀────┘
```

## 为什么用它

- **工作到一半该吃饭了，不慌** —— 转到钉钉继续干活，电脑锁屏也支持
- **支持 codefree-o 与钉钉共享会话** —— 意味着可以随意切换对话位置
- **零公网 IP** —— 走钉钉 Stream 长连接，不需要域名、不需要内网穿透
- **过程可见**（配了 AI 卡片后）—— 思考块、工具调用、结果实时回显
- **解压即用** —— 不需要 Go / Node / Python / Java

## 如何安装 / 卸载

1. 下载 `cf-connect-dingtalk-plugin-0.1.0.tgz`，**解压到一个目录**（建议英文路径，如 `D:\tools\cf-connect-dingtalk\`）
2. 在这个目录里打开 codefree-o，直接对话：

```
帮我安装 cf-connect-dingtalk
```

卸载也是同一句话：

```
帮我卸载 cf-connect-dingtalk
```

> 安装靠包内自带的 `cf-connect-setup` skill 自动完成。遇到**只能由你提供**的信息
> （钉钉凭证、工作目录）它会停下来问你，并给出获取步骤。

### 装完还需要一个钉钉企业内部应用

机器人要用**你自己的**钉钉应用凭证，这一步插件替不了你：

1. 打开 [钉钉开放平台](https://open-dev.dingtalk.com/) → 应用开发 → **企业内部应用** → 创建应用
2. 记下 **AppKey** 与 **AppSecret**
3. 左侧「机器人」→ **消息接收模式选 `Stream`**（这样本机不用公网 IP）
4. 「权限管理」勾选机器人发送消息、接收消息相关权限
5. **发布应用**（没发布，消息进不来）

> ⚠️ 只解压、没配凭证时，`cf-connect.exe` 起不来或收不到消息——这是正常的，把凭证填进
> `config.toml` 再启动即可。图文步骤见同目录的 `QUICKSTART.md`。

## 如何简单启动 / 关闭

- **启动**：安装成功后，用 PowerShell 打开 `package\bin` 目录，执行 `cf-connect.exe`
  （把 `bin` 加进 PATH 后，任意目录直接敲 `cf-connect` 也行）
- **关闭**：直接关掉那个 PowerShell 窗口

想后台常驻（关窗口也不停）：

```powershell
cf-connect daemon install
cf-connect daemon start
```

## 配置（config.toml）

配置样例里每个字段都有中文注释，**通常只需要改这 4 处**：

| 字段 | 作用 |
|---|---|
| `work_dir` | codefree-o 读写代码的目录 |
| `cmd` | 要驱动的 CLI（`codefree-o` 或绝对路径） |
| `client_id` / `client_secret` | 钉钉企业内部应用凭证（AppKey / AppSecret） |
| `allow_from` | 谁可以跟机器人对话（填你自己的 userId） |

### 怎么拿到自己的 userId

凭证填好、服务启动后，在钉钉里对机器人发：

```
/whoami
```

正常情况下会输出你的个人信息，其中包含 **userId** —— 就是下面这两项要填的值：

```toml
allow_from = "你的 userId"        # 谁可以对话（强烈建议填，防止别人用你的额度）
admin_from = "你的 userId"        # 管理员的 userId（留空 = 管理命令对所有人都关闭）
```

## 常用命令（在钉钉里发）

| 命令 | 作用 |
|---|---|
| `/list` | 加载当前工作目录所有会话 |
| `/switch <会话标题>` 或 `/switch <序号>` | 切换会话 |
| `/dir` | 查看当前工作目录 |
| `/dir <绝对路径>` | 切换工作路径 |
| `/whoami` | 查看自己的 userId |
| `/new` | 开一个新会话 |

> **建议用会话标题（或会话 ID 前缀）来 `/switch`，尽量别用序号** —— 序号是列表里的位置，
> 列表一变就会错位。
>
> 同一时间**不要在终端和钉钉两边同时聊同一个会话**，消息可能交错。

## Q&A

**Q：每次切换不同工作空间都需要手动改一次 `work_dir` 么？**
A：不需要，直接在钉钉用命令 `/dir <绝对路径>` 切换工作路径。

**Q：我在终端里聊到一半，能在钉钉接着聊吗？**
A：可以。钉钉里发 `/list` 找到那个会话，再 `/switch <标题>`；反过来也一样。
（切换前先把终端那侧退出，避免两边同时写同一个会话。）

**Q：回复为什么不是一句一句出来的？**
A：没配 AI 卡片流式。想看到实时过程：钉钉开放平台 → **卡片平台** → 新建「AI 卡片」模板，
放一个绑定变量 `content` 的 Markdown 组件并发布，把模板 ID 填进 `config.toml` 的
`card_template_id`。不配也能用，只是整轮跑完一次性到达。

**Q：机器人能看到我电脑上别的项目吗？**
A：只在 `work_dir`（或 `/dir` 切过去的目录）里干活，其它目录它看不到。

**Q：安全吗？**
A：钉钉凭证只存在**你本机**的 `~/.cf-connect/config.toml`；建议 `allow_from` 填自己的 userId；
`work_dir` 指向专用工作目录，别指到家目录或系统盘根目录；演示时 `mode` 用 `default`
（`yolo` 会跳过 CLI 的权限提示）。

## 排障

日志位置：

| 文件 | 内容 |
|---|---|
| `~/.cf-connect/plugin.log` | 插件（启动、回推、告警）的记录 |
| `~/.cf-connect/` 下的网关日志 | Stream 连接、会话、错误 |
| 钉钉应用后台 | 机器人收发的原始回执 |

| 现象 / 报错 | 原因 | 处置 |
|---|---|---|
| 钉钉里没收到消息 | 应用没开 Stream 模式 / 没发布 / 没加机器人 | 回「装完还需要一个钉钉企业内部应用」第 3、5 步 |
| `cf-connect is not running (socket not found)` | 网关没启动 | `cf-connect daemon start`，或前台直接跑 `cf-connect.exe` 看日志 |
| `unknown flag: --dangerously-skip-permissions` | `yolo` 给老版本 CLI 发了不支持的标志 | 配 `permission_flag = "--auto"`（默认值） |
| 回复慢、没有过程 | 没配 AI 卡片流式 | 配 `card_template_id`（见 Q&A） |
| 会话标题 / 消息数显示为空 | 数据目录没识别对 | 配 `data_dir` / `db_file`（见 config.toml 高级段） |
| 钉钉凭证报错 | AppKey / AppSecret 不对 | 核对 `client_id` / `client_secret`（别贴进聊天） |
| `cf-connect doctor` 报 not supported | Windows 上不支持该命令 | 改用 `cf-connect --version` + `cf-connect daemon status` |
| 中文 / 空格路径下行为异常 | 第三方 CLI 解析问题 | 换到英文路径，如 `D:\tools\cf-connect\` |
| SmartScreen 拦截 exe | 未签名 | 「更多信息 → 仍要运行」；企业内可统一签名 |

## 卸载 / 回滚

```powershell
cf-connect daemon stop          # 停服务
cf-connect daemon uninstall     # 删服务（保留数据与配置）
```

- 从 PATH 移除（装过 `install.ps1` 的话）：`powershell -ExecutionPolicy Bypass -File .\install.ps1 -Uninstall`
- 删数据目录（**会丢掉会话历史、定时任务、web token**）：删除 `%USERPROFILE%\.cf-connect`
- 插件本身：在解压目录里对 codefree-o 说「帮我卸载 cf-connect-dingtalk」
- ⚠️ **钉钉那边的应用不会自动解绑**，要不要删由你去钉钉开放平台决定

插件不改动 `codefree-o` 的任何原生文件：注入的 skills / commands 只在加载期生效，
删掉插件目录即完全回滚。

## 版本与许可

- 插件版本 **0.1.0**（`plugin.json` = `package.json` = tgz 文件名）
- 网关 `cf-connect` 版本 **v0.1.0**
- MIT
