package evals

import (
	"testing"

	"github.com/plexusone/omniobserve/llmops"
	"github.com/plexusone/phoenix-go/internal/api"
)

// mockMetric is a test metric implementation.
type mockMetric struct {
	name  string
	score float64
	err   error
}

func (m *mockMetric) Name() string { return m.name }

func (m *mockMetric) Evaluate(_ llmops.EvalInput) (llmops.MetricScore, error) {
	if m.err != nil {
		return llmops.MetricScore{}, m.err
	}
	return llmops.MetricScore{
		Name:  m.name,
		Score: m.score,
	}, nil
}

func TestNewEvaluator(t *testing.T) {
	e := NewEvaluator(nil)
	if e == nil {
		t.Fatal("expected non-nil evaluator")
	}
	if !e.recordResults {
		t.Error("expected recordResults to be true by default")
	}
}

func TestNewEvaluatorWithOptions(t *testing.T) {
	e := NewEvaluatorWithOptions(nil, WithRecordResults(false))
	if e == nil {
		t.Fatal("expected non-nil evaluator")
	}
	if e.recordResults {
		t.Error("expected recordResults to be false")
	}
}

func TestEvaluator_Evaluate_NoRecording(t *testing.T) {
	// Create evaluator with recording disabled (no client needed)
	e := NewEvaluatorWithOptions(nil, WithRecordResults(false))

	metric := &mockMetric{name: "test_metric", score: 0.85}

	result, err := e.Evaluate(t.Context(), llmops.EvalInput{
		Input:  "test input",
		Output: "test output",
	}, metric)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Scores) != 1 {
		t.Fatalf("expected 1 score, got %d", len(result.Scores))
	}
	if result.Scores[0].Name != "test_metric" {
		t.Errorf("expected name 'test_metric', got '%s'", result.Scores[0].Name)
	}
	if result.Scores[0].Score != 0.85 {
		t.Errorf("expected score 0.85, got %f", result.Scores[0].Score)
	}
	// Note: Duration may be 0 on Windows due to lower timer resolution
	if result.Duration < 0 {
		t.Error("expected non-negative duration")
	}
}

func TestEvaluator_Evaluate_MultipleMetrics(t *testing.T) {
	e := NewEvaluatorWithOptions(nil, WithRecordResults(false))

	metrics := []llmops.Metric{
		&mockMetric{name: "metric1", score: 0.9},
		&mockMetric{name: "metric2", score: 0.75},
		&mockMetric{name: "metric3", score: 1.0},
	}

	result, err := e.Evaluate(t.Context(), llmops.EvalInput{
		Input:  "test",
		Output: "test",
	}, metrics...)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Scores) != 3 {
		t.Fatalf("expected 3 scores, got %d", len(result.Scores))
	}

	expected := map[string]float64{
		"metric1": 0.9,
		"metric2": 0.75,
		"metric3": 1.0,
	}

	for _, score := range result.Scores {
		if expected[score.Name] != score.Score {
			t.Errorf("expected %s score %f, got %f", score.Name, expected[score.Name], score.Score)
		}
	}
}

func TestEvaluator_Evaluate_MetricError(t *testing.T) {
	e := NewEvaluatorWithOptions(nil, WithRecordResults(false))

	metric := &mockMetric{
		name: "error_metric",
		err:  errTestMetric,
	}

	result, err := e.Evaluate(t.Context(), llmops.EvalInput{
		Input:  "test",
		Output: "test",
	}, metric)

	// Evaluate should not return error, but record it in the score
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Scores) != 1 {
		t.Fatalf("expected 1 score, got %d", len(result.Scores))
	}
	if result.Scores[0].Error != "test metric error" {
		t.Errorf("expected error message, got '%s'", result.Scores[0].Error)
	}
}

var errTestMetric = &testError{msg: "test metric error"}

type testError struct {
	msg string
}

func (e *testError) Error() string { return e.msg }

