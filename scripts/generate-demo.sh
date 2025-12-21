#!/bin/bash
set -e

# Generate Demo Materials for Stream Relay Go
# This script helps create screenshots and demo content

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}🎬 Stream Relay Demo Generator${NC}"
echo ""

# Check if relay is running
if ! curl -s http://localhost:8080/healthz > /dev/null 2>&1; then
    echo -e "${RED}❌ Relay is not running!${NC}"
    echo "Please start the relay first:"
    echo "  make dev"
    exit 1
fi

echo -e "${GREEN}✅ Relay is running${NC}"

# Check if Grafana is running
if ! curl -s http://localhost:3000/api/health > /dev/null 2>&1; then
    echo -e "${YELLOW}⚠️  Grafana is not running${NC}"
    echo "Start Grafana for dashboard screenshots:"
    echo "  cd deployments/grafana && docker-compose up -d"
else
    echo -e "${GREEN}✅ Grafana is running${NC}"
fi

echo ""
echo -e "${YELLOW}Generating demo traffic...${NC}"

# Generate varied traffic for interesting metrics
for i in {1..20}; do
    # Vary the prompts for more interesting demos
    PROMPTS=(
        "Write a haiku about programming"
        "Explain quantum computing in one sentence"
        "What is the meaning of life?"
        "Count from 1 to 5"
        "Tell me a joke"
    )

    PROMPT="${PROMPTS[$((i % 5))]}"

    echo -e "${GREEN}[$i/20]${NC} Sending request: \"$PROMPT\""

    curl -s -N http://localhost:8080/v1/chat/completions \
      -H 'Authorization: Bearer sk-relay-test-key-123' \
      -H 'Content-Type: application/json' \
      -d "{
        \"model\": \"Qwen/Qwen2.5-7B-Instruct\",
        \"messages\": [{\"role\": \"user\", \"content\": \"$PROMPT\"}],
        \"stream\": true,
        \"max_tokens\": $((20 + i * 2))
      }" > /dev/null 2>&1

    # Small delay between requests
    sleep 0.5
done

echo ""
echo -e "${GREEN}✅ Generated 20 requests${NC}"
echo ""
echo -e "${YELLOW}📊 View metrics:${NC}"
echo "  Grafana:    http://localhost:3000"
echo "  Prometheus: http://localhost:9090"
echo "  Metrics:    http://localhost:8080/metrics"
echo ""
echo -e "${YELLOW}📸 Next steps:${NC}"
echo "1. Open Grafana at http://localhost:3000 (admin/admin)"
echo "2. Navigate to 'Stream Relay Monitoring' dashboard"
echo "3. Take a full-screen screenshot"
echo "4. Save as: docs/images/grafana-dashboard.png"
echo ""
echo "For detailed instructions, see: docs/DEMO.md"
