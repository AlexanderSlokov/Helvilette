package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite driver, same as k3s/kine

	"helvilette/pkg/types"
)

// schema is run once when the DB is opened. Uses IF NOT EXISTS so it is
// safe to re-run on an existing database.
const schema = `
CREATE TABLE IF NOT EXISTS nodes (
    node_id    TEXT PRIMARY KEY,
    labels     TEXT NOT NULL DEFAULT '{}',
    registered DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    status_json TEXT NOT NULL DEFAULT '{}',
    observed_at DATETIME
);

CREATE TABLE IF NOT EXISTS reports (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    node_id     TEXT NOT NULL,
    job_id      TEXT NOT NULL,
    status      TEXT NOT NULL,
    task_logs   TEXT NOT NULL DEFAULT '{}',
    reported_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    observed_at DATETIME
);
`

// SQLiteStore implements both NodeStore and ReportStore backed by a
// single SQLite database file. Following k3s convention, the DB path
// is typically {data-dir}/server/db/state.db.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens (or creates) a SQLite database at dbPath, runs
// the schema migration, and returns a store ready for use.
//
// Usage:
//
//	store, err := storage.NewSQLiteStore("/var/lib/othela/server/db/state.db")
//	defer store.Close()
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	if err := ensureDir(filepath.Dir(dbPath)); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// WAL mode + single writer is the safe default for embedded use.
	// _busy_timeout avoids SQLITE_BUSY on short contention windows.
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	// Migrations: ignore errors if columns already exist
	_, _ = db.Exec(`ALTER TABLE nodes ADD COLUMN status_json TEXT NOT NULL DEFAULT '{}'`)
	_, _ = db.Exec(`ALTER TABLE nodes ADD COLUMN observed_at DATETIME`)
	_, _ = db.Exec(`ALTER TABLE reports ADD COLUMN observed_at DATETIME`)

	return &SQLiteStore{db: db}, nil
}

// Close releases the database connection. Call during graceful shutdown.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// --- NodeStore implementation ---

// Register inserts a new node or updates its labels and last_seen timestamp.
func (s *SQLiteStore) Register(nodeID string, labels map[string]string) error {
	labelsJSON, err := json.Marshal(labels)
	if err != nil {
		return fmt.Errorf("marshal labels for node %q: %w", nodeID, err)
	}

	// UPSERT: insert or update on conflict
	_, err = s.db.Exec(`
		INSERT INTO nodes (node_id, labels) VALUES (?, ?)
		ON CONFLICT(node_id) DO UPDATE SET
			labels   = excluded.labels,
			last_seen = CURRENT_TIMESTAMP
	`, nodeID, string(labelsJSON))

	if err != nil {
		return fmt.Errorf("register node %q: %w", nodeID, err)
	}
	return nil
}

// GetLabels returns the labels for a registered node.
// Returns (nil, false) if the node is not found.
func (s *SQLiteStore) GetLabels(nodeID string) (map[string]string, bool) {
	var labelsJSON string
	err := s.db.QueryRow(`SELECT labels FROM nodes WHERE node_id = ?`, nodeID).Scan(&labelsJSON)
	if err != nil {
		return nil, false
	}

	var labels map[string]string
	if err := json.Unmarshal([]byte(labelsJSON), &labels); err != nil {
		return nil, false
	}
	return labels, true
}

// IsRegistered returns true if the node_id exists in the nodes table.
func (s *SQLiteStore) IsRegistered(nodeID string) bool {
	var exists int
	err := s.db.QueryRow(`SELECT 1 FROM nodes WHERE node_id = ? LIMIT 1`, nodeID).Scan(&exists)
	return err == nil
}

// UpdateStatus updates the node's current status and observation time.
func (s *SQLiteStore) UpdateStatus(nodeID string, status types.NodeStatus, observedAt time.Time) error {
	statusJSON, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("marshal status for node %q: %w", nodeID, err)
	}

	_, err = s.db.Exec(`
		UPDATE nodes SET
			status_json = ?,
			observed_at = ?
		WHERE node_id = ?
	`, string(statusJSON), observedAt, nodeID)

	if err != nil {
		return fmt.Errorf("update status for node %q: %w", nodeID, err)
	}
	return nil
}

// ListNodes returns all registered nodes.
func (s *SQLiteStore) ListNodes() ([]Node, error) {
	rows, err := s.db.Query(`SELECT node_id, labels, registered, last_seen, status_json, observed_at FROM nodes ORDER BY node_id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		var labelsJSON, statusJSON string
		var observedAt sql.NullTime
		if err := rows.Scan(&n.NodeID, &labelsJSON, &n.Registered, &n.LastSeen, &statusJSON, &observedAt); err != nil {
			return nil, fmt.Errorf("scan node row: %w", err)
		}
		json.Unmarshal([]byte(labelsJSON), &n.Labels)
		json.Unmarshal([]byte(statusJSON), &n.Status)
		if observedAt.Valid {
			n.ObservedAt = observedAt.Time
		}
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate node rows: %w", err)
	}
	if nodes == nil {
		nodes = []Node{}
	}
	return nodes, nil
}

// --- ReportStore implementation ---

// Save persists a single execution report.
func (s *SQLiteStore) Save(report types.Report) error {
	taskLogs := string(report.TaskLogs)
	if taskLogs == "" {
		taskLogs = "{}"
	}

	var observedAt interface{}
	if !report.ObservedAt.IsZero() {
		observedAt = report.ObservedAt
	}

	_, err := s.db.Exec(`
		INSERT INTO reports (node_id, job_id, status, task_logs, observed_at)
		VALUES (?, ?, ?, ?, ?)
	`, report.NodeID, report.JobID, report.Status, taskLogs, observedAt)

	if err != nil {
		return fmt.Errorf("save report for node %q job %q: %w", report.NodeID, report.JobID, err)
	}
	return nil
}

// List returns all stored reports ordered by insertion time (oldest first).
func (s *SQLiteStore) List() ([]types.Report, error) {
	rows, err := s.db.Query(`SELECT node_id, job_id, status, task_logs, observed_at FROM reports ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list reports: %w", err)
	}
	defer rows.Close()

	var reports []types.Report
	for rows.Next() {
		var r types.Report
		var taskLogs string
		var observedAt sql.NullTime
		if err := rows.Scan(&r.NodeID, &r.JobID, &r.Status, &taskLogs, &observedAt); err != nil {
			return nil, fmt.Errorf("scan report row: %w", err)
		}
		r.TaskLogs = json.RawMessage(taskLogs)
		if observedAt.Valid {
			r.ObservedAt = observedAt.Time
		}
		reports = append(reports, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate report rows: %w", err)
	}

	// Match memory adapter behavior: return empty slice, not nil
	if reports == nil {
		reports = []types.Report{}
	}
	return reports, nil
}

// ensureDir creates the directory (and parents) if it does not exist.
func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0750)
}
