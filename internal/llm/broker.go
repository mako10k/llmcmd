package llm

import (
    "context"
    "errors"
    "fmt"
    "sync"
    "time"
)

// Provider defines the minimal interface the broker needs to call an LLM backend.
type Provider interface {
    Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
}

// ChatRequest represents a logical chat request.
type ChatRequest struct {
    Messages    []Message
    Model       string
    MaxTokens   int
    Temperature float64
    Tools       any
    Metadata    map[string]any
}

// Message is a minimal message type.
type Message struct {
    Role    string
    Content string
}

// Usage tracks token usage reported by provider.
type Usage struct {
    PromptTokens    int
    CachedTokens    int
    CompletionTokens int
    TotalTokens     int
}

// ChatResponse is the provider response.
type ChatResponse struct {
    Messages []Message
    Usage    Usage
    Model    string
    Metadata map[string]any
}

// Weights expresses pricing weights normalized to "input:1.0" style.
type Weights struct {
    Input  float64
    Cached float64
    Output float64
}

// QuotaSnapshot is returned by QUOTA_GET.
type QuotaSnapshot struct {
    RemainingUnits float64
    Weights        Weights
    Window         string
    SessionID      string
    Currency       string // "USD"
    Unit           string // "per_1M_tokens"
}

// BrokerError is a structured error with a canonical code.
type BrokerError struct {
    Code    string
    Message string
}

func (e *BrokerError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

var (
    ErrQueueTimeout         = &BrokerError{Code: "queue_timeout", Message: "lock not acquired within max wait"}
    ErrCancelledBeforeStart = &BrokerError{Code: "cancelled_before_start", Message: "request cancelled while waiting"}
    ErrProviderTimeout      = &BrokerError{Code: "provider_timeout", Message: "provider call timed out"}
)

// BrokerConfig contains tunables for broker behavior.
type BrokerConfig struct {
    QueueMaxWait           time.Duration
    RequestTimeout         time.Duration
    SafetyFactor           float64
    InjectRemainingContext bool
    DefaultModel           string
}

// Broker serializes calls and tracks quota for a single parent session.
type Broker struct {
    prov Provider

    mu     sync.Mutex
    locked bool
    waitQ  []chan struct{}

    remainingUnits float64
    weights        Weights
    window         string
    sessionID      string

    cfg BrokerConfig
}

// NewBroker creates a broker with initial quota snapshot and configuration.
func NewBroker(prov Provider, initial QuotaSnapshot, cfg BrokerConfig) *Broker {
    if cfg.SafetyFactor <= 0 || cfg.SafetyFactor > 1 {
        cfg.SafetyFactor = 0.9
    }
    if cfg.QueueMaxWait <= 0 {
        cfg.QueueMaxWait = 30 * time.Second
    }
    if cfg.RequestTimeout <= 0 {
        cfg.RequestTimeout = 60 * time.Second
    }
    if cfg.DefaultModel == "" {
        cfg.DefaultModel = "gpt-4o-mini"
    }
    b := &Broker{
        prov:           prov,
        remainingUnits: initial.RemainingUnits,
        weights:        initial.Weights,
        window:         initial.Window,
        sessionID:      initial.SessionID,
        cfg:            cfg,
    }
    return b
}

// QuotaGet returns a snapshot of current quota.
func (b *Broker) QuotaGet() QuotaSnapshot {
    b.mu.Lock()
    defer b.mu.Unlock()
    return QuotaSnapshot{
        RemainingUnits: b.remainingUnits,
        Weights:        b.weights,
        Window:         b.window,
        SessionID:      b.sessionID,
        Currency:       "USD",
        Unit:           "per_1M_tokens",
    }
}

// Chat serializes provider calls, applies soft trim, and updates quota with actual usage.
func (b *Broker) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
    // Admission: acquire lock with FIFO semantics
    if err := b.acquire(ctx); err != nil {
        return ChatResponse{}, err
    }
    defer b.release()

    // Defaults
    if req.Model == "" {
        req.Model = b.cfg.DefaultModel
    }

    // Soft trim on max tokens
    effectiveMax, trimmed := b.effectiveMaxTokens(req.MaxTokens)
    if trimmed {
        if req.Metadata == nil {
            req.Metadata = map[string]any{}
        }
        req.Metadata["trim_applied"] = true
    }
    req.MaxTokens = effectiveMax

    // Inject remaining summary if enabled (metadata only; no content mutation here)
    if b.cfg.InjectRemainingContext {
        if req.Metadata == nil {
            req.Metadata = map[string]any{}
        }
        snap := b.QuotaGet()
        req.Metadata["remaining_units"] = snap.RemainingUnits
        req.Metadata["weights"] = snap.Weights
    }

    // Timeout for provider call
    callCtx, cancel := context.WithTimeout(ctx, b.cfg.RequestTimeout)
    defer cancel()

    resp, err := b.prov.Chat(callCtx, req)
    if err != nil {
        if errors.Is(err, context.DeadlineExceeded) || (errors.Is(err, context.Canceled) && callCtx.Err() == context.DeadlineExceeded) {
            return ChatResponse{}, ErrProviderTimeout
        }
        return ChatResponse{}, err
    }

    // Update quota with actual usage
    b.updateQuotaFromUsage(resp.Usage)
    // Bubble up trim marker in response metadata too
    if trimmed {
        if resp.Metadata == nil {
            resp.Metadata = map[string]any{}
        }
        resp.Metadata["trim_applied"] = true
    }
    return resp, nil
}

