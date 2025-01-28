package llm_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xhad/yes/internal/models"
	"github.com/xhad/yes/pkg/llm"
)

func TestNewWithConfig(t *testing.T) {
	config := llm.ChatConfig{
		Model:           "testmodel",
		Temperature:     0.5,
		MaxTokens:       1000,
		SystemTemplate:  "Test system template",
		ContextTemplate: "Test context template",
		BaseURL:         "http://localhost:1234",
	}
	engine, err := llm.NewWithConfig(config)
	assert.NoError(t, err)
	assert.NotNil(t, engine)
}

func TestChat(t *testing.T) {
	config := llm.ChatConfig{
		Model:           "codestral",
		Temperature:     0.5,
		MaxTokens:       1000,
		SystemTemplate:  "Test system template",
		ContextTemplate: "Test context template",
		BaseURL:         "http://localhost:11434",
	}
	engine, err := llm.NewWithConfig(config)
	assert.NoError(t, err)
	query := "What would be a good company name for a company that makes colorful socks?"

	docs := make([]models.Document, 1)

	doc := models.Document{
		ID:      "doc123",
		URL:     "https://example.com/document123",
		Title:   "Test Document Title",
		Content: "This is the content of the test document.",
		Metadata: map[string]interface{}{
			"Author":    "John Doe",
			"Published": "2021-08-30",
		},
	}

	docs[0] = doc

	response, err := engine.Chat(query, docs)
	assert.NoError(t, err)
	assert.NotNil(t, response)
}
