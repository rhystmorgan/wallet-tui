package views

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"rhystmorgan/veWallet/internal/models"
	"rhystmorgan/veWallet/internal/storage"
	"rhystmorgan/veWallet/internal/utils"
)

type ContactCreateStep int

const (
	ContactCreateStepName ContactCreateStep = iota
	ContactCreateStepAddress
	ContactCreateStepNotes
	ContactCreateStepReview
)

type ContactCreateModel struct {
	// Data
	storage *storage.Storage

	// Form state
	step    ContactCreateStep
	name    string
	address string
	notes   string

	// Validation state
	nameValid    bool
	addressValid bool
	nameError    string
	addressError string

	// UI state
	visible         bool
	width           int
	height          int
	feedbackMessage *FeedbackMessage

	// Callbacks
	onContactCreated func(contact *models.Contact) tea.Cmd
	onCancel         func() tea.Cmd
}

type ContactCreatedMsg struct {
	Contact *models.Contact
}

func NewContactCreateModel() *ContactCreateModel {
	return &ContactCreateModel{
		step:    ContactCreateStepName,
		visible: false,
	}
}

func (m *ContactCreateModel) SetStorage(storage *storage.Storage) {
	m.storage = storage
}

func (m *ContactCreateModel) SetCallbacks(
	onContactCreated func(contact *models.Contact) tea.Cmd,
	onCancel func() tea.Cmd,
) {
	m.onContactCreated = onContactCreated
	m.onCancel = onCancel
}

func (m *ContactCreateModel) Show(prefilledAddress string) {
	m.step = ContactCreateStepName
	m.name = ""
	m.address = prefilledAddress
	m.notes = ""
	m.nameValid = false
	m.addressValid = false
	m.nameError = ""
	m.addressError = ""
	m.feedbackMessage = nil
	m.visible = true

	// If address is prefilled, validate it and move to name step
	if prefilledAddress != "" {
		m.validateAddress()
	}
}

func (m *ContactCreateModel) Hide() {
	m.visible = false
}

func (m *ContactCreateModel) IsVisible() bool {
	return m.visible
}

func (m *ContactCreateModel) Update(msg tea.Msg) (*ContactCreateModel, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.Hide()
			if m.onCancel != nil {
				return m, m.onCancel()
			}

		case "enter":
			return m.handleEnterKey()

		case "tab":
			m.goToNextStep()

		case "shift+tab":
			m.goToPreviousStep()

		case "backspace":
			m.handleBackspace()

		default:
			m.handleTextInput(msg.String())
		}

	case FeedbackTimeoutMsg:
		m.feedbackMessage = nil
	}

	// Auto-validate current step
	m.validateCurrentStep()

	return m, nil
}

func (m *ContactCreateModel) View() string {
	if !m.visible {
		return ""
	}

	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(utils.Colours.Green)).
		Background(lipgloss.Color(utils.Colours.Base)).
		Padding(1).
		Width(50).
		Height(15)

	var content strings.Builder

	// Title
	content.WriteString(m.renderHeader())
	content.WriteString("\n\n")

	// Current step content
	switch m.step {
	case ContactCreateStepName:
		content.WriteString(m.renderNameStep())
	case ContactCreateStepAddress:
		content.WriteString(m.renderAddressStep())
	case ContactCreateStepNotes:
		content.WriteString(m.renderNotesStep())
	case ContactCreateStepReview:
		content.WriteString(m.renderReviewStep())
	}

	content.WriteString("\n\n")

	// Help text
	content.WriteString(m.renderHelpText())

	// Feedback message
	if m.feedbackMessage != nil {
		content.WriteString("\n")
		content.WriteString(m.renderFeedbackMessage())
	}

	return containerStyle.Render(content.String())
}

