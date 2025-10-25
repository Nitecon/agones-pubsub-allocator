# <img src="https://agones.dev/site/images/logo.svg" alt="agones logo" width="32"/> Agones Pub/Sub Allocator

[![Latest Release](https://img.shields.io/github/release/Nitecon/agones-pubsub-allocator.svg)](https://github.com/Nitecon/agones-pubsub-allocator/releases/latest)
[![License](https://img.shields.io/github/license/Nitecon/agones-pubsub-allocator.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/Nitecon/agones-pubsub-allocator)](https://goreportcard.com/report/github.com/Nitecon/agones-pubsub-allocator)
[![Tests](https://github.com/Nitecon/agones-pubsub-allocator/actions/workflows/ci.yml/badge.svg)](https://github.com/Nitecon/agones-pubsub-allocator/actions/workflows/ci.yml)
[![Docker Pulls](https://img.shields.io/docker/pulls/nitecon/agones-pubsub-allocator)](https://hub.docker.com/repository/docker/nitecon/agones-pubsub-allocator)

A small service that listens to Google Cloud Pub/Sub for allocation requests, allocates Agones GameServers on Kubernetes, and publishes allocation results back to Pub/Sub.

## Documentation
- [Architecture:](Docs/Architecture.md)
- [Coding Guidelines:](Docs/CodingGuidelines.md)
- [Testing Guidelines:](Docs/TestingGuidelines.md)
- [Development Setup (env vars and examples):](Docs/DevSetup.md)
- [New: Join on player IDs](Docs/JoinOnIds.md)

## Status
- Fully working: config, metrics, health, Pub/Sub (with service account support), Docker, and example K8s manifests.
- Allocator creates `GameServerAllocation` resources targeting a Fleet and publishes results.

## Quick Run
See `Docs/DevSetup.md` for the exact environment variables. Important:
- Pub/Sub uses Topic ID and Subscription ID (not full resource names).

## Install on bare metal k8s
1) Create a secret with your credentials JSON in your namespace (e.g., `starx`) (you must have a service account with Pub/Sub permissions):
```bash
kubectl -n starx create secret generic gcp-sa --from-file=service-account.json=/path/to/service-account.json
```
2) Now you need to create a configmap with your pubsub details etc
```bash
echo 'apiVersion: v1
kind: ConfigMap
metadata:
  name: agones-allocator-config
  labels:
    app: agones-allocator
data:
  projectId: your-gcp-project
  requestSubscription: allocator-requests-sub
  resultTopic: allocator-results
  targetNamespace: starx' > configmap.yaml
kubectl -n starx apply -f configmap.yaml
```
3) Apply the deployment manifest to run the allocator:
```bash
kubectl apply -f https://raw.githubusercontent.com/Nitecon/agones-pubsub-allocator/refs/heads/main/deployments/deployment-metal.yaml
```
The example `Deployment` mounts the secret at `/var/secrets/google/service-account.json` and sets `GOOGLE_APPLICATION_CREDENTIALS` to that path.

Note that once you have run the commands above you should see the pod spin up and you should see logs like so:
```
$ kubectl get po -n starx
NAME                                     READY   STATUS        RESTARTS       AGE
agones-allocator-55d6484fcb-rkjgl        1/1     Running       0              53s
quilkin-manage-agones-5d5b4595cd-99bt6   0/1     Running       42 (38m ago)   2d7h
quilkin-proxies-847f5545cc-bp4wj         0/1     Running       0              2d7h
quilkin-proxies-847f5545cc-bzjdk         0/1     Running       0              2d7h
quilkin-proxies-847f5545cc-h6t6q         0/1     Running       0              2d7h
```
Then you can query the agones-allocator logs:
```
$ kubectl logs -n starx agones-allocator-55d6484fcb-rkjgl
```
Which should respond with output similar to following which should match your configmap:
```
{"level":"info","time":"2025-10-19T00:51:42.951186146Z","message":"Starting agones-pubsub-allocator version: main"}
{"level":"info","credsFile":"/var/secrets/google/service-account.json","time":"2025-10-19T00:51:42.951310936Z","message":"GOOGLE_APPLICATION_CREDENTIALS is set; extracting project_id from credentials file"}
{"level":"info","config":{"credentialsProvided":true,"logLevel":"info","metricsPort":8080,"projectID":"starx-123123","requestSubscription":"dev-sub","resultTopic":"dev","targetNamespace":"starx"},"time":"2025-10-19T00:51:42.951441878Z","message":"config loaded"}
{"level":"info","credsFile":"/var/secrets/google/service-account.json","time":"2025-10-19T00:51:42.952468016Z","message":"using explicit Google credentials file"}
{"level":"info","subscription":"dev-sub","time":"2025-10-19T00:51:42.952502712Z","message":"starting subscriber loop"}
{"level":"info","addr":"0.0.0.0:8080","time":"2025-10-19T00:51:43.043740047Z","message":"starting metrics/health server"}
{"level":"info","subscription":"dev-sub","time":"2025-10-19T00:51:43.152131405Z","message":"pubsub subscriber initialized"}
```

## Develop locally:

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

### Request Schema
**Pub/Sub message on request subscription:**

```json
{
  "envelopeVersion": "1.0",
  "type": "allocation-request",
  "ticketId": "abcdef",
  "fleet": "starx",
  "playerId": "123asdf",
  "joinOnIds": ["friend1", "friend2"],     // optional: array of player IDs to join
  "canJoinNotFound": true                  // optional: allow allocation if friends not found
}
```

**Fields:**
- **`ticketId`** (required): Unique identifier for this allocation request
- **`fleet`** (required): Name of the Agones fleet to allocate from
- **`playerId`** (required): Player's unique identifier (used for token generation)
- **`joinOnIds`** (optional): Array of player IDs to join (friends/party members). If provided, the allocator will search for gameservers where these players are already allocated
- **`canJoinNotFound`** (optional): If `true` and friends are not found, proceeds with normal allocation. If `false` and friends are not found, the request fails

### Behavior

**Token Cleanup:**
- Before any allocation, the player's token is removed from all gameservers in the fleet
- This ensures a player only has one active server allocation at a time
- Allows players to switch servers during a play session

**Friend Joining:**
- If `joinOnIds` is provided, the allocator searches for gameservers with those player tokens
- If found, the player is added to the friend's gameserver
- If not found and `canJoinNotFound=true`, proceeds with normal allocation
- If not found and `canJoinNotFound=false`, the request fails

**Normal Allocation:**
- Controller allocates a `GameServer` via Agones using selector `agones.dev/fleet: <fleet>`
- Agones handles proper server allocation based on capacity, player count, etc.
- On success, Publisher emits an `allocation-result` with a Quilkin-compatible token

### Result Schema
**Published to result topic:**

```json
{
  "envelopeVersion": "1.0",
  "type": "allocation-result",
  "ticketId": "<ticket-id>",
  "status": "Success | Failure | Queued",
  "token": "<base64-encoded-token>",      // present on Success
  "errorMessage": "<string>",              // present on Failure
  "queuePosition": 5,                      // present on Queued
  "queueId": "gameserver-name"             // present on Queued
}
```

**Status Values:**
- **`Success`**: Player successfully allocated to a gameserver. `token` field contains the routing token
- **`Failure`**: Allocation failed. `errorMessage` contains the error details
- **`Queued`**: Player is queued waiting for a slot (future feature). `queuePosition` and `queueId` indicate position in queue

## Quilkin Token Format

This allocator generates **Quilkin-compatible routing tokens** that are added to the GameServer's `quilkin.dev/tokens` annotation.

### Token Specification
- **Format**: 16-byte string
- **Source**: Derived from the `playerId` field in the allocation request
- **Encoding**: Base64 encoded
- **Behavior**:
  - PlayerIDs shorter than 16 bytes are zero-padded
  - PlayerIDs longer than 16 bytes are truncated

### Example
```
PlayerID: lRTSKLe4sKQYbqo0
Token (base64): bFJUU0tMZTRzS1FZYnFvMA==
Token (decoded): lRTSKLe4sKQYbqo0  (exactly 16 bytes)
```

### Quilkin Configuration

To use these tokens with Quilkin, configure your filter chain to capture the 16-byte suffix:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: quilkin-xds-filter-config
  namespace: starx
  labels:
    quilkin.dev/configmap: "true"
data:
  quilkin.yaml: |
    version: v1alpha1
    filters:
      - name: quilkin.filters.capture.v1alpha1.Capture
        config:
          metadataKey: "quilkin.dev/token"
          suffix:
            size: 16
            remove: true
      - name: quilkin.filters.token_router.v1alpha1.TokenRouter
```

**Important**: The `suffix.size` must be set to `16` to match the token format.

## Environment Configuration
Environment variables (see `Docs/DevSetup.md` for details and precedence):
- `ALLOCATION_REQUEST_SUBSCRIPTION`, `ALLOCATION_RESULT_TOPIC`
- `GOOGLE_APPLICATION_CREDENTIALS` or `ALLOCATOR_GSA_CREDENTIALS`
- `ALLOCATOR_PUBSUB_PROJECT_ID` or `GOOGLE_PROJECT_ID`
- `TARGET_NAMESPACE`, `ALLOCATOR_METRICS_PORT`, `ALLOCATOR_LOG_LEVEL`, `DEBUG`

## Contributing
Contributions are welcome. Please open an issue or PR.

## License
This project is licensed; see the [LICENSE](LICENSE) file for details.
