# CF-Connect 安装与部署

> 本文可直接喂给 AI 编程助手，让它帮你完成安装配置。

CF-Connect 把本机的 **CodeFree-O**（或 OpenCode）接到 **钉钉**：
钉钉里发消息 → 本机 agent 干活 → 流式结果回到钉钉。

GitHub: https://github.com/ItsQifan/cf-connect

> 第一次使用请先看 [QUICKSTART.md](./QUICKSTART.md)（3 步上手 + 排障表）。

---

## 1. 运行环境

### 使用者需要什么

| 需要 | 说明 |
|---|---|
| 本项目的发行包 | `cf-connect-vX-{os}-{arch}.zip`（Windows）/ `.tar.gz`（Linux/macOS） |
| 钉钉企业内部应用 | 每人一套，**Stream 模式**机器人（无需公网 IP、无需回调地址） |
| CodeFree-O CLI | 已安装并已登录；`codefree-o --version` 能正常输出 |

**不需要**：Go、Node、Python、Java、npm、sqlite3。

原因：Go 编译产物是静态单文件（`CGO_ENABLED=0`），Web 管理界面通过 `go:embed` 打进二进制；
会话标题/消息数使用进程内纯 Go sqlite 读取，不依赖外部 `sqlite3` 命令。

### 构建者需要什么

- Go 1.25 或更高
- Node 22 + pnpm（仅用于构建 Web 管理界面）

---

## 2. 安装

### 2.1 解压

解压到**英文路径**：

| 平台 | 建议路径 |
|---|---|
| Windows | `D:\tools\cf-connect\` |
| macOS / Linux | `/opt/cf-connect/` 或 `~/cf-connect/` |

> 中文路径或含空格的路径可能引发第三方 CLI 的解析问题，建议避免。

解压后应包含：

```
cf-connect(.exe)        主程序
config.example.toml     配置模板
QUICKSTART.md           快速上手
install.ps1             可选：加入用户 PATH（仅 Windows）
README.md
```

### 2.2 加入 PATH（可选，Windows）

```powershell
cd D:\tools\cf-connect
powershell -ExecutionPolicy Bypass -File .\install.ps1
```

只写 `HKCU`（不需要管理员权限）。撤销：

```powershell
powershell -ExecutionPolicy Bypass -File .\install.ps1 -Uninstall
```

**只有新开的终端**才会看到 PATH 变化。

### 2.3 未签名程序提示（Windows）

`cf-connect.exe` 未做代码签名，首次运行可能被 SmartScreen 或杀软拦截：

- 右键 exe → 属性 → 勾选「解除锁定」→ 确定
- 或在 SmartScreen 弹窗里「更多信息」→「仍要运行」
- 企业环境请走统一的签名/白名单流程

### 2.4 从源码构建

```bash
git clone https://github.com/ItsQifan/cf-connect
cd cf-connect

# Web 管理界面会被 go:embed 进二进制，必须先构建
cd web && pnpm install && pnpm build && cd ..

go build -o cf-connect ./cmd/cf-connect

# 打全部平台的发行包（自动附带 config.example.toml / QUICKSTART.md / install.ps1）
make release-all
```

不带 Web 管理界面：

```bash
go build -tags 'no_web' -o cf-connect ./cmd/cf-connect
```

---

## 3. 配置

```bash
cp config.example.toml config.toml
```

配置文件查找顺序：

1. `--config <path>` 显式指定
2. 当前目录的 `./config.toml`
3. `~/.cf-connect/config.toml`

最小可用配置：

```toml
[[projects]]
name = "my-project"

[projects.agent]
type = "opencode"          # 或写 "codefree-o"，同一个适配器

[projects.agent.options]
work_dir = "E:\\work\\my-project"
cmd = "codefree-o"          # 建议改成绝对路径（daemon 不继承终端 PATH）
mode = "default"

[[projects.platforms]]
type = "dingtalk"

