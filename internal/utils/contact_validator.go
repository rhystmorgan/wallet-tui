package utils

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"rhystmorgan/veWallet/internal/blockchain"
	"rhystmorgan/veWallet/internal/models"

	"github.com/ethereum/go-ethereum/common"
)

// ValidationSeverity represents the severity level of a validation issue
type ValidationSeverity int

const (
	ValidationSeverityError ValidationSeverity = iota
	ValidationSeverityWarning
	ValidationSeverityInfo
)

// ValidationErrorCode represents specific validation error types
type ValidationErrorCode int

const (
	ErrorInvalidAddress ValidationErrorCode = iota
	ErrorDuplicateAddress
	ErrorDuplicateName
	ErrorInvalidFormat
	ErrorAddressNotFound
	ErrorChecksumMismatch
	ErrorNameTooLong
	ErrorNameRequired
	ErrorAddressRequired
	ErrorInvalidCategory
	ErrorTooManyTags
	ErrorInvalidTag
)

// ValidationError represents a specific validation error
type ValidationError struct {
	Field    string
	Code     ValidationErrorCode
	Message  string
	Severity ValidationSeverity
}

// ValidationResult represents the result of contact validation
type ValidationResult struct {
	IsValid     bool
	ValidatedAt time.Time
	Errors      []ValidationError
	Warnings    []ValidationError
	Suggestions []string
}

// ValidationCache represents a cached validation result
type ValidationCache struct {
	Result    ValidationResult
	ExpiresAt time.Time
}

// ContactValidator handles contact validation with caching
type ContactValidator struct {
	blockchainClient *blockchain.Client
	cache            map[string]ValidationCache
	cacheMutex       sync.RWMutex
	cacheExpiry      time.Duration
}

// NewContactValidator creates a new ContactValidator instance
func NewContactValidator(client *blockchain.Client) *ContactValidator {
	return &ContactValidator{
		blockchainClient: client,
		cache:            make(map[string]ValidationCache),
		cacheExpiry:      15 * time.Minute, // Cache results for 15 minutes
	}
}

