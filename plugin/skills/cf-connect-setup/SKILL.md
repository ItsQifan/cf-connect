---
name: cf-connect
description: 把 CodeFree 接到钉钉上的桥接工具 cf-connect 的使用与运维。当用户说"装 cf-connect / 配置钉钉机器人 / 我在钉钉里跟你说话 / 手机钉钉派活 / 测一下连通性 / 卸载 cf-connect / 换个工作目录 / 为什么钉钉没反应 / cf-connect 命令找不到"时使用。负责：读懂解压目录 → 落 PATH（直接把 bin 目录写进 HKCU 用户 PATH）→ 把宿主 codefree-o 解析成能被 exec 拉起的绝对路径（.exe/.cmd，绝不能是 .ps1）并填进 cmd → 触发 bin\cf-connect.exe 生成 config.toml（**配置固定放在 `%USERPROFILE%\.cf-connect\config.toml`，和运行时数据同处一个目录，插件升级/换版本不会丢**）并填写（缺钉钉凭证时引导用户去开放平台获取）→ 用 daemon install 装成后台服务启动（不要用前台方式，前台等于没启动）→ 服务 running 之后拿到用户 /whoami 的 userId 再回填 allow_from / admin_from → 端到端验证 → 卸载/升级/排障。不负责：帮用户注册钉钉应用本体、替用户保管或外传 AppSecret。
---

# cf-connect（用户机器上的运维手册）

## 0. 先明白它是什么、不是什么

```
用户在钉钉里发消息 ──Stream 长连接──▶ cf-connect.exe ──子进程 + NDJSON──▶ codefree-o
        ▲                                    │
        └──────── 流式卡片 / 结果回推 ────────┘
```

- **是**：一个本机网关。它把钉钉消息变成对**本机 codefree-o** 的一次调用，再把结果推回钉钉。
- **不是**：不是模型、不是 MCP server、不提供算力。模型额度走用户自己的 codefree-o 登录。
- **不需要**：Go / Node / Python / Java / 公网 IP / 域名 / 内网穿透。走钉钉 Stream 模式，只出网、不监听公网。
- **它和你（agent）的关系**：当用户在**钉钉里**说话时，那侧是另一个 cf-connect 拉起来的会话；当用户在你这个终端里说话时，你能通过 `cf-connect send` 把消息推到钉钉。两边是**同一个 codefree-o 会话库**，用 `/list` + `/switch` 可以互相接管（见 §7）。

## 1. 你要做的事与你不该做的事

| 你能自动做 | 必须问用户（**按批次**，见 A0） | 永远不要做 |
|---|---|---|
| 找 exe 目录、读版本、**落 PATH（写 HKCU）**、**把宿主解析成绝对路径填 `cmd`**、触发 exe 生成配置 | **① 钉钉 AppKey / AppSecret** —— 只能用户自己去开放平台拿，没有它网关起不来 | ❌ 替用户注册钉钉应用（需要登录钉钉后台） |
| 写 `config.toml`、填 `cmd`、装后台服务（daemon install）、跑连通性验证 | **① 用哪个目录当 `work_dir`** —— 本地信息，一开始就能问 | ❌ 把 AppSecret 打进日志、提交进 git、发到任何对话里 |
| 检测 codefree-o 是否已安装/登录 | **② `allow_from` / `admin_from` 要填的 userId** —— ⚠️ 必须等网关 running、用户能在钉钉里跟机器人说话之后 | ❌ 在凭证没到手、服务没起来时就要 userId（拿不到，白问一轮） |
| 生成交接摘要、推送消息 | 是否要重启服务（重启会中断正在跑的会话）／是否要 AI 卡片流式 | ❌ 声称"验证通过"却没真的收到过一条钉钉消息<br>❌ 在用户没确认的情况下用 `mode = "yolo"`<br>❌ 改用户的全局 PATH 之外的系统设置、装驱动 |

**铁律**：`client_secret` 只能来自用户输入或已有的 `config.toml`，你只写进配置文件、只回显脱敏形式（`0qec…d88Z`）。

## 2. 先探测现状（任何操作前都先跑这一步）

```powershell
# 1) 有没有装、装在哪（⚠️ 这条只反映"当前进程继承的 PATH"；注册表改了这里不会变，
#    要判断"用户新开终端能不能敲到"必须读 HKCU，见 A2 的读回方式）
Get-Command cf-connect -ErrorAction SilentlyContinue | Select-Object Source
@("$env:USERPROFILE\.cf-connect\cf-connect.exe","D:\software\cf-connect-v0.1.1-windows-amd64\cf-connect.exe",
  "D:\software\cf-connect-v0.1.0-windows-amd64\cf-connect.exe",
  "D:\tools\cf-connect\cf-connect.exe") | ForEach-Object { if (Test-Path $_) { "FOUND: $_" } }

# 2) 有没有配置（本插件的固定位置：用户目录下的 .cf-connect，和运行时数据同一个目录）
Test-Path "$env:USERPROFILE\.cf-connect\config.toml"   # ★ 唯一位置
Test-Path .\config.toml                                # ⚠️ 当前目录下的 config.toml 优先级更高，会把它顶掉

# 3) 宿主在不在
codefree-o --version

# 4) 服务状态（正常形态是"后台服务"：daemon install 装的计划任务在跑）
Get-Process cf-connect -ErrorAction SilentlyContinue | Select-Object Id,ProcessName
Test-Path "$env:USERPROFILE\.cf-connect\run\api.sock"   # 存在 = 有实例在驻留（运行时数据仍在这里）
cf-connect daemon status                                # 期望 running
```

配置查找顺序（`main.go:1440`）：**`--config` 参数 > 当前目录 `config.toml` > `~/.cf-connect/config.toml`**。
三者都没有时，`cf-connect` 会在**自己启动的那一刻**写一份带占位符的样例到"解析出来的那个路径"，
然后立刻退出 —— 所以"配置不存在"这件事**只有真正去启动它才会被解决**。

> **本插件的约定：配置固定在 `%USERPROFILE%\.cf-connect\config.toml`**（也就是上面查找顺序里的默认回退位置）。
> 这么定的原因：① 配置和运行时数据（会话、日志、`api.sock`）在同一个目录，排障只看一个地方；
> ② 插件目录名带版本号，**换版本、删插件目录、换解压位置都不会动到配置**，钉钉凭证不会丢。
> ⚠️ 唯一要防的是查找顺序里"当前目录 `config.toml`"排在它前面 —— 所以生成和校验时**一律显式带 `--config $cfg`**，
> 免得某个工作目录里恰好躺着一份 `config.toml` 把真正那份顶掉（见 A5）。

> ⚠️ `cf-connect daemon uninstall` **会真的执行卸载**，加 `--help` 也会执行。别拿它当"看看用法"。
> ⚠️ **Windows 上 `cf-connect doctor` 不支持**（会直接说 not supported）。别把它当验证手段，用 §5 的验证流程。

