package utils

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"rhystmorgan/veWallet/internal/models"
)

type ExportFormat int

const (
	FormatJSON ExportFormat = iota
	FormatCSV
)

type ImportExportOptions struct {
	Format          ExportFormat
	FilePath        string
	IncludeMetadata bool
	IncludeUsage    bool
	IncludeTags     bool
	Encrypt         bool
}

type ImportResult struct {
	TotalContacts    int
	ImportedContacts int
	SkippedContacts  int
	Conflicts        []ContactConflict
	Errors           []ImportError
	Warnings         []string
}

type ContactConflict struct {
	ImportContact   ContactImportData
	ExistingContact models.Contact
	ConflictType    ConflictType
	Resolution      ConflictResolution
}

type ConflictType int

const (
	ConflictAddress ConflictType = iota
	ConflictName
	ConflictBoth
)

type ConflictResolution int

const (
	ResolutionSkip ConflictResolution = iota
	ResolutionOverwrite
	ResolutionMerge
	ResolutionRename
)

type ContactImportData struct {
	Name       string            `json:"name" csv:"name"`
	Address    string            `json:"address" csv:"address"`
	Notes      string            `json:"notes" csv:"notes"`
	Category   string            `json:"category" csv:"category"`
	Tags       []string          `json:"tags" csv:"tags"`
	IsFavorite bool              `json:"is_favorite" csv:"is_favorite"`
	CreatedAt  time.Time         `json:"created_at" csv:"created_at"`
	UseCount   int               `json:"use_count" csv:"use_count"`
	LastUsed   time.Time         `json:"last_used" csv:"last_used"`
	Metadata   map[string]string `json:"metadata,omitempty" csv:"-"`

	// Import metadata
	LineNumber int      `json:"-" csv:"-"`
	IsValid    bool     `json:"-" csv:"-"`
	Errors     []string `json:"-" csv:"-"`
}

type ImportError struct {
	LineNumber int
	Field      string
	Message    string
	Severity   ErrorSeverity
}

type ErrorSeverity int

const (
	SeverityError ErrorSeverity = iota
	SeverityWarning
	SeverityInfo
)

type ContactExporter struct {
	options ImportExportOptions
}

type ContactImporter struct {
	options ImportExportOptions
}

// NewContactExporter creates a new contact exporter
func NewContactExporter(options ImportExportOptions) *ContactExporter {
	return &ContactExporter{
		options: options,
	}
}

// NewContactImporter creates a new contact importer
func NewContactImporter(options ImportExportOptions) *ContactImporter {
	return &ContactImporter{
		options: options,
	}
}

