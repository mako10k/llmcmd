package llm

import (
    "encoding/json"
    "errors"
    "fmt"
    "os"
)

type PricingWeights struct {
    Input  float64 `json:"input"`
    Cached float64 `json:"cached"`
    Output float64 `json:"output"`
}

type pricingCatalog struct {
    Version  string                 `json:"version"`
    Currency string                 `json:"currency"`
    Unit     string                 `json:"unit"`
    Models   map[string]any         `json:"models"`
    Defaults struct {
        DefaultModel  string `json:"default_model"`
        RegionPricing string `json:"region_pricing"`
    } `json:"defaults"`
}

// ResolveWeights reads a local JSON catalog and resolves weights for a model id.
// Unknown models fall back to default_model weights with approx=true.
func ResolveWeights(catalogPath, model string) (Weights, string, bool, error) {
    b, err := os.ReadFile(catalogPath)
    if err != nil {
        return Weights{}, "", false, fmt.Errorf("read catalog: %w", err)
    }
    var cat pricingCatalog
    if err := json.Unmarshal(b, &cat); err != nil {
        return Weights{}, "", false, fmt.Errorf("parse catalog: %w", err)
    }
    if cat.Models == nil {
        return Weights{}, cat.Version, false, errors.New("catalog missing models")
    }
    if model == "" {
        model = cat.Defaults.DefaultModel
    }
    if w, ok := extractModelWeights(cat.Models, model); ok {
        return w, cat.Version, false, nil
    }
    // fallback to default model
    if w, ok := extractModelWeights(cat.Models, cat.Defaults.DefaultModel); ok {
        return w, cat.Version, true, nil
    }
    // conservative fallback
    return Weights{Input: 1.0, Cached: 0.25, Output: 4.0}, cat.Version, true, nil
}

func extractModelWeights(models map[string]any, model string) (Weights, bool) {
    raw, ok := models[model]
    if !ok {
        return Weights{}, false
    }
    // Try direct weights shape {input,cached,output}
    if m, ok := raw.(map[string]any); ok {
        // direct pricing
        if iw, ok := maybeWeights(m); ok {
            return iw, true
        }
        // nested (e.g., finetune)
        for _, v := range m {
            if sub, ok := v.(map[string]any); ok {
                if iw, ok := maybeWeights(sub); ok {
                    return iw, true
                }
            }
        }
    }
    return Weights{}, false
}

func maybeWeights(m map[string]any) (Weights, bool) {
    in, iok := num(m["input"]) 
    ca, cok := num(m["cached"]) 
    out, ook := num(m["output"]) 
    if iok && cok && ook {
        return Weights{Input: in, Cached: ca, Output: out}, true
    }
    return Weights{}, false
}

func num(v any) (float64, bool) {
    switch t := v.(type) {
    case float64:
        return t, true
    case float32:
        return float64(t), true
    case int:
        return float64(t), true
    case int64:
        return float64(t), true
    case json.Number:
        f, err := t.Float64()
        if err == nil { return f, true }
        return 0, false
    default:
        return 0, false
    }
}
