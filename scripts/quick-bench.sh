#!/bin/bash

# Quick Haystack Benchmark - Fast performance check
# This script runs a subset of benchmarks for quick performance validation

set -e

echo "⚡ Haystack Quick Performance Check"
echo "=================================="

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}Running quick benchmarks...${NC}"
echo ""

# Quick server benchmarks (3 iterations, shorter duration)
echo -e "${YELLOW}Server Performance:${NC}"
go test -bench="BenchmarkServer_(SET|GET)$" -benchmem -count=1 -benchtime=1s ./server/ | \
    grep -E "(BenchmarkServer|ops/sec|ns/op)" | \
    grep -v "testing:" || echo "No server results"

echo ""

# Quick client benchmarks
echo -e "${YELLOW}Client Performance:${NC}"
go test -bench="BenchmarkClient_(Set|Get)$" -benchmem -count=1 -benchtime=1s ./client/ | \
    grep -E "(BenchmarkClient|ops/sec|ns/op)" | \
    grep -v "testing:" || echo "No client results"

echo ""

# Quick concurrent test
echo -e "${YELLOW}Concurrent Performance:${NC}"
go test -bench="BenchmarkServer_Concurrent_SET" -benchmem -count=1 -benchtime=2s ./server/ | \
    grep -E "(BenchmarkServer_Concurrent|ops/sec|ns/op)" | \
    grep -v "testing:" || echo "No concurrent results"

echo ""
echo -e "${GREEN}✓ Quick benchmark completed!${NC}"
echo ""
echo -e "${BLUE}For comprehensive benchmarks, run:${NC}"
echo "  ./scripts/benchmark.sh"