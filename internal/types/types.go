package types

import (
	"context"
	"time"

	"github.com/xhad/yes/internal/models"
)

// Core interfaces
type Document interface {
	GetID() string
	GetURL() string
	GetContent() string
	GetMetadata() map[string]interface{}
}

type VectorStore interface {
	Store(docs []models.ProcessedDocument) error
	Query(embedding []float32, limit int) ([]Document, error)
	Close()
}

type Embedder interface {
	CreateEmbedding(ctx context.Context, texts []string) ([][]float32, error)
	FlattenEmbeddings(embeddings [][]float32) []float32
}

type Processor interface {
	Process(docs []models.Document) ([]models.ProcessedDocument, error)
}

type ProcessorConfig struct {
	ChunkSize          int
	ChunkOverlap       int
	MinChunkLength     int
	RemoveStopwords    bool
	CustomStopwords    []string
	PreserveLineBreaks bool
}

type Config struct {
	LLM      LLMConfig
	Database DatabaseConfig
	Scraper  ScraperConfig
	UI       UIConfig
}

type LLMConfig struct {
	BaseURL         string
	Model           string
	MaxTokens       int
	Temperature     float64
	SystemTemplate  string
	ContextTemplate string
}

type DatabaseConfig struct {
	URL            string
	TableName      string
	VectorDim      int
	BatchSize      int
	SearchLimit    int
	SearchDistance float32
}

type ScraperConfig struct {
	MaxDepth          int
	RateLimit         float64
	IgnorePatterns    []string
	AllowedExtensions []string
	Timeout           time.Duration
	OnProgress        func(url string)
}

type UIConfig struct {
	Streaming bool
}
