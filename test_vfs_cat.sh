#!/usr/bin/env bash
set -euo pipefail

# Paths
LLMSH=llmsh-rs/target/debug/llmsh-rs
VFSD=vfsd/target/debug/vfsd

if [[ ! -x "$LLMSH" ]]; then
  echo "Build llmsh binary first (cargo build in llmsh-rs)" >&2
  exit 1
fi
if [[ ! -x "$VFSD" ]]; then
  echo "Build vfsd binary first (cargo build in vfsd)" >&2
  exit 1
fi

export LLMSH_VFSD_BIN="$VFSD"
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

# Prepare files
ALLOWED_IN="$TMPDIR/allowed.txt"
DENIED_IN="$TMPDIR/denied.txt"
ALLOWED_OUT="$TMPDIR/out.txt"
ALLOWED_APPEND="$TMPDIR/append.txt"

printf 'hello world\nsecond line\n' > "$ALLOWED_IN"
printf 'secret\n' > "$DENIED_IN"
: > "$ALLOWED_OUT"
: > "$ALLOWED_APPEND"

pass=0; fail=0
report(){ printf '%s\n' "$1"; }
case_ok(){ ((pass++)) || true; report "[OK ] $1"; }
case_ng(){ ((fail++)) || true; report "[NG ] $1"; }

run_case(){ desc="$1" shift || true; if "$@"; then case_ok "$desc"; else case_ng "$desc"; fi }

# 1. Simple cat allowed file -> stdout
OUT1=$("$LLMSH" -i "$ALLOWED_IN" -o "$ALLOWED_OUT" -c "cat $ALLOWED_IN" 2>&1 | tr -d '\r') || true
if grep -q 'hello world' <<<"$OUT1" && ! grep -q 'Error' <<<"$OUT1"; then case_ok 'cat allowed -> stdout'; else echo "OUT:$OUT1"; case_ng 'cat allowed -> stdout'; fi

# 2. Cat denied file (not in -i) should error
OUT2=$("$LLMSH" -i "$ALLOWED_IN" -o "$ALLOWED_OUT" -c "cat $DENIED_IN" 2>&1 || true)
if grep -qi 'open_read' <<<"$OUT2" || grep -qi 'error' <<<"$OUT2"; then case_ok 'cat denied -> error'; else echo "OUT:$OUT2"; case_ng 'cat denied -> error'; fi

# 3. Redirect output to allowed out file
"$LLMSH" -i "$ALLOWED_IN" -o "$ALLOWED_OUT" -c "cat $ALLOWED_IN > $ALLOWED_OUT" >/dev/null 2>&1 || true
if grep -q 'hello world' "$ALLOWED_OUT"; then case_ok 'redirect write allowed'; else case_ng 'redirect write allowed'; fi

# 4. Append mode
printf 'BASE\n' > "$ALLOWED_APPEND"
"$LLMSH" -i "$ALLOWED_IN" -o "$ALLOWED_APPEND" -c "cat $ALLOWED_IN >> $ALLOWED_APPEND" >/dev/null 2>&1 || true
if grep -q 'BASE' "$ALLOWED_APPEND" && grep -q 'second line' "$ALLOWED_APPEND"; then case_ok 'append write allowed'; else case_ng 'append write allowed'; fi

# 5. Input redirection < (should be ignored if args present) -> use arg content
OUT5=$("$LLMSH" -i "$ALLOWED_IN" -c "cat $ALLOWED_IN < $DENIED_IN" 2>&1 | head -n1)
if grep -q 'hello world' <<<"$OUT5"; then case_ok 'ignore < when args present'; else echo "OUT:$OUT5"; case_ng 'ignore < when args present'; fi

# 6. No args but < redir should open file
OUT6=$("$LLMSH" -i "$ALLOWED_IN" -c "cat < $ALLOWED_IN" 2>&1 | head -n1)
if grep -q 'hello world' <<<"$OUT6"; then case_ok 'cat via < redir no args'; else echo "OUT:$OUT6"; case_ng 'cat via < redir no args'; fi

# Summary
echo "---- SUMMARY ----"; echo "PASS: $pass"; echo "FAIL: $fail"; test $fail -eq 0
