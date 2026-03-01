package phoenix

import (
	"os"
	"strings"
)

const (
	// DefaultURL is the default Phoenix API URL for local instances.
	DefaultURL = "http://localhost:6006"

	// DefaultProjectName is the default project name.
	DefaultProjectName = "default"
)

// Environment variable names.
const (
	EnvURL         = "PHOENIX_URL"
	EnvAPIKey      = "PHOENIX_API_KEY" //nolint:gosec // G101: This is an environment variable name, not a credential
	EnvProjectName = "PHOENIX_PROJECT_NAME"
	EnvSpaceID     = "PHOENIX_SPACE_ID"
)

// Config holds the configuration for the Phoenix client.
type Config struct {
	// URL is the Phoenix API endpoint URL.
	// Defaults to DefaultURL (http://localhost:6006).
	// For Phoenix Cloud, use https://app.phoenix.arize.com
	URL string

	// SpaceID is the space identifier for Phoenix Cloud.
	// When set, the URL is constructed as {URL}/s/{SpaceID}.
	// Not needed for self-hosted Phoenix instances.
	SpaceID string

	// APIKey is the API key for authentication.
	// Optional for local instances, may be required for hosted instances.
	APIKey string //nolint:gosec // G117: SDK config needs API key field

	// ProjectName is the default project name for operations.
	ProjectName string
}

// NewConfig creates a new Config with default values.
func NewConfig() *Config {
	return &Config{
		URL:         DefaultURL,
		APIKey:      "",
		ProjectName: DefaultProjectName,
	}
}

// LoadConfig loads configuration from environment variables.
// Priority order (highest to lowest):
//  1. Explicitly set values (via options)
//  2. Environment variables
//  3. Default values
func LoadConfig() *Config {
	cfg := NewConfig()
	cfg.loadFromEnv()
	return cfg
}

// loadFromEnv loads configuration from environment variables.
func (c *Config) loadFromEnv() {
	if url := os.Getenv(EnvURL); url != "" {
		c.URL = url
	}
	if spaceID := os.Getenv(EnvSpaceID); spaceID != "" {
		c.SpaceID = spaceID
		// Default to Phoenix Cloud when SpaceID is set
		if c.URL == DefaultURL {
			c.URL = "https://app.phoenix.arize.com"
		}
	}
	if apiKey := os.Getenv(EnvAPIKey); apiKey != "" {
		c.APIKey = apiKey
	}
	if projectName := os.Getenv(EnvProjectName); projectName != "" {
		c.ProjectName = projectName
	}
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c.URL == "" {
		return ErrMissingURL
	}
	// Normalize URL - remove trailing slash
	c.URL = strings.TrimSuffix(c.URL, "/")

	// If SpaceID is set, append it to the URL for Phoenix Cloud
	if c.SpaceID != "" {
		c.URL = c.URL + "/s/" + c.SpaceID
	}
	return nil
}

// BaseURL returns the effective base URL (with space if configured).
func (c *Config) BaseURL() string {
	return c.URL
}
