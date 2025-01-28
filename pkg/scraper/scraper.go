package scraper

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/xhad/yes/internal/models"
	"golang.org/x/time/rate"
)

type ScraperConfig struct {
	BaseURL           string
	MaxDepth          int
	RateLimit         float64 // requests per second
	IgnorePatterns    []string
	AllowedExtensions []string
	Timeout           time.Duration
	OnProgress        func(url string) // Add progress callback
}

type Scraper struct {
	config   ScraperConfig
	client   *http.Client
	visited  map[string]bool
	limiter  *rate.Limiter
	baseHost string
}

func NewWithConfig(config ScraperConfig) (*Scraper, error) {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxDepth == 0 {
		config.MaxDepth = 3
	}
	if config.RateLimit == 0 {
		config.RateLimit = 2 // 2 requests per second by default
	}
	if len(config.AllowedExtensions) == 0 {
		config.AllowedExtensions = []string{".html", ".htm", "/", ""}
	}

	parsedURL, err := url.Parse(config.BaseURL)
	if err != nil {
		return nil, err
	}

	return &Scraper{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		visited:  make(map[string]bool),
		limiter:  rate.NewLimiter(rate.Limit(config.RateLimit), 1),
		baseHost: parsedURL.Host,
	}, nil
}

func New(baseURL string) *Scraper {
	s, _ := NewWithConfig(ScraperConfig{
		BaseURL: baseURL,
	})
	return s
}

func (s *Scraper) shouldProcessURL(urlStr string) bool {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	// Check if URL is from the same host
	if parsedURL.Host != s.baseHost {
		return false
	}

	// Check extensions
	ext := strings.ToLower(parsedURL.Path)
	validExt := false
	for _, allowedExt := range s.config.AllowedExtensions {
		if strings.HasSuffix(ext, allowedExt) {
			validExt = true
			break
		}
	}
	if !validExt {
		return false
	}

	// Check ignore patterns
	for _, pattern := range s.config.IgnorePatterns {
		if strings.Contains(urlStr, pattern) {
			return false
		}
	}

	return true
}

func (s *Scraper) cleanContent(content string) string {
	// Remove extra whitespace
	content = strings.Join(strings.Fields(content), " ")

	// Remove common noise
	noisePatterns := []string{
		"Cookie Policy",
		"Accept Cookies",
		"Privacy Policy",
		"Terms of Service",
	}

	for _, pattern := range noisePatterns {
		content = strings.ReplaceAll(content, pattern, "")
	}

	return strings.TrimSpace(content)
}

func (s *Scraper) extractMainContent(doc *goquery.Document) string {
	// Try to find main content area
	selectors := []string{
		"main",
		"article",
		".content",
		"#content",
		".documentation",
		"#documentation",
	}

	var content string
	for _, selector := range selectors {
		if selected := doc.Find(selector); selected.Length() > 0 {
			content = selected.Text()
			break
		}
	}

	// Fallback to body if no main content found
	if content == "" {
		content = doc.Find("body").Text()
	}

	return s.cleanContent(content)
}

func (s *Scraper) Scrape(url string) ([]models.Document, error) {
	var documents []models.Document
	err := s.scrapeRecursive(url, 0, &documents)
	return documents, err
}
func (s *Scraper) scrapeRecursive(urlStr string, depth int, documents *[]models.Document) error {
	if depth > s.config.MaxDepth || s.visited[urlStr] {
		return nil
	}

	if !s.shouldProcessURL(urlStr) {
		return nil
	}

	s.visited[urlStr] = true
	if s.config.OnProgress != nil {
		s.config.OnProgress(urlStr)
	}

	// Apply rate limiting
	err := s.limiter.Wait(context.Background())
	if err != nil {
		return err
	}

	resp, err := s.client.Get(urlStr)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received status code %d for URL: %s", resp.StatusCode, urlStr)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return err
	}

	// Extract content
	content := s.extractMainContent(doc)
	title := doc.Find("title").Text()

	// Create document
	document := models.Document{
		URL:     urlStr,
		Title:   title,
		Content: content,
		Metadata: map[string]interface{}{
			"depth":        depth,
			"time":         time.Now(),
			"contentType":  resp.Header.Get("Content-Type"),
			"lastModified": resp.Header.Get("Last-Modified"),
		},
	}
	*documents = append(*documents, document)

	// Find and follow links
	doc.Find("a[href]").Each(func(_ int, selection *goquery.Selection) {
		href, exists := selection.Attr("href")
		if !exists {
			return
		}

		absoluteURL, err := url.Parse(href)
		if err != nil {
			log.Printf("Error parsing URL: %v", err)
			return
		}

		// Make sure the URL is absolute
		if !absoluteURL.IsAbs() {
			base, err := url.Parse(urlStr)
			if err != nil {
				log.Printf("Error parsing base URL: %v", err)
				return
			}
			absoluteURL = base.ResolveReference(absoluteURL)
		}

		// Scrape the URL recursively by keeping a reference to `s`
		scraper := s // <--- Keep a reference to `s` here!
		if err := scraper.scrapeRecursive(absoluteURL.String(), depth+1, documents); err != nil {
			log.Printf("Error scraping URL: %v", err)
		}
	})

	return nil
}
