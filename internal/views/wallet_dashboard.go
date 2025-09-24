package views

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"rhystmorgan/veWallet/internal/blockchain"
	"rhystmorgan/veWallet/internal/models"
	"rhystmorgan/veWallet/internal/security"
	"rhystmorgan/veWallet/internal/utils"
)

type WalletDashboardModel struct {
	wallet           *models.Wallet
	blockchainClient *blockchain.Client
	sessionManager   *security.SessionManager

	// Balance state
	balanceLoading   bool
	balanceError     error
	lastRefresh      time.Time
	autoRefreshTimer *time.Timer

	// UI state
	selectedMenuItem   int
	showRefreshSpinner bool
	networkStatus      blockchain.NetworkStatus
	feedbackMessage    *FeedbackMessage

	// Menu options
	menuItems []string

	// Performance optimization
	lastRenderTime     time.Time
	renderCache        string
	cacheValid         bool
	lastRefreshRequest time.Time

	// Responsive design
	terminalWidth  int
	terminalHeight int
}

type FeedbackMessage struct {
	Type     FeedbackType
	Message  string
	Duration time.Duration
	ShowTime time.Time
}

type FeedbackType string

const (
	FeedbackSuccess FeedbackType = "success"
	FeedbackError   FeedbackType = "error"
	FeedbackWarning FeedbackType = "warning"
	FeedbackInfo    FeedbackType = "info"
)

type BalanceUpdateMsg struct {
	Balance *blockchain.Balance
	Error   error
}

type RefreshBalanceMsg struct{}

type AutoRefreshMsg struct{}

type CopyAddressMsg struct{}

type FeedbackTimeoutMsg struct{}

func NewWalletDashboardModel(wallet *models.Wallet) *WalletDashboardModel {
	return &WalletDashboardModel{
		wallet:           wallet,
		selectedMenuItem: 0,
		balanceLoading:   false,
		menuItems: []string{
			"Send Transaction",
			"Transaction History",
			"Contacts",
			"Settings",
			"Back to Wallet Selection",
		},
	}
}

func (m *WalletDashboardModel) SetBlockchainClient(client *blockchain.Client) {
	m.blockchainClient = client
	if client != nil {
		m.networkStatus = client.GetStatus()
	}
}

func (m WalletDashboardModel) Init() tea.Cmd {
	return tea.Batch(
		m.refreshBalance(),
		m.startAutoRefresh(),
	)
}

