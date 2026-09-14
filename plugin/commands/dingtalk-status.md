---
description: 检查 cf-connect ↔ 钉钉桥接是否正常工作
---

检查本机钉钉桥接链路，逐条执行并把原始输出贴给用户：

1. `cf-connect --version`
2. `cf-connect config format --config "%USERPROFILE%\.cf-connect\config.toml"` —— 确认配置能解析
   （⚠️ **不要用 `cf-connect doctor`：Windows 上不支持**，会直接返回 "doctor command is not supported on Windows"）
3. 宿主在不在：`codefree-o --version`
4. 看网关是否在跑：数据目录下有没有 `run/api.sock`
   （Windows：`dir %USERPROFILE%\.cf-connect\run`；类 Unix：`ls -l ~/.cf-connect/run`）
5. `cf-connect send -m "bridge self-check"` —— 只有这一条真的往钉钉发消息

按下面这张表给结论：

| 现象 | 结论 | 处置 |
|---|---|---|
| 第 1 条报 `command not found` | 二进制不在 PATH | 用绝对路径，或运行插件包里的 `install.ps1` 后**新开终端** |
| 第 2 条报解析错误 | `config.toml` 语法/键位不对 | 对照 `config.example.toml` 修；注意 `allow_from` 必须在 `[projects.platforms.options]` 下 |
| 第 3 条失败 | 宿主缺失/未登录 | 安装并登录 CodeFree，必要时 `codefree-o auth` |
| 第 4 条没有 `api.sock` | 网关没启动 | `cf-connect daemon start`（或前台直接跑 `cf-connect` 看日志） |
| 第 5 条报 socket not found | 同上（没跑起来） | 查 `~/.cf-connect/` 日志 |
| 第 5 条报凭据错误 | 钉钉应用凭证不对 | 检查 config.toml 里的 `client_id` / `client_secret` |
| 第 5 条返回成功但钉钉没收到 | 应用没开「Stream 模式」/没发布/`allow_from` 不含自己 | 钉钉开放平台 → 机器人 → 消息接收模式选 Stream，并发布应用 |

**验证成功的唯一标准：用户真的在钉钉里收到了这条消息。** 命令返回 0 不算。

最后给出「结论 + 下一步动作」两句话，不要罗列无关信息。
