<p align="center">
  <img src="./docs/images/banner.svg" alt="CF-Connect Banner" width="800"/>
</p>

<p align="center">
  <a href="./README.md">English</a> | <a href="./README.zh-CN.md">中文</a>
</p>

---

# CF-Connect

> 这份文档是 [README.md](./README.md) 的中文说明。本项目发行方式是 **zip 解压即用**，
> 主 README 与 [QUICKSTART.md](./QUICKSTART.md) 已是完整说明，因此这里不再重复维护
> 一份平行文档——上游的双语 README 会随上游功能演进，本项目已裁剪为单通道，
> 保留副本只会产生不一致。

## 一句话

把 **CodeFree-O** 从"必须坐在电脑前用"变成"**在钉钉里随时用**"。

```
钉钉 ──(Stream 长连接，无需公网 IP)──▶ cf-connect ──(子进程 + NDJSON)──▶ codefree-o
```

## 3 步上手（简述）

1. **解压** `cf-connect-vX-windows-amd64.zip` 到英文路径（如 `D:\tools\cf-connect\`），
   可选执行包内 `install.ps1` 加入 PATH
2. **建钉钉应用**：[钉钉开放平台](https://open-dev.dingtalk.com/) → 企业内部应用 →
   创建应用 → 添加「机器人」能力 → **消息接收模式选 Stream** → 记下 Client ID / Secret
3. **配置并运行**：`copy config.example.toml config.toml` → 填 `work_dir`、`cmd`、
   钉钉凭证 → `cf-connect.exe`（前台）或 `cf-connect.exe daemon install`（常驻）

📖 完整图文步骤、排障表、常用命令：**[QUICKSTART.md](./QUICKSTART.md)**

## 使用者需要准备什么

| 角色 | 需要 | 不需要 |
|---|---|---|
| 同事（使用者） | ① 解压 zip ② 自己的钉钉应用凭证 ③ 已安装并登录的 CodeFree-O ④ 填 `config.toml` | **Go、Node、Python、Java、npm 全都不需要** |
| 构建者 | Go 1.25+、Node + pnpm（仅用于构建发行包） | — |

每人一套钉钉凭证、各自登录模型，互不影响。建议设 `allow_from = "自己的 userid"`
（钉钉里发 `/whoami` 可查），避免他人误用你的额度。

## 支持范围

| | 支持 |
|---|---|
| **平台** | 钉钉（DingTalk，Stream 模式） |
| **智能体** | CodeFree-O、OpenCode（同一个适配器，按二进制名区分） |

上游 [cc-connect](https://github.com/chenhg5/cc-connect) 支持 20+ 平台与 12+ agent；
本项目在其基础上裁剪，并为 CodeFree-O 做了兼容适配。

## 快速索引

| 你想做的事 | 看这里 |
|---|---|
| 了解它是什么、亮点、已适配 CodeFree-O 的点 | [README.md](./README.md) |
| 3 步跑起来 | [QUICKSTART.md](./QUICKSTART.md) |
| 安装与部署细节 | [INSTALL.md](./INSTALL.md) |
| 钉钉里能用哪些命令 | [docs/usage.md](./docs/usage.md) |
| 钉钉适配器细节（卡片/媒体/引用） | [docs/dingtalk.md](./docs/dingtalk.md) |
| 管理 API | [docs/management-api.md](./docs/management-api.md) |
| 参与开发 | [AGENTS.md](./AGENTS.md) |

## 许可

MIT。基于 [cc-connect](https://github.com/chenhg5/cc-connect) 二次开发。
