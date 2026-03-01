# Go SDK for Arize Phoenix

[![Build Status][build-status-svg]][build-status-url]
[![Lint Status][lint-status-svg]][lint-status-url]
[![Go Report Card][goreport-svg]][goreport-url]
[![Docs][docs-godoc-svg]][docs-godoc-url]
[![Visualization][viz-svg]][viz-url]
[![License][license-svg]][license-url]

Go SDK for [Arize Phoenix](https://phoenix.arize.com/) - an open-source observability platform for LLM applications.

## Installation

```bash
go get github.com/plexusone/phoenix-go
```

## Quick Start

### Sending Traces with OTEL

```go
package main

import (
    "context"
    "log"

    phoenixotel "github.com/plexusone/phoenix-go/otel"
    "go.opentelemetry.io/otel/trace"
)

func main() {
    ctx := context.Background()

    // Register with Phoenix (sends traces to localhost:6006)
    tp, err := phoenixotel.Register(
        phoenixotel.WithProjectName("my-app"),
        phoenixotel.WithBatch(true),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer tp.Shutdown(ctx)

    // Create traces
    tracer := tp.Tracer("my-service")
    ctx, span := tracer.Start(ctx, "llm-call",
        trace.WithAttributes(
            phoenixotel.WithSpanKind(phoenixotel.SpanKindLLM),
            phoenixotel.WithModelName("gpt-4"),
            phoenixotel.WithInput("What is the capital of France?"),
        ),
    )
    // ... do work ...
    span.SetAttributes(phoenixotel.WithOutput("The capital of France is Paris."))
    span.End()
}
```

### Using the REST API Client

```go
package main

import (
    "context"
    "log"

    phoenix "github.com/plexusone/phoenix-go"
)

func main() {
    ctx := context.Background()

    client, err := phoenix.NewClient(
        phoenix.WithBaseURL("http://localhost:6006"),
    )
    if err != nil {
        log.Fatal(err)
    }

    // List projects
    projects, _, err := client.ListProjects(ctx)
    if err != nil {
        log.Fatal(err)
    }

    for _, p := range projects {
        log.Printf("Project: %s", p.Name)
    }
}
```

### Phoenix Cloud Configuration

```go
tp, err := phoenixotel.Register(
    phoenixotel.WithSpaceID("your-space-id"),  // From app.phoenix.arize.com/s/{space-id}
    phoenixotel.WithAPIKey("your-api-key"),
    phoenixotel.WithProjectName("my-project"),
)
```

Or via environment variables:

```bash
export PHOENIX_SPACE_ID=your-space-id
export PHOENIX_API_KEY=your-api-key
```

### Evaluating LLM Outputs

Metrics are defined in `omniobserve/llmops/metrics` and can be used with the Phoenix evaluator:

```go
import (
    "context"
    "fmt"
    "os"

    phoenix "github.com/plexusone/phoenix-go"
    "github.com/plexusone/phoenix-go/evals"
    "github.com/plexusone/omniobserve/llmops"
    "github.com/plexusone/omniobserve/llmops/metrics"
    "github.com/plexusone/omnillm"
)

func main() {
    ctx := context.Background()

    // Setup omnillm for LLM-based metrics
    llmClient, _ := omnillm.NewClient(omnillm.ClientConfig{
        Provider: omnillm.ProviderNameOpenAI,
        APIKey:   os.Getenv("OPENAI_API_KEY"),
    })
    defer llmClient.Close()
    llm := metrics.NewLLM(llmClient, "gpt-4o")

    // Setup Phoenix client
    phoenixClient, _ := phoenix.NewClient()

    // Create evaluator
    evaluator := evals.NewEvaluator(phoenixClient)

    // Run evaluation with metrics
    result, _ := evaluator.Evaluate(ctx, llmops.EvalInput{
        Input:   "What is the capital of France?",
        Output:  "The capital of France is London.",
        Context: []string{"Paris is the capital of France."},
        SpanID:  "your-span-id", // Optional: records results to Phoenix
    },
        metrics.NewHallucinationMetric(llm), // LLM-based
        metrics.NewExactMatchMetric(),       // Code-based
    )

    // result.Scores contains evaluation results
    for _, score := range result.Scores {
        fmt.Printf("%s: %.2f\n", score.Name, score.Score)
    }
}
```

**Available Metrics:**

| Metric | Type | Description |
|--------|------|-------------|
| `HallucinationMetric` | LLM | Detects unsupported claims |
| `RelevanceMetric` | LLM | Evaluates document relevance |
| `QACorrectnessMetric` | LLM | Checks answer correctness |
| `ToxicityMetric` | LLM | Detects harmful content |
| `ExactMatchMetric` | Code | Exact string comparison |
| `RegexMetric` | Code | Regex pattern matching |
| `ContainsMetric` | Code | Substring presence check |

## Environment Variables

| Variable | Description |
|----------|-------------|
| `PHOENIX_COLLECTOR_ENDPOINT` | Phoenix collector endpoint (defaults to Phoenix Cloud when PHOENIX_SPACE_ID is set) |
| `PHOENIX_SPACE_ID` | Space identifier for Phoenix Cloud (e.g., "johncwang" from app.phoenix.arize.com/s/johncwang) |
| `PHOENIX_PROJECT_NAME` | Project name for traces |
| `PHOENIX_API_KEY` | API key for authentication |
| `PHOENIX_CLIENT_HEADERS` | Additional headers (W3C Baggage format) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Fallback OTLP endpoint |

## Feature Parity with Python SDK

The sections below are ordered by typical usage: start with **phoenix-otel** to send traces, use **phoenix-client** to manage resources, then add **phoenix-evals** to evaluate outputs. These can be used independently—users may adopt any subset based on their needs.

| Feature | Python SDK | go-phoenix | Status |
|---------|:----------:|:----------:|--------|
| **phoenix-otel** | | | |
| Register tracer provider | :white_check_mark: | :white_check_mark: | Parity |
| OTLP HTTP exporter | :white_check_mark: | :white_check_mark: | Parity |
| OpenInference attributes | :white_check_mark: | :white_check_mark: | Parity |
| Environment variables | :white_check_mark: | :white_check_mark: | Parity |
| Batch processing | :white_check_mark: | :white_check_mark: | Parity |
| **phoenix-client (REST API)** | | | |
| Projects | :white_check_mark: | :white_check_mark: | Parity |
| Spans | :white_check_mark: | :white_check_mark: | Parity |
| Datasets | :white_check_mark: | :white_check_mark: | Parity |
| Experiments | :white_check_mark: | :white_check_mark: | Parity |
| Prompts | :white_check_mark: | :white_check_mark: | Parity |
| Annotations | :white_check_mark: | :white_check_mark: | Parity |
| Sessions | :white_check_mark: | :white_check_mark: | Parity |
| **phoenix-evals** | | | |
| LLM evaluators | :white_check_mark: | :white_check_mark: | Parity |
| Hallucination detection | :white_check_mark: | :white_check_mark: | Parity |
| Relevance scoring | :white_check_mark: | :white_check_mark: | Parity |
| Q&A correctness | :white_check_mark: | :white_check_mark: | Parity |
| Toxicity detection | :white_check_mark: | :white_check_mark: | Parity |
| Exact match | :white_check_mark: | :white_check_mark: | Parity |
| Regex matching | :white_check_mark: | :white_check_mark: | Parity |
| Custom templates | :white_check_mark: | :white_check_mark: | Parity |
| **Auto-instrumentation** | | | |
| OpenAI auto-instrument | :white_check_mark: | :x: | N/A (Go limitation) |
| Anthropic auto-instrument | :white_check_mark: | :x: | N/A (Go limitation) |

## OpenInference Span Kinds

The `otel` package provides constants for OpenInference span kinds:

- `SpanKindLLM` - LLM inference calls
- `SpanKindChain` - Chain/workflow operations
- `SpanKindTool` - Tool/function calls
- `SpanKindAgent` - Agent operations
- `SpanKindRetriever` - Document retrieval
- `SpanKindEmbedding` - Embedding generation
- `SpanKindReranker` - Reranking operations
- `SpanKindGuardrail` - Guardrail checks

## Integration with omniobserve

go-phoenix can be used as a provider for [omniobserve](https://github.com/plexusone/omniobserve):

```go
import (
    "github.com/plexusone/omniobserve/llmops"
    _ "github.com/plexusone/phoenix-go/llmops" // Register provider
)

func main() {
    provider, err := llmops.Open("phoenix",
        llmops.WithEndpoint("http://localhost:6006"),
    )
    // ...
}
```

### Feature Comparison

| Feature | Phoenix (Python) | go-phoenix | omniobserve/llmops | Tests | Notes |
|---------|:----------------:|:----------:|:------------------:|:-----:|-------|
| **Tracing** | | | | | |
| StartTrace | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | Via phoenix-otel |
| StartSpan | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | Via phoenix-otel |
| SetInput/Output | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | OpenInference attributes |
| SetModel/Provider | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | OpenInference attributes |
| SetUsage (tokens) | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | OpenInference attributes |
| AddFeedbackScore | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | Via OTEL events |
| TraceFromContext | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | |
| SpanFromContext | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | |
| Nested Spans | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | |
| Span Types | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | general, llm, tool, retrieval, agent, chain |
| Duration/Timing | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | |
| **Prompts** | | | | | |
| CreatePrompt | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | Use WithPromptModel/WithPromptProvider |
| GetPromptLatest | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | |
| GetPromptVersion | :white_check_mark: | :white_check_mark: | :white_check_mark: | | Via GetPrompt(name, versionID) |
| GetPromptByTag | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | Via GetPrompt(name, tagName) |
| ListPromptVersions | :white_check_mark: | :white_check_mark: | :x: | | |
| ListPrompts | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | |
| **Datasets** | | | | | |
| CreateDataset | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | |
| GetDataset | :white_check_mark: | :white_check_mark: | :white_check_mark: | | By name |
| GetDatasetById | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | |
| AddDatasetItems | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | |
| ListDatasets | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | |
| DeleteDataset | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | |
| **Experiments** | | | | | |
| CreateExperiment | :white_check_mark: | :white_check_mark: | :x: | | Not in omniobserve interface |
| RunExperiment | :white_check_mark: | :white_check_mark: | :x: | | Not in omniobserve interface |
| ListExperiments | :white_check_mark: | :white_check_mark: | :x: | | Not in omniobserve interface |
| **Projects** | | | | | |
| CreateProject | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | |
| GetProject | :white_check_mark: | :white_check_mark: | :white_check_mark: | | |
| ListProjects | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | |
| SetProject | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | |
| **Evaluation** | | | | | |
| Evaluate | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | Run metrics |
| AddFeedbackScore | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | Record results |
| **Annotations** | | | | | |
| CreateAnnotation | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | |
| ListAnnotations | :white_check_mark: | :white_check_mark: | :white_check_mark: | :white_check_mark: | |

**Running omniobserve/llmops tests:**

```bash
# Skip tests when no API key is set
go test -v ./llmops/

# Run tests with Phoenix Cloud
export PHOENIX_API_KEY=your-api-key
export PHOENIX_SPACE_ID=your-space-id
go test -v ./llmops/
```

## License

MIT License - see [LICENSE](LICENSE) for details.

 [build-status-svg]: https://github.com/plexusone/phoenix-go/actions/workflows/ci.yaml/badge.svg?branch=main
 [build-status-url]: https://github.com/plexusone/phoenix-go/actions/workflows/ci.yaml
 [lint-status-svg]: https://github.com/plexusone/phoenix-go/actions/workflows/lint.yaml/badge.svg?branch=main
 [lint-status-url]: https://github.com/plexusone/phoenix-go/actions/workflows/lint.yaml
 [goreport-svg]: https://goreportcard.com/badge/github.com/plexusone/phoenix-go
 [goreport-url]: https://goreportcard.com/report/github.com/plexusone/phoenix-go
 [docs-godoc-svg]: https://pkg.go.dev/badge/github.com/plexusone/phoenix-go
 [docs-godoc-url]: https://pkg.go.dev/github.com/plexusone/phoenix-go
 [viz-svg]: https://img.shields.io/badge/visualizaton-Go-blue.svg
 [viz-url]: https://mango-dune-07a8b7110.1.azurestaticapps.net/?repo=plexusone%2Fphoenix-go
 [license-svg]: https://img.shields.io/badge/license-MIT-blue.svg
 [license-url]: https://github.com/plexusone/phoenix-go/blob/master/LICENSE
 [used-by-svg]: https://sourcegraph.com/github.com/plexusone/phoenix-go/-/badge.svg
 [used-by-url]: https://sourcegraph.com/github.com/plexusone/phoenix-go?badge
 [version-svg]: https://img.shields.io/github/v/release/plexusone/phoenix-go
 [version-url]: https://github.com/plexusone/phoenix-go/releases
