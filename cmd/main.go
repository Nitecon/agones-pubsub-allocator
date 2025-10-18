package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"agones-pubsub-allocator/allocator"
	"agones-pubsub-allocator/config"
	"agones-pubsub-allocator/health"
	"agones-pubsub-allocator/metrics"
	"agones-pubsub-allocator/queues"
	qpubsub "agones-pubsub-allocator/queues/pubsub"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var version = "source"

func setLogger() {
	zerolog.TimeFieldFormat = time.RFC3339Nano
	//if len(os.Getenv("CONSOLE_LOG")) > 0 {
	//}
	if os.Getenv("DEBUG") != "" {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		return
	}
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

func main() {
	setLogger()
	log.Info().Msgf("Starting agones-pubsub-allocator version: %s", version)
	// Load config
	cfg := config.Load()
	log.Info().Interface("config", cfg.Redacted()).Msg("config loaded")

	// Preflight required configuration
	if cfg.GoogleProjectID == "" {
		log.Fatal().Msg("missing Google project id; set GOOGLE_APPLICATION_CREDENTIALS or GOOGLE_PROJECT_ID or ALLOCATOR_PUBSUB_PROJECT_ID")
	}
	if cfg.Subscription == "" {
		log.Fatal().Msg("missing Pub/Sub subscription; set ALLOCATION_REQUEST_SUBSCRIPTION or ALLOCATOR_PUBSUB_SUBSCRIPTION")
	}
	if cfg.PubsubTopic == "" {
		log.Fatal().Msg("missing Pub/Sub topic; set ALLOCATION_RESULT_TOPIC or ALLOCATOR_PUBSUB_TOPIC")
	}

	// Context and shutdown handling
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Metrics and health HTTP server
	mux := http.NewServeMux()
	metrics.Register(mux)
	health.Register(mux)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr(),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info().Str("addr", cfg.HTTPAddr()).Msg("starting metrics/health server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("http server error")
		}
	}()

	if cfg.CredentialsFile != "" {
		log.Info().Str("credsFile", cfg.CredentialsFile).Msg("using explicit Google credentials file")
	} else {
		log.Info().Msg("using default Google credentials (in-cluster or ambient)")
	}
	publisher := qpubsub.NewPublisher(cfg.GoogleProjectID, cfg.PubsubTopic, cfg.CredentialsFile)
	controller := allocator.NewController(publisher, cfg.TargetNamespace)
	subscriber := qpubsub.NewSubscriber(cfg.GoogleProjectID, cfg.Subscription, cfg.CredentialsFile)

	// Start subscriber loop
	go func() {
		log.Info().Str("subscription", cfg.Subscription).Msg("starting subscriber loop")
		if err := subscriber.Start(ctx, func(ctx context.Context, req *queues.AllocationRequest) error {
			return controller.Handle(ctx, req)
		}); err != nil {
			// Non-recoverable: if we can't receive from Pub/Sub, terminate the process
			log.Fatal().Err(err).Msg("subscriber exited with fatal error; shutting down")
		}
	}()

	// Block until shutdown
	<-ctx.Done()
	log.Info().Msg("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("http server graceful shutdown failed")
	}
	log.Info().Msg("shutdown complete")
}
