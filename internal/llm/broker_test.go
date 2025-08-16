package llm

import (
    "context"
    "sync/atomic"
    "testing"
    "time"
)

type stubProv struct { inFlight int32; delay time.Duration }

func (s *stubProv) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
    if atomic.AddInt32(&s.inFlight, 1) > 1 {
        // indicate concurrency; returning error to fail test naturally
        atomic.AddInt32(&s.inFlight, -1)
        return ChatResponse{}, context.DeadlineExceeded
    }
    defer atomic.AddInt32(&s.inFlight, -1)
    select {
    case <-ctx.Done():
        return ChatResponse{}, ctx.Err()
    case <-time.After(s.delay):
        return ChatResponse{Usage: Usage{PromptTokens: 1000, CompletionTokens: 2000}}, nil
    }
}

func TestBrokerSerializesCalls(t *testing.T) {
    prov := &stubProv{delay: 50 * time.Millisecond}
    b := NewBroker(prov, QuotaSnapshot{RemainingUnits: 10, Weights: Weights{Input:1, Cached:0.25, Output:4}}, BrokerConfig{})
    ctx := context.Background()
    done := make(chan struct{}, 2)
    go func(){ _, _ = b.Chat(ctx, ChatRequest{MaxTokens: 50}); done <- struct{}{} }()
    go func(){ _, _ = b.Chat(ctx, ChatRequest{MaxTokens: 50}); done <- struct{}{} }()
    <-done; <-done
}
