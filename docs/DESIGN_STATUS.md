# Design Status (Draft — Subject to Change)

Date: 2025-08-16
Owner: llmcmd design thread
Status: Changeable (no API freeze; implementation intentionally deferred)

This document tracks areas currently under active design. Interfaces and behavior are subject to change until explicitly approved.

## vfsd Integration & Mux-based I/O
- Status: Changeable (Draft)
- Intent: Parent process launches vfsd + mux on startup; all file read/write go through mux; spawn runs llmsh as a child with stdio pipes.
- Security: Input/output allowlists; virtual files; 4KB read cap; vfsd remains I/O-only.
- Implementation: Deferred until specification approval.

## LLM Broker & Quota (Parent-scoped)
- Status: Changeable (Draft)
- Principles:
  - Quota is scoped to parent; children inherit via `--vfs-fds`.
  - All LLM calls go through a parent-side broker channel; vfsd is not involved in LLM logic.
  - No preflight denials: broker injects remaining quota into context and trims `max_tokens` softly.
  - Accounting uses actual usage from API responses.
- Implementation: Deferred until specification approval.

## Dynamic Model Pricing Weights
- Status: Changeable (Draft)
- Goal: Compute model-dependent weights (input/cached/output) aligned to current official pricing (incl. GPT-5 when available).
- Strategy: Config-first with periodic online refresh and caching; safe fallbacks for offline/test runs; no external calls in tests.
- Implementation: Deferred until specification approval.

---
Notes: This page serves as a living status indicator, not a finalized spec. When design stabilizes, add finalized interfaces to the appropriate spec docs and remove the "Changeable" label.
