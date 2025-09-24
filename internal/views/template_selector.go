package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"rhystmorgan/veWallet/internal/models"
	"rhystmorgan/veWallet/internal/storage"
	"rhystmorgan/veWallet/internal/utils"
)

type TemplateSelectorModel struct {
	// Data
	templateManager   *models.TransactionTemplateManager
	filteredTemplates []models.TransactionTemplate
	storage           *storage.Storage

	// Search and filtering
	searchQuery   string
	showFavorites bool
	showRecent    bool

	// UI state
	selectedIndex   int
	maxDisplayItems int
	scrollOffset    int
	width           int
	height          int
	visible         bool

	// Loading and error states
	loading bool
	error   error

	// Callbacks
	onTemplateSelected func(template *models.TransactionTemplate) tea.Cmd
	onCreateTemplate   func() tea.Cmd
	onCancel           func() tea.Cmd
}

type TemplateSelectedMsg struct {
	Template *models.TransactionTemplate
}

type CreateTemplateMsg struct{}

func NewTemplateSelectorModel() *TemplateSelectorModel {
	return &TemplateSelectorModel{
		maxDisplayItems: 8,
		showFavorites:   true,
		showRecent:      true,
		visible:         false,
		templateManager: models.NewTransactionTemplateManager(),
	}
}

func (m *TemplateSelectorModel) SetStorage(storage *storage.Storage) {
	m.storage = storage
}

func (m *TemplateSelectorModel) SetTemplateManager(manager *models.TransactionTemplateManager) {
	m.templateManager = manager
	m.updateFilteredTemplates()
}

func (m *TemplateSelectorModel) SetCallbacks(
	onTemplateSelected func(template *models.TransactionTemplate) tea.Cmd,
	onCreateTemplate func() tea.Cmd,
	onCancel func() tea.Cmd,
) {
	m.onTemplateSelected = onTemplateSelected
	m.onCreateTemplate = onCreateTemplate
	m.onCancel = onCancel
}

func (m *TemplateSelectorModel) Show() {
	m.selectedIndex = 0
	m.scrollOffset = 0
	m.searchQuery = ""
	m.updateFilteredTemplates()
	m.visible = true
}

func (m *TemplateSelectorModel) Hide() {
	m.visible = false
}

func (m *TemplateSelectorModel) IsVisible() bool {
	return m.visible
}

func (m *TemplateSelectorModel) Update(msg tea.Msg) (*TemplateSelectorModel, tea.Cmd) {
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
			var cmd tea.Cmd
			if m.onCancel != nil {
				cmd = m.onCancel()
			}
			return m, cmd

		case "enter":
			cmd := m.handleSelection()
			return m, cmd

		case "up", "k":
			m.navigateUp()

		case "down", "j":
			m.navigateDown()

		case "/":
			// Focus search - for now just clear search
			m.searchQuery = ""
			m.updateFilteredTemplates()

		case "ctrl+n":
			m.Hide()
			var cmd tea.Cmd
			if m.onCreateTemplate != nil {
				cmd = m.onCreateTemplate()
			}
			return m, cmd

		case "backspace":
			if len(m.searchQuery) > 0 {
				m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
				m.updateFilteredTemplates()
				m.selectedIndex = 0
				m.scrollOffset = 0
			}

		default:
			// Handle search input
			if len(msg.String()) == 1 && msg.String() >= " " {
				m.searchQuery += msg.String()
				m.updateFilteredTemplates()
				m.selectedIndex = 0
				m.scrollOffset = 0
			}
		}
	}

	return m, nil
}

func (m *TemplateSelectorModel) View() string {
	if !m.visible {
		return ""
	}

	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(utils.Colours.Mauve)).
		Background(lipgloss.Color(utils.Colours.Base)).
		Padding(1).
		Width(70).
		Height(20)

	return containerStyle.Render(m.renderContent())
}

func (m *TemplateSelectorModel) renderContent() string {
	if m.loading {
		return m.renderLoading()
	}

	if m.error != nil {
		return m.renderError()
	}

	var content strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Mauve)).
		Bold(true).
		Align(lipgloss.Center).
		Width(m.width - 6)

	content.WriteString(titleStyle.Render("Select Transaction Template"))
	content.WriteString("\n\n")

	// Search bar
	content.WriteString(m.renderSearchBar())
	content.WriteString("\n\n")

	// Favorites section
	if m.showFavorites {
		favorites := m.getFavoriteTemplates()
		if len(favorites) > 0 {
			content.WriteString(m.renderFavoriteTemplates(favorites))
			content.WriteString("\n")
		}
	}

	// Recent section
	if m.showRecent {
		recent := m.getRecentTemplates()
		if len(recent) > 0 {
			content.WriteString(m.renderRecentTemplates(recent))
			content.WriteString("\n")
		}
	}

	// All templates section
	content.WriteString(m.renderAllTemplates())
	content.WriteString("\n")

	// Action buttons
	content.WriteString(m.renderActionButtons())
	content.WriteString("\n\n")

	// Help text
	content.WriteString(m.renderHelpText())

	return content.String()
}

