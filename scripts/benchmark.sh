#!/bin/bash

# Haystack Performance Benchmark Script
# This script runs comprehensive benchmarks and generates performance reports

set -e

echo "🚀 Haystack Performance Benchmark Suite"
echo "========================================"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Create results directory
RESULTS_DIR="benchmark_results_$(date +%Y%m%d_%H%M%S)"
mkdir -p "$RESULTS_DIR"

echo -e "${BLUE}Results will be saved to: $RESULTS_DIR${NC}"
echo ""

# Function to run benchmark and save results
run_benchmark() {
    local name="$1"
    local package="$2"
    local bench_pattern="$3"
    local output_file="$RESULTS_DIR/${name}.txt"
    
    echo -e "${YELLOW}Running $name benchmarks...${NC}"
    
    if go test -bench="$bench_pattern" -benchmem -count=3 -timeout=30m "$package" > "$output_file" 2>&1; then
        echo -e "${GREEN}✓ $name completed${NC}"
        
        # Extract key metrics
        echo "Key Results:" >> "$output_file"
        echo "============" >> "$output_file"
        # Avoid reading and writing same file in pipeline - use temp file
        grep -E "(ops/sec|ns/op|B/op|allocs/op)" "$output_file" | tail -10 > "$output_file.tmp" || true
        cat "$output_file.tmp" >> "$output_file"
        rm -f "$output_file.tmp"
    else
        echo -e "${RED}✗ $name failed${NC}"
        echo "Error details saved to $output_file"
    fi
    echo ""
}

# 1. Server Benchmarks
echo -e "${BLUE}1. Server Performance Benchmarks${NC}"
run_benchmark "server_basic" "./server/" "BenchmarkServer_(SET|GET)$"
run_benchmark "server_concurrent" "./server/" "BenchmarkServer_Concurrent"
run_benchmark "server_mixed" "./server/" "BenchmarkServer_Mixed"
run_benchmark "server_throughput" "./server/" "BenchmarkServer_Throughput"

# 2. Client Benchmarks  
echo -e "${BLUE}2. Client Library Benchmarks${NC}"
run_benchmark "client_basic" "./client/" "BenchmarkClient_(Set|Get)$"
run_benchmark "client_concurrent" "./client/" "BenchmarkClient_Concurrent"
run_benchmark "client_mixed" "./client/" "BenchmarkClient_Mixed"
run_benchmark "client_pool" "./client/" "BenchmarkClient_ConnectionPool"
run_benchmark "client_throughput" "./client/" "BenchmarkClient_HighThroughput"

# 3. End-to-End Benchmarks
echo -e "${BLUE}3. End-to-End Performance Benchmarks${NC}"
if [ -d "./benchmarks" ]; then
    run_benchmark "e2e_workload" "./benchmarks/" "BenchmarkE2E_Realistic"
    run_benchmark "e2e_scale" "./benchmarks/" "BenchmarkE2E_Throughput_Scale"
    run_benchmark "e2e_latency" "./benchmarks/" "BenchmarkE2E_Latency"
    run_benchmark "e2e_memory" "./benchmarks/" "BenchmarkE2E_Memory_Pressure"
else
    echo -e "${YELLOW}Skipping E2E benchmarks (./benchmarks not found)${NC}"
fi

# 4. Generate Summary Report
echo -e "${BLUE}4. Generating Summary Report${NC}"
SUMMARY_FILE="$RESULTS_DIR/SUMMARY.md"

cat > "$SUMMARY_FILE" << EOF
# Haystack Performance Benchmark Results

**Date:** $(date)
**Go Version:** $(go version)
**Hardware:** $(uname -m)
**OS:** $(uname -s)

## Test Environment
- **CPU:** $(sysctl -n machdep.cpu.brand_string 2>/dev/null || grep "model name" /proc/cpuinfo | head -1 | cut -d: -f2 | xargs || echo "Unknown")
- **Memory:** $(($(sysctl -n hw.memsize 2>/dev/null || grep MemTotal /proc/meminfo | awk '{print $2 * 1024}' || echo 0) / 1024 / 1024 / 1024))GB
- **Go Version:** $(go version)

## Key Performance Metrics

EOF

# Extract and summarize key metrics
echo "### Server Performance" >> "$SUMMARY_FILE"
echo "" >> "$SUMMARY_FILE"

