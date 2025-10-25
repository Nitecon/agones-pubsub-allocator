package queues

import "context"

type AllocationRequest struct {
	TicketID        string   `json:"ticketId"`
	Fleet           string   `json:"fleet"`
	PlayerID        string   `json:"playerId,omitempty"`
	JoinOnIDs       []string `json:"joinOnIds,omitempty"`       // Array of player IDs to join (friends/party lead)
	CanJoinNotFound bool     `json:"canJoinNotFound,omitempty"` // Allow allocation if joinOnIds not found on any server
}

type AllocationStatus string

const (
	StatusSuccess AllocationStatus = "Success"
	StatusFailure AllocationStatus = "Failure"
	StatusQueued  AllocationStatus = "Queued" // Player is queued waiting for a slot
)

type AllocationResult struct {
	EnvelopeVersion string           `json:"envelopeVersion"`
	Type            string           `json:"type"`
	TicketID        string           `json:"ticketId"`
	Status          AllocationStatus `json:"status"`
	Token           *string          `json:"token,omitempty"`
	ErrorMessage    *string          `json:"errorMessage,omitempty"`
	QueuePosition   *int             `json:"queuePosition,omitempty"` // Position in queue if status is Queued
	QueueID         *string          `json:"queueId,omitempty"`       // Identifier for the queue (e.g., gameserver name)
}

type Subscriber interface {
	Start(ctx context.Context, handler func(context.Context, *AllocationRequest) error) error
}

type Publisher interface {
	PublishResult(ctx context.Context, res *AllocationResult) error
}
