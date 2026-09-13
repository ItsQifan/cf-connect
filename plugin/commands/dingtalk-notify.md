---
description: 把一段文字推送到钉钉（经本机 cf-connect 网关）
---

把下面的内容推送到钉钉当前会话：

$ARGUMENTS

执行步骤：

1. 如果 `$ARGUMENTS` 为空，先问用户要推送什么，不要自己编内容。
2. 用标准输入推送，避免引号/换行被 shell 解释：

   ```bash
   cf-connect send --stdin <<'EOF'
   <内容>
   EOF
   ```

3. 命令成功（退出码 0，输出 `Message sent successfully.`）就回一句"已推送到钉钉"。
4. 如果报 `cf-connect is not running (socket not found: ...)`，说明网关没启动或
   没有活跃会话 —— 把内容直接回复给用户，并提示可用
   `cf-connect daemon start` 启动网关，**不要反复重试**。