func (m *TemplateSelectorModel) renderSearchBar() string {
	searchStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Background(lipgloss.Color(utils.Colours.Surface0)).
		Padding(0, 1).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(utils.Colours.Surface1)).
		Width(m.width - 10)

	searchText := m.searchQuery
	if searchText == "" {
		searchText = "Type to search templates..."
	}
	searchText += "█"

	return fmt.Sprintf("Search: %s", searchStyle.Render(searchText))
}

func (m *TemplateSelectorModel) renderFavoriteTemplates(favorites []models.TransactionTemplate) string {
	sectionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Yellow)).
		Bold(true)

	var content strings.Builder
	content.WriteString(sectionStyle.Render("Favorites:"))
	content.WriteString("\n")

	for i, template := range favorites {
		if i >= 3 { // Limit favorites display
			break
		}

		isSelected := m.isFavoriteTemplateSelected(i)
		content.WriteString(m.renderTemplateItem(template, isSelected, true))
		content.WriteString("\n")
	}

	return content.String()
}

func (m *TemplateSelectorModel) renderRecentTemplates(recent []models.TransactionTemplate) string {
	sectionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Blue)).
		Bold(true)

	var content strings.Builder
	content.WriteString(sectionStyle.Render("Recent:"))
	content.WriteString("\n")

	for i, template := range recent {
		if i >= 3 { // Limit recent display
			break
		}

		isSelected := m.isRecentTemplateSelected(i)
		content.WriteString(m.renderTemplateItem(template, isSelected, false))
		content.WriteString("\n")
	}

	return content.String()
}

func (m *TemplateSelectorModel) renderAllTemplates() string {
	sectionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Bold(true)

	var content strings.Builder
	content.WriteString(sectionStyle.Render("All Templates:"))
	content.WriteString("\n")

	if len(m.filteredTemplates) == 0 {
		noTemplatesStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(utils.Colours.Subtext0)).
			Italic(true)
		content.WriteString(noTemplatesStyle.Render("No templates found"))
		content.WriteString("\n")
		return content.String()
	}

	// Calculate visible range
	startIdx := m.scrollOffset
	endIdx := startIdx + m.maxDisplayItems
	if endIdx > len(m.filteredTemplates) {
		endIdx = len(m.filteredTemplates)
	}

	for i := startIdx; i < endIdx; i++ {
		template := m.filteredTemplates[i]
		isSelected := m.isAllTemplateSelected(i)
		content.WriteString(m.renderTemplateItem(template, isSelected, template.IsFavorite))
		content.WriteString("\n")
	}

	// Scroll indicators
	if len(m.filteredTemplates) > m.maxDisplayItems {
		scrollInfo := fmt.Sprintf("(%d-%d of %d)", startIdx+1, endIdx, len(m.filteredTemplates))
		scrollStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(utils.Colours.Subtext0)).
			Italic(true)
		content.WriteString(scrollStyle.Render(scrollInfo))
		content.WriteString("\n")
	}

	return content.String()
}

func (m *TemplateSelectorModel) renderTemplateItem(template models.TransactionTemplate, isSelected bool, isFavorite bool) string {
	itemStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Padding(0, 1)

	if isSelected {
		itemStyle = itemStyle.
			Background(lipgloss.Color(utils.Colours.Surface0)).
			Bold(true)
	}

	favoriteIcon := " "
	if isFavorite {
		favoriteIcon = "★"
	}

	// Format amount if present
	amountStr := ""
	if template.Amount != nil {
		amountStr = fmt.Sprintf(" %s %s", utils.FormatAmount(template.Amount, 4), template.Asset)
	} else {
		amountStr = fmt.Sprintf(" %s", template.Asset)
	}

	line := fmt.Sprintf("%s %-20s%s → %s",
		favoriteIcon,
		template.Name,
		amountStr,
		template.ContactName)

	if template.ContactName == "" {
		line = fmt.Sprintf("%s %-20s%s → %s",
			favoriteIcon,
			template.Name,
			amountStr,
			utils.FormatAddress(template.ToAddress, 6, 4))
	}

	// Add use count
	if template.UseCount > 0 {
		line += fmt.Sprintf("  Used: %d times", template.UseCount)
	}

	return itemStyle.Render(line)
}

func (m *TemplateSelectorModel) renderActionButtons() string {
	buttonStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Mauve)).
		Background(lipgloss.Color(utils.Colours.Surface0)).
		Padding(0, 2).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(utils.Colours.Surface1))

	var buttons []string
	buttons = append(buttons, buttonStyle.Render("Create New Template"))
	buttons = append(buttons, buttonStyle.Render("Import Templates"))

	return lipgloss.JoinHorizontal(lipgloss.Left, buttons...)
}

func (m *TemplateSelectorModel) renderHelpText() string {
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Subtext0)).
		Italic(true).
		Align(lipgloss.Center)

	return helpStyle.Render("↑/↓: navigate • Enter: use template • /: search • Ctrl+N: new template • Esc: close")
}

func (m *TemplateSelectorModel) renderLoading() string {
	loadingStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Mauve)).
		Bold(true).
		Align(lipgloss.Center)

	return loadingStyle.Render("Loading templates...")
}

