// Package phoenix provides a Go client for the Arize Phoenix API.
//
// Phoenix is an open-source AI observability platform for experimentation,
// evaluation, and troubleshooting of LLM applications.
//
// The client wraps the ogen-generated API client with a higher-level
// interface that handles authentication and provides convenient methods
// for common operations.
package phoenix

import (
	"net/http"

	"github.com/plexusone/phoenix-go/internal/api"
)

// Version is the SDK version.
const Version = "0.1.0"

// Client is the main Phoenix client for interacting with the API.
type Client struct {
	config    *Config
	apiClient *api.Client
}

// NewClient creates a new Phoenix client with the given options.
func NewClient(opts ...Option) (*Client, error) {
	options := defaultClientOptions()
	for _, opt := range opts {
		opt(options)
	}

	if err := options.config.Validate(); err != nil {
		return nil, err
	}

	// Create HTTP client with auth headers
	httpClient := options.httpClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: options.timeout,
		}
	}

	// Wrap with auth transport
	authClient := &authHTTPClient{
		client: httpClient,
		apiKey: options.config.APIKey,
	}

	// Create the ogen client
	apiClient, err := api.NewClient(
		options.config.URL,
		api.WithClient(authClient),
	)
	if err != nil {
		return nil, err
	}

	return &Client{
		config:    options.config,
		apiClient: apiClient,
	}, nil
}

// authHTTPClient wraps an http.Client to add authentication headers.
type authHTTPClient struct {
	client *http.Client
	apiKey string
}

// Do implements ht.Client interface.
func (c *authHTTPClient) Do(req *http.Request) (*http.Response, error) {
	// Add authentication header if API key is set
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	// Add SDK version headers
	req.Header.Set("X-Phoenix-SDK-Version", Version)
	req.Header.Set("X-Phoenix-SDK-Lang", "go")

	return c.client.Do(req) //nolint:gosec // G704: URL is configured by SDK user
}

// Config returns the client configuration.
func (c *Client) Config() *Config {
	return c.config
}

// ProjectName returns the default project name.
func (c *Client) ProjectName() string {
	return c.config.ProjectName
}

// SetProjectName sets the default project name.
func (c *Client) SetProjectName(name string) {
	c.config.ProjectName = name
}

// API returns the underlying ogen-generated API client for advanced usage.
// Use this when you need access to API endpoints not covered by the
// high-level wrapper methods.
func (c *Client) API() *api.Client {
	return c.apiClient
}
