#!/bin/bash

# Search API Examples for ConnectLLM
# Make sure the server is running on localhost:8080

echo "=== ConnectLLM Search API Examples ==="
echo ""

# Simple GET search
echo "1. Simple GET search for 'database migration':"
echo "Command: curl 'http://localhost:8080/api/v1/search?q=database%20migration&limit=5'"
curl -s 'http://localhost:8080/api/v1/search?q=database%20migration&limit=5' | jq .
echo ""

# GET search with pagination
echo "2. Paginated search (page 2):"
echo "Command: curl 'http://localhost:8080/api/v1/search?q=security&limit=10&offset=10'"
curl -s 'http://localhost:8080/api/v1/search?q=security&limit=10&offset=10' | jq .
echo ""

# POST search with filters
echo "3. POST search with source filter:"
cat <<EOF > /tmp/search_request.json
{
  "query": "authentication",
  "limit": 10,
  "filters": {
    "source": "slack"
  }
}
EOF

echo "Command: curl -X POST http://localhost:8080/api/v1/search -H 'Content-Type: application/json' -d @/tmp/search_request.json"
curl -s -X POST http://localhost:8080/api/v1/search \
  -H 'Content-Type: application/json' \
  -d @/tmp/search_request.json | jq .
echo ""

# POST search with multiple filters
echo "4. Advanced search with multiple filters:"
cat <<EOF > /tmp/advanced_search.json
{
  "query": "database performance",
  "limit": 15,
  "filters": {
    "source": "slack",
    "tags": ["database", "backend", "performance"],
    "dateFrom": "2023-01-01T00:00:00Z",
    "author": "john.doe"
  }
}
EOF

echo "Command: curl -X POST http://localhost:8080/api/v1/search -H 'Content-Type: application/json' -d @/tmp/advanced_search.json"
curl -s -X POST http://localhost:8080/api/v1/search \
  -H 'Content-Type: application/json' \
  -d @/tmp/advanced_search.json | jq .
echo ""

# Error case - empty query
echo "5. Error case - empty query:"
echo "Command: curl 'http://localhost:8080/api/v1/search?q='"
curl -s 'http://localhost:8080/api/v1/search?q='
echo ""
echo ""

# Clean up
rm -f /tmp/search_request.json /tmp/advanced_search.json

echo "=== End of examples ==="
