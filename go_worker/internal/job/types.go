package job

// SidekiqJob represents a job in Sidekiq-compatible format
type SidekiqJob struct {
	Class      string        `json:"class"`
	Args       []interface{} `json:"args"`
	JID        string        `json:"jid"`
	Queue      string        `json:"queue"`
	CreatedAt  float64       `json:"created_at"`
	EnqueuedAt float64       `json:"enqueued_at"`
	Retry      int           `json:"retry,omitempty"`
	FailedAt   float64       `json:"failed_at,omitempty"`
	ErrorMsg   string        `json:"error_message,omitempty"`
	ErrorClass string        `json:"error_class,omitempty"`
}

// JobResult represents the response from the Rails sidecar after job execution
type JobResult struct {
	Status        string  `json:"status"`
	Result        string  `json:"result"`
	ExecutionTime float64 `json:"execution_time"`
	ErrorMessage  string  `json:"error_message,omitempty"`
}
