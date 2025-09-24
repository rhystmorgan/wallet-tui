package views

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"rhystmorgan/veWallet/internal/models"
	"rhystmorgan/veWallet/internal/storage"
	"rhystmorgan/veWallet/internal/utils"
)

type ContactSelectorModel struct {
	// Data
	contacts         *models.ContactList
	recentAddresses  []models.RecentAddress
	filteredContacts []models.Contact
	storage          *storage.Storage

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
	onContactSelected func(contact *models.Contact) tea.Cmd
	onAddressSelected func(address string, contactName string) tea.Cmd
	onCreateContact   func() tea.Cmd
	onCancel          func() tea.Cmd
}

type ContactSelectedMsg struct {
	Contact *models.Contact
}

type AddressSelectedMsg struct {
	Address     string
	ContactName string
}

type CreateContactMsg struct{}

func NewContactSelectorModel() *ContactSelectorModel {
	return &ContactSelectorModel{
		maxDisplayItems: 8,
		showFavorites:   true,
		showRecent:      true,
		visible:         false,
	}
}

func (m *ContactSelectorModel) SetStorage(storage *storage.Storage) {
	m.storage = storage
}

func (m *ContactSelectorModel) SetContacts(contacts *models.ContactList) {
	m.contacts = contacts
	m.updateFilteredContacts()
}

func (m *ContactSelectorModel) SetRecentAddresses(addresses []models.RecentAddress) {
	m.recentAddresses = addresses
	// Sort by frequency and recency
	sort.Slice(m.recentAddresses, func(i, j int) bool {
		if m.recentAddresses[i].Frequency != m.recentAddresses[j].Frequency {
			return m.recentAddresses[i].Frequency > m.recentAddresses[j].Frequency
		}
		return m.recentAddresses[i].LastUsed.After(m.recentAddresses[j].LastUsed)
	})
}

func (m *ContactSelectorModel) SetCallbacks(
	onContactSelected func(contact *models.Contact) tea.Cmd,
	onAddressSelected func(address string, contactName string) tea.Cmd,
	onCreateContact func() tea.Cmd,
	onCancel func() tea.Cmd,
) {
	m.onContactSelected = onContactSelected
	m.onAddressSelected = onAddressSelected
	m.onCreateContact = onCreateContact
	m.onCancel = onCancel
}

func (m *ContactSelectorModel) Show() {
	m.selectedIndex = 0
	m.scrollOffset = 0
	m.searchQuery = ""
	m.updateFilteredContacts()
	m.visible = true
}

func (m *ContactSelectorModel) Hide() {
	m.visible = false
}

func (m *ContactSelectorModel) IsVisible() bool {
	return m.visible
}

func (m *ContactSelectorModel) Update(msg tea.Msg) (*ContactSelectorModel, tea.Cmd) {
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
			return m, nil

		case "down", "j":
			m.navigateDown()
			return m, nil

		case "/":
			// Focus search - for now just clear search
			m.searchQuery = ""
			m.updateFilteredContacts()
			return m, nil

		case "ctrl+n":
			m.Hide()
			var cmd tea.Cmd
			if m.onCreateContact != nil {
				cmd = m.onCreateContact()
			}
			return m, cmd

		case " ":
			// Toggle favorite for selected contact
			m.toggleSelectedContactFavorite()
			return m, nil

		case "backspace":
			if len(m.searchQuery) > 0 {
				m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
				m.updateFilteredContacts()
				m.selectedIndex = 0
				m.scrollOffset = 0
			}
			return m, nil

		default:
			// Handle search input
			if len(msg.String()) == 1 && msg.String() >= " " {
				m.searchQuery += msg.String()
				m.updateFilteredContacts()
				m.selectedIndex = 0
				m.scrollOffset = 0
			}
			return m, nil
		}
	}

	return m, nil
}

