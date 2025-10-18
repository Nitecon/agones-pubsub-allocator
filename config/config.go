package config

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

type Config struct {
	PubsubTopic     string
	Subscription    string
	GoogleProjectID string
	TargetNamespace string
	MetricsPort     int
	LogLevel        string
	CredentialsFile string
}

func Load() *Config {
	cfg := &Config{
		Subscription:    strings.TrimSpace(getEnv("ALLOCATION_REQUEST_SUBSCRIPTION", os.Getenv("ALLOCATOR_PUBSUB_SUBSCRIPTION"))),
		PubsubTopic:     strings.TrimSpace(getEnv("ALLOCATION_RESULT_TOPIC", os.Getenv("ALLOCATOR_PUBSUB_TOPIC"))),
		TargetNamespace: strings.TrimSpace(getEnv("TARGET_NAMESPACE", "default")),
		MetricsPort:     getEnvInt("ALLOCATOR_METRICS_PORT", 8080),
		LogLevel:        strings.TrimSpace(getEnv("ALLOCATOR_LOG_LEVEL", "info")),
		CredentialsFile: strings.TrimSpace(firstNonEmpty(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"), os.Getenv("ALLOCATOR_GSA_CREDENTIALS"))),
	}

	cfg.GoogleProjectID = getGoogleProjectID(cfg.CredentialsFile, strings.TrimSpace(getEnv("ALLOCATOR_PUBSUB_PROJECT_ID", "")))
	if cfg.GoogleProjectID == "" {
		log.Warn().Msg("Google project ID not resolved; set GOOGLE_APPLICATION_CREDENTIALS or GOOGLE_PROJECT_ID or ALLOCATOR_PUBSUB_PROJECT_ID")
	}
	if cfg.Subscription == "" {
		log.Warn().Msg("Pub/Sub subscription not set; set ALLOCATION_REQUEST_SUBSCRIPTION or ALLOCATOR_PUBSUB_SUBSCRIPTION")
	}
	if cfg.PubsubTopic == "" {
		log.Warn().Msg("Pub/Sub topic not set; set ALLOCATION_RESULT_TOPIC or ALLOCATOR_PUBSUB_TOPIC")
	}
	return cfg
}

func (c *Config) HTTPAddr() string {
	return net.JoinHostPort("0.0.0.0", strconv.Itoa(c.MetricsPort))
}

// Redacted returns a view safe for logging
func (c *Config) Redacted() map[string]any {
	return map[string]any{
		"projectID":           c.GoogleProjectID,
		"requestSubscription": c.Subscription,
		"resultTopic":         c.PubsubTopic,
		"targetNamespace":     c.TargetNamespace,
		"metricsPort":         c.MetricsPort,
		"logLevel":            c.LogLevel,
		"credentialsProvided": c.CredentialsFile != "",
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	if def == "" {
		return def
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		iv, err := strconv.Atoi(v)
		if err == nil {
			return iv
		}
		fmt.Printf("invalid int for %s: %s\n", key, v)
	}
	return def
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func projectIDFromCredentials(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}
	var x struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.Unmarshal(b, &x); err != nil {
	}
	return x.ProjectID, nil
}

func getGoogleProjectID(credsFile string, explicit string) string {
	// 1) Prefer GOOGLE_APPLICATION_CREDENTIALS if set
	if p := strings.TrimSpace(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")); p != "" {
		log.Info().Str("credsFile", p).Msg("GOOGLE_APPLICATION_CREDENTIALS is set; extracting project_id from credentials file")
		if pid, err := projectIDFromCredentials(p); err == nil && pid != "" {
			return strings.TrimSpace(pid)
		}
		log.Warn().Str("credsFile", p).Msg("project_id not found in credentials file or unreadable")
	}

	// 2) Explicit override from allocator env
	if explicit := strings.TrimSpace(explicit); explicit != "" {
		log.Info().Str("projectID", explicit).Msg("using ALLOCATOR_PUBSUB_PROJECT_ID for Google project")
		return explicit
	}

	// 3) External k8s override
	if v := strings.TrimSpace(os.Getenv("GOOGLE_PROJECT_ID")); v != "" {
		log.Info().Str("projectID", v).Msg("using GOOGLE_PROJECT_ID from environment")
		return v
	}

	// 4) Common Google envs
	if v := firstNonEmpty(os.Getenv("GOOGLE_CLOUD_PROJECT"), os.Getenv("GCLOUD_PROJECT"), os.Getenv("GCP_PROJECT")); strings.TrimSpace(v) != "" {
		v = strings.TrimSpace(v)
		log.Info().Str("projectID", v).Msg("using Google project from common environment variables")
		return v
	}

	// 5) Fallback to provided credentials file path (ALLOCATOR_GSA_CREDENTIALS)
	if p := strings.TrimSpace(credsFile); p != "" {
		if pid, err := projectIDFromCredentials(p); err == nil && pid != "" {
			log.Info().Str("credsFile", p).Msg("using project_id from provided credentials file")
			return strings.TrimSpace(pid)
		}
	}
	return ""
}
