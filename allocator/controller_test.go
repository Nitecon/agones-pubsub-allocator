package allocator

import (
	"context"
	"testing"
	"time"

	"agones-pubsub-allocator/queues"
)

type mockPublisher struct{ err error }

func (m *mockPublisher) PublishResult(ctx context.Context, res *queues.AllocationResult) error {
	return m.err
}

func TestNewController(t *testing.T) {
	type args struct {
		p  queues.Publisher
		ns string
	}
	tests := []struct {
		name string
		args args
	}{
		{name: "with namespace", args: args{p: &mockPublisher{}, ns: "test-ns"}},
		{name: "empty namespace", args: args{p: &mockPublisher{}, ns: ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := NewController(tt.args.p, tt.args.ns)
			if ctrl == nil || ctrl.publisher != tt.args.p || ctrl.targetNamespace != tt.args.ns {
				t.Errorf("NewController() mismatch\nctrl: %#v\nargs: %#v", ctrl, tt.args)
			}
		})
	}
}

func Test_publishFailure(t *testing.T) {
	tests := []struct {
		name    string
		req     *queues.AllocationRequest
		message string
		pubErr  error
		wantErr bool
	}{
		{name: "successful publish", req: &queues.AllocationRequest{TicketID: "test-ticket", Fleet: "test-fleet"}, message: "test error", pubErr: nil, wantErr: false},
		{name: "publish error", req: &queues.AllocationRequest{TicketID: "test-ticket", Fleet: "test-fleet"}, message: "test error", pubErr: context.Canceled, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPub := &mockPublisher{err: tt.pubErr}
			ctrl := &Controller{publisher: mockPub}

			err := ctrl.publishFailure(context.Background(), tt.req, time.Now(), tt.message)

			gotErr := (err != nil)
			if gotErr != tt.wantErr {
				t.Errorf("publishFailure() error mismatch\ngotErr: %#v\nwantErr: %#v\nerr: %#v", gotErr, tt.wantErr, err)
			}
		})
	}
}
