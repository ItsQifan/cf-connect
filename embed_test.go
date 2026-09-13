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

// TestConfigExampleTOML_AllowFromIsPlatformLevel is the regression test for the
// silently-ignored-allowlist bug.
//
// `allow_from` is read by the platform adapter out of
// [projects.platforms.options]. It is not a field of ProjectConfig, and
// BurntSushi/toml ignores unknown keys, so the version of this template that
// documented `allow_from` under [[projects]] produced a bot that accepted every
// user while appearing to be locked to one — no error, no warning.
//
// The shipped template is what users actually copy, so pin the level it
// documents. (config.TestMisplacedKeyWarning_* covers the startup warning that
// now catches hand-edited configs.)
func TestConfigExampleTOML_AllowFromIsPlatformLevel(t *testing.T) {
	section := ""
	allowFromSections := map[string]int{}

	for _, line := range strings.Split(ConfigExampleTOML, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			section = trimmed
			continue
		}
		// A commented-out example is still documentation of where the key goes.
		key := strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
		if strings.HasPrefix(key, "allow_from") {
			allowFromSections[section]++
		}
	}

	if allowFromSections["[projects.platforms.options]"] == 0 {
		t.Errorf("config.example.toml does not document allow_from under "+
			"[projects.platforms.options]; occurrences by section: %v", allowFromSections)
	}
	if n := allowFromSections["[[projects]]"]; n != 0 {
		t.Errorf("config.example.toml documents allow_from under [[projects]] "+
			"(%d occurrence(s)); that key is dropped silently by the TOML decoder, "+
			"leaving the bot open to everyone", n)
	}
}

// TestConfigExampleTOML_AdminFromIsProjectLevel is the regression test for a
// false statement in the shipped template.
//
// The template used to say admin_from "defaults to allow_from". It does not:
// main.go hands proj.AdminFrom straight to Engine.SetAdminFrom, and isAdmin
// treats an empty allowlist as deny-all (core/engine.go). A user who trusted
// that comment therefore had every privileged command refused while believing
// the allowlist covered them. Pin both halves — the level the key belongs to,
// and the absence of the bogus default.
func TestConfigExampleTOML_AdminFromIsProjectLevel(t *testing.T) {
	section := ""
	adminFromSections := map[string]int{}

	for _, line := range strings.Split(ConfigExampleTOML, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			section = trimmed
			continue
		}
		key := strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
		if strings.HasPrefix(key, "admin_from") {
			adminFromSections[section]++
		}
	}

	if adminFromSections["[[projects]]"] == 0 {
		t.Errorf("config.example.toml does not document admin_from under "+
			"[[projects]]; occurrences by section: %v", adminFromSections)
	}
	if n := adminFromSections["[projects.platforms.options]"]; n != 0 {
		t.Errorf("config.example.toml documents admin_from under "+
			"[projects.platforms.options] (%d occurrence(s)); it is a "+
			"project-level key, the exact opposite of allow_from", n)
	}

	if strings.Contains(ConfigExampleTOML, "defaults to allow_from") {
		t.Error("config.example.toml claims admin_from defaults to allow_from; " +
			"an unset admin_from is fail-closed (denies every privileged " +
			"command), not an allow_from fallback")
	}
}

// TestConfigExampleTOML_DoesNotAdvertiseDeadDingTalkSwitches guards the other
// half of the DingTalk smoke-test findings: `card_mode` is an upstream Feishu
// Card 2.0 switch and `[stream_preview]` needs a platform MessageUpdater, which
// the DingTalk adapter does not implement. Shipping them in this DingTalk-only
// template sends users down a path that cannot work.
func TestConfigExampleTOML_DoesNotAdvertiseDeadDingTalkSwitches(t *testing.T) {
	for _, line := range strings.Split(ConfigExampleTOML, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "card_mode") {
			t.Errorf("config.example.toml sets an active card_mode; it has no "+
				"effect on DingTalk (AI Card streaming is switched by "+
				"card_template_id): %q", trimmed)
		}
	}
	if strings.Contains(ConfigExampleTOML, "\n[stream_preview]") {
		t.Error("config.example.toml ships an active [stream_preview] block; " +
			"DingTalk has no MessageUpdater, so it is silently ignored")
	}
}
