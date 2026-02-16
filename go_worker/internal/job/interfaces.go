package job

// JobProcessor defines the interface for processing jobs
type JobProcessor interface {
	// ProcessJob processes a single job and returns an error if processing fails
	ProcessJob(job *SidekiqJob) error

	// RetryJob handles retry logic for a failed job with the given attempt number
	RetryJob(job *SidekiqJob, attempt int) error
}
