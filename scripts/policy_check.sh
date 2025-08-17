#!/usr/bin/env bash
set -euo pipefail

fail() { echo "[POLICY] FAIL: $*" >&2; exit 1; }
warn() { echo "[POLICY] WARN: $*" >&2; }

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")"/.. && pwd)"
cd "$ROOT_DIR"

# 1) Forbid external command execution in llmsh-rs except approved cases
#    Allowed: execvp only in llmsh-rs/src/mux.rs::spawn_upstream (to launch vfsd)
#    Allowed: Command::new only for nested llmsh in run_builtin_llmsh

exec_hits=$(grep -RInE "\bexecvp\(|std::process::Command::new\(" llmsh-rs/src || true)
if [[ -n "$exec_hits" ]]; then
  # Filter allowed usages
  disallowed=$(echo "$exec_hits" \
    | grep -vE "llmsh-rs/src/mux\.rs:.*execvp\(" \
    | grep -vE "llmsh-rs/src/main\.rs:.*Command::new\(\&?bin\)" \
    | grep -vE "^$" || true)
  if [[ -n "$disallowed" ]]; then
    echo "$disallowed" >&2
    fail "External execution primitives found outside allowed locations"
  fi
fi

# 2) Guard direct filesystem access in llmsh-rs (std::fs). Temporary allowlist for known safe probes.
fs_hits=$(grep -RInE "std::fs::(File::open|OpenOptions::new|read_to_string|metadata)" llmsh-rs/src || true)
if [[ -n "$fs_hits" ]]; then
  disallowed=$(echo "$fs_hits" \
    | grep -vE "llmsh-rs/src/main\.rs:.*metadata\(|llmsh-rs/src/main\.rs:.*read_to_string\(" \
    | grep -vE "llmsh-rs/src/mux\.rs:.*from_raw_fd\(" \
    | grep -vE "^$" || true)
  if [[ -n "$disallowed" ]]; then
    echo "$disallowed" >&2
    fail "Direct filesystem access detected (not in allowlist)"
  fi
fi

echo "[POLICY] OK: checks passed"
