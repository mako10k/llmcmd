package security

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AuditEvent represents a security audit log event
type AuditEvent struct {
	Timestamp time.Time `json:"timestamp"`
	UserID    string    `json:"user_id"`
	EventType string    `json:"event_type"`
	Resource  string    `json:"resource"`
	Action    string    `json:"action"`
	Details   string    `json:"details"`
	Success   bool      `json:"success"`
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
		"timestamp":  event.Timestamp.Format(time.RFC3339),
		"user_id":    event.UserID,
		"event_type": event.EventType,
		"resource":   event.Resource,
		"action":     event.Action,
		"details":    event.Details,
		"success":    event.Success,
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
