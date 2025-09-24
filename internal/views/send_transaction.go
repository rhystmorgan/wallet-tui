package views

import (
	"fmt"
	"math/big"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"rhystmorgan/veWallet/internal/blockchain"
	"rhystmorgan/veWallet/internal/models"
	"rhystmorgan/veWallet/internal/security"
	"rhystmorgan/veWallet/internal/storage"
	"rhystmorgan/veWallet/internal/utils"
)

type SendTransactionStep int

const (
	StepRecipient SendTransactionStep = iota
	StepAmount
	StepAssetSelection
	StepMetadata
	StepReview
	StepPasswordPrompt
	StepSending
	StepCompleteTransaction
)

type SendTransactionModel struct {
	wallet           *models.Wallet
	blockchainClient *blockchain.Client
	storage          *storage.Storage
	sessionManager   *security.SessionManager

	// Form state
	step             SendTransactionStep
	recipientAddress string
	amount           string
	selectedAsset    blockchain.AssetType

	// Validation state
	addressValid bool
	amountValid  bool
	addressError string
	amountError  string

	// Transaction state
	estimatedGas  *big.Int
	totalFee      *big.Int
	finalBalance  *big.Int
	transaction   *blockchain.Transaction
	transactionID string

	// UI state
	loading         bool
	feedbackMessage *FeedbackMessage
	terminalWidth   int
	terminalHeight  int

	// Password prompt integration
	passwordPrompt     *PasswordPromptModel
	showPasswordPrompt bool
	unlockedWallet     *models.Wallet

	// Enhanced recipient selection
	contactSelector     *ContactSelectorModel
	recentAddresses     *models.RecentAddressManager
	showContactSelector bool
	selectedContact     *models.Contact
	contacts            *models.ContactList

	// Contact creation
	contactCreate     *ContactCreateModel
	showContactCreate bool

	// Transaction templates
	templateSelector     *TemplateSelectorModel
	templateManager      *models.TransactionTemplateManager
	showTemplateSelector bool
	selectedTemplate     *models.TransactionTemplate

	// Transaction metadata
	notes    string
	tags     []string
	category string
}

type GasEstimateMsg struct {
	Gas   *big.Int
	Error error
}

type TransactionBroadcastMsg struct {
	TxID  string
	Error error
}

type TransactionStatusMsg struct {
	Status string
	Error  error
}

func NewSendTransactionModel(wallet *models.Wallet) *SendTransactionModel {
	passwordPrompt := NewPasswordPromptModel()
	contactSelector := NewContactSelectorModel()
	contactCreate := NewContactCreateModel()
	templateSelector := NewTemplateSelectorModel()
	recentAddresses := models.NewRecentAddressManager(50)
	templateManager := models.NewTransactionTemplateManager()

	model := &SendTransactionModel{
		wallet:           wallet,
		step:             StepRecipient,
		selectedAsset:    blockchain.VET,
		passwordPrompt:   passwordPrompt,
		contactSelector:  contactSelector,
		contactCreate:    contactCreate,
		templateSelector: templateSelector,
		templateManager:  templateManager,
		recentAddresses:  recentAddresses,
		contacts:         &models.ContactList{},
	}

	// Set up password prompt callbacks
	passwordPrompt.SetCallbacks(
		model.onPasswordSuccess,
		model.onPasswordCancel,
		model.onPasswordError,
	)

	// Set up contact selector callbacks
	contactSelector.SetCallbacks(
		model.onContactSelected,
		model.onAddressSelected,
		model.onCreateContact,
		model.onContactSelectorCancel,
	)

	// Set up contact create callbacks
	contactCreate.SetCallbacks(
		model.onContactCreated,
		model.onContactCreateCancel,
	)

	// Set up template selector callbacks
	templateSelector.SetCallbacks(
		model.onTemplateSelected,
		model.onCreateTemplate,
		model.onTemplateSelectorCancel,
	)
	templateSelector.SetTemplateManager(templateManager)

	return model
}

func (m *SendTransactionModel) SetBlockchainClient(client *blockchain.Client) {
	m.blockchainClient = client
}

func (m *SendTransactionModel) SetStorage(storage *storage.Storage) {
	m.storage = storage
	m.passwordPrompt.SetStorage(storage)
}

func (m SendTransactionModel) Init() tea.Cmd {
	return nil
}

