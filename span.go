package phoenix

import (
	"context"
	"time"

	"github.com/plexusone/phoenix-go/internal/api"
)

// Span represents a Phoenix span.
type Span struct {
	ID            string
	Name          string
	SpanKind      string
	StatusCode    string
	StatusMessage string
	ParentID      string
	TraceID       string
	SpanID        string
	StartTime     time.Time
	EndTime       time.Time
}

// GetSpans retrieves spans for a project.
func (c *Client) GetSpans(ctx context.Context, projectIdentifier string, opts ...SpanOption) ([]*Span, string, error) {
	options := &spanOptions{
		limit: 100,
	}
	for _, opt := range opts {
		opt(options)
	}

	params := api.GetSpansParams{
		ProjectIdentifier: projectIdentifier,
	}
	if options.cursor != "" {
		params.Cursor.SetTo(options.cursor)
	}
	if options.limit > 0 {
		params.Limit.SetTo(options.limit)
	}

	res, err := c.apiClient.GetSpans(ctx, params)
	if err != nil {
		return nil, "", err
	}

	resp, ok := res.(*api.SpansResponseBody)
	if !ok {
		return nil, "", &APIError{Message: "unexpected response type"}
	}

	spans := make([]*Span, 0, len(resp.Data))
	for i := range resp.Data {
		spans = append(spans, convertSpan(&resp.Data[i]))
	}

	var nextCursor string
	if !resp.NextCursor.Null {
		nextCursor = resp.NextCursor.Value
	}

	return spans, nextCursor, nil
}

// DeleteSpan deletes a span.
func (c *Client) DeleteSpan(ctx context.Context, spanIdentifier string) error {
	_, err := c.apiClient.DeleteSpan(ctx, api.DeleteSpanParams{
		SpanIdentifier: spanIdentifier,
	})
	return err
}

// DeleteTrace deletes a trace.
func (c *Client) DeleteTrace(ctx context.Context, traceIdentifier string) error {
	_, err := c.apiClient.DeleteTrace(ctx, api.DeleteTraceParams{
		TraceIdentifier: traceIdentifier,
	})
	return err
}

// SpanOption is a functional option for span operations.
type SpanOption func(*spanOptions)

type spanOptions struct {
	cursor string
	limit  int
}

// WithSpanCursor sets the pagination cursor for spans.
func WithSpanCursor(cursor string) SpanOption {
	return func(o *spanOptions) {
		o.cursor = cursor
	}
}

// WithSpanLimit sets the max number of spans to return.
func WithSpanLimit(limit int) SpanOption {
	return func(o *spanOptions) {
		o.limit = limit
	}
}

func convertSpan(s *api.Span) *Span {
	if s == nil {
		return nil
	}
	span := &Span{
		Name:       s.Name,
		SpanKind:   s.SpanKind,
		StatusCode: s.StatusCode,
		StartTime:  s.StartTime,
		EndTime:    s.EndTime,
		TraceID:    s.Context.TraceID,
		SpanID:     s.Context.SpanID,
	}
	if s.ID.Set {
		span.ID = s.ID.Value
	}
	if s.StatusMessage.Set {
		span.StatusMessage = s.StatusMessage.Value
	}
	if s.ParentID.Set && !s.ParentID.Null {
		span.ParentID = s.ParentID.Value
	}
	return span
}
