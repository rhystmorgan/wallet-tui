package validation

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"rhystmorgan/veWallet/internal/blockchain"

	"github.com/ethereum/go-ethereum/common"
)

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
func (v *ContactValidator) ValidateContact(name, address, category string, tags []string, existingAddresses []string) ValidationResult {
	result := ValidationResult{
		IsValid:     true,
		ValidatedAt: time.Now(),
	}

	// Validate required fields
	if strings.TrimSpace(name) == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:    "name",
			Code:     ErrorNameRequired,
			Message:  "Name is required",
			Severity: ValidationSeverityError,
		})
		result.IsValid = false
	}

	if strings.TrimSpace(address) == "" {
		result.Errors = append(result.Errors, ValidationError{
			Field:    "address",
			Code:     ErrorAddressRequired,
			Message:  "Address is required",
			Severity: ValidationSeverityError,
		})
		result.IsValid = false
	}

	// Validate field lengths
	if len(name) > 50 {
		result.Errors = append(result.Errors, ValidationError{
			Field:    "name",
			Code:     ErrorNameTooLong,
			Message:  "Name too long (max 50 characters)",
			Severity: ValidationSeverityError,
		})
		result.IsValid = false
	}

	// Validate address format and checksum
	if address != "" {
		if !common.IsHexAddress(address) {
			result.Errors = append(result.Errors, ValidationError{
				Field:    "address",
				Code:     ErrorInvalidAddress,
				Message:  "Invalid VeChain address format",
				Severity: ValidationSeverityError,
			})
			result.IsValid = false
		} else {
			// Verify checksum
			checksumAddr := common.HexToAddress(address).Hex()
			if checksumAddr != address {
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
				if exists, err := v.blockchainClient.AddressExists(address); err != nil {
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
	if category != "" {
		validCategories := []string{"Personal", "Business", "Exchange", "DeFi", "Friends", "Family"}
		isValidCategory := false
		for _, cat := range validCategories {
			if strings.EqualFold(category, cat) {
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
	if len(tags) > 10 {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:    "tags",
			Code:     ErrorTooManyTags,
			Message:  "Too many tags (max 10)",
			Severity: ValidationSeverityWarning,
		})
	}
	for _, tag := range tags {
		if len(tag) > 20 {
			result.Warnings = append(result.Warnings, ValidationError{
				Field:    "tags",
				Code:     ErrorInvalidTag,
				Message:  fmt.Sprintf("Tag too long: %s (max 20 characters)", tag),
				Severity: ValidationSeverityWarning,
			})
		}
	}

	// Check for duplicates in existing addresses
	for _, existingAddr := range existingAddresses {
		if strings.EqualFold(existingAddr, address) {
			result.Errors = append(result.Errors, ValidationError{
				Field:    "address",
				Code:     ErrorDuplicateAddress,
				Message:  "Address already exists",
				Severity: ValidationSeverityError,
			})
			result.IsValid = false
			break
		}
	}

	return result
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