func (m SendTransactionModel) Update(msg tea.Msg) (SendTransactionModel, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle template selector if visible
	if m.templateSelector.IsVisible() {
		var cmd tea.Cmd
		m.templateSelector, cmd = m.templateSelector.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	}

	// Handle contact create if visible
	if m.contactCreate.IsVisible() {
		var cmd tea.Cmd
		m.contactCreate, cmd = m.contactCreate.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	}

	// Handle contact selector if visible
	if m.contactSelector.IsVisible() {
		var cmd tea.Cmd
		m.contactSelector, cmd = m.contactSelector.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	}

	// Handle password prompt if visible
	if m.passwordPrompt.IsVisible() {
		var cmd tea.Cmd
		*m.passwordPrompt, cmd = m.passwordPrompt.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.terminalWidth = msg.Width
		m.terminalHeight = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.step == StepRecipient {
				return m, NavigateTo(ViewWalletDashboard, nil)
			} else {
				m.goToPreviousStep()
			}

		case "enter":
			cmds = append(cmds, m.handleEnterKey())

		case "tab":
			if m.step == StepAssetSelection {
				m.toggleAsset()
				m.clearGasEstimate()
			}

		case "ctrl+k":
			if m.step == StepRecipient {
				cmds = append(cmds, m.openContactSelector())
			}

		case "ctrl+a":
			if m.step == StepRecipient && m.addressValid && m.recipientAddress != "" {
				m.contactCreate.Show(m.recipientAddress)
				m.showContactCreate = true
			}

		case "ctrl+t":
			if m.step == StepRecipient {
				m.templateSelector.Show()
				m.showTemplateSelector = true
			}

		case "ctrl+s":
			if m.step == StepReview && m.addressValid && m.amountValid {
				cmds = append(cmds, m.saveAsTemplate())
			}

		case "backspace":
			cmds = append(cmds, m.handleBackspace())

		default:
			cmds = append(cmds, m.handleTextInput(msg.String()))
		}

	case GasEstimateMsg:
		m.loading = false
		if msg.Error != nil {
			m.showFeedback(FeedbackError, fmt.Sprintf("Gas estimation failed: %s", msg.Error.Error()), 5*time.Second)
		} else {
			m.estimatedGas = msg.Gas
			m.calculateTotalFee()
		}

	case TransactionBroadcastMsg:
		m.loading = false
		if msg.Error != nil {
			m.showFeedback(FeedbackError, fmt.Sprintf("Transaction failed: %s", msg.Error.Error()), 10*time.Second)
			m.step = StepReview
		} else {
			m.transactionID = msg.TxID
			m.step = StepCompleteTransaction
			m.showFeedback(FeedbackSuccess, "Transaction sent successfully!", 5*time.Second)

			// Add to recent addresses
			if amountWei, err := utils.ValidateAmount(m.amount, 18); err == nil {
				contactName := ""
				if m.selectedContact != nil {
					contactName = m.selectedContact.Name
				}
				m.recentAddresses.AddAddress(m.recipientAddress, contactName, string(m.selectedAsset), amountWei)
			}
		}

	case TransactionStatusMsg:
		if msg.Error == nil {
			m.showFeedback(FeedbackInfo, fmt.Sprintf("Transaction status: %s", msg.Status), 3*time.Second)
		}

	case FeedbackTimeoutMsg:
		m.feedbackMessage = nil
	}

	// Auto-validate current step
	m.validateCurrentStep()

	// Auto-estimate gas when ready
	if m.shouldEstimateGas() && !m.loading {
		cmds = append(cmds, m.estimateGas())
	}

	return m, tea.Batch(cmds...)
}

