package security

import (
	"fmt"
	"sync"
)

// AuditManager manages audit logging across the application
type AuditManager struct {
	logger    AuditLogger
	sessionID string // Session identifier for this application run
	mutex     sync.RWMutex
}

var (
	globalAuditManager *AuditManager
	managerOnce        sync.Once
)

// InitGlobalAuditManager initializes the global audit manager
func InitGlobalAuditManager(logger AuditLogger, sessionID string) {
	managerOnce.Do(func() {
		globalAuditManager = &AuditManager{
			logger:    logger,
			sessionID: sessionID,
		}
	})
}

// GetGlobalAuditManager returns the global audit manager
func GetGlobalAuditManager() *AuditManager {
	return globalAuditManager
}

// LogAPIKeyUsage logs API key usage events
func (m *AuditManager) LogAPIKeyUsage(apiKeyPrefix string, success bool, details string) {
	if m == nil || m.logger == nil {
		return
	}

	event := AuditEvent{
		UserID:    m.userID,
		EventType: EventTypeAPIKeyUsage,
		Resource:  fmt.Sprintf("api_key:%s", apiKeyPrefix),
		Action:    ActionCall,
		Details:   details,
		Success:   success,
	}

	m.logger.LogEvent(event)
}

// LogConfigAccess logs configuration file access events
func (m *AuditManager) LogConfigAccess(configPath string, action string, success bool, details string) {
	if m == nil || m.logger == nil {
		return
	}

	event := AuditEvent{
		UserID:    m.userID,
		EventType: EventTypeConfigAccess,
		Resource:  configPath,
		Action:    action,
		Details:   details,
		Success:   success,
	}

	m.logger.LogEvent(event)
}

// LogFileIO logs file input/output operations
func (m *AuditManager) LogFileIO(filePath string, action string, success bool, details string) {
	if m == nil || m.logger == nil {
		return
	}

	event := AuditEvent{
		UserID:    m.userID,
		EventType: EventTypeFileIO,
		Resource:  filePath,
		Action:    action,
		Details:   details,
		Success:   success,
	}

	m.logger.LogEvent(event)
}

// LogOpenAICall logs OpenAI API calls
func (m *AuditManager) LogOpenAICall(endpoint string, model string, success bool, details string) {
	if m == nil || m.logger == nil {
		return
	}

	event := AuditEvent{
		UserID:    m.userID,
		EventType: EventTypeOpenAICall,
		Resource:  fmt.Sprintf("%s:%s", endpoint, model),
		Action:    ActionCall,
		Details:   details,
		Success:   success,
	}

	m.logger.LogEvent(event)
}

// LogQuotaUsage logs quota usage events
func (m *AuditManager) LogQuotaUsage(quotaType string, usage string, success bool, details string) {
	if m == nil || m.logger == nil {
		return
	}

	event := AuditEvent{
		UserID:    m.userID,
		EventType: EventTypeQuotaUsage,
		Resource:  quotaType,
		Action:    ActionValidate,
		Details:   details,
		Success:   success,
	}

	m.logger.LogEvent(event)
}

// LogToolExecution logs tool execution events
func (m *AuditManager) LogToolExecution(toolName string, success bool, details string) {
	if m == nil || m.logger == nil {
		return
	}

	event := AuditEvent{
		UserID:    m.userID,
		EventType: EventTypeToolExecution,
		Resource:  toolName,
		Action:    ActionExecute,
		Details:   details,
		Success:   success,
	}

	m.logger.LogEvent(event)
}

// Close closes the audit manager and underlying logger
func (m *AuditManager) Close() error {
	if m == nil || m.logger == nil {
		return nil
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	return m.logger.Close()
}

// CreateAuditManagerFromConfig creates an audit manager with configuration
func CreateAuditManagerFromConfig(auditLogPath string, userID string) (*AuditManager, error) {
	if auditLogPath == "" {
		auditLogPath = GetDefaultAuditLogPath()
	}

	logger, err := NewFileAuditLogger(auditLogPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create audit logger: %w", err)
	}

	return &AuditManager{
		logger: logger,
		userID: userID,
	}, nil
}

// Helper functions for global audit manager
func LogAPIKeyUsage(apiKeyPrefix string, success bool, details string) {
	if manager := GetGlobalAuditManager(); manager != nil {
		manager.LogAPIKeyUsage(apiKeyPrefix, success, details)
	}
}

func LogConfigAccess(configPath string, action string, success bool, details string) {
	if manager := GetGlobalAuditManager(); manager != nil {
		manager.LogConfigAccess(configPath, action, success, details)
	}
}

func LogFileIO(filePath string, action string, success bool, details string) {
	if manager := GetGlobalAuditManager(); manager != nil {
		manager.LogFileIO(filePath, action, success, details)
	}
}

func LogOpenAICall(endpoint string, model string, success bool, details string) {
	if manager := GetGlobalAuditManager(); manager != nil {
		manager.LogOpenAICall(endpoint, model, success, details)
	}
}

func LogQuotaUsage(quotaType string, usage string, success bool, details string) {
	if manager := GetGlobalAuditManager(); manager != nil {
		manager.LogQuotaUsage(quotaType, usage, success, details)
	}
}

func LogToolExecution(toolName string, success bool, details string) {
	if manager := GetGlobalAuditManager(); manager != nil {
		manager.LogToolExecution(toolName, success, details)
	}
}
