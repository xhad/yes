package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/fatih/color"
	"github.com/schollz/progressbar/v3"
	"github.com/xhad/yes/internal/models"
	cfgPkg "github.com/xhad/yes/pkg/config"
	"github.com/xhad/yes/pkg/llm"
	"github.com/xhad/yes/pkg/processor"
	"github.com/xhad/yes/pkg/scraper"
	"github.com/xhad/yes/pkg/store"
)

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

func main() {
	config := parseFlags()

	if err := run(config); err != nil {
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

func getProgressBar(total int, description string) *progressbar.ProgressBar {
	return progressbar.NewOptions(total,
		progressbar.OptionSetDescription(color.BlueString(description)),
		progressbar.OptionSetItsString("items"),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "█",
			SaucerHead:    "█",
			SaucerPadding: "░",
			BarStart:      "[",
			BarEnd:        "]",
		}),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetWidth(40),
		progressbar.OptionShowElapsedTimeOnFinish(),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetRenderBlankState(true),
	)
}

func getSpinner(description string) *progressbar.ProgressBar {
	return progressbar.NewOptions(-1,
		progressbar.OptionSetDescription(color.CyanString(description)),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionSetWidth(20),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetRenderBlankState(true),
	)
}

func run(config Config) error {
	// Initialize components
	var processedCount int32
	scraper, err := scraper.NewWithConfig(scraper.ScraperConfig{
		BaseURL:   config.DocsURL,
		MaxDepth:  config.MaxDepth,
		RateLimit: config.RateLimit,
		OnProgress: func(url string) {
			atomic.AddInt32(&processedCount, 1)
		},
	})
	if err != nil {
		return fmt.Errorf("failed to initialize scraper: %v", err)
	}
	chatEngine, err := llm.NewWithConfig(llm.ChatConfig{
		Model:       config.Model,
		MaxTokens:   config.MaxTokens,
		BaseURL:     config.BaseURL,
		Temperature: config.Temperature,
	})

	if err != nil {
		return fmt.Errorf("failed to initialize chat engine: %v", err)
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
		return fmt.Errorf("failed to initialize vector store: %v", err)
	}

	defer vectorStore.Close()

	// If docs URL is provided, scrape and store documents
	if config.DocsURL != "" {
		color.Blue("\nStarting documentation pipeline for %s\n", config.DocsURL)

		// Create progress bar for scraping
		scrapingBar := getProgressBar(-1, "📄 Scraping documentation...")

		// Start progress updater with ETA calculation
		startTime := time.Now()
		lastCount := int32(0)

		go func() {
			for {
				count := atomic.LoadInt32(&processedCount)
				scrapingBar.Set(int(count))

				// Calculate and show rate
				if count > lastCount {
					elapsed := time.Since(startTime).Seconds()
					rate := float64(count) / elapsed
					scrapingBar.Describe(color.BlueString(
						"📄 Scraping documentation... (%.1f pages/sec)", rate))
				}
				lastCount = count
				time.Sleep(100 * time.Millisecond)
			}
		}()

		docs, err := scraper.Scrape(config.DocsURL)
		if err != nil {
			return fmt.Errorf("failed to scrape documents: %v", err)
		}
		scrapingBar.Finish()
		color.Green("\n✓ Scraped %d documents\n", len(docs))

		// Processing progress bar
		processingBar := getProgressBar(len(docs), "🔄 Processing documents...")
		processed := make([]models.ProcessedDocument, 0, len(docs))

		startTime = time.Now()
		for i, doc := range docs {
			processedDocs, err := processor.Process([]models.Document{doc})
			if err != nil {
				return fmt.Errorf("failed to process document %s: %v", doc.URL, err)
			}
			processed = append(processed, processedDocs...)
			processingBar.Add(1)

			// Update rate
			elapsed := time.Since(startTime).Seconds()
			rate := float64(i+1) / elapsed
			processingBar.Describe(color.BlueString(
				"🔄 Processing documents... (%.1f docs/sec)", rate))
		}
		color.Green("\n✓ Processed into %d chunks\n", len(processed))

		// Storage progress bar
		storageBar := getProgressBar(len(processed), "💾 Storing in vector database...")

		// Store in batches with rate display
		startTime = time.Now()
		batchSize := config.BatchSize
		for i := 0; i < len(processed); i += batchSize {
			end := i + batchSize
			if end > len(processed) {
				end = len(processed)
			}
			batch := processed[i:end]

			if err := vectorStore.Store(batch); err != nil {
				return fmt.Errorf("failed to store batch: %v", err)
			}
			storageBar.Add(len(batch))

			// Update rate
			elapsed := time.Since(startTime).Seconds()
			rate := float64(i+len(batch)) / elapsed
			storageBar.Describe(color.BlueString(
				"💾 Storing in vector database... (%.1f chunks/sec)", rate))
		}
		color.Green("\n✓ Storage complete\n")
	}

	// Interactive chat loop with colored output
	color.Cyan("\nChat with your knowledge base (type 'exit' to quit)")

	scanner := bufio.NewScanner(os.Stdin)
	userPrompt := color.New(color.FgGreen).PrintfFunc()
	assistantPrompt := color.New(color.FgCyan).PrintfFunc()

	for {
		userPrompt("\nYou: ")
		if !scanner.Scan() {
			break
		}

		query := scanner.Text()
		if strings.ToLower(query) == "exit" {
			break
		}

		emb := llm.NewEmbedder()
		queryArray := make([]string, 1)
		queryArray[0] = query

		embeddings, err := emb.Embed.CreateEmbedding(context.Background(), queryArray)

		if err != nil {
			fmt.Errorf("failed to create query embeddgins %s", err)
		}

		flatEmbeddings := emb.FlattenEmbeddings(embeddings)

		// Show spinner while querying
		querySpinner := getSpinner("🔍 Searching documentation...")

		docs, err := vectorStore.Query(flatEmbeddings, 5)
		fmt.Print("\r") // Clear spinner line

		querySpinner.Finish()

		if err != nil {
			color.Red("Error querying documents: %v\n", err)
			continue
		}

		if config.Streaming {

			stream, err := chatEngine.ChatStream(query, docs)

			responseSpinner := getSpinner("🤖 Generating response...")

			fmt.Print("\n")
			assistantPrompt("Assistant: ")

			if err != nil {
				color.Red("Error: %v\n", err)
				responseSpinner.Finish()
				continue
			}

			for chunk := range stream {
				assistantPrompt("Assistant: %s", chunk)
			}

			responseSpinner.Finish()
			fmt.Print("\n")
		} else {
			responseSpinner := getSpinner("🤖 Generating response...")
			response, err := chatEngine.Chat(query, docs)
			responseSpinner.Finish()
			fmt.Print("\r")

			if err != nil {
				color.Red("Error: %v\n", err)
				continue
			}
			assistantPrompt("Assistant: %s\n", response)
		}
	}

	return nil
}
