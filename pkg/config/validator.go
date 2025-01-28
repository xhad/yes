package config

import (
	"fmt"
	"net/url"
	"strings"
)

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

func (c *Config) Validate() []ValidationError {
	var errors []ValidationError

	// Validate LLM config
	if c.LLM.BaseURL == "" {
		errors = append(errors, ValidationError{
			Field:   "llm.base_url",
			Message: "Ollama base URL is required",
		})
	}

	if c.LLM.MaxTokens < 1 || c.LLM.MaxTokens > 4096 {
		errors = append(errors, ValidationError{
			Field:   "llm.max_tokens",
			Message: "max_tokens must be between 1 and 4096",
		})
	}

	if c.LLM.Temperature < 0 || c.LLM.Temperature > 2 {
		errors = append(errors, ValidationError{
			Field:   "llm.temperature",
			Message: "temperature must be between 0 and 2",
		})
	}

	// Validate Database config
	if c.Database.URL != "" {
		if _, err := url.Parse(c.Database.URL); err != nil {
			errors = append(errors, ValidationError{
				Field:   "database.url",
				Message: "invalid database URL",
			})
		}
	}

	if c.Database.VectorDim < 1 {
		errors = append(errors, ValidationError{
			Field:   "database.vector_dim",
			Message: "vector_dim must be positive",
		})
	}

	if c.Database.BatchSize < 1 {
		errors = append(errors, ValidationError{
			Field:   "database.batch_size",
			Message: "batch_size must be positive",
		})
	}

	// Validate Scraper config
	if c.Scraper.MaxDepth < 1 {
		errors = append(errors, ValidationError{
			Field:   "scraper.max_depth",
			Message: "max_depth must be positive",
		})
	}

	if c.Scraper.RateLimit <= 0 {
		errors = append(errors, ValidationError{
			Field:   "scraper.rate_limit",
			Message: "rate_limit must be positive",
		})
	}

	// Validate extensions format
	for _, ext := range c.Scraper.AllowedExtensions {
		if !strings.HasPrefix(ext, ".") && ext != "" && ext != "/" {
			errors = append(errors, ValidationError{
				Field:   "scraper.allowed_extensions",
				Message: fmt.Sprintf("invalid extension format: %s", ext),
			})
		}
	}

	// Validate Processor config
	if c.Processor.ChunkSize < 1 {
		errors = append(errors, ValidationError{
			Field:   "processor.chunk_size",
			Message: "chunk_size must be positive",
		})
	}

	if c.Processor.ChunkOverlap < 0 || c.Processor.ChunkOverlap >= c.Processor.ChunkSize {
		errors = append(errors, ValidationError{
			Field:   "processor.chunk_overlap",
			Message: "chunk_overlap must be non-negative and less than chunk_size",
		})
	}

	// Validate base URL format
	if _, err := url.Parse(c.LLM.BaseURL); err != nil {
		errors = append(errors, ValidationError{
			Field:   "llm.base_url",
			Message: "invalid Ollama base URL",
		})
	}

	return errors
}
