package utils

import (
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/ethereum/go-ethereum/common"
	"github.com/tyler-smith/go-bip39"
)

// PasswordStrength represents the strength level of a password
type PasswordStrength int

const (
	PasswordWeak PasswordStrength = iota
	PasswordMedium
	PasswordStrong
)

// ValidatePassword checks password strength and returns validation result
func ValidatePassword(password string) (PasswordStrength, []string) {
	var issues []string
	strength := PasswordStrong

	if len(password) < 8 {
		issues = append(issues, "Password must be at least 8 characters long")
		strength = PasswordWeak
	}

	hasUpper := false
	hasLower := false
	hasDigit := false
	hasSpecial := false

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasDigit = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	if !hasUpper {
		issues = append(issues, "Password must contain at least one uppercase letter")
		if strength == PasswordStrong {
			strength = PasswordMedium
		}
	}

	if !hasLower {
		issues = append(issues, "Password must contain at least one lowercase letter")
		if strength == PasswordStrong {
			strength = PasswordMedium
		}
	}

	if !hasDigit {
		issues = append(issues, "Password must contain at least one number")
		if strength == PasswordStrong {
			strength = PasswordMedium
		}
	}

	if !hasSpecial {
		issues = append(issues, "Password should contain at least one special character")
		if strength == PasswordStrong {
			strength = PasswordMedium
		}
	}

	if len(issues) > 2 {
		strength = PasswordWeak
	}

	return strength, issues
}

// ValidateMnemonic validates a BIP39 mnemonic phrase
func ValidateMnemonic(mnemonic string) bool {
	return bip39.IsMnemonicValid(mnemonic)
}

// ValidateMnemonicWords validates individual words against BIP39 wordlist
func ValidateMnemonicWords(words []string) []bool {
	wordList := bip39.GetWordList()
	wordMap := make(map[string]bool)
	for _, word := range wordList {
		wordMap[word] = true
	}

	results := make([]bool, len(words))
	for i, word := range words {
		results[i] = wordMap[strings.ToLower(strings.TrimSpace(word))]
	}
	return results
}

// ValidateWalletName validates wallet name format
func ValidateWalletName(name string) []string {
	var issues []string

	name = strings.TrimSpace(name)
	if len(name) == 0 {
		issues = append(issues, "Wallet name cannot be empty")
		return issues
	}

	if len(name) < 3 {
		issues = append(issues, "Wallet name must be at least 3 characters long")
	}

	if len(name) > 50 {
		issues = append(issues, "Wallet name must be less than 50 characters")
	}

	// Check for valid characters (alphanumeric, spaces, hyphens, underscores)
	validName := regexp.MustCompile(`^[a-zA-Z0-9\s\-_]+$`)
	if !validName.MatchString(name) {
		issues = append(issues, "Wallet name can only contain letters, numbers, spaces, hyphens, and underscores")
	}

	return issues
}

// SplitMnemonic splits a mnemonic string into individual words
func SplitMnemonic(mnemonic string) []string {
	words := strings.Fields(strings.TrimSpace(mnemonic))
	// Ensure we have exactly 12 words
	result := make([]string, 12)
	for i := 0; i < 12; i++ {
		if i < len(words) {
			result[i] = strings.ToLower(strings.TrimSpace(words[i]))
		} else {
			result[i] = ""
		}
	}
	return result
}

// JoinMnemonic joins mnemonic words into a single string
func JoinMnemonic(words []string) string {
	var validWords []string
	for _, word := range words {
		word = strings.TrimSpace(word)
		if word != "" {
			validWords = append(validWords, word)
		}
	}
	return strings.Join(validWords, " ")
}