func (m SendTransactionModel) View() string {
	// Main container
	containerStyle := lipgloss.NewStyle().
		Padding(1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(utils.Colours.Blue))

	var content strings.Builder

	// Title and step indicator
	content.WriteString(m.renderHeader())
	content.WriteString("\n\n")

	// Current step content
	switch m.step {
	case StepRecipient:
		content.WriteString(m.renderRecipientStep())
	case StepAmount:
		content.WriteString(m.renderAmountStep())
	case StepAssetSelection:
		content.WriteString(m.renderAssetStep())
	case StepMetadata:
		content.WriteString(m.renderMetadataStep())
	case StepReview:
		content.WriteString(m.renderReviewStep())
	case StepSending:
		content.WriteString(m.renderSendingStep())
	case StepCompleteTransaction:
		content.WriteString(m.renderCompleteStep())
	}

	content.WriteString("\n\n")

	// Help text
	content.WriteString(m.renderHelpText())

	// Feedback message
	if m.feedbackMessage != nil {
		content.WriteString("\n\n")
		content.WriteString(m.renderFeedbackMessage())
	}

	result := containerStyle.Render(content.String())

	// Overlay template selector if visible
	if m.templateSelector.IsVisible() {
		templateView := m.templateSelector.View()
		return lipgloss.Place(m.terminalWidth, m.terminalHeight, lipgloss.Center, lipgloss.Center,
			lipgloss.JoinVertical(lipgloss.Center, result, templateView))
	}

	// Overlay contact create if visible
	if m.contactCreate.IsVisible() {
		createView := m.contactCreate.View()
		return lipgloss.Place(m.terminalWidth, m.terminalHeight, lipgloss.Center, lipgloss.Center,
			lipgloss.JoinVertical(lipgloss.Center, result, createView))
	}

	// Overlay contact selector if visible
	if m.contactSelector.IsVisible() {
		selectorView := m.contactSelector.renderContent()
		overlayStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(utils.Colours.Blue)).
			Background(lipgloss.Color(utils.Colours.Base)).
			Padding(1).
			Width(60).
			Height(20)

		styledSelector := overlayStyle.Render(selectorView)
		return lipgloss.Place(m.terminalWidth, m.terminalHeight, lipgloss.Center, lipgloss.Center,
			lipgloss.JoinVertical(lipgloss.Center, result, styledSelector))
	}
	// Overlay password prompt if visible
	if m.passwordPrompt.IsVisible() {
		// Center the password prompt
		promptView := m.passwordPrompt.View()
		return lipgloss.Place(m.terminalWidth, m.terminalHeight, lipgloss.Center, lipgloss.Center,
			lipgloss.JoinVertical(lipgloss.Center, result, promptView))
	}

	return result
}

func (m *SendTransactionModel) renderHeader() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Blue)).
		Bold(true).
		Align(lipgloss.Center)

	stepStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Subtext0)).
		Align(lipgloss.Center)

	stepNames := []string{"Recipient", "Amount", "Asset", "Notes", "Review", "Send"}
	stepIndicator := utils.FormatStepIndicator(int(m.step), len(stepNames), stepNames)

	var content strings.Builder
	content.WriteString(titleStyle.Render("Send Transaction"))
	content.WriteString("\n")
	content.WriteString(stepStyle.Render(stepIndicator))

	return content.String()
}

func (m *SendTransactionModel) renderRecipientStep() string {
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Bold(true)

	inputStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Background(lipgloss.Color(utils.Colours.Surface0)).
		Padding(0, 1).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(utils.Colours.Surface1)).
		Width(50)

	buttonStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Blue)).
		Background(lipgloss.Color(utils.Colours.Surface0)).
		Padding(0, 1).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(utils.Colours.Blue))

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Red))

	contactStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Green))

	var content strings.Builder
	content.WriteString(labelStyle.Render("Recipient Address:"))
	content.WriteString("\n\n")

	// Address input with contact selector button
	displayAddr := m.recipientAddress
	if len(displayAddr) > 48 {
		displayAddr = displayAddr[:48]
	}

	inputLine := lipgloss.JoinHorizontal(lipgloss.Left,
		inputStyle.Render(displayAddr+"█"),
		" ",
		buttonStyle.Render("📋"),
		" ",
		buttonStyle.Render("👥"),
	)
	content.WriteString(inputLine)
	content.WriteString("\n")

	// Show contact name if address matches a contact
	if m.selectedContact != nil {
		content.WriteString("\n")
		contactInfo := fmt.Sprintf("Contact: %s", m.selectedContact.Name)
		if m.selectedContact.Notes != "" {
			contactInfo += fmt.Sprintf(" (%s)", m.selectedContact.Notes)
		}
		content.WriteString(contactStyle.Render(contactInfo))
		content.WriteString("\n")
	}

	// Validation error
	if m.addressError != "" {
		content.WriteString("\n")
		content.WriteString(errorStyle.Render("✗ " + m.addressError))
	} else if m.addressValid && m.recipientAddress != "" {
		validStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(utils.Colours.Green))
		content.WriteString("\n")
		content.WriteString(validStyle.Render("✓ Valid VeChain address"))
	}

	return content.String()
}

