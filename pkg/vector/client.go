package vector

import (
	"context"
	"fmt"
	"time"

	"github.com/weaviate/weaviate-go-client/v4/weaviate"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/auth"
	"github.com/weaviate/weaviate-go-client/v4/weaviate/graphql"
	"github.com/weaviate/weaviate/entities/models"
)

// Document represents a document to be stored in the vector database
type Document struct {
	ID        string
	Content   string
	Embedding []float32
	Source    string
	SourceID  string
	Metadata  DocumentMetadata
}

// DocumentMetadata contains metadata for a document
type DocumentMetadata struct {
	Title       string
	Author      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Permissions []string
	Tags        []string
	URL         string
}

// Client interface for vector database operations
type Client interface {
	// Initialize sets up the database schema
	Initialize(ctx context.Context) error

	// Store stores a document with its embedding
	Store(ctx context.Context, doc Document) error

	// Search performs a vector similarity search
	Search(ctx context.Context, query []float32, limit int) ([]Document, error)

	// Delete removes a document by ID
	Delete(ctx context.Context, id string) error

	// HealthCheck verifies the connection to the vector database
	HealthCheck(ctx context.Context) error
}

// WeaviateClient implements the Client interface for Weaviate
type WeaviateClient struct {
	client *weaviate.Client
	scheme string
	host   string
}

// NewWeaviateClient creates a new Weaviate client
func NewWeaviateClient(scheme, host string, apiKey string) (*WeaviateClient, error) {
	if host == "" {
		return nil, fmt.Errorf("weaviate host cannot be empty")
	}

	cfg := weaviate.Config{
		Scheme: scheme,
		Host:   host,
	}

	// Add API key authentication if provided
	if apiKey != "" {
		cfg.AuthConfig = auth.ApiKey{Value: apiKey}
	}

	client, err := weaviate.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create weaviate client: %w", err)
	}

	return &WeaviateClient{
		client: client,
		scheme: scheme,
		host:   host,
	}, nil
}

// Initialize sets up the Weaviate schema
func (c *WeaviateClient) Initialize(ctx context.Context) error {
	// Check if Document class already exists
	exists, err := c.client.Schema().ClassExistenceChecker().
		WithClassName("Document").
		Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to check class existence: %w", err)
	}

	if exists {
		// Class already exists, no need to create
		return nil
	}

	// Create the Document class schema
	classObj := &models.Class{
		Class:       "Document",
		Description: "A document with content and metadata",
		Properties: []*models.Property{
			{
				Name:        "content",
				DataType:    []string{"text"},
				Description: "The main content of the document",
			},
			{
				Name:        "source",
				DataType:    []string{"string"},
				Description: "The source system of the document",
			},
			{
				Name:        "sourceId",
				DataType:    []string{"string"},
				Description: "The ID in the source system",
			},
			{
				Name:        "title",
				DataType:    []string{"string"},
				Description: "The title of the document",
			},
			{
				Name:        "author",
				DataType:    []string{"string"},
				Description: "The author of the document",
			},
			{
				Name:        "createdAt",
				DataType:    []string{"date"},
				Description: "When the document was created",
			},
			{
				Name:        "updatedAt",
				DataType:    []string{"date"},
				Description: "When the document was last updated",
			},
			{
				Name:        "permissions",
				DataType:    []string{"string[]"},
				Description: "User IDs with access to this document",
			},
			{
				Name:        "tags",
				DataType:    []string{"string[]"},
				Description: "Tags associated with the document",
			},
			{
				Name:        "url",
				DataType:    []string{"string"},
				Description: "URL to the original document",
			},
		},
		VectorIndexType: "hnsw",
		VectorIndexConfig: map[string]interface{}{
			"distance": "cosine",
		},
	}

	err = c.client.Schema().ClassCreator().
		WithClass(classObj).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("failed to create class schema: %w", err)
	}

	return nil
}

// Store stores a document in Weaviate
func (c *WeaviateClient) Store(ctx context.Context, doc Document) error {
	// Create the data object
	dataObj := map[string]interface{}{
		"content":     doc.Content,
		"source":      doc.Source,
		"sourceId":    doc.SourceID,
		"title":       doc.Metadata.Title,
		"author":      doc.Metadata.Author,
		"createdAt":   doc.Metadata.CreatedAt,
		"updatedAt":   doc.Metadata.UpdatedAt,
		"permissions": doc.Metadata.Permissions,
		"tags":        doc.Metadata.Tags,
		"url":         doc.Metadata.URL,
	}

	// Store the document with its embedding
	_, err := c.client.Data().Creator().
		WithClassName("Document").
		WithID(doc.ID).
		WithProperties(dataObj).
		WithVector(doc.Embedding).
		Do(ctx)

	if err != nil {
		return fmt.Errorf("failed to store document: %w", err)
	}

	return nil
}

// Search performs vector similarity search in Weaviate
func (c *WeaviateClient) Search(ctx context.Context, query []float32, limit int) ([]Document, error) {
	result, err := c.client.GraphQL().Get().
		WithClassName("Document").
		WithFields(
			graphql.Field{Name: "content"},
			graphql.Field{Name: "source"},
			graphql.Field{Name: "sourceId"},
			graphql.Field{Name: "title"},
			graphql.Field{Name: "author"},
			graphql.Field{Name: "createdAt"},
			graphql.Field{Name: "updatedAt"},
			graphql.Field{Name: "permissions"},
			graphql.Field{Name: "tags"},
			graphql.Field{Name: "url"},
			graphql.Field{Name: "_additional", Fields: []graphql.Field{
				{Name: "id"},
				{Name: "distance"},
			}},
		).
		WithNearVector(c.client.GraphQL().NearVectorArgBuilder().
			WithVector(query)).
		WithLimit(limit).
		Do(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to search documents: %w", err)
	}

	return c.parseSearchResults(result)
}

// Delete removes a document from Weaviate
func (c *WeaviateClient) Delete(ctx context.Context, id string) error {
	err := c.client.Data().Deleter().
		WithClassName("Document").
		WithID(id).
		Do(ctx)

	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	return nil
}

// HealthCheck verifies Weaviate connection
func (c *WeaviateClient) HealthCheck(ctx context.Context) error {
	ready, err := c.client.Misc().ReadyChecker().Do(ctx)
	if err != nil {
		return fmt.Errorf("weaviate health check failed: %w", err)
	}

	if !ready {
		return fmt.Errorf("weaviate is not ready")
	}

	return nil
}

// parseSearchResults converts Weaviate GraphQL results to Document slice
func (c *WeaviateClient) parseSearchResults(result *models.GraphQLResponse) ([]Document, error) {
	// Implementation would parse the GraphQL response
	// This is a placeholder - actual implementation would extract documents
	// from the result.Data.Get map structure
	documents := []Document{}

	// TODO: Implement actual parsing logic

	return documents, nil
}
