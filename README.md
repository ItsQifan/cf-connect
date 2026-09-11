<p align="center">
  <img src="./docs/images/banner.svg" alt="CF-Connect Banner" width="800"/>
</p>

<p align="center">
  把 <b>CodeFree-O</b> 从"必须坐在电脑前用"变成"<b>在钉钉里随时用</b>"。
</p>

<p align="center">
  <a href="./README.md">English</a> | <a href="./README.zh-CN.md">中文</a>
</p>

---

## 这是什么

CF-Connect 是一个 **IM 通道网关**：把钉钉消息转成对本地 CodeFree-O 进程的调用，再把流式结果推回钉钉。

```
钉钉（手机/桌面）
      │  DingTalk Stream 长连接（不需要公网 IP）
      ▼
  cf-connect            ← 本程序
      │  子进程 + NDJSON 事件流
      ▼
  codefree-o / opencode  ← 在你本机真的改代码
```

它是什么：

- ✅ 一个 **钉钉 ↔ 本地智能体** 的桥
- ✅ 手机钉钉里就能派活、看过程、收结果
- ✅ 单文件静态二进制，**解压即用**

它不是什么：

- ❌ 不是 MCP server，也不是 CodeFree-O 插件
- ❌ 不提供模型，模型额度走你自己的 CodeFree-O 登录

## 亮点

| | |
|---|---|
| **零公网 IP** | 走钉钉 Stream 长连接，不需要域名、不需要配回调地址 |
| **过程可见** | 思考块、工具调用、结果实时流式回显到钉钉 |
| **权限可控** | `default`（逐次确认）/ `yolo`（全自动）可配 |
| **可复制** | 一个 zip 解压即用 + 一份配置模板 |
| **可审计** | 会话历史、管理 API、定时任务 |
| **零运行时依赖** | Go 静态编译 + Web 管理界面已内嵌，不需要 Go/Node/Python/Java |

---

## 3 步上手

### 1. 解压

解压 `cf-connect-vX-windows-amd64.zip` 到**英文路径**（例如 `D:\tools\cf-connect\`）。
可选：执行包内 `install.ps1` 把它加入用户 PATH。

### 2. 建钉钉应用（Stream 模式）

[钉钉开放平台](https://open-dev.dingtalk.com/) → 企业内部应用 → 创建应用 →
添加「机器人」能力 → **消息接收模式选 Stream** → 记下 `Client ID` / `Client Secret`。

### 3. 配置并运行

```powershell
copy config.example.toml config.toml
notepad config.toml     # 填 work_dir、cmd 和钉钉凭证
.\cf-connect.exe        # 前台运行；验证通过后 daemon install 常驻
```

📖 **完整图文步骤、排障表、常用命令见 [QUICKSTART.md](./QUICKSTART.md)。**

---

## 使用者需要准备什么

| 角色 | 需要 | 不需要 |
|---|---|---|
| 同事（使用者） | ① 解压 zip ② 自己的钉钉应用凭证 ③ 已安装并登录的 CodeFree-O ④ 填 `config.toml` | **Go、Node、Python、Java、npm 全都不需要** |
| 构建者 | Go 1.25+、Node + pnpm（仅用于构建发行包） | — |

每人一套钉钉凭证、各自登录模型，互不影响。建议在配置里设 `allow_from = "自己的 userid"`（钉钉里发 `/whoami` 可查），避免他人误用你的额度。

---

## 支持范围

这是一个**单通道**发行版，刻意只保留一条链路：

| | 支持 |
|---|---|
| **平台** | 钉钉（DingTalk，Stream 模式） |
| **智能体** | CodeFree-O、OpenCode（同一个适配器，按二进制名区分） |

上游 [cc-connect](https://github.com/chenhg5/cc-connect) 支持 20+ 平台与 12+ agent；
本项目在其基础上裁剪并为 CodeFree-O 做了兼容适配。

### 已适配 CodeFree-O 的点

| 问题 | 处理 |
|---|---|
| `yolo` 模式硬编码 `--dangerously-skip-permissions`，新 CLI 不认 | 改为可配置 `permission_flag`，默认 `--auto` |
| 会话标题/消息数固定读 `~/.local/share/opencode/opencode.db` | 按二进制名识别品牌，读 `~/.codefree-o/.local/share/codefree.db` |
| 读数据库依赖外部 `sqlite3` 命令（多数机器没有） | 改为**进程内纯 Go sqlite**，开箱可用 |
| 全局记忆文件固定 `~/.opencode/OPENCODE.md` | 按品牌探测 `~/.codefree-o/.config/{OPENCODE,AGENTS}.md` |
| `doctor` 写死 CLI 名 | 实现 `AgentDoctorInfo`，显示真实 CLI |

---

## 升级

自更新已移除，升级 = **换新 zip**：

```powershell
cf-connect daemon stop
# 用新 zip 覆盖解压到同一目录（config.toml 不会被动）
cf-connect daemon start
```

---

## 从源码构建

```bash
# 需要 Go 1.25+ 与 Node/pnpm（Web 管理界面会被 go:embed 进二进制）
cd web && pnpm install && pnpm build && cd ..
go build -o cf-connect ./cmd/cf-connect

# 打包全部平台的压缩包（含 config.example.toml / QUICKSTART.md / install.ps1）
make release-all
```

构建标签（本项目已只含一个平台一个 agent，标签主要为扩展留口）：

```bash
go build -tags 'no_web' ./cmd/cf-connect        # 不带 Web 管理界面
```

---

## 文档

| 文档 | 内容 |
|---|---|
| [QUICKSTART.md](./QUICKSTART.md) | 3 步上手 + 排障（**随 zip 分发**） |
| [INSTALL.md](./INSTALL.md) | 安装与部署细节 |
| [docs/dingtalk.md](./docs/dingtalk.md) | 钉钉适配器（卡片、媒体、引用） |
| [docs/usage.md](./docs/usage.md) | 钉钉里的命令用法 |
| [docs/management-api.md](./docs/management-api.md) | 管理 API |
| [docs/bridge-protocol.md](./docs/bridge-protocol.md) | Bridge 协议 |

---

## 开发

见 [AGENTS.md](./AGENTS.md)（架构约定、测试要求、如何新增平台/agent）。

```bash
go build ./...                  # 构建
go test ./...                   # 全部测试
go test ./core -run TestCUJ     # 用户视角端到端场景
cd web && pnpm build            # 前端（会写入 web/dist，go:embed 需要它）
```

---

## 许可

MIT。基于 [cc-connect](https://github.com/chenhg5/cc-connect) 二次开发。
