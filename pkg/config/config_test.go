package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configData := `
llm:
  base_url: "http://localhost:11434"
  model: "gpt-4"
  max_tokens: 1000
  temperature: 0.5

database:
  url: "postgres://localhost:5432/test"
  table_name: "test_docs"
  vector_dim: 768
  batch_size: 50

scraper:
  max_depth: 5
  rate_limit: 1.5
  ignore_patterns:
    - "/test/"
  allowed_extensions:
    - ".html"
    - "/"

processor:
  chunk_size: 500
  chunk_overlap: 100
  remove_stopwords: true

ui:
  streaming: false
  theme: "dark"
`
	err := os.WriteFile(configPath, []byte(configData), 0644)
	require.NoError(t, err)

	// Test loading config
	config, err := LoadConfig(configPath)
	require.NoError(t, err)

	// Verify loaded values
	assert.Equal(t, "http://localhost:11434", config.LLM.BaseURL)
	assert.Equal(t, "gpt-4", config.LLM.Model)
	assert.Equal(t, 1000, config.LLM.MaxTokens)
	assert.Equal(t, 0.5, config.LLM.Temperature)
	assert.Equal(t, "postgres://localhost:5432/test", config.Database.URL)
	assert.Equal(t, 5, config.Scraper.MaxDepth)
	assert.Equal(t, 500, config.Processor.ChunkSize)
	assert.False(t, config.UI.Streaming)
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name          string
		config        Config
		expectedErrs  int
		errorMessages []string
	}{
		{
			name: "valid config",
			config: Config{
				LLM: struct {
					BaseURL     string  `yaml:"base_url"`
					Model       string  `yaml:"model"`
					MaxTokens   int     `yaml:"max_tokens"`
					Temperature float64 `yaml:"temperature"`
				}{
					BaseURL:     "http://localhost:11434",
					MaxTokens:   1000,
					Temperature: 0.7,
				},
				Database: struct {
					URL       string `yaml:"url"`
					TableName string `yaml:"table_name"`
					VectorDim int    `yaml:"vector_dim"`
					BatchSize int    `yaml:"batch_size"`
				}{
					VectorDim: 1536,
					BatchSize: 100,
				},
				Scraper: struct {
					MaxDepth          int      `yaml:"max_depth"`
					RateLimit         float64  `yaml:"rate_limit"`
					IgnorePatterns    []string `yaml:"ignore_patterns"`
					AllowedExtensions []string `yaml:"allowed_extensions"`
				}{
					MaxDepth:  3,
					RateLimit: 2.0,
				},
				Processor: struct {
					ChunkSize       int  `yaml:"chunk_size"`
					ChunkOverlap    int  `yaml:"chunk_overlap"`
					RemoveStopwords bool `yaml:"remove_stopwords"`
				}{
					ChunkSize:    1000,
					ChunkOverlap: 200,
				},
			},
			expectedErrs: 0,
		},
		{
			name: "invalid config",
			config: Config{
				LLM: struct {
					BaseURL     string  `yaml:"base_url"`
					Model       string  `yaml:"model"`
					MaxTokens   int     `yaml:"max_tokens"`
					Temperature float64 `yaml:"temperature"`
				}{
					BaseURL:     "invalid-url",
					MaxTokens:   5000, // Invalid
					Temperature: 3.0,  // Invalid
				},
				Database: struct {
					URL       string `yaml:"url"`
					TableName string `yaml:"table_name"`
					VectorDim int    `yaml:"vector_dim"`
					BatchSize int    `yaml:"batch_size"`
				}{
					URL:       "invalid-url", // Invalid
					VectorDim: -1,            // Invalid
				},
			},
			expectedErrs: 5, // Including missing API key
			errorMessages: []string{
				"base_url: Ollama base URL is required",
				"max_tokens: max_tokens must be between 1 and 4096",
				"temperature: temperature must be between 0 and 2",
				"database.url: invalid database URL",
				"vector_dim: vector_dim must be positive",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := tt.config.Validate()
			assert.Len(t, errors, tt.expectedErrs)

			if tt.errorMessages != nil {
				for i, msg := range tt.errorMessages {
					assert.Contains(t, errors[i].Error(), msg)
				}
			}
		})
	}
}

func TestEnvironmentOverrides(t *testing.T) {
	// Set environment variables
	os.Setenv("OLLAMA_BASE_URL", "http://env-ollama:11434")
	os.Setenv("DATABASE_URL", "postgres://env-db:5432/test")
	defer func() {
		os.Unsetenv("OLLAMA_BASE_URL")
		os.Unsetenv("DATABASE_URL")
	}()

	config := &Config{}
	mergeWithEnv(config)

	assert.Equal(t, "http://env-ollama:11434", config.LLM.BaseURL)
	assert.Equal(t, "postgres://env-db:5432/test", config.Database.URL)
}
