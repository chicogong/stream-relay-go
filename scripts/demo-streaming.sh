#!/bin/bash

# Simple streaming demo script
# Record this with asciinema or terminalizer for docs/images/streaming-demo.gif

echo "🚀 Stream Relay Demo - Token Streaming"
echo ""
echo "Sending a streaming request to the relay..."
echo ""

# Show the command
cat << 'EOF'
curl -N http://localhost:8080/v1/chat/completions \
  -H 'Authorization: Bearer sk-relay-test-key-123' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "Qwen/Qwen2.5-7B-Instruct",
    "messages": [
      {"role": "user", "content": "Write a haiku about streaming data"}
    ],
    "stream": true,
    "max_tokens": 50
  }'
EOF

echo ""
echo "---"
echo ""

# Execute the request
curl -N http://localhost:8080/v1/chat/completions \
  -H 'Authorization: Bearer sk-relay-test-key-123' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "Qwen/Qwen2.5-7B-Instruct",
    "messages": [
      {"role": "user", "content": "Write a haiku about streaming data"}
    ],
    "stream": true,
    "max_tokens": 50
  }'

echo ""
echo ""
echo "✅ Stream complete! Tokens arrived in real-time."
