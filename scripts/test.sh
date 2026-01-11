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
HTML_REPORT=false
RACE=false

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
        --html)
            HTML_REPORT=true
            ;;
        --race)
            RACE=true
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
            echo "  --html              Open HTML coverage report in browser"
            echo "  --race              Enable race detector"
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

if [ "$RACE" = true ]; then
    CMD="$CMD -race"
fi

if [ "$SKIP_COVERAGE" = false ]; then
    # Remove old coverage file to prevent stale data
    rm -f coverage.out
    # Use -count=1 to disable test caching (cache doesn't include coverage data)
    CMD="$CMD -count=1 -coverprofile=coverage.out -covermode=atomic -coverpkg=./..."
fi

TAGS="testcoverage"

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
    TAGS="$TAGS,integration"
    CMD="$CMD -timeout 10m"
fi

CMD="$CMD -tags=$TAGS"

CMD="$CMD ./..."

echo "Running: $CMD"
$CMD

if [ "$SKIP_COVERAGE" = false ]; then
    echo ""
    # Install go-ignore-cov if not available, then apply coverage:ignore comments
    GOBIN="$(go env GOPATH)/bin"
    if ! command -v go-ignore-cov &> /dev/null && [ ! -f "$GOBIN/go-ignore-cov" ]; then
        echo "Installing go-ignore-cov..."
        go install github.com/hexira/go-ignore-cov@latest
    fi
    "$GOBIN/go-ignore-cov" --file coverage.out --root "$PROJECT_DIR"
    echo "Coverage summary:"
    go tool cover -func=coverage.out | tail -1
    if [ "$HTML_REPORT" = true ]; then
        echo ""
        echo "Opening HTML coverage report..."
        go tool cover -html=coverage.out
    else
        echo ""
        echo "To view HTML report: go tool cover -html=coverage.out"
    fi
fi
