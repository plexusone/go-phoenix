# CLAUDE.md - go-phoenix SDK Development Guide

This document defines patterns and conventions for developing the go-phoenix SDK, based on established patterns from go-comet-ml-opik and go-elevenlabs.

## Project Overview

go-phoenix is a Go SDK for the [Arize Phoenix](https://github.com/Arize-ai/phoenix) AI observability platform. It provides:

- OpenAPI-generated client code via [ogen](https://github.com/ogen-go/ogen)
- High-level wrapper for ergonomic Go API
- Support for tracing, datasets, experiments, prompts, and annotations

## Project Structure

```
go-phoenix/
├── openapi/
│   └── openapi.json          # Phoenix OpenAPI spec (from upstream)
├── internal/api/
│   └── oas_*_gen.go          # ogen-generated code (DO NOT EDIT)
├── llmops/                    # omniobserve integration (optional)
│   ├── provider.go           # llmops.Provider implementation
│   ├── trace.go              # Trace adapter
│   └── span.go               # Span adapter
├── cmd/openapi-convert/       # OpenAPI 3.1 → 3.0 converter
├── ogen.yml                   # ogen configuration
├── generate.sh                # Code generation script
├── client.go                  # Main client with auth wrapper
├── options.go                 # Functional options (WithXxx pattern)
├── errors.go                  # Sentinel errors + APIError type
├── config.go                  # Environment-based configuration
├── project.go                 # Project operations
├── span.go                    # Span operations
├── dataset.go                 # Dataset operations
├── experiment.go              # Experiment operations
├── prompt.go                  # Prompt operations
├── Makefile                   # Build/test/lint targets
├── .golangci.yaml             # Linter configuration
└── examples/
    └── basic/main.go          # Usage examples
```

## Code Generation

### OpenAPI Spec

The OpenAPI spec is sourced from Phoenix upstream:

- Location: `openapi/openapi.json`
- Source: `/Users/johnwang/go/src/github.com/Arize-ai/phoenix/schemas/openapi.json`
- Version: OpenAPI 3.1.0

### ogen Configuration

`ogen.yml` configures code generation:

```yaml
generator:
  ignore_not_implemented:
    - "unsupported content types"
```

### Running Generation

```bash
./generate.sh
```

This script:

1. Runs ogen to generate Go code
2. Applies any necessary fixes (e.g., jx.Raw comparison issues)
3. Runs `go mod tidy`
4. Verifies the build compiles

## SDK Patterns

### Client Structure

```go
type Client struct {
    config    *Config
    apiClient *api.Client  // ogen-generated client
}

func NewClient(opts ...Option) (*Client, error) {
    // 1. Apply options to defaults
    // 2. Load config from env/file
    // 3. Validate configuration
    // 4. Create authHTTPClient wrapper
    // 5. Create ogen client
    // 6. Return wrapped client
}
```

### Authentication

Wrap http.Client to inject auth headers:

```go
type authHTTPClient struct {
    client *http.Client
    apiKey string
}

func (c *authHTTPClient) Do(req *http.Request) (*http.Response, error) {
    if c.apiKey != "" {
        req.Header.Set("Authorization", "Bearer " + c.apiKey)
    }
    req.Header.Set("X-Phoenix-SDK-Version", Version)
    req.Header.Set("X-Phoenix-SDK-Lang", "go")
    return c.client.Do(req)
}
```

### Functional Options

Use the functional options pattern for configuration:

```go
type Option func(*clientOptions)

func WithAPIKey(apiKey string) Option {
    return func(o *clientOptions) {
        o.apiKey = apiKey
    }
}

func WithBaseURL(url string) Option {
    return func(o *clientOptions) {
        o.baseURL = url
    }
}
```

### Configuration Priority

Load config in this order (highest to lowest priority):

1. Explicitly set values (via options)
2. Environment variables
3. Config file (~/.phoenix.config)
4. Default values

### Environment Variables

```go
const (
    EnvAPIKey      = "PHOENIX_API_KEY"
    EnvURL         = "PHOENIX_URL"
    EnvProjectName = "PHOENIX_PROJECT_NAME"
)
```

### Error Handling

Define sentinel errors for common cases:

```go
var (
    ErrMissingURL      = errors.New("phoenix: missing API URL")
    ErrMissingAPIKey   = errors.New("phoenix: missing API key")
    ErrProjectNotFound = errors.New("phoenix: project not found")
    ErrTraceNotFound   = errors.New("phoenix: trace not found")
)

type APIError struct {
    StatusCode int
    Message    string
    Details    string
}

func IsNotFound(err error) bool { ... }
func IsUnauthorized(err error) bool { ... }
func IsRateLimited(err error) bool { ... }
```

### High-Level Wrappers

Wrap ogen-generated methods with ergonomic Go APIs:

```go
// Instead of exposing raw ogen types:
// resp, err := c.apiClient.ListProjectsV1ProjectsGet(ctx, params)

// Provide clean wrapper:
func (c *Client) ListProjects(ctx context.Context, opts ...ListOption) ([]*Project, error) {
    params := api.ListProjectsV1ProjectsGetParams{
        Limit: api.NewOptInt(opts.limit),
    }
    resp, err := c.apiClient.ListProjectsV1ProjectsGet(ctx, params)
    if err != nil {
        return nil, err
    }
    return convertProjects(resp.Data), nil
}
```

### Exposing Raw API

Always provide access to the underlying ogen client:

```go
func (c *Client) API() *api.Client {
    return c.apiClient
}
```

## Phoenix API Domains

The SDK should cover these API domains:

| Domain | Endpoints | Priority |
|--------|-----------|----------|
| Projects | `/v1/projects` | High |
| Traces | `/v1/traces`, `/v1/projects/{}/spans` | High |
| Spans | `/v1/spans`, `/v1/span_annotations` | High |
| Datasets | `/v1/datasets` (CRUD, upload, export) | High |
| Experiments | `/v1/experiments`, runs, evaluations | Medium |
| Prompts | `/v1/prompts`, versions, tags | Medium |
| Annotations | `/v1/annotation_configs`, trace/span/session annotations | Medium |
| Users | `/v1/users` | Low |

## Testing

### Unit Tests

```go
func TestClient_ListProjects(t *testing.T) {
    // Use httptest.Server for mocking
}
```

### Integration Tests

Tag with build constraint:

```go
//go:build integration

func TestIntegration_ListProjects(t *testing.T) {
    // Requires PHOENIX_API_KEY and PHOENIX_URL
}
```

## Build Commands

```bash
make test      # Run tests with race detection
make lint      # Run golangci-lint
make build     # Build all packages
./generate.sh  # Regenerate API client
```

## Dependencies

Core dependencies:

- `github.com/ogen-go/ogen` - OpenAPI code generator
- `github.com/go-faster/jx` - JSON handling (ogen dependency)
- `github.com/go-faster/errors` - Error handling (ogen dependency)
- `github.com/google/uuid` - UUID generation
- `go.opentelemetry.io/otel` - OpenTelemetry (for metrics/tracing in client)

## Versioning

Follow semantic versioning. Update `Version` constant in `client.go`:

```go
const Version = "0.1.0"
```

## omniobserve Integration (Hybrid Mode)

The SDK supports a hybrid architecture for integration with `omniobserve/llmops`:

### Architecture

```
go-phoenix/
├── *.go                      # Standalone SDK (no omniobserve dep)
└── llmops/
    ├── provider.go           # Implements llmops.Provider
    ├── trace.go              # Trace adapter
    └── span.go               # Span adapter
```

### Two Usage Modes

**1. Standalone (no omniobserve dependency):**

```go
import "github.com/plexusone/phoenix-go"

client, err := phoenix.NewClient(
    phoenix.WithURL("http://localhost:6006"),
    phoenix.WithAPIKey("..."),
)
projects, _, _ := client.ListProjects(ctx)
```

**2. Via omniobserve/llmops (unified interface):**

```go
import (
    "github.com/plexusone/omniobserve/llmops"
    _ "github.com/plexusone/phoenix-go/llmops"  // Register provider
)

provider, err := llmops.Open("phoenix",
    llmops.WithEndpoint("http://localhost:6006"),
    llmops.WithAPIKey("..."),
)
ctx, trace, _ := provider.StartTrace(ctx, "my-trace")
defer trace.End()
```

### Key Points

- The `llmops/` subpackage imports omniobserve, creating an optional dependency
- The root package remains standalone with zero omniobserve coupling
- Provider registration happens via `init()` when the subpackage is imported
- The llmops adapter wraps the standalone SDK client

### Interface Implementation

The `llmops/` package implements:

- `llmops.Provider` - Main provider interface
- `llmops.Tracer` - Trace/span operations
- `llmops.Evaluator` - Evaluation and feedback
- `llmops.PromptManager` - Prompt operations (limited)
- `llmops.DatasetManager` - Dataset operations
- `llmops.ProjectManager` - Project operations

## Common Issues

### OpenAPI 3.1 vs 3.0

ogen may have issues with OpenAPI 3.1. If needed, convert to 3.0:

```bash
# See go-elevenlabs/cmd/openapi-convert for reference
```

### jx.Raw Comparison

ogen generates code that compares `jx.Raw` directly. Fix with:

```go
// Replace: if a.Input != b.Input {
// With:    if !bytes.Equal([]byte(a.Input), []byte(b.Input)) {
```

Apply fixes in `generate.sh`.
