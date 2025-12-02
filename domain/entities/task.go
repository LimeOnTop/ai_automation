package entities

// Task represents a user task
type Task struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Status      TaskStatus `json:"status"`
	Actions     []Action `json:"actions,omitempty"`
	Context     string   `json:"context,omitempty"`
}

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusWaiting   TaskStatus = "waiting_user_input"
)

