package models

type Document struct {
	ID       string
	URL      string
	Title    string
	Content  string
	Metadata map[string]interface{}
}

type ProcessedDocument struct {
	Document
	Chunks    []string
	Embedding [][]float32
}