func (m *ContactSelectorModel) renderContent() string {
	if m.loading {
		return m.renderLoading()
	}

	if m.error != nil {
		return m.renderError()
	}

	var content strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Blue)).
		Bold(true).
		Align(lipgloss.Center).
		Width(m.width - 6)

	content.WriteString(titleStyle.Render("Select Contact"))
	content.WriteString("\n\n")

	// Search bar
	content.WriteString(m.renderSearchBar())
	content.WriteString("\n\n")

	// Recent addresses section
	if m.showRecent && len(m.recentAddresses) > 0 {
		content.WriteString(m.renderRecentAddresses())
		content.WriteString("\n")
	}

	// Favorites section
	if m.showFavorites {
		favorites := m.getFavoriteContacts()
		if len(favorites) > 0 {
			content.WriteString(m.renderFavoriteContacts(favorites))
			content.WriteString("\n")
		}
	}

	// All contacts section
	content.WriteString(m.renderAllContacts())
	content.WriteString("\n")

	// Action buttons
	content.WriteString(m.renderActionButtons())
	content.WriteString("\n\n")

	// Help text
	content.WriteString(m.renderHelpText())

	return content.String()
}

func (m *ContactSelectorModel) renderSearchBar() string {
	searchStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Background(lipgloss.Color(utils.Colours.Surface0)).
		Padding(0, 1).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(utils.Colours.Surface1)).
		Width(m.width - 10)

	searchText := m.searchQuery
	if searchText == "" {
		searchText = "Type to search..."
	}
	searchText += "█"

	return fmt.Sprintf("Search: %s", searchStyle.Render(searchText))
}

func (m *ContactSelectorModel) renderRecentAddresses() string {
	sectionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Yellow)).
		Bold(true)

	var content strings.Builder
	content.WriteString(sectionStyle.Render("Recent Addresses:"))
	content.WriteString("\n")

	maxRecent := 3
	for i, addr := range m.recentAddresses {
		if i >= maxRecent {
			break
		}

		isSelected := m.isRecentAddressSelected(i)
		content.WriteString(m.renderRecentAddressItem(addr, isSelected))
		content.WriteString("\n")
	}

	return content.String()
}

func (m *ContactSelectorModel) renderRecentAddressItem(addr models.RecentAddress, isSelected bool) string {
	itemStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Padding(0, 1)

	if isSelected {
		itemStyle = itemStyle.
			Background(lipgloss.Color(utils.Colours.Surface0)).
			Bold(true)
	}

	displayName := addr.ContactName
	if displayName == "" {
		displayName = utils.FormatAddress(addr.Address, 8, 6)
	}

	timeAgo := utils.FormatTimeAgo(addr.LastUsed)

	line := fmt.Sprintf("● %-20s %s  %s",
		displayName,
		utils.FormatAddress(addr.Address, 6, 4),
		timeAgo)

	return itemStyle.Render(line)
}

func (m *ContactSelectorModel) renderFavoriteContacts(favorites []models.Contact) string {
	sectionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Green)).
		Bold(true)

	var content strings.Builder
	content.WriteString(sectionStyle.Render("Favorites:"))
	content.WriteString("\n")

	for i, contact := range favorites {
		if i >= 3 { // Limit favorites display
			break
		}

		isSelected := m.isFavoriteContactSelected(i)
		content.WriteString(m.renderContactItem(contact, isSelected, true))
		content.WriteString("\n")
	}

	return content.String()
}

func (m *ContactSelectorModel) renderAllContacts() string {
	sectionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Bold(true)

	var content strings.Builder
	content.WriteString(sectionStyle.Render("All Contacts:"))
	content.WriteString("\n")

	if len(m.filteredContacts) == 0 {
		noContactsStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(utils.Colours.Subtext0)).
			Italic(true)
		content.WriteString(noContactsStyle.Render("No contacts found"))
		content.WriteString("\n")
		return content.String()
	}

	// Calculate visible range
	startIdx := m.scrollOffset
	endIdx := startIdx + m.maxDisplayItems
	if endIdx > len(m.filteredContacts) {
		endIdx = len(m.filteredContacts)
	}

	for i := startIdx; i < endIdx; i++ {
		contact := m.filteredContacts[i]
		isSelected := m.isAllContactSelected(i)
		isFavorite := m.isContactFavorite(contact)
		content.WriteString(m.renderContactItem(contact, isSelected, isFavorite))
		content.WriteString("\n")
	}

	// Scroll indicators
	if len(m.filteredContacts) > m.maxDisplayItems {
		scrollInfo := fmt.Sprintf("(%d-%d of %d)", startIdx+1, endIdx, len(m.filteredContacts))
		scrollStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(utils.Colours.Subtext0)).
			Italic(true)
		content.WriteString(scrollStyle.Render(scrollInfo))
		content.WriteString("\n")
	}

	return content.String()
}

