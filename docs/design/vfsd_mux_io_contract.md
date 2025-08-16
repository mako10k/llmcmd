# vfsd + Mux I/O Contract (Draft — Subject to Change)

Status: Changeable
Owner: llmcmd design thread
Date: 2025-08-16

## Scope
Parent process manages I/O through a mux channel to a sandboxed vfsd. Children (e.g., llmsh) inherit access via `--vfs-fds`. vfsd provides I/O-only primitives.

## Non-Goals
- No LLM logic in vfsd
- No shell expansion or environment variable interpretation
- No direct filesystem access beyond allowlisted paths and virtual files
- No directory semantics (VFS has no concept of directories; listing is unsupported)

## Interfaces (high-level)
- Startup
  - Parent launches vfsd with input/output allowlists and 4KB read cap.
  - Parent establishes mux channels; records fd mapping.
  - Children receive `--vfs-fds` to bind to parent-controlled I/O.
- Operations (per vfsd protocol)
  - open(path, flags, mode?) → handle_id
    - flags: bitmask of { RDONLY, WRONLY, RDWR, CREAT, TRUNC, APPEND }
    - mode: optional file mode when CREAT is set (octal)
  - read(handle_id, max_bytes<=4096) → bytes
  - write(handle_id, bytes) → n_written
  - seek(handle_id, offset, whence in {SEEK_SET, SEEK_CUR, SEEK_END}) → new_offset
  - stat(handle_id) → { size (int64), mtime (unix_sec), atime? (unix_sec), ctime? (unix_sec) } (path-based stat optional)
  - close(handle_id) → ok
  - create_temp(prefix?) → handle_id (virtual file, tempfile-backed)
- Message framing
  - Stdio-framed JSON-RPC with request_id; see vfsd server protocol documentation and `vfsd/src/main.rs`.

## Security Invariants
- Enforce allowlists for readable/writable paths
- Read cap: ≤ 4096 bytes per read call
- vfsd remains I/O-only (no process spawn, no network, no env access)
- Virtual files are ephemeral and tracked by parent session

## Error Modes
- EINVAL: invalid parameters (negative sizes, unknown handle_id)
- EACCES: disallowed path per allowlist
- EIO: underlying OS errors surfaced with sanitized messages
- EOF: read past end-of-file returns empty payload
- ENOTSUP: any directory-related operation or unsupported flag

## Edge Cases
- Large files: require multiple reads respecting 4KB cap
- Concurrent access: per-handle serialization in vfsd; parent ensures ordering if needed
- Handle leaks: parent tracks handle lifecycle; close on session exit
- Path traversal: reject `..` outside allowlists
- Directories: unsupported; any directory operation returns ENOTSUP

## Success Criteria
- All fs reads/writes in parent and children flow through mux→vfsd
- No deadlocks on stdio; pipes closed deterministically
- Allowlist violations fail early and visibly
- Performance: streaming reads/writes viable for ≥10MB files via chunking

## Observability
- Structured logs with request_id and timing
- Counters: open/read/write/close, bytes_in/out, allowlist rejects

## Open Questions
- None for stat/seek: supported as above
- Directory listing: Not supported by design (no directory concept)
