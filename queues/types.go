package queues

import "context"

type AllocationRequest struct {
	TicketID string `json:"ticketId"`
	Fleet    string `json:"fleet"`
	PlayerID string `json:"playerId,omitempty"`
}

type AllocationStatus string

const (
	StatusSuccess AllocationStatus = "Success"
	StatusFailure AllocationStatus = "Failure"
)

type AllocationResult struct {
	EnvelopeVersion string           `json:"envelopeVersion"`
	Type            string           `json:"type"`
	TicketID        string           `json:"ticketId"`
	Status          AllocationStatus `json:"status"`
	Token           *string          `json:"token,omitempty"`
	ErrorMessage    *string          `json:"errorMessage,omitempty"`
}

type Subscriber interface {
	Start(ctx context.Context, handler func(context.Context, *AllocationRequest) error) error
}

type Publisher interface {
	PublishResult(ctx context.Context, res *AllocationResult) error
}
