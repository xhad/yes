package store_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xhad/yes/internal/models"
	"github.com/xhad/yes/pkg/llm"
	"github.com/xhad/yes/pkg/store"
)

func getTestConfig() store.VectorStoreConfig {
	return store.VectorStoreConfig{
		ConnString: "postgresql://testuser:testpass@localhost:5432/yes",
		TableName:  "test_documents",
		VectorDim:  768,
	}
}

func TestVectorStore(t *testing.T) {

	config := getTestConfig()
	s, err := store.NewWithConfig(config)
	require.NoError(t, err)
	defer s.Close()

	// Test document
	docs := []models.ProcessedDocument{
		{
			Document: models.Document{
				ID:    "test1",
				URL:   "https://example.com/1",
				Title: "Test Document 1",
				Metadata: map[string]interface{}{
					"source": "test",
				},
			},
			Chunks: []string{
				"This is chunk 1",
				"This is chunk 2",
				"This is chunk 3",
			},
		},
	}

	// // Test storing
	err = s.Store(docs)
	require.NoError(t, err)

	emb := llm.NewEmbedder()

	query := make([]string, 1)
	query[0] = "chunk 1"

	embedding, err := emb.Embed.CreateEmbedding(context.Background(), query)

	if err != nil {
		fmt.Errorf("error creating the queyr embeddings %w", err)
	}

	tempBuffer := make([]float32, 0)
	for _, emb := range embedding {
		tempBuffer = append(tempBuffer, emb...)
	}
	var vectorSlice []float32 // falttend embeddings

	vectorSlice = append(vectorSlice, tempBuffer...)

	results, err := s.Query(vectorSlice, 1)

	if err != nil {
		fmt.Errorf("error in store Query %w", err)
	}

	require.NoError(t, err)
	require.Len(t, results, 1)

	// Verify results
	assert.Equal(t, docs[0].URL, results[0].URL)
	assert.Equal(t, docs[0].Title, results[0].Title)
}
