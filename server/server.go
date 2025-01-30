package server

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync/atomic"

	"github.com/gorilla/websocket"
	cfgPkg "github.com/xhad/yes/pkg/config"
	"github.com/xhad/yes/pkg/llm"
	"github.com/xhad/yes/pkg/processor"
	"github.com/xhad/yes/pkg/scraper"
	"github.com/xhad/yes/pkg/store"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Be careful with this in production
	},
}

type Message struct {
	Type    string      `json:"type"`
	Content string      `json:"content"`
	Data    interface{} `json:"data,omitempty"`
}

type WSServer struct {
	config      Config
	chatEngine  *llm.ChatEngine
	processor   *processor.Processor
	vectorStore *store.VectorStore
}

type Config struct {
	BaseURL     string
	DBUrl       string
	DocsURL     string
	Model       string
	MaxDepth    int
	ChunkSize   int
	VectorDim   int
	TableName   string
	BatchSize   int
	RateLimit   float64
	MaxTokens   int
	Streaming   bool
	Temperature float64
}

func NewWSServer(config Config) (*WSServer, error) {
	chatEngine, err := llm.NewWithConfig(llm.ChatConfig{
		Model:       config.Model,
		MaxTokens:   config.MaxTokens,
		BaseURL:     config.BaseURL,
		Temperature: config.Temperature,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize chat engine: %v", err)
	}

	processor := processor.NewWithConfig(processor.ProcessorConfig{
		ChunkSize:    config.ChunkSize,
		ChunkOverlap: 200,
	})

	vectorStore, err := store.NewWithConfig(store.VectorStoreConfig{
		ConnString: config.DBUrl,
		TableName:  config.TableName,
		VectorDim:  config.VectorDim,
		BatchSize:  config.BatchSize,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize vector store: %v", err)
	}

	return &WSServer{
		config:      config,
		chatEngine:  chatEngine,
		processor:   &processor,
		vectorStore: vectorStore,
	}, nil
}

func (s *WSServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Error reading message: %v", err)
			break
		}

		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Error unmarshaling message: %v", err)
			continue
		}

		go s.handleMessage(conn, msg)
	}
}

func (s *WSServer) handleMessage(conn *websocket.Conn, msg Message) {
	query := msg.Content

	// Check for URL in the query
	urlRegex := regexp.MustCompile(`https?://[^\s]+`)
	if url := urlRegex.FindString(query); url != "" {
		s.sendMessage(conn, "status", fmt.Sprintf("Processing URL: %s", url))

		if !strings.HasPrefix(url, "http") {
			url = "https://" + url
		}

		// Process URL similar to the original code, but with WebSocket updates
		var processedCount int32
		scraper, err := scraper.NewWithConfig(scraper.ScraperConfig{
			BaseURL:   url,
			MaxDepth:  s.config.MaxDepth,
			RateLimit: s.config.RateLimit,
			OnProgress: func(url string) {
				atomic.AddInt32(&processedCount, 1)
				s.sendMessage(conn, "progress", fmt.Sprintf("Scraped %d pages", processedCount))
			},
		})

		if err != nil {
			s.sendMessage(conn, "error", fmt.Sprintf("Failed to initialize scraper: %v", err))
			return
		}

		// Continue with scraping, processing, and storing...
		// Send progress updates via WebSocket
		docs, err := scraper.Scrape(url)
		if err != nil {
			s.sendMessage(conn, "error", fmt.Sprintf("Failed to scrape URL: %v", err))
			return
		}

		s.sendMessage(conn, "status", fmt.Sprintf("Scraped %d documents", len(docs)))

		// Only continue with chat if query contains more than just the URL
		if strings.TrimSpace(query) == url {
			return
		}
	}

	// Handle regular chat query
	emb := llm.NewEmbedder()
	queryArray := []string{query}

	embeddings, err := emb.Embed.CreateEmbedding(context.Background(), queryArray)
	if err != nil {
		s.sendMessage(conn, "error", fmt.Sprintf("Failed to create query embeddings: %v", err))
		return
	}

	flatEmbeddings := emb.FlattenEmbeddings(embeddings)
	docs, err := s.vectorStore.Query(flatEmbeddings, 5)
	if err != nil {
		s.sendMessage(conn, "error", fmt.Sprintf("Error querying documents: %v", err))
		return
	}

	if s.config.Streaming {
		stream, err := s.chatEngine.ChatStream(query, docs)
		if err != nil {
			s.sendMessage(conn, "error", fmt.Sprintf("Error: %v", err))
			return
		}

		for chunk := range stream {
			if strings.HasPrefix(chunk, "Error:") {
				s.sendMessage(conn, "error", chunk)
				break
			}
			s.sendMessage(conn, "stream", chunk)
		}
	} else {
		response, err := s.chatEngine.Chat(query, docs)
		if err != nil {
			s.sendMessage(conn, "error", fmt.Sprintf("Error: %v", err))
			return
		}
		s.sendMessage(conn, "response", response.Choices[0].Content)
	}
}

