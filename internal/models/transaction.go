package models

import (
	"math/big"
	"time"
)

type TransactionStatus string

const (
	TransactionStatusPending   TransactionStatus = "pending"
	TransactionStatusConfirmed TransactionStatus = "confirmed"
	TransactionStatusFailed    TransactionStatus = "failed"
	TransactionStatusReverted  TransactionStatus = "reverted"
)

func (s TransactionStatus) String() string {
	switch s {
	case TransactionStatusPending:
		return "Pending"
	case TransactionStatusConfirmed:
		return "Confirmed"
	case TransactionStatusFailed:
		return "Failed"
	case TransactionStatusReverted:
		return "Reverted"
	default:
		return "Unknown"
	}
}

type TransactionDirection string

const (
	TransactionDirectionSent     TransactionDirection = "sent"
	TransactionDirectionReceived TransactionDirection = "received"
	TransactionDirectionSelf     TransactionDirection = "self"
)

func (d TransactionDirection) String() string {
	switch d {
	case TransactionDirectionSent:
		return "Sent"
	case TransactionDirectionReceived:
		return "Received"
	case TransactionDirectionSelf:
		return "Self"
	default:
		return "Unknown"
	}
}

type Transaction struct {
	ID        string    `json:"id"`
	Hash      string    `json:"hash"`
	From      string    `json:"from"`
	To        string    `json:"to"`
	Amount    *big.Int  `json:"amount"`
	Asset     string    `json:"asset"`
	Timestamp time.Time `json:"timestamp"`

	// Enhanced fields for Phase 3
	Status         TransactionStatus    `json:"status"`
	Confirmations  int                  `json:"confirmations"`
	GasUsed        *big.Int             `json:"gas_used,omitempty"`
	GasPrice       *big.Int             `json:"gas_price,omitempty"`
	BlockNumber    uint64               `json:"block_number,omitempty"`
	BlockHash      string               `json:"block_hash,omitempty"`
	TransactionFee *big.Int             `json:"transaction_fee,omitempty"`
	Direction      TransactionDirection `json:"direction"`
	ContactName    string               `json:"contact_name,omitempty"`
	Notes          string               `json:"notes,omitempty"`
	Tags           []string             `json:"tags,omitempty"`

	// Legacy fields for backward compatibility
	Gas  uint64 `json:"gas,omitempty"`
	Data []byte `json:"data,omitempty"`
	Memo string `json:"memo,omitempty"`
}

type TransactionHistory struct {
	Transactions  []Transaction `json:"transactions"`
	TotalCount    int           `json:"total_count"`
	CurrentPage   int           `json:"current_page"`
	PageSize      int           `json:"page_size"`
	HasMore       bool          `json:"has_more"`
	LastFetch     time.Time     `json:"last_fetch"`
	FilteredCount int           `json:"filtered_count"`
}

type TransactionFilter struct {
	Direction    TransactionDirection `json:"direction,omitempty"`
	Status       TransactionStatus    `json:"status,omitempty"`
	Asset        string               `json:"asset,omitempty"`
	DateFrom     time.Time            `json:"date_from,omitempty"`
	DateTo       time.Time            `json:"date_to,omitempty"`
	AmountMin    *big.Int             `json:"amount_min,omitempty"`
	AmountMax    *big.Int             `json:"amount_max,omitempty"`
	SearchQuery  string               `json:"search_query,omitempty"`
	ContactsOnly bool                 `json:"contacts_only,omitempty"`
}

type TransactionRequest struct {
	To     string   `json:"to"`
	Amount *big.Int `json:"amount"`
	Asset  string   `json:"asset"`
	Memo   string   `json:"memo,omitempty"`
	Gas    uint64   `json:"gas,omitempty"`
}

func NewTransaction(from, to string, amount *big.Int, asset string) *Transaction {
	return &Transaction{
		ID:        generateTransactionID(),
		From:      from,
		To:        to,
		Amount:    amount,
		Asset:     asset,
		Status:    TransactionStatusPending,
		Timestamp: time.Now(),
	}
}

func generateTransactionID() string {
	return "tx_" + time.Now().Format("20060102150405") + "_" + generateRandomString(8)
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(result)
}