func (m *ContactSelectorModel) renderContactItem(contact models.Contact, isSelected bool, isFavorite bool) string {
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

	line := fmt.Sprintf("%s %-20s %s",
		favoriteIcon,
		contact.Name,
		utils.FormatAddress(contact.Address, 6, 4))

	if contact.Notes != "" && len(contact.Notes) < 30 {
		line += fmt.Sprintf("  %s", contact.Notes)
	}

	return itemStyle.Render(line)
}

func (m *ContactSelectorModel) renderActionButtons() string {
	buttonStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Blue)).
		Background(lipgloss.Color(utils.Colours.Surface0)).
		Padding(0, 2).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(utils.Colours.Surface1))

	var buttons []string
	buttons = append(buttons, buttonStyle.Render("Add New Contact"))
	buttons = append(buttons, buttonStyle.Render("Import Contacts"))

	return lipgloss.JoinHorizontal(lipgloss.Left, buttons...)
}

func (m *ContactSelectorModel) renderHelpText() string {
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Subtext0)).
		Italic(true).
		Align(lipgloss.Center)

	return helpStyle.Render("↑/↓: navigate • Enter: select • Space: toggle favorite • /: search • Ctrl+N: new contact • Esc: close")
}

func (m *ContactSelectorModel) renderLoading() string {
	loadingStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Blue)).
		Bold(true).
		Align(lipgloss.Center)

	return loadingStyle.Render("Loading contacts...")
}

func (m *ContactSelectorModel) renderError() string {
	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Red)).
		Bold(true).
		Align(lipgloss.Center)

	return errorStyle.Render(fmt.Sprintf("Error: %s", m.error.Error()))
}

// Navigation and selection methods
func (m *ContactSelectorModel) navigateUp() {
	if m.selectedIndex > 0 {
		m.selectedIndex--
		m.updateScrollOffset()
	}
}

func (m *ContactSelectorModel) navigateDown() {
	maxIndex := m.getTotalSelectableItems() - 1
	if m.selectedIndex < maxIndex {
		m.selectedIndex++
		m.updateScrollOffset()
	}
}

func (m *ContactSelectorModel) updateScrollOffset() {
	// Adjust scroll offset to keep selected item visible
	if m.selectedIndex < m.scrollOffset {
		m.scrollOffset = m.selectedIndex
	} else if m.selectedIndex >= m.scrollOffset+m.maxDisplayItems {
		m.scrollOffset = m.selectedIndex - m.maxDisplayItems + 1
	}
}

func (m *ContactSelectorModel) handleSelection() tea.Cmd {
	// Determine what is selected based on current index
	currentIndex := 0

	// Check recent addresses
	if m.showRecent && len(m.recentAddresses) > 0 {
		maxRecent := 3
		if len(m.recentAddresses) < maxRecent {
			maxRecent = len(m.recentAddresses)
		}

		if m.selectedIndex < maxRecent {
			addr := m.recentAddresses[m.selectedIndex]
			m.Hide()
			if m.onAddressSelected != nil {
				return m.onAddressSelected(addr.Address, addr.ContactName)
			}
			return nil
		}
		currentIndex += maxRecent
	}

	// Check favorite contacts
	if m.showFavorites {
		favorites := m.getFavoriteContacts()
		maxFavorites := 3
		if len(favorites) < maxFavorites {
			maxFavorites = len(favorites)
		}

		if m.selectedIndex >= currentIndex && m.selectedIndex < currentIndex+maxFavorites {
			contact := favorites[m.selectedIndex-currentIndex]
			m.Hide()
			if m.onContactSelected != nil {
				return m.onContactSelected(&contact)
			}
			return nil
		}
		currentIndex += maxFavorites
	}

	// Check all contacts
	contactIndex := m.selectedIndex - currentIndex
	if contactIndex >= 0 && contactIndex < len(m.filteredContacts) {
		contact := m.filteredContacts[contactIndex]
		m.Hide()
		if m.onContactSelected != nil {
			return m.onContactSelected(&contact)
		}
	}

	return nil
}

