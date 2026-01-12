package store

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/throw-if-null/molecular/internal/api"
	"github.com/throw-if-null/molecular/internal/silicon"
)

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// Init runs migrations using PRAGMA user_version.
func (s *Store) Init() error {
	// Check current version
	var ver int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&ver); err != nil {
		return err
	}
	if ver >= 1 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// v1 schema
	if _, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS tasks (
  task_id TEXT PRIMARY KEY,
  prompt TEXT NOT NULL,
  status TEXT NOT NULL,
  phase TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  carbon_budget INTEGER NOT NULL DEFAULT 3,
  helium_budget INTEGER NOT NULL DEFAULT 3,
  review_budget INTEGER NOT NULL DEFAULT 2,
  carbon_retries INTEGER NOT NULL DEFAULT 0,
  helium_retries INTEGER NOT NULL DEFAULT 0,
  review_retries INTEGER NOT NULL DEFAULT 0,
  artifacts_root TEXT NOT NULL,
  worktree_path TEXT NOT NULL,
  current_attempt_id INTEGER
);
`); err != nil {
		return err
	}

	if _, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS attempts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  task_id TEXT NOT NULL REFERENCES tasks(task_id) ON DELETE CASCADE,
  role TEXT NOT NULL,
  attempt_num INTEGER NOT NULL,
  status TEXT NOT NULL,
  started_at TEXT NOT NULL,
  finished_at TEXT,
  error_summary TEXT,
  artifacts_dir TEXT NOT NULL
);
`); err != nil {
		return err
	}

	if _, err := tx.Exec(`PRAGMA user_version = 1`); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *Store) CreateTaskOrGetExisting(r *api.CreateTaskRequest) (*api.Task, bool, error) {
	createdAt := time.Now().UTC().Format(time.RFC3339Nano)
	updatedAt := createdAt
	artifactsRoot := filepath.ToSlash(filepath.Join(".molecular", "runs", r.TaskID))
	worktreePath := filepath.ToSlash(filepath.Join(".molecular", "worktrees", r.TaskID))

	_, err := s.db.Exec(
		`INSERT INTO tasks (task_id, prompt, status, phase, created_at, updated_at, artifacts_root, worktree_path) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		r.TaskID,
		r.Prompt,
		"running",
		"lithium",
		createdAt,
		updatedAt,
		artifactsRoot,
		worktreePath,
	)
	if err == nil {
		t, err := s.GetTask(r.TaskID)
		return t, false, err
	}

	if !isUniqueConstraintError(err) {
		return nil, false, err
	}

	t, getErr := s.GetTask(r.TaskID)
	return t, true, getErr
}

func (s *Store) GetTask(taskID string) (*api.Task, error) {
	row := s.db.QueryRow(`SELECT task_id, prompt, status, phase, created_at, updated_at, carbon_budget, helium_budget, review_budget, artifacts_root, worktree_path, current_attempt_id FROM tasks WHERE task_id = ?`, taskID)

	var task api.Task
	var currentAttempt sql.NullInt64
	if err := row.Scan(&task.TaskID, &task.Prompt, &task.Status, &task.Phase, &task.CreatedAt, &task.UpdatedAt, &task.CarbonBudget, &task.HeliumBudget, &task.ReviewBudget, &task.ArtifactsRoot, &task.WorktreePath, &currentAttempt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, silicon.ErrNotFound
		}
		return nil, err
	}
	if currentAttempt.Valid {
		tid := currentAttempt.Int64
		task.CurrentAttemptID = &tid
	}
	return &task, nil
}

// ListTasks returns tasks ordered newest first. If limit <= 0, return all.
func (s *Store) ListTasks(limit int) ([]*api.Task, error) {
	q := `SELECT task_id, prompt, status, phase, created_at, updated_at, carbon_budget, helium_budget, review_budget, artifacts_root, worktree_path, current_attempt_id FROM tasks ORDER BY created_at DESC`
	var rows *sql.Rows
	var err error
	if limit > 0 {
		q = q + ` LIMIT ?`
		rows, err = s.db.Query(q, limit)
	} else {
		rows, err = s.db.Query(q)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*api.Task
	for rows.Next() {
		var task api.Task
		var currentAttempt sql.NullInt64
		if err := rows.Scan(&task.TaskID, &task.Prompt, &task.Status, &task.Phase, &task.CreatedAt, &task.UpdatedAt, &task.CarbonBudget, &task.HeliumBudget, &task.ReviewBudget, &task.ArtifactsRoot, &task.WorktreePath, &currentAttempt); err != nil {
			return nil, err
		}
		if currentAttempt.Valid {
			tid := currentAttempt.Int64
			task.CurrentAttemptID = &tid
		}
		out = append(out, &task)
	}
	return out, nil
}

// CancelTask sets status to 'cancelled' if task exists and not already terminal.
// Returns true if the status was changed.
func (s *Store) CancelTask(taskID string) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRow(`SELECT status FROM tasks WHERE task_id = ?`, taskID)
	var status string
	if err := row.Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, silicon.ErrNotFound
		}
		return false, err
	}
	if status == "cancelled" || status == "failed" || status == "completed" {
		return false, tx.Commit()
	}

	if _, err := tx.Exec(`UPDATE tasks SET status = ?, updated_at = ? WHERE task_id = ?`, "cancelled", time.Now().UTC().Format(time.RFC3339Nano), taskID); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// UpdateTaskPhaseAndStatus updates the task's phase and status and sets updated_at.
func (s *Store) UpdateTaskPhaseAndStatus(taskID, phase, status string) error {
	_, err := s.db.Exec(`UPDATE tasks SET phase = ?, status = ?, updated_at = ? WHERE task_id = ?`, phase, status, time.Now().UTC().Format(time.RFC3339Nano), taskID)
	return err
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return msg == "UNIQUE constraint failed: tasks.task_id" ||
		msg == "constraint failed: UNIQUE constraint failed: tasks.task_id" ||
		(msg != "" && contains(msg, "UNIQUE constraint failed"))
}

func contains(s, substr string) bool {
	if substr == "" {
		return true
	}
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func (s *Store) String() string {
	return fmt.Sprintf("store(%p)", s)
}

// CreateAttempt creates a new attempt row for the given task and role.
// Returns the inserted attempt id, artifacts_dir (relative path), attempt_num and started_at.
func (s *Store) CreateAttempt(taskID, role string) (int64, string, int64, string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, "", 0, "", err
	}
	defer func() { _ = tx.Rollback() }()

	// compute next attempt_num
	var maxNum sql.NullInt64
	if err := tx.QueryRow(`SELECT MAX(attempt_num) FROM attempts WHERE task_id = ? AND role = ?`, taskID, role).Scan(&maxNum); err != nil {
		return 0, "", 0, "", err
	}
	next := int64(1)
	if maxNum.Valid {
		next = maxNum.Int64 + 1
	}

	// insert with empty artifacts_dir first
	startedAt := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := tx.Exec(`INSERT INTO attempts (task_id, role, attempt_num, status, started_at, artifacts_dir) VALUES (?, ?, ?, ?, ?, ?)`, taskID, role, next, "running", startedAt, "")
	if err != nil {
		return 0, "", 0, "", err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, "", 0, "", err
	}

	// new layout: attempts are stored under .molecular/runs/<task_id>/attempts/<attempt_id>
	artifactsDir := filepath.ToSlash(filepath.Join(".molecular", "runs", taskID, "attempts", fmt.Sprintf("%d", id)))

	if _, err := tx.Exec(`UPDATE attempts SET artifacts_dir = ? WHERE id = ?`, artifactsDir, id); err != nil {
		return 0, "", 0, "", err
	}

	if _, err := tx.Exec(`UPDATE tasks SET current_attempt_id = ? WHERE task_id = ?`, id, taskID); err != nil {
		return 0, "", 0, "", err
	}

	if err := tx.Commit(); err != nil {
		return 0, "", 0, "", err
	}
	return id, artifactsDir, next, startedAt, nil
}
