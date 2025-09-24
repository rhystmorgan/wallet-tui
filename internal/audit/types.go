package audit

import (
	"time"
)

// AuditAction represents the type of action performed on a contact
type AuditAction string

const (
	AuditActionCreate   AuditAction = "create"
	AuditActionUpdate   AuditAction = "update"
	AuditActionDelete   AuditAction = "delete"
	AuditActionView     AuditAction = "view"
	AuditActionExport   AuditAction = "export"
	AuditActionImport   AuditAction = "import"
	AuditActionFavorite AuditAction = "favorite"
)

// AuditLog represents a single audit log entry
type AuditLog struct {
	ID        string                 `json:"id"`
	ContactID string                 `json:"contact_id"`
	Action    AuditAction            `json:"action"`
	Timestamp time.Time              `json:"timestamp"`
	UserID    string                 `json:"user_id,omitempty"`
	SessionID string                 `json:"session_id,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Changes   map[string]Change      `json:"changes,omitempty"`
}

// Change represents a change in a contact field
type Change struct {
	OldValue interface{} `json:"old_value,omitempty"`
	NewValue interface{} `json:"new_value,omitempty"`
}
