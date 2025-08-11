#!/bin/bash

# Haystack Stress Testing Script
# Tests the deployed Haystack server under various load conditions

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Default values
DEFAULT_ENDPOINT="haystack-example-trunk.fly.dev:1337"
ENDPOINT="${1:-$DEFAULT_ENDPOINT}"

echo -e "${BLUE}🔥 Haystack Stress Testing Suite${NC}"
echo "======================================"
echo -e "${YELLOW}Target: $ENDPOINT${NC}"
echo ""

# Build the stress tool
echo -e "${BLUE}Building stress test tool...${NC}"
go build -o /tmp/haystack-stress ./cmd/stress
echo -e "${GREEN}✓ Built${NC}"
echo ""

# Function to run a stress test
run_stress_test() {
    local name="$1"
    local workers="$2"
    local duration="$3"
    local workload="$4"
    local pool="${5:-5}"
    
    echo -e "${YELLOW}Test: $name${NC}"
    echo "Workers: $workers, Duration: $duration, Workload: $workload, Pool: $pool"
    echo "---"
    
    /tmp/haystack-stress \
        -endpoint "$ENDPOINT" \
        -workers "$workers" \
        -duration "$duration" \
        -workload "$workload" \
        -pool "$pool" \
        -report 10s
    
    echo ""
    echo "---"
    sleep 2
}

# 1. Warm-up test
echo -e "${BLUE}1. Warm-up Test${NC}"
echo "Light load to establish baseline"
run_stress_test "Warm-up" 5 15s mixed 3
echo ""

# 2. Throughput test
echo -e "${BLUE}2. Throughput Test${NC}"
echo "Testing maximum throughput with many workers"
run_stress_test "High Throughput" 50 30s mixed 10
echo ""

# 3. SET-heavy workload
echo -e "${BLUE}3. Write-Heavy Test${NC}"
echo "Testing write performance"
run_stress_test "Write Heavy" 20 30s set 5
echo ""

# 4. GET-heavy workload
echo -e "${BLUE}4. Read-Heavy Test${NC}"
echo "Testing read performance"
run_stress_test "Read Heavy" 30 30s get 5
echo ""

# 5. Realistic workload
echo -e "${BLUE}5. Realistic Workload Test${NC}"
echo "Simulating real-world usage patterns with bursts"
run_stress_test "Realistic" 25 45s realistic 8
echo ""

# 6. Stress test (if confirmed)
echo -e "${RED}6. Maximum Stress Test${NC}"
echo -e "${YELLOW}⚠️  This will put significant load on the server${NC}"
read -p "Run maximum stress test? (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    run_stress_test "Maximum Stress" 100 60s mixed 20
else
    echo "Skipped maximum stress test"
fi

echo ""
echo -e "${GREEN}✅ Stress testing completed!${NC}"
echo ""
echo -e "${BLUE}Summary:${NC}"
echo "- The server handled various workload patterns"
echo "- Check the individual test results above for performance metrics"
echo "- Monitor server logs for any issues: fly logs -a haystack-example-trunk"

# Clean up
rm -f /tmp/haystack-stress