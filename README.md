# <img src="https://raw.githubusercontent.com/github/explore/main/topics/agones/agones.png" alt="agones logo" width="32"/> Agones Pub/Sub Allocator

[![Latest Release](https://img.shields.io/github/release/Nitecon/agones-pubsub-allocator.svg)](https://github.com/Nitecon/agones-pubsub-allocator/releases/latest)
[![License](https://img.shields.io/github/license/Nitecon/agones-pubsub-allocator.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/Nitecon/agones-pubsub-allocator)](https://goreportcard.com/report/github.com/Nitecon/agones-pubsub-allocator)
[![Tests](https://github.com/Nitecon/agones-pubsub-allocator/actions/workflows/ci.yml/badge.svg)](https://github.com/Nitecon/agones-pubsub-allocator/actions/workflows/ci.yml)
[![Docker Pulls](https://img.shields.io/docker/pulls/nitecon/agones-pubsub-allocator)](https://hub.docker.com/r/nitecon/agones-pubsub-allocator)

A small service that listens to Google Cloud Pub/Sub for allocation requests, allocates Agones GameServers on Kubernetes, and publishes allocation results back to Pub/Sub.

## Documentation
- [Architecture:](Docs/Architecture.md)
- [Coding Guidelines:](Docs/CodingGuidelines.md)
- [Testing Guidelines:](Docs/TestingGuidelines.md)
- [Development Setup (env vars and examples):](Docs/DevSetup.md)

## Status
- Fully working: config, metrics, health, Pub/Sub (with service account support), Docker, and example K8s manifests.
- Allocator creates `GameServerAllocation` resources targeting a Fleet and publishes results.

## Quick Run
See `Docs/DevSetup.md` for the exact environment variables. Important:
- Pub/Sub uses Topic ID and Subscription ID (not full resource names).

Windows PowerShell:
```powershell
$env:ALLOCATION_REQUEST_SUBSCRIPTION="<subscription-id>"; \
$env:GOOGLE_APPLICATION_CREDENTIALS="C:\\path\\to\\service-account.json"; \
$env:ALLOCATION_RESULT_TOPIC="<topic-id>"
```

Bash:
```bash
export ALLOCATION_REQUEST_SUBSCRIPTION="<subscription-id>" \
       GOOGLE_APPLICATION_CREDENTIALS="/path/to/service-account.json" \
       ALLOCATION_RESULT_TOPIC="<topic-id>"
```

Run locally:
```bash
go run ./cmd
```

## Usage
- **Request schema (Pub/Sub message on request subscription):**

```json
{
  "envelopeVersion": "1.0",
  "type": "allocation-request",
  "ticketId": "<ticket-id>",
  "fleet": "<fleet-name>",
  "playerId": "<optional-player-id>"
}
```

- **Behavior:**
  - Subscriber receives `{ ticketId, fleet, playerId? }`.
  - Controller allocates a `GameServer` via Agones using selector `agones.dev/fleet: <fleet>`.
  - On success, Publisher emits an `allocation-result` with a base64 token of `IP:Port`.

- **Result schema (published to result topic):**

```json
{
  "envelopeVersion": "1.0",
  "type": "allocation-result",
  "ticketId": "<ticket-id>",
  "status": "Success | Failure",
  "token": "<base64(IP:Port)>",            // present on Success
  "errorMessage": "<string>"               // present on Failure
}
```

## Configuration
Environment variables (see `Docs/DevSetup.md` for details and precedence):
- `ALLOCATION_REQUEST_SUBSCRIPTION`, `ALLOCATION_RESULT_TOPIC`
- `GOOGLE_APPLICATION_CREDENTIALS` or `ALLOCATOR_GSA_CREDENTIALS`
- `ALLOCATOR_PUBSUB_PROJECT_ID` or `GOOGLE_PROJECT_ID`
- `TARGET_NAMESPACE`, `ALLOCATOR_METRICS_PORT`, `ALLOCATOR_LOG_LEVEL`, `DEBUG`

## Contributing
Contributions are welcome. Please open an issue or PR.

## License
This project is licensed; see the [LICENSE](LICENSE) file for details.
