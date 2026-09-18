# CF-Connect 钉钉桥接插件（cf-connect-dingtalk v0.1.1）

## 这是什么

把 **CodeFree-O 接进钉钉**的插件：在手机钉钉里派活，本机 codefree-o 干活，过程和结果流式回推钉钉。

```
钉钉 App ──Stream 长连接──▶ cf-connect.exe ──CLI + NDJSON──▶ codefree-o
   ▲                              │
   └──── 流式卡片 / 完成通知 ◀────┘
```

> 目前只提供 **Windows x64** 版本（包内 `bin\cf-connect.exe` 为 windows-amd64）。

## 为什么用它

- **工作到一半该吃饭了，不慌** —— 转到钉钉继续干活，电脑锁屏也支持
- **codefree-o 与钉钉共享会话** —— 随时切换对话位置
- **零公网 IP** —— 走钉钉 Stream 长连接，不需要域名、不需要内网穿透
- **过程可见**（配了 AI 卡片后）—— 思考块、工具调用、结果实时回显
- **解压即用** —— 不需要 Go / Node / Python / Java

## 如何安装

1. 在 codefree-o 里打开插件市场：

   ```
   /plugins-market
   ```

2. 安装 `cf-connect-dingtalk`，然后**重启 codefree-o**（skill 只在启动时注入）。
3. 对它说一句话，剩下的按引导走：

   ```
   帮我配置 cf-connect-dingtalk
   ```

> 配置由包内自带的 `cf-connect-setup` skill 自动完成：它会触发 `package\bin\cf-connect.exe`
> 自动生成 `config.toml`、用 `daemon install` **把网关装成后台服务**，再引导你在钉钉里发
> `/whoami`，拿到 userId 后回填 `allow_from` / `admin_from` / `work_dir`。

### 装完还需要一个钉钉企业内部应用

机器人要用**你自己的**钉钉应用凭证，这一步插件替不了你：

