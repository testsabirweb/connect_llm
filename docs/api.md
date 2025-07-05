# ConnectLLM API Documentation

## Search API

### Endpoint: `/api/v1/search`

The search endpoint provides semantic search functionality over ingested documents using vector similarity search.

### Methods

- **GET** - Simple search with query parameters
- **POST** - Advanced search with JSON body

### Request Parameters

#### GET Request

Query parameters:

- `q` (required) - The search query text
- `limit` (optional) - Maximum number of results to return (default: 10, max: 100)
- `offset` (optional) - Offset for pagination (default: 0)

Example:

```http
GET /api/v1/search?q=user%20authentication&limit=20&offset=0
```

#### POST Request

JSON body:

```json
{
  "query": "user authentication",
  "limit": 20,
  "offset": 0,
  "filters": {
    "source": "slack",
    "author": "john.doe",
    "tags": ["security", "auth"],
    "dateFrom": "2023-01-01T00:00:00Z",
    "dateTo": "2023-12-31T23:59:59Z",
    "requirePermission": "user123"
  }
}
```

##### Filter Options

- `source` - Filter by source system (e.g., "slack", "github")
- `author` - Filter by document author
- `tags` - Filter by any of the provided tags (array)
- `dateFrom` - Filter documents created after this date (RFC3339 format)
- `dateTo` - Filter documents created before this date (RFC3339 format)
- `requirePermission` - Filter documents accessible by this user ID

### Response

```json
{
  "results": [
    {
      "id": "doc-123",
      "content": "This is a snippet of the document content...",
      "score": 0.95,
      "source": "slack",
      "sourceId": "msg-456",
      "title": "Security Discussion",
      "author": "john.doe",
      "url": "https://slack.com/archives/...",
      "createdAt": "2023-06-15T10:30:00Z",
      "updatedAt": "2023-06-15T10:30:00Z",
      "tags": ["security", "authentication"],
      "highlights": []
    }
  ],
  "total": 42,
  "count": 20,
  "offset": 0,
  "processingTimeMs": 125,
  "metadata": {
    "processedQuery": "user authentication",
    "documentsSearched": -1,
    "filtersApplied": {
      "source": "slack"
    }
  }
}
```

#### Response Fields

- `results` - Array of search results
  - `id` - Document ID
  - `content` - Content snippet (max 500 characters)
  - `score` - Relevance score (0-1, higher is better)
  - `source` - Source system
  - `sourceId` - ID in the source system
  - `title` - Document title
  - `author` - Document author
  - `url` - Original document URL
  - `createdAt` - Creation timestamp
  - `updatedAt` - Last update timestamp
  - `tags` - Document tags
  - `highlights` - Highlighted search terms (currently empty)
- `total` - Total number of matching documents
- `count` - Number of results in this response
- `offset` - Offset used for this query
- `processingTimeMs` - Query processing time in milliseconds
- `metadata` - Additional search metadata
  - `processedQuery` - The actual query used
  - `documentsSearched` - Number of documents searched (-1 if unknown)
  - `filtersApplied` - Summary of applied filters

### Error Responses

#### 400 Bad Request

```json
{
  "error": "search query cannot be empty"
}
```

#### 405 Method Not Allowed

```json
{
  "error": "Method not allowed"
}
```

#### 500 Internal Server Error

```json
{
  "error": "Search failed"
}
```

### Examples

#### Simple Search (GET)

```bash
curl "http://localhost:8080/api/v1/search?q=database%20migration&limit=5"
```

#### Advanced Search with Filters (POST)

```bash
curl -X POST http://localhost:8080/api/v1/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "database migration",
    "limit": 10,
    "filters": {
      "source": "slack",
      "tags": ["database", "backend"],
      "dateFrom": "2023-06-01T00:00:00Z"
    }
  }'
```

#### Pagination Example

```bash
# First page
curl "http://localhost:8080/api/v1/search?q=security&limit=20&offset=0"

# Second page
curl "http://localhost:8080/api/v1/search?q=security&limit=20&offset=20"
```

### Notes

- The search uses semantic similarity, so results may include documents that don't contain the exact search terms but are conceptually related
- Filters are currently limited to exact matches (except tags which use "any match")
- The `total` count currently returns the number of results returned, not the total matching documents in the database
- Metadata filtering is not yet fully implemented in the Weaviate integration
