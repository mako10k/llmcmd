# LLM Broker & Quota Contract (Draft — Subject to Change)

Status: Changeable
Owner: llmcmd design thread
Date: 2025-08-16

## Goals
- Centralize quota at parent process scope
- Route all LLM calls through a parent broker channel
- No preflight denials; provide context of remaining and soft-trim max_tokens
- Account costs using actual API-reported usage
- Enforce serialized execution: API calls are not allowed to run in parallel

## Non-Goals
- vfsd involvement in LLM logic (I/O-only)
- Hard-stop throttling before requests (soft guidance only)

## API (logical)
- LLM_QUOTA_GET
  - Request: {}
  - Response: { remaining_units, weights: { input, cached, output }, window, session_id, currency: "USD", unit: "per_1M_tokens" }
- LLM_CHAT
  - Request: { messages, model, max_tokens?, temperature?, tools?, metadata? }
  - Behavior:
  - Protocol-level mutual exclusion: acquire broker lock before issuing API call; release on response/error/timeout
    - Inject remaining summary into system context (optional feature flag)
    - Compute soft limit for `max_tokens` based on remaining and output weight; trim if requested exceeds safe bound
  - Response: { message(s), usage: { prompt_tokens, cached_tokens?, completion_tokens, total_tokens }, model }
  - Accounting: update parent quota with actual usage

## Serialized Execution & Queueing
- Lock scope: per parent session (one in-flight call per session)
- Queueing policy: FIFO order for waiting requests
- Admission control:
  - If lock not available, request waits up to `queue.max_wait_seconds` (configurable)
  - If exceeded, respond with `queue_timeout` error (no API call issued)
- Cancellation:
  - Requests MAY carry `cancel_token`; if cancelled before lock acquisition → `cancelled_before_start`
  - If cancelled after lock acquisition → broker attempts provider-side cancel if supported; otherwise allow to complete
- Fairness: FIFO with no starvation; broker wakes exactly one waiter on release
- Telemetry: queue_length_at_enqueue, wait_duration_ms, cancelled_before_start

## Soft Limit Algorithm (proposal)
- Compute safe_out = floor((remaining_units / weight.output) * safety_factor)
- safety_factor default 0.9 (configurable)
- Trim: effective_max_tokens = min(request.max_tokens||default, safe_out)
- If effective_max_tokens <= 0: still allow minimal completion (e.g., 1-5 tokens) and warn via metadata

## Concurrency & Isolation
- Strict serialization: exactly one in-flight LLM API call per parent session (critical section spans call start → provider response or timeout)
- Thread-safe quota accounting
- Session propagation to children via `--vfs-fds`/env; broker resolves to parent session

## Error Handling
- Transport errors: surfaced to caller; no hidden retries by default
- API errors: include code/message; partially-consumed quota still recorded if reported
- Timeouts: configurable; partial results allowed if provider supports
  - Timeout behavior: lock is released; request marked as timed out; no parallel retry is auto-issued

### Error/Info Codes (canonical)
- queue_timeout: lock not acquired within queue.max_wait_seconds (no provider call issued)
- cancelled_before_start: request cancelled while waiting (no provider call issued)
- cancelled_after_start: cancellation requested after lock; provider cancel attempted if supported, else ignored
- provider_timeout: provider call exceeded request.timeout_seconds
- provider_error: provider returned an error (see code/message)
- trim_applied (info): requested max_tokens was reduced by broker soft limit

## Configuration (initial)
- queue.max_wait_seconds: default 30s
- request.timeout_seconds: default 60s
- safety_factor (soft trim): default 0.9
- inject_remaining_context: default false
- default_model: "gpt-4o-mini" (used when model unspecified)

## Observability
- Logs: model, latency, effective_max_tokens, actual_usage, trim_applied
- Metrics: requests, rate, tokens_in/out, soft_trim_count, denorm by model

### Telemetry Fields (per-request)
- model: string
- queue_length_at_enqueue: int
- wait_duration_ms: int
- effective_max_tokens: int
- provider_latency_ms: int
- usage: { prompt_tokens, cached_tokens?, completion_tokens, total_tokens }
- trim_applied: bool (info)
- error_code: one of canonical codes or empty on success

## Quota Accounting Formula

Broker decrements remaining units using provider-reported usage and configured weights:

delta_units = (prompt_tokens * weight.input + cached_tokens * weight.cached + completion_tokens * weight.output) / 1_000_000

remaining_units = max(0, remaining_units - delta_units)

This aligns accounting to the pricing catalog (unit: USD per 1M tokens) while staying region-agnostic.

## Open Questions
- How to represent multi-turn tool usage tokens? (aggregate into input/output by provider report)
- Should we expose a dry-run estimate endpoint? (stay aligned with "no preflight")

---

## Implementation Notes (Phase B wiring)

- FSProxy integration: Broker is feature-gated and opt-in to avoid regressions.
  - New fields in `FSProxyManager`: `useBroker bool`, `broker *llm.Broker`.
  - New method: `EnableLLMBroker(snapshot llm.QuotaSnapshot, cfg llm.BrokerConfig)` initializes a broker using the existing OpenAI client via an adapter.
  - When enabled, `LLM_CHAT` path calls `broker.Chat(...)`; otherwise it uses the legacy direct OpenAI path.
  - `LLM_QUOTA` prioritizes `broker.QuotaGet()`; falls back to shared quota manager or client stats when broker is disabled.
- Backward compatibility: Default behavior remains unchanged until `EnableLLMBroker(...)` is called.
- Response metadata: When soft trim is applied, `trim_applied=true` is attached in response metadata (provider-independent info channel).
- Pricing weights: Resolved via a local catalog (config-first). Region pricing is ignored. Default model is `gpt-4o-mini`.
- Error codes: Broker surfaces canonical codes defined above (e.g., `queue_timeout`, `provider_timeout`).

