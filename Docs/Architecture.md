# Agones Pub/Sub Allocator - Architecture

## Overview
The service subscribes to a Google Cloud Pub/Sub subscription for allocation requests, creates Agones `GameServerAllocation` resources in Kubernetes, and publishes results to a Pub/Sub topic. It exposes Prometheus metrics and health endpoints.

## Flow
1. `queues/pubsub.Subscriber.Start()` receives JSON payload `{ ticketId, fleet, playerId? }` from the request subscription.
2. `allocator.Controller.Handle()` validates and invokes the Agones Allocation API using selector `agones.dev/fleet=<fleet>`.
3. On success: build a token as base64 of `"<IP>:<Port>"` from the allocated GameServer status.
4. Publish an `allocation-result` to `ALLOCATION_RESULT_TOPIC` via `queues/pubsub.Publisher.PublishResult()`.
5. `/metrics`, `/healthz`, `/readyz` are served via the HTTP server in `cmd/main.go`.

## Packages
- `cmd/main.go`: wiring and lifecycle (config, health, metrics, queues, controller)
- `config/`: env-based configuration and project ID resolution
  - `queues/pubsub`: Google Pub/Sub implementation for subscriber and publisher
- `allocator/`: domain logic and Agones client integration
- `metrics/`: Prometheus metrics registration
- `health/`: liveness and readiness handlers

## Configuration (env)
- `ALLOCATION_REQUEST_SUBSCRIPTION` (or `ALLOCATOR_PUBSUB_SUBSCRIPTION`)
- `ALLOCATION_RESULT_TOPIC` (or `ALLOCATOR_PUBSUB_TOPIC`)
- `ALLOCATOR_PUBSUB_PROJECT_ID` (alternatively resolved, see below)
- `TARGET_NAMESPACE`
- `ALLOCATOR_METRICS_PORT` (default 8080)
- `ALLOCATOR_LOG_LEVEL` (default info)
- `GOOGLE_APPLICATION_CREDENTIALS` or `ALLOCATOR_GSA_CREDENTIALS` (optional; enables explicit SA file)

### Google Project ID resolution order
1. From `GOOGLE_APPLICATION_CREDENTIALS` JSON `project_id`
2. `ALLOCATOR_PUBSUB_PROJECT_ID`
3. `GOOGLE_PROJECT_ID`
4. `GOOGLE_CLOUD_PROJECT` | `GCLOUD_PROJECT` | `GCP_PROJECT`
5. Fallback: parse `ALLOCATOR_GSA_CREDENTIALS` file

## Kubernetes
- Deploy using `deployments/deployment-metal.yaml` or your own manifests
- Mount a GCP SA secret and set `GOOGLE_APPLICATION_CREDENTIALS` if running outside GCP
- Exposes HTTP on `0.0.0.0:<ALLOCATOR_METRICS_PORT>` for `/metrics`, `/healthz`, `/readyz`

## CI/CD
- Build & test via GitHub Actions (`.github/workflows/ci.yml`)
- Container image published to your registry of choice; see `Dockerfile`

## Future Work
- Retries/backoff and optional DLQ handling
- Tracing and additional metrics (labels by fleet, namespaces, etc.)
- RBAC hardening examples and network policies
