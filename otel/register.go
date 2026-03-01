// Package otel provides OpenTelemetry integration for Phoenix.
//
// This package is the Go equivalent of the Python phoenix-otel package,
// providing easy setup of OpenTelemetry tracing with Phoenix-aware defaults.
//
// # Quick Start
//
//	import "github.com/plexusone/phoenix-go/otel"
//
//	func main() {
//		// Register with Phoenix (sends traces to localhost:6006)
//		tp, err := otel.Register()
//		if err != nil {
//			log.Fatal(err)
//		}
//		defer tp.Shutdown(context.Background())
//
//		// Use the tracer
//		tracer := tp.Tracer("my-service")
//		ctx, span := tracer.Start(context.Background(), "my-operation")
//		defer span.End()
//	}
//
// # Production Configuration
//
//	tp, err := otel.Register(
//		otel.WithEndpoint("https://app.phoenix.arize.com"),
//		otel.WithAPIKey("your-api-key"),
//		otel.WithProjectName("my-project"),
//		otel.WithBatch(true),
//	)
//
// # Environment Variables
//
// The following environment variables are supported:
//
//   - PHOENIX_COLLECTOR_ENDPOINT: Phoenix collector endpoint
//   - PHOENIX_PROJECT_NAME: Project name for traces
//   - PHOENIX_API_KEY: API key for authentication
//   - PHOENIX_CLIENT_HEADERS: Additional headers (W3C Baggage format)
//   - OTEL_EXPORTER_OTLP_ENDPOINT: Fallback OTLP endpoint
package otel

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// TracerProvider wraps the OpenTelemetry TracerProvider with Phoenix-specific functionality.
type TracerProvider struct {
	*sdktrace.TracerProvider
	config *Config
}

// Register creates and configures an OpenTelemetry TracerProvider for Phoenix.
//
// This is the main entry point for Phoenix OTEL integration. It:
//   - Creates an OTLP HTTP exporter configured for Phoenix
//   - Sets up a TracerProvider with Phoenix resource attributes
//   - Optionally registers as the global tracer provider
//
// Example:
//
//	tp, err := otel.Register(
//		otel.WithProjectName("my-app"),
//		otel.WithBatch(true),
//	)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer tp.Shutdown(context.Background())
func Register(opts ...Option) (*TracerProvider, error) {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	// Create exporter
	exporter, err := createExporter(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	// Create resource with Phoenix attributes
	res, err := createResource(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create span processor
	var spanProcessor sdktrace.SpanProcessor
	if cfg.Batch {
		spanProcessor = sdktrace.NewBatchSpanProcessor(exporter,
			sdktrace.WithBatchTimeout(cfg.BatchTimeout),
			sdktrace.WithMaxExportBatchSize(cfg.BatchSize),
		)
	} else {
		spanProcessor = sdktrace.NewSimpleSpanProcessor(exporter)
	}

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(spanProcessor),
		sdktrace.WithResource(res),
	)

	// Set as global provider if requested
	if cfg.SetGlobalProvider {
		otel.SetTracerProvider(tp)
	}

	return &TracerProvider{
		TracerProvider: tp,
		config:         cfg,
	}, nil
}

// Shutdown shuts down the tracer provider, flushing any remaining spans.
func (tp *TracerProvider) Shutdown(ctx context.Context) error {
	return tp.TracerProvider.Shutdown(ctx)
}

// Config returns the configuration used by this tracer provider.
func (tp *TracerProvider) Config() *Config {
	return tp.config
}

// createExporter creates an OTLP exporter based on the configuration.
func createExporter(cfg *Config) (sdktrace.SpanExporter, error) {
	// Use effective endpoint (includes space ID if configured)
	endpoint := cfg.EffectiveEndpoint()
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "http://" + endpoint
	}

	parsedURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint URL: %w", err)
	}

	// Note: Currently only HTTP/protobuf is supported.
	// gRPC support would require importing otlptracegrpc.
	// Protocol inference is available via inferProtocol() for future use.

	// Build options
	var exporterOpts []otlptracehttp.Option

	// Set endpoint (host:port)
	host := parsedURL.Hostname()
	port := parsedURL.Port()
	if port == "" {
		if parsedURL.Scheme == "https" {
			port = "443"
		} else {
			port = "6006"
		}
	}
	exporterOpts = append(exporterOpts, otlptracehttp.WithEndpoint(host+":"+port))

	// Set URL path - always append /v1/traces to the base path
	basePath := parsedURL.Path
	if basePath == "" || basePath == "/" {
		basePath = ""
	}
	// Ensure we have the OTLP trace endpoint path
	path := basePath + DefaultHTTPPath
	exporterOpts = append(exporterOpts, otlptracehttp.WithURLPath(path))

	// Set TLS
	if parsedURL.Scheme == "http" {
		exporterOpts = append(exporterOpts, otlptracehttp.WithInsecure())
	}

	// Set headers
	headers := make(map[string]string)
	for k, v := range cfg.Headers {
		headers[k] = v
	}

	// Add API key as Bearer token
	if cfg.APIKey != "" {
		headers["Authorization"] = "Bearer " + cfg.APIKey
	}

	// Add project name header (Phoenix convention)
	if cfg.ProjectName != "" {
		headers["x-phoenix-project-name"] = cfg.ProjectName
	}

	if len(headers) > 0 {
		exporterOpts = append(exporterOpts, otlptracehttp.WithHeaders(headers))
	}

	return otlptracehttp.New(context.Background(), exporterOpts...)
}

// createResource creates an OpenTelemetry resource with Phoenix attributes.
func createResource(cfg *Config) (*resource.Resource, error) {
	attrs := []attribute.KeyValue{
		// OpenInference resource attribute for project name
		// This is the standard attribute Phoenix uses to route traces to projects
		attribute.String("openinference.project.name", cfg.ProjectName),
	}

	// Add service name if provided
	if cfg.ServiceName != "" {
		attrs = append(attrs, semconv.ServiceName(cfg.ServiceName))
	}

	// Add service version if provided
	if cfg.ServiceVersion != "" {
		attrs = append(attrs, semconv.ServiceVersion(cfg.ServiceVersion))
	}

	// Use NewSchemaless to avoid schema URL conflicts with resource.Default()
	// which may use a different schema version
	return resource.Merge(
		resource.Default(),
		resource.NewSchemaless(attrs...),
	)
}

// inferProtocol infers the transport protocol from the endpoint URL.
// Currently unused but kept for future gRPC support.
func inferProtocol(u *url.URL) Protocol { //nolint:unused // Kept for future gRPC support
	port := u.Port()

	// gRPC typically uses port 4317
	if port == "4317" {
		return ProtocolGRPC
	}

	// Default to HTTP for Phoenix endpoints
	return ProtocolHTTP
}
