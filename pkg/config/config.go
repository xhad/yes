package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	LLM struct {
		BaseURL     string  `yaml:"base_url"`
		Model       string  `yaml:"model"`
		MaxTokens   int     `yaml:"max_tokens"`
		Temperature float64 `yaml:"temperature"`
	} `yaml:"llm"`

	Database struct {
		URL       string `yaml:"url"`
		TableName string `yaml:"table_name"`
		VectorDim int    `yaml:"vector_dim"`
		BatchSize int    `yaml:"batch_size"`
	} `yaml:"database"`

	Scraper struct {
		MaxDepth          int      `yaml:"max_depth"`
		RateLimit         float64  `yaml:"rate_limit"`
		IgnorePatterns    []string `yaml:"ignore_patterns"`
		AllowedExtensions []string `yaml:"allowed_extensions"`
	} `yaml:"scraper"`

	Processor struct {
		ChunkSize       int  `yaml:"chunk_size"`
		ChunkOverlap    int  `yaml:"chunk_overlap"`
		RemoveStopwords bool `yaml:"remove_stopwords"`
	} `yaml:"processor"`

	UI struct {
		Streaming bool   `yaml:"streaming"`
		Theme     string `yaml:"theme"`
	} `yaml:"ui"`
}

func LoadConfig(path string) (*Config, error) {
	// If no path provided, try default locations
	if path == "" {
		locations := []string{
			"config.yaml",
			"config.yml",
			filepath.Join(os.Getenv("HOME"), ".config/yestion/config.yaml"),
			"/etc/yestion/config.yaml",
		}

		for _, loc := range locations {
			if _, err := os.Stat(loc); err == nil {
				path = loc
				break
			}
		}
	}

	if path == "" {
		return getDefaultConfig()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}

	// Merge with environment variables
	mergeWithEnv(&config)

	// Apply defaults for unset values
	applyDefaults(&config)

	return &config, nil
}

func getDefaultConfig() (*Config, error) {
	config := &Config{}
	applyDefaults(config)
	mergeWithEnv(config)
	return config, nil
}

func applyDefaults(config *Config) {
	if config.LLM.Model == "" {
		config.LLM.Model = "mistral"
	}
	if config.LLM.MaxTokens == 0 {
		config.LLM.MaxTokens = 2000
	}
	if config.LLM.Temperature == 0 {
		config.LLM.Temperature = 0.7
	}
	if config.LLM.BaseURL == "" {
		config.LLM.BaseURL = "http://localhost:11434"
	}

	if config.Database.TableName == "" {
		config.Database.TableName = "documents"
	}
	if config.Database.VectorDim == 0 {
		config.Database.VectorDim = 1536
	}
	if config.Database.BatchSize == 0 {
		config.Database.BatchSize = 100
	}

	if config.Scraper.MaxDepth == 0 {
		config.Scraper.MaxDepth = 3
	}
	if config.Scraper.RateLimit == 0 {
		config.Scraper.RateLimit = 2.0
	}
	if len(config.Scraper.AllowedExtensions) == 0 {
		config.Scraper.AllowedExtensions = []string{".html", ".htm", "/", ""}
	}

	if config.Processor.ChunkSize == 0 {
		config.Processor.ChunkSize = 1000
	}
	if config.Processor.ChunkOverlap == 0 {
		config.Processor.ChunkOverlap = 200
	}

	if config.UI.Theme == "" {
		config.UI.Theme = "default"
	}
}

func mergeWithEnv(config *Config) {
	if baseURL := os.Getenv("OLLAMA_BASE_URL"); baseURL != "" {
		config.LLM.BaseURL = baseURL
	}
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		config.Database.URL = dbURL
	}
}