## 3. 场景 A：「安装 cf-connect」

用户可能只说一句"帮我装 cf-connect"、"把 codefree 接到钉钉"。照下面走，**每完成一步就把结果汇报给用户**，不要闷头做完。

### ⛔ A0. 顺序不能乱（**这一节最容易犯的错，先读**）

```
① 问用户要 client_id / client_secret
        ↓  必须先有凭证，网关才连得上钉钉
② 写配置 → 启动后台服务（daemon install）→ 日志出现 dingtalk: stream connected
        ↓  只有网关跑起来、应用已发布，机器人在钉钉里才"在线"
③ 让用户在钉钉里对机器人发 /whoami
        ↓  这一步依赖 ②，早一步都拿不到
④ 用户把 userId 给你
        ↓
⑤ 回填 allow_from / admin_from → daemon restart → 验证
```

**为什么不能一起问**：

- `userId` **只能**在钉钉里对机器人发 `/whoami` 拿（它是钉钉侧的身份，本机查不到）。
- 机器人在钉钉里"在线"的前提是**网关已经跑起来**；网关能跑起来的前提是
  **`client_id` / `client_secret` 已经填好**（缺凭证连 `stream connected` 都不会出现）。
- 所以在没有凭证、服务没起的时候问 userId，用户只能回你一句"机器人没反应 / 我收不到回复"——
  白问一轮，还得重来。

**执行规则**：

- **第一批（现在就问）**：`work_dir`、`client_id` / `client_secret`（+ 可选 `mode` / 卡片模板）。
- **第二批（`daemon status` = running 之后才问）**：`userId` → 回填 `allow_from` / `admin_from`。
- ❌ 不要在同一条消息里既问 AppKey/AppSecret 又问 userId。
- `allow_from` / `admin_from` **先空着完全没问题**：空 = 暂时所有人都能对话（启动日志会有一条 WARN），
  不影响启动，也不影响你取 userId；拿到 userId 再回填并 `daemon restart`。

### A1. 找到 exe 所在目录（两种布局都认）

按顺序找，找不到就**停下来问用户**（不要自己去下载，本机可能没有外网）：

```powershell
# 0) 最可靠的一条线：你现在正在读的这个 skill 自己的位置
#    <插件根>\skills\cf-connect-setup\SKILL.md
#    → 插件根 = SKILL.md 往上数两级；再按下面的布局表拼出 bin 目录
Get-ChildItem "<插件根>\bin\cf-connect.exe","<插件根>\package\bin\cf-connect.exe" -ErrorAction SilentlyContinue
# 用户解压插件 tgz 得到的目录（首选：里面有 package\bin\cf-connect.exe）
Get-ChildItem "$env:USERPROFILE\Downloads","$env:USERPROFILE\Desktop" -Directory -ErrorAction SilentlyContinue |
  Where-Object { $_.Name -like "cf-connect-dingtalk*" }
# 已装进 codefree-o 的插件副本（skill 自己也在这里）
Get-ChildItem "$env:USERPROFILE\.codefree-o\.config\srdplugins" -Directory -ErrorAction SilentlyContinue |
  Where-Object { $_.Name -like "cf-connect-dingtalk@*" }
# 独立发行目录（根级就是 cf-connect.exe）
Get-ChildItem "D:\software" -Directory -ErrorAction SilentlyContinue | Where-Object { $_.Name -like "cf-connect-v*" }
```

| 布局 | exe 在哪 | 说明 |
|---|---|---|
| **插件包**（解压 `.tgz`） | `<解压目录>\package\bin\cf-connect.exe` | 本文档默认按这个走；`config.example.toml` 在 `<解压目录>\package\` |
| 已安装插件 | `~\.codefree-o\.config\srdplugins\cf-connect-dingtalk@<版本>\package\bin\cf-connect.exe` | 和上面结构相同 |
| **源码布局**（本仓库） | `<repo>\plugin\bin\cf-connect.exe` | 本 skill 在 `<repo>\plugin\skills\cf-connect-setup\SKILL.md`；自测/开发时没有 `package\` 这一层 |
| 独立发行目录 | `<发行目录>\cf-connect.exe` | 根级还有 `config.example.toml`、`install.ps1`、`QUICKSTART.md`、`README.md` |

> **下文统一用 `$bin` 表示"exe 所在的那个目录"**（插件包/已安装插件 = `<解压目录>\package\bin`，
> 源码布局 = `<repo>\plugin\bin`）。凡是在本文档里看到 `<解压目录>\package\bin`，就替换成你实际找到的 `$bin`。

**建议把解压目录放到英文路径**（如 `D:\tools\cf-connect-dingtalk\`），中文/空格路径会让第三方 CLI 出问题。

找不到就告诉用户：「请把 cf-connect 插件包解压到 `D:\tools\cf-connect-dingtalk\`，或告诉我它解压在哪」，然后停住。

### A2. 落 PATH（**你来做**，别指望用户自己想起来）

用户装完就想直接敲 `cf-connect`，敲不到就会来问"是不是没装上"。所以这一步由你在 A1 之后**顺手做掉**，
不要留给用户。包里的 `install.ps1` 是给**人手跑**的等价物；你直接改注册表更可控：

```powershell
$target = $bin          # ★ exe 所在目录，见 A1（绝对路径，别写 ~ 或相对路径）

