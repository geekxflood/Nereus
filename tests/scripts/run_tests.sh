#!/bin/bash

# Nereus Test Runner Script
# Comprehensive test execution with reporting and cleanup

set -e

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TEST_DIR="$PROJECT_ROOT/tests"
REPORTS_DIR="$PROJECT_ROOT/test-reports"
COVERAGE_DIR="$REPORTS_DIR/coverage"
LOG_DIR="$REPORTS_DIR/logs"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test configuration
TIMEOUT=${TEST_TIMEOUT:-300}  # 5 minutes default timeout
PARALLEL=${TEST_PARALLEL:-4}  # Number of parallel test processes
VERBOSE=${TEST_VERBOSE:-false}
SHORT=${TEST_SHORT:-false}

# Function to print colored output
print_status() {
    local color=$1
    local message=$2
    echo -e "${color}[$(date '+%Y-%m-%d %H:%M:%S')] ${message}${NC}"
}

print_info() {
    print_status "$BLUE" "INFO: $1"
}

print_success() {
    print_status "$GREEN" "SUCCESS: $1"
}

print_warning() {
    print_status "$YELLOW" "WARNING: $1"
}

print_error() {
    print_status "$RED" "ERROR: $1"
}

# Function to setup test environment
setup_test_environment() {
    print_info "Setting up test environment..."
    
    # Create directories
    mkdir -p "$REPORTS_DIR" "$COVERAGE_DIR" "$LOG_DIR"
    
    # Clean up any existing test artifacts
    rm -rf "$PROJECT_ROOT"/test-data/nereus-test*.db
    rm -rf "$PROJECT_ROOT"/test-temp-*
    
    # Kill any processes using test ports
    for port in 1162 9091; do
        if lsof -ti:$port >/dev/null 2>&1; then
            print_warning "Killing processes using port $port"
            lsof -ti:$port | xargs -r kill -9
            sleep 1
        fi
    done
    
    # Ensure Go modules are up to date
    cd "$PROJECT_ROOT"
    go mod download
    go mod tidy
    
    print_success "Test environment setup complete"
}

# Function to run a specific test suite
run_test_suite() {
    local suite_name=$1
    local suite_path=$2
    local extra_flags=$3
    
    print_info "Running $suite_name tests..."
    
    local log_file="$LOG_DIR/${suite_name}.log"
    local coverage_file="$COVERAGE_DIR/${suite_name}.out"
    
    # Build test flags
    local test_flags="-v -timeout=${TIMEOUT}s -parallel=$PARALLEL"
    
    if [ "$SHORT" = "true" ]; then
        test_flags="$test_flags -short"
    fi
    
    if [ "$VERBOSE" = "true" ]; then
        test_flags="$test_flags -v"
    fi
    
    # Add coverage if requested
    if [ "${ENABLE_COVERAGE:-true}" = "true" ]; then
        test_flags="$test_flags -coverprofile=$coverage_file -covermode=atomic"
    fi
    
    # Add extra flags
    if [ -n "$extra_flags" ]; then
        test_flags="$test_flags $extra_flags"
    fi
    
    # Run tests
    cd "$PROJECT_ROOT"
    if go test $test_flags "$suite_path" 2>&1 | tee "$log_file"; then
        print_success "$suite_name tests passed"
        return 0
    else
        print_error "$suite_name tests failed"
        return 1
    fi
}

