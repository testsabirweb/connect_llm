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

// SearchOptions contains options for search queries
type SearchOptions struct {
	Query   []float32
	Limit   int
	Offset  int
	Filters map[string]interface{}
}

// Client interface for vector database operations
type Client interface {
	// Initialize sets up the database schema
	Initialize(ctx context.Context) error

	// Store stores a document with its embedding
	Store(ctx context.Context, doc Document) error

	// Search performs a vector similarity search
	Search(ctx context.Context, query []float32, limit int) ([]Document, error)

	// SearchWithOptions performs a vector similarity search with filters
	SearchWithOptions(ctx context.Context, opts SearchOptions) ([]Document, error)

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

// SearchWithOptions performs a vector similarity search with filters
func (c *WeaviateClient) SearchWithOptions(ctx context.Context, opts SearchOptions) ([]Document, error) {
	// Build the base query
	query := c.client.GraphQL().Get().
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
		)

	// Add vector search
	if len(opts.Query) > 0 {
		query = query.WithNearVector(c.client.GraphQL().NearVectorArgBuilder().
			WithVector(opts.Query))
	}

	// TODO: Add proper filtering support once we understand the correct Weaviate API
	// For now, we'll implement basic search without metadata filtering

	// Apply limit
	if opts.Limit > 0 {
		query = query.WithLimit(opts.Limit)
	}

	// Apply offset for pagination
	if opts.Offset > 0 {
		query = query.WithOffset(opts.Offset)
	}

	// Execute the query
	result, err := query.Do(ctx)
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
	// Check if the response contains any errors
	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("graphql errors: %v", result.Errors)
	}

	// Navigate to the Document class results
	data, ok := result.Data["Get"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response structure: missing Get")
	}

	documentResults, ok := data["Document"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response structure: missing Document array")
	}

	documents := make([]Document, 0, len(documentResults))

	// Parse each document result
	for _, item := range documentResults {
		docMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		doc := Document{
			Metadata: DocumentMetadata{},
		}

		// Extract basic fields
		if content, ok := docMap["content"].(string); ok {
			doc.Content = content
		}
		if source, ok := docMap["source"].(string); ok {
			doc.Source = source
		}
		if sourceId, ok := docMap["sourceId"].(string); ok {
			doc.SourceID = sourceId
		}

		// Extract metadata fields
		if title, ok := docMap["title"].(string); ok {
			doc.Metadata.Title = title
		}
		if author, ok := docMap["author"].(string); ok {
			doc.Metadata.Author = author
		}
		if url, ok := docMap["url"].(string); ok {
			doc.Metadata.URL = url
		}

		// Extract date fields
		if createdAt, ok := docMap["createdAt"].(string); ok {
			if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
				doc.Metadata.CreatedAt = t
			}
		}
		if updatedAt, ok := docMap["updatedAt"].(string); ok {
			if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
				doc.Metadata.UpdatedAt = t
			}
		}

		// Extract array fields
		if perms, ok := docMap["permissions"].([]interface{}); ok {
			doc.Metadata.Permissions = make([]string, 0, len(perms))
			for _, p := range perms {
				if pStr, ok := p.(string); ok {
					doc.Metadata.Permissions = append(doc.Metadata.Permissions, pStr)
				}
			}
		}
		if tags, ok := docMap["tags"].([]interface{}); ok {
			doc.Metadata.Tags = make([]string, 0, len(tags))
			for _, t := range tags {
				if tStr, ok := t.(string); ok {
					doc.Metadata.Tags = append(doc.Metadata.Tags, tStr)
				}
			}
		}

		// Extract additional fields (ID and distance)
		if additional, ok := docMap["_additional"].(map[string]interface{}); ok {
			if id, ok := additional["id"].(string); ok {
				doc.ID = id
			}
			// Note: distance is available here as additional["distance"] if needed
		}

		documents = append(documents, doc)
	}

	return documents, nil
}
