package models

import (
	"math/big"
	"sort"
	"strings"
	"time"
)

type RecentAddress struct {
	Address     string    `json:"address"`
	ContactName string    `json:"contact_name,omitempty"`
	LastUsed    time.Time `json:"last_used"`
	UseCount    int       `json:"use_count"`
	LastAmount  *big.Int  `json:"last_amount,omitempty"`
	LastAsset   string    `json:"last_asset,omitempty"`
	Frequency   float64   `json:"frequency"`
}

type RecentAddressManager struct {
	addresses  []RecentAddress `json:"addresses"`
	maxEntries int             `json:"max_entries"`
}

func NewRecentAddressManager(maxEntries int) *RecentAddressManager {
	if maxEntries <= 0 {
		maxEntries = 50 // Default maximum entries
	}

	return &RecentAddressManager{
		addresses:  make([]RecentAddress, 0),
		maxEntries: maxEntries,
	}
}

func (ram *RecentAddressManager) AddAddress(address, contactName, asset string, amount *big.Int) {
	address = strings.TrimSpace(address)
	if address == "" {
		return
	}

	now := time.Now()

	// Find existing address
	for i, addr := range ram.addresses {
		if strings.EqualFold(addr.Address, address) {
			// Update existing entry
			ram.addresses[i].LastUsed = now
			ram.addresses[i].UseCount++
			ram.addresses[i].LastAmount = amount
			ram.addresses[i].LastAsset = asset

			// Update contact name if provided
			if contactName != "" {
				ram.addresses[i].ContactName = contactName
			}

			ram.calculateFrequency(&ram.addresses[i])
			ram.sortAddresses()
			return
		}
	}

	// Add new entry
	newAddr := RecentAddress{
		Address:     address,
		ContactName: contactName,
		LastUsed:    now,
		UseCount:    1,
		LastAmount:  amount,
		LastAsset:   asset,
	}

	ram.calculateFrequency(&newAddr)
	ram.addresses = append(ram.addresses, newAddr)

	// Trim to max entries if needed
	if len(ram.addresses) > ram.maxEntries {
		ram.addresses = ram.addresses[:ram.maxEntries]
	}

	ram.sortAddresses()
}

func (ram *RecentAddressManager) GetRecentAddresses(limit int) []RecentAddress {
	if limit <= 0 || limit > len(ram.addresses) {
		limit = len(ram.addresses)
	}

	// Return a copy to prevent external modification
	result := make([]RecentAddress, limit)
	copy(result, ram.addresses[:limit])
	return result
}

func (ram *RecentAddressManager) GetAddressByAddress(address string) *RecentAddress {
	for _, addr := range ram.addresses {
		if strings.EqualFold(addr.Address, address) {
			return &addr
		}
	}
	return nil
}

func (ram *RecentAddressManager) UpdateContactName(address, contactName string) {
	for i, addr := range ram.addresses {
		if strings.EqualFold(addr.Address, address) {
			ram.addresses[i].ContactName = contactName
			return
		}
	}
}

func (ram *RecentAddressManager) RemoveAddress(address string) bool {
	for i, addr := range ram.addresses {
		if strings.EqualFold(addr.Address, address) {
			ram.addresses = append(ram.addresses[:i], ram.addresses[i+1:]...)
			return true
		}
	}
	return false
}

func (ram *RecentAddressManager) Clear() {
	ram.addresses = make([]RecentAddress, 0)
}

func (ram *RecentAddressManager) GetStats() (int, time.Time) {
	count := len(ram.addresses)
	var lastUsed time.Time

	if count > 0 {
		lastUsed = ram.addresses[0].LastUsed
	}

	return count, lastUsed
}

func (ram *RecentAddressManager) calculateFrequency(addr *RecentAddress) {
	// Calculate frequency score based on use count and recency
	// More recent and more frequently used addresses get higher scores

	now := time.Now()
	daysSinceLastUse := now.Sub(addr.LastUsed).Hours() / 24

	// Base frequency from use count
	baseFreq := float64(addr.UseCount)

	// Recency multiplier (decays over time)
	// Recent usage (within 7 days) gets full weight
	// Older usage gets progressively less weight
	var recencyMultiplier float64
	if daysSinceLastUse <= 7 {
		recencyMultiplier = 1.0
	} else if daysSinceLastUse <= 30 {
		recencyMultiplier = 0.7
	} else if daysSinceLastUse <= 90 {
		recencyMultiplier = 0.4
	} else {
		recencyMultiplier = 0.1
	}

	addr.Frequency = baseFreq * recencyMultiplier
}

func (ram *RecentAddressManager) sortAddresses() {
	// Sort by frequency (descending), then by last used (descending)
	sort.Slice(ram.addresses, func(i, j int) bool {
		if ram.addresses[i].Frequency != ram.addresses[j].Frequency {
			return ram.addresses[i].Frequency > ram.addresses[j].Frequency
		}
		return ram.addresses[i].LastUsed.After(ram.addresses[j].LastUsed)
	})
}

func (ram *RecentAddressManager) SearchAddresses(query string) []RecentAddress {
	if query == "" {
		return ram.GetRecentAddresses(10) // Return top 10 by default
	}

	query = strings.ToLower(strings.TrimSpace(query))
	var results []RecentAddress

	for _, addr := range ram.addresses {
		// Search in address, contact name
		if strings.Contains(strings.ToLower(addr.Address), query) ||
			strings.Contains(strings.ToLower(addr.ContactName), query) {
			results = append(results, addr)
		}
	}

	return results
}

// Maintenance methods
func (ram *RecentAddressManager) CleanupOldEntries(maxAge time.Duration) int {
	if maxAge <= 0 {
		return 0
	}

	cutoff := time.Now().Add(-maxAge)
	originalCount := len(ram.addresses)

	// Filter out old entries
	filtered := make([]RecentAddress, 0, len(ram.addresses))
	for _, addr := range ram.addresses {
		if addr.LastUsed.After(cutoff) {
			filtered = append(filtered, addr)
		}
	}

	ram.addresses = filtered
	return originalCount - len(ram.addresses)
}

func (ram *RecentAddressManager) RecalculateAllFrequencies() {
	for i := range ram.addresses {
		ram.calculateFrequency(&ram.addresses[i])
	}
	ram.sortAddresses()
}

// Export/Import methods for backup/restore
func (ram *RecentAddressManager) Export() []RecentAddress {
	result := make([]RecentAddress, len(ram.addresses))
	copy(result, ram.addresses)
	return result
}

func (ram *RecentAddressManager) Import(addresses []RecentAddress) {
	ram.addresses = make([]RecentAddress, 0, len(addresses))

	for _, addr := range addresses {
		// Recalculate frequency for imported addresses
		ram.calculateFrequency(&addr)
		ram.addresses = append(ram.addresses, addr)
	}

	// Trim to max entries if needed
	if len(ram.addresses) > ram.maxEntries {
		ram.addresses = ram.addresses[:ram.maxEntries]
	}

	ram.sortAddresses()
}