// ExportContacts exports contacts to the specified format
func (e *ContactExporter) ExportContacts(contacts []models.Contact) error {
	// Ensure directory exists
	dir := filepath.Dir(e.options.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	switch e.options.Format {
	case FormatJSON:
		return e.exportJSON(contacts)
	case FormatCSV:
		return e.exportCSV(contacts)
	default:
		return fmt.Errorf("unsupported export format")
	}
}

// exportJSON exports contacts to JSON format
func (e *ContactExporter) exportJSON(contacts []models.Contact) error {
	exportData := make([]ContactImportData, 0, len(contacts))

	for _, contact := range contacts {
		data := ContactImportData{
			Name:       contact.Name,
			Address:    contact.Address,
			Notes:      contact.Notes,
			Category:   contact.Category,
			Tags:       contact.Tags,
			IsFavorite: contact.IsFavorite,
			CreatedAt:  contact.CreatedAt,
		}

		// Include usage data if requested
		if e.options.IncludeUsage {
			data.UseCount = contact.UseCount
			data.LastUsed = contact.LastUsed
		}

		// Include metadata if requested
		if e.options.IncludeMetadata {
			data.Metadata = map[string]string{
				"total_sent":     contact.TotalSent,
				"total_received": contact.TotalReceived,
			}
		}

		exportData = append(exportData, data)
	}

	// Create export wrapper with metadata
	exportWrapper := struct {
		ExportedAt    time.Time           `json:"exported_at"`
		Version       string              `json:"version"`
		TotalContacts int                 `json:"total_contacts"`
		Contacts      []ContactImportData `json:"contacts"`
	}{
		ExportedAt:    time.Now(),
		Version:       "1.0",
		TotalContacts: len(contacts),
		Contacts:      exportData,
	}

	file, err := os.Create(e.options.FilePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(exportWrapper); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}

// exportCSV exports contacts to CSV format
func (e *ContactExporter) exportCSV(contacts []models.Contact) error {
	file, err := os.Create(e.options.FilePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{"name", "address", "notes", "category", "tags", "is_favorite", "created_at"}
	if e.options.IncludeUsage {
		header = append(header, "use_count", "last_used")
	}
	if e.options.IncludeMetadata {
		header = append(header, "total_sent", "total_received")
	}

	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write contact data
	for _, contact := range contacts {
		record := []string{
			contact.Name,
			contact.Address,
			contact.Notes,
			contact.Category,
			strings.Join(contact.Tags, ";"),
			fmt.Sprintf("%t", contact.IsFavorite),
			contact.CreatedAt.Format(time.RFC3339),
		}

		if e.options.IncludeUsage {
			record = append(record,
				fmt.Sprintf("%d", contact.UseCount),
				contact.LastUsed.Format(time.RFC3339),
			)
		}

		if e.options.IncludeMetadata {
			record = append(record,
				contact.TotalSent,
				contact.TotalReceived,
			)
		}

		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write record: %w", err)
		}
	}

	return nil
}

// ImportContacts imports contacts from the specified format
func (i *ContactImporter) ImportContacts() (*ImportResult, []ContactImportData, error) {
	switch i.options.Format {
	case FormatJSON:
		return i.importJSON()
	case FormatCSV:
		return i.importCSV()
	default:
		return nil, nil, fmt.Errorf("unsupported import format")
	}
}

// importJSON imports contacts from JSON format
func (i *ContactImporter) importJSON() (*ImportResult, []ContactImportData, error) {
	file, err := os.Open(i.options.FilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var importWrapper struct {
		ExportedAt    time.Time           `json:"exported_at"`
		Version       string              `json:"version"`
		TotalContacts int                 `json:"total_contacts"`
		Contacts      []ContactImportData `json:"contacts"`
	}

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&importWrapper); err != nil {
		return nil, nil, fmt.Errorf("failed to decode JSON: %w", err)
	}

	result := &ImportResult{
		TotalContacts: len(importWrapper.Contacts),
		Conflicts:     []ContactConflict{},
		Errors:        []ImportError{},
		Warnings:      []string{},
	}

	// Validate imported contacts
	validContacts := make([]ContactImportData, 0, len(importWrapper.Contacts))
	for idx, contact := range importWrapper.Contacts {
		contact.LineNumber = idx + 1
		contact.IsValid = true
		contact.Errors = []string{}

		// Validate contact data
		i.validateContactData(&contact, result)

		if contact.IsValid {
			validContacts = append(validContacts, contact)
		}
	}

	result.ImportedContacts = len(validContacts)
	result.SkippedContacts = result.TotalContacts - result.ImportedContacts

	return result, validContacts, nil
}

// importCSV imports contacts from CSV format
func (i *ContactImporter) importCSV() (*ImportResult, []ContactImportData, error) {
	file, err := os.Open(i.options.FilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read CSV: %w", err)
	}

	if len(records) == 0 {
		return nil, nil, fmt.Errorf("empty CSV file")
	}

	// Parse header
	header := records[0]
	headerMap := make(map[string]int)
	for idx, col := range header {
		headerMap[strings.ToLower(col)] = idx
	}

	result := &ImportResult{
		TotalContacts: len(records) - 1, // Exclude header
		Conflicts:     []ContactConflict{},
		Errors:        []ImportError{},
		Warnings:      []string{},
	}

	validContacts := make([]ContactImportData, 0, len(records)-1)

	// Parse data rows
	for rowIdx, record := range records[1:] {
		contact := ContactImportData{
			LineNumber: rowIdx + 2, // +2 because we skip header and arrays are 0-indexed
			IsValid:    true,
			Errors:     []string{},
		}

		// Parse required fields
		if nameIdx, exists := headerMap["name"]; exists && nameIdx < len(record) {
			contact.Name = strings.TrimSpace(record[nameIdx])
		}
		if addressIdx, exists := headerMap["address"]; exists && addressIdx < len(record) {
			contact.Address = strings.TrimSpace(record[addressIdx])
		}
		if notesIdx, exists := headerMap["notes"]; exists && notesIdx < len(record) {
			contact.Notes = strings.TrimSpace(record[notesIdx])
		}
		if categoryIdx, exists := headerMap["category"]; exists && categoryIdx < len(record) {
			contact.Category = strings.TrimSpace(record[categoryIdx])
		}

		// Parse tags
		if tagsIdx, exists := headerMap["tags"]; exists && tagsIdx < len(record) {
			tagsStr := strings.TrimSpace(record[tagsIdx])
			if tagsStr != "" {
				contact.Tags = strings.Split(tagsStr, ";")
				for j, tag := range contact.Tags {
					contact.Tags[j] = strings.TrimSpace(tag)
				}
			}
		}

		// Parse boolean fields
		if favoriteIdx, exists := headerMap["is_favorite"]; exists && favoriteIdx < len(record) {
			contact.IsFavorite = strings.ToLower(strings.TrimSpace(record[favoriteIdx])) == "true"
		}

		// Parse dates
		if createdIdx, exists := headerMap["created_at"]; exists && createdIdx < len(record) {
			if createdStr := strings.TrimSpace(record[createdIdx]); createdStr != "" {
				if parsed, err := time.Parse(time.RFC3339, createdStr); err == nil {
					contact.CreatedAt = parsed
				}
			}
		}

		// Parse usage data if available
		if useCountIdx, exists := headerMap["use_count"]; exists && useCountIdx < len(record) {
			if useCountStr := strings.TrimSpace(record[useCountIdx]); useCountStr != "" {
				fmt.Sscanf(useCountStr, "%d", &contact.UseCount)
			}
		}
		if lastUsedIdx, exists := headerMap["last_used"]; exists && lastUsedIdx < len(record) {
			if lastUsedStr := strings.TrimSpace(record[lastUsedIdx]); lastUsedStr != "" {
				if parsed, err := time.Parse(time.RFC3339, lastUsedStr); err == nil {
					contact.LastUsed = parsed
				}
			}
		}

		// Validate contact data
		i.validateContactData(&contact, result)

		if contact.IsValid {
			validContacts = append(validContacts, contact)
		}
	}

	result.ImportedContacts = len(validContacts)
	result.SkippedContacts = result.TotalContacts - result.ImportedContacts

	return result, validContacts, nil
}

// validateContactData validates imported contact data
func (i *ContactImporter) validateContactData(contact *ContactImportData, result *ImportResult) {
	// Validate required fields
	if strings.TrimSpace(contact.Name) == "" {
		contact.Errors = append(contact.Errors, "Name is required")
		contact.IsValid = false
		result.Errors = append(result.Errors, ImportError{
			LineNumber: contact.LineNumber,
			Field:      "name",
			Message:    "Name is required",
			Severity:   SeverityError,
		})
	}

	if strings.TrimSpace(contact.Address) == "" {
		contact.Errors = append(contact.Errors, "Address is required")
		contact.IsValid = false
		result.Errors = append(result.Errors, ImportError{
			LineNumber: contact.LineNumber,
			Field:      "address",
			Message:    "Address is required",
			Severity:   SeverityError,
		})
	} else if !isValidVeChainAddress(contact.Address) {
		contact.Errors = append(contact.Errors, "Invalid VeChain address format")
		contact.IsValid = false
		result.Errors = append(result.Errors, ImportError{
			LineNumber: contact.LineNumber,
			Field:      "address",
			Message:    "Invalid VeChain address format",
			Severity:   SeverityError,
		})
	}

	// Validate field lengths
	if len(contact.Name) > 50 {
		contact.Errors = append(contact.Errors, "Name too long (max 50 characters)")
		contact.IsValid = false
		result.Errors = append(result.Errors, ImportError{
			LineNumber: contact.LineNumber,
			Field:      "name",
			Message:    "Name too long (max 50 characters)",
			Severity:   SeverityError,
		})
	}

	if len(contact.Notes) > 200 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Line %d: Notes truncated to 200 characters", contact.LineNumber))
		contact.Notes = contact.Notes[:200]
	}

	// Set defaults
	if contact.Category == "" {
		contact.Category = "Personal"
	}

	if contact.CreatedAt.IsZero() {
		contact.CreatedAt = time.Now()
	}
}

// DetectConflicts detects conflicts between imported and existing contacts
func DetectConflicts(importedContacts []ContactImportData, existingContacts []models.Contact) []ContactConflict {
	conflicts := []ContactConflict{}

	for _, imported := range importedContacts {
		for _, existing := range existingContacts {
			conflict := ContactConflict{
				ImportContact:   imported,
				ExistingContact: existing,
			}

			addressMatch := strings.EqualFold(imported.Address, existing.Address)
			nameMatch := strings.EqualFold(imported.Name, existing.Name)

			if addressMatch && nameMatch {
				conflict.ConflictType = ConflictBoth
				conflicts = append(conflicts, conflict)
			} else if addressMatch {
				conflict.ConflictType = ConflictAddress
				conflicts = append(conflicts, conflict)
			} else if nameMatch {
				conflict.ConflictType = ConflictName
				conflicts = append(conflicts, conflict)
			}
		}
	}

	return conflicts
}

// ResolveConflicts applies conflict resolutions to imported contacts
func ResolveConflicts(conflicts []ContactConflict) []ContactImportData {
	resolvedContacts := []ContactImportData{}

	for _, conflict := range conflicts {
		switch conflict.Resolution {
		case ResolutionSkip:
			// Skip this contact
			continue
		case ResolutionOverwrite:
			// Use imported contact as-is
			resolvedContacts = append(resolvedContacts, conflict.ImportContact)
		case ResolutionMerge:
			// Merge imported and existing contact data
			merged := conflict.ImportContact
			if merged.Notes == "" {
				merged.Notes = conflict.ExistingContact.Notes
			}
			if len(merged.Tags) == 0 {
				merged.Tags = conflict.ExistingContact.Tags
			}
			if merged.Category == "" || merged.Category == "Personal" {
				merged.Category = conflict.ExistingContact.Category
			}
			resolvedContacts = append(resolvedContacts, merged)
		case ResolutionRename:
			// Rename imported contact
			renamed := conflict.ImportContact
			renamed.Name = renamed.Name + " (Imported)"
			resolvedContacts = append(resolvedContacts, renamed)
		}
	}

	return resolvedContacts
}

// ConvertToContacts converts ContactImportData to Contact models
func ConvertToContacts(importData []ContactImportData) []models.Contact {
	contacts := make([]models.Contact, 0, len(importData))

	for _, data := range importData {
		contact := models.NewContact(data.Name, data.Address, data.Notes)
		contact.Category = data.Category
		contact.Tags = data.Tags
		contact.IsFavorite = data.IsFavorite
		contact.UseCount = data.UseCount
		contact.LastUsed = data.LastUsed

		if !data.CreatedAt.IsZero() {
			contact.CreatedAt = data.CreatedAt
		}

		contacts = append(contacts, *contact)
	}

	return contacts
}

// Helper function for VeChain address validation
func isValidVeChainAddress(address string) bool {
	if len(address) != 42 {
		return false
	}
	if !strings.HasPrefix(strings.ToLower(address), "0x") {
		return false
	}

	hex := address[2:]
	for _, char := range hex {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
			return false
		}
	}

	return true
}

// GenerateBackupFilename generates a timestamped backup filename
func GenerateBackupFilename(format ExportFormat) string {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	switch format {
	case FormatJSON:
		return fmt.Sprintf("contacts_backup_%s.json", timestamp)
	case FormatCSV:
		return fmt.Sprintf("contacts_backup_%s.csv", timestamp)
	default:
		return fmt.Sprintf("contacts_backup_%s.json", timestamp)
	}
}

// GetDefaultExportPath returns the default export path for the user's system
func GetDefaultExportPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	exportDir := filepath.Join(homeDir, ".veterm", "exports")
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		return "", err
	}

	return exportDir, nil
}
