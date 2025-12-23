#!/bin/bash
# Run all tests including integration tests using .env file

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

# Load .env if it exists
if [ -f .env ]; then
    echo "Loading .env file..."
    set -a
    source .env
    set +a
fi

# Parse arguments
COVERAGE=false
INTEGRATION=false
VERBOSE=false

for arg in "$@"; do
    case $arg in
        --coverage)
            COVERAGE=true
            ;;
        --integration)
            INTEGRATION=true
            ;;
        -v|--verbose)
            VERBOSE=true
            ;;
        --help)
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  --coverage     Generate coverage report"
            echo "  --integration  Include integration tests"
            echo "  -v, --verbose  Verbose output"
            echo "  --help         Show this help"
            exit 0
            ;;
    esac
done

# Build test command
CMD="go test"

if [ "$VERBOSE" = true ]; then
    CMD="$CMD -v"
fi

if [ "$COVERAGE" = true ]; then
    CMD="$CMD -coverprofile=coverage.out -covermode=atomic -coverpkg=./..."
fi

if [ "$INTEGRATION" = true ]; then
    if [ -z "$VAULTSANDBOX_API_KEY" ]; then
        echo "Error: VAULTSANDBOX_API_KEY not set"
        echo "Create a .env file with VAULTSANDBOX_API_KEY and VAULTSANDBOX_URL"
        exit 1
    fi
    if [ -z "$VAULTSANDBOX_URL" ]; then
        echo "Error: VAULTSANDBOX_URL not set"
        echo "Create a .env file with VAULTSANDBOX_API_KEY and VAULTSANDBOX_URL"
        exit 1
    fi
    echo "Using API URL: $VAULTSANDBOX_URL"
    CMD="$CMD -tags=integration -timeout 10m"
fi

CMD="$CMD ./..."

echo "Running: $CMD"
$CMD

if [ "$COVERAGE" = true ]; then
    echo ""
    echo "Coverage summary:"
    go tool cover -func=coverage.out | tail -1
    echo ""
    echo "To view HTML report: go tool cover -html=coverage.out"
fi
