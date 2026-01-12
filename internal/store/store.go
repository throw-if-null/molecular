package store

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/throw-if-null/molecular/internal/api"
)

type Store struct {
	db *sql.DB
}

var ErrNotFound = errors.New("not found")

var ErrInProgress = errors.New("attempt in progress")

func isNotFound(err error) bool {
	return errors.Is(err, ErrNotFound) || errors.Is(err, sql.ErrNoRows)
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
	// Preserve original behavior by delegating to CreateTaskWithBudgets
	// using the DB/schema defaults (3,3,2) so existing callers are unaffected.
	return s.CreateTaskWithBudgets(r, 3, 3, 2)
}

// CreateTaskWithBudgets creates a task row setting per-task budgets. Returns
// the created task (or existing) and a boolean indicating whether it already
// existed. This allows callers (e.g. server) to pass configured budgets at
// task creation time.
func (s *Store) CreateTaskWithBudgets(r *api.CreateTaskRequest, carbonBudget, heliumBudget, reviewBudget int) (*api.Task, bool, error) {
	createdAt := time.Now().UTC().Format(time.RFC3339Nano)
	updatedAt := createdAt
	artifactsRoot := filepath.ToSlash(filepath.Join(".molecular", "runs", r.TaskID))
	worktreePath := filepath.ToSlash(filepath.Join(".molecular", "worktrees", r.TaskID))

	_, err := s.db.Exec(
		`INSERT INTO tasks (task_id, prompt, status, phase, created_at, updated_at, carbon_budget, helium_budget, review_budget, artifacts_root, worktree_path) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.TaskID,
		r.Prompt,
		"running",
		"lithium",
		createdAt,
		updatedAt,
		carbonBudget,
		heliumBudget,
		reviewBudget,
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
			return nil, ErrNotFound
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
			return false, ErrNotFound
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

	// ensure task exists
	var exists int
	if err := tx.QueryRow(`SELECT 1 FROM tasks WHERE task_id = ?`, taskID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, "", 0, "", ErrNotFound
		}
		return 0, "", 0, "", err
	}

	// compute next attempt_num
	var maxNum sql.NullInt64
	if err := tx.QueryRow(`SELECT MAX(attempt_num) FROM attempts WHERE task_id = ? AND role = ?`, taskID, role).Scan(&maxNum); err != nil {
		return 0, "", 0, "", err
	}
	next := int64(1)
	if maxNum.Valid {
		next = maxNum.Int64 + 1
	}

	// insert attempt with empty artifacts_dir
	startedAt := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := tx.Exec(`INSERT INTO attempts (task_id, role, attempt_num, status, started_at, artifacts_dir) VALUES (?, ?, ?, ?, ?, ?)`, taskID, role, next, "running", startedAt, "")
	if err != nil {
		return 0, "", 0, "", err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, "", 0, "", err
	}

	// set artifacts_dir
	artifactsDir := filepath.ToSlash(filepath.Join(".molecular", "runs", taskID, "attempts", fmt.Sprintf("%d", id)))
	if _, err := tx.Exec(`UPDATE attempts SET artifacts_dir = ? WHERE id = ?`, artifactsDir, id); err != nil {
		return 0, "", 0, "", err
	}

	// try to claim the task slot; if someone else claimed, abort and rollback
	res2, err := tx.Exec(`UPDATE tasks SET current_attempt_id = ? WHERE task_id = ? AND current_attempt_id IS NULL`, id, taskID)
	if err != nil {
		return 0, "", 0, "", err
	}
	n, err := res2.RowsAffected()
	if err != nil {
		return 0, "", 0, "", err
	}
	if n == 0 {
		// another in-flight attempt exists; rollback so our inserted attempt is not persisted
		_ = tx.Rollback()
		return 0, "", 0, "", ErrInProgress
	}

	if err := tx.Commit(); err != nil {
		return 0, "", 0, "", err
	}
	return id, artifactsDir, next, startedAt, nil
}
func (s *Store) UpdateAttemptStatus(attemptID int64, status, errorSummary string) (int, error) {
	// Some environments experience transient SQLITE_BUSY errors when
	// concurrent writers contend. Retry the whole UpdateAttemptStatus
	// transaction a few times with a small backoff to avoid leaving the
	// attempt stuck in `running` with `current_attempt_id` set.
	const maxRetries = 5
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		tx, err := s.db.Begin()
		if err != nil {
			lastErr = err
			// if begin failed due to busy, backoff and retry
			if isSqliteBusy(err) {
				time.Sleep(time.Duration(10*(1<<i)) * time.Millisecond)
				continue
			}
			return 0, err
		}
		var newCount int
		func() {
			defer func() { _ = tx.Rollback() }()

			// read attempt to get task_id and role
			var taskID string
			var role string
			if err := tx.QueryRow(`SELECT task_id, role FROM attempts WHERE id = ?`, attemptID).Scan(&taskID, &role); err != nil {
				if isNotFound(err) {
					lastErr = ErrNotFound
					return
				}
				lastErr = err
				return
			}

			newCount := 0
			// if this attempt failed, increment role-specific retry counters
			if status == "failed" {
				switch role {
				case "helium":
					if _, err := tx.Exec(`UPDATE tasks SET helium_retries = helium_retries + 1, updated_at = ? WHERE task_id = ?`, time.Now().UTC().Format(time.RFC3339Nano), taskID); err != nil {
						lastErr = err
						return
					}
					if err := tx.QueryRow(`SELECT helium_retries FROM tasks WHERE task_id = ?`, taskID).Scan(&newCount); err != nil {
						lastErr = err
						return
					}
				case "carbon":
					if _, err := tx.Exec(`UPDATE tasks SET carbon_retries = carbon_retries + 1, updated_at = ? WHERE task_id = ?`, time.Now().UTC().Format(time.RFC3339Nano), taskID); err != nil {
						lastErr = err
						return
					}
					if err := tx.QueryRow(`SELECT carbon_retries FROM tasks WHERE task_id = ?`, taskID).Scan(&newCount); err != nil {
						lastErr = err
						return
					}
				default:
					// do not touch review_retries here; other flows handle it
				}
			}

			if _, err := tx.Exec(`UPDATE attempts SET status = ?, finished_at = ?, error_summary = ? WHERE id = ?`, status, time.Now().UTC().Format(time.RFC3339Nano), errorSummary, attemptID); err != nil {
				lastErr = err
				return
			}

			// clear current_attempt_id on tasks if it matches this attempt
			if _, err := tx.Exec(`UPDATE tasks SET current_attempt_id = NULL WHERE current_attempt_id = ?`, attemptID); err != nil {
				lastErr = err
				return
			}

			if err := tx.Commit(); err != nil {
				lastErr = err
				return
			}
			// success
			lastErr = nil
		}()
		if lastErr == nil {
			return newCount, nil
		}
		// if lastErr indicates SQLITE_BUSY/locked, retry with backoff
		if isSqliteBusy(lastErr) {
			time.Sleep(time.Duration(10*(1<<i)) * time.Millisecond)
			continue
		}
		// non-retriable error
		return 0, lastErr
	}
	// all retries exhausted
	if lastErr != nil {
		// best-effort: try to mark the attempt finished (best-effort) but return the error
		return 0, lastErr
	}
	return 0, nil
}

// isSqliteBusy reports whether err represents a busy/locked sqlite condition.
func isSqliteBusy(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return msg == "database is locked" || msg == "database is busy" || contains(msg, "SQLITE_BUSY")
}

func (s *Store) GetAttempt(taskID string, attemptID int64) (*api.Attempt, error) {
	row := s.db.QueryRow(`
 	SELECT id, task_id, role, attempt_num, status, started_at, COALESCE(finished_at, ''), artifacts_dir, COALESCE(error_summary, '')
 	FROM attempts
 	WHERE task_id = ? AND id = ?
 	`, taskID, attemptID)
	var a api.Attempt

	if err := row.Scan(&a.ID, &a.TaskID, &a.Role, &a.AttemptNum, &a.Status, &a.StartedAt, &a.FinishedAt, &a.ArtifactsDir, &a.ErrorSummary); err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

func (s *Store) GetLatestAttempt(taskID string) (*api.Attempt, error) {
	row := s.db.QueryRow(`
 	SELECT id, task_id, role, attempt_num, status, started_at, COALESCE(finished_at, ''), artifacts_dir, COALESCE(error_summary, '')
 	FROM attempts
 	WHERE task_id = ?
 	ORDER BY id DESC
 	LIMIT 1
 	`, taskID)
	var a api.Attempt

	if err := row.Scan(&a.ID, &a.TaskID, &a.Role, &a.AttemptNum, &a.Status, &a.StartedAt, &a.FinishedAt, &a.ArtifactsDir, &a.ErrorSummary); err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

func (s *Store) GetLatestAttemptByRole(taskID string, role string) (*api.Attempt, error) {
	row := s.db.QueryRow(`
 	SELECT id, task_id, role, attempt_num, status, started_at, COALESCE(finished_at, ''), artifacts_dir, COALESCE(error_summary, '')
 	FROM attempts
 	WHERE task_id = ? AND role = ?
 	ORDER BY id DESC
 	LIMIT 1
 	`, taskID, role)
	var a api.Attempt

	if err := row.Scan(&a.ID, &a.TaskID, &a.Role, &a.AttemptNum, &a.Status, &a.StartedAt, &a.FinishedAt, &a.ArtifactsDir, &a.ErrorSummary); err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &a, nil
}

// IncrementCarbonRetries atomically increments carbon_retries and returns the new value.
func (s *Store) IncrementCarbonRetries(taskID string) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`UPDATE tasks SET carbon_retries = carbon_retries + 1, updated_at = ? WHERE task_id = ?`, time.Now().UTC().Format(time.RFC3339Nano), taskID); err != nil {
		return 0, err
	}
	var v int
	if err := tx.QueryRow(`SELECT carbon_retries FROM tasks WHERE task_id = ?`, taskID).Scan(&v); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return v, nil
}

// IncrementHeliumRetries atomically increments helium_retries and returns the new value.
func (s *Store) IncrementHeliumRetries(taskID string) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`UPDATE tasks SET helium_retries = helium_retries + 1, updated_at = ? WHERE task_id = ?`, time.Now().UTC().Format(time.RFC3339Nano), taskID); err != nil {
		return 0, err
	}
	var v int
	if err := tx.QueryRow(`SELECT helium_retries FROM tasks WHERE task_id = ?`, taskID).Scan(&v); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return v, nil
}

// IncrementReviewRetries atomically increments review_retries and returns the new value.
func (s *Store) IncrementReviewRetries(taskID string) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`UPDATE tasks SET review_retries = review_retries + 1, updated_at = ? WHERE task_id = ?`, time.Now().UTC().Format(time.RFC3339Nano), taskID); err != nil {
		return 0, err
	}
	var v int
	if err := tx.QueryRow(`SELECT review_retries FROM tasks WHERE task_id = ?`, taskID).Scan(&v); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return v, nil
}
