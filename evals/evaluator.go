// Package evals provides evaluation capabilities for Phoenix.
//
// This package implements the llmops.Evaluator interface, enabling
// evaluation of LLM outputs with metrics and recording results to Phoenix.
//
// # Usage
//
//	import (
//	    "github.com/plexusone/phoenix-go"
//	    "github.com/plexusone/phoenix-go/evals"
//	    "github.com/plexusone/omniobserve/llmops"
//	    "github.com/plexusone/omniobserve/llmops/metrics"
//	)
//
//	// Create Phoenix client
//	client, _ := phoenix.NewClient()
//
//	// Create evaluator
//	evaluator := evals.NewEvaluator(client)
//
//	// Run evaluation
//	result, _ := evaluator.Evaluate(ctx, llmops.EvalInput{
//	    Input:   "What is the capital of France?",
//	    Output:  "The capital of France is Paris.",
//	    SpanID:  "span-123",
//	}, metrics.NewExactMatchMetric())
package evals

import (
	"context"
	"time"

	"github.com/plexusone/omniobserve/llmops"
	phoenix "github.com/plexusone/phoenix-go"
	"github.com/plexusone/phoenix-go/internal/api"
)

// Evaluator implements llmops.Evaluator for Phoenix.
type Evaluator struct {
	client        *phoenix.Client
	recordResults bool // Whether to record results to Phoenix
}

// NewEvaluator creates a new Phoenix evaluator.
func NewEvaluator(client *phoenix.Client) *Evaluator {
	return &Evaluator{
		client:        client,
		recordResults: true,
	}
}

// EvaluatorOption configures the Evaluator.
type EvaluatorOption func(*Evaluator)

// WithRecordResults sets whether to record results to Phoenix.
func WithRecordResults(record bool) EvaluatorOption {
	return func(e *Evaluator) {
		e.recordResults = record
	}
}

