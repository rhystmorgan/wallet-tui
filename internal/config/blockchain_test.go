package config

import (
	"os"
	"testing"
	"time"

	"rhystmorgan/veWallet/internal/blockchain"
)

func TestLoadBlockchainConfig(t *testing.T) {
	// Clear environment variables
	os.Unsetenv("VETERM_NETWORK")
	os.Unsetenv("VETERM_NODE_URL")
	os.Unsetenv("VETERM_TIMEOUT")
	os.Unsetenv("VETERM_RETRY_COUNT")
	os.Unsetenv("VETERM_CACHE_TTL")

	config, err := LoadBlockchainConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test defaults
	if config.Network != "mainnet" {
		t.Errorf("Expected default network 'mainnet', got '%s'", config.Network)
	}

	if config.NodeURL != "" {
		t.Errorf("Expected empty NodeURL by default, got '%s'", config.NodeURL)
	}

	if config.Timeout != 30*time.Second {
		t.Errorf("Expected default timeout 30s, got %v", config.Timeout)
	}

	if config.RetryCount != 3 {
		t.Errorf("Expected default retry count 3, got %d", config.RetryCount)
	}

	if config.CacheTTL != 30*time.Second {
		t.Errorf("Expected default cache TTL 30s, got %v", config.CacheTTL)
	}
}

