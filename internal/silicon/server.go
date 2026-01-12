package silicon

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/throw-if-null/molecular/internal/api"
	"github.com/throw-if-null/molecular/internal/paths"
	"github.com/throw-if-null/molecular/internal/store"
)

type Server struct {
	store        Store
	carbonBudget int
	heliumBudget int
	reviewBudget int
}

// maximum number of bytes we'll allow reading for a task log
const maxLogBytes = 5 << 20 // 5 MiB

type Store interface {
	CreateTaskOrGetExisting(r *api.CreateTaskRequest) (*api.Task, bool, error)
	CreateTaskWithBudgets(r *api.CreateTaskRequest, carbonBudget, heliumBudget, reviewBudget int) (*api.Task, bool, error)
	GetTask(taskID string) (*api.Task, error)
	ListTasks(limit int) ([]*api.Task, error)
	CancelTask(taskID string) (bool, error)
	IsTaskCancelled(taskID string) (bool, error)
	UpdateTaskPhaseAndStatus(taskID, phase, status string) error

	// Attempt management for workers
	CreateAttempt(taskID, role string) (int64, string, int64, string, error)
	UpdateAttemptStatus(attemptID int64, status, errorSummary string) (int, error)

	// Attempt queries for logs endpoint
	GetAttempt(taskID string, attemptID int64) (*api.Attempt, error)
	GetLatestAttempt(taskID string) (*api.Attempt, error)
	GetLatestAttemptByRole(taskID string, role string) (*api.Attempt, error)

	// Retry counters
	IncrementCarbonRetries(taskID string) (int, error)
	IncrementHeliumRetries(taskID string) (int, error)
	IncrementReviewRetries(taskID string) (int, error)
}

func NewServer(store Store, carbonBudget, heliumBudget, reviewBudget int) *Server {
	return &Server{store: store, carbonBudget: carbonBudget, heliumBudget: heliumBudget, reviewBudget: reviewBudget}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/tasks", s.handleCreateTask)
	mux.HandleFunc("GET /v1/tasks/{task_id}", s.handleGetTask)
	mux.HandleFunc("GET /v1/tasks", s.handleListTasks)
	mux.HandleFunc("POST /v1/tasks/{task_id}/cancel", s.handleCancelTask)
	mux.HandleFunc("GET /v1/tasks/{task_id}/logs", s.handleGetTaskLogs)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var req api.CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if req.TaskID == "" || req.Prompt == "" {
		http.Error(w, "task_id and prompt are required", http.StatusBadRequest)
		return
	}

	if err := paths.ValidateTaskID(req.TaskID); err != nil {
		http.Error(w, "invalid task_id", http.StatusBadRequest)
		return
	}

	task, existed, err := s.store.CreateTaskWithBudgets(&req, s.carbonBudget, s.heliumBudget, s.reviewBudget)
	if err != nil {
		http.Error(w, "failed to create task", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if existed {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusAccepted)
	}
	_ = json.NewEncoder(w).Encode(task)
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("task_id")
	if taskID == "" {
		http.Error(w, "missing task_id", http.StatusBadRequest)
		return
	}
	if err := paths.ValidateTaskID(taskID); err != nil {
		http.Error(w, "invalid task_id", http.StatusBadRequest)
		return
	}
	if err := paths.ValidateTaskID(taskID); err != nil {
		http.Error(w, "invalid task_id", http.StatusBadRequest)
		return
	}

	task, err := s.store.GetTask(taskID)
	if isNotFound(err) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "failed to read task", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(task)
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := 0
	if v := q.Get("limit"); v != "" {
		var x int
		_, _ = fmt.Sscanf(v, "%d", &x)
		limit = x
	}

	tasks, err := s.store.ListTasks(limit)
	if err != nil {
		http.Error(w, "failed to list tasks", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tasks)
}

func (s *Server) handleCancelTask(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("task_id")
	if taskID == "" {
		http.Error(w, "missing task_id", http.StatusBadRequest)
		return
	}
	changed, err := s.store.CancelTask(taskID)
	if isNotFound(err) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "failed to cancel task", http.StatusInternalServerError)
		return
	}
	// signal any in-memory running attempt for this task so workers can stop
	_ = CancelInMemory(taskID)
	if changed {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("cancelled"))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("no-op"))
}

func (s *Server) handleGetTaskLogs(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("task_id")
	if taskID == "" {
		http.Error(w, "missing task_id", http.StatusBadRequest)
		return
	}
	if err := paths.ValidateTaskID(taskID); err != nil {
		http.Error(w, "invalid task_id", http.StatusBadRequest)
		return
	}

	// validate task exists
	if _, err := s.store.GetTask(taskID); isNotFound(err) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "failed to read task", http.StatusInternalServerError)
		return
	}

	q := r.URL.Query()
	role := q.Get("role")
	attemptIDStr := q.Get("attempt_id")
	tailStr := q.Get("tail")

	var attempt *api.Attempt
	var err error

	if attemptIDStr != "" {
		id, perr := strconv.ParseInt(attemptIDStr, 10, 64)
		if perr != nil || id <= 0 {
			http.Error(w, "invalid attempt_id", http.StatusBadRequest)
			return
		}
		attempt, err = s.store.GetAttempt(taskID, id)
	} else if role != "" {
		if !isValidRole(role) {
			http.Error(w, "invalid role", http.StatusBadRequest)
			return
		}
		attempt, err = s.store.GetLatestAttemptByRole(taskID, role)
	} else {
		attempt, err = s.store.GetLatestAttempt(taskID)
	}

	if isNotFound(err) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "failed to read attempts", http.StatusInternalServerError)
		return
	}

	logPath := filepath.Join(attempt.ArtifactsDir, "log.txt")
	// hard cap: avoid reading extremely large logs into memory
	if fi, serr := os.Stat(logPath); serr == nil {
		if fi.Size() > maxLogBytes {
			http.Error(w, "log too large", http.StatusRequestEntityTooLarge)
			return
		}
	}

	b, err := os.ReadFile(logPath)
	if errors.Is(err, os.ErrNotExist) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "failed to read log", http.StatusInternalServerError)
		return
	}

	logText := string(b)
	if tailStr != "" {
		n, perr := strconv.Atoi(tailStr)
		if perr != nil || n < 0 {
			http.Error(w, "invalid tail", http.StatusBadRequest)
			return
		}
		logText = tailLines(logText, n)
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Molecular-Attempt-Id", strconv.FormatInt(attempt.ID, 10))
	w.Header().Set("X-Molecular-Role", attempt.Role)
	_, _ = w.Write([]byte(logText))
}

func isValidRole(role string) bool {
	switch role {
	case "lithium", "carbon", "helium", "chlorine":
		return true
	default:
		return false
	}
}

func isNotFound(err error) bool {
	return errors.Is(err, ErrNotFound) || errors.Is(err, os.ErrNotExist) || errors.Is(err, store.ErrNotFound)
}

func tailLines(s string, n int) string {
	if n == 0 {
		return ""
	}
	// Normalize to \n lines; treat Windows newlines too.
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	// If file ends with newline, Split will include a trailing empty.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if n >= len(lines) {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
