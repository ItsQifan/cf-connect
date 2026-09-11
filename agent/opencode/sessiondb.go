package opencode

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "modernc.org/sqlite" // pure-Go driver: no external sqlite3 CLI required
)

// sessionDBSource carries the inputs needed to locate the agent CLI's local
// session database.
//
// This exists because the CLI binary is configurable (cmd = "opencode" or
// cmd = "codefree-o") and each brand keeps its own data directory:
//
//	opencode   -> ~/.local/share/opencode/opencode.db
//	codefree-o -> ~/.codefree-o/.local/share/codefree.db
//
// Both databases use the same schema:
//
//	session(id, project_id, ..., title, ...)
//	message(id, session_id, time_created, time_updated, data)
//
// so only the path resolution differs between them.
type sessionDBSource struct {
	cmd     string // CLI binary name or path; used to infer the brand
	dataDir string // explicit override (agent option data_dir)
	dbFile  string // explicit override (agent option db_file)
}

// sqliteDriverName is provided by modernc.org/sqlite (pure Go, CGO-free).
const sqliteDriverName = "sqlite"

// codefreeBrandTokens are matched case-insensitively against the CLI name to
// decide which brand's data directory to read.
var codefreeBrandTokens = []string{"codefree", "code-free", "code_free"}

// resolveSessionDBPath returns the absolute path of the session database, or
// "" when it cannot be determined.
//
// Precedence:
//  1. explicit db_file + data_dir (db_file may be absolute on its own)
//  2. explicit data_dir + a brand-appropriate default filename
//  3. $XDG_DATA_HOME + brand subdirectory
//  4. brand-derived default under the user's home directory
func resolveSessionDBPath(src sessionDBSource) string {
	// 1. Fully explicit.
	if f := strings.TrimSpace(src.dbFile); f != "" {
		if filepath.IsAbs(f) {
			return f
		}
		if d := strings.TrimSpace(src.dataDir); d != "" {
			return filepath.Join(d, f)
		}
		// A bare filename with no directory: fall through to the brand-derived
		// directory so the user does not have to repeat the path.
		if dir := defaultDataDirForCmd(src.cmd); dir != "" {
			return filepath.Join(dir, f)
		}
		return ""
	}

	// 2. Explicit data directory + brand-appropriate filename.
	if d := strings.TrimSpace(src.dataDir); d != "" {
		return filepath.Join(d, defaultDBFileForCmd(src.cmd))
	}

	// 3./4. Brand-derived location.
	return filepath.Join(defaultDataDirForCmd(src.cmd), defaultDBFileForCmd(src.cmd))
}

// defaultDataDirForCmd returns the brand-specific data directory for the CLI,
// or "" when the home directory is unavailable.
func defaultDataDirForCmd(cmd string) string {
	if isCodefreeCmd(cmd) {
		// codefree-o reports this location via `codefree-o debug paths`.
		if xdg := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); xdg != "" {
			home := homeDir()
			if home != "" && isUnder(xdg, home) {
				// codefree-o deliberately keeps XDG_DATA_HOME-insensitive
				// paths under ~/.codefree-o; only honour XDG when it points
				// into the codefree-o tree itself.
				return xdg
			}
		}
		home := homeDir()
		if home == "" {
			return ""
		}
		return filepath.Join(home, ".codefree-o", ".local", "share")
	}

	if xdg := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); xdg != "" {
		return filepath.Join(xdg, "opencode")
	}
	home := homeDir()
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".local", "share", "opencode")
}

// defaultDBFileForCmd returns the session database filename for the CLI brand.
func defaultDBFileForCmd(cmd string) string {
	if isCodefreeCmd(cmd) {
		return "codefree.db"
	}
	return "opencode.db"
}

// isCodefreeCmd reports whether the configured CLI is a codefree-o build.
func isCodefreeCmd(cmd string) bool {
	base := strings.ToLower(filepath.Base(strings.TrimSpace(cmd)))
	for _, tok := range codefreeBrandTokens {
		if strings.Contains(base, tok) {
			return true
		}
	}
	return false
}

func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

// isUnder reports whether path is inside root (used to reject unrelated
// XDG_DATA_HOME directories for the codefree-o brand).
func isUnder(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// -- sqlite access (pure Go, in-process) --

var (
	sessionDBCacheMu sync.Mutex
	sessionDBCache   = map[string]*sql.DB{}
)

// openSessionDB opens (and caches) the session database read-only.
//
// Returns (nil, "") when the database is absent or cannot be opened: session
// titles and message counts are a non-critical enrichment, so every failure
// degrades to "unavailable" instead of surfacing an error to the user.
func openSessionDB(src sessionDBSource) (*sql.DB, string) {
	path := resolveSessionDBPath(src)
	if path == "" {
		return nil, ""
	}
	if _, err := os.Stat(path); err != nil {
		slog.Debug("opencode: session db not found, titles/counts unavailable", "path", path)
		return nil, path
	}

	sessionDBCacheMu.Lock()
	defer sessionDBCacheMu.Unlock()
	if db, ok := sessionDBCache[path]; ok {
		return db, path
	}

	// mode=ro keeps the agent's own writer authoritative and avoids creating a
	// WAL sidecar in a directory we only meant to read.
	dsn := fmt.Sprintf("file:%s?mode=ro&_pragma=busy_timeout(3000)", filepath.ToSlash(path))
	db, err := sql.Open(sqliteDriverName, dsn)
	if err != nil {
		slog.Warn("opencode: open session db failed, titles/counts unavailable", "path", path, "err", err)
		return nil, path
	}
	db.SetMaxOpenConns(1)
	sessionDBCache[path] = db
	return db, path
}

// querySessionMessageCounts returns message counts keyed by session ID.
// An empty map means "unavailable", never "zero messages".
func querySessionMessageCounts(src sessionDBSource) map[string]int {
	db, path := openSessionDB(src)
	if db == nil {
		return nil
	}

	rows, err := db.Query("SELECT session_id, COUNT(*) FROM message GROUP BY session_id")
	if err != nil {
		slog.Warn("opencode: query message counts failed", "db_path", path, "err", err)
		return nil
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			slog.Warn("opencode: scan message count failed", "db_path", path, "err", err)
			return nil
		}
		counts[id] = n
	}
	if err := rows.Err(); err != nil {
		slog.Warn("opencode: iterate message counts failed", "db_path", path, "err", err)
		return nil
	}
	return counts
}

// querySessionTitle returns the stored title for a session, or "" when the
// database or the row is unavailable.
func querySessionTitle(src sessionDBSource, sessionID string) string {
	if strings.TrimSpace(sessionID) == "" {
		return ""
	}
	db, path := openSessionDB(src)
	if db == nil {
		return ""
	}

	var title string
	err := db.QueryRow("SELECT title FROM session WHERE id = ? LIMIT 1", sessionID).Scan(&title)
	if err != nil {
		if err != sql.ErrNoRows {
			slog.Warn("opencode: query session title failed", "db_path", path, "session_id", sessionID, "err", err)
		}
		return ""
	}
	return strings.TrimSpace(title)
}
