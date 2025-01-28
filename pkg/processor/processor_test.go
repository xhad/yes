package processor_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xhad/yes/internal/models"
	"github.com/xhad/yes/pkg/processor"
)

func TestProcessor_Process(t *testing.T) {

	config := processor.ProcessorConfig{
		ChunkSize:          50,
		ChunkOverlap:       10,
		MinChunkLength:     20,
		RemoveStopwords:    true,
		CustomStopwords:    []string{"document"},
		PreserveLineBreaks: false,
	}
	p := processor.NewWithConfig(config)

	documents := []models.Document{
		{Content: "This is a test document. It contains several sentences to demonstrate text processing."},
	}

	processedDocs, err := p.Process(documents)

	assert.NoError(t, err)
	assert.Len(t, processedDocs, 1)
	assert.Contains(t, processedDocs[0].Chunks[0], "test document") // Checking if the chunk contains meaningful text after processing
}

// func TestProcessor_CleanText(t *testing.T) {

// 	config := processor.ProcessorConfig{
// 		ChunkSize:          50,
// 		ChunkOverlap:       10,
// 		MinChunkLength:     20,
// 		RemoveStopwords:    true,
// 		CustomStopwords:    []string{"test"},
// 		PreserveLineBreaks: false,
// 	}
// 	p := processor.NewWithConfig(config)

// 	documents := []models.Document{
// 		{Content: "This is a test document. It contains several sentences to demonstrate text processing."},
// 		{Content: "This is a test. test."},                                  // Basic cleaning and stopword removal
// 		{Content: "A sentence with multiple    spaces  and  custom words."}, // Multiple spaces removal and custom stopword removal
// 	}

// 	processedDocs, err := p.Process(documents)

// 	assert.NoError(t, err)
// 	assert.Len(t, processedDocs, 1)
// 	assert.Contains(t, processedDocs[0].Chunks[0], "document") // Checking if the chunk contains meaningful text after processing

// }

// func TestProcessor_SplitIntoChunks(t *testing.T) {
// 	config := processor.ProcessorConfig{
// 		ChunkSize:      50,
// 		ChunkOverlap:   10,
// 		MinChunkLength: 20,
// 	}
// 	p := processor.NewWithConfig(config)

// 	tests := []struct {
// 		text string
// 		want []string
// 	}{
// 		{"This is a test document. It contains several sentences.", []string{"this is a test document", "document it contains several"}}, // Basic chunk splitting
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.text, func(t *testing.T) {
// 			got := processor.splitIntoChunks(tt.text)
// 			assert.Equal(t, tt.want, got)
// 		})
// 	}
// }

// func TestProcessor_SplitIntoSentences(t *testing.T) {
// 	processor := NewWithConfig(ProcessorConfig{}, &mockLLM{})

// 	tests := []struct {
// 		text string
// 		want []string
// 	}{
// 		{"This is a test. It contains several sentences.", []string{"this is a test", "it contains several sentences"}}, // Basic sentence splitting
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.text, func(t *testing.T) {
// 			got := processor.splitIntoSentences(tt.text)
// 			assert.Equal(t, tt.want, got)
// 		})
// 	}
// }

// func TestProcessor_RemoveStopwords(t *testing.T) {
// 	config := ProcessorConfig{
// 		CustomStopwords: []string{"custom"},
// 	}
// 	processor := NewWithConfig(config, &mockLLM{})

// 	tests := []struct {
// 		text string
// 		want string
// 	}{
// 		{"This is a test.", "test."},                  // Common English stopword removal
// 		{"A sentence with custom words.", "sentence"}, // Custom stopword removal
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.text, func(t *testing.T) {
// 			got := processor.removeStopwords(tt.text)
// 			assert.Equal(t, tt.want, got)
// 		})
// 	}
// }