// ValidateVeChainAddress validates a VeChain address format and checksum
func ValidateVeChainAddress(address string) error {
	address = strings.TrimSpace(address)

	// Check if empty
	if address == "" {
		return fmt.Errorf("address cannot be empty")
	}

	// Check if it starts with 0x
	if !strings.HasPrefix(address, "0x") {
		return fmt.Errorf("address must start with 0x")
	}

	// Check length (0x + 40 hex characters = 42 total)
	if len(address) != 42 {
		return fmt.Errorf("address must be exactly 42 characters long")
	}

	// Check if it contains only valid hex characters
	hexPattern := regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)
	if !hexPattern.MatchString(address) {
		return fmt.Errorf("address contains invalid characters")
	}

	// Check if it's not the zero address
	if address == "0x0000000000000000000000000000000000000000" {
		return fmt.Errorf("cannot send to zero address")
	}

	// Validate checksum using ethereum common package
	if !common.IsHexAddress(address) {
		return fmt.Errorf("invalid address format")
	}

	return nil
}

// ValidateAmount validates a transaction amount string
func ValidateAmount(amountStr string, maxDecimals int) (*big.Int, error) {
	amountStr = strings.TrimSpace(amountStr)

	// Check if empty
	if amountStr == "" {
		return nil, fmt.Errorf("amount cannot be empty")
	}

	// Check if it's a valid number
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid amount format")
	}

	// Check if positive
	if amount <= 0 {
		return nil, fmt.Errorf("amount must be greater than 0")
	}

	// Check decimal places
	parts := strings.Split(amountStr, ".")
	if len(parts) == 2 && len(parts[1]) > maxDecimals {
		return nil, fmt.Errorf("amount cannot have more than %d decimal places", maxDecimals)
	}

	// Convert to wei (multiply by 10^18)
	multiplier := new(big.Float).SetFloat64(1e18)
	amountFloat := new(big.Float).SetFloat64(amount)
	weiFloat := new(big.Float).Mul(amountFloat, multiplier)

	// Convert to big.Int
	weiInt, _ := weiFloat.Int(nil)

	return weiInt, nil
}

// FormatAmount formats a big.Int amount (in wei) to a human-readable string
func FormatAmount(amount *big.Int, decimals int) string {
	if amount == nil {
		return "0"
	}

	// Convert from wei to readable format
	divisor := new(big.Float).SetFloat64(1e18)
	amountFloat := new(big.Float).SetInt(amount)
	result := new(big.Float).Quo(amountFloat, divisor)

	// Format with specified decimal places
	return fmt.Sprintf("%."+strconv.Itoa(decimals)+"f", result)
}

// ValidateAmountAgainstBalance checks if amount is valid against available balance
func ValidateAmountAgainstBalance(amount *big.Int, balance *big.Int, gasFee *big.Int) error {
	if amount == nil {
		return fmt.Errorf("amount cannot be nil")
	}

	if balance == nil {
		return fmt.Errorf("balance not available")
	}

	// For VET transfers, need to account for gas fees
	totalRequired := new(big.Int).Add(amount, gasFee)

	if totalRequired.Cmp(balance) > 0 {
		return fmt.Errorf("insufficient balance (need %s, have %s)",
			FormatAmount(totalRequired, 4), FormatAmount(balance, 4))
	}

	return nil
}

// ValidateVTHOAmountAgainstBalance checks VTHO amount against balance (gas paid in VET)
func ValidateVTHOAmountAgainstBalance(vthoAmount *big.Int, vthoBalance *big.Int, vetBalance *big.Int, gasFee *big.Int) error {
	if vthoAmount == nil {
		return fmt.Errorf("VTHO amount cannot be nil")
	}

	if vthoBalance == nil {
		return fmt.Errorf("VTHO balance not available")
	}

	if vetBalance == nil {
		return fmt.Errorf("VET balance not available")
	}

	// Check VTHO balance
	if vthoAmount.Cmp(vthoBalance) > 0 {
		return fmt.Errorf("insufficient VTHO balance (need %s, have %s)",
			FormatAmount(vthoAmount, 4), FormatAmount(vthoBalance, 4))
	}

	// Check VET balance for gas fees
	if gasFee.Cmp(vetBalance) > 0 {
		return fmt.Errorf("insufficient VET for gas fees (need %s, have %s)",
			FormatAmount(gasFee, 4), FormatAmount(vetBalance, 4))
	}

	return nil
}
