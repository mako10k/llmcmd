#!/bin/bash

# Integration Test Suite Runner
# Executes comprehensive tests for both llmcmd and llmsh

set -euo pipefail

readonly SCRIPT_DIR="$(dirname "$0")"
readonly PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Source test environment
source "$SCRIPT_DIR/test.env"

# Run the main test framework
exec "$SCRIPT_DIR/framework/test_runner.sh" "$@"
