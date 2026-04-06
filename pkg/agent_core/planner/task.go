package planner

// TaskStatus is the lifecycle of a todo item.
type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusInProgress TaskStatus = "in_progress"
	StatusDone       TaskStatus = "done"
	StatusCancelled  TaskStatus = "cancelled"
)

// Task is one node in the todo tree (flat list with optional parent link).
type Task struct {
	ID       string     `json:"id"`
	ParentID string     `json:"parent_id,omitempty"`
	Title    string     `json:"title"`
	Status   TaskStatus `json:"status"`
	Order    int        `json:"order"`
}