func (m *SendTransactionModel) renderAmountStep() string {
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Bold(true)

	inputStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Background(lipgloss.Color(utils.Colours.Surface0)).
		Padding(0, 1).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(utils.Colours.Surface1)).
		Width(30)

	balanceStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Subtext0))

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Red))

	var content strings.Builder
	content.WriteString(labelStyle.Render(fmt.Sprintf("Amount (%s):", m.selectedAsset)))
	content.WriteString("\n\n")

	// Amount input
	content.WriteString(inputStyle.Render(m.amount + "█"))
	content.WriteString("\n\n")

	// Available balance
	if m.wallet.CachedBalance != nil {
		var balance *big.Int
		if m.selectedAsset == blockchain.VET {
			balance = m.wallet.CachedBalance.VET
		} else {
			balance = m.wallet.CachedBalance.VTHO
		}
		balanceText := fmt.Sprintf("Available: %s", utils.FormatBalance(balance, string(m.selectedAsset), 4))
		content.WriteString(balanceStyle.Render(balanceText))
		content.WriteString("\n")
	}

	// Validation error
	if m.amountError != "" {
		content.WriteString("\n")
		content.WriteString(errorStyle.Render("✗ " + m.amountError))
	} else if m.amountValid && m.amount != "" {
		validStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(utils.Colours.Green))
		content.WriteString("\n")
		content.WriteString(validStyle.Render("✓ Valid amount"))
	}

	return content.String()
}

func (m *SendTransactionModel) renderMetadataStep() string {
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Bold(true)

	inputStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Background(lipgloss.Color(utils.Colours.Surface0)).
		Padding(0, 1).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(utils.Colours.Surface1)).
		Width(50)

	var content strings.Builder
	content.WriteString(labelStyle.Render("Transaction Notes (optional):"))
	content.WriteString("\n\n")

	// Notes input
	content.WriteString(inputStyle.Render(m.notes + "█"))
	content.WriteString("\n\n")

	// Tags display
	content.WriteString(labelStyle.Render("Tags:"))
	content.WriteString("\n")
	if len(m.tags) > 0 {
		tagStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(utils.Colours.Blue)).
			Background(lipgloss.Color(utils.Colours.Surface0)).
			Padding(0, 1).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(utils.Colours.Blue))

		var tagStrings []string
		for _, tag := range m.tags {
			tagStrings = append(tagStrings, tagStyle.Render("#"+tag))
		}
		content.WriteString(lipgloss.JoinHorizontal(lipgloss.Left, tagStrings...))
	} else {
		content.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color(utils.Colours.Subtext0)).
			Italic(true).
			Render("No tags added"))
	}

	return content.String()
}

func (m *SendTransactionModel) renderAssetStep() string {
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Bold(true)

	optionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Padding(1, 2).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(utils.Colours.Surface1))

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Green)).
		Background(lipgloss.Color(utils.Colours.Surface0)).
		Padding(1, 2).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(utils.Colours.Green)).
		Bold(true)

	var content strings.Builder
	content.WriteString(labelStyle.Render("Select Asset:"))
	content.WriteString("\n\n")

	// VET option
	vetStyle := optionStyle
	if m.selectedAsset == blockchain.VET {
		vetStyle = selectedStyle
	}

	vetBalance := "0"
	if m.wallet.CachedBalance != nil {
		vetBalance = utils.FormatAmount(m.wallet.CachedBalance.VET, 4)
	}
	vetText := fmt.Sprintf("VET\nBalance: %s", vetBalance)
	content.WriteString(vetStyle.Render(vetText))
	content.WriteString("  ")

	// VTHO option
	vthoStyle := optionStyle
	if m.selectedAsset == blockchain.VTHO {
		vthoStyle = selectedStyle
	}

	vthoBalance := "0"
	if m.wallet.CachedBalance != nil {
		vthoBalance = utils.FormatAmount(m.wallet.CachedBalance.VTHO, 4)
	}
	vthoText := fmt.Sprintf("VTHO\nBalance: %s", vthoBalance)
	content.WriteString(vthoStyle.Render(vthoText))

	return content.String()
}

