package utils

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"
)

// FormatAddress truncates an address for display purposes
func FormatAddress(address string, prefixLen, suffixLen int) string {
	if len(address) <= prefixLen+suffixLen {
		return address
	}

	return address[:prefixLen] + "..." + address[len(address)-suffixLen:]
}

// FormatAddressWithName formats an address with an optional name
func FormatAddressWithName(address, name string) string {
	if name != "" {
		return fmt.Sprintf("%s (%s)", name, FormatAddress(address, 6, 4))
	}
	return FormatAddress(address, 10, 8)
}

// FormatBalance formats a balance with proper decimal places and units
func FormatBalance(amount *big.Int, symbol string, decimals int) string {
	if amount == nil {
		return fmt.Sprintf("0 %s", symbol)
	}

	formatted := FormatAmount(amount, decimals)
	return fmt.Sprintf("%s %s", formatted, symbol)
}

// FormatBalanceWithCommas formats a balance with comma separators
func FormatBalanceWithCommas(amount *big.Int, symbol string, decimals int) string {
	if amount == nil {
		return fmt.Sprintf("0 %s", symbol)
	}

	formatted := FormatAmount(amount, decimals)
	parts := strings.Split(formatted, ".")

	// Add commas to integer part
	intPart := parts[0]
	if len(intPart) > 3 {
		var result strings.Builder
		for i, digit := range intPart {
			if i > 0 && (len(intPart)-i)%3 == 0 {
				result.WriteString(",")
			}
			result.WriteRune(digit)
		}
		intPart = result.String()
	}

	// Reconstruct with decimal part if exists
	if len(parts) > 1 {
		formatted = intPart + "." + parts[1]
	} else {
		formatted = intPart
	}

	return fmt.Sprintf("%s %s", formatted, symbol)
}

// FormatDuration formats a duration in a human-readable way
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	} else {
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// FormatTransactionID formats a transaction ID for display
func FormatTransactionID(txID string) string {
	if len(txID) <= 16 {
		return txID
	}
	return txID[:8] + "..." + txID[len(txID)-8:]
}

// FormatGasPrice formats gas price in a readable format
func FormatGasPrice(gasPrice *big.Int) string {
	if gasPrice == nil {
		return "0 wei"
	}

	// Convert to Gwei for readability
	gwei := new(big.Float).SetInt(gasPrice)
	gwei.Quo(gwei, big.NewFloat(1e9))

	return fmt.Sprintf("%.2f Gwei", gwei)
}

// FormatPercentage formats a percentage value
func FormatPercentage(value float64) string {
	return fmt.Sprintf("%.1f%%", value*100)
}

// TruncateString truncates a string to a maximum length with ellipsis
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	if maxLen <= 3 {
		return s[:maxLen]
	}

	return s[:maxLen-3] + "..."
}

// PadString pads a string to a specific width
func PadString(s string, width int, padChar rune) string {
	if len(s) >= width {
		return s
	}

	padding := strings.Repeat(string(padChar), width-len(s))
	return s + padding
}

// FormatStepIndicator creates a step indicator string
func FormatStepIndicator(currentStep, totalSteps int, stepNames []string) string {
	var result strings.Builder

	for i := 0; i < totalSteps; i++ {
		if i > 0 {
			result.WriteString(" → ")
		}

		stepName := strconv.Itoa(i + 1)
		if i < len(stepNames) {
			stepName = stepNames[i]
		}

		if i == currentStep {
			result.WriteString("[" + stepName + "]")
		} else if i < currentStep {
			result.WriteString("✓")
		} else {
			result.WriteString(stepName)
		}
	}

	return result.String()
}

// FormatValidationError formats validation errors in a user-friendly way
func FormatValidationError(field string, err error) string {
	if err == nil {
		return ""
	}

	return fmt.Sprintf("%s: %s", field, err.Error())
}

// FormatLoadingText creates animated loading text
func FormatLoadingText(baseText string, frame int) string {
	dots := []string{"", ".", "..", "..."}
	return baseText + dots[frame%len(dots)]
}

// FormatConfirmationText formats confirmation prompts
func FormatConfirmationText(action string, details map[string]string) string {
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Confirm %s:\n\n", action))

	for key, value := range details {
		result.WriteString(fmt.Sprintf("  %s: %s\n", key, value))
	}

	result.WriteString("\nProceed? (y/N)")
	return result.String()
}

// FormatTimeAgo formats a time as "X ago" string
func FormatTimeAgo(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	if diff < time.Minute {
		return "just now"
	} else if diff < time.Hour {
		minutes := int(diff.Minutes())
		if minutes == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d mins ago", minutes)
	} else if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else if diff < 7*24*time.Hour {
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	} else if diff < 30*24*time.Hour {
		weeks := int(diff.Hours() / (24 * 7))
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	} else {
		months := int(diff.Hours() / (24 * 30))
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	}
}
