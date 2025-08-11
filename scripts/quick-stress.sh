#!/bin/bash

# Quick stress test for Haystack server
# A simpler version that focuses on SET operations first

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

ENDPOINT="${1:-haystack-example-trunk.fly.dev:1337}"

echo -e "${BLUE}⚡ Quick Haystack Stress Test${NC}"
echo "================================"
echo -e "${YELLOW}Target: $ENDPOINT${NC}"
echo ""

# Build if needed
if [ ! -f /tmp/haystack-stress ]; then
    echo "Building stress tool..."
    go build -o /tmp/haystack-stress ./cmd/stress
fi

# 1. Quick SET test
echo -e "${BLUE}1. SET Performance Test (10 workers, 20s)${NC}"
/tmp/haystack-stress \
    -endpoint "$ENDPOINT" \
    -workers 10 \
    -duration 20s \
    -workload set \
    -pool 5 \
    -report 5s

echo ""

# 2. Mixed workload
echo -e "${BLUE}2. Mixed Workload Test (20 workers, 30s)${NC}"
/tmp/haystack-stress \
    -endpoint "$ENDPOINT" \
    -workers 20 \
    -duration 30s \
    -workload mixed \
    -pool 10 \
    -report 10s

echo ""
echo -e "${GREEN}✅ Quick stress test completed!${NC}"
echo ""
echo "To monitor server logs:"
echo "  fly logs -a haystack-example-trunk"