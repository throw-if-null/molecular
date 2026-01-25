package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/throw-if-null/molecular/internal/api"
	"github.com/throw-if-null/molecular/internal/task"
	"github.com/throw-if-null/molecular/internal/telemetry"
	"github.com/throw-if-null/molecular/internal/version"
)

// allow tests to override init functions
var telemetryInit = telemetry.Init
var dotenvLoad = godotenv.Load

// setup prepares the HTTP handler and initializes telemetry. It returns the
// handler to serve, a shutdown function to clean up telemetry, and an error
// if initialization failed. This is separated out to allow end-to-end tests
// to call into the server without binding to a fixed port.
func setup(ctx context.Context) (http.Handler, func(context.Context) error, error) {
	if err := dotenvLoad(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: loading .env: %v\n", err)
	}

	// initialize telemetry; fail-fast on error
	shutdown := func(context.Context) error { return nil }
	if initer := telemetryInit; initer != nil {
		var err error
		shutdown, err = initer(ctx, telemetry.Config{ServiceName: "molecular-silicon", ServiceVersion: version.Version})
		if err != nil {
			return nil, nil, err
		}
	}

	srv := newServer()
	return srv, shutdown, nil
}

type server struct {
	mu    sync.Mutex
	tasks map[string]*storedTask
}

type storedTask struct {
	t       api.Task
	cancel  context.CancelFunc
	ctx     context.Context
	created time.Time
	updated time.Time
}

func newServer() *server {
	return &server{tasks: make(map[string]*storedTask)}
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// basic routing
	if r.URL.Path == "/v1/tasks" {
		switch r.Method {
		case http.MethodPost:
			s.handleCreate(w, r)
			return
		case http.MethodGet:
			s.handleList(w, r)
			return
		}
	}
	if strings.HasPrefix(r.URL.Path, "/v1/tasks/") {
		// strip prefix
		p := strings.TrimPrefix(r.URL.Path, "/v1/tasks/")
		// possible forms: {id}, {id}/cancel, {id}/logs, {id}/cleanup
		parts := strings.SplitN(p, "/", 2)
		id := parts[0]
		if len(parts) == 1 || parts[1] == "" {
			if r.Method == http.MethodGet {
				s.handleGet(w, r, id)
				return
			}
		} else {
			switch parts[1] {
			case "cancel":
				if r.Method == http.MethodPost {
					s.handleCancel(w, r, id)
					return
				}
			case "logs":
				if r.Method == http.MethodGet {
					s.handleLogs(w, r, id)
					return
				}
			case "cleanup":
				if r.Method == http.MethodPost {
					http.Error(w, "not implemented", http.StatusNotFound)
					return
				}
			}
		}
	}
	http.NotFound(w, r)
}

func (s *server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req api.CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if req.TaskID == "" {
		http.Error(w, "task_id required", http.StatusBadRequest)
		return
	}

	now := time.Now().UTC()
	t := api.Task{
		TaskID:       req.TaskID,
		Prompt:       req.Prompt,
		Status:       "running",
		Phase:        "pending",
		CreatedAt:    now.Format(time.RFC3339),
		UpdatedAt:    now.Format(time.RFC3339),
		CarbonBudget: 3,
		HeliumBudget: 3,
		ReviewBudget: 2,
	}

	// per-task context with cancel
	ctx, cancel := context.WithCancel(context.Background())

	st := &storedTask{t: t, cancel: cancel, ctx: ctx, created: now, updated: now}

	s.mu.Lock()
	if _, exists := s.tasks[req.TaskID]; exists {
		s.mu.Unlock()
		http.Error(w, "task exists", http.StatusConflict)
		return
	}
	s.tasks[req.TaskID] = st
	s.mu.Unlock()

	// kick off execution in goroutine
	go func(st *storedTask) {
		// update phase/status
		s.mu.Lock()
		st.t.Phase = "executing"
		st.t.Status = "running"
		st.t.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		s.mu.Unlock()

		_ = task.Execute(st.ctx, st.t)

		s.mu.Lock()
		// if context was cancelled, mark cancelled, else completed
		select {
		case <-st.ctx.Done():
			st.t.Status = "cancelled"
			st.t.Phase = "cancelled"
		default:
			st.t.Status = "completed"
			st.t.Phase = "done"
		}
		st.t.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		s.mu.Unlock()
	}(st)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(st.t)
}

func (s *server) handleList(w http.ResponseWriter, r *http.Request) {
	// best-effort limit
	var limit int
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	s.mu.Lock()
	var out []api.Task
	for _, st := range s.tasks {
		out = append(out, st.t)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (s *server) handleGet(w http.ResponseWriter, r *http.Request, id string) {
	s.mu.Lock()
	st, ok := s.tasks[id]
	s.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(st.t)
}

func (s *server) handleCancel(w http.ResponseWriter, r *http.Request, id string) {
	s.mu.Lock()
	st, ok := s.tasks[id]
	s.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	st.cancel()
	// mark cancelled immediately
	s.mu.Lock()
	st.t.Status = "cancelled"
	st.t.Phase = "cancelled"
	st.t.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(st.t)
}

func (s *server) handleLogs(w http.ResponseWriter, r *http.Request, id string) {
	// placeholder: return 404 if task not found, else empty body
	s.mu.Lock()
	_, ok := s.tasks[id]
	s.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(""))
}

func main() {
	// perform setup; fail fast on telemetry init errors
	handler, shutdown, err := setup(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "telemetry init: %v\n", err)
		os.Exit(1)
	}
	defer shutdown(context.Background())

	// use default listen addr
	addr := fmt.Sprintf("%s:%d", api.DefaultHost, api.DefaultPort)
	fmt.Fprintf(os.Stderr, "silicon: listening on %s\n", addr)
	if err := http.ListenAndServe(addr, handler); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(os.Stderr, "server: %v\n", err)
		os.Exit(1)
	}
}