func (m *ContactCreateModel) renderHeader() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Green)).
		Bold(true).
		Align(lipgloss.Center)

	stepStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Subtext0)).
		Align(lipgloss.Center)

	stepNames := []string{"Name", "Address", "Notes", "Review"}
	stepIndicator := utils.FormatStepIndicator(int(m.step), len(stepNames), stepNames)

	var content strings.Builder
	content.WriteString(titleStyle.Render("Create Contact"))
	content.WriteString("\n")
	content.WriteString(stepStyle.Render(stepIndicator))

	return content.String()
}

func (m *ContactCreateModel) renderNameStep() string {
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Bold(true)

	inputStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Background(lipgloss.Color(utils.Colours.Surface0)).
		Padding(0, 1).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(utils.Colours.Surface1)).
		Width(40)

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Red))

	var content strings.Builder
	content.WriteString(labelStyle.Render("Contact Name:"))
	content.WriteString("\n\n")

	// Name input
	content.WriteString(inputStyle.Render(m.name + "█"))
	content.WriteString("\n")

	// Validation error
	if m.nameError != "" {
		content.WriteString("\n")
		content.WriteString(errorStyle.Render("✗ " + m.nameError))
	} else if m.nameValid && m.name != "" {
		validStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(utils.Colours.Green))
		content.WriteString("\n")
		content.WriteString(validStyle.Render("✓ Valid name"))
	}

	return content.String()
}

func (m *ContactCreateModel) renderAddressStep() string {
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Bold(true)

	inputStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Background(lipgloss.Color(utils.Colours.Surface0)).
		Padding(0, 1).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(utils.Colours.Surface1)).
		Width(40)

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Red))

	var content strings.Builder
	content.WriteString(labelStyle.Render("VeChain Address:"))
	content.WriteString("\n\n")

	// Address input
	displayAddr := m.address
	if len(displayAddr) > 38 {
		displayAddr = displayAddr[:38]
	}
	content.WriteString(inputStyle.Render(displayAddr + "█"))
	content.WriteString("\n")

	// Validation error
	if m.addressError != "" {
		content.WriteString("\n")
		content.WriteString(errorStyle.Render("✗ " + m.addressError))
	} else if m.addressValid && m.address != "" {
		validStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(utils.Colours.Green))
		content.WriteString("\n")
		content.WriteString(validStyle.Render("✓ Valid VeChain address"))
	}

	return content.String()
}

func (m *ContactCreateModel) renderNotesStep() string {
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Bold(true)

	inputStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Background(lipgloss.Color(utils.Colours.Surface0)).
		Padding(0, 1).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(utils.Colours.Surface1)).
		Width(40)

	var content strings.Builder
	content.WriteString(labelStyle.Render("Notes (optional):"))
	content.WriteString("\n\n")

	// Notes input
	content.WriteString(inputStyle.Render(m.notes + "█"))

	return content.String()
}

func (m *ContactCreateModel) renderReviewStep() string {
	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(utils.Colours.Green)).
		Padding(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Bold(true)

	var content strings.Builder
	content.WriteString(labelStyle.Render("Review Contact"))
	content.WriteString("\n\n")

	// Contact details
	details := strings.Builder{}
	details.WriteString("Name:    " + m.name + "\n")
	details.WriteString("Address: " + utils.FormatAddress(m.address, 10, 8) + "\n")
	if m.notes != "" {
		details.WriteString("Notes:   " + m.notes + "\n")
	}

	content.WriteString(cardStyle.Render(details.String()))

	return content.String()
}

func (m *ContactCreateModel) renderHelpText() string {
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Subtext0)).
		Italic(true).
		Align(lipgloss.Center)

	var helpText string
	switch m.step {
	case ContactCreateStepName:
		helpText = "Enter contact name • Tab: next • Esc: cancel"
	case ContactCreateStepAddress:
		helpText = "Enter VeChain address • Tab: next • Shift+Tab: back • Esc: cancel"
	case ContactCreateStepNotes:
		helpText = "Enter notes (optional) • Tab: next • Shift+Tab: back • Esc: cancel"
	case ContactCreateStepReview:
		helpText = "Enter: create contact • Shift+Tab: back • Esc: cancel"
	default:
		helpText = "Please wait..."
	}

	return helpStyle.Render(helpText)
}

