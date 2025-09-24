package blockchain

import (
	"math/big"
	"testing"
	"time"
)

func TestNewBalanceCache(t *testing.T) {
	ttl := 30 * time.Second
	cache := NewBalanceCache(ttl)

	if cache == nil {
		t.Fatal("Cache is nil")
	}

	if cache.ttl != ttl {
		t.Errorf("Expected TTL %v, got %v", ttl, cache.ttl)
	}

	if cache.balances == nil {
		t.Error("Balances map should be initialized")
	}
}

func TestCacheSetAndGet(t *testing.T) {
	cache := NewBalanceCache(30 * time.Second)
	address := "0x1234567890123456789012345678901234567890"

	balance := &Balance{
		VET:         big.NewInt(1000000000000000000),
		VTHO:        big.NewInt(500000000000000000),
		LastUpdated: time.Now(),
	}

	// Test cache miss
	_, found := cache.Get(address)
	if found {
		t.Error("Expected cache miss for new address")
	}

	// Test cache set
	cache.Set(address, balance)

	// Test cache hit
	cachedBalance, found := cache.Get(address)
	if !found {
		t.Error("Expected cache hit after setting")
	}

	if cachedBalance.VET.Cmp(balance.VET) != 0 {
		t.Errorf("Expected VET %s, got %s", balance.VET.String(), cachedBalance.VET.String())
	}

	if cachedBalance.VTHO.Cmp(balance.VTHO) != 0 {
		t.Errorf("Expected VTHO %s, got %s", balance.VTHO.String(), cachedBalance.VTHO.String())
	}
}

func TestCacheExpiration(t *testing.T) {
	cache := NewBalanceCache(100 * time.Millisecond)
	address := "0x1234567890123456789012345678901234567890"

	balance := &Balance{
		VET:         big.NewInt(1000000000000000000),
		VTHO:        big.NewInt(500000000000000000),
		LastUpdated: time.Now(),
	}

	cache.Set(address, balance)

	// Should be found immediately
	_, found := cache.Get(address)
	if !found {
		t.Error("Expected cache hit immediately after setting")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired now
	_, found = cache.Get(address)
	if found {
		t.Error("Expected cache miss after expiration")
	}
}

func TestCacheInvalidate(t *testing.T) {
	cache := NewBalanceCache(30 * time.Second)
	address := "0x1234567890123456789012345678901234567890"

	balance := &Balance{
		VET:         big.NewInt(1000000000000000000),
		VTHO:        big.NewInt(500000000000000000),
		LastUpdated: time.Now(),
	}

	cache.Set(address, balance)

	// Should be found
	_, found := cache.Get(address)
	if !found {
		t.Error("Expected cache hit after setting")
	}

	// Invalidate
	cache.Invalidate(address)

	// Should be gone
	_, found = cache.Get(address)
	if found {
		t.Error("Expected cache miss after invalidation")
	}
}

func TestCacheClear(t *testing.T) {
	cache := NewBalanceCache(30 * time.Second)

	addresses := []string{
		"0x1234567890123456789012345678901234567890",
		"0x0987654321098765432109876543210987654321",
		"0x1111111111111111111111111111111111111111",
	}

	balance := &Balance{
		VET:         big.NewInt(1000000000000000000),
		VTHO:        big.NewInt(500000000000000000),
		LastUpdated: time.Now(),
	}

	// Set multiple entries
	for _, addr := range addresses {
		cache.Set(addr, balance)
	}

	// Verify all are present
	for _, addr := range addresses {
		_, found := cache.Get(addr)
		if !found {
			t.Errorf("Expected cache hit for address %s", addr)
		}
	}

	// Clear cache
	cache.Clear()

	// Verify all are gone
	for _, addr := range addresses {
		_, found := cache.Get(addr)
		if found {
			t.Errorf("Expected cache miss for address %s after clear", addr)
		}
	}
}

func TestCacheSize(t *testing.T) {
	cache := NewBalanceCache(30 * time.Second)

	if cache.Size() != 0 {
		t.Errorf("Expected size 0 for empty cache, got %d", cache.Size())
	}

	balance := &Balance{
		VET:         big.NewInt(1000000000000000000),
		VTHO:        big.NewInt(500000000000000000),
		LastUpdated: time.Now(),
	}

	addresses := []string{
		"0x1234567890123456789012345678901234567890",
		"0x0987654321098765432109876543210987654321",
	}

	for i, addr := range addresses {
		cache.Set(addr, balance)
		expectedSize := i + 1
		if cache.Size() != expectedSize {
			t.Errorf("Expected size %d after adding %d entries, got %d", expectedSize, expectedSize, cache.Size())
		}
	}
}

func TestCacheIsExpired(t *testing.T) {
	cache := NewBalanceCache(100 * time.Millisecond)
	address := "0x1234567890123456789012345678901234567890"

	// Should be expired for non-existent entry
	if !cache.IsExpired(address) {
		t.Error("Expected expired for non-existent entry")
	}

	balance := &Balance{
		VET:         big.NewInt(1000000000000000000),
		VTHO:        big.NewInt(500000000000000000),
		LastUpdated: time.Now(),
	}

	cache.Set(address, balance)

	// Should not be expired immediately
	if cache.IsExpired(address) {
		t.Error("Expected not expired immediately after setting")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should be expired now
	if !cache.IsExpired(address) {
		t.Error("Expected expired after TTL")
	}
}

func TestCacheCleanup(t *testing.T) {
	cache := NewBalanceCache(100 * time.Millisecond)

	balance := &Balance{
		VET:         big.NewInt(1000000000000000000),
		VTHO:        big.NewInt(500000000000000000),
		LastUpdated: time.Now(),
	}

	// Add some entries
	cache.Set("addr1", balance)
	cache.Set("addr2", balance)
	cache.Set("addr3", balance)

	if cache.Size() != 3 {
		t.Errorf("Expected size 3, got %d", cache.Size())
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Run cleanup
	cache.Cleanup()

	// All entries should be cleaned up
	if cache.Size() != 0 {
		t.Errorf("Expected size 0 after cleanup, got %d", cache.Size())
	}
}
