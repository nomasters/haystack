#!/bin/bash

# Stress test for remote Haystack server
# Generates a corpus of data, stores it, then reads it back

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

ENDPOINT="${1:-haystack-example-trunk.fly.dev:1337}"

echo -e "${BLUE}🔥 Haystack Remote Stress Test${NC}"
echo "======================================"
echo -e "${YELLOW}Target: $ENDPOINT${NC}"
echo ""

echo ""
echo -e "${BLUE}Test Configuration:${NC}"
echo "• 10 MB of test data (~54,613 needles)"
echo "• Phase 1: Store all data with 5 workers"
echo "• Phase 2: Read data for 30 seconds with 20 workers"
echo ""

read -p "Continue with stress test? (y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Aborted."
    exit 0
fi

echo ""
echo -e "${GREEN}Starting stress test...${NC}"
echo ""

go run ./cmd/stress \
    -endpoint "$ENDPOINT" \
    -size 10 \
    -set-workers 5 \
    -get-workers 20 \
    -get-duration 30s \
    -pool 15 \
    -report 5s

echo ""
echo -e "${GREEN}✅ Stress test completed!${NC}"
echo ""
echo "To view server logs:"
echo "  fly logs -a haystack-example-trunk"
echo ""
echo "For a more intensive test, run:"
echo "  go run ./cmd/stress -endpoint $ENDPOINT -size 100 -set-workers 10 -get-workers 50 -get-duration 60s"