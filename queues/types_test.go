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
			if !reflect.DeepEqual(tt.in, out) {
				t.Errorf("roundtrip mismatch\n in=%#v\nout=%#v", tt.in, out)
			}
		})
	}
}

func TestAllocationResult_JSON(t *testing.T) {
	tests := []struct {
		name string
		in   AllocationResult
	}{
		{"success", AllocationResult{EnvelopeVersion: "1.0", Type: "allocation-result", TicketID: "t1", Status: StatusSuccess, Token: strPtr("tok")}},
		{"failure", AllocationResult{EnvelopeVersion: "1.0", Type: "allocation-result", TicketID: "t2", Status: StatusFailure, ErrorMessage: strPtr("err")}},
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
