package phoenix

import (
	"context"

	"github.com/plexusone/phoenix-go/internal/api"
)

// Prompt represents a Phoenix prompt.
type Prompt struct {
	ID             string
	Name           string
	Description    string
	SourcePromptID string
}

// PromptVersion represents a version of a prompt.
type PromptVersion struct {
	ID            string
	Description   string
	Template      string // The prompt template content
	TemplateType  PromptTemplateType
	ModelName     string
	ModelProvider PromptModelProvider
}

// PromptTemplateType represents the type of prompt template.
type PromptTemplateType string

const (
	PromptTemplateTypeChat   PromptTemplateType = "CHAT"
	PromptTemplateTypeString PromptTemplateType = "STRING"
)

// PromptModelProvider represents the LLM provider for a prompt.
type PromptModelProvider string

const (
	PromptModelProviderOpenAI      PromptModelProvider = "OPENAI"
	PromptModelProviderAzureOpenAI PromptModelProvider = "AZURE_OPENAI"
	PromptModelProviderAnthropic   PromptModelProvider = "ANTHROPIC"
	PromptModelProviderGoogle      PromptModelProvider = "GOOGLE"
	PromptModelProviderDeepseek    PromptModelProvider = "DEEPSEEK"
	PromptModelProviderXAI         PromptModelProvider = "XAI"
	PromptModelProviderOllama      PromptModelProvider = "OLLAMA"
	PromptModelProviderAWS         PromptModelProvider = "AWS"
)

// PromptMessage represents a message in a chat prompt.
type PromptMessage struct {
	Role    string
	Content string
}

// PromptOption is a functional option for prompt operations.
type PromptOption func(*promptOptions)

type promptOptions struct {
	description string
}

// WithPromptDescription sets the prompt description.
func WithPromptDescription(desc string) PromptOption {
	return func(o *promptOptions) {
		o.description = desc
	}
}

// ListPrompts lists all prompts.
func (c *Client) ListPrompts(ctx context.Context, opts ...ListOption) ([]*Prompt, string, error) { //nolint:dupl // Type-safe pattern differs only in types
	options := defaultListOptions()
	for _, opt := range opts {
		opt(options)
	}

	params := api.GetPromptsParams{}
	if options.cursor != "" {
		params.Cursor.SetTo(options.cursor)
	}
	if options.limit > 0 {
		params.Limit.SetTo(options.limit)
	}

	res, err := c.apiClient.GetPrompts(ctx, params)
	if err != nil {
		return nil, "", err
	}

	resp, ok := res.(*api.GetPromptsResponseBody)
	if !ok {
		return nil, "", &APIError{Message: "unexpected response type"}
	}

	prompts := make([]*Prompt, 0, len(resp.Data))
	for i := range resp.Data {
		prompts = append(prompts, convertPrompt(&resp.Data[i]))
	}

	var nextCursor string
	if !resp.NextCursor.Null {
		nextCursor = resp.NextCursor.Value
	}

	return prompts, nextCursor, nil
}

func convertPrompt(p *api.Prompt) *Prompt {
	if p == nil {
		return nil
	}
	prompt := &Prompt{
		ID:   p.ID,
		Name: string(p.Name),
	}
	if p.Description.Set && !p.Description.Null {
		prompt.Description = p.Description.Value
	}
	if p.SourcePromptID.Set && !p.SourcePromptID.Null {
		prompt.SourcePromptID = p.SourcePromptID.Value
	}
	return prompt
}

// CreatePrompt creates a new prompt with the given template.
func (c *Client) CreatePrompt(ctx context.Context, name string, template string, modelName string, modelProvider PromptModelProvider, opts ...PromptOption) (*PromptVersion, error) {
	options := &promptOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Build prompt data
	promptData := api.PromptData{
		Name: api.Identifier(name),
	}
	if options.description != "" {
		promptData.Description.SetTo(options.description)
	}

	// Build version data with string template
	versionData := api.PromptVersionData{
		ModelName:            modelName,
		ModelProvider:        api.ModelProvider(modelProvider),
		TemplateFormat:       api.PromptTemplateFormatMUSTACHE,
		TemplateType:         api.PromptTemplateTypeSTR,
		InvocationParameters: api.PromptVersionDataInvocationParameters{},
	}

	// Set the string template
	versionData.Template.SetPromptStringTemplate(api.PromptStringTemplate{
		Template: template,
		Type:     api.PromptStringTemplateTypeString,
	})

	req := &api.CreatePromptRequestBody{
		Prompt:  promptData,
		Version: versionData,
	}

	res, err := c.apiClient.PostPromptVersion(ctx, req)
	if err != nil {
		return nil, err
	}

	resp, ok := res.(*api.CreatePromptResponseBody)
	if !ok {
		return nil, &APIError{Message: "unexpected response type"}
	}

	return convertPromptVersion(&resp.Data), nil
}

