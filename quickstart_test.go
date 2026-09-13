package ccconnect

import (
	"os"
	"strings"
	"testing"
)

// TestQuickstartDocumentsAdminFrom is the regression test for the shipped-doc
// gap that made /dir look broken.
//
// QUICKSTART.md is the only guide that travels inside the release zip — docs/
// is not packaged by scripts/release-windows.ps1. It documented allow_from but
// said nothing about admin_from, while every privileged command (/dir, /shell,
// /show, /diff, /web, /restart, /upgrade) is fail-closed when admin_from is
// unset (core/engine.go isAdmin). A user who followed the quickstart exactly
// therefore got
//
//	Command /dir requires admin privilege. Set admin_from in config ...
//
// with no documented remedy anywhere in the zip. Pin the three things that fix
// it: the key name, the user-visible symptom, and the command that surfaces it.
//
// The symptom string is quoted deliberately — the quickstart already quotes the
// allow_from startup warning the same way, so a user can match what they see on
// screen against the doc.
func TestQuickstartDocumentsAdminFrom(t *testing.T) {
	body, err := os.ReadFile("QUICKSTART.md")
	if err != nil {
		t.Fatalf("read QUICKSTART.md: %v", err)
	}
	text := string(body)

	for _, want := range []string{
		"admin_from",
		"requires admin privilege",
		"/dir",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("QUICKSTART.md does not mention %q; the privileged "+
				"commands are denied without admin_from, and docs/ is not "+
				"shipped in the release zip", want)
		}
	}
}
