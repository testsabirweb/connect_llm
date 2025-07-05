#!/bin/bash

# Test script for the ingestion service

set -e

echo "=== Testing Connect LLM Ingestion Service ==="
echo

# Check if services are running
echo "1. Checking services..."
if ! curl -s http://localhost:8000/v1/.well-known/ready > /dev/null 2>&1; then
    echo "❌ Weaviate is not running. Please run: make docker-up"
    exit 1
fi
echo "✅ Weaviate is running"

if ! curl -s http://localhost:11434/api/tags > /dev/null 2>&1; then
    echo "❌ Ollama is not running. Please run: make docker-up"
    exit 1
fi
echo "✅ Ollama is running"

# Check if the server is running
if ! curl -s http://localhost:8080/health > /dev/null 2>&1; then
    echo "❌ Server is not running. Please run: make run"
    exit 1
fi
echo "✅ Server is running"

# Check health endpoint
echo
echo "2. Checking health endpoint..."
HEALTH=$(curl -s http://localhost:8080/health)
echo "Health response: $HEALTH"

# Check if we have test data
echo
echo "3. Checking for test data..."
if [ ! -d "slack" ]; then
    echo "❌ No slack directory found. Creating sample data..."
    mkdir -p slack
    cat > slack/test_messages.csv << 'EOF'
client_msg_id,type,text,user,ts,team,user_team,source_team,user_profile,channel_id,thread_ts,parent_user_id,reply_count,reply_users,reply_users_count,latest_reply,reactions,file_ids,subtype,bot_id
msg_001,message,"Hello, this is a test message!",U001,1609459200.000100,T001,T001,T001,,C001,,,0,,,,,,,
msg_002,message,"This is another test message with more content. We're testing the ingestion system.",U002,1609459300.000200,T001,T001,T001,,C001,,,0,,,,,,,
msg_003,message,"Reply to the first message",U003,1609459400.000300,T001,T001,T001,,C001,1609459200.000100,U001,1,U003,1,1609459400.000300,,,,
msg_004,message,"",U004,1609459500.000400,T001,T001,T001,,C001,,,0,,,,,,"[""file_001""]",,
msg_005,message,"Final test message with #hashtag and @mention",U001,1609459600.000500,T001,T001,T001,,C001,,,0,,,,,,,
EOF
    echo "✅ Created sample test data in slack/test_messages.csv"
else
    echo "✅ Found slack directory"
    ls -la slack/*.csv 2>/dev/null || echo "⚠️  No CSV files found in slack directory"
fi

# Test ingestion via API
echo
echo "4. Testing ingestion via API..."
echo "Ingesting test data..."

RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/ingest \
  -H "Content-Type: application/json" \
  -d '{
    "type": "file",
    "path": "slack/test_messages.csv"
  }')

echo "Ingestion response:"
echo "$RESPONSE" | jq . 2>/dev/null || echo "$RESPONSE"

# Check if ingestion was successful
if echo "$RESPONSE" | grep -q '"success":true'; then
    echo "✅ Ingestion completed successfully"

    # Extract stats
    PROCESSED=$(echo "$RESPONSE" | jq -r '.stats.processed_messages' 2>/dev/null || echo "unknown")
    STORED=$(echo "$RESPONSE" | jq -r '.stats.stored_documents' 2>/dev/null || echo "unknown")
    DURATION=$(echo "$RESPONSE" | jq -r '.stats.duration_seconds' 2>/dev/null || echo "unknown")

    echo
    echo "Summary:"
    echo "- Messages processed: $PROCESSED"
    echo "- Documents stored: $STORED"
    echo "- Duration: $DURATION seconds"
else
    echo "❌ Ingestion failed"
    exit 1
fi

# Test querying Weaviate directly
echo
echo "5. Verifying data in Weaviate..."
WEAVIATE_COUNT=$(curl -s -X POST http://localhost:8000/v1/graphql \
  -H 'Content-Type: application/json' \
  -d '{
    "query": "{
      Aggregate {
        Document {
          meta {
            count
          }
        }
      }
    }"
  }' | jq -r '.data.Aggregate.Document[0].meta.count' 2>/dev/null || echo "0")

echo "Documents in Weaviate: $WEAVIATE_COUNT"

if [ "$WEAVIATE_COUNT" -gt 0 ]; then
    echo "✅ Data successfully stored in Weaviate"
else
    echo "⚠️  No documents found in Weaviate"
fi

echo
echo "=== Test completed ==="
echo
echo "Next steps:"
echo "1. To ingest more data: make ingest INPUT=<path>"
echo "2. To query data: Use the search API (coming in Task 6)"
