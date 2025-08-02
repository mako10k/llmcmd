package security

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Security Audit Logging for llmcmd CLI Tool
//
// Purpose:
// - Track API key usage for security monitoring
// - Record configuration file access for compliance
// - Log file I/O operations for debugging
// - Monitor OpenAI API calls for cost management
//
// Note: llmcmd is a single-user CLI tool, so "audit" focuses on:
// - Operation history rather than user authentication
// - Process and session identification rather than user identity
// - Security-relevant events rather than access control

// AuditEvent represents a security audit log event for llmcmd CLI operations
type AuditEvent struct {
	Timestamp  time.Time `json:"timestamp"`   // RFC3339 format
	SystemUser string    `json:"system_user"` // OS username
	ProcessID  int       `json:"process_id"`  // Process ID
	SessionID  string    `json:"session_id"`  // Session identifier (optional)
	EventType  string    `json:"event_type"`  // Event category
	Resource   string    `json:"resource"`    // Target resource (file, API endpoint, etc.)
	Action     string    `json:"action"`      // Action performed
	Details    string    `json:"details"`     // Additional information
	Success    bool      `json:"success"`     // Operation success status
}

// AuditEventType defines standard event types for consistent logging
const (
	EventTypeAPIKeyUsage   = "API_KEY_USAGE"
	EventTypeConfigAccess  = "CONFIG_ACCESS"
	EventTypeFileIO        = "FILE_IO"
	EventTypeOpenAICall    = "OPENAI_CALL"
	EventTypeQuotaUsage    = "QUOTA_USAGE"
	EventTypeToolExecution = "TOOL_EXECUTION"
)

// AuditAction defines standard actions for consistent logging
const (
	ActionRead     = "read"
	ActionWrite    = "write"
	ActionCall     = "call"
	ActionLoad     = "load"
	ActionSave     = "save"
	ActionExecute  = "execute"
	ActionValidate = "validate"
)

// AuditLogger interface defines the contract for audit logging
type AuditLogger interface {
	LogEvent(event AuditEvent) error
	Close() error
}

// FileAuditLogger implements AuditLogger for file-based logging
type FileAuditLogger struct {
	file   *os.File
	mutex  sync.Mutex
	closed bool
}

// NewFileAuditLogger creates a new file-based audit logger
func NewFileAuditLogger(filename string) (*FileAuditLogger, error) {
	// Create audit log directory if it doesn't exist
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create audit log directory: %w", err)
	}

	// Open audit log file with append mode and restricted permissions
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log file: %w", err)
	}

	return &FileAuditLogger{
		file: file,
	}, nil
}

// LogEvent logs an audit event to the file
func (l *FileAuditLogger) LogEvent(event AuditEvent) error {
	if l.closed {
		return fmt.Errorf("audit logger is closed")
	}

	l.mutex.Lock()
	defer l.mutex.Unlock()

	// Ensure timestamp is set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	// Create structured log entry
	logEntry := map[string]interface{}{
		"timestamp":   event.Timestamp.Format(time.RFC3339),
		"system_user": event.SystemUser,
		"process_id":  event.ProcessID,
		"session_id":  event.SessionID,
		"event_type":  event.EventType,
		"resource":    event.Resource,
		"action":      event.Action,
		"details":     event.Details,
		"success":     event.Success,
	}

	// Convert to JSON
	jsonData, err := json.Marshal(logEntry)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}

	// Write to file with newline
	if _, err := l.file.Write(append(jsonData, '\n')); err != nil {
		return fmt.Errorf("failed to write audit event: %w", err)
	}

	// Sync to ensure immediate write
	if err := l.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync audit log: %w", err)
	}

	return nil
}

// Close closes the audit logger
func (l *FileAuditLogger) Close() error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if l.closed {
		return nil
	}

	l.closed = true
	return l.file.Close()
}

// GetDefaultAuditLogPath returns the default audit log file path
func GetDefaultAuditLogPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".llmcmd_audit.log"
	}
	return filepath.Join(homeDir, ".llmcmd_audit.log")
}

// CreateDefaultAuditLogger creates an audit logger with default settings
func CreateDefaultAuditLogger() (AuditLogger, error) {
	logPath := GetDefaultAuditLogPath()
	return NewFileAuditLogger(logPath)
}

// GetCurrentSystemUser returns the current OS username
func GetCurrentSystemUser() string {
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	if user := os.Getenv("USERNAME"); user != "" { // Windows
		return user
	}
	return "unknown"
}

// GetCurrentProcessID returns the current process ID
func GetCurrentProcessID() int {
	return os.Getpid()
}

// CreateAuditEvent creates a new audit event with system information pre-filled
func CreateAuditEvent(eventType, resource, action, details string, success bool, sessionID string) AuditEvent {
	return AuditEvent{
		Timestamp:  time.Now().UTC(),
		SystemUser: GetCurrentSystemUser(),
		ProcessID:  GetCurrentProcessID(),
		SessionID:  sessionID,
		EventType:  eventType,
		Resource:   resource,
		Action:     action,
		Details:    details,
		Success:    success,
	}
}