[projects.platforms.options]
client_id = "你的 AppKey"
client_secret = "你的 AppSecret"
```

创建钉钉应用的完整步骤见 [QUICKSTART.md](./QUICKSTART.md)。

### 常用 agent 选项

| 选项 | 默认 | 说明 |
|---|---|---|
| `work_dir` | `.` | agent 改代码的目录 |
| `cmd` | `opencode` | CLI 名或绝对路径；数组形式可带额外参数 |
| `mode` | `default` | `default` 逐次确认；`yolo` 全自动 |
| `permission_flag` | `--auto` | `yolo` 追加的 flag。老版 opencode 可设 `--dangerously-skip-permissions`，或 `none` 表示不追加 |
| `model` | CLI 默认 | `provider/model`，用 `codefree-o models` 查 |
| `agent` | — | 传给 CLI 的 `--agent` |
| `data_dir` / `db_file` | 自动识别 | 会话数据库位置。codefree-o 自动读 `~/.codefree-o/.local/share/codefree.db`，opencode 读 `~/.local/share/opencode/opencode.db` |
| `global_memory_file` | 自动探测 | 全局指令文件；codefree-o 优先 `~/.codefree-o/.config/{OPENCODE,AGENTS}.md` |
| `env` | — | 传给 agent 进程的额外环境变量 |

### 常用钉钉选项

| 选项 | 说明 |
|---|---|
| `client_id` / `client_secret` | 应用凭证（必填） |
| `robot_code` | 与 client_id 不同时才需要 |
| `allow_from` | 允许的使用者（逗号分隔，`*` 表示全部） |
| `share_session_in_channel` | 群内共用一个会话（默认每人独立） |
| `card_template_id` / `card_template_key` | 启用 AI 卡片流式回显 |
| `card_throttle_ms` | 卡片更新节流 |
| `reaction_emoji` / `done_emoji` | 用表情回应代替文字确认 |

---

## 4. 运行与常驻

### 前台运行（先这样验证）

```bash
./cf-connect
```

### 常驻服务

```bash
cf-connect daemon install     # 安装并启动（systemd / launchd / schtasks）
cf-connect daemon status
cf-connect daemon logs -f
cf-connect daemon restart
cf-connect daemon stop
cf-connect daemon uninstall
```

日志轮转由服务管理器驱动（`CC_LOG_FILE` + `CC_LOG_MAX_SIZE` + `CC_LOG_MAX_BACKUPS`），
也可在启动时用 `--log-max-size` / `--log-max-backups` 覆盖。

### 验证安装

```bash
cf-connect --version              # 版本与构建信息
cf-connect config path            # 实际使用的配置文件路径
cf-connect doctor                 # 自检：agent CLI、数据目录、权限
cf-connect config example         # 打印完整配置模板
```

在钉钉里对机器人发 `/whoami` 可以拿到自己的 userid，用来配 `allow_from`。

---

## 5. 升级

自更新功能已移除，升级 = 换新压缩包：

```bash
cf-connect daemon stop
# 备份 config.toml 与 ~/.cf-connect/
# 用新压缩包覆盖解压到同一目录（压缩包内没有 config.toml，不会被覆盖）
cf-connect daemon start
cf-connect --version
```

---

## 6. 卸载

```bash
cf-connect daemon stop
cf-connect daemon uninstall
# 删除解压目录；如需彻底清理，再删数据目录：
#   Windows:     %USERPROFILE%\.cf-connect
#   macOS/Linux: ~/.cf-connect
```

若执行过 `install.ps1`，记得同时撤销 PATH 项（见 2.2）。

---

## 7. 多项目与多人部署

一个进程可以管理多个项目：重复 `[[projects]]` 块即可，每个项目有独立的 `work_dir`
与 agent 会话。同一份配置不会被重复启动（有实例锁保护），需要强杀旧实例时用 `--force`。

推荐落地方式：**每人部署自己的一套**（各自的钉钉应用凭证 + 各自的 CodeFree-O 登录），
互不影响、额度各算各的。

---

## 8. 故障排查

见 [QUICKSTART.md 的排障章节](./QUICKSTART.md#4-排障)，以及：

```bash
cf-connect doctor                # 一键自检
cf-connect daemon logs -n 200    # 最近的日志
```

---

## 9. 进一步阅读

| 文档 | 内容 |
|---|---|
| [QUICKSTART.md](./QUICKSTART.md) | 3 步上手 + 排障 |
| [docs/dingtalk.md](./docs/dingtalk.md) | 钉钉适配器（AI 卡片、媒体、引用） |
| [docs/usage.md](./docs/usage.md) | 命令与功能用法 |
| [docs/management-api.md](./docs/management-api.md) | 管理 API |
| [docs/bridge-protocol.md](./docs/bridge-protocol.md) | Bridge 协议 |
| [AGENTS.md](./AGENTS.md) | 开发约定 |
