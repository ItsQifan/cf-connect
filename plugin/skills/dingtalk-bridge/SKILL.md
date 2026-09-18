---
name: dingtalk-bridge
description: 当用户提到"钉钉"、"推送到钉钉"、"通知我"、"在群里说一声"、"手机上派活"，或需要把长任务的进展/结果发回钉钉会话时使用。也用于排查 cf-connect ↔ codefree-o 的桥接问题（网关是否在跑、会话是否通、日志在哪）。
---

# 钉钉桥接（dingtalk-bridge）

这套能力的结构很简单：**本机 cf-connect 网关负责"钉钉 ↔ codefree-o"的长连接，
这个 skill 只教你在什么时候、用什么命令去用它。**

```
钉钉 App  ──Stream 长连接──▶  cf-connect.exe  ──CLI/NDJSON──▶  codefree-o
   ▲                              │
   └──── 流式卡片 / 完成通知 ◀────┘
```

## 什么时候用

| 场景 | 做法 |
|---|---|
| 用户明确要求"发到钉钉 / 通知我 / 群里说一声" | `cf-connect send -m "<内容>"` |
| 任务会跑很久（> 2 分钟），中途有阶段性结论 | 每个阶段结束推一条简短进展 |
| 需要用户决策才能继续（选方案、确认破坏性操作） | 推送问题 + 可选项，然后停下等回复 |
| 排查桥接问题 | 依次执行下方「自检」的三条命令 |

## 怎么用

推送文本（推荐用 `--stdin`，避免引号和换行被 shell 吃掉）：

```bash
cf-connect send --stdin <<'EOF'
**构建完成** ✅
- 用例：128 passed / 0 failed
- 产物：dist/cf-connect-v0.1.1-windows-amd64.zip
EOF
```

常用参数：

| 参数 | 说明 |
|---|---|
| `-m, --message <text>` | 直接给文本（短消息够用） |
| `--stdin` | 从标准输入读（长文本/含特殊字符时首选） |
| `-p, --project <name>` | 指定 cf-connect 项目（只有一个项目时可省略） |
| `-s, --session <key>` | 指定会话 key（省略则发给当前活跃会话） |
| `--file <path>` / `--image <path>` | 附带文件 / 图片 |
| `--at-users <id,id>` / `--at-all` | 钉钉 @ 某人 / @ 所有人 |

> 推送目标是"当前活跃的钉钉会话"。**没有活跃会话时 `send` 会报
> `cf-connect is not running (socket not found: ...)`**，这时不要重试，
> 直接把内容留在回答里即可。

## 自检（排障三步）

```bash
cf-connect --version                  # 1. 二进制在不在、什么版本
cf-connect doctor                     # 2. 依赖检查（会显示真实的 CLI 名字）
cf-connect send -m "bridge ping"      # 3. 能不能真的推到钉钉
```

日志位置：

| 文件 | 内容 |
|---|---|
| `~/.cf-connect/plugin.log` | 本插件（启动、回推、告警）的记录 |
| `~/.cf-connect/` | 会话历史、项目状态、`run/api.sock` |
| 钉钉应用后台 | 机器人收发的原始回执 |

## 硬性注意

> 还没装 cf-connect、或钉钉那边没配好？先看同包里的 **`cf-connect-setup`** skill
> （找 release 目录 / 落 PATH / 引导拿钉钉凭证 / 连通性验证 / 卸载升级）。

1. **不要自己实现钉钉协议**。Stream 长连接、卡片流式、消息路由都在 Go 二进制里，
   你只需要调用 `cf-connect send`。
2. **推送要克制**：一个任务默认只推「开始 + 结束」两条，除非用户要求更细。
3. **失败不重试**：`send` 非 0 退出说明网关没跑或没有活跃会话，重试也只是同样的报错。
4. **不要推密钥**：钉钉 `client_secret`、模型 token 一律不出现在推送内容里。