func TestParseSpanAnnotatorKind(t *testing.T) {
	tests := []struct {
		source   string
		expected api.SpanAnnotationDataAnnotatorKind
	}{
		{"llm", api.SpanAnnotationDataAnnotatorKindLLM},
		{"LLM", api.SpanAnnotationDataAnnotatorKindLLM},
		{"code", api.SpanAnnotationDataAnnotatorKindCODE},
		{"CODE", api.SpanAnnotationDataAnnotatorKindCODE},
		{"human", api.SpanAnnotationDataAnnotatorKindHUMAN},
		{"HUMAN", api.SpanAnnotationDataAnnotatorKindHUMAN},
		{"", api.SpanAnnotationDataAnnotatorKindHUMAN},
		{"unknown", api.SpanAnnotationDataAnnotatorKindHUMAN},
	}

	for _, tc := range tests {
		t.Run(tc.source, func(t *testing.T) {
			result := parseSpanAnnotatorKind(tc.source)
			if result != tc.expected {
				t.Errorf("parseSpanAnnotatorKind(%q) = %v, want %v", tc.source, result, tc.expected)
			}
		})
	}
}

func TestParseTraceAnnotatorKind(t *testing.T) {
	tests := []struct {
		source   string
		expected api.TraceAnnotationDataAnnotatorKind
	}{
		{"llm", api.TraceAnnotationDataAnnotatorKindLLM},
		{"LLM", api.TraceAnnotationDataAnnotatorKindLLM},
		{"code", api.TraceAnnotationDataAnnotatorKindCODE},
		{"CODE", api.TraceAnnotationDataAnnotatorKindCODE},
		{"human", api.TraceAnnotationDataAnnotatorKindHUMAN},
		{"", api.TraceAnnotationDataAnnotatorKindHUMAN},
	}

	for _, tc := range tests {
		t.Run(tc.source, func(t *testing.T) {
			result := parseTraceAnnotatorKind(tc.source)
			if result != tc.expected {
				t.Errorf("parseTraceAnnotatorKind(%q) = %v, want %v", tc.source, result, tc.expected)
			}
		})
	}
}

func TestInferAnnotatorKind(t *testing.T) {
	tests := []struct {
		name     string
		score    llmops.MetricScore
		expected api.SpanAnnotationDataAnnotatorKind
	}{
		{
			name:     "no metadata",
			score:    llmops.MetricScore{Name: "test"},
			expected: api.SpanAnnotationDataAnnotatorKindCODE,
		},
		{
			name: "llm kind",
			score: llmops.MetricScore{
				Name:     "test",
				Metadata: map[string]any{"kind": "llm"},
			},
			expected: api.SpanAnnotationDataAnnotatorKindLLM,
		},
		{
			name: "LLM kind uppercase",
			score: llmops.MetricScore{
				Name:     "test",
				Metadata: map[string]any{"kind": "LLM"},
			},
			expected: api.SpanAnnotationDataAnnotatorKindLLM,
		},
		{
			name: "code kind",
			score: llmops.MetricScore{
				Name:     "test",
				Metadata: map[string]any{"kind": "code"},
			},
			expected: api.SpanAnnotationDataAnnotatorKindCODE,
		},
		{
			name: "human kind",
			score: llmops.MetricScore{
				Name:     "test",
				Metadata: map[string]any{"kind": "human"},
			},
			expected: api.SpanAnnotationDataAnnotatorKindHUMAN,
		},
		{
			name: "wrong metadata type",
			score: llmops.MetricScore{
				Name:     "test",
				Metadata: "not a map",
			},
			expected: api.SpanAnnotationDataAnnotatorKindCODE,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := inferAnnotatorKind(tc.score)
			if result != tc.expected {
				t.Errorf("inferAnnotatorKind() = %v, want %v", result, tc.expected)
			}
		})
	}
}

func TestBuildAnnotationResult(t *testing.T) {
	t.Run("with reason", func(t *testing.T) {
		result := buildAnnotationResult(0.85, "test reason")
		if !result.Score.Set {
			t.Error("expected score to be set")
		}
		if result.Score.Value != 0.85 {
			t.Errorf("expected score 0.85, got %f", result.Score.Value)
		}
		if !result.Explanation.Set {
			t.Error("expected explanation to be set")
		}
		if result.Explanation.Value != "test reason" {
			t.Errorf("expected explanation 'test reason', got '%s'", result.Explanation.Value)
		}
	})

	t.Run("without reason", func(t *testing.T) {
		result := buildAnnotationResult(1.0, "")
		if !result.Score.Set {
			t.Error("expected score to be set")
		}
		if result.Score.Value != 1.0 {
			t.Errorf("expected score 1.0, got %f", result.Score.Value)
		}
		if result.Explanation.Set {
			t.Error("expected explanation to not be set")
		}
	})
}
