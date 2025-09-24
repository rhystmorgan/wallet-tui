package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ContactAuditor handles contact audit logging
type ContactAuditor struct {
	logFile    string
	mu         sync.RWMutex
	batchSize  int
	batchMu    sync.Mutex
	batchLogs  []AuditLog
	flushTimer *time.Timer
}

// NewContactAuditor creates a new ContactAuditor instance
func NewContactAuditor(logDir string) (*ContactAuditor, error) {
	// Ensure log directory exists
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create audit log directory: %w", err)
	}

	logFile := filepath.Join(logDir, fmt.Sprintf("contact_audit_%s.log", time.Now().Format("2006-01-02")))

	auditor := &ContactAuditor{
		logFile:   logFile,
		batchSize: 10, // Flush logs after 10 entries
		batchLogs: make([]AuditLog, 0, 10),
	}

	// Start flush timer (flush every minute if batch not full)
	auditor.flushTimer = time.AfterFunc(time.Minute, func() {
		_ = auditor.Flush() // Ignore error in timer callback
	})

	return auditor, nil
}

// LogContactAction logs a contact-related action
func (a *ContactAuditor) LogContactAction(action AuditAction, contactID string, userID, sessionID string, details map[string]interface{}) error {
	log := AuditLog{
		ID:        fmt.Sprintf("audit_%s_%s", contactID, time.Now().Format("20060102150405")),
		ContactID: contactID,
		Action:    action,
		Timestamp: time.Now(),
		UserID:    userID,
		SessionID: sessionID,
		Details:   details,
	}

	a.batchMu.Lock()
	a.batchLogs = append(a.batchLogs, log)

	// Flush if batch is full
	if len(a.batchLogs) >= a.batchSize {
		a.batchMu.Unlock()
		return a.Flush()
	}
	a.batchMu.Unlock()

	return nil
}

// LogContactChange logs changes made to a contact
func (a *ContactAuditor) LogContactChange(contactID string, changes map[string]Change, userID, sessionID string) error {
	if len(changes) == 0 {
		return nil // No changes to log
	}

	log := AuditLog{
		ID:        fmt.Sprintf("audit_%s_%s", contactID, time.Now().Format("20060102150405")),
		ContactID: contactID,
		Action:    AuditActionUpdate,
		Timestamp: time.Now(),
		UserID:    userID,
		SessionID: sessionID,
		Changes:   changes,
	}

	a.batchMu.Lock()
	a.batchLogs = append(a.batchLogs, log)

	// Flush if batch is full
	if len(a.batchLogs) >= a.batchSize {
		a.batchMu.Unlock()
		return a.Flush()
	}
	a.batchMu.Unlock()

	return nil
}

// Flush writes all pending audit logs to storage
func (a *ContactAuditor) Flush() error {
	a.batchMu.Lock()
	if len(a.batchLogs) == 0 {
		a.batchMu.Unlock()
		return nil
	}

	// Reset timer
	if a.flushTimer != nil {
		a.flushTimer.Reset(time.Minute)
	}

	// Get logs to flush and clear batch
	logsToFlush := make([]AuditLog, len(a.batchLogs))
	copy(logsToFlush, a.batchLogs)
	a.batchLogs = a.batchLogs[:0]
	a.batchMu.Unlock()

	// Write logs to file
	file, err := os.OpenFile(a.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open audit log file: %w", err)
	}
	defer file.Close()

	for _, log := range logsToFlush {
		logJSON, err := json.Marshal(log)
		if err != nil {
			return fmt.Errorf("failed to marshal audit log: %w", err)
		}

		if _, err := file.Write(append(logJSON, '\n')); err != nil {
			return fmt.Errorf("failed to write audit log: %w", err)
		}
	}

	return nil
}

// GetContactHistory retrieves audit history for a specific contact
func (a *ContactAuditor) GetContactHistory(contactID string) ([]AuditLog, error) {
	var logs []AuditLog

	// Flush pending logs first
	if err := a.Flush(); err != nil {
		return nil, err
	}

	// Read and parse log file
	file, err := os.Open(a.logFile)
	if err != nil {
		if os.IsNotExist(err) {
			return logs, nil
		}
		return nil, fmt.Errorf("failed to open audit log file: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	for {
		var log AuditLog
		if err := decoder.Decode(&log); err != nil {
			break // End of file or error
		}

		if log.ContactID == contactID {
			logs = append(logs, log)
		}
	}

	return logs, nil
}

// Close ensures all pending logs are written
func (a *ContactAuditor) Close() error {
	if a.flushTimer != nil {
		a.flushTimer.Stop()
	}
	return a.Flush()
}
