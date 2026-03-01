# Release Notes v0.2.0

**Release Date:** March 1, 2026

## Highlights

- Organization rename from agentplexus to plexusone
- Repository rename from go-phoenix to phoenix-go

## Breaking Changes

### Module Path Changed

The Go module path has changed from `github.com/agentplexus/go-phoenix` to `github.com/plexusone/phoenix-go`.

**Before:**

```go
import phoenix "github.com/agentplexus/go-phoenix"
import phoenixotel "github.com/agentplexus/go-phoenix/otel"
import _ "github.com/agentplexus/go-phoenix/llmops"
```

**After:**

```go
import phoenix "github.com/plexusone/phoenix-go"
import phoenixotel "github.com/plexusone/phoenix-go/otel"
import _ "github.com/plexusone/phoenix-go/llmops"
```

### Upgrade Guide

Update all import statements in your code:

```bash
# Using sed (macOS)
find . -name "*.go" -exec sed -i '' 's|github.com/agentplexus/go-phoenix|github.com/plexusone/phoenix-go|g' {} +

# Using sed (Linux)
find . -name "*.go" -exec sed -i 's|github.com/agentplexus/go-phoenix|github.com/plexusone/phoenix-go|g' {} +
```

Then update your dependencies:

```bash
go get github.com/plexusone/phoenix-go@v0.2.0
go mod tidy
```

## Dependencies

- Upgraded `github.com/plexusone/omniobserve` from v0.5.0 to v0.7.0
