package phoenix

import (
	"context"
	"time"

	"github.com/plexusone/phoenix-go/internal/api"
)

// Experiment represents a Phoenix experiment.
type Experiment struct {
	ID                 string
	DatasetID          string
	DatasetVersionID   string
	ProjectName        string
	ExampleCount       int
	SuccessfulRunCount int
	FailedRunCount     int
	MissingRunCount    int
	Repetitions        int
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// ListExperiments lists experiments for a dataset.
func (c *Client) ListExperiments(ctx context.Context, datasetID string, opts ...ListOption) ([]*Experiment, string, error) { //nolint:dupl // Type-safe pattern differs only in types
	options := defaultListOptions()
	for _, opt := range opts {
		opt(options)
	}

	params := api.ListExperimentsParams{
		DatasetID: datasetID,
	}
	if options.cursor != "" {
		params.Cursor.SetTo(options.cursor)
	}
	if options.limit > 0 {
		params.Limit.SetTo(options.limit)
	}

	res, err := c.apiClient.ListExperiments(ctx, params)
	if err != nil {
		return nil, "", err
	}

	resp, ok := res.(*api.ListExperimentsResponseBody)
	if !ok {
		return nil, "", &APIError{Message: "unexpected response type"}
	}

	experiments := make([]*Experiment, 0, len(resp.Data))
	for i := range resp.Data {
		experiments = append(experiments, convertExperiment(&resp.Data[i]))
	}

	var nextCursor string
	if !resp.NextCursor.Null {
		nextCursor = resp.NextCursor.Value
	}

	return experiments, nextCursor, nil
}

// DeleteExperiment deletes an experiment.
func (c *Client) DeleteExperiment(ctx context.Context, experimentID string) error {
	_, err := c.apiClient.DeleteExperiment(ctx, api.DeleteExperimentParams{
		ExperimentID: experimentID,
	})
	return err
}

func convertExperiment(e *api.Experiment) *Experiment {
	if e == nil {
		return nil
	}
	exp := &Experiment{
		ID:                 e.ID,
		DatasetID:          e.DatasetID,
		DatasetVersionID:   e.DatasetVersionID,
		ExampleCount:       e.ExampleCount,
		SuccessfulRunCount: e.SuccessfulRunCount,
		FailedRunCount:     e.FailedRunCount,
		MissingRunCount:    e.MissingRunCount,
		Repetitions:        e.Repetitions,
		CreatedAt:          e.CreatedAt,
		UpdatedAt:          e.UpdatedAt,
	}
	if !e.ProjectName.Null {
		exp.ProjectName = e.ProjectName.Value
	}
	return exp
}
