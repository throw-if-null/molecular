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
}

type CreateTaskRequest struct {
	TaskID string `json:"task_id"`
	Prompt string `json:"prompt"`
}
