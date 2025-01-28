package processor

import (
	"strings"

	"github.com/xhad/yes/internal/models"
)

type ProcessorConfig struct {
	ChunkSize          int
	ChunkOverlap       int
	MinChunkLength     int
	RemoveStopwords    bool
	CustomStopwords    []string
	PreserveLineBreaks bool
}

type Processor struct {
	config ProcessorConfig
}

func NewWithConfig(config ProcessorConfig) Processor {
	if config.ChunkSize == 0 {
		config.ChunkSize = 1000
	}
	if config.ChunkOverlap == 0 {
		config.ChunkOverlap = 200
	}
	if config.MinChunkLength == 0 {
		config.MinChunkLength = 100
	}

	return Processor{
		config: config,
	}
}

func (p *Processor) Process(docs []models.Document) ([]models.ProcessedDocument, error) {
	var processed []models.ProcessedDocument

	for _, doc := range docs {
		// Clean the content
		cleanContent := p.cleanText(doc.Content)

		// Split into chunks
		chunks := p.splitIntoChunks(cleanContent)

		// Create processed document
		processedDoc := models.ProcessedDocument{
			Document: doc,
			Chunks:   chunks,
		}
		processed = append(processed, processedDoc)
	}

	return processed, nil
}

func (p *Processor) cleanText(text string) string {
	// Convert to lowercase if needed
	if !p.config.PreserveLineBreaks {
		text = strings.ToLower(text)
	}

	// Replace multiple spaces with single space
	text = strings.Join(strings.Fields(text), " ")

	// Remove stopwords if configured
	if p.config.RemoveStopwords {
		text = p.removeStopwords(text)
	}

	return strings.TrimSpace(text)
}

func (p *Processor) splitIntoChunks(text string) []string {
	var chunks []string

	// Split by sentences first
	sentences := p.splitIntoSentences(text)

	currentChunk := strings.Builder{}

	for _, sentence := range sentences {
		// If adding this sentence would exceed chunk size
		if currentChunk.Len()+len(sentence) > p.config.ChunkSize {
			// Save current chunk if it meets minimum length
			if currentChunk.Len() >= p.config.MinChunkLength {
				chunks = append(chunks, currentChunk.String())
			}

			// Start new chunk with overlap
			if p.config.ChunkOverlap > 0 && currentChunk.Len() > p.config.ChunkOverlap {
				// Get the last few characters for overlap
				text := currentChunk.String()
				lastPart := text[len(text)-p.config.ChunkOverlap:]
				currentChunk.Reset()
				currentChunk.WriteString(lastPart)
			} else {
				currentChunk.Reset()
			}
		}

		currentChunk.WriteString(sentence)
		currentChunk.WriteString(" ")
	}

	// Add the last chunk if it meets minimum length
	if currentChunk.Len() >= p.config.MinChunkLength {
		chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
	}

	return chunks
}

func (p *Processor) splitIntoSentences(text string) []string {
	// Basic sentence splitting - can be improved with NLP libraries
	sentenceEnders := []string{". ", "! ", "? ", ".\n", "!\n", "?\n"}
	var sentences []string

	current := strings.Builder{}

	for i := 0; i < len(text); i++ {
		current.WriteByte(text[i])

		// Check for sentence endings
		for _, ender := range sentenceEnders {
			if strings.HasSuffix(current.String(), ender) {
				sentences = append(sentences, strings.TrimSpace(current.String()))
				current.Reset()
				break
			}
		}
	}

	// Add any remaining text
	if current.Len() > 0 {
		sentences = append(sentences, strings.TrimSpace(current.String()))
	}

	return sentences
}

func (p *Processor) removeStopwords(text string) string {
	words := strings.Fields(text)
	var filtered []string

	stopwords := getStopwords()
	if len(p.config.CustomStopwords) > 0 {
		stopwords = append(stopwords, p.config.CustomStopwords...)
	}

	for _, word := range words {
		if !contains(stopwords, word) {
			filtered = append(filtered, word)
		}
	}

	return strings.Join(filtered, " ")
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Common English stopwords
func getStopwords() []string {
	return []string{
		"a", "an", "and", "are", "as", "at", "be", "by", "for",
		"from", "has", "he", "in", "is", "it", "its", "of", "on",
		"that", "the", "to", "was", "were", "will", "with",
	}
}
