#!/bin/bash

# Test: llmsh --virtual real file access denial
# Validates that virtual mode denies access to real filesystem files

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_NAME="real_file_denied"
source "$SCRIPT_DIR/../../../framework/fsproxy_helpers.sh"

main() {
    echo "=== Testing llmsh --virtual real file access denial ==="
    
    # Setup test environment
    setup_fsproxy_test_env "$TEST_NAME" "llmsh-virtual"
    
    # Check if llmsh binary exists
    local llmsh_binary="$SCRIPT_DIR/../../../../bin/llmsh"
    check_binary "$llmsh_binary"
    
    # Test access to system files (should be denied)
    echo "Testing denial of access to /etc/passwd..."
    if "$llmsh_binary" --virtual "cat /etc/passwd" >/dev/null 2>&1; then
        echo "ERROR: Virtual mode allowed access to /etc/passwd"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ Access to /etc/passwd correctly denied"
    
    # Test access to /proc filesystem (should be denied)
    echo "Testing denial of access to /proc/version..."
    if "$llmsh_binary" --virtual "cat /proc/version" >/dev/null 2>&1; then
        echo "ERROR: Virtual mode allowed access to /proc/version"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ Access to /proc filesystem correctly denied"
    
    # Test access to /sys filesystem (should be denied)
    echo "Testing denial of access to /sys/..."
    if "$llmsh_binary" --virtual "ls /sys" >/dev/null 2>&1; then
        echo "ERROR: Virtual mode allowed access to /sys"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ Access to /sys filesystem correctly denied"
    
    # Test access to /dev devices (should be denied)
    echo "Testing denial of access to /dev/null..."
    if "$llmsh_binary" --virtual "echo test > /dev/null" >/dev/null 2>&1; then
        echo "ERROR: Virtual mode allowed write to /dev/null"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ Access to /dev devices correctly denied"
    
    # Test access to home directory files (should be denied unless explicitly allowed)
    echo "Testing denial of access to home directory..."
    if "$llmsh_binary" --virtual "ls ~" >/dev/null 2>&1; then
        echo "ERROR: Virtual mode allowed access to home directory"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ Access to home directory correctly denied"
    
    # Test access to /tmp (should be denied for security)
    echo "Testing denial of access to /tmp..."
    if "$llmsh_binary" --virtual "ls /tmp" >/dev/null 2>&1; then
        echo "ERROR: Virtual mode allowed access to /tmp"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ Access to /tmp correctly denied"
    
    # Test directory traversal attempts
    echo "Testing directory traversal attempts..."
    if "$llmsh_binary" --virtual "cat ../../../etc/passwd" >/dev/null 2>&1; then
        echo "ERROR: Virtual mode allowed directory traversal"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ Directory traversal attempts correctly blocked"
    
    # Test absolute path attempts
    echo "Testing absolute path bypass attempts..."
    if "$llmsh_binary" --virtual "cat /bin/sh" >/dev/null 2>&1; then
        echo "ERROR: Virtual mode allowed access to /bin/sh"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ Absolute path bypass attempts correctly blocked"
    
    # Test environment variable access attempts
    echo "Testing environment variable access..."
    local env_result
    env_result=$("$llmsh_binary" --virtual "echo \$HOME" 2>&1) || true
    
    if echo "$env_result" | grep -q "/home/"; then
        echo "ERROR: Virtual mode exposed HOME environment variable"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ Environment variable access correctly restricted"
    
    # Test that only explicit input/output files are accessible
    echo "Testing that only explicit files are accessible..."
    
    # Create a file that's not explicitly specified
    local unspecified_file="/tmp/unspecified_test_file.txt"
    echo "secret content" > "$unspecified_file"
    
    if "$llmsh_binary" --virtual "cat $unspecified_file" >/dev/null 2>&1; then
        echo "ERROR: Virtual mode allowed access to unspecified file"
        rm -f "$unspecified_file"
        cleanup_test_env
        exit 1
    fi
    
    rm -f "$unspecified_file"
    echo "✓ Access to unspecified files correctly denied"
    
    # Test network access restrictions (if applicable)
    echo "Testing network access restrictions..."
    if "$llmsh_binary" --virtual "curl --connect-timeout 1 http://example.com" >/dev/null 2>&1; then
        echo "⚠ Virtual mode allowed network access (may be expected)"
    else
        echo "✓ Network access correctly restricted"
    fi
    
    # Test process execution restrictions
    echo "Testing process execution restrictions..."
    if "$llmsh_binary" --virtual "ps aux" >/dev/null 2>&1; then
        echo "⚠ Virtual mode allowed process listing (may be expected)"
    else
        echo "✓ Process execution correctly restricted"
    fi
    
    # Test file creation outside allowed areas
    echo "Testing file creation restrictions..."
    if "$llmsh_binary" --virtual "touch /tmp/virtual_test_file" >/dev/null 2>&1; then
        echo "ERROR: Virtual mode allowed file creation in /tmp"
        cleanup_test_env
        exit 1
    fi
    
    echo "✓ File creation outside allowed areas correctly restricted"
    
    # Cleanup
    cleanup_test_env
    
    echo "=== real_file_denied test PASSED ==="
}

cleanup_test_env() {
    if [[ -n "${TEST_ENV_DIR:-}" && -d "$TEST_ENV_DIR" ]]; then
        rm -rf "$TEST_ENV_DIR"
    fi
}

# Trap cleanup on exit
trap cleanup_test_env EXIT

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