func (m *SendTransactionModel) renderReviewStep() string {
	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(utils.Colours.Green)).
		Padding(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Bold(true)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Green))

	var content strings.Builder
	content.WriteString(labelStyle.Render("Transaction Review"))
	content.WriteString("\n\n")

	// Transaction details
	details := strings.Builder{}
	details.WriteString(fmt.Sprintf("From:     %s\n", utils.FormatAddress(m.wallet.Address, 10, 8)))
	details.WriteString(fmt.Sprintf("To:       %s\n", utils.FormatAddress(m.recipientAddress, 10, 8)))
	details.WriteString(fmt.Sprintf("Amount:   %s %s\n", m.amount, m.selectedAsset))

	if m.notes != "" {
		details.WriteString(fmt.Sprintf("Notes:    %s\n", m.notes))
	}

	if len(m.tags) > 0 {
		var tagStrings []string
		for _, tag := range m.tags {
			tagStrings = append(tagStrings, "#"+tag)
		}
		details.WriteString(fmt.Sprintf("Tags:     %s\n", strings.Join(tagStrings, " ")))
	}

	if m.estimatedGas != nil {
		gasText := utils.FormatAmount(m.estimatedGas, 4)
		details.WriteString(fmt.Sprintf("Gas Fee:  %s VET\n", gasText))

		if m.totalFee != nil {
			totalText := utils.FormatAmount(m.totalFee, 4)
			details.WriteString(fmt.Sprintf("Total:    %s VET\n", totalText))
		}
	} else if m.loading {
		details.WriteString("Gas Fee:  Calculating...\n")
	} else {
		details.WriteString("Gas Fee:  Unknown\n")
	}

	content.WriteString(cardStyle.Render(details.String()))

	// Final balance
	if m.finalBalance != nil {
		content.WriteString("\n\n")
		balanceText := fmt.Sprintf("Balance After Transaction: %s VET", utils.FormatAmount(m.finalBalance, 4))
		content.WriteString(valueStyle.Render(balanceText))
	}

	return content.String()
}

func (m *SendTransactionModel) renderSendingStep() string {
	loadingStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Blue)).
		Bold(true).
		Align(lipgloss.Center)

	var content strings.Builder
	content.WriteString(loadingStyle.Render("Sending Transaction..."))
	content.WriteString("\n\n")
	content.WriteString(loadingStyle.Render("Please wait while your transaction is broadcast to the network."))

	return content.String()
}

func (m *SendTransactionModel) renderCompleteStep() string {
	successStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Green)).
		Bold(true).
		Align(lipgloss.Center)

	txStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Background(lipgloss.Color(utils.Colours.Surface0)).
		Padding(0, 1)

	var content strings.Builder
	content.WriteString(successStyle.Render("✓ Transaction Sent Successfully!"))
	content.WriteString("\n\n")

	if m.transactionID != "" {
		content.WriteString("Transaction ID:")
		content.WriteString("\n")
		content.WriteString(txStyle.Render(m.transactionID))
		content.WriteString("\n\n")
	}

	content.WriteString("Your transaction has been broadcast to the VeChain network.")
	content.WriteString("\n")
	content.WriteString("It may take a few moments to be confirmed.")

	return content.String()
}

func (m *SendTransactionModel) renderHelpText() string {
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Subtext0)).
		Italic(true).
		Align(lipgloss.Center)

	var helpText string
	switch m.step {
	case StepRecipient:
		helpText = "Enter recipient address • Ctrl+K: select contact • Ctrl+T: use template • Ctrl+A: add to contacts • Enter: next • Esc: back"
	case StepAmount:
		helpText = "Enter amount • Enter: next • Esc: back"
	case StepAssetSelection:
		helpText = "Tab: toggle asset • Enter: next • Esc: back"
	case StepMetadata:
		helpText = "Enter notes (optional) • Enter: next • Esc: back"
	case StepReview:
		helpText = "Enter: send transaction • Ctrl+S: save as template • Esc: back"
	case StepCompleteTransaction:
		helpText = "Enter: return to dashboard • Esc: back"
	default:
		helpText = "Please wait..."
	}

	return helpStyle.Render(helpText)
}

func (m *SendTransactionModel) renderFeedbackMessage() string {
	if m.feedbackMessage == nil {
		return ""
	}

	var color string
	switch m.feedbackMessage.Type {
	case FeedbackSuccess:
		color = utils.Colours.Green
	case FeedbackError:
		color = utils.Colours.Red
	case FeedbackWarning:
		color = utils.Colours.Yellow
	case FeedbackInfo:
		color = utils.Colours.Blue
	default:
		color = utils.Colours.Text
	}

	feedbackStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(color)).
		Background(lipgloss.Color(utils.Colours.Surface0)).
		Padding(0, 1).
		Bold(true).
		Align(lipgloss.Center)

	return feedbackStyle.Render(m.feedbackMessage.Message)
}

