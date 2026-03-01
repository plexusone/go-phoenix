package phoenix

import (
	"context"
	"time"

	"github.com/plexusone/phoenix-go/internal/api"
)

// Project represents a Phoenix project.
type Project struct {
	ID          string
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ListProjects lists all projects.
func (c *Client) ListProjects(ctx context.Context, opts ...ListOption) ([]*Project, string, error) { //nolint:dupl // Type-safe pattern differs only in types
	options := defaultListOptions()
	for _, opt := range opts {
		opt(options)
	}

	params := api.GetProjectsParams{}
	if options.cursor != "" {
		params.Cursor.SetTo(options.cursor)
	}
	if options.limit > 0 {
		params.Limit.SetTo(options.limit)
	}

	res, err := c.apiClient.GetProjects(ctx, params)
	if err != nil {
		return nil, "", err
	}

	resp, ok := res.(*api.GetProjectsResponseBody)
	if !ok {
		return nil, "", &APIError{Message: "unexpected response type"}
	}

	projects := make([]*Project, 0, len(resp.Data))
	for i := range resp.Data {
		projects = append(projects, convertProject(&resp.Data[i]))
	}

	var nextCursor string
	if !resp.NextCursor.Null {
		nextCursor = resp.NextCursor.Value
	}

	return projects, nextCursor, nil
}

// GetProject retrieves a project by identifier (ID or name).
func (c *Client) GetProject(ctx context.Context, identifier string) (*Project, error) {
	res, err := c.apiClient.GetProject(ctx, api.GetProjectParams{
		ProjectIdentifier: identifier,
	})
	if err != nil {
		return nil, err
	}

	resp, ok := res.(*api.GetProjectResponseBody)
	if !ok {
		return nil, &APIError{Message: "unexpected response type"}
	}

	return &Project{
		ID:   resp.Data.ID,
		Name: resp.Data.Name,
	}, nil
}

// CreateProject creates a new project.
func (c *Client) CreateProject(ctx context.Context, name string, opts ...ProjectOption) (*Project, error) {
	options := &projectOptions{}
	for _, opt := range opts {
		opt(options)
	}

	req := api.CreateProjectRequestBody{
		Name: name,
	}
	if options.description != "" {
		req.Description.SetTo(options.description)
	}

	res, err := c.apiClient.CreateProject(ctx, &req)
	if err != nil {
		return nil, err
	}

	resp, ok := res.(*api.CreateProjectResponseBody)
	if !ok {
		return nil, &APIError{Message: "unexpected response type"}
	}

	return &Project{
		ID:   resp.Data.ID,
		Name: resp.Data.Name,
	}, nil
}

// DeleteProject deletes a project by identifier.
func (c *Client) DeleteProject(ctx context.Context, identifier string) error {
	_, err := c.apiClient.DeleteProject(ctx, api.DeleteProjectParams{
		ProjectIdentifier: identifier,
	})
	return err
}

// ProjectOption is a functional option for project operations.
type ProjectOption func(*projectOptions)

type projectOptions struct {
	name        string
	description string
}

// WithName sets the project name.
func WithName(name string) ProjectOption {
	return func(o *projectOptions) {
		o.name = name
	}
}

// WithDescription sets the project description.
func WithDescription(desc string) ProjectOption {
	return func(o *projectOptions) {
		o.description = desc
	}
}

func convertProject(p *api.Project) *Project {
	if p == nil {
		return nil
	}
	project := &Project{
		ID:   p.ID,
		Name: p.Name,
	}
	if p.Description.Set && !p.Description.Null {
		project.Description = p.Description.Value
	}
	return project
}
