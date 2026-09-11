package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name:    "requires at least one project",
			cfg:     Config{},
			wantErr: "at least one [[projects]] entry is required",
		},
		{
			name: "requires project name",
			cfg: Config{
				Projects: []ProjectConfig{
					validProject(""),
				},
			},
			wantErr: `projects[0].name is required`,
		},
		{
			name: "requires agent type",
			cfg: Config{
				Projects: []ProjectConfig{
					func() ProjectConfig {
						p := validProject("demo")
						p.Agent.Type = ""
						return p
					}(),
				},
			},
			wantErr: `projects[0].agent.type is required`,
		},
		{
			name: "requires at least one platform",
			cfg: Config{
				Projects: []ProjectConfig{
					func() ProjectConfig {
						p := validProject("demo")
						p.Platforms = nil
						return p
					}(),
				},
			},
			wantErr: `projects[0] needs at least one [[projects.platforms]]`,
		},
		{
			name: "requires platform type",
			cfg: Config{
				Projects: []ProjectConfig{
					func() ProjectConfig {
						p := validProject("demo")
						p.Platforms[0].Type = ""
						return p
					}(),
				},
			},
			wantErr: `projects[0].platforms[0].type is required`,
		},
		{
			name: "multi workspace requires base dir",
			cfg: Config{
				Projects: []ProjectConfig{
					func() ProjectConfig {
						p := validProject("demo")
						p.Mode = "multi-workspace"
						return p
					}(),
				},
			},
			wantErr: `project "demo": multi-workspace mode requires base_dir`,
		},
		{
			name: "multi workspace rejects work dir",
			cfg: Config{
				Projects: []ProjectConfig{
					func() ProjectConfig {
						p := validProject("demo")
						p.Mode = "multi-workspace"
						p.BaseDir = "~/workspace"
						p.Agent.Options["work_dir"] = "/tmp/demo"
						return p
					}(),
				},
			},
			wantErr: `project "demo": multi-workspace mode conflicts with agent work_dir`,
		},
		{
			name: "accepts valid config",
			cfg: Config{
				Projects: []ProjectConfig{validProject("demo")},
			},
		},
		{
			name: "accepts valid references config",
			cfg: Config{
				Projects: []ProjectConfig{
					func() ProjectConfig {
						p := validProject("demo")
						p.References = ReferenceConfig{
							NormalizeAgents: []string{"codex", "claudecode"},
							RenderPlatforms: []string{"feishu", "weixin"},
							DisplayPath:     "dirname_basename",
							MarkerStyle:     "emoji",
							EnclosureStyle:  "code",
						}
						return p
					}(),
				},
			},
		},
		{
			name: "rejects unsupported reference agent",
			cfg: Config{
				Projects: []ProjectConfig{
					func() ProjectConfig {
						p := validProject("demo")
						p.References.NormalizeAgents = []string{"gemini"}
						return p
					}(),
				},
			},
			wantErr: `projects[0].references.normalize_agents has unsupported value "gemini"`,
		},
		{
			name: "rejects unsupported reference platform",
			cfg: Config{
				Projects: []ProjectConfig{
					func() ProjectConfig {
						p := validProject("demo")
						p.References.RenderPlatforms = []string{"telegram"}
						return p
					}(),
				},
			},
			wantErr: `projects[0].references.render_platforms has unsupported value "telegram"`,
		},
		{
			name: "rejects unsupported reference display path",
			cfg: Config{
				Projects: []ProjectConfig{
					func() ProjectConfig {
						p := validProject("demo")
						p.References.DisplayPath = "full"
						return p
					}(),
				},
			},
			wantErr: `projects[0].references.display_path has unsupported value "full"`,
		},
		{
			name: "accepts all shorthand in references scopes",
			cfg: Config{
				Projects: []ProjectConfig{
					func() ProjectConfig {
						p := validProject("demo")
						p.References.NormalizeAgents = []string{"all"}
						p.References.RenderPlatforms = []string{"all"}
						return p
					}(),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validate() unexpected error: %v", err)
				}
				return
			}
			assertErrContains(t, err, tt.wantErr)
		})
	}
}

func TestRunAsEnv_RejectsDangerousVars(t *testing.T) {
	dangerous := []string{"PATH", "path", "LD_PRELOAD", "HOME", "USER", "SHELL", "SUDO_USER", "SUDO_COMMAND", "LD_LIBRARY_PATH", "DYLD_INSERT_LIBRARIES"}
	for _, v := range dangerous {
		err := validateRunAsEnv("projects[0]", []string{v})
		if err == nil {
			t.Errorf("validateRunAsEnv(%q) = nil, want error", v)
		}
	}

	safe := []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "CUSTOM_VAR"}
	for _, v := range safe {
		err := validateRunAsEnv("projects[0]", []string{v})
		if err != nil {
			t.Errorf("validateRunAsEnv(%q) = %v, want nil", v, err)
		}
	}
}

func TestEffectiveDisplayQuiet(t *testing.T) {
	tru, fal := true, false
	compact := DisplayModeCompact
	quiet := DisplayModeQuiet
	tests := []struct {
		name     string
		cfg      Config
		proj     ProjectConfig
		wantMode string
		wantTM   bool
		wantTool bool
	}{
		{
			name:     "defaults no quiet",
			cfg:      Config{},
			proj:     ProjectConfig{},
			wantMode: "full",
			wantTM:   true,
			wantTool: true,
		},
		{
			name:     "global quiet maps to quiet mode",
			cfg:      Config{Quiet: &tru},
			proj:     ProjectConfig{},
			wantMode: "quiet",
			wantTM:   false,
			wantTool: false,
		},
		{
			name:     "project quiet maps to quiet mode",
			cfg:      Config{},
			proj:     ProjectConfig{Quiet: &tru},
			wantMode: "quiet",
			wantTM:   false,
			wantTool: false,
		},
		{
			name: "explicit thinking_messages wins over quiet",
			cfg: Config{
				Quiet:   &tru,
				Display: DisplayConfig{ThinkingMessages: &tru},
			},
			proj:     ProjectConfig{},
			wantMode: "quiet",
			wantTM:   true,
			wantTool: false,
		},
		{
			name:     "project quiet false overrides global quiet",
			cfg:      Config{Quiet: &tru},
			proj:     ProjectConfig{Quiet: &fal},
			wantMode: "full",
			wantTM:   true,
			wantTool: true,
		},
		{
			name:     "explicit mode compact",
			cfg:      Config{Display: DisplayConfig{Mode: &compact}},
			proj:     ProjectConfig{},
			wantMode: "compact",
			wantTM:   false,
			wantTool: false,
		},
		{
			name:     "project mode overrides global mode",
			cfg:      Config{Display: DisplayConfig{Mode: &quiet}},
			proj:     ProjectConfig{Display: &DisplayConfig{Mode: &compact}},
			wantMode: "compact",
			wantTM:   false,
			wantTool: false,
		},
		{
			name:     "explicit mode wins over legacy quiet",
			cfg:      Config{Quiet: &tru, Display: DisplayConfig{Mode: &compact}},
			proj:     ProjectConfig{},
			wantMode: "compact",
			wantTM:   false,
			wantTool: false,
		},
		{
			name: "explicit mode quiet with thinking override",
			cfg: Config{
				Display: DisplayConfig{Mode: &quiet, ThinkingMessages: &tru},
			},
			proj:     ProjectConfig{},
			wantMode: "quiet",
			wantTM:   true,
			wantTool: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, tm, tool, _, _, _, _, _ := EffectiveDisplay(&tt.cfg, &tt.proj)
			if mode != tt.wantMode {
				t.Fatalf("Mode = %q, want %q", mode, tt.wantMode)
			}
			if tm != tt.wantTM {
				t.Fatalf("ThinkingMessages = %v, want %v", tm, tt.wantTM)
			}
			if tool != tt.wantTool {
				t.Fatalf("ToolMessages = %v, want %v", tool, tt.wantTool)
			}
		})
	}
}

