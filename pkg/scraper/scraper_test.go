package scraper

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScraper(t *testing.T) {
	s := New("https://example.com")

	docs, err := s.Scrape("https://example.com")
	assert.NoError(t, err)
	assert.NotEmpty(t, docs)

	// Test document properties
	for _, doc := range docs {
		assert.NotEmpty(t, doc.URL)
		assert.NotEmpty(t, doc.Content)
		assert.NotNil(t, doc.Metadata)
	}
}

func TestScraperConfig(t *testing.T) {
	config := ScraperConfig{
		BaseURL:        "https://example.com",
		MaxDepth:       5,
		RateLimit:      1.0,
		IgnorePatterns: []string{"/ignore/", "private"},
		Timeout:        10 * time.Second,
	}

	s, err := NewWithConfig(config)
	require.NoError(t, err)
	assert.Equal(t, config.BaseURL, s.config.BaseURL)
	assert.Equal(t, config.MaxDepth, s.config.MaxDepth)
}

func TestShouldProcessURL(t *testing.T) {
	config := ScraperConfig{
		BaseURL:           "https://example.com",
		IgnorePatterns:    []string{"/ignore/", "private"},
		AllowedExtensions: []string{".html", "/"},
	}

	s, err := NewWithConfig(config)
	require.NoError(t, err)

	tests := []struct {
		url      string
		expected bool
	}{
		{"https://example.com/docs/", true},
		{"https://example.com/page.html", true},
		{"https://example.com/ignore/page.html", false},
		{"https://other-domain.com/page.html", false},
		{"https://example.com/file.pdf", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			result := s.shouldProcessURL(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestScrapeWithMockServer(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
			<html>
				<head><title>Test Page</title></head>
				<body>
					<main>
						<h1>Test Content</h1>
						<p>This is a test paragraph.</p>
						<a href="/page2.html">Link</a>
					</main>
				</body>
			</html>
		`))
	}))
	defer server.Close()

	config := ScraperConfig{
		BaseURL:   server.URL,
		MaxDepth:  1,
		RateLimit: 10,
	}

	s, err := NewWithConfig(config)
	require.NoError(t, err)

	docs, err := s.Scrape(server.URL)
	require.NoError(t, err)
	require.NotEmpty(t, docs)

	doc := docs[0]
	assert.Equal(t, server.URL, doc.URL)
	assert.Equal(t, "Test Page", doc.Title)
	assert.Contains(t, doc.Content, "Test Content")
	assert.Contains(t, doc.Content, "This is a test paragraph")
}
