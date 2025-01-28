package llm_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/xhad/yes/internal/models"
	"github.com/xhad/yes/pkg/llm"
)

var config = llm.EmbedderConfig{
	Model:     "nomic-embed-text:latest",
	MaxTokens: 1000,
	BaseURL:   "http://localhost:11434",
}

func TestNewEmbedderWithConfig(t *testing.T) {
	emb := llm.NewEmbedderWithConfig(config)
	assert.NotNil(t, emb)
}

func TestCreateEmbedding(t *testing.T) {
	// This test requires a running Ollama server with the correct model.
	// Mocking the LLM is complex due to its interface, so this test assumes a real Ollama server is available.
	emb := llm.NewEmbedderWithConfig(config)

	documents := []models.ProcessedDocument{
		{
			Chunks: []string{"This is the first chunk.", "And this is the second chunk."},
		},
		{
			Chunks: []string{"Another document's first chunk.", "Its second chunk."},
		},
		// Add more documents as needed...
	}

	var allStrings []string
	for _, doc := range documents {
		allStrings = append(allStrings, doc.Chunks...)
	}

	ctx := context.Background()

	embeddings, err := emb.Embed.CreateEmbedding(ctx, allStrings)

	if err != nil {
		t.Log("fmterr", err)
	}

	for i := range embeddings {
		assert.Equal(t, len(embeddings[i]), 768)
	}
}
