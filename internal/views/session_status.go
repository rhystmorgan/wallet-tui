package views

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"rhystmorgan/veWallet/internal/security"
	"rhystmorgan/veWallet/internal/utils"
)

type SessionStatusModel struct {
	sessionManager *security.SessionManager
	walletID       string
	showDetails    bool
	timeRemaining  time.Duration
	warningShown   bool
	lastUpdate     time.Time
}

type SessionTimeoutWarningMsg struct {
	WalletID      string
	TimeRemaining time.Duration
}

type SessionExpiredMsg struct {
	WalletID string
}

func NewSessionStatusModel(sessionManager *security.SessionManager, walletID string) *SessionStatusModel {
	return &SessionStatusModel{
		sessionManager: sessionManager,
		walletID:       walletID,
		showDetails:    false,
		lastUpdate:     time.Now(),
	}
}

func (m SessionStatusModel) Init() tea.Cmd {
	return m.checkSessionStatus()
}

func (m SessionStatusModel) Update(msg tea.Msg) (SessionStatusModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "s":
			m.showDetails = !m.showDetails
		}

	case SessionTimeoutWarningMsg:
		if msg.WalletID == m.walletID {
			m.timeRemaining = msg.TimeRemaining
			m.warningShown = true
		}

	case SessionExpiredMsg:
		if msg.WalletID == m.walletID {
			m.timeRemaining = 0
		}

	case time.Time:
		return m, m.checkSessionStatus()
	}

	return m, nil
}

func (m SessionStatusModel) View() string {
	if m.sessionManager == nil {
		return ""
	}

	status := m.sessionManager.GetSessionStatus(m.walletID)
	timeRemaining := m.sessionManager.GetTimeRemaining(m.walletID)

	return m.renderSessionStatus(status, timeRemaining)
}

func (m *SessionStatusModel) renderSessionStatus(status security.SessionStatus, timeRemaining time.Duration) string {
	var statusText string
	var statusColor lipgloss.Color

	switch status {
	case security.SessionStatusActive:
		statusText = "●"
		statusColor = lipgloss.Color(utils.Colours.Green)
	case security.SessionStatusExpiring:
		statusText = "●"
		statusColor = lipgloss.Color(utils.Colours.Yellow)
	case security.SessionStatusExpired:
		statusText = "●"
		statusColor = lipgloss.Color(utils.Colours.Red)
	case security.SessionStatusInactive:
		statusText = "○"
		statusColor = lipgloss.Color(utils.Colours.Surface0)
	default:
		statusText = "?"
		statusColor = lipgloss.Color(utils.Colours.Surface0)
	}

	statusStyle := lipgloss.NewStyle().
		Foreground(statusColor).
		Bold(true)

	if !m.showDetails {
		return statusStyle.Render(statusText)
	}

	// Detailed view
	var details []string
	details = append(details, fmt.Sprintf("Status: %s", string(status)))

	if timeRemaining > 0 {
		details = append(details, fmt.Sprintf("Time: %s", formatSessionDuration(timeRemaining)))
	}

	detailStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Padding(0, 1)

	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		statusStyle.Render(statusText),
		detailStyle.Render(lipgloss.JoinVertical(lipgloss.Left, details...)),
	)
}

func (m *SessionStatusModel) renderTimeoutWarning() string {
	if !m.warningShown || m.timeRemaining <= 0 {
		return ""
	}

	warningStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Yellow)).
		Background(lipgloss.Color(utils.Colours.Surface0)).
		Bold(true).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(utils.Colours.Yellow))

	message := fmt.Sprintf("⚠ Session expires in %s", formatSessionDuration(m.timeRemaining))
	return warningStyle.Render(message)
}

func (m *SessionStatusModel) checkSessionStatus() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		if m.sessionManager == nil {
			return nil
		}

		timeRemaining := m.sessionManager.GetTimeRemaining(m.walletID)

		// Send warning when 2 minutes, 1 minute, or 30 seconds remain
		if timeRemaining > 0 && timeRemaining <= 2*time.Minute && !m.warningShown {
			return SessionTimeoutWarningMsg{
				WalletID:      m.walletID,
				TimeRemaining: timeRemaining,
			}
		}

		// Send expired message when session expires
		if timeRemaining <= 0 && m.timeRemaining > 0 {
			return SessionExpiredMsg{WalletID: m.walletID}
		}

		return t
	})
}

func formatSessionDuration(d time.Duration) string {
	if d <= 0 {
		return "expired"
	}

	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	} else {
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
}