func (m *ContactCreateModel) renderFeedbackMessage() string {
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

// Navigation methods
func (m *ContactCreateModel) goToNextStep() {
	switch m.step {
	case ContactCreateStepName:
		if m.nameValid {
			m.step = ContactCreateStepAddress
		}
	case ContactCreateStepAddress:
		if m.addressValid {
			m.step = ContactCreateStepNotes
		}
	case ContactCreateStepNotes:
		m.step = ContactCreateStepReview
	}
}

func (m *ContactCreateModel) goToPreviousStep() {
	if m.step > ContactCreateStepName {
		m.step--
	}
}

// Input handling methods
func (m *ContactCreateModel) handleEnterKey() (*ContactCreateModel, tea.Cmd) {
	switch m.step {
	case ContactCreateStepReview:
		return m.createContact()
	default:
		m.goToNextStep()
		return m, nil
	}
}

func (m *ContactCreateModel) handleBackspace() {
	switch m.step {
	case ContactCreateStepName:
		if len(m.name) > 0 {
			m.name = m.name[:len(m.name)-1]
		}
	case ContactCreateStepAddress:
		if len(m.address) > 0 {
			m.address = m.address[:len(m.address)-1]
		}
	case ContactCreateStepNotes:
		if len(m.notes) > 0 {
			m.notes = m.notes[:len(m.notes)-1]
		}
	}
}

func (m *ContactCreateModel) handleTextInput(input string) {
	if len(input) != 1 {
		return
	}

	switch m.step {
	case ContactCreateStepName:
		if len(m.name) < 50 { // Limit name length
			m.name += input
		}
	case ContactCreateStepAddress:
		if len(m.address) < 42 { // VeChain address length
			m.address += input
		}
	case ContactCreateStepNotes:
		if len(m.notes) < 200 { // Limit notes length
			m.notes += input
		}
	}
}

// Validation methods
func (m *ContactCreateModel) validateCurrentStep() {
	switch m.step {
	case ContactCreateStepName:
		m.validateName()
	case ContactCreateStepAddress:
		m.validateAddress()
	}
}

func (m *ContactCreateModel) validateName() {
	name := strings.TrimSpace(m.name)
	if name == "" {
		m.nameValid = false
		m.nameError = ""
		return
	}

	if len(name) < 2 {
		m.nameValid = false
		m.nameError = "Name must be at least 2 characters"
		return
	}

	m.nameValid = true
	m.nameError = ""
}

func (m *ContactCreateModel) validateAddress() {
	err := utils.ValidateVeChainAddress(m.address)
	if err != nil {
		m.addressValid = false
		m.addressError = err.Error()
	} else {
		m.addressValid = true
		m.addressError = ""
	}
}

// Contact creation
func (m *ContactCreateModel) createContact() (*ContactCreateModel, tea.Cmd) {
	// Final validation
	m.validateName()
	m.validateAddress()

	if !m.nameValid || !m.addressValid {
		m.showFeedback(FeedbackError, "Please fix validation errors", 3*time.Second)
		return m, nil
	}

	// Create the contact
	contact := models.NewContact(m.name, m.address, m.notes)

	// TODO: Save to storage
	if m.storage != nil {
		// This would save the contact to storage
		// For now, we'll just create the contact object
	}

	m.Hide()
	m.showFeedback(FeedbackSuccess, "Contact created successfully!", 3*time.Second)

	if m.onContactCreated != nil {
		return m, m.onContactCreated(contact)
	}

	return m, nil
}

// Feedback methods
func (m *ContactCreateModel) showFeedback(feedbackType FeedbackType, message string, duration time.Duration) {
	m.feedbackMessage = &FeedbackMessage{
		Type:     feedbackType,
		Message:  message,
		Duration: duration,
		ShowTime: time.Now(),
	}
}
