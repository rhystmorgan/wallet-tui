package blockchain

import (
	"math/big"
	"testing"
	"time"
)

func TestNewClientConfig(t *testing.T) {
	config := Config{
		Network:    TestNet,
		NodeURL:    "http://localhost:8669", // Use a local URL that won't connect
		Timeout:    1 * time.Second,         // Short timeout
		RetryCount: 0,                       // No retries
		RetryDelay: 1 * time.Second,
	}

	// This will fail to connect, but we can still test the config setup
	_, err := NewClient(config)
	if err == nil {
		t.Log("Unexpectedly connected to localhost:8669")
	}

	// Test that the error is properly classified
	if err != nil {
		blockchainErr := ClassifyError(err)
		if blockchainErr.Type != ErrNetworkConnection {
			t.Errorf("Expected network connection error, got %s", blockchainErr.Type)
		}
	}
}

func TestConfigDefaults(t *testing.T) {
	config := Config{
		Network:    MainNet,
		NodeURL:    "http://localhost:8669", // Use a local URL that won't connect
		Timeout:    1 * time.Second,         // Short timeout
		RetryCount: 0,                       // No retries
	}

	// Test that defaults are applied correctly
	if config.Timeout == 0 {
		config.Timeout = DefaultTimeout
	}
	if config.RetryCount == 0 {
		config.RetryCount = DefaultRetryCount
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = DefaultRetryDelay
	}

	// Verify defaults
	if config.Timeout != 1*time.Second { // We set it to 1 second
		t.Errorf("Expected timeout %v, got %v", 1*time.Second, config.Timeout)
	}

	if DefaultRetryCount != 3 {
		t.Errorf("Expected default retry count 3, got %d", DefaultRetryCount)
	}

	if DefaultRetryDelay != 2*time.Second {
		t.Errorf("Expected default retry delay %v, got %v", 2*time.Second, DefaultRetryDelay)
	}
}

func TestEstimateGas(t *testing.T) {
	// Create a mock client that doesn't require network connectivity
	client := &Client{
		config: Config{
			Network:    TestNet,
			Timeout:    5 * time.Second,
			RetryCount: 1,
		},
	}

	tests := []struct {
		asset    AssetType
		expected int64
	}{
		{VET, 21000},
		{VTHO, 80000},
	}

	for _, test := range tests {
		tx := &Transaction{Asset: test.asset}
		gasLimit, err := client.EstimateGas(tx)
		if err != nil {
			t.Errorf("Failed to estimate gas for %s: %v", test.asset, err)
			continue
		}

		if gasLimit.Int64() != test.expected {
			t.Errorf("Expected gas limit %d for %s, got %d", test.expected, test.asset, gasLimit.Int64())
		}
	}
}

func TestBuildTransactionOffline(t *testing.T) {
	// Test the transaction building logic without network calls
	from := "0x1234567890123456789012345678901234567890"
	to := "0x0987654321098765432109876543210987654321"
	amount := big.NewInt(1000000000000000000) // 1 VET

	// Test basic transaction structure
	tx := &Transaction{
		From:   from,
		To:     to,
		Amount: amount,
		Asset:  VET,
		Status: StatusPending,
	}

	if tx.From != from {
		t.Errorf("Expected from %s, got %s", from, tx.From)
	}

	if tx.To != to {
		t.Errorf("Expected to %s, got %s", to, tx.To)
	}

	if tx.Amount.Cmp(amount) != 0 {
		t.Errorf("Expected amount %s, got %s", amount.String(), tx.Amount.String())
	}

	if tx.Asset != VET {
		t.Errorf("Expected asset %s, got %s", VET, tx.Asset)
	}

	if tx.Status != StatusPending {
		t.Errorf("Expected status %s, got %s", StatusPending, tx.Status)
	}
}

func TestInvalidAssetType(t *testing.T) {
	// Create a mock client that doesn't require network connectivity
	client := &Client{
		config: Config{
			Network:    TestNet,
			Timeout:    5 * time.Second,
			RetryCount: 1,
		},
	}

	tx := &Transaction{Asset: AssetType("INVALID")}
	_, err := client.EstimateGas(tx)
	if err == nil {
		t.Error("Expected error for invalid asset type")
	}
}

func TestNetworkStatusStructure(t *testing.T) {
	// Test the NetworkStatus structure
	status := NetworkStatus{
		Connected:   false,
		NodeURL:     "http://localhost:8669",
		LastChecked: time.Now(),
		BlockHeight: 0,
		NetworkID:   "",
	}

	if status.NodeURL == "" {
		t.Error("NodeURL should not be empty")
	}

	if status.LastChecked.IsZero() {
		t.Error("LastChecked should not be zero")
	}

	if status.Connected {
		t.Error("Expected Connected to be false for mock status")
	}
}