type SessionManagerOverlay struct {
	sessionManager   *security.SessionManager
	activeSessions   []string
	selectedSession  int
	showConfirmation bool
	confirmAction    string
	confirmTarget    string
}

func NewSessionManagerOverlay(sessionManager *security.SessionManager) *SessionManagerOverlay {
	return &SessionManagerOverlay{
		sessionManager: sessionManager,
		activeSessions: sessionManager.GetActiveSessions(),
	}
}

func (m SessionManagerOverlay) Init() tea.Cmd {
	return nil
}

func (m SessionManagerOverlay) Update(msg tea.Msg) (SessionManagerOverlay, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.showConfirmation {
			switch msg.String() {
			case "y", "Y":
				return m.executeConfirmAction()
			case "n", "N", "esc":
				m.showConfirmation = false
				m.confirmAction = ""
				m.confirmTarget = ""
			}
			return m, nil
		}

		switch msg.String() {
		case "up", "k":
			if m.selectedSession > 0 {
				m.selectedSession--
			}
		case "down", "j":
			if m.selectedSession < len(m.activeSessions)-1 {
				m.selectedSession++
			}
		case "d", "D":
			if len(m.activeSessions) > 0 {
				m.confirmAction = "close"
				m.confirmTarget = m.activeSessions[m.selectedSession]
				m.showConfirmation = true
			}
		case "r", "R":
			m.activeSessions = m.sessionManager.GetActiveSessions()
		case "esc", "q":
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m SessionManagerOverlay) View() string {
	if m.showConfirmation {
		return m.renderConfirmation()
	}

	return m.renderSessionList()
}

func (m *SessionManagerOverlay) renderSessionList() string {
	if len(m.activeSessions) == 0 {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color(utils.Colours.Text)).
			Padding(2).
			Render("No active sessions")
	}

	var items []string
	for i, walletID := range m.activeSessions {
		status := m.sessionManager.GetSessionStatus(walletID)
		timeRemaining := m.sessionManager.GetTimeRemaining(walletID)

		itemStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(utils.Colours.Text)).
			Padding(0, 1)

		if i == m.selectedSession {
			itemStyle = itemStyle.
				Background(lipgloss.Color(utils.Colours.Surface1)).
				Bold(true)
		}

		statusIcon := "●"
		switch status {
		case security.SessionStatusActive:
			statusIcon = lipgloss.NewStyle().
				Foreground(lipgloss.Color(utils.Colours.Green)).
				Render("●")
		case security.SessionStatusExpiring:
			statusIcon = lipgloss.NewStyle().
				Foreground(lipgloss.Color(utils.Colours.Yellow)).
				Render("●")
		case security.SessionStatusExpired:
			statusIcon = lipgloss.NewStyle().
				Foreground(lipgloss.Color(utils.Colours.Red)).
				Render("●")
		}

		item := fmt.Sprintf("%s %s (%s)",
			statusIcon,
			walletID[:8]+"...",
			formatSessionDuration(timeRemaining),
		)

		items = append(items, itemStyle.Render(item))
	}

	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Blue)).
		Bold(true).
		Padding(1, 0).
		Render("Active Sessions")

	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Surface2)).
		Padding(1, 0).
		Render("↑/↓: navigate • d: close session • r: refresh • esc: exit")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		lipgloss.JoinVertical(lipgloss.Left, items...),
		help,
	)
}

func (m *SessionManagerOverlay) renderConfirmation() string {
	message := fmt.Sprintf("Close session for wallet %s?", m.confirmTarget[:8]+"...")

	confirmStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Background(lipgloss.Color(utils.Colours.Surface0)).
		Padding(1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(utils.Colours.Yellow))

	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Surface2)).
		Render("y: yes • n: no")

	return confirmStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Center,
			message,
			"",
			help,
		),
	)
}

func (m SessionManagerOverlay) executeConfirmAction() (SessionManagerOverlay, tea.Cmd) {
	if m.confirmAction == "close" && m.confirmTarget != "" {
		err := m.sessionManager.CloseSession(m.confirmTarget)
		if err == nil {
			m.activeSessions = m.sessionManager.GetActiveSessions()
			if m.selectedSession >= len(m.activeSessions) && len(m.activeSessions) > 0 {
				m.selectedSession = len(m.activeSessions) - 1
			}
		}
	}

	m.showConfirmation = false
	m.confirmAction = ""
	m.confirmTarget = ""

	return m, nil
}
