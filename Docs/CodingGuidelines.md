# Agones Pub/Sub Allocator - Coding Guidelines

This document outlines the coding standards, architectural patterns, and best practices for the Agones Pub/Sub Allocator service.

## Table of Contents
1. [Project Overview](#project-overview)
2. [Project Structure](#project-structure)
3. [Code Organization](#code-organization)
4. [Configuration Management](#configuration-management)
5. [Error Handling](#error-handling)
6. [Logging Standards](#logging-standards)
7. [Testing Guidelines](#testing-guidelines)
8. [Documentation](#documentation)
9. [CI/CD](#cicd)

---

## Project Overview

The Agones Pub/Sub Allocator is a Kubernetes-based service that handles game server allocations via Google Cloud Pub/Sub. It listens for allocation requests, creates Agones GameServer allocations, and publishes the results back to Pub/Sub.

### Core Components

- **Kubernetes Client**: Interacts with the Kubernetes API for GameServer allocations
- **Pub/Sub Subscriber**: Listens for allocation requests
- **Pub/Sub Publisher**: Sends allocation results back
- **Metrics**: Exposes Prometheus metrics for monitoring
- **Health Checks**: Provides liveness and readiness probes

### Key Architectural Components

```
┌─────────────────────────────────────┐
│       Pub/Sub Subscription          │
│    - Listens for allocation requests│
└───────────────────┬─────────────────┘
                    │
                    ▼
┌─────────────────────────────────────┐
│      Allocation Controller          │
│    - Processes incoming requests    │
│    - Manages allocation lifecycle   │
└───────────────────┬─────────────────┘
                    │
                    ▼
┌─────────────────────────────────────┐
│      Agones Allocation API          │
│    - Creates GameServer allocations │
└───────────────────┬─────────────────┘
                    │
                    ▼
┌─────────────────────────────────────┐
│       Pub/Sub Publisher             │
│    - Sends allocation results       │
└─────────────────────────────────────┘
```

---

## Project Structure

```
.
├── cmd/
│   └── main.go           # Application entry point
├── queues/               # Queue implementations
│   └── pubsub/           # Google Cloud Pub/Sub implementation
├── allocator/            # Core allocation logic
│   ├── controller.go     # Main allocation controller
│   ├── types.go          # Core types and interfaces
│   └── errors.go         # Custom error types
├── metrics/              # Prometheus metrics
│   ├── metrics.go        # Metrics definition
│   └── middleware.go     # HTTP middleware
├── health/               # Health checks
│   ├── server.go         # Health check HTTP server
│   └── probes.go         # Liveness and readiness probes
├── config/               # Configuration management
├── deployments/          # Kubernetes manifests
│   ├── base/             # Kustomize base
│   ├── overlays/         # Environment-specific configs
│   └── examples/         # Example configurations
├── docs/                 # Documentation
├── scripts/              # Utility scripts
├── test/                 # Test files
├── .github/              # GitHub workflows
├── go.mod                # Go module definition
├── go.sum                # Go module checksums
└── README.md             # Project documentation
```

## Code Organization

### `/queues/pubsub`

Google Cloud Pub/Sub implementation for message queuing.

**Key Components:**
- `subscriber.go`: Handles incoming allocation requests
- `publisher.go`: Handles publishing allocation results
- `message.go`: Message format definitions
- `types.go`: Queue interface and types

### `/allocator`

Core allocation logic for handling GameServer allocations.

**Key Components:**
- `controller.go`: Main allocation controller
- `types.go`: Core types and interfaces
- `errors.go`: Custom error types

### `/metrics`

Prometheus metrics collection and exposure.

**Key Components:**
- `metrics.go`: Metrics definition and collection
- `middleware.go`: HTTP middleware for metrics

### `/health`

Health check endpoints and logic.

**Key Components:**
- `server.go`: Health check HTTP server
- `probes.go`: Liveness and readiness probes

---

## Configuration Management

### Environment Variables

All configuration should be done through environment variables with sensible defaults. Use the following naming convention:

```
ALLOCATOR_<SECTION>_<NAME>=<value>
```

**Required Configuration:**

```
# Pub/Sub Configuration
ALLOCATOR_PUBSUB_PROJECT_ID=your-project-id
ALLOCATOR_PUBSUB_SUBSCRIPTION=allocator-requests
ALLOCATOR_PUBSUB_TOPIC=allocator-responses

# Kubernetes Configuration
ALLOCATOR_KUBECONFIG=~/.kube/config  # Optional, for local development

# Metrics Configuration
ALLOCATOR_METRICS_PORT=8080

# Logging
ALLOCATOR_LOG_LEVEL=info  # debug, info, warn, error
```

### Configuration Loading

Use the following pattern for configuration:

```go
type Config struct {
    PubSub struct {
        ProjectID    string `envconfig:"PUBSUB_PROJECT_ID" required:"true"`
        Subscription string `envconfig:"PUBSUB_SUBSCRIPTION" required:"true"`
        Topic        string `envconfig:"PUBSUB_TOPIC" required:"true"`
    }
    Kubeconfig string `envconfig:"KUBECONFIG" default:""`
    LogLevel  string `envconfig:"LOG_LEVEL" default:"info"`
    Metrics   struct {
        Port int `envconfig:"METRICS_PORT" default:"8080"`
    }
}

func Load() (*Config, error) {
    var cfg Config
    if err := envconfig.Process("ALLOCATOR", &cfg); err != nil {
        return nil, fmt.Errorf("failed to load config: %w", err)
    }
    return &cfg, nil
}
```

## Error Handling

### Error Types

Define custom error types for different error scenarios:

```go
// Package errors defines custom error types for the allocator
package errors

type AllocationError struct {
    Code    string
    Message string
    Err     error
}

func (e *AllocationError) Error() string {
    if e.Err != nil {
        return fmt.Sprintf("%s: %v", e.Message, e.Err)
    }
    return e.Message
}

func (e *AllocationError) Unwrap() error {
    return e.Err
}

// Common error types
var (
    ErrInvalidRequest = &AllocationError{Code: "invalid_request", Message: "invalid request"}
    ErrAllocationFailed = &AllocationError{Code: "allocation_failed", Message: "failed to allocate game server"}
)
```

### Error Handling Pattern

Use the following pattern for error handling:

```go
func (c *Controller) HandleAllocation(ctx context.Context, req *AllocationRequest) (*AllocationResponse, error) {
    if err := validateRequest(req); err != nil {
        return nil, fmt.Errorf("invalid allocation request: %w", err)
    }
    
    // ... allocation logic ...
    
    if err := c.allocateGameServer(ctx, req); err != nil {
        return nil, &errors.AllocationError{
            Code:    "allocation_failed",
            Message: "failed to allocate game server",
            Err:     err,
        }
    }
    
    return &AllocationResponse{/* ... */}, nil
}
```

## Logging Standards

### Log Levels

Use the following log levels consistently:

- **DEBUG**: Detailed information for debugging purposes
- **INFO**: General operational information
- **WARN**: Non-critical issues that don't prevent the application from functioning
- **ERROR**: Errors that prevent a specific operation from completing
- **FATAL**: Critical errors that require the application to stop

### Logging Best Practices

1. **Structured Logging**: Use structured logging with key-value pairs
2. **Context**: Include relevant context in log messages
3. **Error Wrapping**: Always wrap errors with additional context
4. **Sensitive Data**: Never log sensitive information

Example:
```go
import "github.com/rs/zerolog/log"

// Good
log.Info().
    Str("request_id", reqID).
    Str("allocation_id", allocID).
    Msg("processing allocation request")

// Bad - missing context
log.Info("processing allocation")
```

## Testing Guidelines

### Unit Tests

- Place unit tests in the same package as the code being tested
- Use the standard `testing` package
- Follow the table-driven test pattern
- Use test helpers for common test setup/teardown

Example:
```go
func TestAllocateGameServer(t *testing.T) {
    tests := []struct {
        name    string
        req     *AllocationRequest
        want    *AllocationResponse
        wantErr bool
    }{
        {
            name: "valid request",
            req:  &AllocationRequest{/* ... */},
            want: &AllocationResponse{/* ... */},
        },
        // more test cases...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            c := &Controller{/* ... */}
            got, err := c.AllocateGameServer(context.Background(), tt.req)
            if (err != nil) != tt.wantErr {
                t.Errorf("AllocateGameServer() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("AllocateGameServer() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Integration Tests

- Place in `test/integration` directory
- Use build tags to separate from unit tests
- Test against a real Kubernetes cluster or use a test environment

## Documentation

### Code Documentation

- Document all exported functions, types, and packages
- Follow Go doc conventions
- Include examples where helpful

### Project Documentation

- Keep `README.md` up to date
- Add new features to the appropriate documentation in `docs/`
- Include architecture diagrams for complex systems
- Document all configuration options

## CI/CD

### GitHub Actions

- Use GitHub Actions for CI/CD
- Include the following workflows:
  - `build.yml`: Run tests on pull requests
  - `release.yml`: Create releases and publish images
  - `deploy.yml`: Deploy to different environments

### Container Images

- Use multi-stage builds
- Keep images small and secure
- Use semantic versioning for releases
- Scan for vulnerabilities in CI

### Kubernetes Manifests

- Use Kustomize for environment-specific configurations
- Include resource limits and requests
- Configure liveness and readiness probes
- Set up proper RBAC rules

## Performance Considerations

- Use connection pooling for Kubernetes and Pub/Sub clients
- Implement proper context timeouts
- Monitor and optimize memory usage
- Profile CPU and memory usage under load

## Security Best Practices

- Follow the principle of least privilege
- Use Kubernetes service accounts with minimal permissions
- Encrypt sensitive data at rest and in transit
- Regularly update dependencies
- Scan for vulnerabilities in dependencies

## Monitoring and Observability

- Expose Prometheus metrics
- Use structured logging
- Implement distributed tracing
- Set up alerts for critical errors
- Monitor resource usage and performance metrics

    _, err := Cli().GetAll(ctx, q, &results)
    if err != nil {
        log.Error().Err(err).Msg("failed to list MyEntity by OwnerID")
        return nil, err
    }
    return results, nil
}
```

#### 3. Query Operators and FilterField

All datastore queries MUST use `FilterField` (not the deprecated `Filter` method).

**FilterField Signature:**
```go
FilterField(fieldName, operator string, value interface{}) *Query
```

**Supported Operators:**
- `"="` - Equal to
- `"!="` - Not equal to
- `">"` - Greater than
- `"<"` - Less than
- `">="` - Greater than or equal to
- `"<="` - Less than or equal to
- `"in"` - Value in array (requires `[]interface{}` as value)
- `"not-in"` - Value not in array (requires `[]interface{}` as value)

**Examples:**
```go
// Simple equality
q := datastore.NewQuery("MyEntity").FilterField("Status", "=", "active")

// Comparison operators
q := datastore.NewQuery("MyEntity").FilterField("Score", ">=", 100)

// Multiple filters (AND'ed together)
q := datastore.NewQuery("MyEntity").
    FilterField("OwnerID", "=", ownerID).
    FilterField("Status", "=", "active").
    FilterField("CreatedAt", ">=", startTime)

// IN operator (note the []interface{} type)
q := datastore.NewQuery("MyEntity").
    FilterField("Category", "in", []interface{}{"cat1", "cat2", "cat3"})

// NOT-IN operator
q := datastore.NewQuery("MyEntity").
    FilterField("Status", "not-in", []interface{}{"deleted", "archived"})

// Field names with special characters (use strconv.Quote or %q)
import "strconv"
fieldName := strconv.Quote("field with spaces")
q := datastore.NewQuery("MyEntity").FilterField(fieldName, "=", value)
```

**Important Notes:**
- Multiple `FilterField` calls are AND'ed together
- For OR operations, you need multiple queries
- Field names with spaces, quotes, or operators should be quoted using `strconv.Quote` or `fmt.Sprintf("%q", fieldName)`
- The `in` and `not-in` operators require `[]interface{}` type for the value parameter

#### 4. Handling Map Fields (Advanced Pattern)

When you need to store map data in Datastore (which doesn't natively support maps), use the dual-representation pattern from `CharacterData`:

```go
type MyEntity struct {
    ID string
    // JSON-facing map (ignored by datastore)
    MetadataMap map[string]string `json:"metadata" datastore:"-"`
    // Datastore-facing slice (hidden from JSON)
    Metadata []KeyValuePair `json:"-"`
}

type KeyValuePair struct {
    Key   string `datastore:",noindex"`
    Value string `datastore:",noindex"`
}

// Convert maps to slices before save
func (e *MyEntity) populateSlicesFromMaps() {
    e.Metadata = e.Metadata[:0]
    for k, v := range e.MetadataMap {
        e.Metadata = append(e.Metadata, KeyValuePair{Key: k, Value: v})
    }
}

// Convert slices to maps after load
        delete(e.MetadataMap, k)
    }
    for _, kv := range e.Metadata {
        e.MetadataMap[kv.Key] = kv.Value
    }
}

// Call in Upsert before saving
func (e *MyEntity) Upsert() error {
    e.populateSlicesFromMaps()
    // ... rest of upsert logic
}

// Call in GetByID after loading
func (e *MyEntity) GetByID() error {
    // ... get logic
    e.populateMapsFromSlices()
    return nil
}
```

---

## Web Framework & Routing

### Entry Point: `web/web.go`

The `web` package provides the main entry point for the web server.

**Key Functions:**

#### `Start(r *gin.Engine) *gin.Engine`
Main initialization function that:
1. Calls `HandleRoutes()` to register all routes
2. Registers the `/events` WebSocket endpoint
3. Returns the configured engine

#### `HandleRoutes(r *gin.Engine, staticDir string) *gin.Engine`
Registers all application routes:
1. Serves root index.html
2. Calls `handlers.RegisterRoutes(r)` for API routes
3. Registers redirect routes via `GotoRedirects()`
4. Serves static files (in production mode)

#### `GetStaticFiles(staticDir string) (map[string]string, error)`
Walks the static directory and builds a map of URL paths to file paths.

### Route Registration: `handlers/routes.go`

**Central Route Registration Pattern:**

All API routes MUST be registered in `handlers/routes.go` via the `RegisterRoutes()` function.

```go
func RegisterRoutes(r *gin.Engine) {
    // Create route groups
    authRoutes := r.Group("/api/auth")
    AuthRoutes(authRoutes)
    
    charRoutes := r.Group("/api/characters")
    CharacterRoutes(charRoutes)
    
    serverRoutes := r.Group("/api/serverx")
    ServerRoutes(serverRoutes)
}
```

**Pattern:**
1. Create a route group with `r.Group("/api/domain")`
2. Pass the group to a domain-specific route function (e.g., `CharacterRoutes()`)
3. Define the actual routes in domain-specific files

### Utility Functions in `routes.go`

```go
// Render error with proper status code and not-found detection
func renderError(c *gin.Context, e error, statusCode int, message string)

// Render success response
func renderSuccess(c *gin.Context)

// Render final result (error or success)
func renderFinal(c *gin.Context, err error, message string)

// Render content with optional key
func renderFinalContent(c *gin.Context, content interface{}, key string, err error)
```

**Note:** These are legacy functions. Prefer using `helpers.RenderError()`, `helpers.RenderSuccess()`, and `helpers.RenderContent()` for consistency.

---

## Handler Patterns

### Domain-Specific Handler Files

Each major API subsection has its own file in `handlers/`:
- `characters.go` - Character management endpoints
- `serverx.go` - Server authoritative endpoints
- `auth.go` - Authentication endpoints

### Handler File Structure

Each handler file should follow this pattern:

```go
package handlers

import (
	"net/http"
	"starxapi/data"
	"starxapi/helpers"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// 1. Define route registration function
func MyDomainRoutes(r *gin.RouterGroup) {
	r.GET("", ListHandler)
	r.GET("/:id", GetByIDHandler)
	r.POST("", CreateHandler)
	r.PUT("/:id", UpdateHandler)
	r.DELETE("/:id", DeleteHandler)
}

// 2. Implement handlers
func ListHandler(ctx *gin.Context) {
	// Implementation
}

func GetByIDHandler(ctx *gin.Context) {
	// Implementation
}

// ... etc
```

### Standard Handler Pattern

```go
func MyHandler(ctx *gin.Context) {
    // 1. Extract and validate parameters
    id := ctx.Param("id")
    if len(id) < 1 {
        helpers.RenderError(ctx, http.StatusBadRequest, "id cannot be empty")
        return
    }
    
    // 2. Check authorization if needed
    oid := helpers.GetOwner(ctx)
    if oid == "" {
        helpers.RenderError(ctx, http.StatusUnauthorized, "missing userID header")
        return
    }
    
    // 3. Bind JSON body if needed
    var entity data.MyEntity
    if err := ctx.ShouldBindJSON(&entity); err != nil {
        helpers.RenderError(ctx, http.StatusBadRequest, "bad json format cannot bind")
        return
    }
    
    // 4. Perform data operations
    if err := entity.Upsert(); err != nil {
        log.Error().Err(err).Msg("failed to upsert entity")
        helpers.RenderError(ctx, http.StatusInternalServerError, "couldn't save entity")
        return
    }
    
    // 5. Return response
    helpers.RenderSuccess(ctx)
    // OR
    helpers.RenderContent(ctx, entity)
}
```

### Middleware Pattern

```go
        // 2. Validate
        if !isValid(token) {
            helpers.RenderError(c, http.StatusUnauthorized, "invalid token")
            c.Abort()
            return
        }
        
        // 3. Set context values
        c.Set("user_id", extractUserID(token))
        
        // 4. Continue
        c.Next()
    }
}

// Usage in route registration
func MyRoutes(r *gin.RouterGroup) {
    protected := r.Group("")
    protected.Use(MyAuthMiddleware())
    protected.GET("/protected", ProtectedHandler)
}
```

---

## Configuration Management

### Accessing Configuration

**Always use `config.Get()` to access configuration:**

```go
cfg := config.Get()
if cfg == nil {
    // Handle error
}

// Access fields
projectID := cfg.GoogleProjectID
apiKey := cfg.FirebaseAPIKey
```

### Configuration Fields

The `AppConfig` struct contains:
- `DataStoreName` - Datastore database name
- `PubsubTopic` - Pub/Sub topic name
- `Subscription` - Pub/Sub subscription name
- `GoogleProjectID` - GCP project ID
- `OAuthClientID` - OAuth client ID
- `FrontendEndpoint` - Frontend URL
- `QueueType` - Queue type (default: "pubsub")
- `FirebaseAPIKey` - Firebase API key
- `FirebaseAuthDomain` - Firebase auth domain
- `DevBearerToken` - Development bypass token
- `SteamAPIKey` - Steam API key
- `ServerSecret` - Server provisioning secret

### Thread Safety

The config package uses `sync.RWMutex` for thread-safe access:
- `LoadConfig()` uses write lock
- `Get()` uses read lock

---

## Static File Serving

### Directory Structure

Static files are served from the `static/` directory, which acts as the web root.

**Important:** When linking to static files in HTML, **DO NOT** prefix paths with `static/`.

### Correct HTML Linking

```html
<!-- CORRECT -->
<link rel="stylesheet" href="/css/style.css">
<script src="/js/app.js"></script>
<img src="/images/logo.png">

<!-- INCORRECT -->
<link rel="stylesheet" href="/static/css/style.css">
```

### How It Works

1. `web.GetStaticFiles()` walks the `static/` directory
2. Creates URL mappings without the `static/` prefix
3. `web.HandleRoutes()` registers each file with Gin
4. `.html` files are accessible with or without the extension

**Example:**
- File: `static/dashboard/index.html`
- Accessible at: `/dashboard/index.html` OR `/dashboard/index` OR `/dashboard`

### Development Mode

When `IS_DEV=true` environment variable is set, static file serving is disabled to allow for hot-reloading via external tools.

---

## WebSocket & Events

### WebSocket Endpoint

The WebSocket system is available at `/events` and is registered in `web/web.go`:

```go
r.GET("/events", EventHandler)
```

### WebSocket Message Format

All WebSocket messages use JSON with this structure:

```go
type SocketMessage struct {
    MsgType string `json:"MsgType"`  // Message type: "auth", "token", "heartbeat", "message"
    Action  string `json:"Action"`   // Action within type
    Message string `json:"Message"`  // Payload
}
```

### Authentication Flow

1. Client connects to `/events`
2. Server sends auth prompt:
   ```json
   {"MsgType": "auth", "Action": "authenticate", "Message": "client not authenticated, send authentication token"}
   ```
3. Client sends token:
   ```json
   {"MsgType": "auth", "Action": "authenticate", "Message": "<firebase_jwt_token>"}
   ```
4. Server validates and responds:
   ```json
   {"MsgType": "auth", "Action": "authorization", "Message": "success"}
   ```

### Message Types

- **`auth`**: Authentication messages
- **`token`**: Token-related messages
- **`heartbeat`**: Keep-alive pings
- **`message`**: General application messages

### Socket Manager

The `wsock.SocketManager` manages all WebSocket connections:
- Stored in Gin context as `socketManager`
- Tracks clients by identity
- Handles heartbeats and disconnections
- Manages token expiry

### Pub/Sub Integration

The WebSocket system integrates with Google Cloud Pub/Sub for message distribution (stubbed for future capabilities).

---

## Code Style Guidelines

### General Go Conventions

1. **Follow standard Go formatting**: Use `gofmt` or `goimports`
2. **Package naming**: Short, lowercase, single-word names
3. **Exported vs unexported**: Use PascalCase for exported, camelCase for unexported
4. **Error handling**: Always check errors, never ignore them

### Naming Conventions

#### Variables
```go
// Good
userID := "123"
accountData := &data.Account{}
cfg := config.Get()

// Avoid
user_id := "123"  // No snake_case
ud := &data.Account{}  // Too abbreviated in broad scope
```

#### Functions
```go
// Good
func GetAccountByID(id string) (*Account, error)
func (a *Account) Update() error
func CreateCharacter(char *CharacterData) error

// Avoid
```

#### Constants
```go
// Good
const MaxRetries = 3
const DefaultTimeout = 30 * time.Second

// Avoid
const max_retries = 3  // No snake_case
```

### Import Organization

Group imports in this order:
1. Standard library
2. External dependencies
3. Internal packages

```go
import (
    "context"
    "errors"
    "time"
    
    "cloud.google.com/go/datastore"
    "github.com/gin-gonic/gin"
    "github.com/rs/zerolog/log"
    
    "starxapi/config"
    "starxapi/data"
    "starxapi/helpers"
)
```

### Function Length

- Keep functions focused and concise
- Extract complex logic into helper functions
- Aim for functions under 50 lines when possible

### Comments

```go
// Package-level comment
// Describes the package purpose

// Exported function comment
// Describes what the function does, parameters, and return values
func GetAccountByID(id string) (*Account, error) {
    // Implementation comments for complex logic
}

// Unexported functions should have comments if non-obvious
func extractBearerToken(gc *gin.Context) (string, bool) {
    // ...
}
```

---

## Error Handling

### Data Layer Errors

```go
// Always log errors with context
if err := entity.Update(); err != nil {
    log.Error().Err(err).Msg("failed to update entity")
    return err
}

// Check for not-found specifically
account, err := data.GetAccountByID(id)
if err != nil {
    if data.IsNotFound(err) {
        // Handle not found case
        return nil, errors.New("account not found")
    }
    log.Error().Err(err).Msg("failed to get account")
    return nil, err
}
```

### Handler Errors

```go
// Use helpers for consistent error responses
if err := entity.Upsert(); err != nil {
    log.Error().Err(err).Msg("failed to upsert entity")
    helpers.RenderError(ctx, http.StatusInternalServerError, "couldn't save entity")
    return
}

// Handle not-found errors
entity := &data.MyEntity{ID: id}
if err := entity.GetByID(); err != nil {
    if data.IsNotFound(err) {
        helpers.RenderError(ctx, http.StatusNotFound, "entity not found")
        return
    }
    helpers.RenderError(ctx, http.StatusInternalServerError, "failed to retrieve entity")
    return
}
```

### Error Messages

- **User-facing**: Generic, don't expose internals
    - Good: `"failed to save entity"`
    - Bad: `"datastore put failed: connection timeout"`

- **Logs**: Detailed, include context
    - Good: `log.Error().Err(err).Msgf("failed to upsert character %s", id)`
    - Bad: `log.Error().Msg("error")`

---

## Logging Standards

### Use Zerolog

The project uses `github.com/rs/zerolog/log` for structured logging.

### Log Levels

```go
// Debug: Detailed information for debugging
log.Debug().Msgf("Creating Account with id: %s", id)

// Info: General informational messages
log.Info().Msgf("Authorized account: %s", ident)

// Warn: Warning messages for unexpected but handled situations
log.Warn().Err(err).Msg("Client closed connection with error")

// Error: Error messages for failures
log.Error().Err(err).Msg("failed to create Account")

// Fatal: Critical errors that require application shutdown
log.Fatal().Err(err).Msg("failed to create datastore client")
```

### Logging Patterns

```go
// Include error with .Err()
log.Error().Err(err).Msg("failed to update entity")

// Include context with fields
log.Info().
    Str("user_id", userID).
    Str("character_id", charID).
    Msg("character created")

// Use Msgf for formatted messages
log.Debug().Msgf("Upserting Character with id: %s", c.ID)
```

### What to Log

**DO log:**
- All errors with context
- Authentication events (success/failure)
- Important state changes (create, update, delete)
- WebSocket connections/disconnections

**DON'T log:**
- Sensitive data (passwords, tokens, API keys)
- Excessive debug info in production
- Personal information without anonymization

---

## Best Practices Summary

### Data Layer
- ✅ Use `data.Cli()` for datastore client
- ✅ Implement standard CRUD methods on models
- ✅ Use `datastore.NameKey()` for entity keys
- ✅ Log all database operations at Debug level
- ✅ Return `datastore.ErrNoSuchEntity` for not-found cases
- ✅ Use map-to-slice conversion for complex data structures

### Handlers
- ✅ Register all routes in `handlers/routes.go`
- ✅ Group related handlers in domain-specific files
- ✅ Use `helpers.RenderError()`, `helpers.RenderSuccess()`, `helpers.RenderContent()`
- ✅ Validate input before processing
- ✅ Check authorization using `helpers.GetOwner()` or `helpers.IsOwner()`
- ✅ Use `ctx.ShouldBindJSON()` for request body parsing
- ✅ Log errors before returning error responses

### Configuration
- ✅ Access config via `config.Get()`
- ✅ Never hardcode configuration values
- ✅ Use relative paths from web root

### WebSocket
- ✅ Use `SocketMessage` struct for all messages
- ✅ Implement authentication before allowing other messages
- ✅ Handle disconnections gracefully
- ✅ Use heartbeats to detect dead connections

### General
- ✅ Use structured logging with zerolog
- ✅ Handle all errors explicitly
- ✅ Follow Go naming conventions
- ✅ Keep functions focused and concise
- ✅ Write comments for exported functions
- ✅ Use meaningful variable names

---

## Quick Reference

### Creating a New API Endpoint

1. **Create handler function** in appropriate file under `handlers/`
2. **Register route** in `handlers/routes.go` via route group
3. **Implement handler** following standard pattern
4. **Add data model** in `data/` if needed
5. **Test endpoint** with proper authentication

### Creating a New Data Model

1. **Define struct** in `data/` package
2. **Implement CRUD methods**: `Create()`, `GetByID()`, `Update()`, `Delete()`
3. **Add query methods** as needed (e.g., `ListByOwner()`)
4. **Handle map fields** with dual representation if needed
5. **Add logging** to all database operations

### Adding a Configuration Value

1. **Add field** to `AppConfig` struct in `config/config.go`
2. **Load from environment** in `LoadConfig()` function
3. **Add validation** if required
4. **Access via** `config.Get().YourField`

---

**Last Updated**: 2025-10-01  
**Version**: 1.0
