// Package llmops provides an omniobserve/llmops adapter for go-phoenix.
//
// Import this package to register the Phoenix provider:
//
//	import _ "github.com/plexusone/phoenix-go/llmops"
//
// Then open it:
//
//	provider, err := llmops.Open("phoenix", llmops.WithEndpoint("http://localhost:6006"))
//
// This adapter uses phoenix-otel behind the scenes to send traces via OpenTelemetry.
// Traces are automatically batched and sent to Phoenix with OpenInference semantic
// conventions for full LLM observability support.
package llmops

import (
	"context"
	"time"

	"github.com/plexusone/omniobserve/llmops"
	"github.com/plexusone/phoenix-go"
	phoenixotel "github.com/plexusone/phoenix-go/otel"
	"go.opentelemetry.io/otel/trace"
)

const ProviderName = "phoenix"

func init() {
	llmops.Register(ProviderName, New)
	llmops.RegisterInfo(llmops.ProviderInfo{
		Name:        ProviderName,
		Description: "Arize Phoenix - Open-source LLM observability platform",
		Website:     "https://phoenix.arize.com",
		OpenSource:  true,
		SelfHosted:  true,
		Capabilities: []llmops.Capability{
			llmops.CapabilityTracing,
			llmops.CapabilityEvaluation,
			llmops.CapabilityDatasets,
			llmops.CapabilityExperiments,
			llmops.CapabilityOTel,
		},
	})
}

// Provider implements llmops.Provider for Phoenix using phoenix-otel for tracing.
type Provider struct {
	client       *phoenix.Client
	tp           *phoenixotel.TracerProvider
	tracer       trace.Tracer
	projectName  string
	serviceName  string
	batchEnabled bool
}

// New creates a new Phoenix provider.
func New(opts ...llmops.ClientOption) (llmops.Provider, error) {
	cfg := llmops.ApplyClientOptions(opts...)

	// Map llmops options to phoenix REST client options
	phoenixOpts := []phoenix.Option{}
	if cfg.Endpoint != "" {
		phoenixOpts = append(phoenixOpts, phoenix.WithURL(cfg.Endpoint))
	}
	// Map Workspace to SpaceID for Phoenix Cloud
	if cfg.Workspace != "" {
		phoenixOpts = append(phoenixOpts, phoenix.WithSpaceID(cfg.Workspace))
	}
	if cfg.APIKey != "" {
		phoenixOpts = append(phoenixOpts, phoenix.WithAPIKey(cfg.APIKey))
	}
	if cfg.HTTPClient != nil {
		phoenixOpts = append(phoenixOpts, phoenix.WithHTTPClient(cfg.HTTPClient))
	}
	if cfg.Timeout > 0 {
		phoenixOpts = append(phoenixOpts, phoenix.WithTimeout(cfg.Timeout))
	}

	client, err := phoenix.NewClient(phoenixOpts...)
	if err != nil {
		return nil, err
	}

	// Map llmops options to phoenix-otel options
	otelOpts := []phoenixotel.Option{
		phoenixotel.WithBatch(true),           // Enable batching by default
		phoenixotel.WithGlobalProvider(false), // Don't set as global
	}
	if cfg.Endpoint != "" {
		otelOpts = append(otelOpts, phoenixotel.WithEndpoint(cfg.Endpoint))
	}
	// Map Workspace to SpaceID for Phoenix Cloud
	if cfg.Workspace != "" {
		otelOpts = append(otelOpts, phoenixotel.WithSpaceID(cfg.Workspace))
	}
	if cfg.APIKey != "" {
		otelOpts = append(otelOpts, phoenixotel.WithAPIKey(cfg.APIKey))
	}
	if cfg.ProjectName != "" {
		otelOpts = append(otelOpts, phoenixotel.WithProjectName(cfg.ProjectName))
	}

	// Determine service name
	serviceName := cfg.ProjectName
	if serviceName == "" {
		serviceName = "phoenix-llmops"
	}
	otelOpts = append(otelOpts, phoenixotel.WithServiceName(serviceName))

	// Register phoenix-otel tracer provider
	tp, err := phoenixotel.Register(otelOpts...)
	if err != nil {
		return nil, err
	}

	return &Provider{
		client:       client,
		tp:           tp,
		tracer:       tp.Tracer(serviceName),
		projectName:  cfg.ProjectName,
		serviceName:  serviceName,
		batchEnabled: true,
	}, nil
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return ProviderName
}

// Close closes the provider and flushes pending traces.
func (p *Provider) Close() error {
	if p.tp != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return p.tp.Shutdown(ctx)
	}
	return nil
}

