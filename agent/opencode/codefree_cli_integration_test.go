//go:build codefree_integration

// End-to-end checks that spawn a real agent CLI. Opt-in via build tag because
// they need a working CodeFree-O/OpenCode installation with credentials:
//
//	go test -tags codefree_integration ./agent/opencode/ -run TestRealCLI -v
//
// These cover the two acceptance items that cannot be checked with a fake CLI:
//   - a session can actually be started and driven for both `cmd` values
//   - yolo mode does not fail with an "unknown flag" error
package opencode

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/ItsQifan/cf-connect/core"
)

// notLoggedIn reports whether an agent error means "the CLI wants an
// interactive login" rather than "the adapter passed something wrong".
//
// CodeFree-O answers `run` with an OAuth prompt when its stored refresh token
// is unavailable, which happens in headless CI and in sandboxes that cannot
// reach the OS credential store. That is an environment problem, not a code
// defect, so the integration tests skip instead of failing.
func notLoggedIn(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, marker := range []string{
		"登录超时", "正在通过 codefree登录", "授权", "login", "oauth",
		"not logged in", "unauthorized", "authentication",
	} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

// firstAvailableCLI returns the first configured CLI that resolves on PATH.
func firstAvailableCLI(t *testing.T, candidates ...string) string {
	t.Helper()
	for _, c := range candidates {
		if _, err := exec.LookPath(c); err == nil {
			return c
		}
	}
	t.Skipf("none of %v is on PATH", candidates)
	return ""
}

// runOneTurn starts a session, sends one prompt, and returns the concatenated
// assistant text. It fails on any EventError the CLI reports, except a missing
// login, which skips the test.
func runOneTurn(t *testing.T, opts map[string]any) string {
	t.Helper()

	a, err := New(opts)
	if err != nil {
		t.Fatalf("New(%v): %v", opts, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	sess, err := a.StartSession(ctx, "")
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	defer sess.Close()

	if err := sess.Send("reply with exactly the word PONG and nothing else", "m1", nil, nil); err != nil {
		t.Fatalf("Send: %v", err)
	}

	var out strings.Builder
	var sendErr error
	deadline := time.After(180 * time.Second)

	for {
		select {
		case ev := <-sess.Events():
			switch ev.Type {
			case core.EventText:
				out.WriteString(ev.Content)
			case core.EventError:
				sendErr = ev.Error
			case core.EventResult:
				if sendErr != nil {
					if notLoggedIn(sendErr) {
						t.Skipf("%s is not logged in: %v", opts["cmd"], sendErr)
					}
					t.Fatalf("agent reported an error: %v", sendErr)
				}
				got := out.String()
				if strings.TrimSpace(got) == "" {
					t.Fatal("agent produced no text")
				}
				return got
			}
		case <-deadline:
			t.Fatalf("timed out; partial output = %q", out.String())
		}
	}
}

// newWorkDir returns a temp working directory that outlives the test.
//
// CodeFree-O keeps a handle on its working directory (sqlite, snapshots) for a
// moment after the CLI process reports its final result. t.TempDir() removes
// the directory during test cleanup, which on Windows then fails with
// "the process cannot access the file because it is being used by another
// process" - a teardown artefact that would mask a real result. Cleaning up in
// TestMain-ish fashion with a delay is not possible here, so the directory is
// simply left to the OS temp reaper, and the test asserts on behaviour only.
func newWorkDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "cf-connect-cli-it-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() {
		// Best effort: the CLI may still hold the directory briefly.
		for i := 0; i < 10; i++ {
			if err := os.RemoveAll(dir); err == nil {
				return
			}
			time.Sleep(500 * time.Millisecond)
		}
		t.Logf("note: could not remove %s (a CLI process still holds it)", dir)
	})
	return dir
}

// TestRealCLI_DefaultModeForBothBinaryNames covers acceptance item 9.1: the
// same adapter must drive `opencode` and `codefree-o` in default mode.
func TestRealCLI_DefaultModeForBothBinaryNames(t *testing.T) {
	for _, bin := range []string{"codefree-o", "opencode"} {
		bin := bin
		t.Run(bin, func(t *testing.T) {
			if _, err := exec.LookPath(bin); err != nil {
				t.Skipf("%s not on PATH", bin)
			}
			got := runOneTurn(t, map[string]any{
				"work_dir": newWorkDir(t),
				"cmd":      bin,
				"mode":     "default",
			})
			if !strings.Contains(strings.ToUpper(got), "PONG") {
				t.Errorf("response = %q, want it to contain PONG", got)
			}
		})
	}
}

// TestRealCLI_YoloModeDoesNotHitUnknownFlag covers acceptance item 9.2: the
// default yolo flag must be accepted by both CLIs.
func TestRealCLI_YoloModeDoesNotHitUnknownFlag(t *testing.T) {
	for _, bin := range []string{"codefree-o", "opencode"} {
		bin := bin
		t.Run(bin, func(t *testing.T) {
			if _, err := exec.LookPath(bin); err != nil {
				t.Skipf("%s not on PATH", bin)
			}

			a, err := New(map[string]any{
				"work_dir": newWorkDir(t),
				"cmd":      bin,
				"mode":     "yolo",
			})
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
			defer cancel()
			sess, err := a.StartSession(ctx, "")
			if err != nil {
				t.Fatalf("StartSession: %v", err)
			}
			defer sess.Close()

			if err := sess.Send("say ok", "m1", nil, nil); err != nil {
				t.Fatalf("Send: %v", err)
			}

			deadline := time.After(180 * time.Second)
			for {
				select {
				case ev := <-sess.Events():
					if ev.Type == core.EventError && ev.Error != nil {
						msg := ev.Error.Error()
						// The whole point of patch 1: the flag the adapter
						// appends in yolo mode must be one the CLI accepts.
						if strings.Contains(strings.ToLower(msg), "unknown flag") ||
							strings.Contains(strings.ToLower(msg), "unknown option") {
							t.Fatalf("yolo mode passed an unsupported flag: %v", ev.Error)
						}
						if notLoggedIn(ev.Error) {
							t.Skipf("%s is not logged in: %v", bin, ev.Error)
						}
						t.Fatalf("agent error: %v", ev.Error)
					}
					if ev.Type == core.EventResult {
						return
					}
				case <-deadline:
					t.Fatal("timed out waiting for the turn to finish")
				}
			}
		})
	}
}

// TestRealCLI_DoctorReportsConfiguredBinary covers acceptance item 9.4.
func TestRealCLI_DoctorReportsConfiguredBinary(t *testing.T) {
	bin := firstAvailableCLI(t, "codefree-o", "opencode")

	a, err := New(map[string]any{"work_dir": newWorkDir(t), "cmd": bin})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	info, ok := a.(core.AgentDoctorInfo)
	if !ok {
		t.Fatal("agent does not implement core.AgentDoctorInfo")
	}
	if got := info.CLIBinaryName(); !strings.Contains(got, bin) {
		t.Errorf("CLIBinaryName() = %q, want it to name %q", got, bin)
	}
	t.Logf("doctor would report: binary=%q display=%q", info.CLIBinaryName(), info.CLIDisplayName())
}