// CreateChatPrompt creates a new chat-style prompt with messages.
func (c *Client) CreateChatPrompt(ctx context.Context, name string, messages []PromptMessage, modelName string, modelProvider PromptModelProvider, opts ...PromptOption) (*PromptVersion, error) {
	options := &promptOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Build prompt data
	promptData := api.PromptData{
		Name: api.Identifier(name),
	}
	if options.description != "" {
		promptData.Description.SetTo(options.description)
	}

	// Convert messages
	apiMessages := make([]api.PromptMessage, len(messages))
	for i, msg := range messages {
		apiMessages[i] = api.PromptMessage{
			Role: api.PromptMessageRole(msg.Role),
		}
		// Set string content
		apiMessages[i].Content.SetString(msg.Content)
	}

	// Build version data with chat template
	versionData := api.PromptVersionData{
		ModelName:            modelName,
		ModelProvider:        api.ModelProvider(modelProvider),
		TemplateFormat:       api.PromptTemplateFormatMUSTACHE,
		TemplateType:         api.PromptTemplateTypeCHAT,
		InvocationParameters: api.PromptVersionDataInvocationParameters{},
	}

	// Set the chat template
	versionData.Template.SetPromptChatTemplate(api.PromptChatTemplate{
		Messages: apiMessages,
		Type:     api.PromptChatTemplateTypeChat,
	})

	req := &api.CreatePromptRequestBody{
		Prompt:  promptData,
		Version: versionData,
	}

	res, err := c.apiClient.PostPromptVersion(ctx, req)
	if err != nil {
		return nil, err
	}

	resp, ok := res.(*api.CreatePromptResponseBody)
	if !ok {
		return nil, &APIError{Message: "unexpected response type"}
	}

	return convertPromptVersion(&resp.Data), nil
}

// GetPromptLatest retrieves the latest version of a prompt by name.
func (c *Client) GetPromptLatest(ctx context.Context, name string) (*PromptVersion, error) {
	res, err := c.apiClient.GetPromptVersionLatest(ctx, api.GetPromptVersionLatestParams{
		PromptIdentifier: name,
	})
	if err != nil {
		return nil, err
	}

	resp, ok := res.(*api.GetPromptResponseBody)
	if !ok {
		return nil, &APIError{Message: "unexpected response type"}
	}

	return convertPromptVersion(&resp.Data), nil
}

// GetPromptVersion retrieves a specific prompt version by its ID.
func (c *Client) GetPromptVersion(ctx context.Context, versionID string) (*PromptVersion, error) {
	res, err := c.apiClient.GetPromptVersionByPromptVersionId(ctx, api.GetPromptVersionByPromptVersionIdParams{
		PromptVersionID: versionID,
	})
	if err != nil {
		return nil, err
	}

	resp, ok := res.(*api.GetPromptResponseBody)
	if !ok {
		return nil, &APIError{Message: "unexpected response type"}
	}

	return convertPromptVersion(&resp.Data), nil
}

// GetPromptVersionByTag retrieves a prompt version by its tag name.
func (c *Client) GetPromptVersionByTag(ctx context.Context, promptName, tagName string) (*PromptVersion, error) {
	res, err := c.apiClient.GetPromptVersionByTagName(ctx, api.GetPromptVersionByTagNameParams{
		PromptIdentifier: promptName,
		TagName:          tagName,
	})
	if err != nil {
		return nil, err
	}

	resp, ok := res.(*api.GetPromptResponseBody)
	if !ok {
		return nil, &APIError{Message: "unexpected response type"}
	}

	return convertPromptVersion(&resp.Data), nil
}

// ListPromptVersions lists all versions of a prompt.
func (c *Client) ListPromptVersions(ctx context.Context, promptName string, opts ...ListOption) ([]*PromptVersion, string, error) { //nolint:dupl // Type-safe pattern differs only in types
	options := defaultListOptions()
	for _, opt := range opts {
		opt(options)
	}

	params := api.ListPromptVersionsParams{
		PromptIdentifier: promptName,
	}
	if options.cursor != "" {
		params.Cursor.SetTo(options.cursor)
	}
	if options.limit > 0 {
		params.Limit.SetTo(options.limit)
	}

	res, err := c.apiClient.ListPromptVersions(ctx, params)
	if err != nil {
		return nil, "", err
	}

	resp, ok := res.(*api.GetPromptVersionsResponseBody)
	if !ok {
		return nil, "", &APIError{Message: "unexpected response type"}
	}

	versions := make([]*PromptVersion, 0, len(resp.Data))
	for i := range resp.Data {
		versions = append(versions, convertPromptVersion(&resp.Data[i]))
	}

	var nextCursor string
	if !resp.NextCursor.Null {
		nextCursor = resp.NextCursor.Value
	}

	return versions, nextCursor, nil
}

func convertPromptVersion(v *api.PromptVersion) *PromptVersion {
	if v == nil {
		return nil
	}
	pv := &PromptVersion{
		ID:            v.ID,
		ModelName:     v.ModelName,
		ModelProvider: PromptModelProvider(v.ModelProvider),
		TemplateType:  PromptTemplateType(v.TemplateType),
	}
	if !v.Description.Null {
		pv.Description = v.Description.Value
	}
	// Extract template content
	if v.Template.IsPromptStringTemplate() {
		pv.Template = v.Template.PromptStringTemplate.Template
	}
	// For chat templates, we would need to serialize the messages
	return pv
}