// Password prompt callback methods
func (m *SendTransactionModel) onPasswordSuccess(wallet *models.Wallet) tea.Cmd {
	m.unlockedWallet = wallet
	m.passwordPrompt.Hide()
	m.step = StepSending
	return m.broadcastTransaction()
}

func (m *SendTransactionModel) onPasswordCancel() tea.Cmd {
	m.passwordPrompt.Hide()
	return nil
}

func (m *SendTransactionModel) onPasswordError(err error) tea.Cmd {
	m.passwordPrompt.Hide()
	m.showFeedback(FeedbackError, fmt.Sprintf("Password error: %s", err.Error()), 5*time.Second)
	return nil
}

// Navigation methods
func (m *SendTransactionModel) goToPreviousStep() {
	if m.step > StepRecipient {
		m.step--
	}
}

func (m *SendTransactionModel) goToNextStep() {
	switch m.step {
	case StepRecipient:
		if m.addressValid {
			m.step = StepAmount
		}
	case StepAmount:
		if m.amountValid {
			m.step = StepAssetSelection
		}
	case StepAssetSelection:
		m.step = StepMetadata
	case StepMetadata:
		m.step = StepReview
	case StepReview:
		m.passwordPrompt.SetWallet(m.wallet)
		m.passwordPrompt.Show("Unlock Wallet", "Enter your wallet password to sign the transaction")
	case StepCompleteTransaction:
		// Return to dashboard
	}
}

// Input handling methods
func (m *SendTransactionModel) handleEnterKey() tea.Cmd {
	switch m.step {
	case StepCompleteTransaction:
		return NavigateTo(ViewWalletDashboard, nil)
	default:
		m.goToNextStep()
		return nil
	}
}

func (m *SendTransactionModel) handleBackspace() tea.Cmd {
	switch m.step {
	case StepRecipient:
		if len(m.recipientAddress) > 0 {
			m.recipientAddress = m.recipientAddress[:len(m.recipientAddress)-1]
		}
	case StepAmount:
		if len(m.amount) > 0 {
			m.amount = m.amount[:len(m.amount)-1]
		}
	case StepMetadata:
		if len(m.notes) > 0 {
			m.notes = m.notes[:len(m.notes)-1]
		}
	}
	return nil
}

func (m *SendTransactionModel) handleTextInput(input string) tea.Cmd {
	if len(input) != 1 {
		return nil
	}

	switch m.step {
	case StepRecipient:
		m.recipientAddress += input
	case StepAmount:
		// Only allow digits, decimal point, and basic editing
		if (input >= "0" && input <= "9") || input == "." {
			m.amount += input
		}
	case StepMetadata:
		// Allow all printable characters for notes
		if input >= " " && len(m.notes) < 200 {
			m.notes += input
		}
	}
	return nil
}

func (m *SendTransactionModel) toggleAsset() {
	if m.selectedAsset == blockchain.VET {
		m.selectedAsset = blockchain.VTHO
	} else {
		m.selectedAsset = blockchain.VET
	}
}

// Validation methods
func (m *SendTransactionModel) validateCurrentStep() {
	switch m.step {
	case StepRecipient:
		m.validateAddress()
	case StepAmount:
		m.validateAmount()
	}
}

func (m *SendTransactionModel) validateAddress() {
	err := utils.ValidateVeChainAddress(m.recipientAddress)
	if err != nil {
		m.addressValid = false
		m.addressError = err.Error()
	} else {
		// Check if not sending to self
		if strings.EqualFold(m.recipientAddress, m.wallet.Address) {
			m.addressValid = false
			m.addressError = "Cannot send to your own address"
		} else {
			m.addressValid = true
			m.addressError = ""
		}
	}
}

