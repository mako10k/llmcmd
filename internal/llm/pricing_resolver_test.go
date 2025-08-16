package llm

import (
    "path/filepath"
    "testing"
)

func TestResolveWeightsFromSampleCatalog(t *testing.T) {
    p := filepath.FromSlash("../../docs/pricing/model_pricing_catalog.sample.json")
    w, _, approx, err := ResolveWeights(p, "gpt-5")
    if err != nil { t.Fatalf("resolve err: %v", err) }
    if approx { t.Fatalf("expected exact weights for gpt-5") }
    if w.Input <= 0 || w.Output <= 0 { t.Fatalf("invalid weights: %+v", w) }
}
