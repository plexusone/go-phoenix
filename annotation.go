package phoenix

import (
	"context"
	"time"

	"github.com/plexusone/phoenix-go/internal/api"
)

// Annotation represents a Phoenix annotation on a span or trace.
type Annotation struct {
	ID          string
	SpanID      string // Set for span annotations
	TraceID     string // Set for trace annotations
	Name        string
	Score       float64
	Label       string
	Explanation string
	Source      AnnotatorKind
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// AnnotatorKind indicates who created the annotation.
type AnnotatorKind string

const (
	AnnotatorKindHuman AnnotatorKind = "HUMAN"
	AnnotatorKindLLM   AnnotatorKind = "LLM"
	AnnotatorKindCode  AnnotatorKind = "CODE"
)

// CreateSpanAnnotation creates an annotation on a span.
func (c *Client) CreateSpanAnnotation(ctx context.Context, spanID, name string, score float64, opts ...AnnotationOption) error { //nolint:dupl // Type-safe pattern differs only in types
	options := &annotationOptions{}
	for _, opt := range opts {
		opt(options)
	}

	result := api.AnnotationResult{}
	result.SetScore(api.OptNilFloat64{Value: score, Set: true})
	if options.explanation != "" {
		result.SetExplanation(api.OptNilString{Value: options.explanation, Set: true})
	}
	if options.label != "" {
		result.SetLabel(api.OptNilString{Value: options.label, Set: true})
	}

	annotatorKind := api.SpanAnnotationDataAnnotatorKindHUMAN
	switch options.source {
	case AnnotatorKindLLM:
		annotatorKind = api.SpanAnnotationDataAnnotatorKindLLM
	case AnnotatorKindCode:
		annotatorKind = api.SpanAnnotationDataAnnotatorKindCODE
	}

	_, err := c.apiClient.AnnotateSpans(ctx, &api.AnnotateSpansRequestBody{
		Data: []api.SpanAnnotationData{{
			SpanID:        spanID,
			Name:          name,
			AnnotatorKind: annotatorKind,
			Result:        api.OptAnnotationResult{Value: result, Set: true},
		}},
	}, api.AnnotateSpansParams{})

	return err
}

// CreateTraceAnnotation creates an annotation on a trace.
func (c *Client) CreateTraceAnnotation(ctx context.Context, traceID, name string, score float64, opts ...AnnotationOption) error { //nolint:dupl // Type-safe pattern differs only in types
	options := &annotationOptions{}
	for _, opt := range opts {
		opt(options)
	}

	result := api.AnnotationResult{}
	result.SetScore(api.OptNilFloat64{Value: score, Set: true})
	if options.explanation != "" {
		result.SetExplanation(api.OptNilString{Value: options.explanation, Set: true})
	}
	if options.label != "" {
		result.SetLabel(api.OptNilString{Value: options.label, Set: true})
	}

	annotatorKind := api.TraceAnnotationDataAnnotatorKindHUMAN
	switch options.source {
	case AnnotatorKindLLM:
		annotatorKind = api.TraceAnnotationDataAnnotatorKindLLM
	case AnnotatorKindCode:
		annotatorKind = api.TraceAnnotationDataAnnotatorKindCODE
	}

	_, err := c.apiClient.AnnotateTraces(ctx, &api.AnnotateTracesRequestBody{
		Data: []api.TraceAnnotationData{{
			TraceID:       traceID,
			Name:          name,
			AnnotatorKind: annotatorKind,
			Result:        api.OptAnnotationResult{Value: result, Set: true},
		}},
	}, api.AnnotateTracesParams{})

	return err
}

// ListSpanAnnotations lists annotations for the given span IDs.
func (c *Client) ListSpanAnnotations(ctx context.Context, spanIDs []string) ([]*Annotation, error) {
	res, err := c.apiClient.ListSpanAnnotationsBySpanIds(ctx, api.ListSpanAnnotationsBySpanIdsParams{
		SpanIds: spanIDs,
	})
	if err != nil {
		return nil, err
	}

	resp, ok := res.(*api.SpanAnnotationsResponseBody)
	if !ok {
		return nil, &APIError{Message: "unexpected response type"}
	}

	annotations := make([]*Annotation, 0, len(resp.Data))
	for i := range resp.Data {
		annotations = append(annotations, convertSpanAnnotation(&resp.Data[i]))
	}

	return annotations, nil
}

// ListTraceAnnotations lists annotations for the given trace IDs.
func (c *Client) ListTraceAnnotations(ctx context.Context, traceIDs []string) ([]*Annotation, error) {
	res, err := c.apiClient.ListTraceAnnotationsByTraceIds(ctx, api.ListTraceAnnotationsByTraceIdsParams{
		TraceIds: traceIDs,
	})
	if err != nil {
		return nil, err
	}

	resp, ok := res.(*api.TraceAnnotationsResponseBody)
	if !ok {
		return nil, &APIError{Message: "unexpected response type"}
	}

	annotations := make([]*Annotation, 0, len(resp.Data))
	for i := range resp.Data {
		annotations = append(annotations, convertTraceAnnotation(&resp.Data[i]))
	}

	return annotations, nil
}

// AnnotationOption configures annotation creation.
type AnnotationOption func(*annotationOptions)

type annotationOptions struct {
	explanation string
	label       string
	source      AnnotatorKind
}

// WithAnnotationExplanation sets the explanation for the annotation.
func WithAnnotationExplanation(explanation string) AnnotationOption {
	return func(o *annotationOptions) {
		o.explanation = explanation
	}
}

// WithAnnotationLabel sets the label for the annotation.
func WithAnnotationLabel(label string) AnnotationOption {
	return func(o *annotationOptions) {
		o.label = label
	}
}

// WithAnnotationSource sets the source (annotator kind) for the annotation.
func WithAnnotationSource(source AnnotatorKind) AnnotationOption {
	return func(o *annotationOptions) {
		o.source = source
	}
}

func convertSpanAnnotation(a *api.SpanAnnotation) *Annotation { //nolint:dupl // Type-safe pattern differs only in types
	if a == nil {
		return nil
	}
	ann := &Annotation{
		ID:        a.ID,
		SpanID:    a.SpanID,
		Name:      a.Name,
		CreatedAt: a.CreatedAt,
		UpdatedAt: a.UpdatedAt,
	}

	// Map AnnotatorKind
	switch a.AnnotatorKind {
	case api.SpanAnnotationAnnotatorKindLLM:
		ann.Source = AnnotatorKindLLM
	case api.SpanAnnotationAnnotatorKindCODE:
		ann.Source = AnnotatorKindCode
	default:
		ann.Source = AnnotatorKindHuman
	}

	// Extract result fields
	if a.Result.Set {
		result := a.Result.Value
		if result.Score.Set && !result.Score.Null {
			ann.Score = result.Score.Value
		}
		if result.Label.Set && !result.Label.Null {
			ann.Label = result.Label.Value
		}
		if result.Explanation.Set && !result.Explanation.Null {
			ann.Explanation = result.Explanation.Value
		}
	}

	return ann
}

func convertTraceAnnotation(a *api.TraceAnnotation) *Annotation { //nolint:dupl // Type-safe pattern differs only in types
	if a == nil {
		return nil
	}
	ann := &Annotation{
		ID:        a.ID,
		TraceID:   a.TraceID,
		Name:      a.Name,
		CreatedAt: a.CreatedAt,
		UpdatedAt: a.UpdatedAt,
	}

	// Map AnnotatorKind
	switch a.AnnotatorKind {
	case api.TraceAnnotationAnnotatorKindLLM:
		ann.Source = AnnotatorKindLLM
	case api.TraceAnnotationAnnotatorKindCODE:
		ann.Source = AnnotatorKindCode
	default:
		ann.Source = AnnotatorKindHuman
	}

	// Extract result fields
	if a.Result.Set {
		result := a.Result.Value
		if result.Score.Set && !result.Score.Null {
			ann.Score = result.Score.Value
		}
		if result.Label.Set && !result.Label.Null {
			ann.Label = result.Label.Value
		}
		if result.Explanation.Set && !result.Explanation.Null {
			ann.Explanation = result.Explanation.Value
		}
	}

	return ann
}