func (m *SendTransactionModel) validateAmount() {
	if m.amount == "" {
		m.amountValid = false
		m.amountError = ""
		return
	}

	// Parse amount
	amountWei, err := utils.ValidateAmount(m.amount, 18)
	if err != nil {
		m.amountValid = false
		m.amountError = err.Error()
		return
	}

	// Check against balance
	if m.wallet.CachedBalance == nil {
		m.amountValid = false
		m.amountError = "Balance not available"
		return
	}

	// Validate against balance based on asset type
	if m.selectedAsset == blockchain.VET {
		gasFee := m.estimatedGas
		if gasFee == nil {
			gasFee = big.NewInt(21000) // Default gas estimate
		}
		err = utils.ValidateAmountAgainstBalance(amountWei, m.wallet.CachedBalance.VET, gasFee)
	} else {
		gasFee := m.estimatedGas
		if gasFee == nil {
			gasFee = big.NewInt(80000) // Default gas estimate for VTHO
		}
		err = utils.ValidateVTHOAmountAgainstBalance(amountWei, m.wallet.CachedBalance.VTHO, m.wallet.CachedBalance.VET, gasFee)
	}

	if err != nil {
		m.amountValid = false
		m.amountError = err.Error()
	} else {
		m.amountValid = true
		m.amountError = ""
	}
}

// Gas estimation methods
func (m *SendTransactionModel) shouldEstimateGas() bool {
	return m.step >= StepReview && m.addressValid && m.amountValid && m.estimatedGas == nil
}

func (m *SendTransactionModel) estimateGas() tea.Cmd {
	if m.blockchainClient == nil {
		return nil
	}

	m.loading = true
	return func() tea.Msg {
		// Parse amount for gas estimation
		amountWei, err := utils.ValidateAmount(m.amount, 18)
		if err != nil {
			return GasEstimateMsg{Error: err}
		}

		// Create transaction for gas estimation
		tx := &blockchain.Transaction{
			From:   m.wallet.Address,
			To:     m.recipientAddress,
			Amount: amountWei,
			Asset:  m.selectedAsset,
		}

		gas, err := m.blockchainClient.EstimateGas(tx)
		return GasEstimateMsg{Gas: gas, Error: err}
	}
}

func (m *SendTransactionModel) clearGasEstimate() {
	m.estimatedGas = nil
	m.totalFee = nil
	m.finalBalance = nil
}

func (m *SendTransactionModel) calculateTotalFee() {
	if m.estimatedGas == nil || m.wallet.CachedBalance == nil {
		return
	}

	// For VET transfers, total = amount + gas
	// For VTHO transfers, total = gas (paid in VET)
	amountWei, err := utils.ValidateAmount(m.amount, 18)
	if err != nil {
		return
	}

	if m.selectedAsset == blockchain.VET {
		m.totalFee = new(big.Int).Add(amountWei, m.estimatedGas)
		m.finalBalance = new(big.Int).Sub(m.wallet.CachedBalance.VET, m.totalFee)
	} else {
		m.totalFee = m.estimatedGas
		m.finalBalance = new(big.Int).Sub(m.wallet.CachedBalance.VET, m.estimatedGas)
	}
}

// Transaction methods
func (m *SendTransactionModel) broadcastTransaction() tea.Cmd {
	if m.blockchainClient == nil || m.unlockedWallet == nil {
		return func() tea.Msg {
			return TransactionBroadcastMsg{Error: fmt.Errorf("blockchain client or wallet not available")}
		}
	}

	return func() tea.Msg {
		// Parse amount
		amountWei, err := utils.ValidateAmount(m.amount, 18)
		if err != nil {
			return TransactionBroadcastMsg{Error: err}
		}

		// Build transaction
		tx, err := m.blockchainClient.BuildTransaction(m.wallet.Address, m.recipientAddress, amountWei, m.selectedAsset)
		if err != nil {
			return TransactionBroadcastMsg{Error: fmt.Errorf("failed to build transaction: %w", err)}
		}

		// Sign transaction
		signedTx, err := m.blockchainClient.SignTransaction(tx, m.unlockedWallet.PrivateKey)
		if err != nil {
			return TransactionBroadcastMsg{Error: fmt.Errorf("failed to sign transaction: %w", err)}
		}

		// Broadcast transaction
		txID, err := m.blockchainClient.BroadcastTransaction(signedTx)
		if err != nil {
			return TransactionBroadcastMsg{Error: fmt.Errorf("failed to broadcast transaction: %w", err)}
		}

		return TransactionBroadcastMsg{TxID: txID}
	}
}

func (m *SendTransactionModel) SetSessionManager(sessionManager *security.SessionManager) {
	m.sessionManager = sessionManager
}

// Feedback methods
func (m *SendTransactionModel) showFeedback(feedbackType FeedbackType, message string, duration time.Duration) {
	m.feedbackMessage = &FeedbackMessage{
		Type:     feedbackType,
		Message:  message,
		Duration: duration,
		ShowTime: time.Now(),
	}
}

