package pubsub

import (
	"context"
	"testing"

	"agones-pubsub-allocator/queues"
	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/pubsub/pstest"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type args struct {
	res *queues.AllocationResult
}

type test struct {
	name    string
	setup   func() *Publisher
	args    args
	wantErr bool
}

func TestPublisher_PublishResult(t *testing.T) {
	if testing.Short() {
		t.Skip("short")
	}

	// Start in-memory Pub/Sub server
	srv := pstest.NewServer()
	defer srv.Close()

	ctx := context.Background()
	conn, err := grpc.Dial(srv.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial error: %#v", err)
	}
	defer conn.Close()

	client, err := pubsub.NewClient(ctx, "test-project", option.WithGRPCConn(conn))
	if err != nil {
		t.Fatalf("client error: %#v", err)
	}
	defer client.Close()

	tests := []test{
		{
			name: "success",
			setup: func() *Publisher {
				topic, err := client.CreateTopic(ctx, "test-topic")
				if err != nil {
					t.Fatalf("create topic: %#v", err)
				}
				// Build publisher with injected client/topic
				return &Publisher{projectID: "test-project", resultTopic: "test-topic", client: client, topic: topic}
			},
			args:    args{res: &queues.AllocationResult{EnvelopeVersion: "1.0", Type: "allocated-result", TicketID: "t1", Status: queues.StatusSuccess}},
			wantErr: false,
		},
		{
			name: "missing topic error",
			setup: func() *Publisher {
				// Get handle to non-existent topic
				topic := client.Topic("missing-topic")
				return &Publisher{projectID: "test-project", resultTopic: "missing-topic", client: client, topic: topic}
			},
			args:    args{res: &queues.AllocationResult{EnvelopeVersion: "1.0", Type: "allocated-result", TicketID: "t2", Status: queues.StatusFailure}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.setup()
			err := p.PublishResult(ctx, tt.args.res)
			gotErr := (err != nil)
			if gotErr != tt.wantErr {
				t.Errorf("PublishResult() error mismatch\ngotErr: %#v\nwantErr: %#v\nerr: %#v", gotErr, tt.wantErr, err)
			}
		})
	}
}

func strPtr(s string) *string { return &s }
