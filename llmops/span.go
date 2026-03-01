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

// spanWrapper implements llmops.Span wrapping an OTEL span.
type spanWrapper struct {
	provider     *Provider
	otelSpan     trace.Span
	traceID      string
	parentSpanID string
	name         string
	spanType     llmops.SpanType
	startTime    time.Time
	endTime      *time.Time
	mu           sync.RWMutex
}

func newSpan(provider *Provider, name string, otelSpan trace.Span, traceID, parentSpanID string, cfg *llmops.SpanOptions) *spanWrapper {
	s := &spanWrapper{
		provider:     provider,
		otelSpan:     otelSpan,
		traceID:      traceID,
		parentSpanID: parentSpanID,
		name:         name,
		spanType:     cfg.Type,
		startTime:    time.Now(),
	}

	// Set span kind based on type
	if cfg.Type != "" {
		otelKind := mapSpanTypeToOpenInference(cfg.Type)
		otelSpan.SetAttributes(phoenixotel.WithSpanKind(otelKind))
	}

	// Set initial attributes from config
	if cfg.Input != nil {
		_ = s.SetInput(cfg.Input)
	}
	if cfg.Metadata != nil {
		_ = s.SetMetadata(cfg.Metadata)
	}
	if len(cfg.Tags) > 0 {
		for _, tag := range cfg.Tags {
			_ = s.AddTag(tag)
		}
	}
	if cfg.Model != "" {
		_ = s.SetModel(cfg.Model)
	}
	if cfg.Provider != "" {
		_ = s.SetProvider(cfg.Provider)
	}
	if cfg.Usage != nil {
		_ = s.SetUsage(*cfg.Usage)
	}

	return s
}

// mapSpanTypeToOpenInference maps llmops.SpanType to OpenInference span kind.
func mapSpanTypeToOpenInference(spanType llmops.SpanType) string {
	switch spanType {
	case llmops.SpanTypeLLM:
		return phoenixotel.SpanKindLLM
	case llmops.SpanTypeTool:
		return phoenixotel.SpanKindTool
	case llmops.SpanTypeAgent:
		return phoenixotel.SpanKindAgent
	case llmops.SpanTypeChain:
		return phoenixotel.SpanKindChain
	case llmops.SpanTypeRetrieval:
		return phoenixotel.SpanKindRetriever
	case llmops.SpanTypeGuardrail:
		return phoenixotel.SpanKindGuardrail
	default:
		return phoenixotel.SpanKindChain // Default to chain for general spans
	}
}

// ID returns the span ID.
func (s *spanWrapper) ID() string {
	return s.otelSpan.SpanContext().SpanID().String()
}

// TraceID returns the parent trace ID.
func (s *spanWrapper) TraceID() string {
	// Get from OTEL span context if not stored
	if s.traceID == "" {
		return s.otelSpan.SpanContext().TraceID().String()
	}
	return s.traceID
}

// ParentSpanID returns the parent span ID.
func (s *spanWrapper) ParentSpanID() string {
	return s.parentSpanID
}

// Name returns the span name.
func (s *spanWrapper) Name() string {
	return s.name
}

// Type returns the span type.
func (s *spanWrapper) Type() llmops.SpanType {
	return s.spanType
}

// StartSpan creates a child span within this span.
func (s *spanWrapper) StartSpan(ctx context.Context, name string, opts ...llmops.SpanOption) (context.Context, llmops.Span, error) {
	cfg := llmops.ApplySpanOptions(opts...)

	// Start child span using the provider's tracer
	ctx, otelSpan := s.provider.tracer.Start(ctx, name)

	// Create span wrapper
	child := newSpan(s.provider, name, otelSpan, s.TraceID(), s.ID(), cfg)

	// Store in context
	ctx = contextWithSpan(ctx, child)

	return ctx, child, nil
}

// SetInput sets the span input data using OpenInference attributes.
func (s *spanWrapper) SetInput(input any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	inputStr := toString(input)
	s.otelSpan.SetAttributes(phoenixotel.WithInput(inputStr))

	return nil
}

// SetOutput sets the span output data using OpenInference attributes.
func (s *spanWrapper) SetOutput(output any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	outputStr := toString(output)
	s.otelSpan.SetAttributes(phoenixotel.WithOutput(outputStr))

	return nil
}

// SetMetadata sets additional metadata on the span.
func (s *spanWrapper) SetMetadata(metadata map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if data, err := json.Marshal(metadata); err == nil {
		s.otelSpan.SetAttributes(phoenixotel.WithMetadata(string(data)))
	}

	return nil
}

// SetModel sets the LLM model name using OpenInference attributes.
func (s *spanWrapper) SetModel(model string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.otelSpan.SetAttributes(phoenixotel.WithModelName(model))

	return nil
}

// SetProvider sets the LLM provider name using OpenInference attributes.
func (s *spanWrapper) SetProvider(provider string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.otelSpan.SetAttributes(phoenixotel.WithLLMProvider(provider))

	return nil
}

// SetUsage sets token usage information using OpenInference attributes.
func (s *spanWrapper) SetUsage(usage llmops.TokenUsage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	attrs := phoenixotel.WithTokenCounts(
		usage.PromptTokens,
		usage.CompletionTokens,
		usage.TotalTokens,
	)
	s.otelSpan.SetAttributes(attrs...)

	return nil
}

// AddTag adds a tag to the span.
func (s *spanWrapper) AddTag(tag string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.otelSpan.SetAttributes(attribute.String(phoenixotel.TagsKey, tag))

	return nil
}

// AddFeedbackScore adds a feedback score to this span.
func (s *spanWrapper) AddFeedbackScore(ctx context.Context, name string, score float64, opts ...llmops.FeedbackOption) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := &llmops.FeedbackOptions{}
	for _, opt := range opts {
		opt(cfg)
	}

	attrs := buildFeedbackAttrs(name, score, cfg)
	s.otelSpan.AddEvent("feedback", trace.WithAttributes(attrs...))

	return nil
}

// End completes the span.
func (s *spanWrapper) End(opts ...llmops.EndOption) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := &llmops.EndOptions{}
	for _, opt := range opts {
		opt(cfg)
	}

	// Set final output if provided
	if cfg.Output != nil {
		outputStr := toString(cfg.Output)
		s.otelSpan.SetAttributes(phoenixotel.WithOutput(outputStr))
	}

	// Set final metadata if provided
	if cfg.Metadata != nil {
		if data, err := json.Marshal(cfg.Metadata); err == nil {
			s.otelSpan.SetAttributes(phoenixotel.WithMetadata(string(data)))
		}
	}

	// Record error if provided
	if cfg.Error != nil {
		s.otelSpan.RecordError(cfg.Error)
	}

	// End the OTEL span
	s.otelSpan.End()

	now := time.Now()
	s.endTime = &now

	return nil
}

// EndTime returns when the span ended.
func (s *spanWrapper) EndTime() *time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.endTime
}

// Duration returns the span duration.
func (s *spanWrapper) Duration() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.endTime != nil {
		return s.endTime.Sub(s.startTime)
	}
	return time.Since(s.startTime)
}
