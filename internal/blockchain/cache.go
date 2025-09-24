package blockchain

import (
	"math/big"
	"time"
)

func NewBalanceCache(ttl time.Duration) *BalanceCache {
	return &BalanceCache{
		balances: make(map[string]*Balance),
		ttl:      ttl,
	}
}

func (c *BalanceCache) Get(address string) (*Balance, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	balance, exists := c.balances[address]
	if !exists {
		return nil, false
	}

	if time.Since(balance.LastUpdated) > c.ttl {
		return nil, false
	}

	return &Balance{
		VET:         new(big.Int).Set(balance.VET),
		VTHO:        new(big.Int).Set(balance.VTHO),
		LastUpdated: balance.LastUpdated,
	}, true
}

func (c *BalanceCache) Set(address string, balance *Balance) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.balances[address] = &Balance{
		VET:         new(big.Int).Set(balance.VET),
		VTHO:        new(big.Int).Set(balance.VTHO),
		LastUpdated: time.Now(),
	}
}

func (c *BalanceCache) Invalidate(address string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.balances, address)
}

func (c *BalanceCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.balances = make(map[string]*Balance)
}

func (c *BalanceCache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for address, balance := range c.balances {
		if now.Sub(balance.LastUpdated) > c.ttl {
			delete(c.balances, address)
		}
	}
}

func (c *BalanceCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.balances)
}

func (c *BalanceCache) IsExpired(address string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	balance, exists := c.balances[address]
	if !exists {
		return true
	}

	return time.Since(balance.LastUpdated) > c.ttl
}

func (c *BalanceCache) StartCleanupRoutine(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			c.Cleanup()
		}
	}()
}