func (m *TemplateSelectorModel) renderError() string {
	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Red)).
		Bold(true).
		Align(lipgloss.Center)

	return errorStyle.Render(fmt.Sprintf("Error: %s", m.error.Error()))
}

// Navigation and selection methods
func (m *TemplateSelectorModel) navigateUp() {
	if m.selectedIndex > 0 {
		m.selectedIndex--
		m.updateScrollOffset()
	}
}

func (m *TemplateSelectorModel) navigateDown() {
	maxIndex := m.getTotalSelectableItems() - 1
	if m.selectedIndex < maxIndex {
		m.selectedIndex++
		m.updateScrollOffset()
	}
}

func (m *TemplateSelectorModel) updateScrollOffset() {
	// Adjust scroll offset to keep selected item visible
	if m.selectedIndex < m.scrollOffset {
		m.scrollOffset = m.selectedIndex
	} else if m.selectedIndex >= m.scrollOffset+m.maxDisplayItems {
		m.scrollOffset = m.selectedIndex - m.maxDisplayItems + 1
	}
}

func (m *TemplateSelectorModel) handleSelection() tea.Cmd {
	// Determine what is selected based on current index
	currentIndex := 0

	// Check favorite templates
	if m.showFavorites {
		favorites := m.getFavoriteTemplates()
		maxFavorites := 3
		if len(favorites) < maxFavorites {
			maxFavorites = len(favorites)
		}

		if m.selectedIndex < maxFavorites {
			template := favorites[m.selectedIndex]
			m.Hide()
			if m.onTemplateSelected != nil {
				return m.onTemplateSelected(&template)
			}
			return nil
		}
		currentIndex += maxFavorites
	}

	// Check recent templates
	if m.showRecent {
		recent := m.getRecentTemplates()
		maxRecent := 3
		if len(recent) < maxRecent {
			maxRecent = len(recent)
		}

		if m.selectedIndex >= currentIndex && m.selectedIndex < currentIndex+maxRecent {
			template := recent[m.selectedIndex-currentIndex]
			m.Hide()
			if m.onTemplateSelected != nil {
				return m.onTemplateSelected(&template)
			}
			return nil
		}
		currentIndex += maxRecent
	}

	// Check all templates
	templateIndex := m.selectedIndex - currentIndex
	if templateIndex >= 0 && templateIndex < len(m.filteredTemplates) {
		template := m.filteredTemplates[templateIndex]
		m.Hide()
		if m.onTemplateSelected != nil {
			return m.onTemplateSelected(&template)
		}
	}

	return nil
}

// Helper methods
func (m *TemplateSelectorModel) updateFilteredTemplates() {
	if m.templateManager == nil {
		m.filteredTemplates = []models.TransactionTemplate{}
		return
	}

	if m.searchQuery == "" {
		m.filteredTemplates = m.templateManager.GetAllTemplates()
	} else {
		m.filteredTemplates = m.templateManager.SearchTemplates(m.searchQuery)
	}
}

func (m *TemplateSelectorModel) getFavoriteTemplates() []models.TransactionTemplate {
	if m.templateManager == nil {
		return []models.TransactionTemplate{}
	}
	return m.templateManager.GetFavoriteTemplates()
}

func (m *TemplateSelectorModel) getRecentTemplates() []models.TransactionTemplate {
	if m.templateManager == nil {
		return []models.TransactionTemplate{}
	}
	return m.templateManager.GetRecentTemplates(3)
}

func (m *TemplateSelectorModel) getTotalSelectableItems() int {
	total := 0

	if m.showFavorites {
		favorites := m.getFavoriteTemplates()
		maxFavorites := 3
		if len(favorites) < maxFavorites {
			maxFavorites = len(favorites)
		}
		total += maxFavorites
	}

	if m.showRecent {
		recent := m.getRecentTemplates()
		maxRecent := 3
		if len(recent) < maxRecent {
			maxRecent = len(recent)
		}
		total += maxRecent
	}

	total += len(m.filteredTemplates)
	return total
}

func (m *TemplateSelectorModel) isFavoriteTemplateSelected(index int) bool {
	return m.selectedIndex == index
}

func (m *TemplateSelectorModel) isRecentTemplateSelected(index int) bool {
	favoriteCount := 0
	if m.showFavorites {
		favorites := m.getFavoriteTemplates()
		maxFavorites := 3
		if len(favorites) < maxFavorites {
			maxFavorites = len(favorites)
		}
		favoriteCount = maxFavorites
	}
	return m.selectedIndex == favoriteCount+index
}

func (m *TemplateSelectorModel) isAllTemplateSelected(index int) bool {
	offset := 0

	if m.showFavorites {
		favorites := m.getFavoriteTemplates()
		maxFavorites := 3
		if len(favorites) < maxFavorites {
			maxFavorites = len(favorites)
		}
		offset += maxFavorites
	}

	if m.showRecent {
		recent := m.getRecentTemplates()
		maxRecent := 3
		if len(recent) < maxRecent {
			maxRecent = len(recent)
		}
		offset += maxRecent
	}

	return m.selectedIndex == offset+index
}
