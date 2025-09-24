package blockchain

import (
	"fmt"
	"math/big"
	"net"
	"strings"
	"time"
)

func NewBlockchainError(errType ErrorType, message string, cause error) *BlockchainError {
	return &BlockchainError{
		Type:    errType,
		Message: message,
		Cause:   cause,
	}
}

func NewNetworkError(message string, cause error) *BlockchainError {
	return NewBlockchainError(ErrNetworkConnection, message, cause)
}

func NewInvalidAddressError(address string) *BlockchainError {
	return NewBlockchainError(ErrInvalidAddress, fmt.Sprintf("invalid address: %s", address), nil)
}

func NewInsufficientFundsError(required, available *big.Int, asset AssetType) *BlockchainError {
	return NewBlockchainError(ErrInsufficientFunds,
		fmt.Sprintf("insufficient %s: required %s, available %s", asset, required.String(), available.String()), nil)
}

func NewTimeoutError(operation string, timeout time.Duration) *BlockchainError {
	return NewBlockchainError(ErrTimeout,
		fmt.Sprintf("operation %s timed out after %v", operation, timeout), nil)
}

func NewNodeUnavailableError(nodeURL string, cause error) *BlockchainError {
	return NewBlockchainError(ErrNodeUnavailable,
		fmt.Sprintf("node unavailable: %s", nodeURL), cause)
}

func NewRateLimitedError(retryAfter time.Duration) *BlockchainError {
	return NewBlockchainError(ErrRateLimited,
		fmt.Sprintf("rate limited, retry after %v", retryAfter), nil)
}

func NewTransactionFailedError(txID string, reason string) *BlockchainError {
	return NewBlockchainError(ErrTransactionFailed,
		fmt.Sprintf("transaction %s failed: %s", txID, reason), nil)
}

func ClassifyError(err error) *BlockchainError {
	if err == nil {
		return nil
	}

	if blockchainErr, ok := err.(*BlockchainError); ok {
		return blockchainErr
	}

	errStr := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded"):
		return NewTimeoutError("network request", 30*time.Second)
	case strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "no such host"):
		return NewNetworkError("connection failed", err)
	case strings.Contains(errStr, "invalid address") || strings.Contains(errStr, "bad address"):
		return NewBlockchainError(ErrInvalidAddress, "invalid address format", err)
	case strings.Contains(errStr, "insufficient") || strings.Contains(errStr, "not enough"):
		return NewBlockchainError(ErrInsufficientFunds, "insufficient funds", err)
	case strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "too many requests"):
		return NewRateLimitedError(time.Minute)
	default:
		if netErr, ok := err.(net.Error); ok {
			if netErr.Timeout() {
				return NewTimeoutError("network operation", 30*time.Second)
			}
		}
		return NewNetworkError("unknown network error", err)
	}
}

func (e *BlockchainError) IsRetryable() bool {
	switch e.Type {
	case ErrNetworkConnection, ErrNodeUnavailable, ErrTimeout, ErrRateLimited:
		return true
	default:
		return false
	}
}

func (e *BlockchainError) UserMessage() string {
	switch e.Type {
	case ErrNetworkConnection:
		return "Network connection failed. Please check your internet connection."
	case ErrInvalidAddress:
		return "Invalid VeChain address format."
	case ErrInsufficientFunds:
		return "Insufficient funds for this transaction."
	case ErrTransactionFailed:
		return "Transaction failed to process."
	case ErrNodeUnavailable:
		return "VeChain network is temporarily unavailable."
	case ErrRateLimited:
		return "Too many requests. Please wait a moment and try again."
	case ErrTimeout:
		return "Request timed out. Please try again."
	default:
		return "An unexpected error occurred."
	}
}
