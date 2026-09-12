package core

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestTransformLocalReferences_DisabledWithoutNormalizeAgents(t *testing.T) {
	cfg := ReferenceRenderCfg{
		RenderPlatforms: []string{"dingtalk"},
		DisplayPath:     "basename",
		MarkerStyle:     "none",
		EnclosureStyle:  "none",
	}
	input := "See /root/code/demo/src/app.ts:42"
	got := TransformLocalReferences(input, cfg, "opencode", "dingtalk", "/root/code/demo")
	if got != input {
		t.Fatalf("TransformLocalReferences() = %q, want unchanged %q", got, input)
	}
}

func TestTransformLocalReferences_UsesAllScopes(t *testing.T) {
	cfg := ReferenceRenderCfg{
		NormalizeAgents: []string{"all"},
		RenderPlatforms: []string{"all"},
		DisplayPath:     "basename",
		MarkerStyle:     "emoji",
		EnclosureStyle:  "code",
	}
	got := TransformLocalReferences("See /root/code/demo/src/app.ts:42", cfg, "opencode", "dingtalk", "/root/code/demo")
	if !strings.Contains(got, "📄 `app.ts:42`") {
		t.Fatalf("TransformLocalReferences() = %q, want rendered basename reference", got)
	}
}

func TestTransformLocalReferences_PreservesWebMarkdownLinks(t *testing.T) {
	cfg := ReferenceRenderCfg{
		NormalizeAgents: []string{"opencode"},
		RenderPlatforms: []string{"dingtalk"},
		DisplayPath:     "basename",
		MarkerStyle:     "none",
		EnclosureStyle:  "none",
	}
	input := "Docs: [OpenAI](https://openai.com/) and [app.ts](/root/code/demo/src/app.ts#L42)"
	got := TransformLocalReferences(input, cfg, "opencode", "dingtalk", "/root/code/demo")
	if !strings.Contains(got, "[OpenAI](https://openai.com/)") {
		t.Fatalf("TransformLocalReferences() = %q, want web link preserved", got)
	}
	if !strings.Contains(got, "app.ts#L42") {
		t.Fatalf("TransformLocalReferences() = %q, want local hash-line reference rendered", got)
	}
}

func TestTransformLocalReferences_PreservesInlineCodePathRange(t *testing.T) {
	cfg := ReferenceRenderCfg{
		NormalizeAgents: []string{"opencode"},
		RenderPlatforms: []string{"dingtalk"},
		DisplayPath:     "dirname_basename",
		MarkerStyle:     "ascii",
		EnclosureStyle:  "code",
	}
	got := TransformLocalReferences("Inspect `/root/.claude/settings.json:5-10` next.", cfg, "opencode", "dingtalk", "/root")
	want := "[FILE] `.claude/settings.json:5-10`"
	if !strings.Contains(got, want) {
		t.Fatalf("TransformLocalReferences() = %q, want substring %q", got, want)
	}
}

func TestTransformLocalReferences_PreservesWebMarkdownLinksAfterInlineCodeReference(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TransformLocalReferences path handling assumes Unix separators")
	}
	cfg := ReferenceRenderCfg{
		NormalizeAgents: []string{"opencode"},
		RenderPlatforms: []string{"dingtalk"},
		DisplayPath:     "relative",
		MarkerStyle:     "emoji",
		EnclosureStyle:  "code",
	}
	input := "`/root/code/.claude/settings.json:5-10`\n[OpenAI](https://openai.com/)"
	got := TransformLocalReferences(input, cfg, "opencode", "dingtalk", "/root/code")
	want := "📄 `.claude/settings.json:5-10`\n[OpenAI](https://openai.com/)"
	if got != want {
		t.Fatalf("TransformLocalReferences() = %q, want %q", got, want)
	}
}

