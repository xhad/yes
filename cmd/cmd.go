package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
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

	// Interactive chat loop with colored output
	color.Cyan("\nChat with Loreum Sensors and Agents (type 'exit' to quit)")

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

		// Check if input contains a URL
		urlRegex := regexp.MustCompile(`https?://[^\s]+`)
		if url := urlRegex.FindString(query); url != "" {
			color.Blue("\nDetected URL: %s", url)
			if !strings.HasPrefix(url, "http") {
				url = "https://" + url
			}

			// Initialize scraper for this URL
			var scrapeCount int32
			s, err := scraper.NewWithConfig(scraper.ScraperConfig{
				BaseURL:   url,
				MaxDepth:  config.MaxDepth,
				RateLimit: config.RateLimit,
				OnProgress: func(url string) {
					atomic.AddInt32(&scrapeCount, 1)
				},
			})
			if err != nil {
				color.Red("Failed to initialize scraper: %v\n", err)
				continue
			}

			// Create progress bar for scraping
			scrapingBar := getProgressBar(-1, " Scraping documentation...")
			startTime := time.Now()
			lastCount := int32(0)

			// Start progress updater
			go func() {
				for {
					count := atomic.LoadInt32(&scrapeCount)
					scrapingBar.Set(int(count))

					if count > lastCount {
						elapsed := time.Since(startTime).Seconds()
						rate := float64(count) / elapsed
						scrapingBar.Describe(color.BlueString(
							"Scraping documentation (%.1f pages/sec)", rate))
					}
					lastCount = count
					time.Sleep(100 * time.Millisecond)
				}
			}()

			// Scrape the URL
			docs, err := s.Scrape(url)
			if err != nil {
				scrapingBar.Finish()
				color.Red("Failed to scrape URL: %v\n", err)
				continue
			}
			scrapingBar.Finish()
			color.Green("✓ Scraped %d documents\n", len(docs))

			// Process documents with progress bar
			processed := make([]models.ProcessedDocument, 0)
			var processedCount int32
			processingBar := getProgressBar(-1, " Processing documents")

			// Initialize variables for progress tracking
			startTime1 := time.Now()
			lastCount1 := int32(0)

			// Start the progress updater goroutine
			go func() {
				for {
					count := atomic.LoadInt32(&processedCount)
					processingBar.Set(int(count))
					if count > lastCount1 {
						elapsed := time.Since(startTime1).Seconds()
						rate := float64(count) / elapsed
						processingBar.Describe(color.BlueString(
							"Processing documents (%.1f docs/sec)", rate))
					}
					lastCount1 = count
					time.Sleep(100 * time.Millisecond)
				}
			}()

			for _, doc := range docs {
				processedDocs, err := processor.Process([]models.Document{doc})
				if err != nil {
					color.Red("Failed to process document %s: %v\n", doc.URL, err)
					continue
				}

				// Update the count when processing is successful
				atomic.AddInt32(&processedCount, 1)
				processed = append(processed, processedDocs...)
			}
			processingBar.Finish()
			color.Green("✓ Processed into %d chunks\n", len(processed))

			// Store in vector database
			storageBar := getProgressBar(-1, " Storing in vector database")
			startTime = time.Now()
			batchSize := config.BatchSize

			for i := 0; i < len(processed); i += batchSize {
				end := i + batchSize
				if end > len(processed) {
					end = len(processed)
				}
				batch := processed[i:end]

				if err := vectorStore.Store(batch); err != nil {
					color.Red("Failed to store batch: %v\n", err)
					continue
				}
				storageBar.Add(len(batch))

				elapsed := time.Since(startTime).Seconds()
				rate := float64(i+len(batch)) / elapsed
				storageBar.Describe(color.BlueString(
					"Storing in vector database (%.1f chunks/sec)", rate))
			}
			storageBar.Finish()
			color.Green("✓ URL processed and stored\n")

			if strings.TrimSpace(query) == url {
				continue
			}
		}

		// Regular chat flow continues here...
		emb := llm.NewEmbedder()
		queryArray := make([]string, 1)
		queryArray[0] = query

		embeddings, err := emb.Embed.CreateEmbedding(context.Background(), queryArray)
		if err != nil {
			color.Red("Failed to create query embeddings: %v\n", err)
			continue
		}

		flatEmbeddings := emb.FlattenEmbeddings(embeddings)

		// Show spinner while querying
		querySpinner := getSpinner(" Searching documentation...")
		docs, err := vectorStore.Query(flatEmbeddings, 5)
		querySpinner.Finish()

		if err != nil {
			color.Red("Error querying documents: %v\n", err)
			continue
		}

		if config.Streaming {
			stream, err := chatEngine.ChatStream(query, docs)
			if err != nil {
				color.Red("Error: %v\n", err)
				continue
			}

			fmt.Print("\n")
			assistantPrompt("Assistant: ")

			// Create and start the spinner
			responseSpinner := getSpinner(" Thinking...")
			firstChunk := true

			// Process the stream
			for chunk := range stream {
				if strings.HasPrefix(chunk, "Error:") {
					responseSpinner.Finish()
					color.Red("\n%s", chunk)
					break
				}

				// Clear spinner on first chunk
				if firstChunk {
					responseSpinner.Finish()
					firstChunk = false
					fmt.Println("\n")

				}

				fmt.Print(chunk)
			}

			// Ensure spinner is finished in case of early exit
			if firstChunk {
				responseSpinner.Finish()
			}
			fmt.Print("\n")
		} else {
			responseSpinner := getSpinner(" Generating response...")
			response, err := chatEngine.Chat(query, docs)
			responseSpinner.Finish()

			if err != nil {
				color.Red("Error: %v\n", err)
				continue
			}
			assistantPrompt("\nAssistant: %s\n", response)
		}
	}

	return nil
}