// Contact selector callback methods
func (m *SendTransactionModel) onContactSelected(contact *models.Contact) tea.Cmd {
	m.recipientAddress = contact.Address
	m.selectedContact = contact
	m.showContactSelector = false
	m.validateAddress()
	return nil
}

func (m *SendTransactionModel) onAddressSelected(address, contactName string) tea.Cmd {
	m.recipientAddress = address
	// Try to find the contact by address
	if m.contacts != nil {
		if contact := m.contacts.FindByAddress(address); contact != nil {
			m.selectedContact = contact
		}
	}
	m.showContactSelector = false
	m.validateAddress()
	return nil
}

func (m *SendTransactionModel) onCreateContact() tea.Cmd {
	m.showContactSelector = false
	m.contactCreate.Show(m.recipientAddress)
	m.showContactCreate = true
	return nil
}

func (m *SendTransactionModel) onContactSelectorCancel() tea.Cmd {
	m.showContactSelector = false
	return nil
}

func (m *SendTransactionModel) onContactCreated(contact *models.Contact) tea.Cmd {
	// Add the contact to our contact list
	if m.contacts != nil {
		m.contacts.Add(contact)
	}

	// Set the created contact as selected
	m.recipientAddress = contact.Address
	m.selectedContact = contact
	m.showContactCreate = false
	m.validateAddress()

	m.showFeedback(FeedbackSuccess, "Contact created and selected!", 3*time.Second)
	return nil
}

func (m *SendTransactionModel) onContactCreateCancel() tea.Cmd {
	m.showContactCreate = false
	return nil
}

func (m *SendTransactionModel) onTemplateSelected(template *models.TransactionTemplate) tea.Cmd {
	// Apply template to current transaction
	m.recipientAddress = template.ToAddress
	if template.Amount != nil {
		m.amount = utils.FormatAmount(template.Amount, 18)
	}
	m.selectedAsset = blockchain.AssetType(template.Asset)
	m.selectedTemplate = template

	// Try to find the contact by address
	if m.contacts != nil {
		if contact := m.contacts.FindByAddress(template.ToAddress); contact != nil {
			m.selectedContact = contact
		}
	}

	// Mark template as used
	template.Use()

	m.showTemplateSelector = false
	m.validateAddress()
	m.validateAmount()

	m.showFeedback(FeedbackSuccess, "Template applied!", 3*time.Second)
	return nil
}

func (m *SendTransactionModel) onCreateTemplate() tea.Cmd {
	// For now, just close the selector
	// This will be implemented when we add template creation
	m.showTemplateSelector = false
	return nil
}

func (m *SendTransactionModel) onTemplateSelectorCancel() tea.Cmd {
	m.showTemplateSelector = false
	return nil
}

func (m *SendTransactionModel) saveAsTemplate() tea.Cmd {
	// Parse amount
	amountWei, err := utils.ValidateAmount(m.amount, 18)
	if err != nil {
		m.showFeedback(FeedbackError, "Invalid amount for template", 3*time.Second)
		return nil
	}

	// Generate template name
	contactName := ""
	if m.selectedContact != nil {
		contactName = m.selectedContact.Name
	}

	templateName := fmt.Sprintf("Transaction to %s", contactName)
	if contactName == "" {
		templateName = fmt.Sprintf("Transaction to %s", utils.FormatAddress(m.recipientAddress, 6, 4))
	}

	// Create template
	template := models.NewTransactionTemplate(
		templateName,
		"Saved from send transaction",
		m.recipientAddress,
		contactName,
		string(m.selectedAsset),
		m.notes,
		amountWei,
		m.tags,
	)

	// Add to template manager
	m.templateManager.AddTemplate(template)

	m.showFeedback(FeedbackSuccess, "Transaction saved as template!", 3*time.Second)
	return nil
}

func (m *SendTransactionModel) openContactSelector() tea.Cmd {
	// Load contacts from storage if available
	if m.storage != nil {
		// This would load contacts from storage
		// For now, we'll use the existing contacts
		m.contactSelector.SetContacts(m.contacts)
	}

	// Set recent addresses
	recentAddrs := m.recentAddresses.GetRecentAddresses(10)
	m.contactSelector.SetRecentAddresses(recentAddrs)

	m.contactSelector.Show()
	m.showContactSelector = true
	return nil
}