# Function to run all test suites
run_all_tests() {
    local failed_suites=()
    local total_suites=0
    
    print_info "Starting comprehensive test run..."
    
    # Define test suites
    declare -A test_suites=(
        ["unit"]="./internal/..."
        ["integration"]="./tests/integration/..."
        ["e2e"]="./tests/e2e/..."
        ["performance"]="./tests/performance/..."
        ["security"]="./tests/security/..."
        ["chaos"]="./tests/chaos/..."
    )
    
    # Run each test suite
    for suite_name in "${!test_suites[@]}"; do
        total_suites=$((total_suites + 1))
        
        if ! run_test_suite "$suite_name" "${test_suites[$suite_name]}"; then
            failed_suites+=("$suite_name")
        fi
        
        # Small delay between suites
        sleep 2
    done
    
    # Report results
    local passed_suites=$((total_suites - ${#failed_suites[@]}))
    
    print_info "Test Summary:"
    print_info "  Total suites: $total_suites"
    print_info "  Passed: $passed_suites"
    print_info "  Failed: ${#failed_suites[@]}"
    
    if [ ${#failed_suites[@]} -eq 0 ]; then
        print_success "All test suites passed!"
        return 0
    else
        print_error "Failed suites: ${failed_suites[*]}"
        return 1
    fi
}

# Function to generate coverage report
generate_coverage_report() {
    print_info "Generating coverage report..."
    
    cd "$PROJECT_ROOT"
    
    # Combine coverage files
    local combined_coverage="$COVERAGE_DIR/combined.out"
    echo "mode: atomic" > "$combined_coverage"
    
    for coverage_file in "$COVERAGE_DIR"/*.out; do
        if [ -f "$coverage_file" ] && [ "$(basename "$coverage_file")" != "combined.out" ]; then
            tail -n +2 "$coverage_file" >> "$combined_coverage"
        fi
    done
    
    # Generate HTML report
    if [ -f "$combined_coverage" ]; then
        go tool cover -html="$combined_coverage" -o "$COVERAGE_DIR/coverage.html"
        
        # Generate summary
        local coverage_percent=$(go tool cover -func="$combined_coverage" | grep total | awk '{print $3}')
        print_info "Total coverage: $coverage_percent"
        
        # Save coverage summary
        echo "Coverage Report - $(date)" > "$COVERAGE_DIR/summary.txt"
        echo "Total Coverage: $coverage_percent" >> "$COVERAGE_DIR/summary.txt"
        echo "" >> "$COVERAGE_DIR/summary.txt"
        go tool cover -func="$combined_coverage" >> "$COVERAGE_DIR/summary.txt"
        
        print_success "Coverage report generated: $COVERAGE_DIR/coverage.html"
    else
        print_warning "No coverage data found"
    fi
}

# Function to run benchmarks
run_benchmarks() {
    print_info "Running performance benchmarks..."
    
    local benchmark_log="$LOG_DIR/benchmarks.log"
    
    cd "$PROJECT_ROOT"
    if go test -bench=. -benchmem -timeout=600s ./tests/performance/... 2>&1 | tee "$benchmark_log"; then
        print_success "Benchmarks completed"
        
        # Extract benchmark results
        grep "Benchmark" "$benchmark_log" > "$REPORTS_DIR/benchmark_results.txt" || true
        
        return 0
    else
        print_error "Benchmarks failed"
        return 1
    fi
}

# Function to cleanup test environment
cleanup_test_environment() {
    print_info "Cleaning up test environment..."
    
    # Kill any remaining test processes
    for port in 1162 9091; do
        if lsof -ti:$port >/dev/null 2>&1; then
            lsof -ti:$port | xargs -r kill -9
        fi
    done
    
    # Clean up test artifacts
    rm -rf "$PROJECT_ROOT"/test-data/nereus-test*.db
    rm -rf "$PROJECT_ROOT"/test-temp-*
    
    print_success "Cleanup complete"
}

# Function to show usage
show_usage() {
    cat << EOF
Usage: $0 [OPTIONS] [COMMAND]

Commands:
    all         Run all test suites (default)
    unit        Run unit tests only
    integration Run integration tests only
    e2e         Run end-to-end tests only
    performance Run performance tests only
    security    Run security tests only
    chaos       Run chaos engineering tests only
    benchmarks  Run performance benchmarks
    coverage    Generate coverage report only
    clean       Clean up test environment

Options:
    -h, --help      Show this help message
    -v, --verbose   Enable verbose output
    -s, --short     Run tests in short mode (skip long-running tests)
    -p, --parallel  Number of parallel test processes (default: 4)
    -t, --timeout   Test timeout in seconds (default: 300)
    --no-coverage   Disable coverage collection

Environment Variables:
    TEST_VERBOSE    Enable verbose output (true/false)
    TEST_SHORT      Run in short mode (true/false)
    TEST_PARALLEL   Number of parallel processes
    TEST_TIMEOUT    Test timeout in seconds
    ENABLE_COVERAGE Enable coverage collection (true/false)

Examples:
    $0                          # Run all tests
    $0 unit                     # Run unit tests only
    $0 -v -s e2e               # Run e2e tests in verbose short mode
    $0 --parallel=8 performance # Run performance tests with 8 parallel processes

EOF
}

# Main execution
main() {
    local command="all"
    
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_usage
                exit 0
                ;;
            -v|--verbose)
                VERBOSE=true
                shift
                ;;
            -s|--short)
                SHORT=true
                shift
                ;;
            -p|--parallel)
                PARALLEL="$2"
                shift 2
                ;;
            -t|--timeout)
                TIMEOUT="$2"
                shift 2
                ;;
            --no-coverage)
                ENABLE_COVERAGE=false
                shift
                ;;
            all|unit|integration|e2e|performance|security|chaos|benchmarks|coverage|clean)
                command="$1"
                shift
                ;;
            *)
                print_error "Unknown option: $1"
                show_usage
                exit 1
                ;;
        esac
    done
    
    # Set up trap for cleanup
    trap cleanup_test_environment EXIT
    
    # Execute command
    case $command in
        all)
            setup_test_environment
            if run_all_tests; then
                generate_coverage_report
                exit 0
            else
                exit 1
            fi
            ;;
        unit)
            setup_test_environment
            run_test_suite "unit" "./internal/..."
            ;;
        integration)
            setup_test_environment
            run_test_suite "integration" "./tests/integration/..."
            ;;
        e2e)
            setup_test_environment
            run_test_suite "e2e" "./tests/e2e/..."
            ;;
        performance)
            setup_test_environment
            run_test_suite "performance" "./tests/performance/..."
            ;;
        security)
            setup_test_environment
            run_test_suite "security" "./tests/security/..."
            ;;
        chaos)
            setup_test_environment
            run_test_suite "chaos" "./tests/chaos/..."
            ;;
        benchmarks)
            setup_test_environment
            run_benchmarks
            ;;
        coverage)
            generate_coverage_report
            ;;
        clean)
            cleanup_test_environment
            ;;
        *)
            print_error "Unknown command: $command"
            show_usage
            exit 1
            ;;
    esac
}

# Run main function
main "$@"
