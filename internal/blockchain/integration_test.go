//go:build integration
// +build integration

package blockchain

import (
	"math/big"
	"testing"
	"time"
)

// These tests require network connectivity and should be run with:
// go test -tags=integration ./internal/blockchain

func TestIntegrationNewClient(t *testing.T) {
	config := Config{
		Network:    TestNet,
		NodeURL:    "",
		Timeout:    10 * time.Second,
		RetryCount: 1,
		RetryDelay: 1 * time.Second,
	}

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	if client == nil {
		t.Fatal("Client is nil")
	}

	status := client.GetStatus()
	if !status.Connected {
		t.Error("Expected client to be connected")
	}

	if status.BlockHeight == 0 {
		t.Error("Expected block height to be greater than 0")
	}
}

func TestIntegrationGetBalance(t *testing.T) {
	config := Config{
		Network:    TestNet,
		Timeout:    10 * time.Second,
		RetryCount: 1,
		RetryDelay: 1 * time.Second,
	}

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Use a known testnet address (this might have 0 balance, which is fine)
	address := "0x0000000000000000000000000000000000000000"

	balance, err := client.GetBalance(address)
	if err != nil {
		t.Fatalf("Failed to get balance: %v", err)
	}

	if balance == nil {
		t.Fatal("Balance is nil")
	}

	if balance.VET == nil {
		t.Error("VET balance is nil")
	}

	if balance.VTHO == nil {
		t.Error("VTHO balance is nil")
	}

	// Test caching
	cachedBalance, found := client.GetCachedBalance(address)
	if !found {
		t.Error("Expected cached balance to be found")
	}

	if cachedBalance.VET.Cmp(balance.VET) != 0 {
		t.Error("Cached VET balance doesn't match")
	}

	if cachedBalance.VTHO.Cmp(balance.VTHO) != 0 {
		t.Error("Cached VTHO balance doesn't match")
	}
}

func TestIntegrationRefreshBalance(t *testing.T) {
	config := Config{
		Network:    TestNet,
		Timeout:    10 * time.Second,
		RetryCount: 1,
		RetryDelay: 1 * time.Second,
	}

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	address := "0x0000000000000000000000000000000000000000"

	// Get initial balance
	balance1, err := client.GetBalance(address)
	if err != nil {
		t.Fatalf("Failed to get initial balance: %v", err)
	}

	// Refresh balance
	balance2, err := client.RefreshBalance(address)
	if err != nil {
		t.Fatalf("Failed to refresh balance: %v", err)
	}

	// Balances should be the same (assuming no transactions)
	if balance1.VET.Cmp(balance2.VET) != 0 {
		t.Error("VET balance changed unexpectedly")
	}

	if balance1.VTHO.Cmp(balance2.VTHO) != 0 {
		t.Error("VTHO balance changed unexpectedly")
	}

	// LastUpdated should be different
	if balance1.LastUpdated.Equal(balance2.LastUpdated) {
		t.Error("LastUpdated should be different after refresh")
	}
}
