#!/usr/bin/env bash
set -euo pipefail
# Integration test for llmsh virtual mode semantics
# 1. Non-injected real path under --virtual is virtualized (empty)
# 2. Injected -i file is readable real
# 3. Internal unauthorized (simulate via spawned llmsh?) currently relies on unit tests; here we just validate top-level behavior.

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$ROOT_DIR/llmsh"
if [[ ! -x "$BIN" ]]; then
  # Fallback to legacy location if root build not present
  if [[ -x "$ROOT_DIR/bin/llmsh" ]]; then
    BIN="$ROOT_DIR/bin/llmsh"
  else
    echo "llmsh binary not found (looked in $ROOT_DIR/llmsh and $ROOT_DIR/bin/llmsh)" >&2
    exit 1
  fi
fi
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

REAL_FILE="$TMPDIR/real.txt"
INJECT_FILE="$TMPDIR/inject.txt"
OUT_FILE="$TMPDIR/out.txt"

echo 'SECRET DATA' > "$REAL_FILE"
echo 'HELLO WORLD' > "$INJECT_FILE"

# 1) Virtualization test: cat REAL_FILE should produce empty because not injected.
VIRT_OUTPUT=$("$BIN" --virtual -c "cat $REAL_FILE" 2>/dev/null || true)
if [[ -n "$VIRT_OUTPUT" ]]; then
  echo "[FAIL] Expected virtualized output empty, got: '$VIRT_OUTPUT'" >&2
  exit 1
fi
echo "[OK] Virtualized non-injected file produced empty output"

# 2) Injected input: -i INJECT_FILE cat inject.txt should output content.
INJECT_OUTPUT=$("$BIN" --virtual -i "$INJECT_FILE" -c "cat $INJECT_FILE")
if [[ "$INJECT_OUTPUT" != "HELLO WORLD" ]]; then
  echo "[FAIL] Expected injected file content, got: '$INJECT_OUTPUT'" >&2
  exit 1
fi

echo "[OK] Injected file content read successfully"

echo "All virtual mode integration tests passed."
