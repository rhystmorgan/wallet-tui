package models

import (
	"math/big"
	"testing"
	"time"
)

func TestWalletSetBalance(t *testing.T) {
	wallet := &Wallet{
		ID:      "test-wallet",
		Name:    "Test Wallet",
		Address: "0x1234567890123456789012345678901234567890",
	}

	vetBalance := big.NewInt(1000000000000000000) // 1 VET in wei
	vthoBalance := big.NewInt(500000000000000000) // 0.5 VTHO in wei

	wallet.SetBalance(vetBalance, vthoBalance)

	if wallet.CachedBalance == nil {
		t.Fatal("CachedBalance should not be nil after SetBalance")
	}

	if wallet.CachedBalance.VET.Cmp(vetBalance) != 0 {
		t.Errorf("Expected VET balance %s, got %s", vetBalance.String(), wallet.CachedBalance.VET.String())
	}

	if wallet.CachedBalance.VTHO.Cmp(vthoBalance) != 0 {
		t.Errorf("Expected VTHO balance %s, got %s", vthoBalance.String(), wallet.CachedBalance.VTHO.String())
	}

	if wallet.LastSync.IsZero() {
		t.Error("LastSync should be set after SetBalance")
	}

	if wallet.CachedBalance.LastUpdated.IsZero() {
		t.Error("LastUpdated should be set after SetBalance")
	}
}

func TestWalletGetDisplayBalance(t *testing.T) {
	wallet := &Wallet{
		ID:      "test-wallet",
		Name:    "Test Wallet",
		Address: "0x1234567890123456789012345678901234567890",
	}

	// Test with no balance set
	vetDisplay, vthoDisplay := wallet.GetDisplayBalance()
	if vetDisplay != "0" {
		t.Errorf("Expected VET display '0', got '%s'", vetDisplay)
	}
	if vthoDisplay != "0" {
		t.Errorf("Expected VTHO display '0', got '%s'", vthoDisplay)
	}

	// Test with balance set
	vetBalance := big.NewInt(1000000000000000000) // 1 VET in wei
	vthoBalance := big.NewInt(500000000000000000) // 0.5 VTHO in wei

	wallet.SetBalance(vetBalance, vthoBalance)

	vetDisplay, vthoDisplay = wallet.GetDisplayBalance()
	if vetDisplay != "1.0000" {
		t.Errorf("Expected VET display '1.0000', got '%s'", vetDisplay)
	}
	if vthoDisplay != "0.5000" {
		t.Errorf("Expected VTHO display '0.5000', got '%s'", vthoDisplay)
	}

	// Test with larger amounts
	vetBalance = big.NewInt(0).Mul(big.NewInt(12345), big.NewInt(1000000000000000000)) // 12345 VET
	vthoBalance = big.NewInt(0).Mul(big.NewInt(6789), big.NewInt(1000000000000000000)) // 6789 VTHO

	wallet.SetBalance(vetBalance, vthoBalance)

	vetDisplay, vthoDisplay = wallet.GetDisplayBalance()
	if vetDisplay != "12345.0000" {
		t.Errorf("Expected VET display '12345.0000', got '%s'", vetDisplay)
	}
	if vthoDisplay != "6789.0000" {
		t.Errorf("Expected VTHO display '6789.0000', got '%s'", vthoDisplay)
	}
}

func TestWalletNeedsBalanceRefresh(t *testing.T) {
	wallet := &Wallet{
		ID:      "test-wallet",
		Name:    "Test Wallet",
		Address: "0x1234567890123456789012345678901234567890",
	}

	// Test with no balance set
	if !wallet.NeedsBalanceRefresh() {
		t.Error("Expected wallet to need balance refresh when no balance is set")
	}

	// Test with fresh balance
	vetBalance := big.NewInt(1000000000000000000)
	vthoBalance := big.NewInt(500000000000000000)
	wallet.SetBalance(vetBalance, vthoBalance)

	if wallet.NeedsBalanceRefresh() {
		t.Error("Expected wallet to not need balance refresh when balance is fresh")
	}

	// Test with old balance
	wallet.CachedBalance.LastUpdated = time.Now().Add(-31 * time.Second) // 31 seconds ago

	if !wallet.NeedsBalanceRefresh() {
		t.Error("Expected wallet to need balance refresh when balance is old")
	}
}