func TestTransformLocalReferences_SmartDisplayFallsBackOnBasenameCollision(t *testing.T) {
	cfg := ReferenceRenderCfg{
		NormalizeAgents: []string{"opencode"},
		RenderPlatforms: []string{"dingtalk"},
		DisplayPath:     "smart",
		MarkerStyle:     "none",
		EnclosureStyle:  "none",
	}
	input := "Compare /root/code/demo/src/app.ts and /root/code/demo/tests/app.ts"
	got := TransformLocalReferences(input, cfg, "opencode", "dingtalk", "/root/code/demo")
	if !strings.Contains(got, "src/app.ts") || !strings.Contains(got, "tests/app.ts") {
		t.Fatalf("TransformLocalReferences() = %q, want dirname+basename for both colliding refs", got)
	}
}

func TestTransformLocalReferences_RelativeDisplayUsesWorkspace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TransformLocalReferences path handling assumes Unix separators")
	}
	cfg := ReferenceRenderCfg{
		NormalizeAgents: []string{"opencode"},
		RenderPlatforms: []string{"dingtalk"},
		DisplayPath:     "relative",
		MarkerStyle:     "emoji",
		EnclosureStyle:  "code",
	}
	got := TransformLocalReferences("Look at /root/code/demo/src/app.ts:42:7", cfg, "opencode", "dingtalk", "/root/code/demo")
	want := "📄 `src/app.ts:42:7`"
	if !strings.Contains(got, want) {
		t.Fatalf("TransformLocalReferences() = %q, want substring %q", got, want)
	}
}

func TestTransformLocalReferences_RelativeInputIsNotSplitByAbsoluteMatcher(t *testing.T) {
	cfg := ReferenceRenderCfg{
		NormalizeAgents: []string{"opencode"},
		RenderPlatforms: []string{"dingtalk"},
		DisplayPath:     "relative",
		MarkerStyle:     "emoji",
		EnclosureStyle:  "code",
	}
	input := "See lean-steward/src/lean_topo_steward/prompting/instructions/global_instructions.py:42"
	got := TransformLocalReferences(input, cfg, "opencode", "dingtalk", "/root/code")
	want := "See 📄 `lean-steward/src/lean_topo_steward/prompting/instructions/global_instructions.py:42`"
	if got != want {
		t.Fatalf("TransformLocalReferences() = %q, want %q", got, want)
	}
}

func TestTransformLocalReferences_ChineseListSeparatorsDoNotMergeCandidates(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TransformLocalReferences path handling assumes Unix separators")
	}
	workspace := t.TempDir()
	filePath := filepath.Join(workspace, "demo-repo", "README")
	profileDir := filepath.Join(workspace, "demo-repo", "src", "components", "profile")
	profileExtDir := filepath.Join(workspace, "demo-repo", "src", "components", "profile.ts")
	specDir := filepath.Join(workspace, "demo-repo", "docs", "spec.v1")
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatalf("MkdirAll(file dir) error: %v", err)
	}
	if err := os.WriteFile(filePath, []byte("readme"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	for _, dir := range []string{profileDir, profileExtDir, specDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error: %v", dir, err)
		}
	}

	cfg := ReferenceRenderCfg{
		NormalizeAgents: []string{"opencode"},
		RenderPlatforms: []string{"dingtalk"},
		DisplayPath:     "relative",
		MarkerStyle:     "emoji",
		EnclosureStyle:  "code",
	}
	input := "第 1 步：正在处理路径 demo-repo/README、" + profileDir + "、" + profileExtDir + "、" + specDir + "。"
	got := TransformLocalReferences(input, cfg, "opencode", "dingtalk", workspace)
	want := "第 1 步：正在处理路径 📄 `demo-repo/README`、📁 `demo-repo/src/components/profile/`、📁 `demo-repo/src/components/profile.ts/`、📁 `demo-repo/docs/spec.v1/`。"
	if got != want {
		t.Fatalf("TransformLocalReferences() = %q, want %q", got, want)
	}
}

