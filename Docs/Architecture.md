# Agones Pub/Sub Allocator - Architecture

## Overview
The service subscribes to a Google Cloud Pub/Sub subscription for allocation requests, creates Agones `GameServerAllocation` resources in Kubernetes, and publishes results to a Pub/Sub topic.

## Flow
1. queues/pubsub Subscriber receives JSON payload `{ ticketId, mapName }`.
2. allocator.Controller handles the request, invokes Agones Allocation API.
3. On success: generate Quilkin token as base64("IP:Port").
4. Publish result to `ALLOCATION_RESULT_TOPIC` via queues/pubsub Publisher.
5. Expose `/metrics`, `/healthz`, `/readyz` via HTTP.

## Packages
- cmd/main.go: wiring and lifecycle (config, health, metrics, queues, controller)
- config/: env-based configuration
- queues/: abstraction for message queues
  - queues/pubsub: Google Pub/Sub implementation
- allocator/: domain logic to perform allocations
- metrics/: Prometheus metrics
- health/: liveness and readiness

## Configuration (env)
- ALLOCATOR_PUBSUB_PROJECT_ID
- ALLOCATION_REQUEST_SUBSCRIPTION
- ALLOCATION_RESULT_TOPIC
- TARGET_NAMESPACE
- ALLOCATOR_METRICS_PORT (default 8080)
- ALLOCATOR_LOG_LEVEL (default info)

## Kubernetes
- Namespace: `starx`
- ServiceAccount: `allocator-sa`
- Role/RoleBinding: create on `gameserverallocations` only
- Deployment: non-root, read-only FS; probes enabled
- NetworkPolicy: deny ingress; allow egress TCP 443

## CI/CD
- Build & test via GitHub Actions
- Build/push container to GHCR (adjust image path)

## Future Work
- Implement Pub/Sub client
- Implement Agones client and allocation logic
- Add retries/backoff and DLQ (optional)
- Add tracing and richer metrics