func TestWalletGetBalanceAge(t *testing.T) {
	wallet := &Wallet{
		ID:      "test-wallet",
		Name:    "Test Wallet",
		Address: "0x1234567890123456789012345678901234567890",
	}

	// Test with no balance set
	age := wallet.GetBalanceAge()
	if age != 0 {
		t.Errorf("Expected balance age 0 when no balance is set, got %v", age)
	}

	// Test with balance set
	vetBalance := big.NewInt(1000000000000000000)
	vthoBalance := big.NewInt(500000000000000000)
	wallet.SetBalance(vetBalance, vthoBalance)

	age = wallet.GetBalanceAge()
	if age < 0 {
		t.Errorf("Expected positive balance age, got %v", age)
	}

	if age > 1*time.Second {
		t.Errorf("Expected balance age to be very small (just set), got %v", age)
	}

	// Test with old balance
	oldTime := time.Now().Add(-5 * time.Minute)
	wallet.CachedBalance.LastUpdated = oldTime

	age = wallet.GetBalanceAge()

	// Allow for some variance in timing (should be around 5 minutes)
	if age < 4*time.Minute || age > 6*time.Minute {
		t.Errorf("Expected balance age around 5 minutes, got %v", age)
	}
}

func TestWalletClearBalance(t *testing.T) {
	wallet := &Wallet{
		ID:      "test-wallet",
		Name:    "Test Wallet",
		Address: "0x1234567890123456789012345678901234567890",
	}

	// Set balance first
	vetBalance := big.NewInt(1000000000000000000)
	vthoBalance := big.NewInt(500000000000000000)
	wallet.SetBalance(vetBalance, vthoBalance)

	if wallet.CachedBalance == nil {
		t.Fatal("CachedBalance should not be nil after SetBalance")
	}

	// Clear balance
	wallet.ClearBalance()

	if wallet.CachedBalance != nil {
		t.Error("CachedBalance should be nil after ClearBalance")
	}

	// Test that GetDisplayBalance returns "0" after clearing
	vetDisplay, vthoDisplay := wallet.GetDisplayBalance()
	if vetDisplay != "0" {
		t.Errorf("Expected VET display '0' after clear, got '%s'", vetDisplay)
	}
	if vthoDisplay != "0" {
		t.Errorf("Expected VTHO display '0' after clear, got '%s'", vthoDisplay)
	}
}

func TestNewWallet(t *testing.T) {
	// This test requires a valid mnemonic
	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	name := "Test Wallet"

	wallet, err := NewWallet(name, mnemonic)
	if err != nil {
		t.Fatalf("Failed to create wallet: %v", err)
	}

	if wallet.Name != name {
		t.Errorf("Expected wallet name '%s', got '%s'", name, wallet.Name)
	}

	if wallet.Mnemonic != mnemonic {
		t.Errorf("Expected wallet mnemonic '%s', got '%s'", mnemonic, wallet.Mnemonic)
	}

	if wallet.Address == "" {
		t.Error("Wallet address should not be empty")
	}

	if wallet.PrivateKey == nil {
		t.Error("Wallet private key should not be nil")
	}

	if wallet.ID == "" {
		t.Error("Wallet ID should not be empty")
	}

	if wallet.CreatedAt.IsZero() {
		t.Error("Wallet CreatedAt should not be zero")
	}

	// Test that CachedBalance is initially nil
	if wallet.CachedBalance != nil {
		t.Error("CachedBalance should be nil for new wallet")
	}

	// Test that LastSync is initially zero
	if !wallet.LastSync.IsZero() {
		t.Error("LastSync should be zero for new wallet")
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	time.Sleep(1 * time.Second) // Ensure different timestamp (seconds precision)
	id2 := generateID()

	if id1 == "" {
		t.Error("Generated ID should not be empty")
	}

	if id2 == "" {
		t.Error("Generated ID should not be empty")
	}

	if id1 == id2 {
		t.Error("Generated IDs should be different")
	}

	// Test ID format (should be timestamp-based)
	if len(id1) != 14 { // YYYYMMDDHHMMSS format
		t.Errorf("Expected ID length 14, got %d", len(id1))
	}
}
