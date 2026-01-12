package api

type TaskStatus string

type Task struct {
	TaskID string     `json:"task_id"`
	Prompt string     `json:"prompt"`
	Status TaskStatus `json:"status"`
	Phase  string     `json:"phase"`
}

type CreateTaskRequest struct {
	TaskID string `json:"task_id"`
	Prompt string `json:"prompt"`
}
