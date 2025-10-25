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

func Test_splitAndTrim(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "empty string", input: "", want: nil},
		{name: "single token", input: "token1", want: []string{"token1"}},
		{name: "multiple tokens", input: "token1,token2,token3", want: []string{"token1", "token2", "token3"}},
		{name: "tokens with spaces", input: "token1, token2 , token3", want: []string{"token1", "token2", "token3"}},
		{name: "tokens with empty parts", input: "token1,,token2", want: []string{"token1", "token2"}},
		{name: "only commas", input: ",,,", want: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitAndTrim(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("splitAndTrim() length mismatch\ngot: %v\nwant: %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitAndTrim() element %d mismatch\ngot: %v\nwant: %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func Test_appendToken(t *testing.T) {
	tests := []struct {
		name           string
		existingTokens string
		newToken       string
		want           string
	}{
		{name: "empty existing", existingTokens: "", newToken: "token1", want: "token1"},
		{name: "append to single", existingTokens: "token1", newToken: "token2", want: "token1,token2"},
		{name: "append to multiple", existingTokens: "token1,token2", newToken: "token3", want: "token1,token2,token3"},
		{name: "duplicate token", existingTokens: "token1,token2", newToken: "token1", want: "token1,token2"},
		{name: "duplicate in middle", existingTokens: "token1,token2,token3", newToken: "token2", want: "token1,token2,token3"},
		{name: "with spaces", existingTokens: "token1, token2", newToken: "token3", want: "token1, token2,token3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appendToken(tt.existingTokens, tt.newToken)
			if got != tt.want {
				t.Errorf("appendToken() mismatch\ngot: %v\nwant: %v", got, tt.want)
			}
		})
	}
}
