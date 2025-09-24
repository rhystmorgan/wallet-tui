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

type WalletCreateStep int

const (
	StepName WalletCreateStep = iota
	StepPassword
	StepConfirmPassword
	StepMnemonicDisplay
	StepMnemonicVerification
	StepCreating
	StepComplete
)

type WalletCreateModel struct {
	step                WalletCreateStep
	name                string
	password            string
	confirm             string
	mnemonic            string
	mnemonicWords       []string
	verificationWords   []string
	verificationIndices []int
	currentWordIndex    int
	storage             *storage.Storage

	// Validation states
	nameValid     bool
	passwordValid bool
	confirmValid  bool

	// Error messages
	nameError     string
	passwordError string
	confirmError  string

	// UI state
	cursor int
	err    error
}

type WalletCreatedMsg struct {
	Wallet *models.Wallet
}

func NewWalletCreateModel() *WalletCreateModel {
	return &WalletCreateModel{
		step:                StepName,
		cursor:              0,
		verificationIndices: []int{2, 5, 8}, // Ask for 3rd, 6th, and 9th words
		verificationWords:   make([]string, 3),
	}
}

func (m *WalletCreateModel) SetStorage(storage *storage.Storage) {
	m.storage = storage
}

func (m WalletCreateModel) Init() tea.Cmd {
	return nil
}

func (m WalletCreateModel) Update(msg tea.Msg) (WalletCreateModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.step == StepName {
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
			if m.step == StepMnemonicVerification {
				m.currentWordIndex = (m.currentWordIndex + 1) % 3
			}

		default:
			if len(msg.String()) == 1 {
				return m.handleCharInput(msg.String())
			}
		}

	case WalletCreatedMsg:
		return m, LoadWallet(msg.Wallet)
	}

	return m, nil
}

func (m WalletCreateModel) handleEnter() (WalletCreateModel, tea.Cmd) {
	switch m.step {
	case StepName:
		if m.validateName() {
			m.step = StepPassword
		}

	case StepPassword:
		if m.validatePassword() {
			m.step = StepConfirmPassword
		}

	case StepConfirmPassword:
		if m.validateConfirmPassword() {
			// Generate mnemonic
			mnemonic, err := hdwallet.NewMnemonic(128) // 12 words
			if err != nil {
				m.err = fmt.Errorf("failed to generate mnemonic: %w", err)
				return m, nil
			}
			m.mnemonic = mnemonic
			m.mnemonicWords = utils.SplitMnemonic(mnemonic)
			m.step = StepMnemonicDisplay
		}

	case StepMnemonicDisplay:
		m.step = StepMnemonicVerification

	case StepMnemonicVerification:
		if m.validateMnemonicVerification() {
			m.step = StepCreating
			return m, m.createWallet()
		}

	case StepComplete:
		return m, NavigateTo(ViewWalletSelector, nil)
	}

	return m, nil
}

func (m WalletCreateModel) handleBackspace() (WalletCreateModel, tea.Cmd) {
	switch m.step {
	case StepName:
		if len(m.name) > 0 {
			m.name = m.name[:len(m.name)-1]
			m.validateName()
		}

	case StepPassword:
		if len(m.password) > 0 {
			m.password = m.password[:len(m.password)-1]
			m.validatePassword()
		}

	case StepConfirmPassword:
		if len(m.confirm) > 0 {
			m.confirm = m.confirm[:len(m.confirm)-1]
			m.validateConfirmPassword()
		}

	case StepMnemonicVerification:
		if len(m.verificationWords[m.currentWordIndex]) > 0 {
			m.verificationWords[m.currentWordIndex] = m.verificationWords[m.currentWordIndex][:len(m.verificationWords[m.currentWordIndex])-1]
		}
	}

	return m, nil
}

func (m WalletCreateModel) handleCharInput(char string) (WalletCreateModel, tea.Cmd) {
	switch m.step {
	case StepName:
		m.name += char
		m.validateName()

	case StepPassword:
		m.password += char
		m.validatePassword()

	case StepConfirmPassword:
		m.confirm += char
		m.validateConfirmPassword()

	case StepMnemonicVerification:
		m.verificationWords[m.currentWordIndex] += char
	}

	return m, nil
}

