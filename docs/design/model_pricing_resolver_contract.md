# Model Pricing Resolver Contract (Draft — Subject to Change)

Status: Changeable
Owner: llmcmd design thread
Date: 2025-08-16

## Purpose
Resolve model-dependent pricing weights (input, cached, output) at runtime to align quota accounting with current official pricing.

## Inputs
- model_id (string, e.g., "gpt-4o", "gpt-5", vendor-qualified if needed)
- options: { prefer_offline: bool, allow_online_refresh: bool, now: timestamp }

## Outputs
- weights: { input: float, cached: float, output: float }
- source: enum { config, cache, online }
- version: string (catalog version or timestamp)

## Resolution Order
1) Config-first: local config (shipped JSON/YAML) mapping known models → weights
2) Cached catalog: previously fetched and stored mapping with TTL
3) Optional online refresh: fetch official sources, parse, update cache
4) Defaulting policy: when unspecified, use "gpt-4o-mini" weights for both top-level default and any unspecified subcomponents

Config-first example file: `docs/pricing/model_pricing_catalog.sample.json` (fields: version, currency, unit, models, defaults)

## Caching & TTL
- Persistent on-disk cache with versioning
- TTL default: 24h (configurable)
- Staleness policy: prefer stale cache over online when prefer_offline=true

## Offline/Test Safety
- Always return a weight for known models from config
- For unknown models: fallback to a conservative default and flag as "approx"
- Tests: use fixed fixture catalog; disable network
 - Regional pricing: ignored (region-agnostic pricing only)

## Defaults
- default_model: "gpt-4o-mini" for top-level and subcomponent defaults
- Unknown models use the default_model weights with source="default" and flag `approx=true`

## Sample Config Snippet (illustrative)
```json
{
  "default_model": "gpt-4o-mini",
  "weights": {
    "gpt-4o-mini": { "input": 1.0, "cached": 0.25, "output": 4.0 },
    "gpt-4o": { "input": 1.0, "cached": 0.25, "output": 4.0 }
  }
}
```

## Observability
- Log source, model_id, weights, latency
- Metrics: cache_hit, cache_miss, refresh_errors

## Open Questions
- Multi-region pricing differences? (future: region-aware catalogs)
- Currency normalization? (store normalized USD per 1M tokens; weights derived from that)

## Example (structure only)
```json
{
  "gpt-4o": { "input": 1.0, "cached": 0.25, "output": 4.0 },
  "gpt-5":  { "input": 1.2, "cached": 0.3,  "output": 5.0 }
}
```
(Note: Values are illustrative placeholders, not actual pricing.)