// NewEvaluatorWithOptions creates an evaluator with options.
func NewEvaluatorWithOptions(client *phoenix.Client, opts ...EvaluatorOption) *Evaluator {
	e := NewEvaluator(client)
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Evaluate runs metrics on the input and optionally records results to Phoenix.
func (e *Evaluator) Evaluate(ctx context.Context, input llmops.EvalInput, metrics ...llmops.Metric) (*llmops.EvalResult, error) {
	start := time.Now()
	scores := make([]llmops.MetricScore, 0, len(metrics))

	// Run each metric
	for _, metric := range metrics {
		score, err := metric.Evaluate(input)
		if err != nil {
			// Record error in score but continue with other metrics
			score = llmops.MetricScore{
				Name:  metric.Name(),
				Error: err.Error(),
			}
		}
		scores = append(scores, score)
	}

	result := &llmops.EvalResult{
		Scores:   scores,
		Duration: time.Since(start),
	}

	// Record results to Phoenix if enabled and we have a span ID
	if e.recordResults && input.SpanID != "" {
		if err := e.recordScoresToPhoenix(ctx, input.SpanID, scores); err != nil {
			// Log error but don't fail the evaluation
			result.Metadata = map[string]any{
				"record_error": err.Error(),
			}
		}
	}

	return result, nil
}

// AddFeedbackScore adds a feedback score to a span or trace.
func (e *Evaluator) AddFeedbackScore(ctx context.Context, opts llmops.FeedbackScoreOpts) error {
	if opts.SpanID != "" {
		return e.addSpanAnnotation(ctx, opts.SpanID, opts.Name, opts.Score, opts.Reason, opts.Source)
	}
	if opts.TraceID != "" {
		return e.addTraceAnnotation(ctx, opts.TraceID, opts.Name, opts.Score, opts.Reason, opts.Source)
	}
	return nil
}

// recordScoresToPhoenix records metric scores as span annotations.
func (e *Evaluator) recordScoresToPhoenix(ctx context.Context, spanID string, scores []llmops.MetricScore) error {
	annotations := make([]api.SpanAnnotationData, 0, len(scores))

	for _, score := range scores {
		if score.Error != "" {
			// Skip errored scores
			continue
		}

		annotation := api.SpanAnnotationData{
			SpanID:        spanID,
			Name:          score.Name,
			AnnotatorKind: inferAnnotatorKind(score),
		}

		// Set result
		result := api.AnnotationResult{}
		result.SetScore(api.OptNilFloat64{Value: score.Score, Set: true})
		if score.Reason != "" {
			result.SetExplanation(api.OptNilString{Value: score.Reason, Set: true})
		}
		// Extract label from metadata if present
		if score.Metadata != nil {
			if label, ok := score.Metadata.(map[string]any)["label"].(string); ok {
				result.SetLabel(api.OptNilString{Value: label, Set: true})
			}
		}
		annotation.Result = api.OptAnnotationResult{Value: result, Set: true}

		annotations = append(annotations, annotation)
	}

	if len(annotations) == 0 {
		return nil
	}

	_, err := e.client.API().AnnotateSpans(ctx, &api.AnnotateSpansRequestBody{
		Data: annotations,
	}, api.AnnotateSpansParams{})

	return err
}

// addSpanAnnotation adds a single annotation to a span.
func (e *Evaluator) addSpanAnnotation(ctx context.Context, spanID, name string, score float64, reason, source string) error {
	result := buildAnnotationResult(score, reason)
	annotatorKind := parseSpanAnnotatorKind(source)

	_, err := e.client.API().AnnotateSpans(ctx, &api.AnnotateSpansRequestBody{
		Data: []api.SpanAnnotationData{{
			SpanID:        spanID,
			Name:          name,
			AnnotatorKind: annotatorKind,
			Result:        api.OptAnnotationResult{Value: result, Set: true},
		}},
	}, api.AnnotateSpansParams{})
	return err
}

// addTraceAnnotation adds a single annotation to a trace.
func (e *Evaluator) addTraceAnnotation(ctx context.Context, traceID, name string, score float64, reason, source string) error {
	result := buildAnnotationResult(score, reason)
	annotatorKind := parseTraceAnnotatorKind(source)

	_, err := e.client.API().AnnotateTraces(ctx, &api.AnnotateTracesRequestBody{
		Data: []api.TraceAnnotationData{{
			TraceID:       traceID,
			Name:          name,
			AnnotatorKind: annotatorKind,
			Result:        api.OptAnnotationResult{Value: result, Set: true},
		}},
	}, api.AnnotateTracesParams{})
	return err
}

// buildAnnotationResult creates an AnnotationResult with the given score and reason.
func buildAnnotationResult(score float64, reason string) api.AnnotationResult {
	result := api.AnnotationResult{}
	result.SetScore(api.OptNilFloat64{Value: score, Set: true})
	if reason != "" {
		result.SetExplanation(api.OptNilString{Value: reason, Set: true})
	}
	return result
}

// parseSpanAnnotatorKind parses source string to SpanAnnotationDataAnnotatorKind.
func parseSpanAnnotatorKind(source string) api.SpanAnnotationDataAnnotatorKind {
	switch source {
	case "llm", "LLM":
		return api.SpanAnnotationDataAnnotatorKindLLM
	case "code", "CODE":
		return api.SpanAnnotationDataAnnotatorKindCODE
	default:
		return api.SpanAnnotationDataAnnotatorKindHUMAN
	}
}

// parseTraceAnnotatorKind parses source string to TraceAnnotationDataAnnotatorKind.
func parseTraceAnnotatorKind(source string) api.TraceAnnotationDataAnnotatorKind {
	switch source {
	case "llm", "LLM":
		return api.TraceAnnotationDataAnnotatorKindLLM
	case "code", "CODE":
		return api.TraceAnnotationDataAnnotatorKindCODE
	default:
		return api.TraceAnnotationDataAnnotatorKindHUMAN
	}
}

// inferAnnotatorKind infers the annotator kind from the score metadata.
func inferAnnotatorKind(score llmops.MetricScore) api.SpanAnnotationDataAnnotatorKind {
	// Check if metadata contains kind information
	if score.Metadata != nil {
		if m, ok := score.Metadata.(map[string]any); ok {
			if kind, ok := m["kind"].(string); ok {
				switch kind {
				case "llm", "LLM":
					return api.SpanAnnotationDataAnnotatorKindLLM
				case "code", "CODE":
					return api.SpanAnnotationDataAnnotatorKindCODE
				case "human", "HUMAN":
					return api.SpanAnnotationDataAnnotatorKindHUMAN
				}
			}
		}
	}

	// Default to CODE for metrics (most are code-based or LLM-as-judge which we categorize as CODE)
	return api.SpanAnnotationDataAnnotatorKindCODE
}
