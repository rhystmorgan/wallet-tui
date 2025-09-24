package storage

import (
	"encoding/json"
	"fmt"

	"rhystmorgan/veWallet/internal/types"
)

// EncryptContact encrypts sensitive fields of a contact
func EncryptContact(contact map[string]interface{}, password string) (*types.EncryptedContact, error) {
	// Extract sensitive data
	sensitiveData := types.SensitiveContactData{
		Notes: contact["notes"].(string),
	}

	if metadata, ok := contact["metadata"].(map[string]string); ok {
		sensitiveData.Metadata = metadata
	}
	if totalSent, ok := contact["total_sent"].(string); ok {
		sensitiveData.TotalSent = totalSent
	}

	// Encrypt sensitive data
	sensitiveJSON, err := json.Marshal(sensitiveData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal sensitive data: %w", err)
	}

	encryptedData, err := Encrypt(sensitiveJSON, password)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt sensitive data: %w", err)
	}

	// Create encrypted contact
	encContact := &types.EncryptedContact{
		ID:            contact["id"].(string),
		Name:          contact["name"].(string),
		Address:       contact["address"].(string),
		CreatedAt:     contact["created_at"].(string),
		UpdatedAt:     contact["updated_at"].(string),
		IsFavorite:    contact["is_favorite"].(bool),
		Category:      contact["category"].(string),
		Tags:          contact["tags"].([]string),
		LastUsed:      contact["last_used"].(string),
		UseCount:      contact["use_count"].(int),
		SensitiveData: encryptedData.Ciphertext,
	}

	return encContact, nil
}

// DecryptContact decrypts sensitive fields of a contact
func DecryptContact(encContact *types.EncryptedContact, password string) (map[string]interface{}, error) {
	// Start with non-sensitive data
	contact := map[string]interface{}{
		"id":          encContact.ID,
		"name":        encContact.Name,
		"address":     encContact.Address,
		"created_at":  encContact.CreatedAt,
		"updated_at":  encContact.UpdatedAt,
		"is_favorite": encContact.IsFavorite,
		"category":    encContact.Category,
		"tags":        encContact.Tags,
		"last_used":   encContact.LastUsed,
		"use_count":   encContact.UseCount,
	}

	// Decrypt sensitive data if present
	if len(encContact.SensitiveData) > 0 {
		encData := &EncryptedData{
			Ciphertext: encContact.SensitiveData,
		}

		decryptedJSON, err := Decrypt(encData, password)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt sensitive data: %w", err)
		}

		var sensitiveData types.SensitiveContactData
		if err := json.Unmarshal(decryptedJSON, &sensitiveData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal sensitive data: %w", err)
		}

		// Add decrypted sensitive data
		contact["notes"] = sensitiveData.Notes
		if sensitiveData.Metadata != nil {
			contact["metadata"] = sensitiveData.Metadata
		}
		if sensitiveData.TotalSent != "" {
			contact["total_sent"] = sensitiveData.TotalSent
		}
	}

	return contact, nil
}

// ReEncryptContact re-encrypts a contact's sensitive data with a new password
func ReEncryptContact(encContact *types.EncryptedContact, oldPassword, newPassword string) (*types.EncryptedContact, error) {
	// First decrypt with old password
	contact, err := DecryptContact(encContact, oldPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt with old password: %w", err)
	}

	// Then encrypt with new password
	return EncryptContact(contact, newPassword)
}

// UpdateEncryptedContact updates an encrypted contact's sensitive data
func UpdateEncryptedContact(encContact *types.EncryptedContact, updates map[string]interface{}, password string) (*types.EncryptedContact, error) {
	// First decrypt current data
	contact, err := DecryptContact(encContact, password)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt current data: %w", err)
	}

	// Apply updates
	for key, value := range updates {
		contact[key] = value
	}

	// Re-encrypt with updates
	return EncryptContact(contact, password)
}
