package blockchain

import (
	"math/big"
	"sync"
	"time"
)

type Network string

const (
	MainNet Network = "mainnet"
	TestNet Network = "testnet"
)

type Config struct {
	Network    Network
	NodeURL    string
	Timeout    time.Duration
	RetryCount int
	RetryDelay time.Duration
}

type Balance struct {
	VET         *big.Int
	VTHO        *big.Int
	LastUpdated time.Time
}

type BalanceCache struct {
	balances map[string]*Balance
	mu       sync.RWMutex
	ttl      time.Duration
}

type AssetType string

const (
	VET  AssetType = "VET"
	VTHO AssetType = "VTHO"
)

type Transaction struct {
	From      string
	To        string
	Amount    *big.Int
	Asset     AssetType
	GasLimit  *big.Int
	GasPrice  *big.Int
	Nonce     uint64
	Signature []byte
	TxID      string
	Status    TransactionStatus
}

type TransactionStatus string

const (
	StatusPending   TransactionStatus = "pending"
	StatusConfirmed TransactionStatus = "confirmed"
	StatusFailed    TransactionStatus = "failed"
	StatusReverted  TransactionStatus = "reverted"
)

type ErrorType string

const (
	ErrNetworkConnection ErrorType = "network_connection"
	ErrInvalidAddress    ErrorType = "invalid_address"
	ErrInsufficientFunds ErrorType = "insufficient_funds"
	ErrTransactionFailed ErrorType = "transaction_failed"
	ErrNodeUnavailable   ErrorType = "node_unavailable"
	ErrRateLimited       ErrorType = "rate_limited"
	ErrTimeout           ErrorType = "timeout"
)

type BlockchainError struct {
	Type    ErrorType
	Message string
	Code    int
	Cause   error
}

func (e *BlockchainError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

type NetworkStatus struct {
	Connected   bool
	NodeURL     string
	LastChecked time.Time
	BlockHeight uint64
	NetworkID   string
}
