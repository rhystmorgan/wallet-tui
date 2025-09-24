package blockchain

import (
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"
)

func TestNewBlockchainError(t *testing.T) {
	cause := errors.New("underlying error")
	err := NewBlockchainError(ErrNetworkConnection, "test message", cause)

	if err.Type != ErrNetworkConnection {
		t.Errorf("Expected type %s, got %s", ErrNetworkConnection, err.Type)
	}

	if err.Message != "test message" {
		t.Errorf("Expected message 'test message', got '%s'", err.Message)
	}

	if err.Cause != cause {
		t.Errorf("Expected cause %v, got %v", cause, err.Cause)
	}
}

func TestBlockchainErrorError(t *testing.T) {
	// Test without cause
	err := NewBlockchainError(ErrInvalidAddress, "invalid address", nil)
	expected := "invalid address"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}

	// Test with cause
	cause := errors.New("underlying error")
	err = NewBlockchainError(ErrNetworkConnection, "network failed", cause)
	expected = "network failed: underlying error"
	if err.Error() != expected {
		t.Errorf("Expected error message '%s', got '%s'", expected, err.Error())
	}
}

func TestNewInvalidAddressError(t *testing.T) {
	address := "0xinvalid"
	err := NewInvalidAddressError(address)

	if err.Type != ErrInvalidAddress {
		t.Errorf("Expected type %s, got %s", ErrInvalidAddress, err.Type)
	}

	if !strings.Contains(err.Message, address) {
		t.Errorf("Expected message to contain address '%s', got '%s'", address, err.Message)
	}
}

func TestNewInsufficientFundsError(t *testing.T) {
	required := big.NewInt(1000)
	available := big.NewInt(500)
	asset := VET

	err := NewInsufficientFundsError(required, available, asset)

	if err.Type != ErrInsufficientFunds {
		t.Errorf("Expected type %s, got %s", ErrInsufficientFunds, err.Type)
	}

	if !strings.Contains(err.Message, string(asset)) {
		t.Errorf("Expected message to contain asset '%s', got '%s'", asset, err.Message)
	}

	if !strings.Contains(err.Message, required.String()) {
		t.Errorf("Expected message to contain required amount '%s', got '%s'", required.String(), err.Message)
	}

	if !strings.Contains(err.Message, available.String()) {
		t.Errorf("Expected message to contain available amount '%s', got '%s'", available.String(), err.Message)
	}
}

func TestNewTimeoutError(t *testing.T) {
	operation := "test operation"
	timeout := 30 * time.Second

	err := NewTimeoutError(operation, timeout)

	if err.Type != ErrTimeout {
		t.Errorf("Expected type %s, got %s", ErrTimeout, err.Type)
	}

	if !strings.Contains(err.Message, operation) {
		t.Errorf("Expected message to contain operation '%s', got '%s'", operation, err.Message)
	}

	if !strings.Contains(err.Message, timeout.String()) {
		t.Errorf("Expected message to contain timeout '%s', got '%s'", timeout.String(), err.Message)
	}
}

func TestClassifyError(t *testing.T) {
	tests := []struct {
		input    error
		expected ErrorType
	}{
		{nil, ErrorType("")},
		{errors.New("timeout occurred"), ErrTimeout},
		{errors.New("deadline exceeded"), ErrTimeout},
		{errors.New("connection refused"), ErrNetworkConnection},
		{errors.New("no such host"), ErrNetworkConnection},
		{errors.New("invalid address format"), ErrInvalidAddress},
		{errors.New("bad address"), ErrInvalidAddress},
		{errors.New("insufficient funds"), ErrInsufficientFunds},
		{errors.New("not enough balance"), ErrInsufficientFunds},
		{errors.New("rate limit exceeded"), ErrRateLimited},
		{errors.New("too many requests"), ErrRateLimited},
		{errors.New("unknown error"), ErrNetworkConnection},
	}

	for _, test := range tests {
		result := ClassifyError(test.input)

		if test.input == nil {
			if result != nil {
				t.Errorf("Expected nil for nil input, got %v", result)
			}
			continue
		}

		if result.Type != test.expected {
			t.Errorf("For error '%s', expected type %s, got %s", test.input.Error(), test.expected, result.Type)
		}
	}
}

func TestClassifyNetError(t *testing.T) {
	// Create a mock net.Error that times out
	netErr := &mockNetError{timeout: true}

	result := ClassifyError(netErr)
	if result.Type != ErrTimeout {
		t.Errorf("Expected timeout error for net.Error with timeout, got %s", result.Type)
	}
}

func TestIsRetryable(t *testing.T) {
	retryableTypes := []ErrorType{
		ErrNetworkConnection,
		ErrNodeUnavailable,
		ErrTimeout,
		ErrRateLimited,
	}

	nonRetryableTypes := []ErrorType{
		ErrInvalidAddress,
		ErrInsufficientFunds,
		ErrTransactionFailed,
	}

	for _, errType := range retryableTypes {
		err := &BlockchainError{Type: errType}
		if !err.IsRetryable() {
			t.Errorf("Expected error type %s to be retryable", errType)
		}
	}

	for _, errType := range nonRetryableTypes {
		err := &BlockchainError{Type: errType}
		if err.IsRetryable() {
			t.Errorf("Expected error type %s to not be retryable", errType)
		}
	}
}

func TestUserMessage(t *testing.T) {
	tests := []struct {
		errType  ErrorType
		expected string
	}{
		{ErrNetworkConnection, "Network connection failed"},
		{ErrInvalidAddress, "Invalid VeChain address format"},
		{ErrInsufficientFunds, "Insufficient funds"},
		{ErrTransactionFailed, "Transaction failed to process"},
		{ErrNodeUnavailable, "VeChain network is temporarily unavailable"},
		{ErrRateLimited, "Too many requests"},
		{ErrTimeout, "Request timed out"},
		{ErrorType("unknown"), "An unexpected error occurred"},
	}

	for _, test := range tests {
		err := &BlockchainError{Type: test.errType}
		message := err.UserMessage()

		if !strings.Contains(message, test.expected) {
			t.Errorf("For error type %s, expected message to contain '%s', got '%s'", test.errType, test.expected, message)
		}
	}
}

// Mock net.Error for testing
type mockNetError struct {
	timeout bool
}

func (e *mockNetError) Error() string {
	return "mock network error"
}

func (e *mockNetError) Timeout() bool {
	return e.timeout
}

func (e *mockNetError) Temporary() bool {
	return false
}