func (b *Broker) effectiveMaxTokens(requested int) (int, bool) {
    if requested <= 0 {
        // Nothing to trim; allow provider defaults
        return requested, false
    }
    b.mu.Lock()
    rem := b.remainingUnits
    w := b.weights
    sf := b.cfg.SafetyFactor
    b.mu.Unlock()
    if w.Output <= 0 {
        return requested, false
    }
    safeOut := int((rem / w.Output) * sf)
    if safeOut <= 0 {
        // Allow minimal completion
        if requested > 5 {
            return 5, true
        }
        return requested, true
    }
    if requested > safeOut {
        return safeOut, true
    }
    return requested, false
}

func (b *Broker) updateQuotaFromUsage(u Usage) {
    // Convert usage to remaining units decrement: input*Input + cached*Cached + output*Output
    delta := float64(u.PromptTokens)/1_000_000.0*b.weights.Input +
        float64(u.CachedTokens)/1_000_000.0*b.weights.Cached +
        float64(u.CompletionTokens)/1_000_000.0*b.weights.Output
    b.mu.Lock()
    b.remainingUnits -= delta
    if b.remainingUnits < 0 {
        b.remainingUnits = 0
    }
    b.mu.Unlock()
}

func (b *Broker) acquire(ctx context.Context) error {
    waiter := make(chan struct{}, 1)

    b.mu.Lock()
    if !b.locked {
        b.locked = true
        b.mu.Unlock()
        return nil
    }
    b.waitQ = append(b.waitQ, waiter)
    b.mu.Unlock()

    // Wait with timeout and cancellation
    maxWait := b.cfg.QueueMaxWait
    var timer *time.Timer
    if maxWait > 0 {
        timer = time.NewTimer(maxWait)
        defer timer.Stop()
    }
    select {
    case <-ctx.Done():
        // Remove from queue if still enqueued
        b.removeWaiter(waiter)
        return ErrCancelledBeforeStart
    case <-waiter:
        return nil
    case <-timerC(timer):
        b.removeWaiter(waiter)
        return ErrQueueTimeout
    }
}

func (b *Broker) release() {
    b.mu.Lock()
    defer b.mu.Unlock()
    if len(b.waitQ) == 0 {
        b.locked = false
        return
    }
    // FIFO: pop front and wake
    w := b.waitQ[0]
    copy(b.waitQ[0:], b.waitQ[1:])
    b.waitQ = b.waitQ[:len(b.waitQ)-1]
    // Keep locked=true for next holder; wake it
    w <- struct{}{}
}

func (b *Broker) removeWaiter(target chan struct{}) {
    b.mu.Lock()
    defer b.mu.Unlock()
    for i, w := range b.waitQ {
        if w == target {
            b.waitQ = append(b.waitQ[:i], b.waitQ[i+1:]...)
            break
        }
    }
}

func timerC(t *time.Timer) <-chan time.Time {
    if t == nil {
        return nil
    }
    return t.C
}
