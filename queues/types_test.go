package queues

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestAllocationRequest_JSON(t *testing.T) {
	tests := []struct {
		name string
		in   AllocationRequest
	}{
		{"basic", AllocationRequest{TicketID: "t1", Fleet: "f1", PlayerID: "p1"}},
		{"empty optional", AllocationRequest{TicketID: "t2", Fleet: "f2"}},
		{"with joinOnIds", AllocationRequest{TicketID: "t3", Fleet: "f3", PlayerID: "p3", JoinOnIDs: []string{"friend1", "friend2"}, CanJoinNotFound: true}},
		{"joinOnIds empty", AllocationRequest{TicketID: "t4", Fleet: "f4", PlayerID: "p4", JoinOnIDs: []string{}, CanJoinNotFound: false}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.in)
			if err != nil {
				t.Fatalf("marshal err: %#v", err)
			}
			var out AllocationRequest
			if err := json.Unmarshal(b, &out); err != nil {
				t.Fatalf("unmarshal err: %#v", err)
			}
			if out.TicketID != tt.in.TicketID || out.Fleet != tt.in.Fleet || out.PlayerID != tt.in.PlayerID {
				t.Errorf("round-trip mismatch\nin:  %#v\nout: %#v", tt.in, out)
			}
			if len(out.JoinOnIDs) != len(tt.in.JoinOnIDs) {
				t.Errorf("JoinOnIDs length mismatch: got %d, want %d", len(out.JoinOnIDs), len(tt.in.JoinOnIDs))
			}
			for i := range tt.in.JoinOnIDs {
				if out.JoinOnIDs[i] != tt.in.JoinOnIDs[i] {
					t.Errorf("JoinOnIDs[%d] mismatch: got %q, want %q", i, out.JoinOnIDs[i], tt.in.JoinOnIDs[i])
				}
			}
			if out.CanJoinNotFound != tt.in.CanJoinNotFound {
				t.Errorf("CanJoinNotFound mismatch: got %v, want %v", out.CanJoinNotFound, tt.in.CanJoinNotFound)
			}
		})
	}
}

func TestAllocationResult_JSON(t *testing.T) {
	queuePos := 5
	queueID := "test-gs-123"
	tests := []struct {
		name string
		in   AllocationResult
	}{
		{"success", AllocationResult{EnvelopeVersion: "1.0", Type: "allocation-result", TicketID: "t1", Status: StatusSuccess, Token: strPtr("tok")}},
		{"failure", AllocationResult{EnvelopeVersion: "1.0", Type: "allocation-result", TicketID: "t2", Status: StatusFailure, ErrorMessage: strPtr("err")}},
		{"queued", AllocationResult{EnvelopeVersion: "1.0", Type: "allocation-result", TicketID: "t3", Status: StatusQueued, QueuePosition: &queuePos, QueueID: &queueID}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.in)
			if err != nil {
				t.Fatalf("marshal err: %#v", err)
			}
			var out AllocationResult
			if err := json.Unmarshal(b, &out); err != nil {
				t.Fatalf("unmarshal err: %#v", err)
			}
			if !reflect.DeepEqual(tt.in, out) {
				t.Errorf("roundtrip mismatch\n in=%#v\nout=%#v", tt.in, out)
			}
		})
	}
}

func strPtr(s string) *string { return &s }
