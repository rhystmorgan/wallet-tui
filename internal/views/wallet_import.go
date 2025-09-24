package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/darrenvechain/thorgo/crypto/hdwallet"

	"rhystmorgan/veWallet/internal/models"
	"rhystmorgan/veWallet/internal/storage"
	"rhystmorgan/veWallet/internal/utils"
)

type WalletImportStep int

const (
	StepImportName WalletImportStep = iota
	StepMnemonicInput
	StepMnemonicValidation
	StepPasswordSetup
	StepPasswordConfirm
	StepImporting
	StepImportComplete
)

type WalletImportModel struct {
	step           WalletImportStep
	name           string
	mnemonicWords  [12]string
	currentWordIdx int
	password       string
	confirm        string
	storage        *storage.Storage
	previewAddress string

	// Validation states
	nameValid     bool
	wordsValid    [12]bool
	mnemonicValid bool
	passwordValid bool
	confirmValid  bool

	// Error states
	nameError     string
	mnemonicError string
	passwordError string
	confirmError  string

	// UI state
	cursor int
	err    error
}

type WalletImportedMsg struct {
	Wallet *models.Wallet
}

func NewWalletImportModel() *WalletImportModel {
	return &WalletImportModel{
		step:           StepImportName,
		currentWordIdx: 0,
		cursor:         0,
	}
}

func (m *WalletImportModel) SetStorage(storage *storage.Storage) {
	m.storage = storage
}

func (m WalletImportModel) Init() tea.Cmd {
	return nil
}

func (m WalletImportModel) Update(msg tea.Msg) (WalletImportModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.step == StepImportName {
				return m, NavigateTo(ViewWalletSelector, nil)
			} else {
				// Go back one step
				m.step--
				m.err = nil
				return m, nil
			}

		case "enter":
			return m.handleEnter()

		case "backspace":
			return m.handleBackspace()

		case "tab":
			if m.step == StepMnemonicInput {
				m.currentWordIdx = (m.currentWordIdx + 1) % 12
			}

		case "shift+tab":
			if m.step == StepMnemonicInput {
				m.currentWordIdx = (m.currentWordIdx - 1 + 12) % 12
			}

		case "ctrl+v":
			if m.step == StepMnemonicInput {
				return m.handlePaste()
			}

		default:
			if len(msg.String()) == 1 {
				return m.handleCharInput(msg.String())
			}
		}

	case WalletValidatedMsg:
		m.previewAddress = msg.Address
		m.mnemonicValid = true
		m.mnemonicError = ""
		return m, nil

	case ErrorMsg:
		m.err = msg.Err
		if m.step == StepMnemonicValidation {
			m.mnemonicValid = false
			m.mnemonicError = msg.Err.Error()
		}
		return m, nil

	case WalletImportedMsg:
		return m, LoadWallet(msg.Wallet)
	}

	return m, nil
}

func (m WalletImportModel) handleEnter() (WalletImportModel, tea.Cmd) {
	switch m.step {
	case StepImportName:
		if m.validateName() {
			m.step = StepMnemonicInput
		}

	case StepMnemonicInput:
		if m.validateAllWords() {
			m.step = StepMnemonicValidation
			return m, m.validateAndPreviewWallet()
		}

	case StepMnemonicValidation:
		if m.mnemonicValid {
			m.step = StepPasswordSetup
		}

	case StepPasswordSetup:
		if m.validatePassword() {
			m.step = StepPasswordConfirm
		}

	case StepPasswordConfirm:
		if m.validateConfirmPassword() {
			m.step = StepImporting
			return m, m.importWallet()
		}

	case StepImportComplete:
		return m, NavigateTo(ViewWalletSelector, nil)
	}

	return m, nil
}

func (m WalletImportModel) handleBackspace() (WalletImportModel, tea.Cmd) {
	switch m.step {
	case StepImportName:
		if len(m.name) > 0 {
			m.name = m.name[:len(m.name)-1]
			m.validateName()
		}

	case StepMnemonicInput:
		if len(m.mnemonicWords[m.currentWordIdx]) > 0 {
			m.mnemonicWords[m.currentWordIdx] = m.mnemonicWords[m.currentWordIdx][:len(m.mnemonicWords[m.currentWordIdx])-1]
			m.validateWord(m.currentWordIdx)
		}

	case StepPasswordSetup:
		if len(m.password) > 0 {
			m.password = m.password[:len(m.password)-1]
			m.validatePassword()
		}

	case StepPasswordConfirm:
		if len(m.confirm) > 0 {
			m.confirm = m.confirm[:len(m.confirm)-1]
			m.validateConfirmPassword()
		}
	}

	return m, nil
}