func (s *WSServer) sendMessage(conn *websocket.Conn, msgType string, content string) {
	msg := Message{
		Type:    msgType,
		Content: content,
	}
	if err := conn.WriteJSON(msg); err != nil {
		log.Printf("Error sending message: %v", err)
	}
}

func main() {
	config := parseFlags()

	server, err := NewWSServer(config)
	if err != nil {
		log.Fatal(err)
	}
	defer server.vectorStore.Close()

	// Add WebSocket endpoint
	http.HandleFunc("/ws", server.handleWebSocket)

	// Add a simple health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting WebSocket server on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func parseFlags() Config {
	var config Config
	var configPath string

	flag.StringVar(&configPath, "config", "", "Path to config file")
	flag.StringVar(&config.BaseURL, "ollama-url", os.Getenv("OLLAMA_BASE_URL"), "Ollama server URL")
	flag.StringVar(&config.DBUrl, "db-url", os.Getenv("DATABASE_URL"), "PostgreSQL connection string")
	flag.StringVar(&config.DocsURL, "docs-url", "", "Documentation URL to scrape")
	flag.StringVar(&config.Model, "model", "gpt-3.5-turbo", "LLM model to use")
	flag.IntVar(&config.MaxDepth, "max-depth", 3, "Maximum depth for web scraping")
	flag.IntVar(&config.ChunkSize, "chunk-size", 1000, "Size of text chunks")
	flag.IntVar(&config.VectorDim, "vector-dim", 768, "Vector dimension")
	flag.StringVar(&config.TableName, "table", "documents", "PostgreSQL table name")
	flag.IntVar(&config.BatchSize, "batch-size", 100, "Batch size for database operations")
	flag.Float64Var(&config.RateLimit, "rate-limit", 2.0, "Rate limit for web scraping")
	flag.IntVar(&config.MaxTokens, "max-tokens", 2000, "Maximum tokens for LLM response")
	flag.BoolVar(&config.Streaming, "stream", true, "Enable streaming responses")
	flag.Float64Var(&config.Temperature, "temperature", 0.8, "Set the LLM Temperature")
	flag.Parse()

	// Load config file if specified
	if cfg, err := cfgPkg.LoadConfig(configPath); err == nil {
		// Override config with command line flags if provided
		if flag.Lookup("ollama-url").Value.String() != "" {
			cfg.LLM.BaseURL = config.BaseURL
		}

		// Update config struct
		config.BaseURL = cfg.LLM.BaseURL
		config.Model = cfg.LLM.Model
		config.MaxTokens = cfg.LLM.MaxTokens
		config.DBUrl = cfg.Database.URL
		config.TableName = cfg.Database.TableName
		config.VectorDim = cfg.Database.VectorDim
		config.BatchSize = cfg.Database.BatchSize
		config.MaxDepth = cfg.Scraper.MaxDepth
		config.RateLimit = cfg.Scraper.RateLimit
		config.ChunkSize = cfg.Processor.ChunkSize
		config.Streaming = cfg.UI.Streaming
		config.Temperature = cfg.LLM.Temperature
	}

	return config
}