// StartTrace starts a new trace.
func (p *Provider) StartTrace(ctx context.Context, name string, opts ...llmops.TraceOption) (context.Context, llmops.Trace, error) {
	cfg := llmops.ApplyTraceOptions(opts...)

	// Start OTEL span as root
	ctx, otelSpan := p.tracer.Start(ctx, name)

	// Create our trace wrapper
	t := newTrace(p, name, otelSpan, cfg)

	// Store trace in context
	ctx = contextWithTrace(ctx, t)

	return ctx, t, nil
}

// StartSpan starts a new span.
func (p *Provider) StartSpan(ctx context.Context, name string, opts ...llmops.SpanOption) (context.Context, llmops.Span, error) {
	cfg := llmops.ApplySpanOptions(opts...)

	// Get parent info from context
	var parentTraceID, parentSpanID string
	if t := traceFromContext(ctx); t != nil {
		parentTraceID = t.ID()
	}
	if s := spanFromContext(ctx); s != nil {
		parentSpanID = s.ID()
		if parentTraceID == "" {
			parentTraceID = s.TraceID()
		}
	}

	// Start OTEL span (automatically links to parent via context)
	ctx, otelSpan := p.tracer.Start(ctx, name)

	// Create our span wrapper
	s := newSpan(p, name, otelSpan, parentTraceID, parentSpanID, cfg)

	// Store span in context
	ctx = contextWithSpan(ctx, s)

	return ctx, s, nil
}

// TraceFromContext gets the current trace from context.
func (p *Provider) TraceFromContext(ctx context.Context) (llmops.Trace, bool) {
	t := traceFromContext(ctx)
	if t == nil {
		return nil, false
	}
	return t, true
}

// SpanFromContext gets the current span from context.
func (p *Provider) SpanFromContext(ctx context.Context) (llmops.Span, bool) {
	s := spanFromContext(ctx)
	if s == nil {
		return nil, false
	}
	return s, true
}

// Evaluate runs evaluation metrics.
func (p *Provider) Evaluate(ctx context.Context, input llmops.EvalInput, metrics ...llmops.Metric) (*llmops.EvalResult, error) {
	startTime := time.Now()

	scores := make([]llmops.MetricScore, 0, len(metrics))
	for _, metric := range metrics {
		score, err := metric.Evaluate(input)
		if err != nil {
			scores = append(scores, llmops.MetricScore{
				Name:  metric.Name(),
				Error: err.Error(),
			})
		} else {
			scores = append(scores, score)
		}
	}

	return &llmops.EvalResult{
		Scores:   scores,
		Duration: time.Since(startTime),
	}, nil
}

// AddFeedbackScore adds a feedback score.
func (p *Provider) AddFeedbackScore(ctx context.Context, opts llmops.FeedbackScoreOpts) error {
	if s := spanFromContext(ctx); s != nil {
		return s.AddFeedbackScore(ctx, opts.Name, opts.Score)
	}
	if t := traceFromContext(ctx); t != nil {
		return t.AddFeedbackScore(ctx, opts.Name, opts.Score)
	}
	return llmops.ErrNoActiveTrace
}

// CreatePrompt creates a new prompt template.
func (p *Provider) CreatePrompt(ctx context.Context, name string, template string, opts ...llmops.PromptOption) (*llmops.Prompt, error) {
	cfg := &llmops.PromptOptions{}
	for _, opt := range opts {
		opt(cfg)
	}

	// Validate required model information
	if cfg.ModelName == "" {
		return nil, &phoenix.APIError{Message: "model name is required (use llmops.WithPromptModel)"}
	}
	if cfg.ModelProvider == "" {
		return nil, &phoenix.APIError{Message: "model provider is required (use llmops.WithPromptProvider)"}
	}

	var phoenixOpts []phoenix.PromptOption
	if cfg.Description != "" {
		phoenixOpts = append(phoenixOpts, phoenix.WithPromptDescription(cfg.Description))
	}

	// Map provider string to Phoenix model provider
	modelProvider := phoenix.PromptModelProvider(cfg.ModelProvider)

	pv, err := p.client.CreatePrompt(ctx, name, template, cfg.ModelName, modelProvider, phoenixOpts...)
	if err != nil {
		return nil, err
	}

	return &llmops.Prompt{
		ID:            pv.ID,
		Name:          name,
		Template:      pv.Template,
		Description:   pv.Description,
		ModelName:     pv.ModelName,
		ModelProvider: string(pv.ModelProvider),
	}, nil
}

