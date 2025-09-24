package views

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"rhystmorgan/veWallet/internal/models"
	"rhystmorgan/veWallet/internal/storage"
	"rhystmorgan/veWallet/internal/utils"
)

type PasswordPromptModel struct {
	wallet  *models.Wallet
	storage *storage.Storage

	// Input state
	password    string
	masked      bool
	attempts    int
	maxAttempts int

	// UI state
	visible     bool
	loading     bool
	error       string
	title       string
	description string

	// Security
	sessionTimeout time.Duration
	lastActivity   time.Time

	// Callbacks
	onSuccess func(*models.Wallet) tea.Cmd
	onCancel  func() tea.Cmd
	onError   func(error) tea.Cmd
}

type PasswordPromptMsg struct {
	Action string // "show", "hide", "success", "error"
	Data   interface{}
}

type PasswordVerificationMsg struct {
	Success bool
	Wallet  *models.Wallet
	Error   error
}

func NewPasswordPromptModel() *PasswordPromptModel {
	return &PasswordPromptModel{
		masked:         true,
		maxAttempts:    3,
		sessionTimeout: 5 * time.Minute,
		visible:        false,
	}
}

func (m *PasswordPromptModel) SetWallet(wallet *models.Wallet) {
	m.wallet = wallet
}

func (m *PasswordPromptModel) SetStorage(storage *storage.Storage) {
	m.storage = storage
}

func (m *PasswordPromptModel) SetCallbacks(onSuccess func(*models.Wallet) tea.Cmd, onCancel func() tea.Cmd, onError func(error) tea.Cmd) {
	m.onSuccess = onSuccess
	m.onCancel = onCancel
	m.onError = onError
}

func (m *PasswordPromptModel) Show(title, description string) {
	m.visible = true
	m.title = title
	m.description = description
	m.password = ""
	m.error = ""
	m.loading = false
	m.lastActivity = time.Now()
}

func (m *PasswordPromptModel) Hide() {
	m.visible = false
	m.password = ""
	m.error = ""
	m.loading = false
}

func (m *PasswordPromptModel) IsVisible() bool {
	return m.visible
}

func (m *PasswordPromptModel) IsSessionValid() bool {
	return time.Since(m.lastActivity) < m.sessionTimeout
}

func (m PasswordPromptModel) Init() tea.Cmd {
	return nil
}

func (m PasswordPromptModel) Update(msg tea.Msg) (PasswordPromptModel, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.loading {
			return m, nil // Ignore input while loading
		}

		switch msg.String() {
		case "esc":
			m.Hide()
			if m.onCancel != nil {
				return m, m.onCancel()
			}
			return m, nil

		case "enter":
			if len(m.password) == 0 {
				m.error = "Password cannot be empty"
				return m, nil
			}

			if m.attempts >= m.maxAttempts {
				m.error = "Too many failed attempts"
				return m, nil
			}

			m.loading = true
			m.error = ""
			return m, m.verifyPassword()

		case "backspace":
			if len(m.password) > 0 {
				m.password = m.password[:len(m.password)-1]
			}

		case "ctrl+u":
			m.password = ""

		default:
			// Add character to password
			if len(msg.String()) == 1 && msg.String() != " " {
				m.password += msg.String()
			}
		}

		m.lastActivity = time.Now()

	case PasswordVerificationMsg:
		m.loading = false
		if msg.Success {
			m.Hide()
			if m.onSuccess != nil {
				return m, m.onSuccess(msg.Wallet)
			}
		} else {
			m.attempts++
			if m.attempts >= m.maxAttempts {
				m.error = "Too many failed attempts. Please restart the application."
				if m.onError != nil {
					return m, m.onError(fmt.Errorf("maximum password attempts exceeded"))
				}
			} else {
				m.error = fmt.Sprintf("Incorrect password (%d/%d attempts)", m.attempts, m.maxAttempts)
			}
			m.password = ""
		}

	case PasswordPromptMsg:
		switch msg.Action {
		case "show":
			if data, ok := msg.Data.(map[string]string); ok {
				m.Show(data["title"], data["description"])
			}
		case "hide":
			m.Hide()
		}
	}

	return m, nil
}

func (m *PasswordPromptModel) View() string {
	if !m.visible {
		return ""
	}

	// Create overlay style
	overlayStyle := lipgloss.NewStyle().
		Width(60).
		Height(15).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(utils.Colours.Blue)).
		Background(lipgloss.Color(utils.Colours.Base)).
		Padding(1).
		Align(lipgloss.Center)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Blue)).
		Bold(true).
		Align(lipgloss.Center)

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Align(lipgloss.Center).
		Margin(1, 0)

	inputStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Background(lipgloss.Color(utils.Colours.Surface0)).
		Padding(0, 1).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(utils.Colours.Surface1)).
		Width(40)

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Red)).
		Bold(true).
		Align(lipgloss.Center)

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Subtext0)).
		Italic(true).
		Align(lipgloss.Center)

	var content strings.Builder

	// Title
	content.WriteString(titleStyle.Render(m.title))
	content.WriteString("\n\n")

	// Description
	if m.description != "" {
		content.WriteString(descStyle.Render(m.description))
		content.WriteString("\n")
	}

	// Password input
	passwordDisplay := ""
	if m.masked {
		passwordDisplay = strings.Repeat("*", len(m.password))
	} else {
		passwordDisplay = m.password
	}

	if m.loading {
		content.WriteString(inputStyle.Render("Verifying password..."))
	} else {
		content.WriteString(inputStyle.Render("Password: " + passwordDisplay))
	}
	content.WriteString("\n\n")

	// Error message
	if m.error != "" {
		content.WriteString(errorStyle.Render(m.error))
		content.WriteString("\n\n")
	}

	// Help text
	if !m.loading {
		helpText := "Enter: confirm • Esc: cancel • Ctrl+U: clear"
		content.WriteString(helpStyle.Render(helpText))
	}

	return overlayStyle.Render(content.String())
}

func (m *PasswordPromptModel) verifyPassword() tea.Cmd {
	return func() tea.Msg {
		if m.wallet == nil || m.storage == nil {
			return PasswordVerificationMsg{
				Success: false,
				Error:   fmt.Errorf("wallet or storage not available"),
			}
		}

		// Load and decrypt the wallet
		wallet, err := m.storage.LoadWallet(m.wallet.ID, m.password)
		if err != nil {
			return PasswordVerificationMsg{
				Success: false,
				Error:   err,
			}
		}

		return PasswordVerificationMsg{
			Success: true,
			Wallet:  wallet,
		}
	}
}

// Helper function to show password prompt
func ShowPasswordPrompt(title, description string) tea.Cmd {
	return func() tea.Msg {
		return PasswordPromptMsg{
			Action: "show",
			Data: map[string]string{
				"title":       title,
				"description": description,
			},
		}
	}
}

// Helper function to hide password prompt
func HidePasswordPrompt() tea.Cmd {
	return func() tea.Msg {
		return PasswordPromptMsg{Action: "hide"}
	}
}
