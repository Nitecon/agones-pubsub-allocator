package pubsub

import (
	"context"
	"encoding/json"
	"time"

	"agones-pubsub-allocator/queues"

	gpubsub "cloud.google.com/go/pubsub"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/option"
)

type Subscriber struct {
	projectID        string
	subscriptionName string
	credsFile        string
	client           *gpubsub.Client
	sub              *gpubsub.Subscription
}

func NewSubscriber(projectID, subscriptionName, credsFile string) *Subscriber {
	return &Subscriber{projectID: projectID, subscriptionName: subscriptionName, credsFile: credsFile}
}

func (s *Subscriber) Start(ctx context.Context, handler func(context.Context, *queues.AllocationRequest) error) error {
	if s.client == nil {
		var (
			client *gpubsub.Client
			err    error
		)
		if s.credsFile != "" {
			log.Debug().Str("projectID", s.projectID).Str("subscription", s.subscriptionName).Str("credsFile", s.credsFile).Msg("initializing pubsub subscriber with explicit credentials")
			client, err = gpubsub.NewClient(ctx, s.projectID, option.WithCredentialsFile(s.credsFile))
		} else {
			log.Debug().Str("projectID", s.projectID).Str("subscription", s.subscriptionName).Msg("initializing pubsub subscriber with default credentials")
			client, err = gpubsub.NewClient(ctx, s.projectID)
		}
		if err != nil {
			log.Error().Err(err).Str("projectID", s.projectID).Str("subscription", s.subscriptionName).Msg("failed to create pubsub client for subscriber")
			return err
		}
		s.client = client
		s.sub = client.Subscription(s.subscriptionName)
		// Conservative: disable ordering; allow default settings otherwise
		log.Info().Str("subscription", s.subscriptionName).Msg("pubsub subscriber initialized")
	}

	// Receive blocks; it will create goroutines internally; respect ctx cancellation
	return s.sub.Receive(ctx, func(ctx context.Context, m *gpubsub.Message) {

		log.Debug().Str("messageID", m.ID).Int("size", len(m.Data)).Msg("received pubsub message")
		recvAt := time.Now()
		var req queues.AllocationRequest
		if err := json.Unmarshal(m.Data, &req); err != nil {
			log.Error().Err(err).Msg("failed to unmarshal allocation request")
			// Nack to allow retry
			m.Nack()
			return
		}
		// Basic validation
		if req.TicketID == "" || req.Fleet == "" {
			log.Error().Str("ticketId", req.TicketID).Str("fleet", req.Fleet).Msg("invalid request payload")
			// Ack to drop bad message (poison)
			m.Ack()
			return
		}

		log.Info().Str("ticketId", req.TicketID).Str("fleet", req.Fleet).Str("playerId", req.PlayerID).Msg("handling allocation request")
		if err := handler(ctx, &req); err != nil {
			log.Error().Err(err).Str("ticketId", req.TicketID).Msg("handler failed; will retry")
			m.Nack()
			return
		}
		// Success -> ack
		log.Debug().Str("ticketId", req.TicketID).Dur("latency", time.Since(recvAt)).Msg("handler succeeded; acking message")
		m.Ack()
	})
}
