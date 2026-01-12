package store

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/throw-if-null/molecular/internal/api"
	"github.com/throw-if-null/molecular/internal/silicon"
)

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Init() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS tasks (
  task_id TEXT PRIMARY KEY,
  prompt TEXT NOT NULL,
  status TEXT NOT NULL,
  phase TEXT NOT NULL
);
`)
	return err
}

func (s *Store) CreateTaskOrGetExisting(r *api.CreateTaskRequest) (*api.Task, bool, error) {
	_, err := s.db.Exec(
		`INSERT INTO tasks (task_id, prompt, status, phase) VALUES (?, ?, ?, ?)`,
		r.TaskID,
		r.Prompt,
		"running",
		"lithium",
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
	row := s.db.QueryRow(`SELECT task_id, prompt, status, phase FROM tasks WHERE task_id = ?`, taskID)

	var task api.Task
	if err := row.Scan(&task.TaskID, &task.Prompt, &task.Status, &task.Phase); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, silicon.ErrNotFound
		}
		return nil, err
	}
	return &task, nil
}

func isUniqueConstraintError(err error) bool {
	// Covers modernc.org/sqlite and mattn/go-sqlite3 error strings.
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
