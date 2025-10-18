# Developer Setup

This guide helps you run the allocator locally or in an external Kubernetes cluster using Google Cloud Pub/Sub.

## Prerequisites
- Go 1.25+
- A Google Cloud project with Pub/Sub enabled
- A service account JSON with Pub/Sub permissions (Subscriber on request subscription, Publisher on result topic)

## Environment variables
Pub/Sub expects the resource IDs, not full resource names.
- Topic ID example: `allocator-results` (NOT `projects/your-proj/topics/allocator-results`)
- Subscription ID example: `allocator-requests-sub` (NOT `projects/your-proj/subscriptions/allocator-requests-sub`)

Minimum variables to set:
- `ALLOCATION_REQUEST_SUBSCRIPTION` = <subscription-id>
- `ALLOCATION_RESULT_TOPIC` = <topic-id>
- `GOOGLE_APPLICATION_CREDENTIALS` = path to your service account JSON

Windows PowerShell (one-liner shown across multiple lines for readability):
```powershell
$env:ALLOCATION_REQUEST_SUBSCRIPTION="<subscription-id>"; \
$env:GOOGLE_APPLICATION_CREDENTIALS="C:\path\to\service-account.json"; \
$env:ALLOCATION_RESULT_TOPIC="<topic-id>"
```

Bash example:
```bash
export ALLOCATION_REQUEST_SUBSCRIPTION="<subscription-id>" \
       GOOGLE_APPLICATION_CREDENTIALS="/path/to/service-account.json" \
       ALLOCATION_RESULT_TOPIC="<topic-id>"
```

Optional overrides:
- `ALLOCATOR_PUBSUB_PROJECT_ID` or `GOOGLE_PROJECT_ID` to explicitly set your Google project ID.
- `ALLOCATOR_GSA_CREDENTIALS` as an alternate env var for the credentials file path.
- `TARGET_NAMESPACE` for Agones (default: `default`).
- `ALLOCATOR_METRICS_PORT` (default: `8080`).
- `ALLOCATOR_LOG_LEVEL` (default: `info`). Also set `DEBUG=1` to enable debug logs.

Project ID resolution order used by the service:
1) `GOOGLE_APPLICATION_CREDENTIALS` → read `project_id` from the JSON file
2) `ALLOCATOR_PUBSUB_PROJECT_ID`
3) `GOOGLE_PROJECT_ID`
4) `GOOGLE_CLOUD_PROJECT` | `GCLOUD_PROJECT` | `GCP_PROJECT`
5) `ALLOCATOR_GSA_CREDENTIALS` JSON fallback

## Run locally
```bash
go run ./cmd
```
The service will expose:
- `/metrics` for Prometheus
- `/healthz` and `/readyz` for liveness/readiness

## Request and result payloads

Publish an allocation request to your request subscription's topic with the following schema:

```json
{
  "envelopeVersion": "1.0",
  "type": "allocation-request",
  "ticketId": "<ticket-id>",
  "fleet": "<fleet-name>",
  "playerId": "<optional-player-id>"
}
```

The allocator targets GameServers labeled with `agones.dev/fleet: <fleet-name>`.

On completion, an `allocation-result` is published to the result topic:

```json
{
  "envelopeVersion": "1.0",
  "type": "allocation-result",
  "ticketId": "<ticket-id>",
  "status": "Success | Failure",
  "token": "<base64(IP:Port)>",
  "errorMessage": "<string>"
}
```

## Kubernetes (external cluster)
1) Create a secret with your credentials JSON in your namespace (e.g., `starx`):
```bash
kubectl -n starx create secret generic gcp-sa \
  --from-file=service-account.json=/path/to/service-account.json
```
2) Update `deployments/examples/configmap.yaml` with your Topic ID and Subscription ID.
3) Apply manifests in `deployments/examples/`.

The example `Deployment` mounts the secret at `/var/secrets/google/service-account.json` and sets `GOOGLE_APPLICATION_CREDENTIALS` to that path.

## Troubleshooting
- If you see Pub/Sub errors, verify you are using IDs (topic/subscription) and not full resource paths.
- Use `DEBUG=1` to get detailed logs on initialization, message receipt, and publishing.
- Ensure the service account has Pub/Sub permissions on the specific topic and subscription.