// Helper methods
func (m *ContactSelectorModel) updateFilteredContacts() {
	if m.contacts == nil {
		m.filteredContacts = []models.Contact{}
		return
	}

	m.filteredContacts = []models.Contact{}
	query := strings.ToLower(m.searchQuery)

	for _, contact := range m.contacts.Contacts {
		if query == "" ||
			strings.Contains(strings.ToLower(contact.Name), query) ||
			strings.Contains(strings.ToLower(contact.Address), query) ||
			strings.Contains(strings.ToLower(contact.Notes), query) {
			m.filteredContacts = append(m.filteredContacts, contact)
		}
	}

	// Sort contacts by name
	sort.Slice(m.filteredContacts, func(i, j int) bool {
		return strings.ToLower(m.filteredContacts[i].Name) < strings.ToLower(m.filteredContacts[j].Name)
	})
}

func (m *ContactSelectorModel) getFavoriteContacts() []models.Contact {
	if m.contacts == nil {
		return []models.Contact{}
	}
	return m.contacts.GetFavorites()
}

func (m *ContactSelectorModel) isContactFavorite(contact models.Contact) bool {
	return contact.IsFavorite
}

func (m *ContactSelectorModel) getTotalSelectableItems() int {
	total := 0

	if m.showRecent && len(m.recentAddresses) > 0 {
		maxRecent := 3
		if len(m.recentAddresses) < maxRecent {
			maxRecent = len(m.recentAddresses)
		}
		total += maxRecent
	}

	if m.showFavorites {
		favorites := m.getFavoriteContacts()
		maxFavorites := 3
		if len(favorites) < maxFavorites {
			maxFavorites = len(favorites)
		}
		total += maxFavorites
	}

	total += len(m.filteredContacts)
	return total
}

func (m *ContactSelectorModel) isRecentAddressSelected(index int) bool {
	return m.selectedIndex == index
}

func (m *ContactSelectorModel) isFavoriteContactSelected(index int) bool {
	recentCount := 0
	if m.showRecent && len(m.recentAddresses) > 0 {
		maxRecent := 3
		if len(m.recentAddresses) < maxRecent {
			maxRecent = len(m.recentAddresses)
		}
		recentCount = maxRecent
	}
	return m.selectedIndex == recentCount+index
}

func (m *ContactSelectorModel) isAllContactSelected(index int) bool {
	offset := 0

	if m.showRecent && len(m.recentAddresses) > 0 {
		maxRecent := 3
		if len(m.recentAddresses) < maxRecent {
			maxRecent = len(m.recentAddresses)
		}
		offset += maxRecent
	}

	if m.showFavorites {
		favorites := m.getFavoriteContacts()
		maxFavorites := 3
		if len(favorites) < maxFavorites {
			maxFavorites = len(favorites)
		}
		offset += maxFavorites
	}

	return m.selectedIndex == offset+index
}

func (m *ContactSelectorModel) toggleSelectedContactFavorite() {
	// Determine which contact is selected and toggle its favorite status
	currentIndex := 0

	// Skip recent addresses
	if m.showRecent && len(m.recentAddresses) > 0 {
		maxRecent := 3
		if len(m.recentAddresses) < maxRecent {
			maxRecent = len(m.recentAddresses)
		}
		if m.selectedIndex < maxRecent {
			// Can't favorite recent addresses
			return
		}
		currentIndex += maxRecent
	}

	// Check favorite contacts
	if m.showFavorites {
		favorites := m.getFavoriteContacts()
		maxFavorites := 3
		if len(favorites) < maxFavorites {
			maxFavorites = len(favorites)
		}

		if m.selectedIndex >= currentIndex && m.selectedIndex < currentIndex+maxFavorites {
			contactIndex := m.selectedIndex - currentIndex
			if contactIndex < len(favorites) && m.contacts != nil {
				// Find the contact in the main list and toggle favorite
				for i := range m.contacts.Contacts {
					if m.contacts.Contacts[i].ID == favorites[contactIndex].ID {
						m.contacts.Contacts[i].ToggleFavorite()
						return
					}
				}
			}
			return
		}
		currentIndex += maxFavorites
	}

	// Check all contacts
	contactIndex := m.selectedIndex - currentIndex
	if contactIndex >= 0 && contactIndex < len(m.filteredContacts) && m.contacts != nil {
		// Find the contact in the main list and toggle favorite
		selectedContact := m.filteredContacts[contactIndex]
		for i := range m.contacts.Contacts {
			if m.contacts.Contacts[i].ID == selectedContact.ID {
				m.contacts.Contacts[i].ToggleFavorite()
				return
			}
		}
	}
}
