package llmops

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/plexusone/omniobserve/llmops"
	phoenixotel "github.com/plexusone/phoenix-go/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// traceWrapper implements llmops.Trace wrapping an OTEL span.
type traceWrapper struct {
	provider  *Provider
	otelSpan  trace.Span
	name      string
	startTime time.Time
	endTime   *time.Time
	mu        sync.RWMutex
}

func newTrace(provider *Provider, name string, otelSpan trace.Span, cfg *llmops.TraceOptions) *traceWrapper {
	t := &traceWrapper{
		provider:  provider,
		otelSpan:  otelSpan,
		name:      name,
		startTime: time.Now(),
	}

	// Set initial attributes from config
	if cfg.Input != nil {
		_ = t.SetInput(cfg.Input)
	}
	if cfg.Metadata != nil {
		_ = t.SetMetadata(cfg.Metadata)
	}
	if len(cfg.Tags) > 0 {
		for _, tag := range cfg.Tags {
			_ = t.AddTag(tag)
		}
	}
	if cfg.ThreadID != "" {
		t.otelSpan.SetAttributes(phoenixotel.WithSessionID(cfg.ThreadID))
	}

	return t
}

// ID returns the trace ID (OTEL trace ID).
func (t *traceWrapper) ID() string {
	return t.otelSpan.SpanContext().TraceID().String()
}

// Name returns the trace name.
func (t *traceWrapper) Name() string {
	return t.name
}

// StartSpan creates a child span within this trace.
func (t *traceWrapper) StartSpan(ctx context.Context, name string, opts ...llmops.SpanOption) (context.Context, llmops.Span, error) {
	cfg := llmops.ApplySpanOptions(opts...)

	// Start child span using the provider's tracer
	ctx, otelSpan := t.provider.tracer.Start(ctx, name)

	// Create span wrapper
	s := newSpan(t.provider, name, otelSpan, t.ID(), "", cfg)

	// Store in context
	ctx = contextWithSpan(ctx, s)

	return ctx, s, nil
}

// SetInput sets the trace input data using OpenInference attributes.
func (t *traceWrapper) SetInput(input any) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Convert to string for OTEL attribute
	inputStr := toString(input)
	t.otelSpan.SetAttributes(phoenixotel.WithInput(inputStr))

	return nil
}

// SetOutput sets the trace output data using OpenInference attributes.
func (t *traceWrapper) SetOutput(output any) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	outputStr := toString(output)
	t.otelSpan.SetAttributes(phoenixotel.WithOutput(outputStr))

	return nil
}

// SetMetadata sets additional metadata on the trace.
func (t *traceWrapper) SetMetadata(metadata map[string]any) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Serialize metadata to JSON for OTEL attribute
	if data, err := json.Marshal(metadata); err == nil {
		t.otelSpan.SetAttributes(phoenixotel.WithMetadata(string(data)))
	}

	return nil
}

// AddTag adds a tag to the trace.
func (t *traceWrapper) AddTag(tag string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Add as OTEL attribute
	t.otelSpan.SetAttributes(attribute.String(phoenixotel.TagsKey, tag))

	return nil
}

// AddFeedbackScore adds a feedback score to this trace.
func (t *traceWrapper) AddFeedbackScore(ctx context.Context, name string, score float64, opts ...llmops.FeedbackOption) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	cfg := &llmops.FeedbackOptions{}
	for _, opt := range opts {
		opt(cfg)
	}

	attrs := buildFeedbackAttrs(name, score, cfg)
	t.otelSpan.AddEvent("feedback", trace.WithAttributes(attrs...))

	return nil
}

// End completes the trace.
func (t *traceWrapper) End(opts ...llmops.EndOption) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	cfg := &llmops.EndOptions{}
	for _, opt := range opts {
		opt(cfg)
	}

	// Set final output if provided
	if cfg.Output != nil {
		outputStr := toString(cfg.Output)
		t.otelSpan.SetAttributes(phoenixotel.WithOutput(outputStr))
	}

	// Set final metadata if provided
	if cfg.Metadata != nil {
		if data, err := json.Marshal(cfg.Metadata); err == nil {
			t.otelSpan.SetAttributes(phoenixotel.WithMetadata(string(data)))
		}
	}

	// Record error if provided
	if cfg.Error != nil {
		t.otelSpan.RecordError(cfg.Error)
	}

	// End the OTEL span
	t.otelSpan.End()

	now := time.Now()
	t.endTime = &now

	return nil
}

// EndTime returns when the trace ended.
func (t *traceWrapper) EndTime() *time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.endTime
}

// Duration returns the trace duration.
func (t *traceWrapper) Duration() time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.endTime != nil {
		return t.endTime.Sub(t.startTime)
	}
	return time.Since(t.startTime)
}

// Context key types for trace/span storage.
type traceContextKey struct{}
type spanContextKey struct{}

func contextWithTrace(ctx context.Context, t *traceWrapper) context.Context {
	return context.WithValue(ctx, traceContextKey{}, t)
}

func traceFromContext(ctx context.Context) *traceWrapper {
	if t, ok := ctx.Value(traceContextKey{}).(*traceWrapper); ok {
		return t
	}
	return nil
}

func contextWithSpan(ctx context.Context, s *spanWrapper) context.Context {
	return context.WithValue(ctx, spanContextKey{}, s)
}

func spanFromContext(ctx context.Context) *spanWrapper {
	if s, ok := ctx.Value(spanContextKey{}).(*spanWrapper); ok {
		return s
	}
	return nil
}

// toString converts any value to string for OTEL attributes.
func toString(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	default:
		if data, err := json.Marshal(val); err == nil {
			return string(data)
		}
		return ""
	}
}

// buildFeedbackAttrs builds OTEL attributes for feedback scores.
func buildFeedbackAttrs(name string, score float64, cfg *llmops.FeedbackOptions) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("feedback.name", name),
		attribute.Float64("feedback.score", score),
	}
	if cfg.Reason != "" {
		attrs = append(attrs, attribute.String("feedback.reason", cfg.Reason))
	}
	if cfg.Category != "" {
		attrs = append(attrs, attribute.String("feedback.category", cfg.Category))
	}
	if cfg.Source != "" {
		attrs = append(attrs, attribute.String("feedback.source", cfg.Source))
	}
	return attrs
}
