package opencode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ItsQifan/cf-connect/core"
)

// TestBuildRunArgs_YoloPermissionFlagIsConfigurable is the regression test for
// the codefree-o incompatibility: the adapter used to hardcode
// `--dangerously-skip-permissions`, which neither current opencode (>= 1.18)
// nor codefree-o accepts, so `mode = "yolo"` failed with "unknown flag".
func TestBuildRunArgs_YoloPermissionFlagIsConfigurable(t *testing.T) {
	tests := []struct {
		name             string
		mode             string
		permissionFlag   string
		wantPresent      string
		wantAbsent       []string
		wantNoExtraFlags bool
	}{
		{
			name:           "default mode appends no permission flag",
			mode:           "default",
			permissionFlag: DefaultPermissionFlag,
			wantAbsent:     []string{"--auto", "--dangerously-skip-permissions"},
		},
		{
			name:           "yolo defaults to --auto",
			mode:           "yolo",
			permissionFlag: DefaultPermissionFlag,
			wantPresent:    "--auto",
			wantAbsent:     []string{"--dangerously-skip-permissions"},
		},
		{
			name:           "yolo honours a custom flag",
			mode:           "yolo",
			permissionFlag: "--dangerously-skip-permissions",
			wantPresent:    "--dangerously-skip-permissions",
			wantAbsent:     []string{"--auto"},
		},
		{
			name:           "yolo with no flag appends nothing",
			mode:           "yolo",
			permissionFlag: "",
			wantAbsent:     []string{"--auto", "--dangerously-skip-permissions"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &opencodeSession{
				workDir:        "/repo",
				mode:           tt.mode,
				permissionFlag: tt.permissionFlag,
			}
			args := s.buildRunArgs("hello", nil, "")

			if tt.wantPresent != "" && !containsArg(args, tt.wantPresent) {
				t.Errorf("args = %v, want %q present", args, tt.wantPresent)
			}
			for _, absent := range tt.wantAbsent {
				if containsArg(args, absent) {
					t.Errorf("args = %v, must not contain %q", args, absent)
				}
			}
		})
	}
}