func TestEffectiveDisplay_ProjectOverride(t *testing.T) {
	tru, fal := true, false
	maxA, maxB := 100, 200

	tests := []struct {
		name           string
		cfg            Config
		proj           ProjectConfig
		wantTM         bool
		wantTool       bool
		wantThinkLen   int
		wantToolMaxLen int
	}{
		{
			name: "project overrides global thinking_messages",
			cfg: Config{
				Display: DisplayConfig{ThinkingMessages: &tru, ToolMessages: &tru},
			},
			proj: ProjectConfig{
				Display: &DisplayConfig{ThinkingMessages: &fal},
			},
			wantTM:         false,
			wantTool:       true,
			wantThinkLen:   300,
			wantToolMaxLen: 500,
		},
		{
			name: "project unset falls back to global",
			cfg: Config{
				Display: DisplayConfig{ThinkingMessages: &fal, ToolMessages: &fal},
			},
			proj: ProjectConfig{
				Display: &DisplayConfig{},
			},
			wantTM:         false,
			wantTool:       false,
			wantThinkLen:   300,
			wantToolMaxLen: 500,
		},
		{
			name: "both unset falls back to default",
			cfg:  Config{},
			proj: ProjectConfig{
				Display: &DisplayConfig{},
			},
			wantTM:         true,
			wantTool:       true,
			wantThinkLen:   300,
			wantToolMaxLen: 500,
		},
		{
			name: "project overrides max-len fields",
			cfg: Config{
				Display: DisplayConfig{ThinkingMaxLen: &maxA, ToolMaxLen: &maxA},
			},
			proj: ProjectConfig{
				Display: &DisplayConfig{ThinkingMaxLen: &maxB, ToolMaxLen: &maxB},
			},
			wantTM:         true,
			wantTool:       true,
			wantThinkLen:   200,
			wantToolMaxLen: 200,
		},
		{
			name: "project quiet still respected when project display unset",
			cfg:  Config{Quiet: &tru},
			proj: ProjectConfig{
				Display: &DisplayConfig{},
			},
			wantTM:         false,
			wantTool:       false,
			wantThinkLen:   300,
			wantToolMaxLen: 500,
		},
		{
			name: "project display.thinking_messages true overrides project quiet",
			cfg:  Config{Quiet: &tru},
			proj: ProjectConfig{
				Display: &DisplayConfig{ThinkingMessages: &tru},
			},
			wantTM:         true,
			wantTool:       false,
			wantThinkLen:   300,
			wantToolMaxLen: 500,
		},
		{
			name: "nil project display behaves like before",
			cfg: Config{
				Display: DisplayConfig{ThinkingMessages: &fal},
			},
			proj:           ProjectConfig{},
			wantTM:         false,
			wantTool:       true,
			wantThinkLen:   300,
			wantToolMaxLen: 500,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, tm, tool, thinkLen, toolMaxLen, _, _, _ := EffectiveDisplay(&tt.cfg, &tt.proj)
			if tm != tt.wantTM {
				t.Errorf("ThinkingMessages = %v, want %v", tm, tt.wantTM)
			}
			if tool != tt.wantTool {
				t.Errorf("ToolMessages = %v, want %v", tool, tt.wantTool)
			}
			if thinkLen != tt.wantThinkLen {
				t.Errorf("ThinkingMaxLen = %d, want %d", thinkLen, tt.wantThinkLen)
			}
			if toolMaxLen != tt.wantToolMaxLen {
				t.Errorf("ToolMaxLen = %d, want %d", toolMaxLen, tt.wantToolMaxLen)
			}
		})
	}
}

