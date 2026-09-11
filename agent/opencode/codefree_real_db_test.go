//go:build codefree_integration

// Integration checks that read a real agent session database. These are opt-in
// (build tag) because they depend on a local CLI installation:
//
//	go test -tags codefree_integration ./agent/opencode/ -run TestRealCodefreeDB -v
//
// They exist to prove that the in-process sqlite reader returns real titles and
// message counts for a codefree-o data directory, which the unit tests can only
// assert about path resolution.
package opencode

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRealCodefreeDB_ReadsTitlesAndMessageCounts(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir: %v", err)
	}

	dbPath := filepath.Join(home, ".codefree-o", ".local", "share", "codefree.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Skipf("codefree.db not present at %s: %v", dbPath, err)
	}

	src := sessionDBSource{cmd: "codefree-o"}

	if got, want := resolveSessionDBPath(src), dbPath; got != want {
		t.Fatalf("resolveSessionDBPath() = %q, want %q", got, want)
	}

	counts := querySessionMessageCounts(src)
	if counts == nil {
		t.Fatal("querySessionMessageCounts() returned nil against a real database")
	}
	t.Logf("message counts for %d sessions", len(counts))

	// Pull one session id out of the database and read its title back.
	db, _ := openSessionDB(src)
	if db == nil {
		t.Fatal("openSessionDB() returned nil against a real database")
	}
	var id string
	if err := db.QueryRow("SELECT id FROM session LIMIT 1").Scan(&id); err != nil {
		t.Skipf("no session rows yet: %v", err)
	}
	title := querySessionTitle(src, id)
	if title == "" {
		t.Errorf("querySessionTitle(%q) = \"\", want a stored title", id)
	}
	t.Logf("session %s title = %q, messages = %d", id, title, counts[id])
}
