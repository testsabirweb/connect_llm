#!/bin/bash

echo "Testing MCP Server Connection..."
echo

# Test if MCP server can be started
echo "1. Testing MCP server startup..."
timeout 5s npx -y --package=task-master-ai task-master-ai << EOF
{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"1.0.0","capabilities":{"roots":{"listChanged":true},"sampling":{}}},"id":1}
EOF

echo
echo "2. Checking environment variables..."
echo "OPENROUTER_API_KEY is set: $([ -n "$OPENROUTER_API_KEY" ] && echo "YES" || echo "NO")"

echo
echo "3. Testing CLI commands..."
task-master models | head -20

echo
echo "Test complete. If you see model configuration above, the setup is working." 