package otel

import (
	"os"
	"time"
)

// Environment variable names matching Phoenix Python SDK.
const (
	EnvCollectorEndpoint = "PHOENIX_COLLECTOR_ENDPOINT"
	EnvProjectName       = "PHOENIX_PROJECT_NAME"
	EnvAPIKey            = "PHOENIX_API_KEY"
	EnvSpaceID           = "PHOENIX_SPACE_ID"
	EnvClientHeaders     = "PHOENIX_CLIENT_HEADERS"
	EnvGRPCPort          = "PHOENIX_GRPC_PORT"

	// Standard OTEL fallback
	EnvOTELEndpoint = "OTEL_EXPORTER_OTLP_ENDPOINT"
)

// Default values.
const (
	DefaultEndpoint    = "http://localhost:6006"
	DefaultProjectName = "default"
	DefaultGRPCPort    = 4317
	DefaultHTTPPath    = "/v1/traces"
)

// Config holds the configuration for Phoenix OTEL integration.
type Config struct {
	// Endpoint is the Phoenix collector endpoint.
	// For Phoenix Cloud, use https://app.phoenix.arize.com
	Endpoint string

	// SpaceID is the space identifier for Phoenix Cloud.
	// When set, the endpoint is constructed as {Endpoint}/s/{SpaceID}.
	SpaceID string

	// ProjectName is the project name for traces.
	ProjectName string

	// APIKey is the API key for authentication.
	APIKey string //nolint:gosec // G117: OTEL config needs API key field

	// Headers are additional headers to send with requests.
	Headers map[string]string

	// Protocol specifies the transport protocol (http or grpc).
	Protocol Protocol

	// Batch enables batch span processing (recommended for production).
	Batch bool

	// BatchTimeout is the maximum time to wait before exporting a batch.
	BatchTimeout time.Duration

	// BatchSize is the maximum number of spans to batch.
	BatchSize int

	// SetGlobalProvider sets the tracer provider as global.
	SetGlobalProvider bool

	// ServiceName is the service name for the resource.
	ServiceName string

	// ServiceVersion is the service version for the resource.
	ServiceVersion string

	// Insecure disables TLS for gRPC connections.
	Insecure bool
}

// Protocol specifies the OTLP transport protocol.
type Protocol string

const (
	// ProtocolHTTP uses HTTP/protobuf transport.
	ProtocolHTTP Protocol = "http/protobuf"

	// ProtocolGRPC uses gRPC transport.
	ProtocolGRPC Protocol = "grpc"

	// ProtocolInfer automatically infers the protocol from the endpoint.
	ProtocolInfer Protocol = "infer"
)

// DefaultConfig returns a Config with default values and environment overrides.
func DefaultConfig() *Config {
	cfg := &Config{
		Endpoint:          DefaultEndpoint,
		ProjectName:       DefaultProjectName,
		Protocol:          ProtocolInfer,
		Batch:             false,
		BatchTimeout:      5 * time.Second,
		BatchSize:         512,
		SetGlobalProvider: true,
		Insecure:          false,
	}

	// Load from environment
	if endpoint := getEnvCollectorEndpoint(); endpoint != "" {
		cfg.Endpoint = endpoint
	}
	if spaceID := os.Getenv(EnvSpaceID); spaceID != "" {
		cfg.SpaceID = spaceID
		// Default to Phoenix Cloud when SpaceID is set
		if cfg.Endpoint == DefaultEndpoint {
			cfg.Endpoint = "https://app.phoenix.arize.com"
		}
	}
	if projectName := os.Getenv(EnvProjectName); projectName != "" {
		cfg.ProjectName = projectName
	}
	if apiKey := os.Getenv(EnvAPIKey); apiKey != "" {
		cfg.APIKey = apiKey
	}
	if headers := os.Getenv(EnvClientHeaders); headers != "" {
		cfg.Headers = parseHeaders(headers)
	}

	return cfg
}

// EffectiveEndpoint returns the full endpoint with space ID if configured.
func (c *Config) EffectiveEndpoint() string {
	endpoint := c.Endpoint
	if c.SpaceID != "" {
		// Remove trailing slash if present
		for len(endpoint) > 0 && endpoint[len(endpoint)-1] == '/' {
			endpoint = endpoint[:len(endpoint)-1]
		}
		endpoint = endpoint + "/s/" + c.SpaceID
	}
	return endpoint
}

// getEnvCollectorEndpoint returns the collector endpoint from environment.
func getEnvCollectorEndpoint() string {
	if endpoint := os.Getenv(EnvCollectorEndpoint); endpoint != "" {
		return endpoint
	}
	return os.Getenv(EnvOTELEndpoint)
}

// parseHeaders parses a header string in W3C Baggage format.
func parseHeaders(s string) map[string]string {
	headers := make(map[string]string)
	// Simple parsing: key=value,key2=value2
	for _, part := range splitTrim(s, ",") {
		if kv := splitTrim(part, "="); len(kv) == 2 {
			headers[kv[0]] = kv[1]
		}
	}
	return headers
}

// splitTrim splits a string and trims whitespace from each part.
func splitTrim(s, sep string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			part := trim(s[start:i])
			if part != "" {
				result = append(result, part)
			}
			start = i + len(sep)
		}
	}
	if part := trim(s[start:]); part != "" {
		result = append(result, part)
	}
	return result
}

// trim removes leading and trailing whitespace.
func trim(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
