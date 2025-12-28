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

# Default: run everything
SKIP_UNIT=false
SKIP_INTEGRATION=false
SKIP_COVERAGE=false
VERBOSE=false

for arg in "$@"; do
    case $arg in
        --skip-unit)
            SKIP_UNIT=true
            ;;
        --skip-integration)
            SKIP_INTEGRATION=true
            ;;
        --skip-coverage)
            SKIP_COVERAGE=true
            ;;
        -v|--verbose)
            VERBOSE=true
            ;;
        --help)
            echo "Usage: $0 [options]"
            echo ""
            echo "By default, runs all tests (unit + integration) with coverage."
            echo ""
            echo "Options:"
            echo "  --skip-unit         Skip unit tests"
            echo "  --skip-integration  Skip integration tests"
            echo "  --skip-coverage     Skip coverage collection"
            echo "  -v, --verbose       Verbose output"
            echo "  --help              Show this help"
            exit 0
            ;;
    esac
done

# Build test command
CMD="go test"

if [ "$VERBOSE" = true ]; then
    CMD="$CMD -v"
fi

if [ "$SKIP_COVERAGE" = false ]; then
    CMD="$CMD -coverprofile=coverage.out -covermode=atomic -coverpkg=./..."
fi

if [ "$SKIP_INTEGRATION" = false ]; then
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

if [ "$SKIP_COVERAGE" = false ]; then
    echo ""
    echo "Coverage summary:"
    go tool cover -func=coverage.out | tail -1
    echo ""
    echo "To view HTML report: go tool cover -html=coverage.out"
fi
