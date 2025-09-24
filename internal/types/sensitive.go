package types

// SensitiveContactData represents sensitive fields that need extra encryption
type SensitiveContactData struct {
	Notes     string            `json:"notes,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	TotalSent string            `json:"total_sent,omitempty"`
}

// EncryptedContact represents a contact with encrypted sensitive data
type EncryptedContact struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Address       string   `json:"address"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
	IsFavorite    bool     `json:"is_favorite"`
	Category      string   `json:"category,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	LastUsed      string   `json:"last_used,omitempty"`
	UseCount      int      `json:"use_count"`
	SensitiveData []byte   `json:"sensitive_data,omitempty"`
}
