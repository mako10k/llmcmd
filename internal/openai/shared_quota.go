package openai

import (
	"fmt"
	"sync"
	"time"
)

// SharedQuotaManager handles quota sharing across concurrent llmcmd processes
type SharedQuotaManager struct {
	mu          sync.RWMutex
	config      *QuotaConfig
	globalUsage *QuotaUsage
	processMap  map[string]*ProcessQuotaInfo // process ID -> quota info
	created     time.Time
}

// ProcessQuotaInfo tracks quota usage for a specific process
type ProcessQuotaInfo struct {
	ProcessID  string
	ParentID   string // Parent process ID for inheritance
	StartTime  time.Time
	LocalUsage *QuotaUsage
	IsActive   bool
}

// NewSharedQuotaManager creates a new shared quota manager
func NewSharedQuotaManager(config *QuotaConfig) *SharedQuotaManager {
	return &SharedQuotaManager{
		config: config,
		globalUsage: &QuotaUsage{
			RemainingQuota: float64(config.MaxTokens),
		},
		processMap: make(map[string]*ProcessQuotaInfo),
		created:    time.Now(),
	}
}

// RegisterProcess registers a new process for quota tracking
func (sm *SharedQuotaManager) RegisterProcess(processID, parentID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.processMap[processID] = &ProcessQuotaInfo{
		ProcessID:  processID,
		ParentID:   parentID,
		StartTime:  time.Now(),
		LocalUsage: &QuotaUsage{},
		IsActive:   true,
	}

	return nil
}

// CanMakeCall checks if a process can make an API call without exceeding quota
func (sm *SharedQuotaManager) CanMakeCall(processID string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.globalUsage.RemainingQuota > 0
}

// ConsumeTokens updates quota usage for a specific process
func (sm *SharedQuotaManager) ConsumeTokens(processID string, usage *QuotaUsage) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	process, exists := sm.processMap[processID]
	if !exists {
		return fmt.Errorf("process %s not registered", processID)
	}

	// Update local process usage
	process.LocalUsage.InputTokens += usage.InputTokens
	process.LocalUsage.CachedTokens += usage.CachedTokens
	process.LocalUsage.OutputTokens += usage.OutputTokens

	// Calculate weighted tokens
	weightedInputs := float64(usage.InputTokens) * sm.config.InputWeight
	weightedCached := float64(usage.CachedTokens) * sm.config.CachedWeight
	weightedOutputs := float64(usage.OutputTokens) * sm.config.OutputWeight
	totalWeighted := weightedInputs + weightedCached + weightedOutputs

	process.LocalUsage.WeightedInputs += weightedInputs
	process.LocalUsage.WeightedCached += weightedCached
	process.LocalUsage.WeightedOutputs += weightedOutputs
	process.LocalUsage.TotalWeighted += totalWeighted

	// Update global usage
	sm.globalUsage.InputTokens += usage.InputTokens
	sm.globalUsage.CachedTokens += usage.CachedTokens
	sm.globalUsage.OutputTokens += usage.OutputTokens
	sm.globalUsage.WeightedInputs += weightedInputs
	sm.globalUsage.WeightedCached += weightedCached
	sm.globalUsage.WeightedOutputs += weightedOutputs
	sm.globalUsage.TotalWeighted += totalWeighted
	sm.globalUsage.RemainingQuota = float64(sm.config.MaxTokens) - sm.globalUsage.TotalWeighted

	return nil
}

// GetGlobalUsage returns current global quota usage (thread-safe)
func (sm *SharedQuotaManager) GetGlobalUsage() *QuotaUsage {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Return a copy to avoid race conditions
	usage := *sm.globalUsage
	return &usage
}

// GetProcessUsage returns quota usage for a specific process
func (sm *SharedQuotaManager) GetProcessUsage(processID string) (*QuotaUsage, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	process, exists := sm.processMap[processID]
	if !exists {
		return nil, fmt.Errorf("process %s not found", processID)
	}

	// Return a copy
	usage := *process.LocalUsage
	return &usage, nil
}

// UnregisterProcess removes a process from quota tracking
func (sm *SharedQuotaManager) UnregisterProcess(processID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	process, exists := sm.processMap[processID]
	if !exists {
		return fmt.Errorf("process %s not found", processID)
	}

	process.IsActive = false
	// Note: We keep the process in the map for historical tracking
	// In production, you might want to implement cleanup logic

	return nil
}

// GetActiveProcesses returns a list of currently active processes
func (sm *SharedQuotaManager) GetActiveProcesses() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var active []string
	for id, process := range sm.processMap {
		if process.IsActive {
			active = append(active, id)
		}
	}

	return active
}

// IsQuotaExceeded checks if the global quota has been exceeded
func (sm *SharedQuotaManager) IsQuotaExceeded() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.globalUsage.RemainingQuota <= 0
}