func TestTransformLocalReferences_ExistingDirectoryWithoutTrailingSlashIsDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TransformLocalReferences path handling assumes Unix separators")
	}
	workspace := t.TempDir()
	dirPath := filepath.Join(workspace, "demo-repo", "src", "components")
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	cfg := ReferenceRenderCfg{
		NormalizeAgents: []string{"opencode"},
		RenderPlatforms: []string{"dingtalk"},
		DisplayPath:     "relative",
		MarkerStyle:     "emoji",
		EnclosureStyle:  "code",
	}
	got := TransformLocalReferences("Dir "+dirPath, cfg, "opencode", "dingtalk", workspace)
	want := "Dir 📁 `demo-repo/src/components/`"
	if got != want {
		t.Fatalf("TransformLocalReferences() = %q, want %q", got, want)
	}
}

func TestTransformLocalReferences_WorkspaceRootDisplaysAsRelativeRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TransformLocalReferences path handling assumes Unix separators")
	}
	workspace := t.TempDir()
	cfg := ReferenceRenderCfg{
		NormalizeAgents: []string{"opencode"},
		RenderPlatforms: []string{"dingtalk"},
		DisplayPath:     "relative",
		MarkerStyle:     "emoji",
		EnclosureStyle:  "code",
	}
	got := TransformLocalReferences("Root "+workspace, cfg, "opencode", "dingtalk", workspace)
	want := "Root 📁 `./`"
	if got != want {
		t.Fatalf("TransformLocalReferences() = %q, want %q", got, want)
	}
}

func TestTransformLocalReferences_UnknownNoExtPathKeepsNoMarker(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TransformLocalReferences path handling assumes Unix separators")
	}
	workspace := t.TempDir()
	unknown := filepath.Join(workspace, "mysterypath")
	cfg := ReferenceRenderCfg{
		NormalizeAgents: []string{"opencode"},
		RenderPlatforms: []string{"dingtalk"},
		DisplayPath:     "relative",
		MarkerStyle:     "emoji",
		EnclosureStyle:  "code",
	}
	got := TransformLocalReferences("Unknown "+unknown, cfg, "opencode", "dingtalk", workspace)
	want := "Unknown `mysterypath`"
	if got != want {
		t.Fatalf("TransformLocalReferences() = %q, want %q", got, want)
	}
}

// TestReferenceRender_ScopesCoverTheForkAdapters is the regression test for the
// documented [projects.references] feature being permanently inert.
//
// supportedReferenceNormalizeAgents/RenderPlatforms still named the upstream
// adapters (codex/claudecode, feishu/weixin) after the trim. Because "all"
// expands to the supported set — and docs/usage.zh-CN.md recommends
// `["all"]`/`["all"]` — the renderer could never match this fork's runtime agent
// ("opencode") or platform ("dingtalk"), so the opt-in silently did nothing.
func TestReferenceRender_ScopesCoverTheForkAdapters(t *testing.T) {
	// The documented "all"/"all" configuration must enable rendering for the
	// adapters this fork actually compiles in.
	allCfg := normalizeReferenceRenderCfg(ReferenceRenderCfg{
		NormalizeAgents: []string{"all"},
		RenderPlatforms: []string{"all"},
	})
	if !allCfg.renderEnabled("opencode", "dingtalk") {
		t.Errorf(`references=["all"]/["all"] does not enable the renderer for `+
			`opencode+dingtalk; expanded scopes are agents=%v platforms=%v`,
			allCfg.NormalizeAgents, allCfg.RenderPlatforms)
	}

	// Naming the adapters explicitly must survive scope normalization too.
	explicitCfg := normalizeReferenceRenderCfg(ReferenceRenderCfg{
		NormalizeAgents: []string{"opencode"},
		RenderPlatforms: []string{"dingtalk"},
	})
	if !explicitCfg.renderEnabled("opencode", "dingtalk") {
		t.Errorf("explicit normalize_agents/render_platforms values were dropped: "+
			"agents=%v platforms=%v",
			explicitCfg.NormalizeAgents, explicitCfg.RenderPlatforms)
	}
}