func (m WalletImportModel) handleCharInput(char string) (WalletImportModel, tea.Cmd) {
	switch m.step {
	case StepImportName:
		m.name += char
		m.validateName()

	case StepMnemonicInput:
		m.mnemonicWords[m.currentWordIdx] += char
		m.validateWord(m.currentWordIdx)

	case StepPasswordSetup:
		m.password += char
		m.validatePassword()

	case StepPasswordConfirm:
		m.confirm += char
		m.validateConfirmPassword()
	}

	return m, nil
}

func (m WalletImportModel) handlePaste() (WalletImportModel, tea.Cmd) {
	// Note: In a real implementation, you'd get clipboard content
	// For now, this is a placeholder for paste functionality
	return m, nil
}

func (m *WalletImportModel) validateName() bool {
	issues := utils.ValidateWalletName(m.name)
	if len(issues) > 0 {
		m.nameError = issues[0]
		m.nameValid = false
		return false
	}
	m.nameError = ""
	m.nameValid = true
	return true
}

func (m *WalletImportModel) validateWord(index int) bool {
	word := strings.ToLower(strings.TrimSpace(m.mnemonicWords[index]))
	if word == "" {
		m.wordsValid[index] = false
		return false
	}

	// Validate against BIP39 wordlist
	wordList := utils.ValidateMnemonicWords([]string{word})
	m.wordsValid[index] = wordList[0]
	return m.wordsValid[index]
}

func (m *WalletImportModel) validateAllWords() bool {
	allValid := true
	for i := 0; i < 12; i++ {
		if !m.validateWord(i) || strings.TrimSpace(m.mnemonicWords[i]) == "" {
			allValid = false
		}
	}
	return allValid
}

func (m *WalletImportModel) validatePassword() bool {
	strength, issues := utils.ValidatePassword(m.password)
	if strength == utils.PasswordWeak {
		m.passwordError = strings.Join(issues, ", ")
		m.passwordValid = false
		return false
	}
	if len(issues) > 0 {
		m.passwordError = "Medium strength: " + strings.Join(issues, ", ")
	} else {
		m.passwordError = "Strong password ✓"
	}
	m.passwordValid = true
	return true
}

func (m *WalletImportModel) validateConfirmPassword() bool {
	if m.password != m.confirm {
		m.confirmError = "Passwords do not match"
		m.confirmValid = false
		return false
	}
	m.confirmError = "Passwords match ✓"
	m.confirmValid = true
	return true
}

func (m WalletImportModel) validateAndPreviewWallet() tea.Cmd {
	return func() tea.Msg {
		// Join mnemonic words
		var words []string
		for _, word := range m.mnemonicWords {
			word = strings.TrimSpace(word)
			if word != "" {
				words = append(words, word)
			}
		}
		mnemonic := strings.Join(words, " ")

		// Validate complete mnemonic
		if !utils.ValidateMnemonic(mnemonic) {
			return ErrorMsg{Err: fmt.Errorf("invalid mnemonic phrase: checksum verification failed")}
		}

		// Generate preview address
		derivationPath, err := hdwallet.ParseDerivationPath("m/44'/818'/0'/0/0")
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to parse derivation path: %w", err)}
		}

		hdWallet, err := hdwallet.FromMnemonicAt(mnemonic, derivationPath)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create wallet from mnemonic: %w", err)}
		}

		address := hdWallet.Address().Hex()

		// Check for duplicate wallet
		if m.storage != nil {
			wallets, err := m.storage.ListWallets()
			if err != nil {
				return ErrorMsg{Err: fmt.Errorf("failed to check existing wallets: %w", err)}
			}

			for _, wallet := range wallets {
				if wallet.Address == address {
					return ErrorMsg{Err: fmt.Errorf("wallet with this address already exists: %s", address)}
				}
			}
		}

		return WalletValidatedMsg{Address: address}
	}
}

type WalletValidatedMsg struct {
	Address string
}

func (m WalletImportModel) importWallet() tea.Cmd {
	return func() tea.Msg {
		// Join mnemonic words
		var words []string
		for _, word := range m.mnemonicWords {
			word = strings.TrimSpace(word)
			if word != "" {
				words = append(words, word)
			}
		}
		mnemonic := strings.Join(words, " ")

		// Create wallet from mnemonic
		wallet, err := models.NewWallet(m.name, mnemonic)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create wallet: %w", err)}
		}

		// Save encrypted wallet
		if m.storage != nil {
			if err := m.storage.SaveWallet(wallet, m.password); err != nil {
				return ErrorMsg{Err: fmt.Errorf("failed to save wallet: %w", err)}
			}
		}

		// Clear sensitive data from memory
		m.password = ""
		m.confirm = ""
		for i := range m.mnemonicWords {
			m.mnemonicWords[i] = ""
		}

		return WalletImportedMsg{Wallet: wallet}
	}
}

