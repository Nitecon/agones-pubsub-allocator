package allocator

import "time"

// Request represents an allocation request to Agones
// Mirrors queues.AllocationRequest but kept decoupled to avoid import loops
// This is the internal domain model.
type Request struct {
	TicketID string
	MapName  string
}

// Result represents the outcome of an allocation attempt
// Mirrors queues.AllocationResult semantics

type Status string

const (
	StatusSuccess Status = "Success"
	StatusFailure Status = "Failure"
)

type Result struct {
	TicketID string
	Status   Status
	Token    *string
	Error    *string
	// Timing/diagnostics
	StartedAt  time.Time
	FinishedAt time.Time
	Duration   time.Duration
}