func (m WalletDashboardModel) Update(msg tea.Msg) (WalletDashboardModel, tea.Cmd) {
	var cmds []tea.Cmd

	// Invalidate cache on any state change
	m.cacheValid = false

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.terminalWidth = msg.Width
		m.terminalHeight = msg.Height
		m.cacheValid = false // Invalidate cache on resize

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.selectedMenuItem > 0 {
				m.selectedMenuItem--
			}
		case "down", "j":
			if m.selectedMenuItem < len(m.menuItems)-1 {
				m.selectedMenuItem++
			}
		case "enter", " ":
			switch m.selectedMenuItem {
			case 0:
				return m, NavigateTo(ViewSendTransaction, nil)
			case 1:
				return m, NavigateTo(ViewTransactionHistory, nil)
			case 2:
				return m, NavigateTo(ViewContacts, nil)
			case 3:
				return m, NavigateTo(ViewSettings, nil)
			case 4:
				return m, NavigateTo(ViewWalletSelector, nil)
			}
		case "r", "R":
			// Debounce refresh requests to prevent spam
			if time.Since(m.lastRefreshRequest) > 2*time.Second {
				m.lastRefreshRequest = time.Now()
				m.showFeedback(FeedbackInfo, "Refreshing balance...", 2*time.Second)
				cmds = append(cmds, m.refreshBalance())
			} else {
				m.showFeedback(FeedbackWarning, "Please wait before refreshing again", 2*time.Second)
			}
		case "c", "C":
			cmds = append(cmds, m.copyAddress())
		case "esc":
			return m, NavigateTo(ViewWalletSelector, nil)
		case "?", "F1":
			m.showFeedback(FeedbackInfo, "r: refresh, c: copy address, ↑/↓: navigate, enter: select", 5*time.Second)
		}

	case BalanceUpdateMsg:
		m.balanceLoading = false
		m.showRefreshSpinner = false
		if msg.Error != nil {
			m.balanceError = msg.Error
			m.showFeedback(FeedbackError, fmt.Sprintf("Failed to fetch balance: %s", msg.Error.Error()), 5*time.Second)
		} else {
			m.balanceError = nil
			m.lastRefresh = time.Now()
			// Update wallet balance
			if msg.Balance != nil {
				m.wallet.SetBalance(msg.Balance.VET, msg.Balance.VTHO)
			}
		}

	case RefreshBalanceMsg:
		if !m.balanceLoading {
			m.balanceLoading = true
			m.showRefreshSpinner = true
			cmds = append(cmds, m.refreshBalance())
		}

	case AutoRefreshMsg:
		if m.wallet.NeedsBalanceRefresh() && !m.balanceLoading {
			cmds = append(cmds, m.refreshBalance())
		}
		cmds = append(cmds, m.startAutoRefresh())

	case CopyAddressMsg:
		m.showFeedback(FeedbackSuccess, "Address copied to clipboard!", 3*time.Second)

	case FeedbackTimeoutMsg:
		m.feedbackMessage = nil
	}

	// Update network status if client is available
	if m.blockchainClient != nil {
		m.networkStatus = m.blockchainClient.GetStatus()
	}

	// Handle feedback timeout
	if m.feedbackMessage != nil && time.Since(m.feedbackMessage.ShowTime) > m.feedbackMessage.Duration {
		m.feedbackMessage = nil
	}

	return m, tea.Batch(cmds...)
}

func (m *WalletDashboardModel) View() string {
	// Performance optimization: use cached render if valid and recent
	if m.cacheValid && time.Since(m.lastRenderTime) < 100*time.Millisecond {
		return m.renderCache
	}

	// Main container style
	containerStyle := lipgloss.NewStyle().
		Padding(1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(utils.Colours.Blue))

	// Title style
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Blue)).
		Bold(true).
		Align(lipgloss.Center)

	// Build the main content
	var content strings.Builder

	// Title
	title := fmt.Sprintf("Wallet: %s", m.wallet.Name)
	content.WriteString(titleStyle.Render(title))
	content.WriteString("\n\n")

	// Address section with copy button
	addressSection := m.renderAddressSection()
	content.WriteString(addressSection)
	content.WriteString("\n\n")

	// Balance and network status section
	balanceNetworkSection := m.renderBalanceAndNetworkSection()
	content.WriteString(balanceNetworkSection)
	content.WriteString("\n\n")

	// Quick actions menu
	menuSection := m.renderMenuSection()
	content.WriteString(menuSection)
	content.WriteString("\n\n")

	// Help text
	helpSection := m.renderHelpSection()
	content.WriteString(helpSection)

	// Feedback message
	if m.feedbackMessage != nil {
		content.WriteString("\n\n")
		content.WriteString(m.renderFeedbackMessage())
	}

	result := containerStyle.Render(content.String())

	// Cache the result for performance
	m.renderCache = result
	m.cacheValid = true
	m.lastRenderTime = time.Now()

	return result
}

func (m *WalletDashboardModel) renderAddressSection() string {
	addressStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Background(lipgloss.Color(utils.Colours.Surface0)).
		Padding(0, 1).
		Margin(0, 1)

	copyButtonStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Green)).
		Background(lipgloss.Color(utils.Colours.Surface1)).
		Padding(0, 1).
		Bold(true)

	refreshButtonStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Blue)).
		Background(lipgloss.Color(utils.Colours.Surface1)).
		Padding(0, 1).
		Bold(true)

	// Truncate address for display
	displayAddr := m.wallet.Address
	if len(displayAddr) > 20 {
		displayAddr = displayAddr[:10] + "..." + displayAddr[len(displayAddr)-8:]
	}

	refreshText := "[Refresh]"
	if m.showRefreshSpinner {
		refreshText = "[●]"
	}

	addressLine := lipgloss.JoinHorizontal(
		lipgloss.Left,
		addressStyle.Render(fmt.Sprintf("Address: %s", displayAddr)),
		" ",
		copyButtonStyle.Render("[Copy]"),
		" ",
		refreshButtonStyle.Render(refreshText),
	)

	return addressLine
}

