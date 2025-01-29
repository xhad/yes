# PGVector Store with Ollama Embeddings Example

This example demonstrates how to use pgvector, a PostgreSQL extension for vector similarity search, with Ollama embeddings in a Go application. It showcases the integration of langchain-go, Ollama, and pgvector to create a powerful vector database for similarity searches.

## What This Example Does

1. **Sets up a PostgreSQL Database with pgvector:**
   - Uses Docker to run a PostgreSQL instance with the pgvector extension installed.
   - Automatically creates and enables the vector extension when the container starts.

2. **Initializes Ollama Embeddings:**
   - Creates an embeddings client using the Ollama API.

3. **Creates a PGVector Store:**
   - Establishes a connection to the PostgreSQL database.
   - Initializes a vector store using pgvector and Ollama embeddings.

4. **Scrape URL for Documents:**
   - Inserts scraped documents with metadata into the vector store.
   - Documents are scraped from site urls.

5. **Performs Similarity Searches:**
   - Demonstrates various types of similarity searches

## Key Features

- Integration of pgvector with Ollama embeddings
- Similarity search with score thresholds
- Metadata filtering in vector searches
- Dockerized PostgreSQL setup for easy deployment

This example provides a practical demonstration of using vector databases for semantic search and similarity matching, which can be incredibly useful for various AI and machine learning applications.
