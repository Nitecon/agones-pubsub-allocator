package pubsub

import (
	"context"
	"encoding/json"

	"agones-pubsub-allocator/queues"

	gpubsub "cloud.google.com/go/pubsub"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/option"
)

type Publisher struct {
	projectID   string
	resultTopic string
	credsFile   string
	client      *gpubsub.Client
	topic       *gpubsub.Topic
}

func NewPublisher(projectID, resultTopic, credsFile string) *Publisher {
	return &Publisher{projectID: projectID, resultTopic: resultTopic, credsFile: credsFile}
}

func (p *Publisher) PublishResult(ctx context.Context, res *queues.AllocationResult) error {
	if p.client == nil {
		var (
			client *gpubsub.Client
			err    error
		)
		if p.credsFile != "" {
			log.Debug().Str("projectID", p.projectID).Str("topic", p.resultTopic).Str("credsFile", p.credsFile).Msg("initializing pubsub publisher with explicit credentials")
			client, err = gpubsub.NewClient(ctx, p.projectID, option.WithCredentialsFile(p.credsFile))
		} else {
			log.Debug().Str("projectID", p.projectID).Str("topic", p.resultTopic).Msg("initializing pubsub publisher with default credentials")
			client, err = gpubsub.NewClient(ctx, p.projectID)
		}
		if err != nil {
			log.Error().Err(err).Str("projectID", p.projectID).Str("topic", p.resultTopic).Msg("failed to create pubsub client for publisher")
			return err
		}
		p.client = client
		p.topic = client.Topic(p.resultTopic)
		log.Info().Str("topic", p.resultTopic).Msg("pubsub publisher initialized")
	}
	b, err := json.Marshal(res)
	if err != nil {
		log.Error().Err(err).Interface("result", res).Msg("failed to marshal allocation result")
		return err
	}
	// Publish and wait for server ack
	r := p.topic.Publish(ctx, &gpubsub.Message{Data: b})
	id, err := r.Get(ctx)
	if err != nil {
		log.Error().Err(err).Str("ticketId", res.TicketID).Msg("failed to publish allocation result")
		return err
	}
	log.Debug().Str("messageID", id).Str("ticketId", res.TicketID).Str("status", string(res.Status)).Msg("published allocation result")
	return nil
}