func (m *WalletDashboardModel) renderBalanceAndNetworkSection() string {
	// Balance card
	balanceCard := m.renderBalanceCard()

	// Network status card
	networkCard := m.renderNetworkStatusCard()

	// Responsive layout: stack vertically on narrow terminals
	if m.terminalWidth < 80 {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			balanceCard,
			"",
			networkCard,
		)
	}

	// Join them horizontally on wider terminals
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		balanceCard,
		"  ",
		networkCard,
	)
}

func (m *WalletDashboardModel) renderBalanceCard() string {
	// Responsive card width
	cardWidth := 30
	if m.terminalWidth < 80 {
		cardWidth = m.terminalWidth - 10 // Leave some margin
		if cardWidth < 20 {
			cardWidth = 20
		}
	}

	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(utils.Colours.Green)).
		Padding(1).
		Width(cardWidth)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Green)).
		Bold(true).
		Align(lipgloss.Center)

	balanceStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Align(lipgloss.Left)

	ageStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Subtext0)).
		Italic(true).
		Align(lipgloss.Center)

	var content strings.Builder
	content.WriteString(titleStyle.Render("Balances"))
	content.WriteString("\n\n")

	if m.balanceLoading {
		content.WriteString(balanceStyle.Render("VET:  Loading..."))
		content.WriteString("\n")
		content.WriteString(balanceStyle.Render("VTHO: Loading..."))
	} else if m.balanceError != nil {
		content.WriteString(balanceStyle.Render("VET:  Error"))
		content.WriteString("\n")
		content.WriteString(balanceStyle.Render("VTHO: Error"))
	} else {
		vetBalance, vthoBalance := m.wallet.GetDisplayBalance()
		content.WriteString(balanceStyle.Render(fmt.Sprintf("VET:  %s", vetBalance)))
		content.WriteString("\n")
		content.WriteString(balanceStyle.Render(fmt.Sprintf("VTHO: %s", vthoBalance)))
	}

	content.WriteString("\n\n")

	// Balance age
	if !m.lastRefresh.IsZero() {
		age := time.Since(m.lastRefresh)
		ageText := formatDuration(age)
		content.WriteString(ageStyle.Render(fmt.Sprintf("Updated: %s ago", ageText)))
	} else if m.wallet.CachedBalance != nil {
		age := m.wallet.GetBalanceAge()
		ageText := formatDuration(age)
		content.WriteString(ageStyle.Render(fmt.Sprintf("Updated: %s ago", ageText)))
	} else {
		content.WriteString(ageStyle.Render("Never updated"))
	}

	return cardStyle.Render(content.String())
}

