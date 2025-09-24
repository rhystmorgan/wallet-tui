package validation

import (
	"time"
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
