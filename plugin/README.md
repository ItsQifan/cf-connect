# CF-Connect 钉钉桥接插件（cf-connect-dingtalk v0.1.0）

> **在钉钉里远程驱动 CodeFree（codefree-o）**：手机钉钉里派活 → 本机 codefree-o 写代码 →
> 过程与结果流式回到钉钉会话。

这是「CodeFree 技能实践月」的参赛插件包。真正的钉钉 Stream 长连接、会话路由、卡片流式都在
Go 二进制 `cf-connect.exe` 里实现；插件负责**把网关带进来 + 教会 agent 什么时候怎么用它**。

**装上后 codefree-o 里看得见的东西**（加载器只认这些，`.codefree-plugin/plugin.json` 声明）：

| 注入项 | 内容 | 验证方式 |
|---|---|---|
| skill | `dingtalk-bridge`（何时推送、怎么推送、失败怎么处置） | `codefree-o debug skill` |
| command | `/dingtalk-notify`（手动推一条）、`/dingtalk-status`（一键自检链路） | `codefree-o debug config` → `command` 段 |

**包里还有但不会被自动加载的东西**：`index.js` 是一份标准的 codefree-o 插件入口
（`config` / `event` / `tool` 三个 hook，含 `notify_dingtalk` 工具）。加载器只从
`srdplugins` 注入 skills / commands / agents / mcpServers（二进制里的 `codefree-plugin inject *`
日志可以印证），**不会**执行包内的 JS；要启用它，需要在宿主的 `plugin` 数组里注册这个模块
（`codefree-o plugin <module> -g`）。README 里把它标成「可选增强」，不是本包生效的前提。

```
钉钉 App ──Stream 长连接──▶ cf-connect.exe ──CLI + NDJSON──▶ codefree-o
   ▲                              │
   └──── 流式卡片 / 完成通知 ◀────┘
```

---

## 1. 包结构

```
cf-connect-dingtalk-plugin-0.1.0.tgz       ← 上传到比赛平台的就是这个文件
├── .codefree-plugin/plugin.json        保险副本（比赛文案的 /.codefree-plugin/plugin.json 口径）
├── config.example.toml                 精简配置样例（= 发行 zip 里那份）
├── install.ps1                         可选：把 bin\ 加入用户 PATH
├── QUICKSTART.md                       钉钉应用创建 5 步
└── package/                            ← codefree-o 实际解包读取的目录
    ├── .codefree-plugin/
    │   ├── plugin.json                 ★ 平台校验目标
    │   └── build-info.json             构建溯源（版本/时间/commit）
    ├── package.json                    npm 元数据
    ├── index.js                        插件入口（config / event / tool 三个 hook）
    ├── skills/dingtalk-bridge/SKILL.md 顶层目录 → 自动注入为 skill
    ├── commands/dingtalk-notify.md     → /dingtalk-notify
    ├── commands/dingtalk-status.md     → /dingtalk-status
    ├── bin/cf-connect.exe              网关二进制（打包时从 plugin/bin 带入）
    ├── config.example.toml
    ├── QUICKSTART.md
    └── README.md
```

另外还会产出一份 **`.zip`**（`cf-connect-dingtalk-plugin-0.1.0.zip`），里面放同样的内容 +
上面这个 `.tgz`：Windows 上双击就能看，方便评审翻阅，也可以解出来直接上传。

## 2. 安装

### 2.1 前置（**必读**）

| 依赖 | 说明 |
|---|---|
| CodeFree `codefree-o` | 本插件宿主。`codefree-o --version` 能出版本号即可 |
| `cf-connect.exe` | 桥接网关。**包内 `package/bin/` 里带了一份**；若没有，去 cf-connect 发行 zip 取，或设置 `CF_CONNECT_BIN` 指向它 |
| 钉钉企业内部应用 | 需要 `client_id` / `client_secret`，机器人**消息接收模式选 Stream**（详见 QUICKSTART.md） |
| Node.js | 仅在你自己重打包时需要；最终用户只跑 exe |

> ⚠️ **打包前需放入 `plugin/bin/cf-connect.exe`**。这个路径被 `.gitignore` 的 `*.exe` / `bin/`
> 规则忽略，所以仓库里不会带二进制；从 cf-connect 发行 zip 里复制一份过去即可：
>
> ```powershell
> copy <解压后的发行目录>\cf-connect.exe plugin\bin\cf-connect.exe
> ```
>
> 没有它，`pack.mjs` 会打出一条警告，包仍然合法可发布（平台校验只看 `plugin.json`），
> 但用户装上后没有网关可用，只能自己另外准备二进制。

