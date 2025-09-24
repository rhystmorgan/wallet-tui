package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"rhystmorgan/veWallet/internal/blockchain"
)

type BlockchainConfig struct {
	Network    string        `json:"network"`
	NodeURL    string        `json:"node_url"`
	Timeout    time.Duration `json:"timeout"`
	RetryCount int           `json:"retry_count"`
	CacheTTL   time.Duration `json:"cache_ttl"`
}

func LoadBlockchainConfig() (*BlockchainConfig, error) {
	config := &BlockchainConfig{
		Network:    getEnvOrDefault("VETERM_NETWORK", "mainnet"),
		NodeURL:    getEnvOrDefault("VETERM_NODE_URL", ""),
		Timeout:    parseDurationOrDefault("VETERM_TIMEOUT", 30*time.Second),
		RetryCount: parseIntOrDefault("VETERM_RETRY_COUNT", 3),
		CacheTTL:   parseDurationOrDefault("VETERM_CACHE_TTL", 30*time.Second),
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	return config, nil
}

func (c *BlockchainConfig) Validate() error {
	switch c.Network {
	case "mainnet", "testnet":
		// Valid networks
	default:
		return fmt.Errorf("invalid network: %s (must be 'mainnet' or 'testnet')", c.Network)
	}

	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive, got: %v", c.Timeout)
	}

	if c.RetryCount < 0 {
		return fmt.Errorf("retry count must be non-negative, got: %d", c.RetryCount)
	}

	if c.CacheTTL <= 0 {
		return fmt.Errorf("cache TTL must be positive, got: %v", c.CacheTTL)
	}

	return nil
}

func (c *BlockchainConfig) ToBlockchainConfig() blockchain.Config {
	var network blockchain.Network
	switch c.Network {
	case "mainnet":
		network = blockchain.MainNet
	case "testnet":
		network = blockchain.TestNet
	default:
		network = blockchain.MainNet // Default fallback
	}

	return blockchain.Config{
		Network:    network,
		NodeURL:    c.NodeURL,
		Timeout:    c.Timeout,
		RetryCount: c.RetryCount,
		RetryDelay: 2 * time.Second,
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func parseDurationOrDefault(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func GetDefaultConfig() *BlockchainConfig {
	return &BlockchainConfig{
		Network:    "mainnet",
		NodeURL:    "",
		Timeout:    30 * time.Second,
		RetryCount: 3,
		CacheTTL:   30 * time.Second,
	}
}

func IsDebugEnabled() bool {
	return os.Getenv("VETERM_DEBUG") == "true" || os.Getenv("VETERM_DEBUG") == "1"
}