func TestEffectiveHistoryMaxLen(t *testing.T) {
	globalLen, projectLen, unlimited := 800, 1200, 0

	tests := []struct {
		name string
		cfg  Config
		proj ProjectConfig
		want int
	}{
		{
			name: "default",
			cfg:  Config{},
			proj: ProjectConfig{},
			want: 1000,
		},
		{
			name: "global display",
			cfg: Config{
				Display: DisplayConfig{HistoryMaxLen: &globalLen},
			},
			proj: ProjectConfig{},
			want: 800,
		},
		{
			name: "project display overrides global",
			cfg: Config{
				Display: DisplayConfig{HistoryMaxLen: &globalLen},
			},
			proj: ProjectConfig{
				Display: &DisplayConfig{HistoryMaxLen: &projectLen},
			},
			want: 1200,
		},
		{
			name: "zero disables truncation",
			cfg: Config{
				Display: DisplayConfig{HistoryMaxLen: &unlimited},
			},
			proj: ProjectConfig{},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EffectiveHistoryMaxLen(&tt.cfg, &tt.proj); got != tt.want {
				t.Fatalf("EffectiveHistoryMaxLen() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestEffectiveDisplayHideAgentFooter(t *testing.T) {
	tru := true
	fal := false

	tests := []struct {
		name string
		cfg  Config
		proj ProjectConfig
		want bool
	}{
		{
			name: "default false",
			cfg:  Config{},
			proj: ProjectConfig{},
			want: false,
		},
		{
			name: "global true",
			cfg:  Config{Display: DisplayConfig{HideAgentFooter: &tru}},
			proj: ProjectConfig{},
			want: true,
		},
		{
			name: "project overrides global",
			cfg:  Config{Display: DisplayConfig{HideAgentFooter: &tru}},
			proj: ProjectConfig{Display: &DisplayConfig{HideAgentFooter: &fal}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, _, _, _, _, got := EffectiveDisplay(&tt.cfg, &tt.proj)
			if got != tt.want {
				t.Fatalf("hideAgentFooter = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateProjectDisplayConfig(t *testing.T) {
	mode := "verbose"
	cardMode := "modern"
	negativeHistoryMaxLen := -1

	tests := []struct {
		name    string
		display *DisplayConfig
		wantErr string
	}{
		{
			name:    "invalid project display mode",
			display: &DisplayConfig{Mode: &mode},
			wantErr: `projects[0].display.mode must be "full", "compact", or "quiet"`,
		},
		{
			name:    "invalid project card mode",
			display: &DisplayConfig{CardMode: &cardMode},
			wantErr: `projects[0].display.card_mode must be "legacy" or "rich"`,
		},
		{
			name:    "invalid project history max len",
			display: &DisplayConfig{HistoryMaxLen: &negativeHistoryMaxLen},
			wantErr: `projects[0].display.history_max_len must be >= 0`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{Projects: []ProjectConfig{validProject("demo")}}
			cfg.Projects[0].Display = tt.display
			err := cfg.validate()
			if err == nil {
				t.Fatalf("validate() = nil, want %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("validate() = %q, want contains %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestLoad_DefaultsDataDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfgPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte(baseConfigTOML), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	want := filepath.Join(dir, ".cc-connect")
	if cfg.DataDir != want {
		t.Fatalf("Load() data_dir = %q, want %q", cfg.DataDir, want)
	}
}

func TestLoad_ResolvesEnvPlaceholders(t *testing.T) {

	root := t.TempDir()
	t.Setenv("CC_ROOT", root)
	t.Setenv("TG_TOKEN", "tg-secret")
	t.Setenv("HOOK_TOKEN", "hook-secret")
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:7890")

	configPath := writeConfigFixture(t, `
 data_dir = "${CC_ROOT}/state"

 [webhook]
 token = "${HOOK_TOKEN}"

 [[projects]]
 name = "demo"

 [projects.agent]
 type = "codex"

 [projects.agent.options]
 work_dir = "${CC_ROOT}/repo"
 note = "prefix-${HOOK_TOKEN}-suffix"
 retries = 3

 [[projects.agent.providers]]
 name = "relay"
 api_key = "${OPENAI_API_KEY}"
 base_url = "https://relay.example/${HOOK_TOKEN}"

 [projects.agent.providers.env]
 HTTP_PROXY = "${HTTP_PROXY}"

 [[projects.platforms]]
 type = "telegram"

 [projects.platforms.options]
 token = "${TG_TOKEN}"
 chat_id = 12345
 `)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if got, want := cfg.DataDir, filepath.Join(root, "state"); got != want {
		t.Fatalf("DataDir = %q, want %q", got, want)
	}
	if got := cfg.Webhook.Token; got != "hook-secret" {
		t.Fatalf("Webhook.Token = %q, want hook-secret", got)
	}
	if got := stringMapValue(cfg.Projects[0].Agent.Options, "work_dir"); got != filepath.Join(root, "repo") {
		t.Fatalf("work_dir = %q, want %q", got, filepath.Join(root, "repo"))
	}
	if got := stringMapValue(cfg.Projects[0].Agent.Options, "note"); got != "prefix-hook-secret-suffix" {
		t.Fatalf("note = %q, want prefix-hook-secret-suffix", got)
	}
	if got := cfg.Projects[0].Agent.Providers[0].APIKey; got != "sk-test" {
		t.Fatalf("provider api_key = %q, want sk-test", got)
	}
	if got := cfg.Projects[0].Agent.Providers[0].Env["HTTP_PROXY"]; got != "http://127.0.0.1:7890" {
		t.Fatalf("provider env HTTP_PROXY = %q, want http://127.0.0.1:7890", got)
	}
	if got := stringMapValue(cfg.Projects[0].Platforms[0].Options, "token"); got != "tg-secret" {
		t.Fatalf("platform token = %q, want tg-secret", got)
	}
	if _, ok := cfg.Projects[0].Platforms[0].Options["chat_id"].(int64); !ok {
		t.Fatalf("chat_id type = %T, want int64", cfg.Projects[0].Platforms[0].Options["chat_id"])
	}
}

func TestLoad_MissingEnvPlaceholderBecomesEmptyString(t *testing.T) {

	configPath := writeConfigFixture(t, `
 [[projects]]
 name = "demo"

 [projects.agent]
 type = "codex"

 [projects.agent.options]
 work_dir = "/tmp/demo"
 retries = 5

 [[projects.agent.providers]]
 name = "relay"
 api_key = "${MISSING_API_KEY}"

 [projects.agent.providers.env]
 HTTPS_PROXY = "${MISSING_PROXY}"

 [[projects.platforms]]
 type = "telegram"

 [projects.platforms.options]
 token = "prefix-${MISSING_TOKEN}-suffix"
 `)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if got := cfg.Projects[0].Agent.Providers[0].APIKey; got != "" {
		t.Fatalf("provider api_key = %q, want empty", got)
	}
	if got := cfg.Projects[0].Agent.Providers[0].Env["HTTPS_PROXY"]; got != "" {
		t.Fatalf("provider env HTTPS_PROXY = %q, want empty", got)
	}
	if got := stringMapValue(cfg.Projects[0].Platforms[0].Options, "token"); got != "prefix--suffix" {
		t.Fatalf("platform token = %q, want prefix--suffix", got)
	}
	if _, ok := cfg.Projects[0].Agent.Options["retries"].(int64); !ok {
		t.Fatalf("retries type = %T, want int64", cfg.Projects[0].Agent.Options["retries"])
	}
}

func TestListProjects(t *testing.T) {
	writeTestConfig(t, baseConfigTOML)

	names, err := ListProjects()
	if err != nil {
		t.Fatalf("ListProjects() error: %v", err)
	}
	if len(names) != 1 || names[0] != "demo" {
		t.Fatalf("ListProjects() = %#v, want [demo]", names)
	}
}

func TestSaveLanguage(t *testing.T) {
	writeTestConfig(t, baseConfigTOML)

	if err := SaveLanguage("zh"); err != nil {
		t.Fatalf("SaveLanguage() error: %v", err)
	}

	cfg := readTestConfig(t)
	if cfg.Language != "zh" {
		t.Fatalf("Language = %q, want zh", cfg.Language)
	}
}

func TestProviderConfig_SaveActiveProviderAndGetProjectProviders(t *testing.T) {
	writeTestConfig(t, providerConfigTOML)

	if err := SaveActiveProvider("demo", "backup"); err != nil {
		t.Fatalf("SaveActiveProvider() error: %v", err)
	}

	providers, active, err := GetProjectProviders("demo")
	if err != nil {
		t.Fatalf("GetProjectProviders() error: %v", err)
	}
	if active != "backup" {
		t.Fatalf("active provider = %q, want backup", active)
	}
	if len(providers) != 2 {
		t.Fatalf("provider count = %d, want 2", len(providers))
	}
}

func TestProviderConfig_AddAndRemove(t *testing.T) {
	writeTestConfig(t, providerConfigTOML)

	newProvider := ProviderConfig{Name: "relay", APIKey: "sk-relay", BaseURL: "https://example.com"}
	if err := AddProviderToConfig("demo", newProvider); err != nil {
		t.Fatalf("AddProviderToConfig() error: %v", err)
	}
	if err := AddProviderToConfig("demo", newProvider); err == nil {
		t.Fatal("AddProviderToConfig() duplicate provider: expected error")
	}

	cfg := readTestConfig(t)
	if len(cfg.Projects[0].Agent.Providers) != 3 {
		t.Fatalf("provider count after add = %d, want 3", len(cfg.Projects[0].Agent.Providers))
	}

	if err := RemoveProviderFromConfig("demo", "relay"); err != nil {
		t.Fatalf("RemoveProviderFromConfig() error: %v", err)
	}
	if err := RemoveProviderFromConfig("demo", "relay"); err == nil {
		t.Fatal("RemoveProviderFromConfig() missing provider: expected error")
	}
}

func TestProviderConfig_SaveProviderModel(t *testing.T) {
	writeTestConfig(t, providerConfigTOML)

	if err := SaveProviderModel("demo", "primary", "gpt-5.4"); err != nil {
		t.Fatalf("SaveProviderModel() error: %v", err)
	}

	cfg := readTestConfig(t)
	if got := cfg.Projects[0].Agent.Providers[0].Model; got != "gpt-5.4" {
		t.Fatalf("provider model = %q, want gpt-5.4", got)
	}
	if err := SaveProviderModel("demo", "missing", "gpt-4.1"); err == nil {
		t.Fatal("SaveProviderModel() missing provider: expected error")
	}
}

func TestSaveAgentModel(t *testing.T) {
	writeTestConfig(t, providerConfigTOML)

	if err := SaveAgentModel("demo", "gpt-5.4"); err != nil {
		t.Fatalf("SaveAgentModel() error: %v", err)
	}

	cfg := readTestConfig(t)
	if got, _ := cfg.Projects[0].Agent.Options["model"].(string); got != "gpt-5.4" {
		t.Fatalf("agent.options.model = %q, want gpt-5.4", got)
	}
	if got, _ := cfg.Projects[0].Agent.Options["mode"].(string); got != "default" {
		t.Fatalf("agent.options.mode = %q, want default", got)
	}
	if got, _ := cfg.Projects[0].Agent.Options["provider"].(string); got != "primary" {
		t.Fatalf("agent.options.provider = %q, want primary", got)
	}
	if len(cfg.Projects[0].Agent.Providers) != 2 {
		t.Fatalf("provider count = %d, want 2", len(cfg.Projects[0].Agent.Providers))
	}
}

const providerConfigWithCommentsTOML = `# This is my config file
# Very important - do not lose this!
custom_top = "keep_me"

[[projects]]
name = "demo"
work_dir = "/tmp/demo" # inline comment

[projects.agent]
type = "claudecode"

[projects.agent.options]
mode = "default"
provider = "primary"
custom_option = "still_here" # keep inline comment

[[projects.agent.providers]]
name = "primary"
api_key = "sk-primary"

[[projects.agent.providers]]
name = "backup"
api_key = "sk-backup"

[[projects.platforms]]
type = "telegram"

[projects.platforms.options]
token = "test-token"
`

func TestSaveActiveProvider_PreservesCommentsAndUnknownFields(t *testing.T) {
	writeTestConfig(t, providerConfigWithCommentsTOML)

	if err := SaveActiveProvider("demo", "backup"); err != nil {
		t.Fatalf("SaveActiveProvider() error: %v", err)
	}

	content, err := os.ReadFile(ConfigPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(content)

	if !strings.Contains(text, "# This is my config file") {
		t.Fatalf("expected top comment to be preserved, got:\n%s", text)
	}
	if !strings.Contains(text, "# Very important - do not lose this!") {
		t.Fatalf("expected second comment to be preserved, got:\n%s", text)
	}
	if !strings.Contains(text, `custom_top = "keep_me"`) {
		t.Fatalf("expected unknown top-level field to be preserved, got:\n%s", text)
	}
	if !strings.Contains(text, `custom_option = "still_here"`) {
		t.Fatalf("expected unknown options field to be preserved, got:\n%s", text)
	}
	if !strings.Contains(text, "keep inline comment") {
		t.Fatalf("expected inline comment to be preserved, got:\n%s", text)
	}
	if !strings.Contains(text, `mode = "default"`) {
		t.Fatalf("expected mode to be preserved, got:\n%s", text)
	}
	if !strings.Contains(text, `provider = "backup"`) {
		t.Fatalf("expected provider to be updated to backup, got:\n%s", text)
	}
	if !strings.Contains(text, `work_dir = "/tmp/demo"`) {
		t.Fatalf("expected work_dir to be preserved, got:\n%s", text)
	}

	cfg := readTestConfig(t)
	active, _ := cfg.Projects[0].Agent.Options["provider"].(string)
	if active != "backup" {
		t.Fatalf("active provider = %q, want backup", active)
	}
}

func TestSaveAgentModel_PreservesCommentsAndUnknownFields(t *testing.T) {
	writeTestConfig(t, providerConfigWithCommentsTOML)

	if err := SaveAgentModel("demo", "gpt-5.4"); err != nil {
		t.Fatalf("SaveAgentModel() error: %v", err)
	}

	content, err := os.ReadFile(ConfigPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(content)

	if !strings.Contains(text, "# This is my config file") {
		t.Fatalf("expected top comment to be preserved, got:\n%s", text)
	}
	if !strings.Contains(text, `custom_option = "still_here"`) {
		t.Fatalf("expected unknown options field to be preserved, got:\n%s", text)
	}
	if !strings.Contains(text, `provider = "primary"`) {
		t.Fatalf("expected provider to be preserved, got:\n%s", text)
	}
	if !strings.Contains(text, `model = "gpt-5.4"`) {
		t.Fatalf("expected model to be set, got:\n%s", text)
	}
}

func TestSaveProviderModel_PreservesCommentsAndUnknownFields(t *testing.T) {
	writeTestConfig(t, providerConfigWithCommentsTOML)

	if err := SaveProviderModel("demo", "primary", "gpt-5.4"); err != nil {
		t.Fatalf("SaveProviderModel() error: %v", err)
	}

	content, err := os.ReadFile(ConfigPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(content)

	if !strings.Contains(text, "# This is my config file") {
		t.Fatalf("expected top comment to be preserved, got:\n%s", text)
	}
	if !strings.Contains(text, `custom_option = "still_here"`) {
		t.Fatalf("expected unknown options field to be preserved, got:\n%s", text)
	}
	if !strings.Contains(text, `model = "gpt-5.4"`) {
		t.Fatalf("expected model to be set in provider, got:\n%s", text)
	}
}

func TestSaveLanguage_PreservesComments(t *testing.T) {
	writeTestConfig(t, providerConfigWithCommentsTOML)

	if err := SaveLanguage("zh"); err != nil {
		t.Fatalf("SaveLanguage() error: %v", err)
	}

	content, err := os.ReadFile(ConfigPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(content)

	if !strings.Contains(text, "# This is my config file") {
		t.Fatalf("expected top comment to be preserved, got:\n%s", text)
	}
	if !strings.Contains(text, `custom_option = "still_here"`) {
		t.Fatalf("expected unknown options field to be preserved, got:\n%s", text)
	}
	if !strings.Contains(text, `language = "zh"`) {
		t.Fatalf("expected language to be set, got:\n%s", text)
	}

	cfg := readTestConfig(t)
	if cfg.Language != "zh" {
		t.Fatalf("Language = %q, want zh", cfg.Language)
	}
}

func TestSaveDisplayConfig_PreservesComments(t *testing.T) {
	configWithDisplay := providerConfigWithCommentsTOML + `
[display]
# display settings below
thinking_messages = true
custom_display = "keep" # also keep
`
	writeTestConfig(t, configWithDisplay)

	thinking := 200
	toolShow := false
	if err := SaveDisplayConfig(nil, nil, &thinking, nil, &toolShow); err != nil {
		t.Fatalf("SaveDisplayConfig() error: %v", err)
	}

	content, err := os.ReadFile(ConfigPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(content)

	if !strings.Contains(text, "# This is my config file") {
		t.Fatalf("expected top comment to be preserved, got:\n%s", text)
	}
	if !strings.Contains(text, "# display settings below") {
		t.Fatalf("expected display comment to be preserved, got:\n%s", text)
	}
	if !strings.Contains(text, `custom_display = "keep"`) {
		t.Fatalf("expected unknown display field to be preserved, got:\n%s", text)
	}
	if !strings.Contains(text, `thinking_max_len = 200`) {
		t.Fatalf("expected thinking_max_len to be set, got:\n%s", text)
	}
	if !strings.Contains(text, `tool_messages = false`) {
		t.Fatalf("expected tool_messages to be set, got:\n%s", text)
	}
}

func TestSaveTTSMode_PreservesComments(t *testing.T) {
	configWithTTS := providerConfigWithCommentsTOML + `
[tts]
# tts config
tts_mode = "auto"
`
	writeTestConfig(t, configWithTTS)

	if err := SaveTTSMode("always"); err != nil {
		t.Fatalf("SaveTTSMode() error: %v", err)
	}

	content, err := os.ReadFile(ConfigPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(content)

	if !strings.Contains(text, "# This is my config file") {
		t.Fatalf("expected top comment to be preserved, got:\n%s", text)
	}
	if !strings.Contains(text, "# tts config") {
		t.Fatalf("expected tts comment to be preserved, got:\n%s", text)
	}
	if !strings.Contains(text, `tts_mode = "always"`) {
		t.Fatalf("expected tts_mode to be updated, got:\n%s", text)
	}
}

func TestResolveTTSConfigForProject_AgentOverrides(t *testing.T) {
	raw := `
[tts]
enabled = true
provider = "minimax"
voice = "global-voice"
voice_id = "global-id"
speed = 1.1
language_type = "Chinese"
tts_mode = "voice_only"
max_text_len = 200

[tts.agents.assistant]
voice_id = "Chinese (Mandarin)_Crisp_Girl"
speed = 0.98
max_text_len = 120

[tts.agents.reviewer]
voice = "Chinese (Mandarin)_Gentle_Senior"
`
	var cfg Config
	if _, err := toml.Decode(raw, &cfg); err != nil {
		t.Fatalf("decode config: %v", err)
	}

	assistant := ResolveTTSConfigForProject(cfg.TTS, "assistant")
	if !assistant.Enabled {
		t.Fatal("expected assistant TTS enabled")
	}
	if assistant.Provider != "minimax" {
		t.Fatalf("provider = %q, want minimax", assistant.Provider)
	}
	if assistant.Voice != "Chinese (Mandarin)_Crisp_Girl" {
		t.Fatalf("voice = %q", assistant.Voice)
	}
	if assistant.Speed != 0.98 {
		t.Fatalf("speed = %v, want 0.98", assistant.Speed)
	}
	if assistant.LanguageType != "Chinese" {
		t.Fatalf("language_type = %q, want Chinese", assistant.LanguageType)
	}
	if assistant.MaxTextLen != 120 {
		t.Fatalf("max_text_len = %d, want 120", assistant.MaxTextLen)
	}

	reviewer := ResolveTTSConfigForProject(cfg.TTS, "reviewer")
	if reviewer.Voice != "Chinese (Mandarin)_Gentle_Senior" {
		t.Fatalf("reviewer voice = %q", reviewer.Voice)
	}
	if reviewer.Speed != 1.1 {
		t.Fatalf("reviewer speed = %v, want inherited 1.1", reviewer.Speed)
	}

	unknown := ResolveTTSConfigForProject(cfg.TTS, "unknown")
	if unknown.Voice != "global-id" {
		t.Fatalf("unknown voice = %q, want global voice_id", unknown.Voice)
	}
}

func TestLoadMiniMaxLocalConfig_DefaultDataDir(t *testing.T) {
	dataDir := t.TempDir()
	cfgDir := filepath.Join(dataDir, "config")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "minimax.json"), []byte(`{
  "api_key": "sk-test",
  "api_host": "https://api.minimaxi.com"
}`), 0o600); err != nil {
		t.Fatalf("write minimax config: %v", err)
	}

	cfg, err := LoadMiniMaxLocalConfig(dataDir, "")
	if err != nil {
		t.Fatalf("LoadMiniMaxLocalConfig() error: %v", err)
	}
	if cfg.APIKey != "sk-test" {
		t.Fatalf("api key not loaded")
	}
	if cfg.APIHost != "https://api.minimaxi.com" {
		t.Fatalf("api_host = %q", cfg.APIHost)
	}
}

func TestLoadMiniMaxLocalConfig_MissingFileReturnsEmpty(t *testing.T) {
	cfg, err := LoadMiniMaxLocalConfig(t.TempDir(), "")
	if err != nil {
		t.Fatalf("LoadMiniMaxLocalConfig() error: %v", err)
	}
	if cfg != (MiniMaxLocalConfig{}) {
		t.Fatalf("config = %#v, want empty", cfg)
	}
}

const multiProjectConfigTOML = `# multi-project config
[[projects]]
name = "alpha"
work_dir = "/tmp/alpha"

[projects.agent]
type = "codex"

[projects.agent.options]
provider = "openai"

[[projects.platforms]]
type = "telegram"

[projects.platforms.options]
token = "alpha-token"

[[projects]]
name = "beta"
work_dir = "/tmp/beta"

[projects.agent]
type = "claudecode"

[projects.agent.options]
provider = "anthropic"

[[projects.platforms]]
type = "feishu"

[projects.platforms.options]
app_id = "beta-app"
`

func TestSaveActiveProvider_MultiProject(t *testing.T) {
	writeTestConfig(t, multiProjectConfigTOML)

	if err := SaveActiveProvider("beta", "openai"); err != nil {
		t.Fatalf("SaveActiveProvider() error: %v", err)
	}

	content, err := os.ReadFile(ConfigPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(content)

	if !strings.Contains(text, "# multi-project config") {
		t.Fatalf("expected top comment preserved, got:\n%s", text)
	}

	cfg := readTestConfig(t)
	alphaProvider, _ := cfg.Projects[0].Agent.Options["provider"].(string)
	betaProvider, _ := cfg.Projects[1].Agent.Options["provider"].(string)
	if alphaProvider != "openai" {
		t.Fatalf("alpha provider = %q, want openai (untouched)", alphaProvider)
	}
	if betaProvider != "openai" {
		t.Fatalf("beta provider = %q, want openai (updated)", betaProvider)
	}
}

const globalProviderRefConfigTOML = `# global provider refs
[[providers]]
name = "shared-openai"
api_key = "sk-shared"
model = "gpt-4o"

[[projects]]
name = "demo"
work_dir = "/tmp/demo"

[projects.agent]
type = "codex"
provider_refs = ["shared-openai"]

[[projects.platforms]]
type = "telegram"

[projects.platforms.options]
token = "demo-token"
`

func TestSaveProviderModel_GlobalProviderRef(t *testing.T) {
	writeTestConfig(t, globalProviderRefConfigTOML)

	if err := SaveProviderModel("demo", "shared-openai", "gpt-5"); err != nil {
		t.Fatalf("SaveProviderModel() error: %v", err)
	}

	content, err := os.ReadFile(ConfigPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(content)

	if !strings.Contains(text, "# global provider refs") {
		t.Fatalf("expected comment preserved, got:\n%s", text)
	}
	if !strings.Contains(text, `model = "gpt-5"`) {
		t.Fatalf("expected model updated in global provider, got:\n%s", text)
	}
	if !strings.Contains(text, `api_key = "sk-shared"`) {
		t.Fatalf("expected api_key preserved, got:\n%s", text)
	}

	cfg := readTestConfig(t)
	if cfg.Providers[0].Model != "gpt-5" {
		t.Fatalf("global provider model = %q, want gpt-5", cfg.Providers[0].Model)
	}
}

func TestCommandConfig_AddAndRemove(t *testing.T) {
	writeTestConfig(t, baseConfigTOML)

	cmd := CommandConfig{Name: "review", Description: "code review", Prompt: "review {{args}}"}
	if err := AddCommand(cmd); err != nil {
		t.Fatalf("AddCommand() error: %v", err)
	}
	if err := AddCommand(cmd); err == nil {
		t.Fatal("AddCommand() duplicate command: expected error")
	}

	cfg := readTestConfig(t)
	if len(cfg.Commands) != 1 || cfg.Commands[0].Name != "review" {
		t.Fatalf("commands after add = %#v, want one review command", cfg.Commands)
	}

	if err := RemoveCommand("review"); err != nil {
		t.Fatalf("RemoveCommand() error: %v", err)
	}
	if err := RemoveCommand("review"); err == nil {
		t.Fatal("RemoveCommand() missing command: expected error")
	}
}

func TestAliasConfig_AddAndRemove(t *testing.T) {
	writeTestConfig(t, baseConfigTOML)

	if err := AddAlias(AliasConfig{Name: "帮助", Command: "/help"}); err != nil {
		t.Fatalf("AddAlias() error: %v", err)
	}
	if err := AddAlias(AliasConfig{Name: "帮助", Command: "/list"}); err != nil {
		t.Fatalf("AddAlias() update error: %v", err)
	}

	cfg := readTestConfig(t)
	if len(cfg.Aliases) != 1 || cfg.Aliases[0].Command != "/list" {
		t.Fatalf("aliases after update = %#v, want one updated alias", cfg.Aliases)
	}

	if err := RemoveAlias("帮助"); err != nil {
		t.Fatalf("RemoveAlias() error: %v", err)
	}
	if err := RemoveAlias("帮助"); err == nil {
		t.Fatal("RemoveAlias() missing alias: expected error")
	}
}

func TestDisplayConfig_Save(t *testing.T) {
	writeTestConfig(t, baseConfigTOML)

	thinking := 120
	tool := 240
	showTools := false
	if err := SaveDisplayConfig(nil, nil, &thinking, &tool, &showTools); err != nil {
		t.Fatalf("SaveDisplayConfig() error: %v", err)
	}

	cfg := readTestConfig(t)
	if cfg.Display.ThinkingMaxLen == nil || *cfg.Display.ThinkingMaxLen != 120 {
		t.Fatalf("ThinkingMaxLen = %#v, want 120", cfg.Display.ThinkingMaxLen)
	}
	if cfg.Display.ToolMaxLen == nil || *cfg.Display.ToolMaxLen != 240 {
		t.Fatalf("ToolMaxLen = %#v, want 240", cfg.Display.ToolMaxLen)
	}
	if cfg.Display.ToolMessages == nil || *cfg.Display.ToolMessages {
		t.Fatalf("ToolMessages = %#v, want false", cfg.Display.ToolMessages)
	}

	thinking = 360
	if err := SaveDisplayConfig(nil, nil, &thinking, nil, nil); err != nil {
		t.Fatalf("SaveDisplayConfig() second update error: %v", err)
	}

	cfg = readTestConfig(t)
	if cfg.Display.ThinkingMaxLen == nil || *cfg.Display.ThinkingMaxLen != 360 {
		t.Fatalf("ThinkingMaxLen after update = %#v, want 360", cfg.Display.ThinkingMaxLen)
	}
	if cfg.Display.ToolMaxLen == nil || *cfg.Display.ToolMaxLen != 240 {
		t.Fatalf("ToolMaxLen after nil update = %#v, want 240", cfg.Display.ToolMaxLen)
	}
	if cfg.Display.ToolMessages == nil || *cfg.Display.ToolMessages {
		t.Fatalf("ToolMessages after nil update = %#v, want false", cfg.Display.ToolMessages)
	}
}

func TestTTSConfig_SaveMode(t *testing.T) {
	writeTestConfig(t, baseConfigTOML)

	if err := SaveTTSMode("always"); err != nil {
		t.Fatalf("SaveTTSMode() error: %v", err)
	}

	cfg := readTestConfig(t)
	if cfg.TTS.TTSMode != "always" {
		t.Fatalf("TTSMode = %q, want always", cfg.TTS.TTSMode)
	}
}

const attachmentSendConfigFixture = `
attachment_send = "off"

[[projects]]
name = "alpha"

[projects.agent]
type = "codex"

[projects.agent.options]
work_dir = "/tmp/alpha"

[[projects.platforms]]
type = "telegram"

[projects.platforms.options]
bot_token = "token_xxx"
`

const relayConfigFixture = `
[relay]
timeout_secs = 300
visibility = "none"

[[projects]]
name = "alpha"

[projects.agent]
type = "codex"

[projects.agent.options]
work_dir = "/tmp/alpha"

[[projects.platforms]]
type = "telegram"

[projects.platforms.options]
bot_token = "token_xxx"
`

const relayConfigNegativeFixture = `
[relay]
timeout_secs = -1

[[projects]]
name = "alpha"

[projects.agent]
type = "codex"

[projects.agent.options]
work_dir = "/tmp/alpha"

[[projects.platforms]]
type = "telegram"

[projects.platforms.options]
bot_token = "token_xxx"
`

const relayConfigInvalidVisibilityFixture = `
[relay]
visibility = "verbose"

[[projects]]
name = "alpha"

[projects.agent]
type = "codex"

[projects.agent.options]
work_dir = "/tmp/alpha"

[[projects.platforms]]
type = "telegram"

[projects.platforms.options]
bot_token = "token_xxx"
`

func TestLoad_DefaultsAttachmentSendToOn(t *testing.T) {
	configPath := writeConfigFixture(t, projectWithoutDingtalkFixture)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.AttachmentSend != "on" {
		t.Fatalf("cfg.AttachmentSend = %q, want %q", cfg.AttachmentSend, "on")
	}
}

func TestLoad_DefaultsAutoCompressDisabled(t *testing.T) {
	configPath := writeConfigFixture(t, projectWithoutDingtalkFixture)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(cfg.Projects) == 0 {
		t.Fatalf("expected at least one project")
	}
	if cfg.Projects[0].AutoCompress.Enabled != nil {
		t.Fatalf("expected auto_compress.enabled to default to nil")
	}
}

func TestLoad_ParsesResetOnIdleMins(t *testing.T) {
	configPath := writeConfigFixture(t, projectWithResetOnIdleFixture)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Projects[0].ResetOnIdleMins == nil {
		t.Fatal("expected reset_on_idle_mins to be parsed")
	}
	if got := *cfg.Projects[0].ResetOnIdleMins; got != 60 {
		t.Fatalf("reset_on_idle_mins = %d, want 60", got)
	}
}

func TestLoad_RejectsNegativeResetOnIdleMins(t *testing.T) {
	configPath := writeConfigFixture(t, projectWithNegativeResetOnIdleFixture)

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for negative reset_on_idle_mins")
	}
	if !strings.Contains(err.Error(), "reset_on_idle_mins") {
		t.Fatalf("error = %q, want reset_on_idle_mins validation", err.Error())
	}
}

func TestLoad_ParsesAgentSessionIdleTimeoutMins(t *testing.T) {
	configPath := writeConfigFixture(t, projectWithAgentSessionIdleTimeoutFixture)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Projects[0].AgentSessionIdleTimeoutMins == nil {
		t.Fatal("expected agent_session_idle_timeout_mins to be parsed")
	}
	if got := *cfg.Projects[0].AgentSessionIdleTimeoutMins; got != 45 {
		t.Fatalf("agent_session_idle_timeout_mins = %d, want 45", got)
	}
}

func TestLoad_RejectsNegativeAgentSessionIdleTimeoutMins(t *testing.T) {
	configPath := writeConfigFixture(t, projectWithNegativeAgentSessionIdleTimeoutFixture)

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for negative agent_session_idle_timeout_mins")
	}
	if !strings.Contains(err.Error(), "agent_session_idle_timeout_mins") {
		t.Fatalf("error = %q, want agent_session_idle_timeout_mins validation", err.Error())
	}
}

func TestLoad_ParsesRunAsUser(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("run_as_user is only supported on Linux/macOS")
	}
	configPath := writeConfigFixture(t, projectWithRunAsUserFixture)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got := cfg.Projects[0].RunAsUser; got != "partseeker-coder" {
		t.Fatalf("run_as_user = %q, want %q", got, "partseeker-coder")
	}
	if got := cfg.Projects[0].RunAsEnv; len(got) != 2 || got[0] != "PGSSLROOTCERT" || got[1] != "PGSSLMODE" {
		t.Fatalf("run_as_env = %v, want [PGSSLROOTCERT PGSSLMODE]", got)
	}
}

func TestLoad_RejectsRunAsUserRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("run_as_user is only supported on Linux/macOS")
	}
	configPath := writeConfigFixture(t, projectWithRunAsUserRootFixture)

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for run_as_user = root")
	}
	if !strings.Contains(err.Error(), "must not be root") {
		t.Fatalf("error = %q, want 'must not be root' validation", err.Error())
	}
}

func TestLoad_RejectsRunAsUserInvalidChars(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("run_as_user is only supported on Linux/macOS")
	}
	configPath := writeConfigFixture(t, projectWithRunAsUserInvalidFixture)

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for invalid run_as_user")
	}
	if !strings.Contains(err.Error(), "invalid characters") {
		t.Fatalf("error = %q, want 'invalid characters' validation", err.Error())
	}
}

func TestValidateRunAsUser_ValidNames(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("run_as_user is only supported on Linux/macOS")
	}
	valid := []string{"leigh", "partseeker-coder", "user_name", "user.name", "u1", "_internal"}
	for _, name := range valid {
		if err := validateRunAsUser("projects[0]", name); err != nil {
			t.Errorf("validateRunAsUser(%q) = %v, want nil", name, err)
		}
	}
}

func TestValidateRunAsUser_InvalidNames(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("run_as_user is only supported on Linux/macOS")
	}
	invalid := []string{
		"-leading-dash",
		"1leading-digit",
		"has space",
		"has/slash",
		"has;semi",
		"has$dollar",
		"has`tick",
		strings.Repeat("a", 33), // too long
	}
	for _, name := range invalid {
		if err := validateRunAsUser("projects[0]", name); err == nil {
			t.Errorf("validateRunAsUser(%q) = nil, want error", name)
		}
	}
}

func TestLoad_ParsesAttachmentSendOff(t *testing.T) {
	configPath := writeConfigFixture(t, attachmentSendConfigFixture)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.AttachmentSend != "off" {
		t.Fatalf("cfg.AttachmentSend = %q, want %q", cfg.AttachmentSend, "off")
	}
}

func TestLoad_FilterExternalSessionsDefault(t *testing.T) {
	configPath := writeConfigFixture(t, attachmentSendConfigFixture)
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	proj := cfg.Projects[0]
	if proj.FilterExternalSessions != nil {
		t.Fatalf("FilterExternalSessions should be nil by default, got %v", *proj.FilterExternalSessions)
	}
}

func TestLoad_FilterExternalSessionsTrue(t *testing.T) {
	fixture := `
[[projects]]
name = "beta"
filter_external_sessions = true

[projects.agent]
type = "codex"

[projects.agent.options]
work_dir = "/tmp/beta"

[[projects.platforms]]
type = "telegram"

[projects.platforms.options]
token = "test"
`
	configPath := writeConfigFixture(t, fixture)
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	proj := cfg.Projects[0]
	if proj.FilterExternalSessions == nil || !*proj.FilterExternalSessions {
		t.Fatalf("FilterExternalSessions should be true, got %v", proj.FilterExternalSessions)
	}
}

func TestLoad_FilterExternalSessionsFalse(t *testing.T) {
	fixture := `
[[projects]]
name = "gamma"
filter_external_sessions = false

[projects.agent]
type = "codex"

[projects.agent.options]
work_dir = "/tmp/gamma"

[[projects.platforms]]
type = "telegram"

[projects.platforms.options]
token = "test"
`
	configPath := writeConfigFixture(t, fixture)
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	proj := cfg.Projects[0]
	if proj.FilterExternalSessions == nil || *proj.FilterExternalSessions {
		t.Fatalf("FilterExternalSessions should be false, got %v", proj.FilterExternalSessions)
	}
}

func validProject(name string) ProjectConfig {
	return ProjectConfig{
		Name: name,
		Agent: AgentConfig{
			Type:    "claudecode",
			Options: map[string]any{"mode": "default"},
		},
		Platforms: []PlatformConfig{
			{Type: "telegram", Options: map[string]any{"token": "test-token"}},
		},
	}
}

func assertErrContains(t *testing.T, err error, want string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected error containing %q, got nil", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %q, want substring %q", err.Error(), want)
	}
}

func writeTestConfig(t *testing.T, content string) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	oldPath := ConfigPath
	ConfigPath = path
	t.Cleanup(func() {
		ConfigPath = oldPath
	})
}

func readTestConfig(t *testing.T) Config {
	t.Helper()

	data, err := os.ReadFile(ConfigPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse config: %v", err)
	}
	return cfg
}

func TestLoadRelayTimeoutConfig(t *testing.T) {
	configPath := writeConfigFixture(t, relayConfigFixture)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Relay.TimeoutSecs == nil {
		t.Fatal("cfg.Relay.TimeoutSecs = nil, want non-nil")
	}
	if *cfg.Relay.TimeoutSecs != 300 {
		t.Fatalf("cfg.Relay.TimeoutSecs = %d, want 300", *cfg.Relay.TimeoutSecs)
	}
	if cfg.Relay.Visibility != "none" {
		t.Fatalf("cfg.Relay.Visibility = %q, want none", cfg.Relay.Visibility)
	}
}

func TestLoadRejectsNegativeRelayTimeout(t *testing.T) {
	configPath := writeConfigFixture(t, relayConfigNegativeFixture)

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for negative relay timeout, got nil")
	}
	if !strings.Contains(err.Error(), "relay.timeout_secs must be >= 0") {
		t.Fatalf("error = %q, want contains %q", err.Error(), "relay.timeout_secs must be >= 0")
	}
}

func TestLoadRejectsInvalidRelayVisibility(t *testing.T) {
	configPath := writeConfigFixture(t, relayConfigInvalidVisibilityFixture)

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for invalid relay visibility, got nil")
	}
	if !strings.Contains(err.Error(), `relay.visibility must be "full", "summary", or "none"`) {
		t.Fatalf("error = %q, want relay.visibility validation error", err.Error())
	}
}
func writeConfigFixture(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}
	return path
}

func patchConfigPath(t *testing.T, path string) {
	t.Helper()
	prev := ConfigPath
	ConfigPath = path
	t.Cleanup(func() {
		ConfigPath = prev
	})
}

func readConfigFixture(t *testing.T, path string) *Config {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config fixture: %v", err)
	}
	cfg := &Config{}
	if err := toml.Unmarshal(data, cfg); err != nil {
		t.Fatalf("parse config fixture: %v", err)
	}
	return cfg
}

func stringMapValue(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

const baseConfigTOML = `
[[projects]]
name = "demo"

[projects.agent]
type = "claudecode"

[projects.agent.options]
mode = "default"

[[projects.platforms]]
type = "telegram"

[projects.platforms.options]
token = "test-token"
`

const providerConfigTOML = `
[[projects]]
name = "demo"

[projects.agent]
type = "claudecode"

[projects.agent.options]
mode = "default"
provider = "primary"

[[projects.agent.providers]]
name = "primary"
api_key = "sk-primary"

[[projects.agent.providers]]
name = "backup"
api_key = "sk-backup"

[[projects.platforms]]
type = "telegram"

[projects.platforms.options]
token = "test-token"
`

const multiPlatformConfigFixture = `
[[projects]]
name = "alpha"

[projects.agent]
type = "opencode"

[projects.agent.options]
work_dir = "/tmp/alpha"

[[projects.platforms]]
type = "telegram"

[projects.platforms.options]
bot_token = "token_xxx"

[[projects.platforms]]
type = "dingtalk"

[projects.platforms.options]
client_id = "old_dingtalk_client"
client_secret = "old_dingtalk_secret"

[[projects.platforms]]
type = "dingtalk"

[projects.platforms.options]
client_id = "old_dingtalk_client_2"
client_secret = "old_dingtalk_secret_2"
allow_from = "existing_owner"
`

const projectWithoutDingtalkFixture = `
[[projects]]
name = "beta"

[projects.agent]
type = "opencode"

[projects.agent.options]
work_dir = "/tmp/beta"

[[projects.platforms]]
type = "telegram"

[projects.platforms.options]
bot_token = "token_xxx"
`

const projectWithResetOnIdleFixture = `
[[projects]]
name = "beta"
reset_on_idle_mins = 60

[projects.agent]
type = "codex"

[projects.agent.options]
work_dir = "/tmp/beta"

[[projects.platforms]]
type = "telegram"

[projects.platforms.options]
bot_token = "token_xxx"
`

const projectWithNegativeResetOnIdleFixture = `
[[projects]]
name = "beta"
reset_on_idle_mins = -1

[projects.agent]
type = "codex"

[projects.agent.options]
work_dir = "/tmp/beta"

[[projects.platforms]]
type = "telegram"

[projects.platforms.options]
bot_token = "token_xxx"
`

const projectWithAgentSessionIdleTimeoutFixture = `
[[projects]]
name = "beta"
agent_session_idle_timeout_mins = 45

[projects.agent]
type = "codex"

[projects.agent.options]
work_dir = "/tmp/beta"

[[projects.platforms]]
type = "telegram"

[projects.platforms.options]
bot_token = "token_xxx"
`

const projectWithNegativeAgentSessionIdleTimeoutFixture = `
[[projects]]
name = "beta"
agent_session_idle_timeout_mins = -1

[projects.agent]
type = "codex"

[projects.agent.options]
work_dir = "/tmp/beta"

[[projects.platforms]]
type = "telegram"

[projects.platforms.options]
bot_token = "token_xxx"
`

const projectWithRunAsUserFixture = `
[[projects]]
name = "sandboxed"
run_as_user = "partseeker-coder"
run_as_env = ["PGSSLROOTCERT", "PGSSLMODE"]

[projects.agent]
type = "claudecode"

[projects.agent.options]
work_dir = "/tmp/sandboxed"

[[projects.platforms]]
type = "slack"

[projects.platforms.options]
app_token = "xapp-token"
bot_token = "xoxb-token"
`

const projectWithRunAsUserRootFixture = `
[[projects]]
name = "bad"
run_as_user = "root"

[projects.agent]
type = "claudecode"

[projects.agent.options]
work_dir = "/tmp/bad"

[[projects.platforms]]
type = "slack"

[projects.platforms.options]
app_token = "xapp-token"
bot_token = "xoxb-token"
`

const projectWithRunAsUserInvalidFixture = `
[[projects]]
name = "bad"
run_as_user = "has space"

[projects.agent]
type = "claudecode"

[projects.agent.options]
work_dir = "/tmp/bad"

[[projects.platforms]]
type = "slack"

[projects.platforms.options]
app_token = "xapp-token"
bot_token = "xoxb-token"
`

const preserveFormatFixture = `# top comment should stay
custom_top = "keep_me"

[[projects]]
name = "alpha"

[projects.agent]
type = "opencode"

[projects.agent.options]
work_dir = "/tmp/alpha"

[[projects.platforms]]
type = "dingtalk"

[projects.platforms.options]
client_id = "old_client" # keep inline comment
client_secret = "old_secret"
custom_option = "still_here"
`

// --- validateUsersConfig tests ---

func TestValidateUsersConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name: "nil users is valid",
			cfg: Config{
				Projects: []ProjectConfig{{
					Name:      "p1",
					Agent:     AgentConfig{Type: "codex"},
					Platforms: []PlatformConfig{{Type: "dingtalk", Options: map[string]any{"token": "x"}}},
					Users:     nil,
				}},
			},
			wantErr: "",
		},
		{
			name: "empty roles",
			cfg: Config{
				Projects: []ProjectConfig{{
					Name:      "p1",
					Agent:     AgentConfig{Type: "codex"},
					Platforms: []PlatformConfig{{Type: "dingtalk", Options: map[string]any{"token": "x"}}},
					Users:     &UsersConfig{Roles: map[string]RoleConfig{}},
				}},
			},
			wantErr: `no roles defined`,
		},
		{
			name: "empty user_ids in role",
			cfg: Config{
				Projects: []ProjectConfig{{
					Name:      "p1",
					Agent:     AgentConfig{Type: "codex"},
					Platforms: []PlatformConfig{{Type: "dingtalk", Options: map[string]any{"token": "x"}}},
					Users: &UsersConfig{
						Roles: map[string]RoleConfig{
							"admin": {UserIDs: []string{}},
						},
					},
				}},
			},
			wantErr: `empty user_ids`,
		},
		{
			name: "duplicate user in different roles",
			cfg: Config{
				Projects: []ProjectConfig{{
					Name:      "p1",
					Agent:     AgentConfig{Type: "codex"},
					Platforms: []PlatformConfig{{Type: "dingtalk", Options: map[string]any{"token": "x"}}},
					Users: &UsersConfig{
						Roles: map[string]RoleConfig{
							"admin":  {UserIDs: []string{"user1"}},
							"member": {UserIDs: []string{"user1"}},
						},
					},
				}},
			},
			wantErr: `appears in both role`,
		},
		{
			name: "wildcard in multiple roles",
			cfg: Config{
				Projects: []ProjectConfig{{
					Name:      "p1",
					Agent:     AgentConfig{Type: "codex"},
					Platforms: []PlatformConfig{{Type: "dingtalk", Options: map[string]any{"token": "x"}}},
					Users: &UsersConfig{
						Roles: map[string]RoleConfig{
							"admin":  {UserIDs: []string{"*"}},
							"member": {UserIDs: []string{"*"}},
						},
					},
				}},
			},
			wantErr: `wildcard`,
		},
		{
			name: "default_role not matching any role",
			cfg: Config{
				Projects: []ProjectConfig{{
					Name:      "p1",
					Agent:     AgentConfig{Type: "codex"},
					Platforms: []PlatformConfig{{Type: "dingtalk", Options: map[string]any{"token": "x"}}},
					Users: &UsersConfig{
						DefaultRole: "superadmin",
						Roles: map[string]RoleConfig{
							"admin": {UserIDs: []string{"u1"}},
						},
					},
				}},
			},
			wantErr: `default_role`,
		},
		{
			name: "valid users config",
			cfg: Config{
				Projects: []ProjectConfig{{
					Name:      "p1",
					Agent:     AgentConfig{Type: "codex"},
					Platforms: []PlatformConfig{{Type: "dingtalk", Options: map[string]any{"token": "x"}}},
					Users: &UsersConfig{
						DefaultRole: "member",
						Roles: map[string]RoleConfig{
							"admin":  {UserIDs: []string{"admin1"}},
							"member": {UserIDs: []string{"*"}},
						},
					},
				}},
			},
			wantErr: "",
		},
		{
			name: "valid with wildcard in one role only",
			cfg: Config{
				Projects: []ProjectConfig{{
					Name:      "p1",
					Agent:     AgentConfig{Type: "codex"},
					Platforms: []PlatformConfig{{Type: "dingtalk", Options: map[string]any{"token": "x"}}},
					Users: &UsersConfig{
						Roles: map[string]RoleConfig{
							"admin":  {UserIDs: []string{"u1"}},
							"member": {UserIDs: []string{"*", "u2"}},
						},
					},
				}},
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUsersConfig("projects[0]", tt.cfg.Projects[0].Users)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Error("expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

// --- cloneStringMap tests ---

func TestCloneStringMap(t *testing.T) {
	// nil map
	if got := cloneStringMap(nil); got != nil {
		t.Errorf("cloneStringMap(nil) = %v, want nil", got)
	}

	// empty map
	empty := cloneStringMap(map[string]string{})
	if got := cloneStringMap(empty); got == nil || len(got) != 0 {
		t.Errorf("cloneStringMap(empty) = %v, want empty non-nil map", got)
	}

	// populated map
	orig := map[string]string{"key1": "val1", "key2": "val2"}
	cloned := cloneStringMap(orig)
	if len(cloned) != len(orig) {
		t.Errorf("length mismatch: got %d, want %d", len(cloned), len(orig))
	}
	for k, v := range orig {
		if cloned[k] != v {
			t.Errorf("cloneStringMap[%q] = %q, want %q", k, cloned[k], v)
		}
	}
	// verify it's a deep copy
	delete(cloned, "key1")
	if _, ok := orig["key1"]; !ok {
		t.Error("cloneStringMap returned same map reference, not a copy")
	}
}

// --- pickAgentTemplateForNewProject tests ---

func TestPickAgentTemplateForNewProject(t *testing.T) {
	baseProj := ProjectConfig{
		Name: "base",
		Agent: AgentConfig{
			Type:    "claudecode",
			Options: map[string]any{"mode": "yolo"},
			Providers: []ProviderConfig{{
				Name:   "openai",
				APIKey: "sk-test",
				Model:  "gpt-4",
			}},
		},
		Platforms: []PlatformConfig{{Type: "dingtalk", Options: map[string]any{"token": "x"}}},
	}

	t.Run("clone from existing project", func(t *testing.T) {
		cfg := &Config{Projects: []ProjectConfig{baseProj}}
		opts := NewProjectAgentOptions{CloneFromProject: "base"}
		got := pickAgentTemplateForNewProject(cfg, opts)
		if got.Type != "claudecode" {
			t.Errorf("Type = %q, want claudecode", got.Type)
		}
		if len(got.Providers) != 1 || got.Providers[0].APIKey != "sk-test" {
			t.Errorf("Providers not cloned correctly")
		}
	})

	t.Run("no clone but has projects", func(t *testing.T) {
		cfg := &Config{Projects: []ProjectConfig{baseProj}}
		opts := NewProjectAgentOptions{}
		got := pickAgentTemplateForNewProject(cfg, opts)
		if got.Type != "claudecode" {
			t.Errorf("Type = %q, want claudecode", got.Type)
		}
	})

	t.Run("no projects uses default opencode", func(t *testing.T) {
		cfg := &Config{Projects: []ProjectConfig{}}
		opts := NewProjectAgentOptions{}
		got := pickAgentTemplateForNewProject(cfg, opts)
		if got.Type != "opencode" {
			t.Errorf("Type = %q, want opencode", got.Type)
		}
		if got.Options == nil {
			t.Error("Options should not be nil")
		}
	})

	t.Run("no projects with explicit agent type", func(t *testing.T) {
		cfg := &Config{Projects: []ProjectConfig{}}
		opts := NewProjectAgentOptions{AgentType: "gemini"}
		got := pickAgentTemplateForNewProject(cfg, opts)
		if got.Type != "gemini" {
			t.Errorf("Type = %q, want gemini", got.Type)
		}
	})

	t.Run("explicit agent type overrides clone from first project", func(t *testing.T) {
		cfg := &Config{Projects: []ProjectConfig{baseProj}}
		opts := NewProjectAgentOptions{AgentType: "cursor"}
		got := pickAgentTemplateForNewProject(cfg, opts)
		if got.Type != "cursor" {
			t.Errorf("Type = %q, want cursor (explicit AgentType should take priority over cloning first project)", got.Type)
		}
	})
}

// --- cloneAgentConfig tests ---

func TestCloneAgentConfig(t *testing.T) {
	t.Run("without providers", func(t *testing.T) {
		in := AgentConfig{
			Type:    "codex",
			Options: map[string]any{"mode": "default"},
		}
		got := cloneAgentConfig(in)
		if got.Type != "codex" {
			t.Errorf("Type = %q, want codex", got.Type)
		}
		if got.Options["mode"] != "default" {
			t.Errorf("Options not cloned")
		}
		if len(got.Providers) != 0 {
			t.Errorf("Providers length = %d, want 0", len(got.Providers))
		}
	})

	t.Run("with providers", func(t *testing.T) {
		in := AgentConfig{
			Type:    "claudecode",
			Options: map[string]any{"work_dir": "/tmp/test"},
			Providers: []ProviderConfig{
				{
					Name:     "openai",
					APIKey:   "sk-test",
					BaseURL:  "https://api.openai.com",
					Model:    "gpt-4",
					Thinking: "on",
					Env:      map[string]string{"DEBUG": "1"},
				},
			},
		}
		got := cloneAgentConfig(in)
		if len(got.Providers) != 1 {
			t.Fatalf("Providers length = %d, want 1", len(got.Providers))
		}
		p := got.Providers[0]
		if p.Name != "openai" || p.APIKey != "sk-test" || p.BaseURL != "https://api.openai.com" || p.Model != "gpt-4" {
			t.Errorf("Provider fields not cloned correctly: %+v", p)
		}
		if p.Env["DEBUG"] != "1" {
			t.Errorf("Provider Env not cloned correctly")
		}
		// Verify deep copy of Options
		got.Options["mode"] = "changed"
		if in.Options["mode"] == "changed" {
			t.Error("Options is same reference, not a deep copy")
		}
		// Verify deep copy of Provider Env
		delete(got.Providers[0].Env, "DEBUG")
		if in.Providers[0].Env["DEBUG"] == "" {
			t.Error("Provider Env is same reference, not a deep copy")
		}
	})
}

