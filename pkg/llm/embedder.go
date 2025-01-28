package llm

import (
	"fmt"

	"github.com/tmc/langchaingo/llms/ollama"
)

// ChatConfig represents the configuration for a chat engine.
type EmbedderConfig struct {
	Model     string
	MaxTokens int
	BaseURL   string // Ollama server URL
}

// ChatEngine is an engine that uses an LLM to generate chat responses.
type Embedder struct {
	Config EmbedderConfig
	Embed  *ollama.LLM
}

func NewEmbedderWithConfig(config EmbedderConfig) Embedder {
	// Validate and set default values for config fields if necessary
	if config.Model == "" {
		config.Model = "nomic-embed-text:latest" // Default Ollama model
	}

	if config.MaxTokens < 0 {
		config.MaxTokens = 2000
	}

	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:11434" // Default Ollama URL
	}

	modelOptions := ollama.WithModel(config.Model)

	serverOptions := ollama.WithServerURL(config.BaseURL)

	emb, err := ollama.New(modelOptions, serverOptions)

	if err != nil {
		fmt.Errorf("failed to initialize LLM: %w", err)
	}

	return Embedder{
		Config: config,
		Embed:  emb,
	}
}

func NewEmbedder() Embedder {

	var config = EmbedderConfig{
		Model:     "nomic-embed-text:latest",
		MaxTokens: 1000,
		BaseURL:   "http://localhost:11434",
	}

	modelOptions := ollama.WithModel(config.Model)

	serverOptions := ollama.WithServerURL(config.BaseURL)

	emb, err := ollama.New(modelOptions, serverOptions)

	if err != nil {
		fmt.Errorf("failed to initialize LLM: %w", err)
	}

	return Embedder{
		Config: config,
		Embed:  emb,
	}
}

func (e *Embedder) FlattenEmbeddings(embeddings [][]float32) []float32 {
	var flattened []float32
	for _, emb := range embeddings {
		flattened = append(flattened, emb...)
	}
	return flattened
}
