package ccconnect

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// linkRE matches markdown links `[text](target)` and `[text](target#anchor)`.
var linkRE = regexp.MustCompile(`\]\(([^)\s]+?)(#[^)]*)?\)`)

// docLinkIllustrations are links that appear inside documentation *prose* as
// examples of link syntax rather than as real navigation. Rewriting them would
// make the docs wrong, so they are explicitly allow-listed here.
var docLinkIllustrations = map[string]bool{
	"/abs/path/file.ts": true,
}

// TestDocsHaveNoDanglingLinks guards the docs against pointing at files that no
// longer exist. The platform trim deleted 19 platform adapters and 16 agents
// along with their guides, and a stale `[guide](./feishu.md)` is invisible in
// review but obvious to a reader who clicks it.
func TestDocsHaveNoDanglingLinks(t *testing.T) {
	skipDirs := map[string]bool{
		".git": true, "node_modules": true, "dist": true, "vendor": true,
		".tmp-tools": true, "changelogs": true, "npm": true, ".vite": true,
	}

	var checked int
	var dangling []string

	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		// CHANGELOG.md describes historical releases and intentionally refers to
		// things that have since been removed.
		if !strings.HasSuffix(path, ".md") || filepath.Base(path) == "CHANGELOG.md" {
			return nil
		}

		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		for _, m := range linkRE.FindAllStringSubmatch(string(body), -1) {
			target := m[1]
			if strings.HasPrefix(target, "http://") ||
				strings.HasPrefix(target, "https://") ||
				strings.HasPrefix(target, "mailto:") ||
				strings.HasPrefix(target, "#") ||
				strings.HasPrefix(target, "data:") {
				continue
			}
			if docLinkIllustrations[target] {
				continue
			}
			checked++

			resolved := target
			if !filepath.IsAbs(resolved) {
				resolved = filepath.Join(filepath.Dir(path), target)
			}
			if _, err := os.Stat(resolved); err != nil {
				dangling = append(dangling, path+" -> "+target)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk docs: %v", err)
	}

	if checked < 20 {
		t.Fatalf("only %d relative links checked; the walker probably missed the docs tree", checked)
	}
	for _, d := range dangling {
		t.Errorf("dangling documentation link: %s", d)
	}
}

// TestDocsDoNotAdvertiseRemovedPlatformGuides pins that the per-platform guides
// for adapters this fork does not ship stay deleted.
func TestDocsDoNotAdvertiseRemovedPlatformGuides(t *testing.T) {
	removed := []string{
		"feishu.md", "weixin.md", "wecom.md", "telegram.md", "slack.md",
		"discord.md", "qq.md", "qqbot.md", "line.md", "weibo.md", "tuitui.md",
		"matrix.md", "webex.md", "max-webhook.md", "cloud-web.md",
		"googlechat.md", "wps-xiezuo.md",
	}
	for _, name := range removed {
		if _, err := os.Stat(filepath.Join("docs", name)); err == nil {
			t.Errorf("docs/%s still exists but its adapter was removed", name)
		}
	}
}