### 2.2 方式 A：比赛平台上传（推荐）

在 CodeFree 插件发布页选择本地 `.tgz` 上传：

```
dist/cf-connect-dingtalk-plugin-0.1.0.tgz
```

平台会解包并校验 `package/.codefree-plugin/plugin.json`；本仓库的
`verify-pack.mjs` 用同一套口径做过预检（见 §5）。

### 2.3 方式 B：手动解包到 srdplugins

```powershell
# 1. 解包
tar -xzf cf-connect-dingtalk-plugin-0.1.0.tgz -C $env:TEMP\cfplug

# 2. 放到加载器扫描的目录（目录名必须是 <name>@<version>）
$dst = "$env:USERPROFILE\.codefree-o\.config\srdplugins\cf-connect-dingtalk@0.1.0"
New-Item -ItemType Directory -Force $dst | Out-Null
Copy-Item -Recurse -Force "$env:TEMP\cfplug\package" $dst\

# 3. 重启 codefree-o，确认 skills/commands 已注入
codefree-o debug skill          # 应出现 dingtalk-bridge
```

仓库内的 `node plugin/scripts/install-local.mjs` 会自动完成第 1–2 步。

## 3. 配置

### 3.1 网关配置（config.toml）

```powershell
copy config.example.toml config.toml
# 填 4 处：work_dir、cmd、client_id、client_secret
cf-connect daemon install
cf-connect daemon start
```

配置样例里每个字段都有中文注释，只需要改这 4 处：

| 字段 | 作用 |
|---|---|
| `work_dir` | codefree-o 读写代码的目录 |
| `cmd` | 要驱动的 CLI（`codefree-o` 或绝对路径） |
| `client_id` / `client_secret` | 钉钉企业内部应用凭证 |

### 3.2 插件环境变量（全部可选）

| 变量 | 默认值 | 作用 |
|---|---|---|
| `CF_CONNECT_BIN` | 自动探测 | cf-connect 可执行文件绝对路径 |
| `CF_CONNECT_CONFIG` | 网关默认 | config.toml 路径 |
| `CF_CONNECT_DATA_DIR` | `~/.cf-connect` | 数据目录（会话、socket、日志） |
| `CF_CONNECT_NOTIFY` | `1` | `0` 关闭「一轮结束自动回推钉钉」 |
| `CF_CONNECT_SESSION_KEY` / `CF_CONNECT_PROJECT` | 空 | 默认推送目标会话 / 项目 |

自动探测顺序：`CF_CONNECT_BIN` → `package/bin/` → `~/.cf-connect/` → PATH。

### 3.3 插件行为

**装完即生效（加载器注入，不需要额外动作）：**

| 注入项 | 行为 |
|---|---|
| skill `dingtalk-bridge` | 告诉 agent **何时**推、**怎么**推、**失败时不要重试** |
| `/dingtalk-notify` | 斜杠命令：把 `$ARGUMENTS` 推送到钉钉当前会话 |
| `/dingtalk-status` | 斜杠命令：一键自检桥接链路（版本 / doctor / socket / 实发一条） |

**可选增强（`index.js`，需要在宿主 `plugin` 数组里注册后才生效）：**

| Hook / 工具 | 行为 |
|---|---|
| `config` | 启动时若 `~/.cf-connect/run/api.sock` 不存在，就 `cf-connect daemon start`（detached，只尝试一次） |
| `event` | 收到 `session.idle` 且网关在跑时，取该会话最后一条 assistant 文本，用 `cf-connect send --stdin` 回推钉钉；按 `sessionID + 文本摘要` 去重，失败只写日志抛不出去 |
| `tool: notify_dingtalk` | 让 agent 主动推一条 Markdown 到钉钉（参数：`message`、可选 `session` / `project`） |

## 4. 排障

日志：`~/.cf-connect/plugin.log`（插件）、`~/.cf-connect/` 下的网关日志。