func TestLoadBlockchainConfigWithEnv(t *testing.T) {
	// Set environment variables
	os.Setenv("VETERM_NETWORK", "testnet")
	os.Setenv("VETERM_NODE_URL", "http://localhost:8669")
	os.Setenv("VETERM_TIMEOUT", "60s")
	os.Setenv("VETERM_RETRY_COUNT", "5")
	os.Setenv("VETERM_CACHE_TTL", "60s")

	defer func() {
		// Clean up
		os.Unsetenv("VETERM_NETWORK")
		os.Unsetenv("VETERM_NODE_URL")
		os.Unsetenv("VETERM_TIMEOUT")
		os.Unsetenv("VETERM_RETRY_COUNT")
		os.Unsetenv("VETERM_CACHE_TTL")
	}()

	config, err := LoadBlockchainConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.Network != "testnet" {
		t.Errorf("Expected network 'testnet', got '%s'", config.Network)
	}

	if config.NodeURL != "http://localhost:8669" {
		t.Errorf("Expected NodeURL 'http://localhost:8669', got '%s'", config.NodeURL)
	}

	if config.Timeout != 60*time.Second {
		t.Errorf("Expected timeout 60s, got %v", config.Timeout)
	}

	if config.RetryCount != 5 {
		t.Errorf("Expected retry count 5, got %d", config.RetryCount)
	}

	if config.CacheTTL != 60*time.Second {
		t.Errorf("Expected cache TTL 60s, got %v", config.CacheTTL)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  BlockchainConfig
		wantErr bool
	}{
		{
			name: "valid mainnet config",
			config: BlockchainConfig{
				Network:    "mainnet",
				Timeout:    30 * time.Second,
				RetryCount: 3,
				CacheTTL:   30 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "valid testnet config",
			config: BlockchainConfig{
				Network:    "testnet",
				Timeout:    30 * time.Second,
				RetryCount: 3,
				CacheTTL:   30 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "invalid network",
			config: BlockchainConfig{
				Network:    "invalid",
				Timeout:    30 * time.Second,
				RetryCount: 3,
				CacheTTL:   30 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "zero timeout",
			config: BlockchainConfig{
				Network:    "mainnet",
				Timeout:    0,
				RetryCount: 3,
				CacheTTL:   30 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "negative retry count",
			config: BlockchainConfig{
				Network:    "mainnet",
				Timeout:    30 * time.Second,
				RetryCount: -1,
				CacheTTL:   30 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "zero cache TTL",
			config: BlockchainConfig{
				Network:    "mainnet",
				Timeout:    30 * time.Second,
				RetryCount: 3,
				CacheTTL:   0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestToBlockchainConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   BlockchainConfig
		expected blockchain.Network
	}{
		{
			name: "mainnet conversion",
			config: BlockchainConfig{
				Network:    "mainnet",
				NodeURL:    "http://mainnet.example.com",
				Timeout:    30 * time.Second,
				RetryCount: 3,
				CacheTTL:   30 * time.Second,
			},
			expected: blockchain.MainNet,
		},
		{
			name: "testnet conversion",
			config: BlockchainConfig{
				Network:    "testnet",
				NodeURL:    "http://testnet.example.com",
				Timeout:    30 * time.Second,
				RetryCount: 3,
				CacheTTL:   30 * time.Second,
			},
			expected: blockchain.TestNet,
		},
		{
			name: "invalid network defaults to mainnet",
			config: BlockchainConfig{
				Network:    "invalid",
				NodeURL:    "http://example.com",
				Timeout:    30 * time.Second,
				RetryCount: 3,
				CacheTTL:   30 * time.Second,
			},
			expected: blockchain.MainNet,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blockchainConfig := tt.config.ToBlockchainConfig()

			if blockchainConfig.Network != tt.expected {
				t.Errorf("Expected network %s, got %s", tt.expected, blockchainConfig.Network)
			}

			if blockchainConfig.NodeURL != tt.config.NodeURL {
				t.Errorf("Expected NodeURL %s, got %s", tt.config.NodeURL, blockchainConfig.NodeURL)
			}

			if blockchainConfig.Timeout != tt.config.Timeout {
				t.Errorf("Expected timeout %v, got %v", tt.config.Timeout, blockchainConfig.Timeout)
			}

			if blockchainConfig.RetryCount != tt.config.RetryCount {
				t.Errorf("Expected retry count %d, got %d", tt.config.RetryCount, blockchainConfig.RetryCount)
			}

			if blockchainConfig.RetryDelay != 2*time.Second {
				t.Errorf("Expected retry delay 2s, got %v", blockchainConfig.RetryDelay)
			}
		})
	}
}

func TestGetDefaultConfig(t *testing.T) {
	config := GetDefaultConfig()

	if config.Network != "mainnet" {
		t.Errorf("Expected default network 'mainnet', got '%s'", config.Network)
	}

	if config.NodeURL != "" {
		t.Errorf("Expected empty NodeURL by default, got '%s'", config.NodeURL)
	}

	if config.Timeout != 30*time.Second {
		t.Errorf("Expected default timeout 30s, got %v", config.Timeout)
	}

	if config.RetryCount != 3 {
		t.Errorf("Expected default retry count 3, got %d", config.RetryCount)
	}

	if config.CacheTTL != 30*time.Second {
		t.Errorf("Expected default cache TTL 30s, got %v", config.CacheTTL)
	}
}

func TestIsDebugEnabled(t *testing.T) {
	// Test with no debug env var
	os.Unsetenv("VETERM_DEBUG")
	if IsDebugEnabled() {
		t.Error("Expected debug to be disabled when env var is not set")
	}

	// Test with debug=true
	os.Setenv("VETERM_DEBUG", "true")
	if !IsDebugEnabled() {
		t.Error("Expected debug to be enabled when VETERM_DEBUG=true")
	}

	// Test with debug=1
	os.Setenv("VETERM_DEBUG", "1")
	if !IsDebugEnabled() {
		t.Error("Expected debug to be enabled when VETERM_DEBUG=1")
	}

	// Test with debug=false
	os.Setenv("VETERM_DEBUG", "false")
	if IsDebugEnabled() {
		t.Error("Expected debug to be disabled when VETERM_DEBUG=false")
	}

	// Clean up
	os.Unsetenv("VETERM_DEBUG")
}

func TestParseHelpers(t *testing.T) {
	// Test parseIntOrDefault
	os.Setenv("TEST_INT", "42")
	result := parseIntOrDefault("TEST_INT", 10)
	if result != 42 {
		t.Errorf("Expected 42, got %d", result)
	}

	result = parseIntOrDefault("NONEXISTENT_INT", 10)
	if result != 10 {
		t.Errorf("Expected default 10, got %d", result)
	}

	os.Setenv("TEST_INVALID_INT", "not_a_number")
	result = parseIntOrDefault("TEST_INVALID_INT", 10)
	if result != 10 {
		t.Errorf("Expected default 10 for invalid int, got %d", result)
	}

	// Test parseDurationOrDefault
	os.Setenv("TEST_DURATION", "5m")
	duration := parseDurationOrDefault("TEST_DURATION", 1*time.Minute)
	if duration != 5*time.Minute {
		t.Errorf("Expected 5m, got %v", duration)
	}

	duration = parseDurationOrDefault("NONEXISTENT_DURATION", 1*time.Minute)
	if duration != 1*time.Minute {
		t.Errorf("Expected default 1m, got %v", duration)
	}

	os.Setenv("TEST_INVALID_DURATION", "not_a_duration")
	duration = parseDurationOrDefault("TEST_INVALID_DURATION", 1*time.Minute)
	if duration != 1*time.Minute {
		t.Errorf("Expected default 1m for invalid duration, got %v", duration)
	}

	// Clean up
	os.Unsetenv("TEST_INT")
	os.Unsetenv("TEST_INVALID_INT")
	os.Unsetenv("TEST_DURATION")
	os.Unsetenv("TEST_INVALID_DURATION")
}