// GetPrompt retrieves a prompt by name, optionally at a specific version or tag.
// The version parameter can be:
//   - Empty/omitted: returns the latest version
//   - A tag name (e.g., "production", "staging"): returns the version with that tag
//   - A version ID: returns that specific version
func (p *Provider) GetPrompt(ctx context.Context, name string, version ...string) (*llmops.Prompt, error) {
	var pv *phoenix.PromptVersion
	var err error

	if len(version) > 0 && version[0] != "" {
		// First try as a tag name
		pv, err = p.client.GetPromptVersionByTag(ctx, name, version[0])
		if err != nil {
			// If tag lookup fails, try as a version ID
			pv, err = p.client.GetPromptVersion(ctx, version[0])
			if err != nil {
				return nil, err
			}
		}
	} else {
		// Get latest version
		pv, err = p.client.GetPromptLatest(ctx, name)
		if err != nil {
			return nil, err
		}
	}

	return &llmops.Prompt{
		ID:            pv.ID,
		Name:          name,
		Version:       pv.ID, // Use version ID as the version string
		Template:      pv.Template,
		Description:   pv.Description,
		ModelName:     pv.ModelName,
		ModelProvider: string(pv.ModelProvider),
	}, nil
}

// ListPrompts lists prompts.
func (p *Provider) ListPrompts(ctx context.Context, opts ...llmops.ListOption) ([]*llmops.Prompt, error) {
	prompts, _, err := p.client.ListPrompts(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*llmops.Prompt, len(prompts))
	for i, prompt := range prompts {
		result[i] = &llmops.Prompt{
			ID:          prompt.ID,
			Name:        prompt.Name,
			Description: prompt.Description,
		}
	}
	return result, nil
}

// CreateDataset creates a new dataset.
func (p *Provider) CreateDataset(ctx context.Context, name string, opts ...llmops.DatasetOption) (*llmops.Dataset, error) {
	cfg := &llmops.DatasetOptions{}
	for _, opt := range opts {
		opt(cfg)
	}

	var phoenixOpts []phoenix.DatasetOption
	if cfg.Description != "" {
		phoenixOpts = append(phoenixOpts, phoenix.WithDatasetDescription(cfg.Description))
	}

	// Create empty dataset (Phoenix requires examples, pass empty slice)
	ds, err := p.client.CreateDataset(ctx, name, []phoenix.DatasetExample{}, phoenixOpts...)
	if err != nil {
		return nil, err
	}

	return &llmops.Dataset{
		ID:   ds.ID,
		Name: ds.Name,
	}, nil
}

// GetDataset gets a dataset by name.
func (p *Provider) GetDataset(ctx context.Context, name string) (*llmops.Dataset, error) {
	// List datasets and find by name
	datasets, _, err := p.client.ListDatasets(ctx)
	if err != nil {
		return nil, err
	}

	for _, ds := range datasets {
		if ds.Name == name {
			return &llmops.Dataset{
				ID:          ds.ID,
				Name:        ds.Name,
				Description: ds.Description,
				ItemCount:   ds.ExampleCount,
				CreatedAt:   ds.CreatedAt,
				UpdatedAt:   ds.UpdatedAt,
			}, nil
		}
	}
	return nil, llmops.ErrDatasetNotFound
}

// GetDatasetByID gets a dataset by ID.
func (p *Provider) GetDatasetByID(ctx context.Context, id string) (*llmops.Dataset, error) {
	ds, err := p.client.GetDataset(ctx, id)
	if err != nil {
		return nil, err
	}

	return &llmops.Dataset{
		ID:          ds.ID,
		Name:        ds.Name,
		Description: ds.Description,
		ItemCount:   ds.ExampleCount,
		CreatedAt:   ds.CreatedAt,
		UpdatedAt:   ds.UpdatedAt,
	}, nil
}

// AddDatasetItems adds items to a dataset.
func (p *Provider) AddDatasetItems(ctx context.Context, datasetName string, items []llmops.DatasetItem) error {
	// Convert omniobserve DatasetItem to phoenix DatasetExample
	examples := make([]phoenix.DatasetExample, len(items))
	for i, item := range items {
		examples[i] = phoenix.DatasetExample{
			Input:    item.Input,
			Output:   item.Expected, // omniobserve uses "Expected", Phoenix uses "Output"
			Metadata: item.Metadata,
		}
	}

	return p.client.AddDatasetExamples(ctx, datasetName, examples)
}

// ListDatasets lists datasets.
func (p *Provider) ListDatasets(ctx context.Context, opts ...llmops.ListOption) ([]*llmops.Dataset, error) {
	datasets, _, err := p.client.ListDatasets(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*llmops.Dataset, len(datasets))
	for i, ds := range datasets {
		result[i] = &llmops.Dataset{
			ID:          ds.ID,
			Name:        ds.Name,
			Description: ds.Description,
			ItemCount:   ds.ExampleCount,
			CreatedAt:   ds.CreatedAt,
			UpdatedAt:   ds.UpdatedAt,
		}
	}
	return result, nil
}