# 写"用户 PATH"（HKCU\Environment）。要点：不能走 [Environment]::SetEnvironmentVariable ——
# .NET 的 setter 永远写 REG_SZ，会把原本 REG_EXPAND_SZ 的 PATH 静默降级，
# 里面的 %USERPROFILE% 之类条目就不再展开了。所以读原值、保留原类型、原地写回。
$key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment', $true)
$raw = [string]$key.GetValue('PATH', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
if ($key.GetValueNames() -contains 'PATH') {
  $kind = $key.GetValueKind('PATH')
} else {
  $kind = [Microsoft.Win32.RegistryValueKind]::ExpandString
}
$entries = @($raw -split ';' | Where-Object { $_ -ne '' })
if ($entries | Where-Object { $_.TrimEnd('\') -eq $target.TrimEnd('\') }) {
  Write-Host "用户 PATH 里已有：$target"
} else {
  $key.SetValue('PATH', (@($target.TrimEnd('\')) + $entries) -join ';', $kind)
  Write-Host "已加入用户 PATH：$target（值类型 $kind）"
}
$key.Close()

# 广播一次环境变更，让"已经开着"的资源管理器/终端也能刷新；
# 否则用户新开的窗口可能还是旧 PATH，然后告诉你"按你说的做了但还是找不到"。
Add-Type -Namespace CfConnect -Name Native -MemberDefinition '[DllImport("user32.dll", CharSet=CharSet.Auto, SetLastError=true)] public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg, IntPtr wParam, string lParam, uint fuFlags, uint uTimeout, out IntPtr lpdwResult);'
$out = [IntPtr]::Zero
Write-Output ('broadcast: ' + [CfConnect.Native]::SendMessageTimeout([IntPtr]0xffff, 0x001A, [IntPtr]::Zero, 'Environment', 2, 3000, [ref]$out))
```

验证——**读回注册表原值**：

```powershell
(([string][Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment').GetValue('PATH', '', [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)) -split ';') |
  Where-Object { $_ -like '*cf-connect*' }
```

- ⚠️ **别用 `where.exe cf-connect` 验证这一步**：它看的是**当前进程**继承来的 PATH，永远看不到你刚写进注册表的值，
  改了之后跑它多半还是"找不到"，会让你误判成失败
- **只对新开终端生效**——本会话里一律用绝对路径（`& $cc` 这种写法天然没这个问题），别在写完 PATH 之后立刻假设
  `cf-connect` 能直接敲
- 只写 `HKCU`，**不需要管理员权限**；不要去写 `HKLM`（那才要管理员，而且会污染整机所有用户）
- 顺手清理**失效条目**：PATH 里指向已删除目录的 `...cf-connect...` 老路径很常见（换版本、换解压目录之后），
  一条条过滤掉再写回，别留着当噪音——用户看到 PATH 里一堆 `cf-connect` 只会更糊涂
- 卸载时对应地把它删掉：`install.ps1 -Uninstall`，或用同样的过滤逻辑写回
- ⚠️ **如果包里的 `install.ps1` 在 PowerShell 5.1 上报 `字符串缺少终止符: '`**：那是老包的问题——
  脚本是 UTF-8 **无 BOM** + 中文注释，PS 5.1 会按 GBK 解码、把引号吞掉，于是解析失败（跟路径无关）。
  处理：按上面的注册表写法自己做，或改用 `pwsh -File install.ps1`（PowerShell 7）。
  本仓库已经修掉这个坑（打包时统一补 BOM，`verify-pack` 会卡住回归），但用户手上的老包不会自己变好。

### A3. 认识三个位置（别搞混）

| 位置 | 是什么 | 里面有什么 |
|---|---|---|
| **解压目录**（如 `D:\tools\cf-connect-dingtalk\`） | 用户解压插件 tgz 得到的位置 | `package\`（含 `bin\cf-connect.exe`、`config.example.toml`、`install.ps1`、skill、README） |
| **`package\bin\`**（源码布局是 `<repo>\plugin\bin`） | **只有程序** | `cf-connect.exe`、`.config.toml.lock`。**配置不在这里** |
| `%USERPROFILE%\.cf-connect\` | **配置 + 运行时数据**（一个目录装全部状态） | **`config.toml`（A5 生成，就是要填的那份）**、`run\api.sock`、会话历史、`logs\cf-connect.log`、`daemon.json`、定时任务 |

**配置跟"用户"走，不跟 exe 走**：A5 把配置生成到 `%USERPROFILE%\.cf-connect\config.toml`，
和会话历史 / 日志 / socket 放在一起。bin 目录只是程序：删掉它、换版本、换解压位置都不影响配置和凭证。
后台服务的工作目录是**配置所在目录**（即 `%USERPROFILE%\.cf-connect`），不是 bin 目录。

验证：

```powershell
& "<解压目录>\package\bin\cf-connect.exe" --version      # 必须打印版本号，如 cf-connect v0.1.1
```

### A4. 确认宿主（codefree-o），并解析出**能直接被拉起的绝对路径**

```powershell
codefree-o --version
```

- 没有/报错 → 先解决这一条，cf-connect 离开它就是个空壳。提示用户安装并登录 CodeFree。

接着解析 `cmd` 要填的**绝对路径** —— 别偷懒用 `(Get-Command codefree-o).Source`：
npm 全局装的 codefree-o 同时有三份 shim（`codefree-o` 无扩展名 / `.cmd` / `.ps1`），
PowerShell 多数情况下给你的是 **`.ps1`**，而 `.ps1` 是给交互式 shell 用的，
Go 的 `exec`（cf-connect 内部就是它）**拉不起来**。写错了不会立刻报错，
要等用户在钉钉里发第一句话才炸成 `CLI not found in PATH`。

```powershell
# 优先级：① npm 全局目录里的原生 exe（最稳，推荐）→ ② codefree-o.cmd 转发脚本 → ③ PATH 上的 codefree-o.exe
# 明确排除：codefree-o.ps1（ExternalScript）、无扩展名的 shim
$cmdPath = $null
$shim = (Get-Command codefree-o.cmd -ErrorAction SilentlyContinue).Source
if ($shim) {
  $inner = Join-Path (Split-Path $shim) 'node_modules\@srdcloud\codefree-o\bin\codefree-o.exe'
  if (Test-Path -LiteralPath $inner -PathType Leaf) { $cmdPath = $inner }
  if (-not $cmdPath) { $cmdPath = $shim }
}
if (-not $cmdPath) {
  $native = (Get-Command codefree-o.exe -ErrorAction SilentlyContinue).Source
  if ($native) { $cmdPath = $native }
}

$cmdPath                                            # ★ 原样填进 config.toml 的 cmd
Test-Path -LiteralPath $cmdPath -PathType Leaf       # 必须 True
[IO.Path]::GetExtension($cmdPath)                    # 期望 .exe 或 .cmd；是 .ps1 就回去重解析
```

典型结果：`C:\nvm4w\nodejs\node_modules\@srdcloud\codefree-o\bin\codefree-o.exe`
（同机上 `Get-Command codefree-o` 给的是 `...\codefree-o.ps1` —— 这两个**不是**一回事。）

### A5. 由你触发 exe 自动生成 config.toml（**放在 `~/.cf-connect`**），再填

**配置的固定位置 = `%USERPROFILE%\.cf-connect\config.toml`**（和运行时数据同一个目录）。
程序在 bin，状态在 `~/.cf-connect`：换插件版本、删插件目录都不会丢配置和凭证。

解压/安装完之后这个文件**还不存在** —— `cf-connect` 只在自己第一次启动时才写一份，
而那时它已经因为缺钉钉凭证起不来了。所以**由你（agent）主动触发一次 exe** 让它把配置生成出来，
这份生成出来的文件就是后面填写的唯一依据。

**为什么是"跑 exe"而不是手工复制样例**：exe 写的是它**自己内嵌**的那份样例
（`cmd/cf-connect/main.go` 的 `bootstrapConfig`），永远和这个版本真正支持的能力一致；
手工从 `package\config.example.toml` 复制，一旦包里的样例和 exe 不是同一版就会埋坑。

**触发前先读这三条安全边界**（违反任何一条都可能顶掉用户的实例、或把你自己卡死）：

1. **只在配置确实不存在时触发**：文件已经在时运行它不会"生成配置"，而是**真的去启动网关** ——
   要么因 instance lock 报 `already running`，要么把你这个会话挂在前台一直不返回。
2. **必须显式用 `--config "$cfg"` 指定路径**：查找顺序是
   `--config` > 当前目录 `config.toml` > `~/.cf-connect\config.toml`。
   虽然 `~/.cf-connect\config.toml` 就是默认回退位置，但**当前目录里的 `config.toml` 优先级更高**，
   所以还是要显式带上：`--config` 指向的目标不存在时，它才会把样例写到这个路径并立刻退出。
3. **绝不加 `--force`**：它会杀掉用户已经在跑的实例。

```powershell
$bin = "<解压目录>\package\bin"     # 例：C:\Users\62669\Desktop\cf-connect\cf-connect-dingtalk-plugin-0.1.1-windows-amd64\package\bin
$cc  = "$bin\cf-connect.exe"
$cfg = "$env:USERPROFILE\.cf-connect\config.toml"   # ★ 配置固定在这里（和运行时数据同一个目录）

if (Test-Path $cfg) {
  Write-Host "配置已存在，跳过生成（不要启动它，可能已有实例在跑）：$cfg"
} else {
  $out = "$env:TEMP\cf-connect-bootstrap.log"
  # --config 指向目标路径：文件不存在 → 写样例 → 打印 Created default config at ... → 退出
  $p = Start-Process -FilePath $cc -ArgumentList '--config', "`"$cfg`"" -WorkingDirectory $bin -NoNewWindow -PassThru `
        -RedirectStandardOutput $out -RedirectStandardError "$out.err"
  $p | Wait-Process -Timeout 30 -ErrorAction SilentlyContinue
  if (-not $p.HasExited) {                      # 30s 还没退 = 它其实在跑服务，不是只写配置
    $p | Stop-Process -Force
    Write-Host "!! 30s 未退出，已强制停止：说明触发时配置并不缺失，检查 $cfg"
  }
  Get-Content $out -ErrorAction SilentlyContinue   # 正常应看到：Created default config at C:\Users\<你>\.cf-connect\config.toml
}

# 双闸门：文件在、能解析
Test-Path $cfg
& $cc config format --config $cfg        # 必须回显 Formatted C:\Users\<你>\.cf-connect\config.toml
```

> ⚠️ `cf-connect config path` 这个子命令**不认 `--config`**，它只做自动探测
> （`./config.toml` > `~/.cf-connect\config.toml`）。本插件的配置正好就是它的默认探测结果，
> 所以在**当前目录没有 `config.toml`** 的前提下（比如先 `Set-Location $env:USERPROFILE`）跑
> `& $cc config path` 会回显 `C:\Users\<你>\.cf-connect\config.toml`。
> 想校验任意一份配置是否可用，一律用 `config format --config $cfg`。

> ⚠️ 生成出来的内容里 `work_dir = "D:\\工作目录"`、`client_id = "client_id"`、
> `client_secret = "client_secret"` 全是**占位符**，必须按 A5.1 与 §4 逐项替换 ——
> **"文件已生成"不等于"配置已完成"**，别在这个状态下就说装好了。
> 已经有 `config.toml` 时**不要覆盖**（用户可能填过凭证了）。
> 如果用户是从旧版本升上来的、配置本来就在 `%USERPROFILE%\.cf-connect\config.toml`，那就**直接用现成的**：
> 只补 A5.1 里缺的项（`work_dir` / `cmd`）就行，别重新生成，也别再问一遍 AppKey/AppSecret。

然后**按 A0 的两批顺序问**（第一批现在问，第二批等服务 running 之后再问）：

| 批次 | 要填的 | 谁来定 | 怎么问 / 怎么自动定 |
|---|---|---|---|
| **① 现在** | `work_dir` | **问用户** | 「你希望我在哪个目录里改代码？给绝对路径。建议专用目录，别用家目录或 C 盘根目录。」填完后**必须过 A5.1 的实例校验** |
| **① 现在** | `cmd` | 自动 | **A4 解析出来的 `$cmdPath`**：绝对路径、且必须是 `.exe` / `.cmd`（后台服务不继承你终端的 PATH；写裸名字 `"codefree-o"` 或写成 `.ps1`，钉钉里第一句话就会报 `CLI not found in PATH`） |
| **① 现在** | `client_id` / `client_secret` | **只能用户给** | 见 §4 的**第一批**话术（只要这两个值，**不要**顺带要 userId） |
| **① 现在** | `mode` | 建议 `default` | 「演示或首次使用建议 `default`；`yolo` 会跳过 CLI 的权限提示，确认了再开」 |
| **① 现在** | `language` | 自动 | 用户说中文就 `zh` |
| **① 可选** | `card_template_id` | 可选 | 想要"边跑边看过程"才需要，见 §4 第 2 部分 |
| **② 服务 running 后** | `allow_from` | **问用户** | 见 §4 的**第二批**话术：让用户在钉钉里发 `/whoami`，把 userId 发你 |
| **② 服务 running 后** | `admin_from` | **问用户** | 同一个 userId（留空 = 管理命令对所有人关闭，fail-closed，不是"继承 allow_from"） |

#### A5.1 定稿前必须做的实例校验（**漏掉这一条 = 钉钉里第一条消息必炸**）

样例里的 `work_dir = "D:\\工作目录"`、`cmd = "codefree-o"` 是**占位符**
（A5 生成出来的那份就是样例内容，所以它一定带着这些占位符）。
如果直接照抄，服务能起来，但用户在钉钉发第一句话会收到：

```
Error: opencodeSession: start: chdir D:\工作目录: The system cannot find the file specified.
```

（原因：`agent/opencode/session.go` 把 `cmd.Dir` 设成 `work_dir`，目录不存在 exec 就失败；
cf-connect 启动时**不校验** `work_dir`，所以是延迟到第一条消息才报错。）

把配置里**所有"路径型"和"命令型"的值逐个实例校验**，全部通过才算定稿：

```powershell
# $bin / $cfg 沿用 A5 里定义的值；换了新终端就重新赋值
$bin = "<解压目录>\package\bin"; $cfg = "$env:USERPROFILE\.cf-connect\config.toml"

# work_dir：必须存在，且是目录。不存在就直接建（别留给用户自己去踩）
$wd = "<用户给的工作目录>"
if (-not (Test-Path $wd -PathType Container)) {
  New-Item -ItemType Directory -Force $wd | Out-Null
  Write-Host "已创建工作目录：$wd"
}
Test-Path $wd -PathType Container          # 必须 True

# cmd：必须是绝对路径、指向真实文件、且不是 .ps1（见 A4；别用裸名字）
$cmdRef = "<A4 解析出来的 $cmdPath>"
Test-Path -LiteralPath $cmdRef -PathType Leaf        # 必须 True
[IO.Path]::GetExtension($cmdRef)                     # 期望 .exe / .cmd

# 别留占位符：确认这两个值都不是样例原样
Select-String -Path $cfg -Pattern '^\s*(work_dir|cmd)\s*='
```

判定：

| 检查项 | 不通过时怎么办 |
|---|---|
| `work_dir` 不存在 | **默认直接创建**；用户明确说不要就再问他要别的目录 |
| `work_dir` 指向家目录 / 系统盘根 / 网络共享 | 提醒风险（agent 会在这个目录里读写、跑命令），建议换成专用项目目录 |
| `cmd` 解析不到 | 先装/修好 codefree-o；**不要**带着一个跑不起来的 `cmd` 定稿 |
| `cmd` 是裸名字（`codefree-o`）或 `.ps1` | 当下前台起可能没事，但后台服务/Go 的 `exec` 解析不到 → 回 A4 用 `$cmdPath` 重写 |
| 配置里还留着 `codefree-o`（样例原值） | 说明 `cmd` 没真正改过，回 A4 取绝对路径写进去 |
| 配置里还留着 `D:\工作目录` 这类占位符 | 说明没真正改过，回到上表重新填 |

#### A5.2 回显

回显给用户时，`client_secret` 一律脱敏：**前 4 位 + `…` + 后 4 位**。
并明确告诉他「`work_dir` 已设为 `X`，在钉钉里可以用 `/dir <绝对路径>` 随时切换」。

#### A6. 启动：由你把它装成**后台服务**（前台跑等于没启动）

> 这一步**不需要** `allow_from` / `admin_from`（它俩要等这一步之后才能拿到，见 A0/A7）。
> 只要求 `client_id` / `client_secret` 已经写进配置 —— 否则起来也连不上钉钉。

⚠️ **前台方式不要用**：`& $cc` / `& $cc --force` 在你（agent）的会话里**看着像启动成功，其实没有** ——
日志会照常打印 `dingtalk: stream connected` / `platform ready` / `engine started`，
但那只是这一次命令调用还活着；命令一返回（或被 esc 中断、会话切走），进程就被杀掉。
用户随后在钉钉里发消息，什么都不会发生。所以启动一律走**后台服务（守护进程）**：

```powershell
$bin = "<解压目录>\package\bin"
$cc  = "$bin\cf-connect.exe"        # 绝对路径
$cfg = "$env:USERPROFILE\.cf-connect\config.toml"   # ★ 配置在用户目录（和运行时数据一起）

& $cc daemon install --config $cfg    # 安装并启动后台服务（Windows 上是计划任务/schtasks）
& $cc daemon status                   # 必须是 running
Get-Process cf-connect -ErrorAction SilentlyContinue | Select-Object Id,ProcessName
Test-Path "$env:USERPROFILE\.cf-connect\run\api.sock"      # 存在 = 真的有实例在驻留
& $cc send -m "bridge ping"                                # 钉钉里应该收到这条（send 不需要 --config）
```

- **`--config` 要显式带上**：`daemon install` 默认拿**当前目录**当服务的工作目录，并要求那里就有
  `config.toml`。从别处执行时必须 `--config`，否则直接报
  `Warning: config.toml not found in <当前目录>` 并退出码 1。
  它同时决定服务进程的工作目录（服务脚本等价于 `Set-Location <配置所在目录>` 后再跑 exe）——
  配置在 `%USERPROFILE%\.cf-connect`，于是**服务的工作目录就是 `~/.cf-connect`**（和运行时数据同处，
  `config.toml` 就在那儿，服务起来后按默认查找顺序也能自洽）
- `cf-connect send` 走的是运行时 socket（`~/.cf-connect\run\api.sock`），**不需要 `--config`**，
  所以 `send` 在任何目录下都能用

- ⚠️ **后台服务不继承你终端的 PATH**：配置里的 `cmd` 一定要写 codefree-o 的**绝对路径**，
  否则服务是起来了，但用户在钉钉里发的第一句话会报找不到 CLI（见 A5 的 `cmd` 行）
- Windows 上它是**计划任务（Task Scheduler）**，不需要管理员权限，以当前用户身份在**登录时**启动；
  元数据写在 `%USERPROFILE%\.cf-connect\daemon.json`，日志写在
  `%USERPROFILE%\.cf-connect\logs\cf-connect.log`（`& $cc daemon logs -f` 可跟随）
- 前台方式**只用于排障看实时日志**：`& $cc`（Ctrl+C 退出），看完即止，不要把它当启动手段交付
- ⚠️ `cf-connect daemon uninstall` 会**真的执行卸载**，加 `--help` 也会执行，别拿它试用法
- 记下你实际用的路径：`daemon status` 里会显示服务对应的 config 与工作目录，方便后面排障

#### A7. **服务 running 之后**：要 userId，回填 allow_from / admin_from

**先确认 `daemon status` = running、`api.sock` 存在**，再去找用户要 userId（A0 的 ③④⑤）。
服务起来之后日志里通常有两条 WARN（这是正常的默认态，不是报错，也**不影响你现在取 userId**）：

```
level=WARN msg="allow_from is not set - all users are permitted..."
level=WARN msg="admin_from is not set - privileged commands (/shell, /show, /dir, /restart, /upgrade) are blocked..."
```

拿到 userId 后回填这两个键（`work_dir` 属于第一批，早就填好了），**别写错层级**：

| 要填的键 | 写在哪一段 | 为什么必须填 | 不填的后果 |
|---|---|---|---|
| `allow_from = "<userId>"` | `[projects.platforms.options]` | 只允许你自己跟机器人对话 | 任何搜得到这个机器人的同事都能用**你的额度** |
| `admin_from = "<userId>"` | `[[projects]]`（项目级，不是 platform 下） | 允许你使用管理命令 | 管理命令对所有人关闭（fail-closed），`/dir`、`/show`、`/shell`、`/restart` 都用不了 |

> `work_dir`（`[projects.agent.options]`）在第一批就该填好并过 A5.1 校验；如果那时还没定，
> 现在一并补上 —— 它留占位符的话，钉钉里第一句话就报 `chdir ... cannot find the file specified`。

回填后 `& $cc daemon restart` 让配置生效，再发一条测试消息确认。

#### A8. 汇报模板（**分两次发，别一次把问题问完**）

**第一次：凭证到手 + 服务起来之后立刻发**（此时还没要 userId）：

````text
网关已就绪 ✅
- 网关     ：<解压目录>\package\bin\cf-connect.exe（vX.Y.Z）
- 配置文件 ：%USERPROFILE%\.cf-connect\config.toml（已自动生成，已填 client_id / client_secret / work_dir）
- 后台服务 ：daemon status = running，api.sock 已就绪，日志里已出现 dingtalk: stream connected

还需要你做一件事（现在可以做了）：
在钉钉里搜到你的机器人，发一条 `/whoami`，把回复里的 userId 发我。
我会用它填 allow_from（只允许你对话）和 admin_from（只允许你用管理命令），然后重启验证。
【为保证对话私密性，不建议公用机器人。请自己创建测试企业和机器人来进行使用】
（补充：allow_from 现在还是空的，这一步之前谁都能跟机器人说话；拿到 userId 我马上补上）
````

**第二次：拿到 userId、回填并 restart 之后发**：

````text
配置完成 ✅
- allow_from = "<userId>"（平台层：只有你能跟机器人对话）
- admin_from = "<userId>"（项目层：只有你能用 /dir /show /shell /restart）
- work_dir   = "<你的项目目录>"（agent 实际改代码的目录）
- 已执行 daemon restart，daemon status = running

验证：我发了 `cf-connect send -m "bridge ping"`，你应该在钉钉里收到了。
之后直接在钉钉里派活即可，例如：帮我看下 <项目> 的编译错误并修掉
````

## 4. 引导用户拿钉钉凭证（你无法代替的一步）

**分两条消息发，中间隔着"启动服务"这一步**（原因见 A0：没有凭证起不来网关，没起网关就问不到 userId）。

### 第一批 · 现在发（只要 AppKey / AppSecret）

用户说"你帮我配一下钉钉"时，直接把下面这段给他（别自己瞎猜后台路径）：

> **第一步：请在钉钉开放平台建一个企业内部应用，把两个值发我**
> 1. 打开 https://open-dev.dingtalk.com/ → 应用开发 → **企业内部应用** → 创建应用
> 2. 在「凭证与基础信息」里复制 **AppKey** 和 **AppSecret**（AppKey = 我配置里的 `client_id`）
> 3. 左侧「机器人」→ 打开机器人配置 → **消息接收模式必须选 `Stream`**（这样你本机不用公网 IP）
> 4. 左侧「权限管理」→ 勾选机器人发送消息、接收消息相关权限
> 5. **右上角「版本管理与发布」→ 发布应用**（没发布消息进不来）
>
> 拿到 **AppKey** 和 **AppSecret** 发我即可。我配好并启动网关之后，再请你做第二步
> （在钉钉里发 `/whoami` 拿 userId）—— **现在先别发 `/whoami`，这时候机器人还没上线，发了不会有回复**。
>
> ⚠️【为保证对话私密性，不建议公用机器人。请自己创建测试企业和机器人来进行使用】

拿到后**只写这两个键**（`allow_from` 留到第二批）：

```toml
[projects.platforms.options]
client_id = "<AppKey>"
client_secret = "<AppSecret>"
# allow_from / admin_from 等 A7 拿到 userId 之后再填
```

写完后**立刻接着做 A6（启动后台服务）**——启动不需要白名单，空着照样能连上钉钉。

### 第二批 · `daemon status` = running 之后再发

> **第二步：网关已经跑起来了，请帮我拿一下 userId**
> 1. 打开钉钉，搜到你自己的机器人（就是刚才那个应用），进入它的单聊
> 2. 发一条 `/whoami`
> 3. 机器人会回一条包含 **userId** 的消息，把那串 userId 发我（我用来填 `allow_from` 和 `admin_from`）
>
> 如果发消息**没有任何回复**：先回上面确认第 3 步选了 `Stream`、第 5 步点了「发布应用」；
> 都确认过还是不行，就告诉我，我去查网关日志（`~/.cf-connect/logs/cf-connect.log`）。

拿到 userId 后按 A7 回填、`daemon restart`、再发测试消息验证。

（可选，想要流式过程）再引导一次：

> 想看到"边想边做"的实时过程吗？需要在钉钉开放平台 → **卡片平台** → 新建「AI 卡片」模板，里头放一个绑定变量 `content` 的 Markdown 组件，发布后把**模板 ID** 发我。不配也能用，只是回复会在整轮结束后一次性到达。

如果用户拿不到凭证（无权限、公司流程慢），**不要卡住**：告诉他可以先跳过——但要说清代价：
**没有 `client_id`/`client_secret` 网关起不来，也就没法在钉钉里跟他说话、更拿不到 userId**，
等他拿到凭证再继续 A5/A6。
顺便提醒一句：**不建议用公用机器人**——同一个机器人被多人共用时，谁都可能收到或接管别人的会话；
为保证对话私密性，请自己创建测试企业和机器人。

## 5. 场景 B：「测一下连通性」

按顺序跑，**每一步都给出判断和下一步动作**，不要只看命令退出码：

```powershell
$bin = "<解压目录>\package\bin"
$cc  = "$bin\cf-connect.exe"        # 绝对路径最稳
$cfg = "$env:USERPROFILE\.cf-connect\config.toml"   # ★ 配置在用户目录（和运行时数据一起）

# ① 二进制与版本
& $cc --version

# ② 配置能否解析（会回显规整后的文件）
& $cc config format --config $cfg

# ③ 宿主可用
codefree-o --version

# ④ 服务是否在跑（后台服务形态；有实例会创建这个 socket 文件）
Get-Process cf-connect -ErrorAction SilentlyContinue | Select-Object Id,ProcessName
Test-Path "$env:USERPROFILE\.cf-connect\run\api.sock"

# ⑤ 真正发一条到钉钉（用户必须在钉钉里收到，才算通）
& $cc send -m "cf-connect 连通性测试 $(Get-Date -Format 'HH:mm:ss')"
```

> ⚠️ ④ 没过时**不要用前台跑糊过去**（前台看着有日志，命令一结束进程就没了）：按 §3 A6
> `& $cc daemon install --config $cfg`（或 `daemon restart`），再确认 `daemon status` = running 与 `api.sock` 存在。

判定表（照着回给用户）：

| 现象 / 报错 | 原因 | 下一步 |
|---|---|---|
| 钉钉里回 `Error: opencodeSession: start: chdir <路径>: The system cannot find the file specified.` | **`work_dir` 指向了不存在的目录**（多半是照抄了样例里的占位符） | 配置里改成真实目录后 `& $cc daemon restart`；或直接在钉钉发 `/dir <绝对路径>`（命令不拉 agent，所以还能用）；预防见 §3 的 A5.1 |
| ① 报 command not found | 没装或没在 PATH | 用绝对路径；或跑 `install.ps1` 后**新开终端** |
| ② 报 `Config file not found: ...` | 配置没生成，或 `--config` 指错了位置 | 配置应在 **`%USERPROFILE%\.cf-connect\config.toml`**；回 §3 的 A5 用 `--config $cfg` 生成，再填各处 |
| ② 报解析错误 | 配置格式坏了 | 对着 `config.example.toml` 修；或删掉重跑 A5 重新生成（**先确认用户没填过凭证**） |
| ③ 失败 | 宿主缺失/未登录 | `codefree-o --version` 修复；必要时 `codefree-o auth` |
| ④ 没有 cf-connect 进程 | 后台服务没装或没起来 | 按 §3 A6：`& $cc daemon install --config $cfg`（已装过就 `daemon restart`），再 `daemon status` 确认 running |
| ⑤ 报 `cf-connect is not running (socket not found)` | 同上，服务没起 | 先解决 ④ |
| ⑤ 报凭证错误 | `client_id`/`client_secret` 不对 | 回 §4 重新核对（注意别贴明文） |
| ⑤ 返回成功但钉钉没收到 | 应用没发布 / 不是 Stream 模式 / 机器人没启用 / `allow_from` 没包含自己 | 逐条核 §4 的 3、4、5 步 |
| 全绿但用户说"我发消息它不回" | 会话侧问题 | 让用户在当前会话发 `/status`；或看 `~/.cf-connect` 下的日志 |
| 验证时**故意发一句话**回来了 `chdir ... cannot find the file` | `work_dir` 不存在（照抄了占位符） | 改配置或 `/dir <真实路径>`；预防见 §3 的 A5.1。**收到这条就说明前面几步全过了，只差目录** |

**验证成功的定义只有一个：用户在钉钉里真的收到并回了一条消息。** 命令返回 0 不算。

## 6. 场景 C：「卸载 cf-connect」

**先确认用户要卸到什么程度**，再动手（一句话问清）：

| 级别 | 动作 | 会丢什么 |
|---|---|---|
| 只停服务 | `& $cc daemon stop`（若曾用前台方式跑过，请用户关掉那个窗口） | 不丢 |
| 删服务（保留数据与配置） | `& $cc daemon uninstall` | 不丢 |
| 从 PATH 移除 | `powershell -File <解压目录>\package\install.ps1 -Uninstall` | 不丢 |
| 删数据目录 | 删除 `%USERPROFILE%\.cf-connect` | **会丢会话历史、定时任务、web token，以及 `config.toml`（钉钉凭证）** |
| 删程序目录 | 删除解压目录（或 srdplugins 下的插件目录） | 不丢任何东西：**配置和凭证在 `%USERPROFILE%\.cf-connect`，不在程序目录里** |

标准顺序：

```powershell
$cfg = "$env:USERPROFILE\.cf-connect\config.toml"    # ★ 配置和运行时数据同目录，删之前先备份它
# ① 先停服务（正规形态是后台服务，所以 stop/uninstall 就够了）
& $cc daemon stop
& $cc daemon uninstall
# ② 再决定要不要动 PATH / 数据目录
powershell -ExecutionPolicy Bypass -File "<解压目录>\package\install.ps1" -Uninstall
# 只有用户明确说"全清"才做下面这步（⚠️ 这一步会一起删掉 config.toml = 钉钉凭证，想留就先复制出去）
Remove-Item -Recurse -Force "$env:USERPROFILE\.cf-connect"
```

> ⚠️ 卸载前提醒一句：**钉钉那边不会自动解绑**。如果用户以后还要用，钉钉应用留着别删；应用本身是否删除由用户去开放平台决定。
> ⚠️ 别顺手删 `%USERPROFILE%\.codefree-o`——那是 CodeFree 自己的目录，跟 cf-connect 无关。

## 7. 场景 D：用户说"我在钉钉里跟你说话"、"手机上接着弄"

两边共用同一个 `codefree.db` 会话库，所以可以互相接管：

| 用户想做的事 | 做法 |
|---|---|
| 在钉钉里继续终端那个会话 | 钉钉里发 `/list` 看列表 → `/switch <会话ID前缀或描述关键字>` |
| 把终端的结果推到钉钉 | `cf-connect send --stdin`（长文本用 `--stdin`，避免引号/换行被 shell 吃掉） |
| 查自己在钉钉里的身份 | 钉钉里发 `/whoami` |
| 换工作目录 | 钉钉里 `/dir` 相关命令，或改 `config.toml` 的 `work_dir` 后重启服务 |

**用 `/switch` 的两个要点**（务必转告用户）：

1. **优先用会话 ID 或描述关键字，不要用序号**（`/switch 2` 里的 2 是列表第 2 项，列表一变就错位）
2. **切换前确认终端那侧的会话已经退出**：同一时间两边写一个会话，消息可能交错、历史可能被覆盖（这是当前版本（0.1.1）的已知边界，未做互斥）

推送消息示例：

```powershell
cf-connect send --stdin <<'EOF'
**构建完成** ✅
- 用例：128 passed / 0 failed
- 产物：dist/cf-connect-v0.1.1-windows-amd64.zip
EOF
```

## 8. 场景 E：升级 cf-connect

自更新功能已移除，升级 = **换 exe 覆盖 + 重装/重启后台服务**：

```powershell
$bin = "<解压目录>\package\bin"
$cc  = "$bin\cf-connect.exe"
$cfg = "$env:USERPROFILE\.cf-connect\config.toml"   # ★ 配置在用户目录，换 exe / 换插件版本都不动它
# ① 停后台服务
& $cc daemon stop
# ② 换 exe（覆盖 bin 里的那份；config.toml 不动，凭证保留）
Copy-Item <新版本目录>\package\bin\cf-connect.exe $cc -Force
& $cc --version                     # 确认新版本
# ③ 重新起服务（换过 exe 后要重装一次，让计划任务指向新路径）
& $cc daemon install --config $cfg  # 已存在服务时按提示加 --force
& $cc daemon status                 # running
& $cc send -m "upgrade check"
```

> ⚠️ **插件包升级是另一回事**：插件目录名带版本号（`cf-connect-dingtalk@<新版本>`），
> 换了版本就要把 PATH 指到新目录、删掉旧目录（否则新旧两份 skill 会同时被注入）。
> **配置不用搬**：它在 `%USERPROFILE%\.cf-connect\config.toml`，和插件目录无关，换版本、删旧目录都不动它。
>
> ⚠️ **用户 PATH 也要跟着换**：A2 写进 PATH 的是**带版本号的目录**，升级后那条就指向了马上要被删掉的旧目录。
> 换完版本回到 A2 重写一次，并把旧版本那条删掉（同一次过滤里就能做掉）。

升级不影响：`~/.cf-connect` 里的**配置、会话历史、日志、socket**全都原地不动。
**唯一要注意**：新版本如果加了配置项，用户的 `config.toml` 不会自动获得——拿新版本目录里的 `config.example.toml` 比一比，缺什么补什么。

## 9. 模糊指令 → 动作对照（用户会怎么说）

| 用户可能说 | 你要做什么 |
|---|---|
| "装 cf-connect" / "把 codefree 接到钉钉" | §2 探测 → A0 记住顺序 → §4 **第一批**索要 AppKey/AppSecret → A5 触发 exe 生成并填写配置 → A6 装后台服务 → §4 **第二批**要 userId → A7 回填 allow_from/admin_from + restart → §5 验证 |
| "怎么启动" / "怎么关掉" | §3 A6：`& $cc daemon install --config $cfg`（启动）/ `daemon stop`（停）；**别用前台方式交付** |
| "测试连通性" / "通不通" / "钉钉没反应" | §5 全流程 + 判定表 |
| "卸载" / "不要了" / "删掉" | §6，先问清卸到哪一级 |
| "换个工作目录" | 改 `config.toml` 的 `work_dir` → `& $cc daemon restart` → `cf-connect send -m` 验证 |
| "让 codefree 只在某个项目里干活" | 确认 `work_dir`，并建议把 `<work_dir>/.cf-connect/` 加进 `.gitignore` |
| "把结果发到钉钉" | §7 的 `cf-connect send`；没有活跃会话就说明原因，别反复重试 |
| "手机上能接着弄吗" | §7 的 `/list` + `/switch`，并提醒两个要点 |
| "能不能不改代码只看过程" | 引导配 `card_template_id`（§4 可选段） |
| "担心安全" | 见 §10 安全清单 |

## 10. 安全清单（配完主动跟用户过一遍）

- `client_secret` 是**每人一套**的本机凭据：只放 **`%USERPROFILE%\.cf-connect\config.toml`**（别放项目目录、别放插件目录），不提交 git、不发群里
- **不建议用公用机器人**：多人共用一个机器人时，消息会进同一个网关、会话可能互相接管或覆盖 ——
  为保证对话私密性，请让用户自己创建测试企业和机器人（凭据也各用各的）
- `allow_from` 填用户自己的 userId（钉钉发 `/whoami` 可查），否则任何能搜到机器人的同事都能用其额度 —— **装完必须回填，见 §3 A7**
- `admin_from` 填同一个 userId（写 `[[projects]]` 项目级），否则 `/dir`、`/shell`、`/show`、`/restart` 这类管理命令对所有人关闭
- `work_dir` 指向**专用工作目录**，不要指到家目录或系统盘根
- 首次演示建议 `mode = "default"`；`yolo` 会跳过 CLI 的权限提示（cf-connect 本身没有交互式权限确认，别把它当成安全门）
- 把 `<work_dir>/.cf-connect/` 加进项目的 `.gitignore`（媒体/附件会落在那里）
- Windows 未签名 exe 可能触发 SmartScreen → 「更多信息 → 仍要运行」；企业内可统一签名

## 11. 排障速查

| 症状 | 原因 | 处置 |
|---|---|---|
| `socket not found` / `cf-connect is not running` | 后台服务没装/没起来（**最常见的真实原因：只用前台跑过，进程随命令结束就没了**） | `& $cc daemon install --config $cfg`（装过就 `daemon restart`），`daemon status` 必须 running，再看 `run\api.sock` |
| 跑 `cf-connect.exe` 一下就退出，只打印 `Created default config at ...` | **正常**：当时配置不存在，它写完就退出（§3 A5） | 先看它写到了哪：本插件的约定是 `%USERPROFILE%\.cf-connect\config.toml`。写到别处通常是当前目录里有一份 `config.toml` 被优先命中，或 `--config` 指错。填好后再 `daemon install --config $cfg` |
| 日志里看到 `stream connected` / `platform ready`，但钉钉里发消息没反应 | **前台跑的后遗症**：日志只属于那一次命令调用，进程随后就被杀了 | 改用后台服务：`& $cc daemon install --config $cfg`（§3 A6） |
| 启动报 `already running` / instance lock | 已经有一个实例在跑（可能是残留的前台进程） | 先 `daemon status` 看是不是服务实例；**不要随手加 `--force`** |
| 钉钉里发消息没反应 | 应用没发布 / 不是 Stream 模式 / 机器人没启用 / `allow_from` 不含自己 | 逐条核 §4 |
| `unknown flag: --dangerously-skip-permissions` | `yolo` 给老版本 CLI 发了不支持的标志 | 配 `permission_flag = "--auto"`（默认值） |
| 回复慢、看不到过程 | 没配 AI 卡片流式 | 配 `card_template_id` |
| 会话标题/消息数空 | 数据目录没识别对 | 配 `data_dir` / `db_file`（见 config 里「高级」段） |
| 服务起不来、报配置错误 | `config.toml` 语法/键位不对 | `config format` 校验；`allow_from` 必须在 `[projects.platforms.options]` 下，写到 `[[projects]]` 里会被**静默丢弃**，机器人就对所有人开放 |
| 中文/空格路径下行为异常 | 第三方 CLI 解析问题 | 换到英文路径（如 `D:\tools\cf-connect-dingtalk\`） |
| `cf-connect doctor` 报 not supported | Windows 不支持该命令 | 用 §5 的验证流程替代 |
| 敲 `cf-connect` 报"无法将"cf-connect"项识别为 cmdlet、函数、脚本文件或可运行程序的名称" | `$bin` 不在**用户 PATH（注册表）**里，或者那条指向的是**已被删除的旧目录**（换版本 / 换解压目录之后最典型）；也可能只是**这个终端是改 PATH 之前开的** | 按 A2 直接写 HKCU 并广播，然后**新开**终端。⚠️ 别用 `where.exe cf-connect` 去判断——它只看当前进程继承的 PATH，看不到注册表里的真实值 |
| 跑 `install.ps1` 报 `字符串缺少终止符: '`（ParserError，行号指向某个 `Write-Host`） | 老包里的 .ps1 是 UTF-8 **无 BOM** + 中文注释，PowerShell 5.1 按 GBK 解码把引号吞了 → 字符串没闭合。跟路径无关 | 别用 PS 5.1 跑它：按 A2 自己写注册表，或 `pwsh -File install.ps1`。新打包已统一补 BOM（`verify-pack` 会卡回归） |
| 服务 `running`、日志也 `stream connected`，但钉钉里第一句话报 `"..." CLI not found in PATH` | `cmd` 写的是裸名字 `codefree-o` 或 `.ps1`，后台服务的环境里解析不到 | 回 A4 解析出 `.exe` / `.cmd` 绝对路径写进 `cmd` → `daemon restart` |

## 12. 你可以顺手用的 cf-connect 能力（用户问起时别答"不知道"）

```
cf-connect send -m "..."                发消息到钉钉活跃会话（--stdin 读长文本）
cf-connect sessions list|show <id>      浏览会话历史
cf-connect agent-sid                    打印当前会话的 agent 会话 ID（可对接管/续聊）
cf-connect cron add/list/exec/del       定时任务
cf-connect timer add/list/info/del      一次性提醒
cf-connect relay send --to <项目> ...   跨项目转发并等回复
cf-connect provider add/list/remove     管理 API provider
cf-connect web                          内嵌 Web 管理界面
cf-connect config example|format|path   配置：打印样例 / 规整 / 查路径
cf-connect daemon install|start|stop|restart|status|logs|uninstall
                                        # 正规的启动/停止方式（后台服务）；install=装并启动（§3 A6）
```

配置里可调的关键项（都在 `config.toml` 注释里）：`work_dir`、`cmd`、`mode`、`model`、`agent`、`allow_from`、`admin_from`、`card_template_id`、`language`、`[display]`、`[cron]`、`[management]`。
