package silicon

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/throw-if-null/molecular/internal/api"
)

type Server struct {
	store Store
}

type Store interface {
	CreateTaskOrGetExisting(r *api.CreateTaskRequest) (*api.Task, bool, error)
	GetTask(taskID string) (*api.Task, error)
	ListTasks(limit int) ([]*api.Task, error)
	CancelTask(taskID string) (bool, error)
}

func NewServer(store Store) *Server {
	return &Server{store: store}
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

	task, existed, err := s.store.CreateTaskOrGetExisting(&req)
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

	task, err := s.store.GetTask(taskID)
	if errors.Is(err, ErrNotFound) {
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
	if errors.Is(err, ErrNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "failed to cancel task", http.StatusInternalServerError)
		return
	}
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
	task, err := s.store.GetTask(taskID)
	if errors.Is(err, ErrNotFound) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "failed to read task", http.StatusInternalServerError)
		return
	}

	resp := map[string]interface{}{
		"artifacts_root": task.ArtifactsRoot,
		"paths":          []string{task.ArtifactsRoot + "/log.txt", task.ArtifactsRoot + "/attempt-1/log.txt"},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