for file in "$RESULTS_DIR"/server_*.txt; do
    if [ -f "$file" ]; then
        filename=$(basename "$file" .txt)
        echo "#### ${filename//_/ }" >> "$SUMMARY_FILE"
        echo "\`\`\`" >> "$SUMMARY_FILE"
        grep -E "(BenchmarkServer.*-.*ops/sec|BenchmarkServer.*-.*ns/op)" "$file" | head -5 >> "$SUMMARY_FILE" 2>/dev/null || echo "No results found" >> "$SUMMARY_FILE"
        echo "\`\`\`" >> "$SUMMARY_FILE"
        echo "" >> "$SUMMARY_FILE"
    fi
done

echo "### Client Performance" >> "$SUMMARY_FILE"
echo "" >> "$SUMMARY_FILE"

for file in "$RESULTS_DIR"/client_*.txt; do
    if [ -f "$file" ]; then
        filename=$(basename "$file" .txt)
        echo "#### ${filename//_/ }" >> "$SUMMARY_FILE"
        echo "\`\`\`" >> "$SUMMARY_FILE"
        grep -E "(BenchmarkClient.*-.*ops/sec|BenchmarkClient.*-.*ns/op)" "$file" | head -5 >> "$SUMMARY_FILE" 2>/dev/null || echo "No results found" >> "$SUMMARY_FILE"
        echo "\`\`\`" >> "$SUMMARY_FILE"
        echo "" >> "$SUMMARY_FILE"
    fi
done

echo "### End-to-End Performance" >> "$SUMMARY_FILE"
echo "" >> "$SUMMARY_FILE"

for file in "$RESULTS_DIR"/e2e_*.txt; do
    if [ -f "$file" ]; then
        filename=$(basename "$file" .txt)
        echo "#### ${filename//_/ }" >> "$SUMMARY_FILE"
        echo "\`\`\`" >> "$SUMMARY_FILE"
        grep -E "(BenchmarkE2E.*-.*ops/sec|BenchmarkE2E.*-.*ns/op)" "$file" | head -5 >> "$SUMMARY_FILE" 2>/dev/null || echo "No results found" >> "$SUMMARY_FILE"
        echo "\`\`\`" >> "$SUMMARY_FILE"
        echo "" >> "$SUMMARY_FILE"
    fi
done

# 5. Quick Performance Analysis
echo -e "${BLUE}5. Quick Performance Analysis${NC}"

# Extract best throughput numbers
echo "## Performance Highlights" >> "$SUMMARY_FILE"
echo "" >> "$SUMMARY_FILE"

# Find highest ops/sec across all benchmarks
MAX_OPS=$(grep -h "ops/sec" "$RESULTS_DIR"/*.txt 2>/dev/null | \
          grep -o '[0-9.]*ops/sec' | \
          sed 's/ops\/sec//' | \
          sort -n | \
          tail -1 || echo "0")

if [ "$MAX_OPS" != "0" ]; then
    echo "- **Peak Throughput:** ${MAX_OPS} ops/sec" >> "$SUMMARY_FILE"
fi

# Find lowest latency
MIN_LATENCY=$(grep -h "ns/op" "$RESULTS_DIR"/*.txt 2>/dev/null | \
              grep -o '[0-9.]*ns/op' | \
              sed 's/ns\/op//' | \
              sort -n | \
              head -1 || echo "0")

if [ "$MIN_LATENCY" != "0" ]; then
    echo "- **Lowest Latency:** ${MIN_LATENCY} ns/op" >> "$SUMMARY_FILE"
fi

{
    echo ""
    echo "## Files Generated"
    echo ""
    echo "- Individual benchmark results: \`${RESULTS_DIR}/*.txt\`"
    echo "- This summary: \`${SUMMARY_FILE}\`"
} >> "$SUMMARY_FILE"

echo -e "${GREEN}✓ Benchmark suite completed!${NC}"
echo -e "${BLUE}Summary report: $SUMMARY_FILE${NC}"
echo ""

# Display quick summary
echo -e "${YELLOW}Quick Summary:${NC}"
if [ "$MAX_OPS" != "0" ]; then
    echo -e "Peak Throughput: ${GREEN}${MAX_OPS} ops/sec${NC}"
fi
if [ "$MIN_LATENCY" != "0" ]; then
    echo -e "Lowest Latency: ${GREEN}${MIN_LATENCY} ns/op${NC}"
fi

echo ""
echo -e "${BLUE}To view detailed results:${NC}"
echo "  cat $SUMMARY_FILE"
echo "  ls $RESULTS_DIR/"