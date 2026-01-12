package store

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/throw-if-null/molecular/internal/api"
	"github.com/throw-if-null/molecular/internal/paths"
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
	// validate task id and build safe relative paths
	artifactsRoot, aerr := paths.RunsDir(r.TaskID)
	if aerr != nil {
		return nil, false, aerr
	}
	worktreePath, werr := paths.WorktreeDir(r.TaskID)
	if werr != nil {
		return nil, false, werr
	}

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

// IsTaskCancelled reports whether a task is currently cancelled.
// Returns ErrNotFound if the task can't be found.
func (s *Store) IsTaskCancelled(taskID string) (bool, error) {
	row := s.db.QueryRow(`SELECT status FROM tasks WHERE task_id = ?`, taskID)
	var status string
	if err := row.Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, ErrNotFound
		}
		return false, err
	}
	return status == "cancelled", nil
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
	// Retry on SQLITE_BUSY to avoid transient contention leaving tasks in running state.
	const maxRetries = 5
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		_, err := s.db.Exec(`UPDATE tasks SET phase = ?, status = ?, updated_at = ? WHERE task_id = ?`, phase, status, time.Now().UTC().Format(time.RFC3339Nano), taskID)
		if err == nil {
			return nil
		}
		lastErr = err
		if isSqliteBusy(err) {
			time.Sleep(time.Duration(10*(1<<i)) * time.Millisecond)
			continue
		}
		return err
	}
	return lastErr
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
				log.Printf("UpdateAttemptStatus: Begin failed (busy), retry %d: %v", i, err)
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

			newCount = 0
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

			// If we've incremented role-specific retries, and the new count meets
			// or exceeds the per-task budget, mark the task failed atomically in
			// this same transaction to avoid races where workers leave the task
			// stuck in 'running'. This keeps the status update consistent with
			// the retry counter increment.
			if status == "failed" {
				switch role {
				case "helium":
					var budget int
					if err := tx.QueryRow(`SELECT helium_budget FROM tasks WHERE task_id = ?`, taskID).Scan(&budget); err == nil {
						if newCount >= budget {
							if _, err := tx.Exec(`UPDATE tasks SET phase = ?, status = ?, updated_at = ? WHERE task_id = ?`, "helium", "failed", time.Now().UTC().Format(time.RFC3339Nano), taskID); err != nil {
								lastErr = err
								return
							}
						}
					}
				case "carbon":
					var budget int
					if err := tx.QueryRow(`SELECT carbon_budget FROM tasks WHERE task_id = ?`, taskID).Scan(&budget); err == nil {
						if newCount >= budget {
							if _, err := tx.Exec(`UPDATE tasks SET phase = ?, status = ?, updated_at = ? WHERE task_id = ?`, "carbon", "failed", time.Now().UTC().Format(time.RFC3339Nano), taskID); err != nil {
								lastErr = err
								return
							}
						}
					}
				default:
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

// ReconcileInFlightAttempts marks attempts that were left in-flight due to a
// silicon process crash as failed and clears any task current_attempt_id that
// referenced them. It is safe to run multiple times (idempotent) and performs
// best-effort artifact writes under the given repoRoot.
func (s *Store) ReconcileInFlightAttempts(repoRoot string) error {
	const crashMsg = "crash recovery: silicon restart"
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Collect attempts that are running
	rows, err := tx.Query(`SELECT id, task_id, role, status, COALESCE(finished_at, ''), COALESCE(error_summary, ''), artifacts_dir FROM attempts WHERE status = 'running'`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type attemptInfo struct {
		id           int64
		taskID       string
		role         string
		status       string
		finishedAt   string
		errorSummary string
		artifactsDir string
	}
	attempts := map[int64]attemptInfo{}
	for rows.Next() {
		var a attemptInfo
		if err := rows.Scan(&a.id, &a.taskID, &a.role, &a.status, &a.finishedAt, &a.errorSummary, &a.artifactsDir); err != nil {
			return err
		}
		attempts[a.id] = a
	}

	// Also collect attempts referenced by tasks.current_attempt_id
	trows, err := tx.Query(`SELECT task_id, current_attempt_id FROM tasks WHERE current_attempt_id IS NOT NULL`)
	if err != nil {
		return err
	}
	defer trows.Close()
	for trows.Next() {
		var taskID string
		var aid sql.NullInt64
		if err := trows.Scan(&taskID, &aid); err != nil {
			return err
		}
		if aid.Valid {
			id := aid.Int64
			if _, ok := attempts[id]; ok {
				continue
			}
			// load attempt row
			var a attemptInfo
			row := tx.QueryRow(`SELECT id, task_id, role, status, COALESCE(finished_at, ''), COALESCE(error_summary, ''), artifacts_dir FROM attempts WHERE id = ?`, id)
			if err := row.Scan(&a.id, &a.taskID, &a.role, &a.status, &a.finishedAt, &a.errorSummary, &a.artifactsDir); err != nil {
				// if missing attempt, just clear the task reference
				if errors.Is(err, sql.ErrNoRows) {
					if _, err2 := tx.Exec(`UPDATE tasks SET current_attempt_id = NULL WHERE current_attempt_id = ?`, id); err2 != nil {
						return err2
					}
					continue
				}
				return err
			}
			attempts[a.id] = a
		}
	}

	// For each collected attempt, if not already reconciled, mark failed and
	// clear task.current_attempt_id. Also write best-effort artifacts under
	// repoRoot.
	for _, a := range attempts {
		// idempotency: if attempt already finished and errorSummary contains crashMsg, skip
		if a.finishedAt != "" && strings.Contains(a.errorSummary, "crash recovery") {
			continue
		}

		// update attempt status -> failed and clear task current_attempt_id
		if _, err := tx.Exec(`UPDATE attempts SET status = ?, finished_at = ?, error_summary = ? WHERE id = ?`, "failed", time.Now().UTC().Format(time.RFC3339Nano), crashMsg, a.id); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE tasks SET current_attempt_id = NULL WHERE current_attempt_id = ?`, a.id); err != nil {
			return err
		}

		// Best-effort write artifacts
		if a.artifactsDir != "" && repoRoot != "" {
			fullDir := filepath.Join(repoRoot, a.artifactsDir)
			_ = os.MkdirAll(fullDir, 0o755)
			_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"failed","note":"crash recovery","role":"`+a.role+`"}`), 0o644)
			// append or create log.txt with crash note
			logPath := filepath.Join(fullDir, "log.txt")
			existing := []byte{}
			if b, err := os.ReadFile(logPath); err == nil {
				existing = b
			}
			prefix := []byte("crash recovery: silicon restart\n")
			_ = os.WriteFile(logPath, append(prefix, existing...), 0o644)
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}
