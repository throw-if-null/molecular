package api

type TaskStatus string

type Task struct {
	TaskID           string     `json:"task_id"`
	Prompt           string     `json:"prompt"`
	Status           TaskStatus `json:"status"`
	Phase            string     `json:"phase"`
	CreatedAt        string     `json:"created_at"`
	UpdatedAt        string     `json:"updated_at"`
	CarbonBudget     int        `json:"carbon_budget"`
	HeliumBudget     int        `json:"helium_budget"`
	ReviewBudget     int        `json:"review_budget"`
	ArtifactsRoot    string     `json:"artifacts_root"`
	WorktreePath     string     `json:"worktree_path"`
	CurrentAttemptID *int64     `json:"current_attempt_id,omitempty"`
	LatestAttempt    *Attempt   `json:"latest_attempt,omitempty"`
}

type CreateTaskRequest struct {
	TaskID string `json:"task_id"`
	Prompt string `json:"prompt"`
}

type Attempt struct {
	ID           int64  `json:"id"`
	TaskID       string `json:"task_id"`
	Role         string `json:"role"`
	AttemptNum   int64  `json:"attempt_num"`
	Status       string `json:"status"`
	StartedAt    string `json:"started_at"`
	FinishedAt   string `json:"finished_at"`
	ArtifactsDir string `json:"artifacts_dir"`
	ErrorSummary string `json:"error_summary"`
}