func (m WalletImportModel) View() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Blue)).
		Bold(true).
		Padding(1, 0)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Bold(true)

	inputStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Green)).
		Background(lipgloss.Color(utils.Colours.Surface0)).
		Padding(0, 1)

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Red))

	successStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Green))

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Subtext0)).
		Italic(true)

	var content string
	content += titleStyle.Render("Import Wallet") + "\n\n"

	// Progress indicator
	progress := fmt.Sprintf("Step %d of 6", int(m.step)+1)
	if m.step >= StepImporting {
		progress = "Importing wallet..."
	}
	content += helpStyle.Render(progress) + "\n\n"

	switch m.step {
	case StepImportName:
		content += labelStyle.Render("Wallet Name:") + "\n"
		content += inputStyle.Render(m.name+"_") + "\n"
		if m.nameError != "" {
			if m.nameValid {
				content += successStyle.Render("✓ "+m.nameError) + "\n"
			} else {
				content += errorStyle.Render("✗ "+m.nameError) + "\n"
			}
		}
		content += "\n" + helpStyle.Render("Enter a descriptive name for your wallet")

	case StepMnemonicInput:
		content += labelStyle.Render("Recovery Phrase (12 words):") + "\n\n"

		// Display mnemonic input grid (3x4)
		for row := 0; row < 3; row++ {
			for col := 0; col < 4; col++ {
				wordIdx := row*4 + col
				word := m.mnemonicWords[wordIdx]

				// Style based on validation and focus
				style := inputStyle.Copy()
				if wordIdx == m.currentWordIdx {
					style = style.Copy().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(utils.Colours.Blue))
				}

				if word != "" {
					if m.wordsValid[wordIdx] {
						style = style.Copy().Foreground(lipgloss.Color(utils.Colours.Green))
					} else {
						style = style.Copy().Foreground(lipgloss.Color(utils.Colours.Red))
					}
				}

				cursor := ""
				if wordIdx == m.currentWordIdx {
					cursor = "_"
				}

				wordDisplay := fmt.Sprintf("%2d. %-8s", wordIdx+1, word+cursor)
				content += style.Render(wordDisplay) + " "
			}
			content += "\n"
		}

		content += "\n" + helpStyle.Render("Use Tab to move between words, Ctrl+V to paste complete phrase")

	case StepMnemonicValidation:
		content += labelStyle.Render("Validating Recovery Phrase...") + "\n\n"

		if m.mnemonicValid {
			content += successStyle.Render("✓ Valid recovery phrase") + "\n"
			content += labelStyle.Render("Wallet Address Preview:") + "\n"
			content += inputStyle.Render(m.previewAddress) + "\n\n"
			content += helpStyle.Render("Press Enter to continue with password setup")
		} else if m.mnemonicError != "" {
			content += errorStyle.Render("✗ "+m.mnemonicError) + "\n\n"
			content += helpStyle.Render("Press Esc to go back and fix the recovery phrase")
		} else {
			content += helpStyle.Render("Validating recovery phrase and checking for duplicates...")
		}

	case StepPasswordSetup:
		content += labelStyle.Render("Password:") + "\n"
		content += inputStyle.Render(hidePassword(m.password)+"_") + "\n"
		if m.passwordError != "" {
			if m.passwordValid {
				content += successStyle.Render("✓ "+m.passwordError) + "\n"
			} else {
				content += errorStyle.Render("✗ "+m.passwordError) + "\n"
			}
		}
		content += "\n" + helpStyle.Render("Enter a strong password to encrypt your wallet")

	case StepPasswordConfirm:
		content += labelStyle.Render("Confirm Password:") + "\n"
		content += inputStyle.Render(hidePassword(m.confirm)+"_") + "\n"
		if m.confirmError != "" {
			if m.confirmValid {
				content += successStyle.Render("✓ "+m.confirmError) + "\n"
			} else {
				content += errorStyle.Render("✗ "+m.confirmError) + "\n"
			}
		}
		content += "\n" + helpStyle.Render("Re-enter your password to confirm")

	case StepImporting:
		content += helpStyle.Render("Importing and encrypting your wallet...") + "\n"
		content += helpStyle.Render("Please wait...")

	case StepImportComplete:
		content += successStyle.Render("✓ Wallet imported successfully!") + "\n\n"
		content += helpStyle.Render("Your wallet has been encrypted and saved securely.")
		content += "\n" + helpStyle.Render("Press Enter to continue to wallet selection")
	}

	if m.err != nil {
		content += "\n\n" + errorStyle.Render("Error: "+m.err.Error())
	}

	content += "\n\n" + helpStyle.Render("Press Esc to go back")

	return content
}