1. 打开 [钉钉开放平台](https://open-dev.dingtalk.com/) → 应用开发 → **企业内部应用** → 创建应用
2. 记下 **AppKey** 与 **AppSecret**
3. 左侧「机器人」→ **消息接收模式选 `Stream`**（这样本机不用公网 IP）
4. 「权限管理」勾选机器人发送消息、接收消息相关权限
5. **发布应用**（没发布，消息进不来）

> ⚠️【为保证对话私密性，不建议公用机器人。请自己创建测试企业和机器人来进行使用】
> 多人共用一个机器人时，消息会进同一个网关，会话可能互相接管。

## 如何卸载

```
帮我卸载 cf-connect-dingtalk
```

## 如何启动 / 关闭

配置完成后，网关以**后台服务**形式运行（Windows 计划任务），**登录后自动启动**，关掉终端也不停。

```powershell
cf-connect daemon status      # 查看是否在跑（期望 running）
cf-connect daemon stop        # 临时停掉
cf-connect daemon restart     # 改完配置后重启生效
cf-connect daemon uninstall   # 彻底移除后台服务
```

> 只想临时看日志时，可以在 `package\bin` 里直接跑 `cf-connect.exe` —— 那只是**前台调试**，
> 命令一结束进程就没了，不算启动。

## 配置（config.toml）

配置文件就放在 **`%USERPROFILE%\.cf-connect\config.toml`**（和会话历史 / 日志 / `api.sock` 同一个目录，
后台服务的工作目录也是它）。codefree-o 会协助你填好，字段含义见包内的 `config.example.toml`。
改完记得 `cf-connect daemon restart`。

> 为什么不在 exe 旁边：插件目录名带版本号，配置放那儿的话**每次升级都会连同旧目录一起被删掉**（钉钉凭证也就丢了）。
> 放在用户目录里，换插件版本、删插件目录都不影响它。
>
> ⚠️ 注意 exe 的配置查找顺序是 `--config` > 当前目录 `./config.toml` > `~/.cf-connect/config.toml`，
> 所以生成/校验/装服务时都显式带 `--config`，别让某个工作目录里的 `config.toml` 把真正那份顶掉。

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

**Q：配置文件在哪？**
A：`%USERPROFILE%\.cf-connect\config.toml`。会话历史、日志、`api.sock` 也都在这个目录里。
插件本身（`cf-connect.exe`）则在插件目录 / 解压目录里，两者是分开的。

**Q：为什么配置放在 `~/.cf-connect` 而不是 exe 旁边？**
A：插件目录名带版本号，配置放 exe 旁边的话每次升级都要先手动搬出来再删旧目录，容易丢凭证。
放在用户目录里，升级/换解压位置都不动配置，而且排障时"状态"全在同一个目录。

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
A：钉钉凭证只存在**你本机**的 `%USERPROFILE%\.cf-connect\config.toml`（不要提交进 git、不要外发、别放进项目目录）；
建议 `allow_from` 填自己的 userId；
`work_dir` 指向专用工作目录，别指到家目录或系统盘根目录；演示时 `mode` 用 `default`
（`yolo` 会跳过 CLI 的权限提示）。

## 排障

日志位置：

| 文件 | 内容 |
|---|---|
| `~/.cf-connect/logs/cf-connect.log` | 网关（后台服务）的日志：Stream 连接、会话、错误 |
| `~/.cf-connect/plugin.log` | 插件（启动、回推、告警）的记录 |
| 钉钉应用后台 | 机器人收发的原始回执 |

| 现象 / 报错 | 原因 | 处置 |
|---|---|---|
| 钉钉里没收到消息 | 应用没开 Stream 模式 / 没发布 / 没加机器人 | 回「装完还需要一个钉钉企业内部应用」第 3、5 步 |
| `cf-connect is not running (socket not found)` | 网关没起来（服务没装成，或被停掉了） | `cf-connect daemon status` 确认；没装过就跑 `cf-connect daemon install --config "%USERPROFILE%\.cf-connect\config.toml"`，已装过用 `daemon restart` |
| 日志里出现过 `stream connected` / `platform ready`，但钉钉里发消息没反应 | 那是前台调试留下的日志，进程随后就结束了 —— 网关并未常驻 | 改成后台服务：`cf-connect daemon install --config "%USERPROFILE%\.cf-connect\config.toml"` |
| `unknown flag: --dangerously-skip-permissions` | `yolo` 给老版本 CLI 发了不支持的标志 | 配 `permission_flag = "--auto"`（默认值） |
| 回复慢、没有过程 | 没配 AI 卡片流式 | 配 `card_template_id`（见 Q&A） |
| 会话标题 / 消息数显示为空 | 数据目录没识别对 | 配 `data_dir` / `db_file`（见 config.toml 高级段） |
| 钉钉凭证报错 | AppKey / AppSecret 不对 | 核对 `client_id` / `client_secret`（别贴进聊天） |
| 跑 `cf-connect.exe` 一下就退出，提示 `Created default config at ...<路径>` | **正常**：当时还没有 `config.toml`，它写完配置就退出 | 路径应为 `%USERPROFILE%\.cf-connect\config.toml`；写到别处多半是当前目录有一份 `config.toml` 被优先命中。之后用 `cf-connect daemon install --config "%USERPROFILE%\.cf-connect\config.toml"` |
| 升级插件后发现 `config.toml` 没了 | 那是 0.1.0 的老行为（配置在带版本号的插件目录里） | 0.1.1 起配置固定在 `%USERPROFILE%\.cf-connect\config.toml`，升级不再丢；老包升级时把旧 `package\bin\config.toml` 复制到新位置即可 |
| `cf-connect doctor` 报 not supported | Windows 上不支持该命令 | 改用 `cf-connect --version` + `Get-Process cf-connect` / `~/.cf-connect/run/api.sock` |
| 敲 `cf-connect` 说"无法将"cf-connect"项识别为…" | `bin\` 不在**用户 PATH** 里，或那条指向的是已删除的旧版本目录；也可能只是终端开在改 PATH 之前 | 重跑 `install.ps1` 后**新开**终端；`where cf-connect` 只反映当前进程，判断不了注册表 |
| 跑 `install.ps1` 报 `字符串缺少终止符: '` | 老包里的 .ps1 是 UTF-8 无 BOM + 中文注释，PowerShell 5.1 按 GBK 解码吞掉引号 | 用 `pwsh -File .\install.ps1`，或手工把 `bin\` 加进用户 PATH；新版打包已补 BOM |
| 服务在跑但钉钉里第一句话报 `"..." CLI not found in PATH` | `cmd` 写成裸名字 `codefree-o` 或 `.ps1`（后台服务解析不到） | 换成宿主 `.exe` / `.cmd` 的绝对路径 → `cf-connect daemon restart` |
| 中文 / 空格路径下行为异常 | 第三方 CLI 解析问题 | 换到英文路径，如 `D:\tools\cf-connect-dingtalk\` |
| SmartScreen 拦截 exe | 未签名 | 「更多信息 → 仍要运行」；企业内可统一签名 |

## 版本与许可

- 插件版本 **0.1.1**
- 网关 `cf-connect` 版本 **v0.1.1**
- 包名带平台标识：**`cf-connect-dingtalk-plugin-0.1.1-windows-amd64.tgz`**
  （只提供 Windows x64 版本；包内的 `bin\cf-connect.exe` 就是 `windows-amd64` 架构）
- MIT
