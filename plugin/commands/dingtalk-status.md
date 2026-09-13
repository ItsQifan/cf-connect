---
description: 检查 cf-connect ↔ 钉钉桥接是否正常工作
---

检查本机钉钉桥接链路，逐条执行并把原始输出贴给用户：

1. `cf-connect --version`
2. `cf-connect doctor`
3. 看网关是否在跑：数据目录下有没有 `run/api.sock`
   （Windows：`dir %USERPROFILE%\.cf-connect\run`；类 Unix：`ls -l ~/.cf-connect/run`）
4. `cf-connect send -m "bridge self-check"` —— 只有这一条真的往钉钉发消息

按下面这张表给结论：

| 现象 | 结论 | 处置 |
|---|---|---|
| 第 1 条报 `command not found` | 二进制不在 PATH | 用绝对路径，或运行插件包里的 `install.ps1` |
| 第 2 条显示 codefree-o 缺失 | 没装/没登录 CLI | `codefree-o --version` 确认，必要时 `codefree-o auth` |
| 第 3 条没有 `api.sock` | 网关没启动 | `cf-connect daemon start` |
| 第 4 条报 socket not found | 同上（没跑起来） | 查 `~/.cf-connect/` 日志 |
| 第 4 条报凭据错误 | 钉钉应用凭证不对 | 检查 config.toml 里的 `client_id` / `client_secret` |
| 全绿但钉钉里没反应 | 应用没开「Stream 模式」或没发布 | 钉钉开放平台 → 机器人 → 消息接收模式选 Stream |

最后给出「结论 + 下一步动作」两句话，不要罗列无关信息。
