package store

import (
	"context"
	"fmt"

	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
	"github.com/xhad/yes/internal/models"
	"github.com/xhad/yes/pkg/llm"
)

type VectorStoreConfig struct {
	ConnString     string
	TableName      string
	VectorDim      int
	BatchSize      int
	SearchLimit    int
	SearchDistance float32
}

type VectorStore struct {
	config VectorStoreConfig
	pool   *pgxpool.Pool
}

func NewWithConfig(config VectorStoreConfig) (*VectorStore, error) {
	if config.TableName == "" {
		config.TableName = "documents"
	}
	if config.VectorDim == 0 {
		config.VectorDim = 1536 // Default for OpenAI embeddings
	}
	if config.BatchSize == 0 {
		config.BatchSize = 100
	}
	if config.SearchLimit == 0 {
		config.SearchLimit = 5
	}
	if config.SearchDistance == 0 {
		config.SearchDistance = 0.8
	}

	pool, err := pgxpool.New(context.Background(), config.ConnString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	vs := &VectorStore{
		config: config,
		pool:   pool,
	}

	if err := vs.initialize(); err != nil {
		pool.Close()
		return nil, err
	}

	return vs, nil
}

func (vs *VectorStore) initialize() error {
	ctx := context.Background()

	// Enable pgvector extension
	_, err := vs.pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector")
	if err != nil {
		return fmt.Errorf("failed to create vector extension: %v", err)
	}

	// Create documents table if it doesn't exist
	createTable := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id TEXT PRIMARY KEY,
			url TEXT NOT NULL,
			title TEXT,
			content TEXT,
			chunk_index INTEGER,
			embedding vector(%d),
			metadata JSONB
		)`, vs.config.TableName, vs.config.VectorDim)

	_, err = vs.pool.Exec(ctx, createTable)
	if err != nil {
		return fmt.Errorf("failed to create table: %v", err)
	}

	// Create vector index
	createIndex := fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS %s_embedding_idx 
		ON %s 
		USING ivfflat (embedding vector_cosine_ops)
		WITH (lists = 100)`,
		vs.config.TableName, vs.config.TableName)

	_, err = vs.pool.Exec(ctx, createIndex)
	if err != nil {
		return fmt.Errorf("failed to create index: %v", err)
	}

	return nil
}

func (vs *VectorStore) Store(docs []models.ProcessedDocument) error {
	ctx := context.Background()

	// Begin transaction
	tx, err := vs.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	// Prepare the insert statement
	stmt := fmt.Sprintf(`
		INSERT INTO %s (id, url, title, content, chunk_index, embedding, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			content = EXCLUDED.content,
			embedding = EXCLUDED.embedding,
			metadata = EXCLUDED.metadata`,
		vs.config.TableName)

	emb := llm.NewEmbedder()

	// Insert documents in batches
	for _, doc := range docs {

		cleanTitle := sanitizeUTF8(doc.Title)

		for i, chunk := range doc.Chunks {
			cleanChunk := sanitizeUTF8(chunk)
			id := fmt.Sprintf("%s_%d", doc.ID, i)

			reChunk := make([]string, 1)
			reChunk[0] = cleanChunk // Replace 'chunk1' with your first chunk data

			embedding, err := emb.Embed.CreateEmbedding(ctx, reChunk)

			if err != nil {
				return fmt.Errorf("failed to create embeddings: %v", err)
			}

			var vectorSlice []float32

			// Flatten the embeddings into a single slice using a temporary buffer
			var tempBuffer []float32
			for _, emb := range embedding {
				tempBuffer = append(tempBuffer, emb...)
			}
			vectorSlice = append(vectorSlice, tempBuffer...)

			vectorEmbeddings := pgvector.NewVector(vectorSlice)

			_, err = tx.Exec(ctx, stmt,
				id,
				doc.URL,
				cleanTitle,
				cleanChunk,
				i,
				vectorEmbeddings,
				doc.Metadata,
			)
			if err != nil {
				return fmt.Errorf("failed to insert document: %v", err)
			}
		}
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	return nil
}

func (vs *VectorStore) Query(queryEmbedding []float32, limit int) ([]models.Document, error) {
	ctx := context.Background()

	if limit == 0 {
		limit = vs.config.SearchLimit
	}

	// Query similar documents
	query := fmt.Sprintf(`
		SELECT id, url, title, content, metadata
		FROM %s
		ORDER BY embedding <=> $1
		LIMIT $2`,
		vs.config.TableName)

	embedding := pgvector.NewVector(queryEmbedding)
	rows, err := vs.pool.Query(ctx, query, embedding, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query documents: %v", err)
	}
	defer rows.Close()

	var docs []models.Document
	for rows.Next() {
		var doc models.Document
		err := rows.Scan(
			&doc.ID,
			&doc.URL,
			&doc.Title,
			&doc.Content,
			&doc.Metadata,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %v", err)
		}
		docs = append(docs, doc)
	}

	return docs, nil
}

func (vs *VectorStore) Close() {
	if vs.pool != nil {
		vs.pool.Close()
	}
}

// Add this helper function
func sanitizeUTF8(s string) string {
	if !utf8.ValidString(s) {
		v := make([]rune, 0, len(s))
		for i, r := range s {
			if r == utf8.RuneError {
				_, size := utf8.DecodeRuneInString(s[i:])
				if size == 1 {
					continue
				}
			}
			v = append(v, r)
		}
		return string(v)
	}
	return s
}