### Enabling the broker at runtime (example flow)
1) Resolve weights (pricing resolver) and construct an initial `QuotaSnapshot` for the session.
2) Build `BrokerConfig` (queue/timeouts/safety factor/default model).
3) Call `FSProxyManager.EnableLLMBroker(snapshot, cfg)` after `SetLLMClient(...)`.

This preserves existing behavior by default and allows safe, incremental rollout of serialized LLM execution and quota accounting.

### Protocol Compatibility Notes
- Until a structured quota endpoint is introduced in the external protocol, `LLM_QUOTA` keeps returning a human-readable string containing "weighted tokens" to maintain backward compatibility with existing consumers and tests.
- `trim_applied=true` is carried in response metadata (out-of-band of the textual content) to signal soft trimming without altering user-visible content.

### Minimal Example (LLM_CHAT via broker)
- Request (logical): `{ model: "gpt-4o-mini", messages: [{role:"system",...},{role:"user",...}], max_tokens: 2000, temperature: 0.7 }`
- Broker behavior: acquires lock → computes `effective_max_tokens` with soft limit → calls provider → updates quota using the formula above.
- Response (logical): `{ messages: [{role:"assistant", content:"..."}], usage:{prompt_tokens:10, completion_tokens:5, total_tokens:15}, model:"gpt-4o-mini", metadata:{trim_applied:true?} }`

#### Worked Example (soft trim)
- Given: `remaining_units=0.0100`, `weights={input:1.0, cached:0.25, output:4.0}`, `safety_factor=0.9`, `requested_max_tokens=2000`
- Compute: `safe_out = floor((0.0100 / 4.0) * 0.9 * 1_000_000?)`
  - Note: Units are normalized; implementation applies a consistent scale internally. Conceptually, `remaining_units / weight.output` bounds completion allowance which is then safety-trimmed.
- Result: `effective_max_tokens = min(requested_max_tokens, safe_out)`; if `effective_max_tokens < requested_max_tokens` then `trim_applied=true`.

## Configuration Keys (mapping)
- `queue.max_wait_seconds` (int, default 30)
- `request.timeout_seconds` (int, default 60)
- `safety_factor` (float, default 0.9)
- `inject_remaining_context` (bool, default false)
- `default_model` (string, default "gpt-4o-mini")

Source of truth: application configuration（config-first）; CLI/ENVの表面仕様は別章で定義。値が未設定の場合は上記デフォルトが適用される。

## Cancellation Semantics (details)
- Pre-lock: request cancelled before acquiring the broker lock → `cancelled_before_start`; no provider call.
- Post-lock: cancelled after lock acquisition → provider-side cancel attempted if supported; otherwise call completes.
- Timeout precedence: request-level timeout yields `provider_timeout`; if cancellation races with timeout, whichever the broker observes first wins.
- Accounting: only provider-reported usage is deducted; none for pre-lock cancellation or queue timeout.

## Session Lifecycle
- One broker per parent session; exactly one in-flight call per session.
- Init: app supplies initial `QuotaSnapshot` (remaining_units, weights, window, session_id) — weights resolved by Pricing Resolver.
- Propagation: children share parent session; serialization enforced at parent.
- Snapshot: `QuotaGet()` returns current remaining and weights; remaining decreases on actual usage.

## Limits / Future Work
- Max queue length policy (drop/immediate error) — deferred; currently FIFO + timeout.
- Structured external quota schema — future; human-readable kept for backward compatibility.
- Region-aware pricing — out-of-scope for v3.x; may be added to resolver later.

## References
- Pricing Resolver Contract: `docs/design/model_pricing_resolver_contract.md`
- Sample Pricing Catalog: `docs/pricing/model_pricing_catalog.sample.json`

## Retry & Backoff Policy
- Internal broker: no implicit retries; all retries are caller-driven to preserve transparency and cost control.
- Suggested caller policy:
  - `queue_timeout`: backoff + retry (jitter), or surface to user with wait option.
  - `cancelled_before_start`: retry if the user still intends to proceed.
  - `provider_timeout`: small bounded retries with increasing timeout only if provider SLO allows; otherwise surface.
  - `provider_error`: do not blind-retry; honor provider error class (auth/limit/config errors are non-retriable).

## Caller Contract (summary)
- Inputs: `{messages[], model?, max_tokens?, temperature?, tools?, metadata?}`
- Outputs: `{messages[], usage{}, model, metadata{trim_applied?}}` or structured error with canonical `code`.
- Success criteria: provider call completed; usage accounted; lock released.
- Error modes: `queue_timeout`, `cancelled_before_start`, `provider_timeout`, `provider_error`.
- Retriability guidance:
  - Retriable: `queue_timeout` (yes), `cancelled_before_start` (caller decision), `provider_timeout` (situational).
  - Non-retriable: auth/config/limit classes of `provider_error` unless conditions change.

## Testing Strategy (summary)
- Unit tests:
  - Serialization: ensure no concurrent provider invocations under contention.
  - Soft trim: requested > safe bound → `trim_applied` set and `effective_max_tokens` reduced.
  - Error propagation: queue timeout, cancel before start, provider timeout map to canonical codes.
- Integration tests:
  - FSProxy `LLM_CHAT` routes through broker when enabled; legacy path otherwise。
  - `LLM_QUOTA` reflects broker snapshot and maintains backward-compatible string format.

## Rollout & Feature Flag
- Default: broker disabled; legacy OpenAI direct path remains.
- Enable per-session via application-level call `EnableLLMBroker(snapshot, cfg)` after `SetLLMClient(...)`。
- Config gating: use app config to toggle broker on/off and to set `BrokerConfig` fields; safe to roll back by disabling.