func (m *WalletDashboardModel) renderNetworkStatusCard() string {
	// Responsive card width
	cardWidth := 25
	if m.terminalWidth < 80 {
		cardWidth = m.terminalWidth - 10 // Leave some margin
		if cardWidth < 20 {
			cardWidth = 20
		}
	}

	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(utils.Colours.Blue)).
		Padding(1).
		Width(cardWidth)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Blue)).
		Bold(true).
		Align(lipgloss.Center)

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Align(lipgloss.Left)

	var content strings.Builder
	content.WriteString(titleStyle.Render("Network Status"))
	content.WriteString("\n\n")

	// Connection status
	statusColor := utils.Colours.Red
	statusText := "● Disconnected"
	if m.networkStatus.Connected {
		statusColor = utils.Colours.Green
		statusText = "● Connected"
	}

	connectedStyle := statusStyle.Foreground(lipgloss.Color(statusColor))
	content.WriteString(connectedStyle.Render(statusText))
	content.WriteString("\n")

	// Network type
	networkText := "Unknown"
	if m.blockchainClient != nil {
		// Determine network from URL
		if strings.Contains(m.networkStatus.NodeURL, "mainnet") {
			networkText = "Mainnet"
		} else if strings.Contains(m.networkStatus.NodeURL, "testnet") {
			networkText = "Testnet"
		}
	}
	content.WriteString(statusStyle.Render(networkText))
	content.WriteString("\n")

	// Block height
	if m.networkStatus.BlockHeight > 0 {
		content.WriteString(statusStyle.Render(fmt.Sprintf("Block: %d", m.networkStatus.BlockHeight)))
		content.WriteString("\n")
	}

	// Last checked
	if !m.networkStatus.LastChecked.IsZero() {
		age := time.Since(m.networkStatus.LastChecked)
		ageText := formatDuration(age)
		ageStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(utils.Colours.Subtext0)).
			Italic(true)
		content.WriteString("\n")
		content.WriteString(ageStyle.Render(fmt.Sprintf("Updated: %s ago", ageText)))
	}

	return cardStyle.Render(content.String())
}

func (m *WalletDashboardModel) renderMenuSection() string {
	menuStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(utils.Colours.Mauve)).
		Padding(1)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Mauve)).
		Bold(true).
		Align(lipgloss.Center)

	itemStyle := lipgloss.NewStyle().
		Padding(0, 2)

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Green)).
		Background(lipgloss.Color(utils.Colours.Surface0)).
		Padding(0, 2).
		Bold(true).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color(utils.Colours.Green))

	var content strings.Builder
	content.WriteString(titleStyle.Render("Quick Actions"))
	content.WriteString("\n\n")

	for i, item := range m.menuItems {
		cursor := " "
		if m.selectedMenuItem == i {
			cursor = ">"
		}

		style := itemStyle
		if m.selectedMenuItem == i {
			style = selectedStyle
		}

		content.WriteString(style.Render(fmt.Sprintf("%s %s", cursor, item)))
		content.WriteString("\n")
	}

	return menuStyle.Render(content.String())
}

func (m *WalletDashboardModel) renderHelpSection() string {
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Subtext0)).
		Italic(true).
		Align(lipgloss.Center)

	helpText := "↑/↓: navigate • Enter: select • r: refresh • c: copy address • ?: help • Esc: back"
	return helpStyle.Render(helpText)
}

func (m *WalletDashboardModel) renderFeedbackMessage() string {
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

func (m *WalletDashboardModel) refreshBalance() tea.Cmd {
	if m.blockchainClient == nil || m.wallet == nil {
		return func() tea.Msg {
			return BalanceUpdateMsg{Error: fmt.Errorf("blockchain client or wallet not available")}
		}
	}

	return func() tea.Msg {
		balance, err := m.blockchainClient.RefreshBalance(m.wallet.Address)
		return BalanceUpdateMsg{Balance: balance, Error: err}
	}
}

func (m *WalletDashboardModel) startAutoRefresh() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		return AutoRefreshMsg{}
	})
}

func (m *WalletDashboardModel) copyAddress() tea.Cmd {
	return func() tea.Msg {
		if err := utils.CopyToClipboard(m.wallet.Address); err != nil {
			return BalanceUpdateMsg{Error: fmt.Errorf("failed to copy address: %w", err)}
		}
		return CopyAddressMsg{}
	}
}

func (m *WalletDashboardModel) showFeedback(feedbackType FeedbackType, message string, duration time.Duration) {
	m.feedbackMessage = &FeedbackMessage{
		Type:     feedbackType,
		Message:  message,
		Duration: duration,
		ShowTime: time.Now(),
	}
}

func (m *WalletDashboardModel) SetSessionManager(sessionManager *security.SessionManager) {
	m.sessionManager = sessionManager
}

func formatDuration(d time.Duration) string {
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
