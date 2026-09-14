# CF-Connect 三步上手（钉钉 ↔ CodeFree）

> 配套插件：`cf-connect-dingtalk` v0.1.0 ｜ 网关：`cf-connect.exe` v0.1.0

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

## 第 2 步：写 config.toml

```powershell
copy config.example.toml config.toml
notepad config.toml
```

只需要改这 4 处：

```toml
[projects.agent.options]
work_dir = "D:\\workspace_idea"    # ① codefree-o 读写代码的目录
cmd = "codefree-o"                 # ② 要驱动的 CLI（写绝对路径最稳）

[projects.platforms.options]
client_id = "你的 AppKey"           # ③
client_secret = "你的 AppSecret"    # ④
# allow_from = "你的 userId"        # 建议填上
```

---

## 第 3 步：跑起来

```powershell
# 前台跑（推荐第一次这样，能直接看到日志）
cf-connect

# 常驻（后台服务）
cf-connect daemon install
cf-connect daemon start
```

看到日志里出现 Stream 连接成功，就可以在钉钉里发消息了：

```
帮我看下 D:\workspace_idea\demo 这个项目的编译错误并修掉
```

---

## 自检清单

| 检查 | 命令 | 期望 |
|---|---|---|
| 网关在跑 | `cf-connect daemon status` + `dir %USERPROFILE%\.cf-connect\run` | 服务在跑，且目录里有 `api.sock`（⚠️ Windows 上 `cf-connect doctor` 不支持，别用它） |
| 能发消息到钉钉 | `cf-connect send -m "bridge ping"` | 钉钉里收到 `bridge ping` |
| 插件已注入 | `codefree-o debug skill` | 列表里有 `cf-connect-setup` 和 `dingtalk-bridge` |
| 我的 userId | 钉钉里发 `/whoami` | 回复一串 userId |

## 懒人模式：让 codefree-o 自己装

装上本插件后，用户机器上的 codefree-o 会获得 `cf-connect-setup` skill，
上面这一整套（找 release 目录 → 落 PATH → 写 config.toml → 引导拿钉钉凭证 → 验证）
都可以直接在 codefree-o 对话框里一句话交给 agent：

```
安装 cf-connect
测一下钉钉连通性
卸载 cf-connect
```

agent 会自动做完能自动做的部分，只在**必须用户提供**的地方停下来问你
（钉钉 AppKey/AppSecret、`work_dir`、是否加白名单），并给出获取步骤。

## 常见问题

| 现象 | 原因 | 处置 |
|---|---|---|
| 钉钉发消息没反应 | 应用没发布 / 不是 Stream 模式 / 机器人没启用 | 回第 1 步 3、5 项 |
| `socket not found` | 网关没启动 | `cf-connect daemon start` |
| `unknown flag: --dangerously-skip-permissions` | yolo 模式给老版本发的标志不对 | `permission_flag = "--auto"`（默认值） |
| 回复慢、没有过程 | 没配 AI 卡片流式 | 配 `card_template_id`，见第 1 步可选段 |
| 会话标题/消息数空 | 数据目录没识别对 | 配 `data_dir` / `db_file`（见 config.toml 高级段） |
| SmartScreen 拦截 exe | 未签名 | 「更多信息 → 仍要运行」 |

## 安全提醒

- 钉钉 `client_secret` 是**每人一套**的本机凭据，不要提交到 git、不要发群里。
- 建议 `allow_from` 填自己的 userId；`/whoami` 可以查到。
- 建议 `work_dir` 指向专用工作目录，别指到家目录或系统盘根目录。
- 演示时用 `mode = "default"`；`yolo` 会跳过 CLI 的权限提示。
- 把 `<work_dir>/.cf-connect/` 加进项目的 `.gitignore`。