| 报错 / 现象 | 原因 | 处置 |
|---|---|---|
| `package directory not found in extracted plugin` | tgz 根下不是 `package/` | 用 `scripts/pack.mjs` 重新打包（npm pack 布局） |
| `.codefree-plugin/plugin.json not found` | 路径不对 | 必须是 `package/.codefree-plugin/plugin.json` |
| `is not a JSON object` | plugin.json 顶层是数组 | 顶层必须是对象 `{...}` |
| `does not declare codefree compatibility` | `agentCompatibility` 不含 `codefree` 或含 `opencode-npm` | 改成 `["codefree"]` |
| `skipping ... outside packageRoot` | 包内有指向包外的符号链接 | 不要用符号链接，tgz 里必须是真实文件 |
| `cf-connect is not running (socket not found)` | 网关没启动 | `cf-connect daemon start`，再看 `~/.cf-connect/` 日志 |
| 钉钉里没收到消息 | 应用没开 Stream 模式 / 没发布 / 没加机器人 | 见 QUICKSTART.md 第 2 步 |
| SmartScreen 拦截 exe | 未签名 | 「更多信息 → 仍要运行」；企业内可统一签名 |

## 5. 校验与重新打包（开发者）

```powershell
cd plugin
node scripts/pack.mjs                 # → dist/cf-connect-dingtalk-plugin-0.1.0.tgz
node scripts/verify-pack.mjs          # 七项断言 + 打印 tgz 内完整文件清单
node scripts/make-zip.mjs             # 另出一份 .zip（内含同一个 tgz，便于翻阅）
node scripts/install-local.mjs        # 装到本机 srdplugins，做真机验证
node scripts/install-local.mjs --remove
```

`verify-pack.mjs` 复刻 codefree-o 1.7.0 二进制里的校验函数（变量名为混淆后，逐字摘录）：

```js
async function ul(n){                                   // n = 解包后的插件根目录
  let o = join(n, "package");
  if (!exists(o)) return {ok:false, error:`package directory not found in extracted plugin: ${o}`};
  let c = join(o, ".codefree-plugin", "plugin.json");
  if (!exists(c)) return {ok:false, error:`.codefree-plugin/plugin.json not found: ${c}`};
  let u; try { u = readJson(c) } catch { return {ok:false, error:`failed to parse ...`} }
  if (typeof u!=="object" || u===null || Array.isArray(u))
    return {ok:false, error:`.codefree-plugin/plugin.json is not a JSON object: ${c}`};
  let r = u.agentCompatibility;
  if (!Array.isArray(r) || !r.includes("codefree") || r.includes("opencode-npm"))
    return {ok:false, error:"plugin does not declare codefree compatibility in agentCompatibility"};
  return {ok:true};
}
```

真机验证记录（本机 codefree-o 1.7.0，装到 `srdplugins/cf-connect-dingtalk@0.1.0` 之后）：

| 命令 | 结果 |
|---|---|
| `codefree-o debug skill` | 列表中出现 `dingtalk-bridge`，location 指向 `…/srdplugins/cf-connect-dingtalk@0.1.0/package/skills/dingtalk-bridge/SKILL.md` |
| `codefree-o debug config` | `command` 段出现 `dingtalk-status` / `dingtalk-notify`；`skills.paths` 出现 `…/srdplugins/cf-connect-dingtalk@0.1.0/package/skills` |
| 加载日志 | 无 `not found` / `does not declare codefree compatibility` / `is not a JSON object` / `outside packageRoot` 报错 |

## 6. 卸载 / 回滚

```powershell
# 1. 卸载插件
node plugin/scripts/install-local.mjs --remove
#    或手动删目录：
Remove-Item -Recurse -Force "$env:USERPROFILE\.codefree-o\.config\srdplugins\cf-connect-dingtalk@0.1.0"

# 2. 停掉并卸载网关服务（可选）
cf-connect daemon stop
cf-connect daemon uninstall

# 3. 彻底清干净（会丢掉会话历史）
Remove-Item -Recurse -Force "$env:USERPROFILE\.cf-connect"
```

插件不改动 `codefree-o` 的任何原生文件：注入的 skills / commands 只在加载期生效，
删掉 `srdplugins` 下的目录即完全回滚。

## 7. 版本与许可

- 插件版本 **0.1.0**（`plugin.json` = `package.json` = tgz 文件名）
- 网关 `cf-connect` 版本 **v0.1.0**
- MIT