// CreateProject creates a new project.
func (p *Provider) CreateProject(ctx context.Context, name string, opts ...llmops.ProjectOption) (*llmops.Project, error) {
	cfg := &llmops.ProjectOptions{}
	for _, opt := range opts {
		opt(cfg)
	}

	var phoenixOpts []phoenix.ProjectOption
	if cfg.Description != "" {
		phoenixOpts = append(phoenixOpts, phoenix.WithDescription(cfg.Description))
	}

	project, err := p.client.CreateProject(ctx, name, phoenixOpts...)
	if err != nil {
		return nil, err
	}

	return &llmops.Project{
		ID:          project.ID,
		Name:        project.Name,
		Description: project.Description,
		CreatedAt:   project.CreatedAt,
		UpdatedAt:   project.UpdatedAt,
	}, nil
}

// GetProject gets a project by name.
func (p *Provider) GetProject(ctx context.Context, name string) (*llmops.Project, error) {
	project, err := p.client.GetProject(ctx, name)
	if err != nil {
		return nil, err
	}

	return &llmops.Project{
		ID:          project.ID,
		Name:        project.Name,
		Description: project.Description,
		CreatedAt:   project.CreatedAt,
		UpdatedAt:   project.UpdatedAt,
	}, nil
}

// ListProjects lists projects.
func (p *Provider) ListProjects(ctx context.Context, opts ...llmops.ListOption) ([]*llmops.Project, error) {
	projects, _, err := p.client.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*llmops.Project, len(projects))
	for i, proj := range projects {
		result[i] = &llmops.Project{
			ID:          proj.ID,
			Name:        proj.Name,
			Description: proj.Description,
			CreatedAt:   proj.CreatedAt,
			UpdatedAt:   proj.UpdatedAt,
		}
	}
	return result, nil
}

// SetProject sets the current project.
func (p *Provider) SetProject(ctx context.Context, name string) error {
	p.projectName = name
	return nil
}

// DeleteDataset deletes a dataset by ID.
func (p *Provider) DeleteDataset(ctx context.Context, datasetID string) error {
	return p.client.DeleteDataset(ctx, datasetID)
}

// CreateAnnotation creates an annotation on a span or trace.
func (p *Provider) CreateAnnotation(ctx context.Context, annotation llmops.Annotation) error {
	// Map annotator kind
	var source phoenix.AnnotatorKind
	switch annotation.Source {
	case llmops.AnnotatorKindLLM:
		source = phoenix.AnnotatorKindLLM
	case llmops.AnnotatorKindCode:
		source = phoenix.AnnotatorKindCode
	default:
		source = phoenix.AnnotatorKindHuman
	}

	opts := []phoenix.AnnotationOption{
		phoenix.WithAnnotationSource(source),
	}
	if annotation.Explanation != "" {
		opts = append(opts, phoenix.WithAnnotationExplanation(annotation.Explanation))
	}
	if annotation.Label != "" {
		opts = append(opts, phoenix.WithAnnotationLabel(annotation.Label))
	}

	if annotation.SpanID != "" {
		return p.client.CreateSpanAnnotation(ctx, annotation.SpanID, annotation.Name, annotation.Score, opts...)
	}
	if annotation.TraceID != "" {
		return p.client.CreateTraceAnnotation(ctx, annotation.TraceID, annotation.Name, annotation.Score, opts...)
	}
	return &phoenix.APIError{Message: "either SpanID or TraceID must be set"}
}

// ListAnnotations lists annotations for spans or traces.
func (p *Provider) ListAnnotations(ctx context.Context, opts llmops.ListAnnotationsOptions) ([]*llmops.Annotation, error) {
	var result []*llmops.Annotation

	if len(opts.SpanIDs) > 0 {
		annotations, err := p.client.ListSpanAnnotations(ctx, opts.SpanIDs)
		if err != nil {
			return nil, err
		}
		for _, ann := range annotations {
			result = append(result, convertAnnotation(ann))
		}
	}

	if len(opts.TraceIDs) > 0 {
		annotations, err := p.client.ListTraceAnnotations(ctx, opts.TraceIDs)
		if err != nil {
			return nil, err
		}
		for _, ann := range annotations {
			result = append(result, convertAnnotation(ann))
		}
	}

	return result, nil
}

func convertAnnotation(ann *phoenix.Annotation) *llmops.Annotation {
	if ann == nil {
		return nil
	}
	var source llmops.AnnotatorKind
	switch ann.Source {
	case phoenix.AnnotatorKindLLM:
		source = llmops.AnnotatorKindLLM
	case phoenix.AnnotatorKindCode:
		source = llmops.AnnotatorKindCode
	default:
		source = llmops.AnnotatorKindHuman
	}
	return &llmops.Annotation{
		ID:          ann.ID,
		SpanID:      ann.SpanID,
		TraceID:     ann.TraceID,
		Name:        ann.Name,
		Score:       ann.Score,
		Label:       ann.Label,
		Explanation: ann.Explanation,
		Source:      source,
		CreatedAt:   ann.CreatedAt,
		UpdatedAt:   ann.UpdatedAt,
	}
}
