package core

import (
	"os"
	"strings"
	"testing"
)

// displayNameAgent models the real opencode adapter: the registry name is
// "opencode" (that is what CreateAgent and session ownership use), but it drives
// codefree-o, so the user-facing label must be "CodeFree-O".
type displayNameAgent struct {
	stubListAgent
	cliName string
	display string
}

func (a *displayNameAgent) Name() string { return "opencode" }

func (a *displayNameAgent) CLIBinaryName() string {
	if a.cliName != "" {
		return a.cliName
	}
	return "codefree-o"
}

func (a *displayNameAgent) CLIDisplayName() string { return a.display }

// TestAgentDisplayName_PrefersCLIDisplayName pins the resolution order that keeps
// registry keys out of user-visible text.
func TestAgentDisplayName_PrefersCLIDisplayName(t *testing.T) {
	tests := []struct {
		name  string
		agent Agent
		want  string
	}{
		{
			name:  "doctor-info display name wins over the registry key",
			agent: &displayNameAgent{display: "CodeFree-O"},
			want:  "CodeFree-O",
		},
		{
			name:  "blank display name falls back to the registry key",
			agent: &displayNameAgent{display: "   "},
			want:  "opencode",
		},
		{
			name:  "agent without the interface uses its registry key",
			agent: &stubAgent{},
			want:  "stub",
		},
		{
			name:  "nil agent is tolerated",
			agent: nil,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AgentDisplayName(tt.agent); got != tt.want {
				t.Errorf("AgentDisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestCmdList_TitleUsesAgentDisplayName is the regression test for the reported
// bug: /list rendered the registry key, so a CodeFree-O deployment showed
// "**opencode 会话列表** (3)".
//
// DingTalk does not implement CardSender, so this plain-text path is the one a
// dingtalk user actually hits.
func TestCmdList_TitleUsesAgentDisplayName(t *testing.T) {
	agent := &displayNameAgent{
		stubListAgent: stubListAgent{
			sessions: []AgentSessionInfo{
				{ID: "ses_1", Summary: "first", MessageCount: 2},
			},
		},
		display: "CodeFree-O",
	}
	p := &stubPlatformEngine{n: "plain"}
	e := NewEngine("test", agent, []Platform{p}, "", LangChinese)
	userKey := "dingtalk:d:conv:user"

	e.sessions.GetOrCreateActive(userKey).SetAgentSessionID("ses_1", "opencode")
	e.sessions.Save()

	p.sent = nil
	e.cmdList(p, &Message{SessionKey: userKey, ReplyCtx: "ctx"}, nil)

	if len(p.sent) != 1 {
		t.Fatalf("expected 1 reply, got %d: %v", len(p.sent), p.sent)
	}
	reply := p.sent[0]
	if !strings.Contains(reply, "CodeFree-O") {
		t.Errorf("/list title should name the configured CLI (CodeFree-O):\n%s", reply)
	}
	if strings.Contains(reply, "opencode") {
		t.Errorf("/list title still leaks the registry key \"opencode\":\n%s", reply)
	}
}

// TestCmdList_PagedTitleUsesAgentDisplayName covers the paged variant, which uses
// a separate i18n key and was a second copy of the same bug.
func TestCmdList_PagedTitleUsesAgentDisplayName(t *testing.T) {
	sessions := make([]AgentSessionInfo, listPageSize+1)
	for i := range sessions {
		sessions[i] = AgentSessionInfo{
			ID:           "ses_" + strings.Repeat("x", i+1),
			Summary:      "s",
			MessageCount: 1,
		}
	}

	agent := &displayNameAgent{
		stubListAgent: stubListAgent{sessions: sessions},
		display:       "CodeFree-O",
	}
	p := &stubPlatformEngine{n: "plain"}
	e := NewEngine("test", agent, []Platform{p}, "", LangChinese)
	userKey := "dingtalk:d:conv:user"

	p.sent = nil
	e.cmdList(p, &Message{SessionKey: userKey, ReplyCtx: "ctx"}, nil)

	if len(p.sent) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(p.sent))
	}
	if reply := p.sent[0]; !strings.Contains(reply, "CodeFree-O") || strings.Contains(reply, "opencode") {
		t.Errorf("paged /list title should name CodeFree-O, not the registry key:\n%s", reply)
	}
}

// TestRenderListCard_TitleUsesAgentDisplayName covers the card variant, used by
// platforms that do implement CardSender.
func TestRenderListCard_TitleUsesAgentDisplayName(t *testing.T) {
	agent := &displayNameAgent{
		stubListAgent: stubListAgent{
			sessions: []AgentSessionInfo{
				{ID: "ses_1", Summary: "first", MessageCount: 2},
			},
		},
		display: "CodeFree-O",
	}
	p := &stubCardPlatform{stubPlatformEngine: stubPlatformEngine{n: "cards"}}
	e := NewEngine("test", agent, []Platform{p}, "", LangChinese)
	userKey := "dingtalk:d:conv:user"

	e.sessions.GetOrCreateActive(userKey).SetAgentSessionID("ses_1", "opencode")
	e.sessions.Save()

	card, err := e.renderListCard(userKey, 1)
	if err != nil {
		t.Fatalf("renderListCard: %v", err)
	}
	if card == nil || card.Header == nil {
		t.Fatal("renderListCard returned a card with no header")
	}
	title := card.Header.Title
	if !strings.Contains(title, "CodeFree-O") {
		t.Errorf("card title should name the configured CLI, got %q", title)
	}
	if strings.Contains(title, "opencode") {
		t.Errorf("card title still leaks the registry key \"opencode\": %q", title)
	}
}

// TestCmdSkills_TitleUsesAgentDisplayName covers /skills, which shared the bug.
func TestCmdSkills_TitleUsesAgentDisplayName(t *testing.T) {
	agent := &displayNameAgent{display: "CodeFree-O"}
	p := &stubPlatformEngine{n: "plain"}
	e := NewEngine("test", agent, []Platform{p}, "", LangChinese)

	root := t.TempDir()
	if err := os.Mkdir(root+"/demo", 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(root+"/demo/SKILL.md", []byte("---\ndescription: Demo\n---\nDo demo"), 0o644); err != nil {
		t.Fatalf("write skill file: %v", err)
	}
	e.skills.SetDirs([]string{root})

	p.sent = nil
	e.cmdSkills(p, &Message{SessionKey: "dingtalk:d:conv:user", ReplyCtx: "ctx"})

	if len(p.sent) == 0 {
		t.Fatal("expected /skills to reply")
	}
	reply := p.sent[0]
	if !strings.Contains(reply, "CodeFree-O") {
		t.Errorf("/skills title should name the configured CLI:\n%s", reply)
	}
	if strings.Contains(reply, "opencode") {
		t.Errorf("/skills title still leaks the registry key \"opencode\":\n%s", reply)
	}
}

// TestNormalizeProgressAgentLabel_CodefreeBrand pins the progress-card label map:
// the fork's brand must not render as "Codefree-o".
func TestNormalizeProgressAgentLabel_CodefreeBrand(t *testing.T) {
	for _, in := range []string{"codefree-o", "codefree", "codefreeo", "CodeFree-O"} {
		if got := normalizeProgressAgentLabel(in); got != "CodeFree-O" {
			t.Errorf("normalizeProgressAgentLabel(%q) = %q, want %q", in, got, "CodeFree-O")
		}
	}
}
