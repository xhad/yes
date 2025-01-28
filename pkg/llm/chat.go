package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/xhad/yes/internal/models"
)

// ChatConfig represents the configuration for a chat engine.
type ChatConfig struct {
	Model           string
	Temperature     float64
	MaxTokens       int
	SystemTemplate  string
	ContextTemplate string
	BaseURL         string // Ollama server URL
}

// ChatEngine is an engine that uses an LLM to generate chat responses.
type ChatEngine struct {
	config ChatConfig
	llm    llms.Model
}

// NewWithConfig creates a new ChatEngine with the given configuration.
func NewWithConfig(config ChatConfig) (*ChatEngine, error) {
	// Validate and set default values for config fields if necessary
	if config.Model == "" {
		config.Model = "mistral" // Default Ollama model
	}
	if config.Temperature <= 0 || config.Temperature > 1 {
		return nil, fmt.Errorf("temperature must be between 0 and 1")
	}
	if config.MaxTokens < 0 {
		return nil, fmt.Errorf("max tokens cannot be negative")
	} else if config.MaxTokens == 0 {
		config.MaxTokens = 2000
	}
	if config.SystemTemplate == "" {
		config.SystemTemplate = "You are a helpful assistant with access to the following documentation. Answer questions based on this context."
	}
	if config.ContextTemplate == "" {
		config.ContextTemplate = "\nRelevant documentation:\n%s\n\nQuestion: %s"
	}
	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:11434" // Default Ollama URL
	}

	llm, err := ollama.New(ollama.WithModel(config.Model),
		ollama.WithServerURL(config.BaseURL))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize LLM: %w", err)
	}

	return &ChatEngine{
		config: config,
		llm:    llm,
	}, nil
}

// Chat generates a response based on the query and context documents.
func (ce *ChatEngine) Chat(query string, docs []models.Document) (*llms.ContentResponse, error) {
	var response *llms.ContentResponse

	var contextBuilder strings.Builder

	for _, doc := range docs {
		contextBuilder.WriteString(fmt.Sprintf("Source: %s\n%s\n\n", doc.URL, doc.Content))
	}

	content := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, ce.config.SystemTemplate),
		llms.TextParts(llms.ChatMessageTypeHuman, query),
	}

	ctx := context.Background()

	response, err := ce.llm.GenerateContent(ctx, content)

	if err != nil {
		return response, fmt.Errorf("chat error: %w", err)
	}

	return response, nil
}

// ChatStream generates a stream of responses based on the query and context documents.
func (ce *ChatEngine) ChatStream(query string, docs []models.Document) (<-chan string, error) {
	var contextBuilder strings.Builder
	for _, doc := range docs {
		contextBuilder.WriteString(fmt.Sprintf("Source: %s\n%s\n\n", doc.URL, doc.Content))
	}

	content := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, ce.config.SystemTemplate),
		llms.TextParts(llms.ChatMessageTypeHuman, query),
		llms.TextParts(llms.ChatMessageTypeHuman, contextBuilder.String()),
	}

	resultChan := make(chan string)

	go func() {
		defer close(resultChan)

		ctx := context.Background()
		stream, err := ce.llm.GenerateContent(ctx, content)
		if err != nil {
			resultChan <- fmt.Sprintf("Error: %v", err)
			return
		}

		if stream == nil {
			resultChan <- "Error: No response from LLM"
			return
		}

		for _, choice := range stream.Choices {
			if choice != nil && choice.Content != "" {
				resultChan <- choice.Content
			}
		}
	}()

	return resultChan, nil
}

// formatSources formats the sources for citation.
func (ce *ChatEngine) formatSources(docs []models.Document) string {
	if docs == nil {
		return ""
	}

	var sources []string
	seen := make(map[string]bool)

	for _, doc := range docs {
		if !seen[doc.URL] {
			sources = append(sources, doc.URL)
			seen[doc.URL] = true
		}
	}

	if len(sources) == 0 {
		return ""
	}

	return fmt.Sprintf("\nSources:\n%s", strings.Join(sources, "\n"))
}