// TestResolvePermissionFlag covers the option parsing, including the aliases
// that disable the flag.
func TestResolvePermissionFlag(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", DefaultPermissionFlag},
		{"   ", DefaultPermissionFlag},
		{"--auto", "--auto"},
		{"--dangerously-skip-permissions", "--dangerously-skip-permissions"},
		{"  --auto  ", "--auto"},
		{"none", ""},
		{"NONE", ""},
		{"off", ""},
		{"-", ""},
	}
	for _, tt := range tests {
		if got := resolvePermissionFlag(tt.in); got != tt.want {
			t.Errorf("resolvePermissionFlag(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestResolveSessionDBPath_BrandAware is the regression test for patch 2: the
// adapter used to hardcode ~/.local/share/opencode/opencode.db, so a
// codefree-o deployment silently read the wrong database (or none at all) and
// session titles/message counts came back empty.
func TestResolveSessionDBPath_BrandAware(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_DATA_HOME", "")

	// Absolute, platform-correct base paths: filepath.IsAbs("/abs") is false on
	// Windows, so "/abs/custom.db" would be treated as relative there.
	base := t.TempDir()
	absDBFile := filepath.Join(base, "custom.db")
	absent := filepath.Join(base, "ignored")

	tests := []struct {
		name string
		src  sessionDBSource
		want string
	}{
		{
			name: "opencode default",
			src:  sessionDBSource{cmd: "opencode"},
			want: filepath.Join(home, ".local", "share", "opencode", "opencode.db"),
		},
		{
			name: "codefree-o binary name",
			src:  sessionDBSource{cmd: "codefree-o"},
			want: filepath.Join(home, ".codefree-o", ".local", "share", "codefree.db"),
		},
		{
			name: "codefree-o absolute path is still detected",
			src:  sessionDBSource{cmd: filepath.Join(base, "codefree-o.exe")},
			want: filepath.Join(home, ".codefree-o", ".local", "share", "codefree.db"),
		},
		{
			name: "explicit data_dir keeps brand filename",
			src:  sessionDBSource{cmd: "codefree-o", dataDir: filepath.Join(base, "share")},
			want: filepath.Join(base, "share", "codefree.db"),
		},
		{
			name: "explicit db_file absolute wins",
			src: sessionDBSource{
				cmd:     "opencode",
				dataDir: absent,
				dbFile:  absDBFile,
			},
			want: absDBFile,
		},
		{
			name: "explicit db_file relative joins data_dir",
			src: sessionDBSource{
				cmd:     "opencode",
				dataDir: filepath.Join(base, "share"),
				dbFile:  "custom.db",
			},
			want: filepath.Join(base, "share", "custom.db"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveSessionDBPath(tt.src); got != tt.want {
				t.Errorf("resolveSessionDBPath(%+v) = %q, want %q", tt.src, got, tt.want)
			}
		})
	}
}

// TestResolveSessionDBPath_XDGDataHomeCoversOpencode pins the XDG behaviour for
// the upstream brand. codefree-o keeps its state under ~/.codefree-o regardless
// of XDG_DATA_HOME, so an unrelated XDG value must not redirect it.
func TestResolveSessionDBPath_XDGDataHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	xdg := filepath.Join("/xdg", "data")
	t.Setenv("XDG_DATA_HOME", xdg)

	if got, want := resolveSessionDBPath(sessionDBSource{cmd: "opencode"}),
		filepath.Join(xdg, "opencode", "opencode.db"); got != want {
		t.Errorf("opencode XDG path = %q, want %q", got, want)
	}

	if got, want := resolveSessionDBPath(sessionDBSource{cmd: "codefree-o"}),
		filepath.Join(home, ".codefree-o", ".local", "share", "codefree.db"); got != want {
		t.Errorf("codefree-o must ignore unrelated XDG_DATA_HOME: got %q, want %q", got, want)
	}
}

// TestGlobalMemoryFile_BrandAware is the regression test for patch 3: the
// method used to return ~/.opencode/OPENCODE.md unconditionally, pointing
// codefree-o users at a file their CLI never reads.
func TestGlobalMemoryFile_BrandAware(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	t.Run("codefree-o prefers its config dir when the file exists", func(t *testing.T) {
		want := filepath.Join(home, ".codefree-o", ".config", "OPENCODE.md")
		writeFile(t, want, "# memory")

		a := &Agent{cmd: "codefree-o"}
		if got := a.GlobalMemoryFile(); got != want {
			t.Errorf("GlobalMemoryFile() = %q, want %q", got, want)
		}
	})

	t.Run("explicit option wins", func(t *testing.T) {
		explicit := filepath.Join(t.TempDir(), "CUSTOM.md")
		a := &Agent{cmd: "codefree-o", globalMemoryFile: explicit}
		if got := a.GlobalMemoryFile(); got != explicit {
			t.Errorf("GlobalMemoryFile() = %q, want explicit %q", got, explicit)
		}
	})

	t.Run("falls back to the opencode default", func(t *testing.T) {
		emptyHome := t.TempDir()
		t.Setenv("HOME", emptyHome)
		t.Setenv("USERPROFILE", emptyHome)

		a := &Agent{cmd: "opencode"}
		want := filepath.Join(emptyHome, ".opencode", "OPENCODE.md")
		if got := a.GlobalMemoryFile(); got != want {
			t.Errorf("GlobalMemoryFile() = %q, want fallback %q", got, want)
		}
	})
}

// TestIsCodefreeCmd documents brand detection.
func TestIsCodefreeCmd(t *testing.T) {
	codefree := []string{"codefree-o", "codefree-o.exe", "CodeFree-O", "/a/b/codefree-o"}
	opencode := []string{"opencode", "opencode.exe", "", "/usr/local/bin/opencode"}

	for _, c := range codefree {
		if !isCodefreeCmd(c) {
			t.Errorf("isCodefreeCmd(%q) = false, want true", c)
		}
	}
	for _, c := range opencode {
		if isCodefreeCmd(c) {
			t.Errorf("isCodefreeCmd(%q) = true, want false", c)
		}
	}
}

// TestCodefreeOAliasIsRegistered verifies `type = "codefree-o"` is accepted by
// the agent registry, not just `type = "opencode"`.
func TestCodefreeOAliasIsRegistered(t *testing.T) {
	// The registry rejects unknown names before the factory runs, so an error
	// mentioning "unknown agent" means the alias is missing. The factory itself
	// is allowed to fail on a machine without the CLI on PATH.
	_, err := core.CreateAgent("codefree-o", map[string]any{"work_dir": t.TempDir()})
	if err != nil && strings.Contains(err.Error(), "unknown agent") {
		t.Fatalf("codefree-o alias is not registered: %v", err)
	}

	// opencode must keep working: existing configs use it.
	if _, err := core.CreateAgent("opencode", map[string]any{"work_dir": t.TempDir()}); err != nil &&
		strings.Contains(err.Error(), "unknown agent") {
		t.Fatalf("opencode must remain registered: %v", err)
	}
}

func containsArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
