package ccconnect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ItsQifan/cf-connect/config"
)

// TestConfigExampleTOML_Parses pins the shipped configuration template.
//
// `cf-connect config example` prints this string verbatim and QUICKSTART.md
// tells users to save it as config.toml, so a syntax error or a reference to an
// option that no longer exists would break the documented first-run path.
func TestConfigExampleTOML_Parses(t *testing.T) {
	if strings.TrimSpace(ConfigExampleTOML) == "" {
		t.Fatal("ConfigExampleTOML is empty: the go:embed directive is broken")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(ConfigExampleTOML), 0o644); err != nil {
		t.Fatalf("write example config: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("the shipped config.example.toml does not parse: %v", err)
	}
	if len(cfg.Projects) != 1 {
		t.Fatalf("projects = %d, want exactly 1 example project", len(cfg.Projects))
	}

	proj := cfg.Projects[0]
	if proj.Agent.Type == "" {
		t.Error("example project has no agent type")
	}
	if len(proj.Platforms) != 1 {
		t.Fatalf("platforms = %d, want exactly 1 example platform", len(proj.Platforms))
	}
	if got := proj.Platforms[0].Type; got != "dingtalk" {
		t.Errorf("example platform type = %q, want dingtalk", got)
	}
}

// TestConfigExampleTOML_OnlyShipsSupportedAdapters guards the trim: the example
// is the first thing a user edits, so it must not advertise a platform or agent
// that this fork no longer compiles in.
func TestConfigExampleTOML_OnlyShipsSupportedAdapters(t *testing.T) {
	removed := []string{
		"claudecode", "codex", "cursor", "gemini", "iflow", "qoder", "kimi",
		"antigravity", "copilot", "devin", "reasonix", "tmux",
		"feishu", "lark", "weixin", "wecom", "telegram", "discord", "slack",
		"qqbot", "yuanbao", "tuitui", "wps-", "cloud_web", "matrix", "webex",
		"googlechat", "max",
	}

	body := ConfigExampleTOML
	// Strip comment lines: prose may legitimately mention a removed platform
	// (e.g. QUICKSTART-style guidance), but no active key may.
	var active []string
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		active = append(active, strings.ToLower(trimmed))
	}
	activeText := strings.Join(active, "\n")

	for _, name := range removed {
		if strings.Contains(activeText, name) {
			t.Errorf("config.example.toml still contains an active setting for removed adapter %q", name)
		}
	}

	if !strings.Contains(activeText, `type = "opencode"`) {
		t.Error(`config.example.toml should select the opencode agent with type = "opencode"`)
	}
	if !strings.Contains(activeText, `type = "dingtalk"`) {
		t.Error(`config.example.toml should select DingTalk with type = "dingtalk"`)
	}
}

// TestConfigExampleTOML_DocumentsCodefreeOptions ensures the options the
// codefree-o compatibility layer added are actually discoverable in the
// template; they are the whole reason this fork exists.
func TestConfigExampleTOML_DocumentsCodefreeOptions(t *testing.T) {
	for _, want := range []string{
		"permission_flag",
		"codefree-o",
		"codefree.db",
		"global_memory_file",
	} {
		if !strings.Contains(ConfigExampleTOML, want) {
			t.Errorf("config.example.toml does not mention %q", want)
		}
	}
}