func (m *WalletCreateModel) validateName() bool {
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

func (m *WalletCreateModel) validatePassword() bool {
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

func (m *WalletCreateModel) validateConfirmPassword() bool {
	if m.password != m.confirm {
		m.confirmError = "Passwords do not match"
		m.confirmValid = false
		return false
	}
	m.confirmError = "Passwords match ✓"
	m.confirmValid = true
	return true
}

func (m *WalletCreateModel) validateMnemonicVerification() bool {
	for i, wordIndex := range m.verificationIndices {
		expectedWord := m.mnemonicWords[wordIndex]
		providedWord := strings.TrimSpace(strings.ToLower(m.verificationWords[i]))
		if expectedWord != providedWord {
			m.err = fmt.Errorf("word %d is incorrect. Expected '%s', got '%s'", wordIndex+1, expectedWord, providedWord)
			return false
		}
	}
	return true
}

func (m WalletCreateModel) createWallet() tea.Cmd {
	return func() tea.Msg {
		// Create wallet from mnemonic
		wallet, err := models.NewWallet(m.name, m.mnemonic)
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
		m.mnemonic = ""
		for i := range m.mnemonicWords {
			m.mnemonicWords[i] = ""
		}
		for i := range m.verificationWords {
			m.verificationWords[i] = ""
		}

		return WalletCreatedMsg{Wallet: wallet}
	}
}

func (m WalletCreateModel) View() string {
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

	warningStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Yellow)).
		Bold(true)

	var content string
	content += titleStyle.Render("Create New Wallet") + "\n\n"

	// Progress indicator
	progress := fmt.Sprintf("Step %d of 5", int(m.step)+1)
	if m.step >= StepCreating {
		progress = "Creating wallet..."
	}
	content += helpStyle.Render(progress) + "\n\n"

	switch m.step {
	case StepName:
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

	case StepPassword:
		content += labelStyle.Render("Password:") + "\n"
		content += inputStyle.Render(hidePassword(m.password)+"_") + "\n"
		if m.passwordError != "" {
			if m.passwordValid {
				content += successStyle.Render("✓ "+m.passwordError) + "\n"
			} else {
				content += errorStyle.Render("✗ "+m.passwordError) + "\n"
			}
		}
		content += "\n" + helpStyle.Render("Enter a strong password (min 8 chars, mixed case, numbers)")

	case StepConfirmPassword:
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

	case StepMnemonicDisplay:
		content += warningStyle.Render("⚠️  IMPORTANT: Backup Your Recovery Phrase") + "\n\n"
		content += labelStyle.Render("Your 12-word recovery phrase:") + "\n\n"

		// Display mnemonic in a grid
		mnemonicStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(utils.Colours.Green)).
			Background(lipgloss.Color(utils.Colours.Surface0)).
			Padding(0, 1).
			Margin(0, 1)

		for i, word := range m.mnemonicWords {
			if i%4 == 0 && i > 0 {
				content += "\n"
			}
			content += mnemonicStyle.Render(fmt.Sprintf("%2d. %s", i+1, word))
		}

		content += "\n\n" + warningStyle.Render("Write this down and store it safely!") + "\n"
		content += helpStyle.Render("Anyone with this phrase can access your wallet.\nPress Enter when you have safely recorded it.")

	case StepMnemonicVerification:
		content += labelStyle.Render("Verify Your Recovery Phrase") + "\n\n"
		content += helpStyle.Render("Enter the following words to confirm you wrote them down:") + "\n\n"

		for i, wordIndex := range m.verificationIndices {
			cursor := ""
			if i == m.currentWordIndex {
				cursor = "_"
			}

			style := inputStyle
			if i == m.currentWordIndex {
				style = style.Copy().Border(lipgloss.RoundedBorder())
			}

			content += labelStyle.Render(fmt.Sprintf("Word %d:", wordIndex+1)) + " "
			content += style.Render(m.verificationWords[i]+cursor) + "\n"
		}

		content += "\n" + helpStyle.Render("Use Tab to move between fields, Enter to continue")

	case StepCreating:
		content += helpStyle.Render("Creating and encrypting your wallet...") + "\n"
		content += helpStyle.Render("Please wait...")

	case StepComplete:
		content += successStyle.Render("✓ Wallet created successfully!") + "\n\n"
		content += helpStyle.Render("Your wallet has been encrypted and saved securely.")
		content += "\n" + helpStyle.Render("Press Enter to continue to wallet selection")
	}

	if m.err != nil {
		content += "\n\n" + errorStyle.Render("Error: "+m.err.Error())
	}

	content += "\n\n" + helpStyle.Render("Press Esc to go back")

	return content
}

func hidePassword(password string) string {
	return strings.Repeat("*", len(password))
}