// ValidateContact performs comprehensive validation of a contact
func (v *ContactValidator) ValidateContact(contact *models.Contact, existingContacts []models.Contact) ValidationResult {
	// Check cache first
	if result, ok := v.getCachedResult(contact); ok {
		return result
	}

	result := ValidationResult{
		IsValid:     true,
		ValidatedAt: time.Now(),
	}

	// Validate required fields
	if strings.TrimSpace(contact.Name) == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:    "name",
			Code:     ErrorNameRequired,
			Message:  "Name is required",
			Severity: ValidationSeverityError,
		})
		result.IsValid = false
	}

	if strings.TrimSpace(contact.Address) == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:    "address",
			Code:     ErrorAddressRequired,
			Message:  "Address is required",
			Severity: ValidationSeverityError,
		})
		result.IsValid = false
	}

	// Validate field lengths
	if len(contact.Name) > 50 {
		result.Errors = append(result.Errors, ValidationError{
			Field:    "name",
			Code:     ErrorNameTooLong,
			Message:  "Name too long (max 50 characters)",
			Severity: ValidationSeverityError,
		})
		result.IsValid = false
	}

	// Validate address format and checksum
	if contact.Address != "" {
		if !common.IsHexAddress(contact.Address) {
			result.Errors = append(result.Errors, ValidationError{
				Field:    "address",
				Code:     ErrorInvalidAddress,
				Message:  "Invalid VeChain address format",
				Severity: ValidationSeverityError,
			})
			result.IsValid = false
		} else {
			// Verify checksum
			checksumAddr := common.HexToAddress(contact.Address).Hex()
			if checksumAddr != contact.Address {
				result.Errors = append(result.Errors, ValidationError{
					Field:    "address",
					Code:     ErrorChecksumMismatch,
					Message:  fmt.Sprintf("Address checksum mismatch. Suggested: %s", checksumAddr),
					Severity: ValidationSeverityWarning,
				})
				result.Suggestions = append(result.Suggestions, fmt.Sprintf("Use checksum address: %s", checksumAddr))
			}

			// Verify address exists on blockchain (if client available)
			if v.blockchainClient != nil {
				if exists, err := v.blockchainClient.AddressExists(contact.Address); err != nil {
					result.Warnings = append(result.Warnings, ValidationError{
						Field:    "address",
						Code:     ErrorAddressNotFound,
						Message:  "Could not verify address on blockchain",
						Severity: ValidationSeverityWarning,
					})
				} else if !exists {
					result.Warnings = append(result.Warnings, ValidationError{
						Field:    "address",
						Code:     ErrorAddressNotFound,
						Message:  "Address not found on blockchain",
						Severity: ValidationSeverityWarning,
					})
				}
			}
		}
	}

	// Validate category
	if contact.Category != "" {
		validCategories := []string{"Personal", "Business", "Exchange", "DeFi", "Friends", "Family"}
		isValidCategory := false
		for _, cat := range validCategories {
			if strings.EqualFold(contact.Category, cat) {
				isValidCategory = true
				break
			}
		}
		if !isValidCategory {
			result.Warnings = append(result.Warnings, ValidationError{
				Field:    "category",
				Code:     ErrorInvalidCategory,
				Message:  "Invalid category",
				Severity: ValidationSeverityWarning,
			})
			result.Suggestions = append(result.Suggestions, "Valid categories: Personal, Business, Exchange, DeFi, Friends, Family")
		}
	}

	// Validate tags
	if len(contact.Tags) > 10 {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:    "tags",
			Code:     ErrorTooManyTags,
			Message:  "Too many tags (max 10)",
			Severity: ValidationSeverityWarning,
		})
	}
	for _, tag := range contact.Tags {
		if len(tag) > 20 {
			result.Warnings = append(result.Warnings, ValidationError{
				Field:    "tags",
				Code:     ErrorInvalidTag,
				Message:  fmt.Sprintf("Tag too long: %s (max 20 characters)", tag),
				Severity: ValidationSeverityWarning,
			})
		}
	}

	// Check for duplicates in existing contacts
	for _, existing := range existingContacts {
		if existing.ID != contact.ID { // Skip self-comparison for updates
			if strings.EqualFold(existing.Address, contact.Address) {
				result.Errors = append(result.Errors, ValidationError{
					Field:    "address",
					Code:     ErrorDuplicateAddress,
					Message:  fmt.Sprintf("Address already exists (Contact: %s)", existing.Name),
					Severity: ValidationSeverityError,
				})
				result.IsValid = false
			}
			if strings.EqualFold(existing.Name, contact.Name) {
				result.Warnings = append(result.Warnings, ValidationError{
					Field:    "name",
					Code:     ErrorDuplicateName,
					Message:  fmt.Sprintf("Name already exists (Address: %s)", existing.Address),
					Severity: ValidationSeverityWarning,
				})
			}
		}
	}

	// Cache the result
	v.cacheResult(contact, result)

	return result
}

// getCachedResult retrieves a cached validation result if available and not expired
func (v *ContactValidator) getCachedResult(contact *models.Contact) (ValidationResult, bool) {
	v.cacheMutex.RLock()
	defer v.cacheMutex.RUnlock()

	if cached, ok := v.cache[contact.ID]; ok {
		if time.Now().Before(cached.ExpiresAt) {
			return cached.Result, true
		}
		// Remove expired cache entry
		delete(v.cache, contact.ID)
	}
	return ValidationResult{}, false
}

// cacheResult stores a validation result in the cache
func (v *ContactValidator) cacheResult(contact *models.Contact, result ValidationResult) {
	v.cacheMutex.Lock()
	defer v.cacheMutex.Unlock()

	v.cache[contact.ID] = ValidationCache{
		Result:    result,
		ExpiresAt: time.Now().Add(v.cacheExpiry),
	}
}

// ClearCache clears all cached validation results
func (v *ContactValidator) ClearCache() {
	v.cacheMutex.Lock()
	defer v.cacheMutex.Unlock()

	v.cache = make(map[string]ValidationCache)
}

// SetCacheExpiry sets the cache expiry duration
func (v *ContactValidator) SetCacheExpiry(duration time.Duration) {
	v.cacheExpiry = duration
}
