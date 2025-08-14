# vfsd API Design (Language-Neutral Draft)

Priority: Implement vfsd first. Language is incidental (currently Rust). llmsh Rust rewrite comes after vfsd baseline.

## Goals
1. Provide a constrained virtual FS + pipe abstraction via a single parent<->child IPC channel.
2. Minimize complexity: single client, no multi-session routing.
3. Fail-first: invalid inputs produce immediate error responses; no silent fallbacks.
4. Streaming-friendly: reads and writes are explicit; no hidden buffering beyond OS.

## Framing
Each message:
```
uint32 (big-endian length of JSON payload)
JSON bytes (UTF-8)
```

## Request Schema
```jsonc
{
  "id": "unique client-generated string",
  "op": "ping|init|open_read|open_write|read|write|close|stat|list|make_pipe|temp",
  "version": 1,              // optional future bump
  "params": { /* op-specific */ }
}
```

### Operations (Phase 0 → Phase 1 Scope)
| Op | Phase | Description |
|----|-------|-------------|
| ping | 0 | Liveness check |
| init | 0 | Initial handshake (allowlist, config) |
| open_read | 1 | Open existing file for reading |
| open_write | 1 | Create/truncate file for writing |
| read | 1 | Read from handle |
| write | 1 | Write to handle |
| close | 1 | Close handle |
| stat | 2 | Basic metadata (size, mode) |
| list | 2 | Directory listing (filtered) |
| make_pipe | 3 | Create uni-directional pipe pair (r,w handles) |
| temp | 3 | Create temp file (auto-removed on process exit) |

### Params Details (Phase 1 Set)

open_read:
```jsonc
{"path": "relative/or/absolute"}
```
open_write:
```jsonc
{"path": "relative/or/absolute", "append": false}
```
read:
```jsonc
{"h": 3, "max": 4096} // max > internal_limit => clamp (E_RANGE?) or truncate silently → policy: truncate w/out error
```
write:
```jsonc
{"h": 4, "data": "base64..."}
```
close:
```jsonc
{"h": 4}
```

### Response Schema
```jsonc
{
  "id": "same as request",
  "ok": true,
  "result": { /* op-specific */ }
}
// or on error
{
  "id": "same",
  "ok": false,
  "error": {"code": "E_NOENT", "message": "not found"}
}
```

### Result Payloads (Phase 1)
open_read / open_write:
```jsonc
{"handle": 5}
```
read:
```jsonc
{"eof": false, "data": "base64..."}
```
write:
```jsonc
{"written": 123}
```
close:
```jsonc
{"closed": true}
```
ping:
```jsonc
{"pong": true}
```
init:
```jsonc
{"status": "ready"}
```

## Error Codes (Initial Set)
| Code | Meaning | Typical Cause |
|------|---------|---------------|
| E_ARG | Bad/missing parameter | malformed params |
| E_UNSUPPORTED | Unknown op | typo / not implemented |
| E_NOENT | Path not found | open_read/open_write (read mode) |
| E_PERM | Permission denied | allowlist / fs perms |
| E_IO | Underlying IO error | read/write syscall error |
| E_CLOSED | Handle already closed/invalid | read/write/close after close |
| E_RANGE | Parameter out of allowed bounds | max too large (if we decide to signal) |
| E_LIMIT | Resource/quota exceeded | optional future quota |

Policy: For read size exceeding internal clamp we do NOT raise E_RANGE initially; we just clamp.

## Handle Allocation
Sequential u32 starting at 1. Distinct namespace for files vs pipes NOT required—single map h -> HandleEntry { kind, file/pipe }.

## Concurrency Model
Single-threaded loop over stdin → deterministic ordering. Future: could spawn a thread pool for IO heavy ops; not needed now.

## Base64 Rationale
Binary-safe without adding secondary frames. Cost acceptable for small chunk sizes (<=4KB).

## Security / Allowlist (Init)
init params example:
```jsonc
{"allow":["/home/user/project/data","./logs"], "read_only":false}
```
Phase 1: store allowlist but only enforce prefix check on open_*.

## Future Extensions
stat/list/make_pipe/temp/quota_status reserved. Client can probe support by attempting op; E_UNSUPPORTED returned if not ready.

---
Implementation next steps:
1. Extend Request.op match with new ops returning E_UNSUPPORTED placeholders.
2. Introduce handle table (Vec<Option<HandleEntry>> or HashMap<u32, HandleEntry>).
3. Implement open_read/open_write + read/write/close.
4. Enforce allowlist on open_*.
5. Add simple chunk clamp constant MAX_READ (e.g. 4096).
6. Integrate base64 encode/decode (use base64 crate).
