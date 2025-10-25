package allocator

import (
	"context"
	"encoding/base64"
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

func Test_buildQuilkinToken(t *testing.T) {
	tests := []struct {
		name     string
		playerID string
		wantLen  int // Expected length of decoded token (should be 16 bytes)
	}{
		{name: "short playerID", playerID: "player1", wantLen: 16},
		{name: "exact 16 chars", playerID: "player1234567890", wantLen: 16},
		{name: "long playerID truncated", playerID: "verylongplayeridthatexceeds16bytes", wantLen: 16},
		{name: "empty playerID", playerID: "", wantLen: 16},
		{name: "special chars", playerID: "player@123!$%", wantLen: 16},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := buildQuilkinToken(tt.playerID)

			// Decode the base64 token
			decoded, err := base64.StdEncoding.DecodeString(token)
			if err != nil {
				t.Errorf("buildQuilkinToken() produced invalid base64: %v", err)
				return
			}

			// Verify length is exactly 16 bytes
			if len(decoded) != tt.wantLen {
				t.Errorf("buildQuilkinToken() decoded length = %d, want %d", len(decoded), tt.wantLen)
			}

			// Verify first N bytes contain playerID (or truncated version)
			expectedPrefix := tt.playerID
			if len(expectedPrefix) > 16 {
				expectedPrefix = expectedPrefix[:16]
			}
			actualPrefix := string(decoded[:len(expectedPrefix)])
			if actualPrefix != expectedPrefix {
				t.Errorf("buildQuilkinToken() prefix = %q, want %q", actualPrefix, expectedPrefix)
			}

			// Verify padding with zeros if playerID is shorter than 16 bytes
			if len(tt.playerID) < 16 {
				for i := len(tt.playerID); i < 16; i++ {
					if decoded[i] != 0 {
						t.Errorf("buildQuilkinToken() byte[%d] = %d, want 0 (padding)", i, decoded[i])
					}
				}
			}
		})
	}
}

func Test_buildQuilkinToken_RealExample(t *testing.T) {
	// Test with a real Firebase-style UID
	playerID := "lRTSKLe4sKQYbqo0"
	token := buildQuilkinToken(playerID)

	// Decode and verify
	decoded, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("Failed to decode token: %v", err)
	}

	// Should be exactly 16 bytes
	if len(decoded) != 16 {
		t.Errorf("Token length = %d, want 16", len(decoded))
	}

	// All 16 bytes should match playerID
	if string(decoded) != playerID {
		t.Errorf("Token = %q, want %q", string(decoded), playerID)
	}

	t.Logf("PlayerID: %s", playerID)
	t.Logf("Token (base64): %s", token)
	t.Logf("Token (decoded hex): % x", decoded)
}

func Test_removeToken(t *testing.T) {
	tests := []struct {
		name           string
		existingTokens string
		tokenToRemove  string
		want           string
	}{
		{name: "empty string", existingTokens: "", tokenToRemove: "token1", want: ""},
		{name: "single token remove", existingTokens: "token1", tokenToRemove: "token1", want: ""},
		{name: "remove first", existingTokens: "token1,token2,token3", tokenToRemove: "token1", want: "token2,token3"},
		{name: "remove middle", existingTokens: "token1,token2,token3", tokenToRemove: "token2", want: "token1,token3"},
		{name: "remove last", existingTokens: "token1,token2,token3", tokenToRemove: "token3", want: "token1,token2"},
		{name: "token not found", existingTokens: "token1,token2", tokenToRemove: "token3", want: "token1,token2"},
		{name: "with spaces", existingTokens: "token1, token2 , token3", tokenToRemove: "token2", want: "token1,token3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := removeToken(tt.existingTokens, tt.tokenToRemove)
			if got != tt.want {
				t.Errorf("removeToken() = %q, want %q", got, tt.want)
			}
		})
	}
}
