# Usage Guide

Complete guide to using cf-connect features.

## Table of Contents

- [Session Management](#session-management)
- [Permission Modes](#permission-modes)
- [API Provider Management](#api-provider-management)
- [Model Selection](#model-selection)
- [Work Directory Switching (`/dir`, `/cd`)](#work-directory-switching-dir-cd)
- [Voice Messages (STT)](#voice-messages-speech-to-text)
- [Voice Reply (TTS)](#voice-reply-text-to-speech)
- [Image and File Send-Back](#image-and-file-send-back)
- [Scheduled Tasks (Cron)](#scheduled-tasks-cron)
- [Shell Configuration](#shell-configuration)
- [Multi-Bot Relay](#multi-bot-relay)
- [Daemon Mode](#daemon-mode)
- [Multi-Workspace Mode](#multi-workspace-mode)
- [Web Admin Dashboard (Beta)](#web-admin-dashboard-beta)
- [Bridge — External Adapter Access (Beta)](#bridge--external-adapter-access-beta)
- [Configuration Reference](#configuration-reference)

---

## Session Management

Each user gets an independent session with full conversation context. Manage sessions via slash commands:

| Command | Description |
|---------|-------------|
| `/new [name]` | Start a new session |
| `/list` | List all agent sessions for this project |
| `/switch <id>` | Switch to a different session |
| `/current` | Show current session info |
| `/history [n]` | Show last n messages (default 10; each entry follows `[display].history_max_len`, default 1000) |
| `/usage` | Show account/model quota usage (if supported) |
| `/provider [...]` | Manage API providers |
| `/model [switch <alias>]` | List available models or switch by alias |
| `/dir [path]` | Show or switch the agent work directory |
| `/allow <tool>` | Pre-allow a tool (next session) |
| `/mode [name]` | View or switch permission mode |
| `/stop` | Stop current execution |
| `/help` | Show available commands |

During a session, the agent may request tool permissions. Reply **allow** / **deny** / **allow all**.

cf-connect rotates to a fresh session automatically after long inactivity:

```toml
[[projects]]
name = "demo"
reset_on_idle_mins = 30   # default when unset; set to 0 to disable
```

The next normal message after a long idle period starts in a fresh session automatically, without deleting the old session from `/list`.

**Why this is on by default:** without idle rotation, every workspace-pool eviction (~15 min) caused the next message to resume the previous transcript via `--continue`. Over many cycles this re-ingests stale chat history (failed commands, debugging noise, abandoned tangents) and the model's attention drifts away from the original intent. Rotating after 30 minutes of user inactivity gives a clean slate when you come back to a task, while preserving the old session for `/list` and `/switch`.

To restore the previous behavior of always continuing, set `reset_on_idle_mins = 0`.

### Model switch preserves history

`/model` preserves the current session — the agent resumes the conversation with the new model (no extra token cost). Model switching affects the shared agent instance — if multiple platforms use the same project, the model change applies to all of them.

---

## Permission Modes

Permission modes are switchable at runtime via `/mode`.

### CodeFree-O / OpenCode Modes

| Mode | Config Value | Behavior |
|------|-------------|----------|
| Default | `default` | Every risky tool call requires approval |
| YOLO | `yolo` (aliases: `auto`, `force`, `bypasspermissions`) | Auto-approve all tool calls |

In `yolo` mode the adapter appends the flag configured by
`permission_flag` (default `--auto`). Both current OpenCode (>= 1.18) and
CodeFree-O accept `--auto`. Set the option to
`--dangerously-skip-permissions` if you run an older OpenCode build, or to
`none` to append nothing.

### Configuration

```toml
[projects.agent.options]
mode = "default"
# permission_flag = "--auto"   # only used by mode = "yolo"
```

Switch at runtime:
```
/mode          # show current and available modes
/mode yolo     # switch to YOLO mode
/mode default  # switch back
```

---

## API Provider Management

Switch between API providers at runtime without restart.

### Configure Providers

```toml
[projects.agent.options]
work_dir = "/path/to/project"
provider = "anthropic"   # active provider

[[projects.agent.providers]]
name = "anthropic"
api_key = "sk-ant-xxx"

[[projects.agent.providers]]
name = "relay"
api_key = "sk-xxx"
base_url = "https://api.relay-service.com"
model = "claude-sonnet-4-20250514"

[[projects.agent.providers.models]]
model = "claude-sonnet-4-20250514"
alias = "sonnet"

[[projects.agent.providers.models]]
model = "claude-opus-4-20250514"
alias = "opus"

[[projects.agent.providers.models]]
model = "claude-haiku-3-5-20241022"
alias = "haiku"

# MiniMax — OpenAI-compatible agent provider, 1M context
[[projects.agent.providers]]
name = "minimax"
api_key = "your-minimax-api-key"
# Use https://api.minimaxi.com/v1 for China-region accounts.
base_url = "https://api.minimax.io/v1"
model = "MiniMax-M2.7"

# For Bedrock, Vertex, etc.
[[projects.agent.providers]]
name = "bedrock"
env = { CLAUDE_CODE_USE_BEDROCK = "1", AWS_PROFILE = "bedrock" }
```

### CLI Commands

```bash
cf-connect provider add --project my-backend --name relay --api-key sk-xxx --base-url https://api.relay.com
cf-connect provider list --project my-backend
cf-connect provider remove --project my-backend --name relay
cf-connect provider import --project my-backend  # from cc-switch
```

### Chat Commands

```
/provider                   Show current provider
/provider list              List all providers
/provider add <name> <key> [url] [model]
/provider remove <name>
/provider switch <name>
/provider <name>            Shortcut for switch
```

### Env Var Mapping

| Agent | api_key → | base_url → |
|-------|-----------|------------|
| CodeFree-O / OpenCode | `ANTHROPIC_API_KEY` | `ANTHROPIC_BASE_URL` (or use the `env` map) |
| OpenCode | `ANTHROPIC_API_KEY` | use `env` map |

---

## Model Selection

Pre-configure a list of selectable models per provider using `[[providers.models]]`. Each entry has a `model` identifier and an optional `alias` (short name shown in `/model`).

### Configure Models

```toml
[[projects.agent.providers]]
name = "openai"
api_key = "sk-xxx"

[[projects.agent.providers.models]]
model = "gpt-5.3-codex"
alias = "codex"

[[projects.agent.providers.models]]
model = "gpt-5.4"
alias = "gpt"

[[projects.agent.providers.models]]
model = "gpt-5.3-codex-spark"
alias = "spark"
```

### Chat Commands

```
/model              List available models (format: alias - model)
/model switch <alias>      Switch to the model matching the alias
/model switch <name>       Switch to the model by its full name
/model <alias>             Legacy syntax, still supported
```

When `models` is configured, `/model` shows exactly that list without making an API round-trip. When omitted, models are fetched from the provider API or fall back to a built-in list.

---

## Work Directory Switching (`/dir`, `/cd`)

Switch where the next agent session starts, directly from chat.

### Chat Commands

```
/dir                    Show current work directory and recent history
/dir <path>             Switch to a path (relative or absolute)
/dir <number>           Switch to a directory from history
/dir -                  Switch back to previous directory
/dir help               Show command usage
/cd <path>              Backward-compatible alias of /dir <path>
```

### Behavior Notes

- `/dir` is a privileged command. You must set `admin_from` under `[[projects]]` in `config.toml` before it can be used.
- Do not put `admin_from` under `[projects.platforms.options]`, or it will be ignored.
- Use `/whoami` or `/status` to get your current `User ID`, then place that ID into `admin_from`.
- If you are the only user of this bot, `admin_from = "*"` also works, but it grants every allowed user privileged command access.
- Restart `cf-connect` after updating `config.toml`.
- Directory changes apply to the next session in the current project.
- Relative paths are resolved from the current agent work directory.
- Directory history is project-scoped and can be switched by index.
- `/cd` is kept for compatibility, but `/dir` is the primary command.

Example config:

```toml
[[projects]]
name = "my-project"
admin_from = "ou_xxx"
```

Examples:

```text
/dir ../another-repo
/dir 2
/dir -
```

---

## Running agents as a different Unix user (`run_as_user`)

> **Platform support**: Linux and macOS. Not supported on Windows.
> **Agent support**: works with any agent whose CLI accepts `--dir`
> (CodeFree-O and OpenCode do). Other agents fall back to the
> supervisor user; see the tracking issue for migration status.

### What this is

By default, every agent session cf-connect spawns runs as the same Unix
user that runs `cf-connect` itself. If an agent misbehaves — reads a
secret, overwrites a sibling repo, trashes `~/.ssh/` — it has the
supervisor user's full file-system reach.

`run_as_user` sets a per-project target Unix user. When it is set,
cf-connect spawns that project's agent command via

```
sudo -n -iu <target-user> -- claude ...
```

The target user is a real, unprivileged Unix account that you create.
The agent runs under that account's uid/gid, with **its own** home
directory, shell profile, PATH, and tool credentials. File-system
isolation is enforced by the kernel, not by hooks or allowlists.

### Security guarantee and non-guarantee

**This provides OS-user isolation from any file or process the target
user cannot reach.** An agent can no longer read or clobber the
supervisor's `~/.ssh/`, another project user's `~/.pgpass`, or a repo
whose UNIX permissions don't grant access to the target user.

**This does not automatically isolate projects from each other** if they
share the same `run_as_user`. If you want per-project isolation, create
a separate Unix user per project.

**This is not a sandbox in the sense of Linux namespaces, seccomp, or
container isolation.** It is strictly file-system scoping by uid.

### Setup

#### 1. Create the target user and install their tooling

The target user needs its own copy of everything the agent touches,
because `sudo -i` loads the *target* user's login environment — not the
supervisor's.

```bash
sudo useradd -m -s /bin/bash partseeker-coder
sudo -iu partseeker-coder

# Install the agent CLI under the target user's PATH
#   (install the CodeFree-O / OpenCode CLI for that user)

# Set up the target user's ~/.claude/
mkdir -p ~/.claude
# Copy or re-create:
#   ~/.claude/settings.json     (MCP servers, hooks, model settings)
#   the CLI's own auth/config under that user's home
#   ~/.claude/plugins/          (claude-mem and any other plugin state)

exit
```

#### 2. Grant the supervisor passwordless sudo to the target

Add a scoped sudoers rule. Do **not** use `NOPASSWD: ALL` for the
supervisor — that grants the supervisor root, which is irrelevant here
and dangerous.

```
# /etc/sudoers.d/cf-connect (install with `sudo visudo -f ...`)
partseeker-orchestrator ALL=(partseeker-coder) NOPASSWD: ALL
```

Adjust the usernames for your setup. The rule says: *"the supervisor
user may run any command as this specific target user, without a
password."*

#### 3. Verify the target user cannot sudo

The whole point of stepping down into a target user is that the target
cannot immediately escalate back. Verify:

```bash
sudo -n -iu partseeker-coder -- sudo -n true
# must FAIL with "a password is required" or similar
```

If that command succeeds, cf-connect will refuse to start. Remove any
`NOPASSWD` sudo grants for the target user first.

#### 4. Make the project's `work_dir` accessible to the target user

The target user needs read AND write on the project's `work_dir`. If
the directory is owned by the supervisor, either `chown` it to the
target, add group ownership the target is in, or apply a POSIX ACL:

```bash
sudo setfacl -R -m u:partseeker-coder:rwX /home/leigh/workspace/sandboxed-repo
sudo setfacl -R -dm u:partseeker-coder:rwX /home/leigh/workspace/sandboxed-repo
```

cf-connect refuses to start if the target user cannot read+write the
`work_dir` root, and warns (non-fatal) for descendant paths that look
inaccessible.

#### 5. Audit the setup before starting cf-connect

```bash
cf-connect doctor user-isolation
```

This runs the full preflight (the three go/no-go gates from
[#496](https://github.com/ItsQifan/cf-connect/issues/496)) and an
**isolation probe**: it spawns a fixed shell script as the target user
and reports what the target can read, what it's denied, and any
cross-user leaks. Output goes to stdout plus a JSON report in
`~/.cf-connect/audits/<timestamp>-<project>.json`.

Exit code 0 = clean. Exit code 1 = at least one fatal problem.

You can inspect the probe script itself with:

```bash
cf-connect doctor user-isolation --print-script
```

### Configuration

```toml
[[projects]]
name = "claude-sandboxed"
run_as_user = "partseeker-coder"

# Optional: extend the default env var allowlist that crosses the sudo
# boundary. The defaults (PATH, LANG, LC_*, TERM) are always included.
# Only list vars the target user cannot reasonably set in their own
# shell profile. Secrets belong in the target user's ~/.claude/settings.json
# env block, NOT here.
run_as_env = ["PGSSLROOTCERT", "PGSSLMODE"]

[projects.agent]
type = "opencode"

[projects.agent.options]
mode = "default"
model = "sonnet"
work_dir = "/home/leigh/workspace/sandboxed-repo"
```

### Environment propagation: what moves into the target user's home

This is the 2am-debugging section. When you switch a project to
`run_as_user`, the supervisor's environment is **not** forwarded across
the sudo boundary — that's the whole point. Everything the agent needs
has to live in the target user's home.

Migration checklist:

- [ ] **Agent config** — `~/.claude/settings.json` (MCP servers, hooks,
      model settings), `~/.claude.json` (auth). Copy from the supervisor
      or re-create from scratch.
- [ ] **Plugin state** — `~/.claude/plugins/` — claude-mem, any other
      agent plugins.
- [ ] **MCP server binaries** — must be on the target user's `PATH`, not
      just the supervisor's. Either install under the target user or
      reference full paths in `settings.json`.
- [ ] **Postgres TLS** — `PGSSLROOTCERT`, `PGSSLCERT`, `PGSSLKEY` belong
      in the target user's `~/.claude/settings.json` `env` block. Their
      referenced cert files must be readable by the target user.
- [ ] **Claude OAuth credentials** — if you authenticate via `claude.ai`
      (OAuth), the token lives in `~/.claude/.credentials.json`. OAuth
      access tokens expire after a few hours and are refreshed
      automatically by whichever Claude CLI session is running. The
      target user's token will **not** be refreshed unless the target
      user has an active session — which it often doesn't between
      cf-connect spawns. The recommended fix is to symlink the target
      user's credentials to the supervisor's file so both share one
      token that stays fresh:

      ```bash
      # Grant target user read access via ACL (keeps 600 for everyone else)
      setfacl -m u:<target-user>:rx ~/.claude/
      setfacl -m u:<target-user>:r  ~/.claude/.credentials.json

      # Replace the target user's credentials with a symlink
      sudo -iu <target-user> bash -c \
        'rm -f ~/.claude/.credentials.json && \
         ln -s /home/<supervisor>/.claude/.credentials.json \
               ~/.claude/.credentials.json'
      ```

      **If you use an API key** (`ANTHROPIC_API_KEY`) instead of OAuth,
      this is not an issue — set the key in the target user's
      `~/.claude/settings.json` `env` block and it won't expire.
- [ ] **Credential files** — `~/.pgpass`, `~/.gitconfig`, `~/.netrc`,
      `~/.aws/`, `~/.config/gh/`, `~/.kube/` — whichever the agent
      actually uses. Each needs its own copy or a group-readable shared
      copy.
- [ ] **SSH keys** — `~/.ssh/id_ed25519` etc., if the agent runs `git
      push` over SSH. Same story: copy or group-share.
- [ ] **Key material under** `~/keys/` — custom directories the
      supervisor uses need an equivalent under the target user's home
      or a group-readable shared copy.
- [ ] **Language toolchains** — if the agent uses `asdf`, `mise`, `nvm`,
      `rustup`, etc., those live in `~`. The target user needs either
      its own install or a system-wide install that both users can run.
- [ ] **Shell profile** — `~/.profile` / `~/.bashrc` on the target user
      needs to set `PATH` and any tool init the agent depends on. Test
      with `sudo -iu partseeker-coder` before wiring cf-connect.

After migration, run `cf-connect doctor user-isolation` again. The
`target home` section reports which expected paths are present and
which are missing — missing isn't necessarily wrong, but it's your
checklist.

### Opting out

Remove `run_as_user` from the project entry, or set it to `""`. Legacy
behavior (spawn as supervisor) returns on the next restart.

### Failure modes and error messages

- **"passwordless sudo to user X is not configured"** — step 2 of setup
  is missing or the sudoers rule is scoped to the wrong supervisor. Fix
  the rule, run `visudo -c` to validate syntax, then restart cf-connect.
- **"target user X can run passwordless sudo"** — step 3 failed. The
  error includes the output of `sudo -l` from the target context; find
  the offending rule and remove it.
- **"target user X cannot read AND write work_dir Y"** — step 4 failed.
  `chown` the directory or add an ACL as shown above.
- **"CROSS_LEAKED"** or **"SUPERVISOR_LEAKED"** in the audit — the
  target user can read another user's secrets. Tighten the offending
  file's permissions (usually `chmod 600 file; chown user:user file`)
  and re-audit.
- **"descendant scan timed out"** — non-fatal. The `work_dir` is large
  enough that the permission walk exceeded its timeout. Run
  `cf-connect doctor user-isolation` manually if you want the full
  walk, or narrow the project's `work_dir`.

---

# Ubuntu/Debian
sudo apt install ffmpeg

# macOS
brew install ffmpeg
```

---

## Voice Reply (Text-to-Speech)

Synthesize AI replies into voice messages.

**Supported:** platforms that implement audio sending - in this build, DingTalk.

### Configure

```toml
[tts]
enabled = true
provider = "minimax"     # qwen | openai | minimax | mimo | espeak | pico | edge
voice_id = "Chinese (Mandarin)_Crisp_Girl"
speed = 0.98             # provider-specific range; MiniMax commonly accepts 0.5-2.0
tts_mode = "voice_only"  # "voice_only" | "always"
max_text_len = 0         # 0 = no limit

[tts.minimax]
api_key = ""             # optional: falls back to data_dir/config/minimax.json
base_url = ""            # optional: default https://api.minimaxi.com
model = "speech-2.8-hd"

[tts.agents.assistant]
voice_id = "Chinese (Mandarin)_Crisp_Girl"
speed = 0.98

[tts.agents.reviewer]
voice_id = "Chinese (Mandarin)_Gentle_Senior"
speed = 0.96
```

### TTS Modes

| Mode | Behavior |
|------|----------|
| `voice_only` | Reply with voice only when user sends voice |
| `always` | Always send voice reply |

Switch: `/tts always` or `/tts voice_only`

---

## Image, File, and Voice Send-Back

When an agent generates a local image, PDF, report, bundle, or other file and needs to deliver it directly to the current chat, use attachment mode in `cf-connect send`. When the user explicitly asks for a voice message, the agent can also send synthesized speech through the same CLI.

**Currently supported platforms:**
- DingTalk

### When to run setup first

If the current agent does not natively inject the system prompt, run this once in chat after upgrading:

```text
/bind setup
```

or:

```text
/cron setup
```

These two commands write the same cf-connect instructions. Either one is enough. After that, the agent knows:
- normal text replies should be returned normally
- generated attachments should be sent back with `cf-connect send --image/--file`
- requested voice messages should be sent with `cf-connect send --tts`

If you have run setup before, run it again after upgrading so the instructions are refreshed to the latest version.

### Config switch

Add this to `config.toml` if you want to disable agent-driven attachment send-back:

```toml
attachment_send = "off"
```

The default is `on`. This switch is independent from the agent's `/mode` and only affects `cf-connect send --image/--file`. Synthesized voice send-back uses the `[tts]` provider config and is controlled by TTS availability instead.

### CLI examples

```bash
cf-connect send --image /absolute/path/to/chart.png
cf-connect send --file /absolute/path/to/report.pdf
cf-connect send --file /absolute/path/to/report.pdf --image /absolute/path/to/chart.png
cf-connect send --tts "Hello from cf-connect"
```

Notes:
- `--image` is for image attachments.
- `--file` is for any file attachment.
- `--tts` synthesizes text and sends the generated audio through the active TTS provider.
- `--message` is optional and sends a text note before the attachments.
- `--image` and `--file` can both be repeated.
- Absolute paths are recommended so the command does not depend on the agent's current working directory.
- With `attachment_send = "off"`, image/file send-back is blocked but ordinary text replies still work.
- Each attachment is capped at **50 MiB** by default. Configure it with `max_attachment_size_mb` (MiB) in config.toml, or override that value with the `CC_MAX_ATTACHMENT_SIZE_MB` env var (same MiB unit; takes precedence when set), e.g. `CC_MAX_ATTACHMENT_SIZE_MB=100 cf-connect send --file big.bin`.

### Typical use cases

1. The agent generates a screenshot or chart and should send it directly to the user.
2. The agent generates a PDF, Markdown export, log bundle, or patch file that should be delivered as an attachment.
3. The agent wants to send a short status message together with one or more generated files.
4. The user asks the agent to "send this as voice" without typing a slash command.

### Important notes

- This command is for generated attachment and voice delivery, not ordinary text replies.
- The files must exist on the local machine where the agent runs.
- There must be an active session; otherwise the command fails because cf-connect has no chat context to deliver to.
- The target platform also enforces its own file-size/type limit at delivery; the effective per-attachment ceiling is the smaller of that limit and `max_attachment_size_mb` (a file that passes cf-connect may still be rejected by the platform).

---

## Scheduled Tasks (Cron)

Create scheduled tasks that run automatically.

### Chat Commands

```
/cron                                          List all jobs
/cron add <min> <hour> <day> <mon> <wk> <prompt>   Create job
/cron del <id>                                 Delete job
/cron enable <id>                              Enable job
/cron disable <id>                             Disable job
```

Example:
```
/cron add 0 6 * * * Summarize GitHub trending repos
```

### CLI Commands

```bash
cf-connect cron add --cron "0 6 * * *" --prompt "Summarize GitHub trending" --desc "Daily Trending"
cf-connect cron list
cf-connect cron edit <job-id> <field> <value>   # e.g. cron_expr, prompt, enabled, mute, timeout_mins
cf-connect cron exec <job-id>
cf-connect cron del <job-id>
```

Optional: `--session-mode new-per-run` starts a fresh agent session on each run (default is `reuse`, same as before). `--timeout-mins N` sets how long the scheduler waits per run (`0` = no limit; omit = 30 minutes).

### Natural Language

> "Every day at 6am, summarize GitHub trending"

Agents that write cron entries from a memory file need `/cron setup` or `/bind setup` run once first; both write the same instructions.

---

## Shell Configuration

By default, cf-connect uses `sh` on Unix and `powershell.exe` on Windows for all shell execution (`/shell` commands, cron exec jobs, hooks, and webhook exec). You can override this to use a different shell.

### Supported Shells

| Shell | Config value | Flag |
|-------|-------------|------|
| sh (default on Unix) | `sh` | `-c` |
| bash | `/bin/bash` | `-c` |
| zsh | `/bin/zsh` | `-c` |
| fish | `/bin/fish` | `-c` |
| cmd (Windows) | `cmd` | `/C` |
| PowerShell (default on Windows) | `powershell.exe` | `-Command` |
| PowerShell Core | `pwsh` | `-Command` |

The flag is auto-detected from the shell name — no manual configuration needed.

### Global Configuration

Set `shell` at the top level of `config.toml` to change the default for all projects:

```toml
shell = "/bin/zsh"
```

### Per-Project Override

Override the shell for a specific project:

```toml
[[projects]]
name = "my-project"
shell = "/bin/fish"
```

### Shell Profile

Use `shell_profile` to prepend a setup script to every shell command. This is useful for sourcing your shell profile so that custom functions, aliases, and environment variables are available:

```toml
shell = "/bin/zsh"
shell_profile = "source ~/.zshrc"
```

The shell profile and the user's command are joined with a newline and passed as a single script to the shell, avoiding quoting issues. For example, `/shell echo $MY_VAR` becomes:

```zsh
source ~/.zshrc
echo $MY_VAR
```

`shell_profile` also supports per-project override:

```toml
[[projects]]
name = "my-project"
shell = "/bin/fish"
shell_profile = "source ~/.config/fish/config.fish"
```

### Affected Execution Paths

The shell configuration applies to all command execution in cf-connect:

- **`/shell` command** — interactive shell commands from chat
- **Cron exec jobs** — `[[cron]]` entries with `exec` field
- **Hooks** — `[[hooks]]` entries with `type = "command"`
- **Webhook exec** — webhook requests with `exec` field

---

## Multi-Bot Relay

Cross-platform bot communication in group chats.

### Group Chat Binding

```
/bind              Show bindings
/bind my-project    Add a project
/bind other-proj    Add another project
/bind -my-project   Remove a project
```

### Bot-to-Bot Communication

```bash
cf-connect relay send --to other-proj "What do you think about this architecture?"
```

---

## Daemon Mode

Run as background service.

```bash
cf-connect daemon install --config ~/.cf-connect/config.toml
cf-connect daemon start
cf-connect daemon stop
cf-connect daemon restart
cf-connect daemon status
cf-connect daemon logs [-f]
cf-connect daemon uninstall
```

---

## Multi-Workspace Mode

One bot serving multiple workspaces per channel.

### Configure

```toml
[[projects]]
name = "my-project"
mode = "multi-workspace"
base_dir = "~/workspaces"

[projects.agent]
type = "opencode"
```

### Commands

```
/workspace                    Show current binding
/workspace bind <name>        Bind local folder
/workspace init <git-url>     Clone and bind repo
/workspace unbind             Remove binding
/workspace list               List all bindings
```

### How It Works

- Channel name `#project-a` → auto-binds to `base_dir/project-a/`
- Each channel has isolated sessions and agent state

---

## Web Admin Dashboard (Beta)

> **Status: Beta.** This feature is available since v1.2.2-beta.5. The UI and API may change in future releases.

A full-featured management UI embedded in the binary — project CRUD, session management, cron job editor, global settings, chat interface, and i18n support.

### Quick Setup (Chat Command)

The easiest way to enable web admin:

```
/web setup
```

This automatically enables both the **Management API** and the **Bridge** in `config.toml`, generates tokens, and prints the access URL. You may need to run `/restart` for changes to take effect.

After setup, open the URL shown (default `http://localhost:9820`) and log in with the token.

### Check Status

```
/web           # or /web status — show current web admin URL and status
```

### Manual Configuration

Add the following to `config.toml`:

```toml
[management]
enabled = true
port = 9820                     # Management UI & API listen port
token = "your-secret-token"     # Login token; /web setup generates one automatically
cors_origins = ["*"]            # Allowed CORS origins; empty = no CORS headers
```

Then restart cf-connect.

### Build Options

Web assets are compiled into the binary by default. To exclude them (saves ~1MB):

```bash
make build-noweb
# or
go build -tags 'no_web' ./cmd/cf-connect
```

When built with `no_web`, the `/web` command will report that web admin is not available.

### Management API

The Management API is served on the same port as the UI. Base URL: `http://<host>:<port>/api/v1`

All API requests require the `Authorization: Bearer <token>` header.

Key endpoints:

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/status` | System status (version, uptime, platforms) |
| `POST` | `/api/v1/restart` | Restart cf-connect |
| `POST` | `/api/v1/reload` | Reload configuration |
| `GET` | `/api/v1/projects` | List projects |
| `GET` | `/api/v1/sessions?project=<name>` | List sessions for a project |
| `GET` | `/api/v1/cron` | List cron jobs |
| `GET` | `/api/v1/settings` | Get global settings |
| `PATCH` | `/api/v1/settings` | Update global settings |

Full API reference: [management-api.md](./management-api.md)

---

## Bridge — External Adapter Access (Beta)

> **Status: Beta.** This feature is available since v1.2.2-beta.5. The protocol may change in future releases.

The Bridge exposes a WebSocket + REST server so external adapters (custom UIs, bots, scripts) can interact with cf-connect sessions — send messages, receive events, manage sessions.

### Enable via Chat

The `/web setup` command enables Bridge automatically alongside the Management API.

### Manual Configuration

Add the following to `config.toml`:

```toml
[bridge]
enabled = true
port = 9810                     # Bridge listen port (separate from management)
token = "your-bridge-secret"    # Auth token for WebSocket and REST
path = "/bridge/ws"             # WebSocket endpoint path
cors_origins = ["*"]            # Allowed CORS origins; empty = no CORS
```

Then restart cf-connect.

### Authentication

All Bridge connections require a token. Supported methods:

- Query parameter: `?token=<bridge-token>`
- Header: `Authorization: Bearer <bridge-token>`
- Header: `X-Bridge-Token: <bridge-token>`

### WebSocket

Connect to:

```
ws://<host>:<bridge-port>/bridge/ws?token=<bridge-token>
```

The WebSocket supports bidirectional messaging — send user messages to the agent and receive agent events (text, tool calls, permission requests, etc.) in real time.

### REST API

Served on the same port as the WebSocket.

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/bridge/sessions?session_key=...&project=...` | List sessions |
| `POST` | `/bridge/sessions` | Create a new session |
| `GET` | `/bridge/sessions/{id}?session_key=...&project=...` | Get session detail + history |
| `DELETE` | `/bridge/sessions/{id}?session_key=...&project=...` | Delete a session |
| `POST` | `/bridge/sessions/switch` | Switch active session |

Full protocol reference: [bridge-protocol.md](./bridge-protocol.md)

### Port Summary

| Service | Default Port | Config Block |
|---------|-------------|--------------|
| Management (Web UI + API) | 9820 | `[management]` |
| Bridge (WebSocket + REST) | 9810 | `[bridge]` |

---

## Configuration Reference

See [config.example.toml](../config.example.toml) for full examples.

### Project Structure

```toml
[[projects]]
name = "my-project"

[projects.agent]
type = "opencode"    # or "codefree-o" (same adapter)

[projects.agent.options]
work_dir = "/path/to/project"
mode = "default"
provider = "anthropic"

[[projects.platforms]]
type = "dingtalk"    # the only platform in this build

[projects.platforms.options]
# platform-specific options
```

---

## FAQ

Quick answers to questions that came up repeatedly in issues and that the
maintainers have resolved. Each entry links back to the originating issue
or PR so you can dig further if needed.

