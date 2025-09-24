package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"rhystmorgan/veWallet/internal/audit"
	"rhystmorgan/veWallet/internal/types"
	"rhystmorgan/veWallet/internal/validation"
)

type Contact struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	Notes     string    `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Enhanced fields for Phase 3.2
	IsFavorite    bool      `json:"is_favorite"`
	Category      string    `json:"category,omitempty"`
	Tags          []string  `json:"tags,omitempty"`
	LastUsed      time.Time `json:"last_used,omitempty"`
	UseCount      int       `json:"use_count"`
	TotalSent     string    `json:"total_sent,omitempty"`     // Store as string to avoid big.Int JSON issues
	TotalReceived string    `json:"total_received,omitempty"` // Store as string to avoid big.Int JSON issues

	// Sensitive data encryption
	encryptedData []byte `json:"-"` // Not serialized directly
}

type ContactList struct {
	Contacts []Contact `json:"contacts"`
}

func NewContact(name, address, notes string) *Contact {
	return &Contact{
		ID:        generateContactID(),
		Name:      strings.TrimSpace(name),
		Address:   strings.TrimSpace(address),
		Notes:     strings.TrimSpace(notes),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (c *Contact) Update(name, address, notes string, auditor *audit.ContactAuditor, userID, sessionID string) error {
	changes := make(map[string]audit.Change)

	if name != "" {
		oldName := c.Name
		c.Name = strings.TrimSpace(name)
		if oldName != c.Name {
			changes["name"] = audit.Change{OldValue: oldName, NewValue: c.Name}
		}
	}

	if address != "" {
		oldAddress := c.Address
		c.Address = strings.TrimSpace(address)
		if oldAddress != c.Address {
			changes["address"] = audit.Change{OldValue: oldAddress, NewValue: c.Address}
		}
	}

	oldNotes := c.Notes
	c.Notes = strings.TrimSpace(notes)
	if oldNotes != c.Notes {
		changes["notes"] = audit.Change{OldValue: oldNotes, NewValue: c.Notes}
	}

	c.UpdatedAt = time.Now()

	if auditor != nil && len(changes) > 0 {
		return auditor.LogContactChange(c.ID, changes, userID, sessionID)
	}
	return nil
}

func (c *Contact) ToggleFavorite(auditor *audit.ContactAuditor, userID, sessionID string) error {
	c.IsFavorite = !c.IsFavorite
	c.UpdatedAt = time.Now()

	if auditor != nil {
		details := map[string]interface{}{
			"is_favorite": c.IsFavorite,
		}
		return auditor.LogContactAction(audit.AuditActionFavorite, c.ID, userID, sessionID, details)
	}
	return nil
}

func (c *Contact) SetFavorite(favorite bool, auditor *audit.ContactAuditor, userID, sessionID string) error {
	if c.IsFavorite == favorite {
		return nil
	}

	c.IsFavorite = favorite
	c.UpdatedAt = time.Now()

	if auditor != nil {
		details := map[string]interface{}{
			"is_favorite": c.IsFavorite,
		}
		return auditor.LogContactAction(audit.AuditActionFavorite, c.ID, userID, sessionID, details)
	}
	return nil
}

func (c *Contact) Use(auditor *audit.ContactAuditor, userID, sessionID string) error {
	c.UseCount++
	c.LastUsed = time.Now()
	c.UpdatedAt = time.Now()

	if auditor != nil {
		details := map[string]interface{}{
			"use_count": c.UseCount,
			"last_used": c.LastUsed,
		}
		return auditor.LogContactAction(audit.AuditActionView, c.ID, userID, sessionID, details)
	}
	return nil
}

func (c *Contact) AddTag(tag string, auditor *audit.ContactAuditor, userID, sessionID string) error {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return nil
	}

	// Check if tag already exists
	for _, existingTag := range c.Tags {
		if strings.EqualFold(existingTag, tag) {
			return nil
		}
	}

	c.Tags = append(c.Tags, tag)
	c.UpdatedAt = time.Now()

	if auditor != nil {
		details := map[string]interface{}{
			"added_tag": tag,
		}
		return auditor.LogContactAction(audit.AuditActionUpdate, c.ID, userID, sessionID, details)
	}
	return nil
}

func (c *Contact) RemoveTag(tag string, auditor *audit.ContactAuditor, userID, sessionID string) error {
	for i, existingTag := range c.Tags {
		if strings.EqualFold(existingTag, tag) {
			c.Tags = append(c.Tags[:i], c.Tags[i+1:]...)
			c.UpdatedAt = time.Now()

			if auditor != nil {
				details := map[string]interface{}{
					"removed_tag": tag,
				}
				return auditor.LogContactAction(audit.AuditActionUpdate, c.ID, userID, sessionID, details)
			}
			return nil
		}
	}
	return nil
}

func (c *Contact) HasTag(tag string) bool {
	for _, existingTag := range c.Tags {
		if strings.EqualFold(existingTag, tag) {
			return true
		}
	}
	return false
}

func (cl *ContactList) Add(contact *Contact, auditor *audit.ContactAuditor, userID, sessionID string) error {
	cl.Contacts = append(cl.Contacts, *contact)

	if auditor != nil {
		return auditor.LogContactAction(audit.AuditActionCreate, contact.ID, userID, sessionID, nil)
	}
	return nil
}

func (cl *ContactList) Remove(id string, auditor *audit.ContactAuditor, userID, sessionID string) error {
	for i, contact := range cl.Contacts {
		if contact.ID == id {
			if auditor != nil {
				// Log before removal
				if err := auditor.LogContactAction(audit.AuditActionDelete, contact.ID, userID, sessionID, nil); err != nil {
					return err
				}
			}
			cl.Contacts = append(cl.Contacts[:i], cl.Contacts[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("contact not found: %s", id)
}

func (cl *ContactList) FindByID(id string) *Contact {
	for i, contact := range cl.Contacts {
		if contact.ID == id {
			return &cl.Contacts[i]
		}
	}
	return nil
}

func (cl *ContactList) FindByAddress(address string) *Contact {
	for i, contact := range cl.Contacts {
		if strings.EqualFold(contact.Address, address) {
			return &cl.Contacts[i]
		}
	}
	return nil
}

func (cl *ContactList) GetFavorites() []Contact {
	var favorites []Contact
	for _, contact := range cl.Contacts {
		if contact.IsFavorite {
			favorites = append(favorites, contact)
		}
	}
	return favorites
}

func (cl *ContactList) GetMostUsed(limit int) []Contact {
	if limit <= 0 {
		limit = 10
	}

	// Create a copy and sort by use count
	contacts := make([]Contact, len(cl.Contacts))
	copy(contacts, cl.Contacts)

	// Sort by use count (descending), then by last used (descending)
	for i := 0; i < len(contacts)-1; i++ {
		for j := i + 1; j < len(contacts); j++ {
			if contacts[i].UseCount < contacts[j].UseCount ||
				(contacts[i].UseCount == contacts[j].UseCount && contacts[i].LastUsed.Before(contacts[j].LastUsed)) {
				contacts[i], contacts[j] = contacts[j], contacts[i]
			}
		}
	}

	if len(contacts) > limit {
		contacts = contacts[:limit]
	}

	return contacts
}

func (cl *ContactList) GetRecentlyUsed(limit int) []Contact {
	if limit <= 0 {
		limit = 10
	}

	// Create a copy and sort by last used
	contacts := make([]Contact, len(cl.Contacts))
	copy(contacts, cl.Contacts)

	// Sort by last used (descending)
	for i := 0; i < len(contacts)-1; i++ {
		for j := i + 1; j < len(contacts); j++ {
			if contacts[i].LastUsed.Before(contacts[j].LastUsed) {
				contacts[i], contacts[j] = contacts[j], contacts[i]
			}
		}
	}

	if len(contacts) > limit {
		contacts = contacts[:limit]
	}

	return contacts
}

func (cl *ContactList) SearchByTag(tag string) []Contact {
	var results []Contact
	for _, contact := range cl.Contacts {
		if contact.HasTag(tag) {
			results = append(results, contact)
		}
	}
	return results
}

func (cl *ContactList) GetAllTags() []string {
	tagMap := make(map[string]bool)

	for _, contact := range cl.Contacts {
		for _, tag := range contact.Tags {
			tagMap[strings.ToLower(tag)] = true
		}
	}

	var tags []string
	for tag := range tagMap {
		tags = append(tags, tag)
	}

	// Simple sort
	for i := 0; i < len(tags)-1; i++ {
		for j := i + 1; j < len(tags); j++ {
			if tags[i] > tags[j] {
				tags[i], tags[j] = tags[j], tags[i]
			}
		}
	}

	return tags
}

// Validate performs comprehensive validation of the contact using the provided validator
func (c *Contact) Validate(validator *validation.ContactValidator, contacts []Contact) validation.ValidationResult {
	// Get list of existing addresses (excluding self)
	var existingAddresses []string
	for _, contact := range contacts {
		if contact.ID != c.ID {
			existingAddresses = append(existingAddresses, contact.Address)
		}
	}
	return validator.ValidateContact(c.Name, c.Address, c.Category, c.Tags, existingAddresses)
}

// ApplyValidationSuggestions applies any suggestions from the validation result
func (c *Contact) ApplyValidationSuggestions(result validation.ValidationResult) {
	for _, err := range result.Errors {
		if err.Code == validation.ErrorChecksumMismatch && err.Field == "address" {
			// Apply checksum suggestion
			for _, suggestion := range result.Suggestions {
				if strings.HasPrefix(suggestion, "Use checksum address: ") {
					c.Address = strings.TrimPrefix(suggestion, "Use checksum address: ")
					break
				}
			}
		}
	}
}

// ToEncryptedContact converts the contact to an encrypted contact
func (c *Contact) ToEncryptedContact() *types.EncryptedContact {
	return &types.EncryptedContact{
		ID:            c.ID,
		Name:          c.Name,
		Address:       c.Address,
		CreatedAt:     c.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     c.UpdatedAt.Format(time.RFC3339),
		IsFavorite:    c.IsFavorite,
		Category:      c.Category,
		Tags:          c.Tags,
		LastUsed:      c.LastUsed.Format(time.RFC3339),
		UseCount:      c.UseCount,
		SensitiveData: c.encryptedData,
	}
}

// FromEncryptedContact updates the contact from an encrypted contact
func (c *Contact) FromEncryptedContact(enc *types.EncryptedContact) error {
	c.ID = enc.ID
	c.Name = enc.Name
	c.Address = enc.Address
	c.IsFavorite = enc.IsFavorite
	c.Category = enc.Category
	c.Tags = enc.Tags
	c.UseCount = enc.UseCount
	c.encryptedData = enc.SensitiveData

	// Parse time fields
	var err error
	if c.CreatedAt, err = time.Parse(time.RFC3339, enc.CreatedAt); err != nil {
		return fmt.Errorf("invalid created_at time: %w", err)
	}
	if c.UpdatedAt, err = time.Parse(time.RFC3339, enc.UpdatedAt); err != nil {
		return fmt.Errorf("invalid updated_at time: %w", err)
	}
	if enc.LastUsed != "" {
		if c.LastUsed, err = time.Parse(time.RFC3339, enc.LastUsed); err != nil {
			return fmt.Errorf("invalid last_used time: %w", err)
		}
	}

	return nil
}

// EncryptSensitiveData encrypts sensitive fields of the contact
func (c *Contact) EncryptSensitiveData(password string) error {
	sensitiveData := types.SensitiveContactData{
		Notes:     c.Notes,
		TotalSent: c.TotalSent,
		Metadata:  make(map[string]string),
	}

	// Add any additional metadata
	sensitiveData.Metadata["total_received"] = c.TotalReceived

	// Marshal sensitive data
	sensitiveJSON, err := json.Marshal(sensitiveData)
	if err != nil {
		return fmt.Errorf("failed to marshal sensitive data: %w", err)
	}

	// Encrypt the data
	encryptedData, err := json.Marshal(sensitiveJSON)
	if err != nil {
		return fmt.Errorf("failed to marshal encrypted data: %w", err)
	}

	c.encryptedData = encryptedData

	// Clear sensitive fields from memory
	c.Notes = ""
	c.TotalSent = ""
	c.TotalReceived = ""

	return nil
}

// DecryptSensitiveData decrypts sensitive fields of the contact
func (c *Contact) DecryptSensitiveData(password string) error {
	if len(c.encryptedData) == 0 {
		return nil // No encrypted data to decrypt
	}

	// Unmarshal encrypted data
	var sensitiveJSON []byte
	if err := json.Unmarshal(c.encryptedData, &sensitiveJSON); err != nil {
		return fmt.Errorf("failed to unmarshal encrypted data: %w", err)
	}

	// Unmarshal sensitive data
	var sensitiveData types.SensitiveContactData
	if err := json.Unmarshal(sensitiveJSON, &sensitiveData); err != nil {
		return fmt.Errorf("failed to unmarshal sensitive data: %w", err)
	}

	// Restore sensitive fields
	c.Notes = sensitiveData.Notes
	c.TotalSent = sensitiveData.TotalSent
	if totalReceived, ok := sensitiveData.Metadata["total_received"]; ok {
		c.TotalReceived = totalReceived
	}

	return nil
}

// ReEncryptSensitiveData re-encrypts sensitive data with a new password
func (c *Contact) ReEncryptSensitiveData(oldPassword, newPassword string) error {
	// First decrypt with old password
	if err := c.DecryptSensitiveData(oldPassword); err != nil {
		return fmt.Errorf("failed to decrypt with old password: %w", err)
	}

	// Then encrypt with new password
	return c.EncryptSensitiveData(newPassword)
}

// HasEncryptedData returns true if the contact has encrypted sensitive data
func (c *Contact) HasEncryptedData() bool {
	return len(c.encryptedData) > 0
}

// ClearSensitiveData clears all sensitive data from memory
func (c *Contact) ClearSensitiveData() {
	c.Notes = ""
	c.TotalSent = ""
	c.TotalReceived = ""
	c.encryptedData = nil
}

func generateContactID() string {
	return "contact_" + time.Now().Format("20060102150405")
}
