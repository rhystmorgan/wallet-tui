package views

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"rhystmorgan/veWallet/internal/models"
	"rhystmorgan/veWallet/internal/security"
	"rhystmorgan/veWallet/internal/storage"
	"rhystmorgan/veWallet/internal/utils"
)

type ContactView int

const (
	ContactViewList ContactView = iota
	ContactViewDetails
	ContactViewCreate
	ContactViewEdit
	ContactViewImportExport
	ContactViewDeleteConfirm
)

type ContactSortBy int

const (
	SortByName ContactSortBy = iota
	SortByAddress
	SortByDateAdded
	SortByLastUsed
	SortByUsageCount
)

type SortOrder int

const (
	SortAscending SortOrder = iota
	SortDescending
)

type ContactsModel struct {
	// Core dependencies
	sessionManager *security.SessionManager
	storage        *storage.Storage
	currentWallet  *models.Wallet

	// Contact data
	contacts         []models.Contact
	filteredContacts []models.Contact
	selectedContact  int
	totalContacts    int

	// UI state
	currentView       ContactView
	showDetails       bool
	showCreateForm    bool
	showEditForm      bool
	showDeleteConfirm bool
	showImportExport  bool

	// Search and filtering
	searchInput     textinput.Model
	searchQuery     string
	filterFavorites bool
	filterTags      []string
	sortBy          ContactSortBy
	sortOrder       SortOrder

	// Form state
	createForm ContactForm
	editForm   ContactForm
	formErrors map[string]string

	// Loading and error states
	loading        bool
	loadingSpinner *utils.Spinner
	error          error
	successMessage string

	// Performance optimization
	renderCache string
	cacheValid  bool
	lastUpdate  time.Time

	// Pagination
	currentPage int
	pageSize    int
	totalPages  int

	// Window dimensions
	width  int
	height int
}

type ContactForm struct {
	// Basic information
	name        textinput.Model
	address     textinput.Model
	description textinput.Model

	// Advanced fields
	tags       textinput.Model
	category   string
	isFavorite bool

	// Validation state
	errors  map[string]string
	isValid bool

	// Form state
	currentField int
	isSubmitting bool

	// UI state
	showAdvanced bool
}

type ContactFormField int

const (
	FormFieldName ContactFormField = iota
	FormFieldAddress
	FormFieldDescription
	FormFieldTags
	FormFieldCategory
	FormFieldSubmit
)

// Messages
type ContactUpdatedMsg struct {
	Contact *models.Contact
}

type ContactDeletedMsg struct {
	ContactID string
}

type ContactsLoadedMsg struct {
	Contacts []models.Contact
}

func NewContactsModel(sessionManager *security.SessionManager, storage *storage.Storage, wallet *models.Wallet) *ContactsModel {
	searchInput := textinput.New()
	searchInput.Placeholder = "Search contacts..."
	searchInput.CharLimit = 50

	return &ContactsModel{
		sessionManager:   sessionManager,
		storage:          storage,
		currentWallet:    wallet,
		contacts:         []models.Contact{},
		filteredContacts: []models.Contact{},
		selectedContact:  0,
		currentView:      ContactViewList,
		searchInput:      searchInput,
		sortBy:           SortByName,
		sortOrder:        SortAscending,
		formErrors:       make(map[string]string),
		pageSize:         20,
		currentPage:      1,
		createForm:       newContactForm(),
		editForm:         newContactForm(),
	}
}

func newContactForm() ContactForm {
	nameInput := textinput.New()
	nameInput.Placeholder = "Enter contact name"
	nameInput.CharLimit = 50
	nameInput.Focus()
	nameInput.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(utils.Colours.Blue))
	nameInput.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(utils.Colours.Text))

	addressInput := textinput.New()
	addressInput.Placeholder = "0x..."
	addressInput.CharLimit = 42
	addressInput.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(utils.Colours.Blue))
	addressInput.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(utils.Colours.Text))

	descriptionInput := textinput.New()
	descriptionInput.Placeholder = "Optional description"
	descriptionInput.CharLimit = 200
	descriptionInput.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(utils.Colours.Blue))
	descriptionInput.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(utils.Colours.Text))

	tagsInput := textinput.New()
	tagsInput.Placeholder = "friend, business, exchange (comma-separated)"
	tagsInput.CharLimit = 100
	tagsInput.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(utils.Colours.Blue))
	tagsInput.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(utils.Colours.Text))

	return ContactForm{
		name:         nameInput,
		address:      addressInput,
		description:  descriptionInput,
		tags:         tagsInput,
		category:     "Personal",
		errors:       make(map[string]string),
		currentField: int(FormFieldName),
	}
}

func (m *ContactsModel) Init() tea.Cmd {
	return m.loadContacts()
}

func (m *ContactsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.invalidateCache()

	case tea.KeyMsg:
		// Handle global shortcuts first
		switch msg.String() {
		case "ctrl+c", "q":
			if m.currentView == ContactViewList {
				return m, tea.Quit
			}
			// Return to list view from other views
			m.currentView = ContactViewList
			m.clearMessages()
			return m, nil

		case "ctrl+n":
			if m.currentView == ContactViewList {
				m.currentView = ContactViewCreate
				m.createForm = newContactForm()
				return m, m.createForm.name.Focus()
			}

		case "ctrl+i":
			if m.currentView == ContactViewList {
				m.currentView = ContactViewImportExport
				return m, nil
			}

		case "ctrl+s":
			if m.currentView == ContactViewList {
				m.searchInput.Focus()
				return m, nil
			}

		case "f2":
			if m.currentView == ContactViewList && len(m.filteredContacts) > 0 {
				contact := m.filteredContacts[m.selectedContact]
				m.editForm = m.populateEditForm(contact)
				m.currentView = ContactViewEdit
				return m, m.editForm.name.Focus()
			}

		case "ctrl+f":
			if m.currentView == ContactViewList {
				m.filterFavorites = !m.filterFavorites
				m.applyFiltersAndSort()
				m.invalidateCache()
			}

		case "ctrl+e":
			if m.currentView == ContactViewList && len(m.filteredContacts) > 0 {
				contact := m.filteredContacts[m.selectedContact]
				m.editForm = m.populateEditForm(contact)
				m.currentView = ContactViewEdit
				return m, m.editForm.name.Focus()
			}

		case "ctrl+d", "delete":
			if m.currentView == ContactViewList && len(m.filteredContacts) > 0 {
				m.currentView = ContactViewDeleteConfirm
				m.invalidateCache()
			}
		}

		// Handle view-specific key events
		switch m.currentView {
		case ContactViewList:
			return m.updateListView(msg)
		case ContactViewCreate:
			return m.updateCreateView(msg)
		case ContactViewEdit:
			return m.updateEditView(msg)
		case ContactViewDeleteConfirm:
			return m.updateDeleteConfirmView(msg)
		case ContactViewImportExport:
			return m.updateImportExportView(msg)
		}

	case ContactsLoadedMsg:
		m.contacts = msg.Contacts
		m.applyFiltersAndSort()
		m.loading = false
		m.invalidateCache()

	case ContactCreatedMsg:
		m.contacts = append(m.contacts, *msg.Contact)
		m.applyFiltersAndSort()
		m.currentView = ContactViewList
		m.successMessage = fmt.Sprintf("Contact '%s' has been created successfully. You can now use it in transactions.", msg.Contact.Name)
		m.invalidateCache()

	case ContactUpdatedMsg:
		for i, contact := range m.contacts {
			if contact.ID == msg.Contact.ID {
				m.contacts[i] = *msg.Contact
				break
			}
		}
		m.applyFiltersAndSort()
		m.currentView = ContactViewList
		m.successMessage = fmt.Sprintf("Contact '%s' has been updated successfully. All changes have been saved.", msg.Contact.Name)
		m.invalidateCache()

	case ContactDeletedMsg:
		for i, contact := range m.contacts {
			if contact.ID == msg.ContactID {
				m.contacts = append(m.contacts[:i], m.contacts[i+1:]...)
				break
			}
		}
		m.applyFiltersAndSort()
		m.currentView = ContactViewList
		m.successMessage = "Contact has been deleted successfully. This action cannot be undone."
		if m.selectedContact >= len(m.filteredContacts) && m.selectedContact > 0 {
			m.selectedContact = len(m.filteredContacts) - 1
		}
		m.invalidateCache()

	case ErrorMsg:
		m.error = msg.Err
		m.loading = false
	}

	// Update search input
	if m.currentView == ContactViewList && m.searchInput.Focused() {
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

		// Update search query and filter
		newQuery := m.searchInput.Value()
		if newQuery != m.searchQuery {
			m.searchQuery = newQuery
			m.applyFiltersAndSort()
			m.invalidateCache()
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *ContactsModel) updateListView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.selectedContact > 0 {
			m.selectedContact--
			m.invalidateCache()
		}

	case "down", "j":
		if m.selectedContact < len(m.filteredContacts)-1 {
			m.selectedContact++
			m.invalidateCache()
		}

	case "enter":
		if len(m.filteredContacts) > 0 {
			m.currentView = ContactViewDetails
			m.invalidateCache()
		}

	case "e":
		if len(m.filteredContacts) > 0 {
			contact := m.filteredContacts[m.selectedContact]
			m.editForm = m.populateEditForm(contact)
			m.currentView = ContactViewEdit
			return m, m.editForm.name.Focus()
		}

	case "d", "delete":
		if len(m.filteredContacts) > 0 {
			m.currentView = ContactViewDeleteConfirm
			m.invalidateCache()
		}

	case "f":
		if len(m.filteredContacts) > 0 {
			contact := &m.filteredContacts[m.selectedContact]
			if err := contact.ToggleFavorite(nil, "", ""); err != nil {
				return m, func() tea.Msg {
					return ErrorMsg{Err: fmt.Errorf("failed to toggle favorite: %w", err)}
				}
			}
			return m, m.saveContact(contact)
		}

	case "s":
		if len(m.filteredContacts) > 0 {
			contact := m.filteredContacts[m.selectedContact]
			// Navigate to send transaction with this contact
			return m, func() tea.Msg {
				return NavigateMsg{
					State: ViewSendTransaction,
					Data:  &contact,
				}
			}
		}

	case "ctrl+f":
		m.filterFavorites = !m.filterFavorites
		m.applyFiltersAndSort()
		m.invalidateCache()

	case "tab":
		m.cycleSortBy()
		m.applyFiltersAndSort()
		m.invalidateCache()

	case "shift+tab":
		m.toggleSortOrder()
		m.applyFiltersAndSort()
		m.invalidateCache()

	case "pgup":
		if m.currentPage > 1 {
			m.currentPage--
			m.selectedContact = 0
			m.invalidateCache()
		}

	case "pgdn":
		if m.currentPage < m.totalPages {
			m.currentPage++
			m.selectedContact = 0
			m.invalidateCache()
		}

	case "esc":
		if m.searchInput.Focused() {
			m.searchInput.Blur()
		} else if m.searchQuery != "" {
			m.searchQuery = ""
			m.searchInput.SetValue("")
			m.applyFiltersAndSort()
			m.invalidateCache()
		}
	}

	return m, nil
}

func (m *ContactsModel) updateCreateView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.currentView = ContactViewList
		m.clearMessages()
		return m, nil

	case "tab":
		m.createForm.nextField()
		return m, m.createForm.focusCurrentField()

	case "shift+tab":
		m.createForm.prevField()
		return m, m.createForm.focusCurrentField()

	case "enter":
		if m.createForm.currentField == int(FormFieldSubmit) {
			return m.submitCreateForm()
		}
		m.createForm.nextField()
		return m, m.createForm.focusCurrentField()

	case "ctrl+a":
		m.createForm.showAdvanced = !m.createForm.showAdvanced
		m.invalidateCache()
	}

	// Update the current field
	var cmd tea.Cmd
	switch ContactFormField(m.createForm.currentField) {
	case FormFieldName:
		m.createForm.name, cmd = m.createForm.name.Update(msg)
	case FormFieldAddress:
		m.createForm.address, cmd = m.createForm.address.Update(msg)
	case FormFieldDescription:
		m.createForm.description, cmd = m.createForm.description.Update(msg)
	case FormFieldTags:
		m.createForm.tags, cmd = m.createForm.tags.Update(msg)
	}

	// Validate form in real-time
	m.validateCreateForm()

	return m, cmd
}

func (m *ContactsModel) updateEditView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.currentView = ContactViewList
		m.clearMessages()
		return m, nil

	case "tab":
		m.editForm.nextField()
		return m, m.editForm.focusCurrentField()

	case "shift+tab":
		m.editForm.prevField()
		return m, m.editForm.focusCurrentField()

	case "enter":
		if m.editForm.currentField == int(FormFieldSubmit) {
			return m.submitEditForm()
		}
		m.editForm.nextField()
		return m, m.editForm.focusCurrentField()

	case "ctrl+a":
		m.editForm.showAdvanced = !m.editForm.showAdvanced
		m.invalidateCache()
	}

	// Update the current field
	var cmd tea.Cmd
	switch ContactFormField(m.editForm.currentField) {
	case FormFieldName:
		m.editForm.name, cmd = m.editForm.name.Update(msg)
	case FormFieldAddress:
		m.editForm.address, cmd = m.editForm.address.Update(msg)
	case FormFieldDescription:
		m.editForm.description, cmd = m.editForm.description.Update(msg)
	case FormFieldTags:
		m.editForm.tags, cmd = m.editForm.tags.Update(msg)
	}

	// Validate form in real-time
	m.validateEditForm()

	return m, cmd
}

func (m *ContactsModel) updateDeleteConfirmView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		if len(m.filteredContacts) > 0 {
			contact := m.filteredContacts[m.selectedContact]
			return m, m.deleteContact(contact.ID)
		}

	case "n", "N", "esc":
		m.currentView = ContactViewList
		m.clearMessages()
	}

	return m, nil
}

func (m *ContactsModel) updateImportExportView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.currentView = ContactViewList
		m.clearMessages()
	}

	return m, nil
}

func (m *ContactsModel) View() string {
	if !m.cacheValid {
		m.renderCache = m.renderView()
		m.cacheValid = true
	}
	return m.renderCache
}

func (m *ContactsModel) renderView() string {
	switch m.currentView {
	case ContactViewList:
		return m.renderListView()
	case ContactViewDetails:
		return m.renderDetailsView()
	case ContactViewCreate:
		return m.renderCreateView()
	case ContactViewEdit:
		return m.renderEditView()
	case ContactViewDeleteConfirm:
		return m.renderDeleteConfirmView()
	case ContactViewImportExport:
		return m.renderImportExportView()
	default:
		return m.renderListView()
	}
}

func (m *ContactsModel) renderListView() string {
	var content strings.Builder

	// Header
	title := "Contacts Management"
	if m.totalContacts > 0 {
		title += fmt.Sprintf(" (%d contacts)", m.totalContacts)
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Background(lipgloss.Color(utils.Colours.Surface0)).
		Padding(0, 1).
		Width(m.width)

	content.WriteString(headerStyle.Render(title))
	content.WriteString("\n")

	// Search and controls bar
	searchBar := m.renderSearchBar()
	content.WriteString(searchBar)
	content.WriteString("\n")

	// Contact list
	if m.loading {
		loadingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(utils.Colours.Yellow)).
			Padding(2, 0).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(utils.Colours.Yellow))
		if m.loadingSpinner == nil {
			m.loadingSpinner = utils.NewSpinner()
		}
		content.WriteString(loadingStyle.Render(m.loadingSpinner.View() + " Loading contacts..."))
	} else if len(m.filteredContacts) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(utils.Colours.Overlay1)).
			Padding(2, 0)
		if m.searchQuery != "" {
			content.WriteString(emptyStyle.Render("No contacts found matching your search."))
		} else {
			content.WriteString(emptyStyle.Render("No contacts yet. Press Ctrl+N to create your first contact."))
		}
	} else {
		contactList := m.renderContactList()
		content.WriteString(contactList)
	}

	// Footer with controls and pagination
	footer := m.renderFooter()
	content.WriteString("\n")
	content.WriteString(footer)

	// Success/error messages
	if m.successMessage != "" {
		successStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(utils.Colours.Green)).
			Padding(1, 0).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(utils.Colours.Green))
		content.WriteString("\n")
		content.WriteString(successStyle.Render("✓ " + m.successMessage))
	}

	if m.error != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(utils.Colours.Red)).
			Padding(1, 0).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(utils.Colours.Red))
		content.WriteString("\n")
		content.WriteString(errorStyle.Render("✗ " + m.formatErrorMessage(m.error)))
	}

	return content.String()
}

func (m *ContactsModel) renderSearchBar() string {
	searchStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(utils.Colours.Surface1)).
		Padding(0, 1).
		Width(40)

	sortInfo := fmt.Sprintf("Sort: %s %s", m.getSortByName(), m.getSortOrderSymbol())
	sortStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Overlay1)).
		Padding(0, 1)

	filterInfo := ""
	if m.filterFavorites {
		filterInfo = "☆ Favorites"
	}
	filterStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Yellow)).
		Padding(0, 1)

	newButton := lipgloss.NewStyle().
		Background(lipgloss.Color(utils.Colours.Green)).
		Foreground(lipgloss.Color(utils.Colours.Base)).
		Padding(0, 1).
		Render("[+New]")

	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		"Search: ",
		searchStyle.Render(m.searchInput.View()),
		" ",
		sortStyle.Render(sortInfo),
		" ",
		filterStyle.Render(filterInfo),
		" ",
		newButton,
	)
}

func (m *ContactsModel) renderContactList() string {
	var items []string

	// Calculate pagination
	startIdx := (m.currentPage - 1) * m.pageSize
	endIdx := startIdx + m.pageSize
	if endIdx > len(m.filteredContacts) {
		endIdx = len(m.filteredContacts)
	}

	for i := startIdx; i < endIdx; i++ {
		contact := m.filteredContacts[i]
		isSelected := i == m.selectedContact

		item := m.renderContactItem(contact, isSelected)
		items = append(items, item)
	}

	return strings.Join(items, "\n")
}

func (m *ContactsModel) renderContactItem(contact models.Contact, isSelected bool) string {
	var style lipgloss.Style

	if isSelected {
		style = lipgloss.NewStyle().
			Background(lipgloss.Color(utils.Colours.Surface1)).
			Foreground(lipgloss.Color(utils.Colours.Text)).
			Padding(0, 1).
			Width(m.width - 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(utils.Colours.Blue))
	} else {
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color(utils.Colours.Text)).
			Padding(0, 1).
			Width(m.width - 2)
	}

	// Format contact info
	favoriteIcon := "  "
	if contact.IsFavorite {
		favoriteIcon = lipgloss.NewStyle().
			Foreground(lipgloss.Color(utils.Colours.Yellow)).
			Render("★ ")
	}

	nameStyle := lipgloss.NewStyle().
		Bold(true).
		Width(20)
	name := contact.Name
	if len(name) > 20 {
		name = name[:17] + "..."
	}

	addressStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Blue)).
		Width(12)
	address := contact.Address
	if len(address) > 12 {
		address = address[:6] + "..." + address[len(address)-4:]
	}

	categoryStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Green)).
		Width(10)
	category := contact.Category
	if category == "" {
		category = "Personal"
	}

	usageStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Overlay1))
	usageInfo := ""
	if contact.UseCount > 0 {
		usageInfo = fmt.Sprintf("📊 %d transactions", contact.UseCount)
	}

	line := fmt.Sprintf("%s%s%s%s%s",
		favoriteIcon,
		nameStyle.Render(name),
		addressStyle.Render(address),
		categoryStyle.Render(category),
		usageStyle.Render(usageInfo),
	)

	return style.Render(line)
}

func (m *ContactsModel) renderFooter() string {
	controls := "[Ctrl+E/F2]Edit [Ctrl+D/Del]Delete [S]end [Ctrl+F]Favorite [Ctrl+I]Import/Export [Ctrl+N]New [Ctrl+S]Search [Q]uit"
	controlsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Overlay1)).
		Padding(0, 1)

	pagination := ""
	if m.totalPages > 1 {
		pagination = fmt.Sprintf("Page %d of %d | ", m.currentPage, m.totalPages)
	}

	stats := fmt.Sprintf("%s%d contacts total", pagination, m.totalContacts)
	if m.filterFavorites {
		favoriteCount := 0
		for _, contact := range m.contacts {
			if contact.IsFavorite {
				favoriteCount++
			}
		}
		stats += fmt.Sprintf(" | %d favorites", favoriteCount)
	}

	statsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Overlay1)).
		Padding(0, 1)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		controlsStyle.Render(controls),
		statsStyle.Render(stats),
	)
}

// Helper methods
func (m *ContactsModel) loadContacts() tea.Cmd {
	return func() tea.Msg {
		// Check if we have a session manager (simplified check)
		if m.sessionManager == nil {
			return ErrorMsg{Err: fmt.Errorf("session expired")}
		}

		contactList, err := m.storage.LoadContacts()
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to load contacts: %w", err)}
		}

		return ContactsLoadedMsg{Contacts: contactList.Contacts}
	}
}

func (m *ContactsModel) applyFiltersAndSort() {
	// Start with all contacts
	filtered := make([]models.Contact, 0, len(m.contacts))

	for _, contact := range m.contacts {
		// Apply search filter
		if m.searchQuery != "" {
			query := strings.ToLower(m.searchQuery)
			if !strings.Contains(strings.ToLower(contact.Name), query) &&
				!strings.Contains(strings.ToLower(contact.Address), query) &&
				!strings.Contains(strings.ToLower(contact.Notes), query) {
				continue
			}
		}

		// Apply favorites filter
		if m.filterFavorites && !contact.IsFavorite {
			continue
		}

		filtered = append(filtered, contact)
	}

	// Sort contacts
	sort.Slice(filtered, func(i, j int) bool {
		var less bool
		switch m.sortBy {
		case SortByName:
			less = strings.ToLower(filtered[i].Name) < strings.ToLower(filtered[j].Name)
		case SortByAddress:
			less = filtered[i].Address < filtered[j].Address
		case SortByDateAdded:
			less = filtered[i].CreatedAt.Before(filtered[j].CreatedAt)
		case SortByLastUsed:
			less = filtered[i].LastUsed.Before(filtered[j].LastUsed)
		case SortByUsageCount:
			less = filtered[i].UseCount < filtered[j].UseCount
		default:
			less = strings.ToLower(filtered[i].Name) < strings.ToLower(filtered[j].Name)
		}

		if m.sortOrder == SortDescending {
			less = !less
		}
		return less
	})

	m.filteredContacts = filtered
	m.totalContacts = len(m.contacts)

	// Update pagination
	m.totalPages = (len(filtered) + m.pageSize - 1) / m.pageSize
	if m.totalPages == 0 {
		m.totalPages = 1
	}
	if m.currentPage > m.totalPages {
		m.currentPage = m.totalPages
	}

	// Adjust selected contact
	if m.selectedContact >= len(filtered) {
		m.selectedContact = 0
	}
}

func (m *ContactsModel) cycleSortBy() {
	switch m.sortBy {
	case SortByName:
		m.sortBy = SortByUsageCount
	case SortByUsageCount:
		m.sortBy = SortByLastUsed
	case SortByLastUsed:
		m.sortBy = SortByDateAdded
	case SortByDateAdded:
		m.sortBy = SortByAddress
	case SortByAddress:
		m.sortBy = SortByName
	}
}

func (m *ContactsModel) toggleSortOrder() {
	if m.sortOrder == SortAscending {
		m.sortOrder = SortDescending
	} else {
		m.sortOrder = SortAscending
	}
}

func (m *ContactsModel) getSortByName() string {
	switch m.sortBy {
	case SortByName:
		return "Name"
	case SortByAddress:
		return "Address"
	case SortByDateAdded:
		return "Date Added"
	case SortByLastUsed:
		return "Last Used"
	case SortByUsageCount:
		return "Usage"
	default:
		return "Name"
	}
}

func (m *ContactsModel) getSortOrderSymbol() string {
	if m.sortOrder == SortAscending {
		return "▲"
	}
	return "▼"
}

func (m *ContactsModel) invalidateCache() {
	m.cacheValid = false
}

func (m *ContactsModel) clearMessages() {
	m.error = nil
	m.successMessage = ""
}

func (m *ContactsModel) formatErrorMessage(err error) string {
	// Extract the root cause from wrapped errors
	for {
		if unwrapped := errors.Unwrap(err); unwrapped != nil {
			err = unwrapped
		} else {
			break
		}
	}

	// Format common error types
	switch {
	case strings.Contains(err.Error(), "session expired"):
		return "Your session has expired. Please log in again."
	case strings.Contains(err.Error(), "failed to load contacts"):
		return "Could not load your contacts. Please try again."
	case strings.Contains(err.Error(), "failed to save contact"):
		return "Could not save the contact. Please try again."
	case strings.Contains(err.Error(), "failed to delete contact"):
		return "Could not delete the contact. Please try again."
	case strings.Contains(err.Error(), "failed to toggle favorite"):
		return "Could not update favorite status. Please try again."
	case strings.Contains(err.Error(), "failed to add tag"):
		return "Could not add the tag. Please try again."
	case strings.Contains(err.Error(), "failed to update contact"):
		return "Could not update the contact. Please try again."
	case strings.Contains(err.Error(), "invalid address"):
		return "The address format is invalid. Please check and try again."
	case strings.Contains(err.Error(), "duplicate address"):
		return "This address already exists in your contacts."
	case strings.Contains(err.Error(), "name required"):
		return "Please enter a name for the contact."
	case strings.Contains(err.Error(), "address required"):
		return "Please enter an address for the contact."
	default:
		return err.Error()
	}
}

func (m *ContactsModel) renderDetailsView() string {
	if len(m.filteredContacts) == 0 {
		return "No contact selected"
	}

	contact := m.filteredContacts[m.selectedContact]

	var content strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Background(lipgloss.Color(utils.Colours.Surface0)).
		Padding(0, 1).
		Width(m.width)

	content.WriteString(headerStyle.Render(fmt.Sprintf("Contact Details: %s", contact.Name)))
	content.WriteString("\n\n")

	// Contact information
	infoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Padding(0, 2)

	info := fmt.Sprintf(`Name: %s
Address: %s
Category: %s
Notes: %s
Created: %s
Last Used: %s
Usage Count: %d
Favorite: %v`,
		contact.Name,
		contact.Address,
		contact.Category,
		contact.Notes,
		contact.CreatedAt.Format("2006-01-02 15:04"),
		contact.LastUsed.Format("2006-01-02 15:04"),
		contact.UseCount,
		contact.IsFavorite,
	)

	content.WriteString(infoStyle.Render(info))
	content.WriteString("\n\n")

	// Controls
	controlsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Overlay1)).
		Padding(0, 1)

	content.WriteString(controlsStyle.Render("[E]dit [D]elete [S]end Transaction [Esc] Back"))

	return content.String()
}

func (m *ContactsModel) renderCreateView() string {
	var content strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Background(lipgloss.Color(utils.Colours.Surface0)).
		Padding(0, 1).
		Width(m.width)

	content.WriteString(headerStyle.Render("Create New Contact"))
	content.WriteString("\n\n")

	// Form fields
	fieldStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Padding(0, 2)

	// Name field
	nameLabel := "Name: *Required"
	if m.createForm.currentField == int(FormFieldName) {
		nameLabel = lipgloss.NewStyle().
			Foreground(lipgloss.Color(utils.Colours.Blue)).
			Bold(true).
			Render("▶ " + nameLabel)
	} else {
		nameLabel = lipgloss.NewStyle().
			Foreground(lipgloss.Color(utils.Colours.Text)).
			Render("  " + nameLabel)
	}
	content.WriteString(fieldStyle.Render(nameLabel))
	content.WriteString("\n")
	content.WriteString(fieldStyle.Render(m.createForm.name.View()))
	if err, exists := m.createForm.errors["name"]; exists {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(utils.Colours.Red)).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(utils.Colours.Red)).
			Padding(0, 1)
		content.WriteString("\n")
		content.WriteString(errorStyle.Render("✗ " + err))
	}
	content.WriteString("\n\n")

	// Address field
	addressLabel := "Address: *Required"
	if m.createForm.currentField == int(FormFieldAddress) {
		addressLabel = lipgloss.NewStyle().
			Foreground(lipgloss.Color(utils.Colours.Blue)).
			Bold(true).
			Render("▶ " + addressLabel)
	} else {
		addressLabel = lipgloss.NewStyle().
			Foreground(lipgloss.Color(utils.Colours.Text)).
			Render("  " + addressLabel)
	}
	content.WriteString(fieldStyle.Render(addressLabel))
	content.WriteString("\n")
	content.WriteString(fieldStyle.Render(m.createForm.address.View()))
	if err, exists := m.createForm.errors["address"]; exists {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(utils.Colours.Red)).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(utils.Colours.Red)).
			Padding(0, 1)
		content.WriteString("\n")
		content.WriteString(errorStyle.Render("✗ " + err))
	}
	content.WriteString("\n\n")

	// Description field
	descLabel := "Description:"
	if m.createForm.currentField == int(FormFieldDescription) {
		descLabel = lipgloss.NewStyle().
			Foreground(lipgloss.Color(utils.Colours.Blue)).
			Bold(true).
			Render("▶ " + descLabel)
	} else {
		descLabel = lipgloss.NewStyle().
			Foreground(lipgloss.Color(utils.Colours.Text)).
			Render("  " + descLabel)
	}
	content.WriteString(fieldStyle.Render(descLabel))
	content.WriteString("\n")
	content.WriteString(fieldStyle.Render(m.createForm.description.View()))
	content.WriteString("\n\n")

	// Advanced fields (if shown)
	if m.createForm.showAdvanced {
		// Tags field
		tagsLabel := "Tags:"
		if m.createForm.currentField == int(FormFieldTags) {
			tagsLabel = "> " + tagsLabel
		}
		content.WriteString(fieldStyle.Render(tagsLabel))
		content.WriteString("\n")
		content.WriteString(fieldStyle.Render(m.createForm.tags.View()))
		content.WriteString("\n\n")

		// Category field
		categoryLabel := fmt.Sprintf("Category: %s", m.createForm.category)
		if m.createForm.currentField == int(FormFieldCategory) {
			categoryLabel = "> " + categoryLabel
		}
		content.WriteString(fieldStyle.Render(categoryLabel))
		content.WriteString("\n\n")

		// Favorite checkbox
		favoriteLabel := fmt.Sprintf("☐ Add to favorites")
		if m.createForm.isFavorite {
			favoriteLabel = "☑ Add to favorites"
		}
		content.WriteString(fieldStyle.Render(favoriteLabel))
		content.WriteString("\n\n")
	}

	// Submit button
	submitLabel := "Create Contact"
	if m.createForm.currentField == int(FormFieldSubmit) {
		submitLabel = "▶ " + submitLabel
	}
	submitStyle := lipgloss.NewStyle().
		Background(lipgloss.Color(utils.Colours.Green)).
		Foreground(lipgloss.Color(utils.Colours.Base)).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(utils.Colours.Green))
	if !m.createForm.isValid {
		submitStyle = submitStyle.
			Background(lipgloss.Color(utils.Colours.Overlay1)).
			BorderForeground(lipgloss.Color(utils.Colours.Overlay1))
	}
	content.WriteString(submitStyle.Render(submitLabel))
	content.WriteString("\n\n")

	// Controls
	controlsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Overlay1)).
		Padding(0, 1)

	controls := "[Tab] Next Field [Shift+Tab] Previous [Ctrl+A] Advanced [Enter] Submit [Esc] Cancel"
	content.WriteString(controlsStyle.Render(controls))

	return content.String()
}

func (m *ContactsModel) renderEditView() string {
	var content strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Background(lipgloss.Color(utils.Colours.Surface0)).
		Padding(0, 1).
		Width(m.width)

	content.WriteString(headerStyle.Render("Edit Contact"))
	content.WriteString("\n\n")

	// Form fields (similar to create view but with "Update" button)
	fieldStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Padding(0, 2)

	// Name field
	nameLabel := "Name: *Required"
	if m.editForm.currentField == int(FormFieldName) {
		nameLabel = "> " + nameLabel
	}
	content.WriteString(fieldStyle.Render(nameLabel))
	content.WriteString("\n")
	content.WriteString(fieldStyle.Render(m.editForm.name.View()))
	if err, exists := m.editForm.errors["name"]; exists {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(utils.Colours.Red))
		content.WriteString("\n")
		content.WriteString(errorStyle.Render("  " + err))
	}
	content.WriteString("\n\n")

	// Address field
	addressLabel := "Address: *Required"
	if m.editForm.currentField == int(FormFieldAddress) {
		addressLabel = "> " + addressLabel
	}
	content.WriteString(fieldStyle.Render(addressLabel))
	content.WriteString("\n")
	content.WriteString(fieldStyle.Render(m.editForm.address.View()))
	if err, exists := m.editForm.errors["address"]; exists {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(utils.Colours.Red))
		content.WriteString("\n")
		content.WriteString(errorStyle.Render("  " + err))
	}
	content.WriteString("\n\n")

	// Description field
	descLabel := "Description:"
	if m.editForm.currentField == int(FormFieldDescription) {
		descLabel = "> " + descLabel
	}
	content.WriteString(fieldStyle.Render(descLabel))
	content.WriteString("\n")
	content.WriteString(fieldStyle.Render(m.editForm.description.View()))
	content.WriteString("\n\n")

	// Advanced fields (if shown)
	if m.editForm.showAdvanced {
		// Tags field
		tagsLabel := "Tags:"
		if m.editForm.currentField == int(FormFieldTags) {
			tagsLabel = "> " + tagsLabel
		}
		content.WriteString(fieldStyle.Render(tagsLabel))
		content.WriteString("\n")
		content.WriteString(fieldStyle.Render(m.editForm.tags.View()))
		content.WriteString("\n\n")

		// Category field
		categoryLabel := fmt.Sprintf("Category: %s", m.editForm.category)
		if m.editForm.currentField == int(FormFieldCategory) {
			categoryLabel = "> " + categoryLabel
		}
		content.WriteString(fieldStyle.Render(categoryLabel))
		content.WriteString("\n\n")

		// Favorite checkbox
		favoriteLabel := fmt.Sprintf("☐ Favorite")
		if m.editForm.isFavorite {
			favoriteLabel = "☑ Favorite"
		}
		content.WriteString(fieldStyle.Render(favoriteLabel))
		content.WriteString("\n\n")
	}

	// Submit button
	submitLabel := "Update Contact"
	if m.editForm.currentField == int(FormFieldSubmit) {
		submitLabel = "> " + submitLabel
	}
	submitStyle := lipgloss.NewStyle().
		Background(lipgloss.Color(utils.Colours.Blue)).
		Foreground(lipgloss.Color(utils.Colours.Base)).
		Padding(0, 1)
	content.WriteString(submitStyle.Render(submitLabel))
	content.WriteString("\n\n")

	// Controls
	controlsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Overlay1)).
		Padding(0, 1)

	controls := "[Tab] Next Field [Shift+Tab] Previous [Ctrl+A] Advanced [Enter] Submit [Esc] Cancel"
	content.WriteString(controlsStyle.Render(controls))

	return content.String()
}

func (m *ContactsModel) renderDeleteConfirmView() string {
	if len(m.filteredContacts) == 0 {
		return "No contact selected"
	}

	contact := m.filteredContacts[m.selectedContact]

	var content strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(utils.Colours.Red)).
		Background(lipgloss.Color(utils.Colours.Surface0)).
		Padding(0, 1).
		Width(m.width)

	content.WriteString(headerStyle.Render("Delete Contact"))
	content.WriteString("\n\n")

	// Warning message
	warningStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Red)).
		Padding(0, 2)

	warning := fmt.Sprintf("Are you sure you want to delete the contact '%s'?", contact.Name)
	content.WriteString(warningStyle.Render(warning))
	content.WriteString("\n\n")

	// Contact info
	infoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Padding(0, 2)

	info := fmt.Sprintf("Address: %s\nUsage Count: %d transactions", contact.Address, contact.UseCount)
	content.WriteString(infoStyle.Render(info))
	content.WriteString("\n\n")

	// Usage warning
	if contact.UseCount > 0 {
		usageWarningStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(utils.Colours.Yellow)).
			Padding(0, 2)

		usageWarning := fmt.Sprintf("⚠ This contact has been used in %d transactions.", contact.UseCount)
		content.WriteString(usageWarningStyle.Render(usageWarning))
		content.WriteString("\n\n")
	}

	// Confirmation
	confirmStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Padding(0, 2)

	content.WriteString(confirmStyle.Render("This action cannot be undone."))
	content.WriteString("\n\n")

	// Controls
	controlsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Overlay1)).
		Padding(0, 1)

	content.WriteString(controlsStyle.Render("[Y] Yes, Delete [N] Cancel [Esc] Cancel"))

	return content.String()
}

func (m *ContactsModel) renderImportExportView() string {
	return "Import/Export View - Coming Soon in Phase 3.3.3"
}

// Form operations
func (m *ContactsModel) populateEditForm(contact models.Contact) ContactForm {
	form := newContactForm()
	form.name.SetValue(contact.Name)
	form.address.SetValue(contact.Address)
	form.description.SetValue(contact.Notes)
	form.isFavorite = contact.IsFavorite
	form.category = contact.Category
	if form.category == "" {
		form.category = "Personal"
	}

	// Set tags as comma-separated string
	if len(contact.Tags) > 0 {
		form.tags.SetValue(strings.Join(contact.Tags, ", "))
	}

	return form
}

func (m *ContactsModel) validateCreateForm() {
	m.createForm.errors = make(map[string]string)
	m.createForm.isValid = true

	// Validate name
	name := strings.TrimSpace(m.createForm.name.Value())
	if name == "" {
		m.createForm.errors["name"] = "Name is required"
		m.createForm.isValid = false
	} else if len(name) > 50 {
		m.createForm.errors["name"] = "Name must be 50 characters or less"
		m.createForm.isValid = false
	}

	// Validate address
	address := strings.TrimSpace(m.createForm.address.Value())
	if address == "" {
		m.createForm.errors["address"] = "Address is required"
		m.createForm.isValid = false
	} else if !m.isValidVeChainAddress(address) {
		m.createForm.errors["address"] = "Invalid VeChain address format"
		m.createForm.isValid = false
	} else if m.isDuplicateAddress(address, "") {
		m.createForm.errors["address"] = "Address already exists in contacts"
		m.createForm.isValid = false
	}
}

func (m *ContactsModel) validateEditForm() {
	m.editForm.errors = make(map[string]string)
	m.editForm.isValid = true

	// Get the current contact being edited
	if len(m.filteredContacts) == 0 {
		m.editForm.isValid = false
		return
	}
	currentContact := m.filteredContacts[m.selectedContact]

	// Validate name
	name := strings.TrimSpace(m.editForm.name.Value())
	if name == "" {
		m.editForm.errors["name"] = "Name is required"
		m.editForm.isValid = false
	} else if len(name) > 50 {
		m.editForm.errors["name"] = "Name must be 50 characters or less"
		m.editForm.isValid = false
	}

	// Validate address
	address := strings.TrimSpace(m.editForm.address.Value())
	if address == "" {
		m.editForm.errors["address"] = "Address is required"
		m.editForm.isValid = false
	} else if !m.isValidVeChainAddress(address) {
		m.editForm.errors["address"] = "Invalid VeChain address format"
		m.editForm.isValid = false
	} else if m.isDuplicateAddress(address, currentContact.ID) {
		m.editForm.errors["address"] = "Address already exists in contacts"
		m.editForm.isValid = false
	}
}

func (m *ContactsModel) submitCreateForm() (tea.Model, tea.Cmd) {
	m.validateCreateForm()

	if !m.createForm.isValid {
		m.invalidateCache()
		return m, nil
	}

	// Create new contact
	name := strings.TrimSpace(m.createForm.name.Value())
	address := strings.TrimSpace(m.createForm.address.Value())
	description := strings.TrimSpace(m.createForm.description.Value())

	contact := models.NewContact(name, address, description)
	contact.Category = m.createForm.category
	contact.IsFavorite = m.createForm.isFavorite

	// Parse tags
	tagsStr := strings.TrimSpace(m.createForm.tags.Value())
	if tagsStr != "" {
		tags := strings.Split(tagsStr, ",")
		for _, tag := range tags {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				if err := contact.AddTag(tag, nil, "", ""); err != nil {
					return m, func() tea.Msg {
						return ErrorMsg{Err: fmt.Errorf("failed to add tag: %w", err)}
					}
				}
			}
		}
	}

	return m, m.createContact(contact)
}

func (m *ContactsModel) submitEditForm() (tea.Model, tea.Cmd) {
	m.validateEditForm()

	if !m.editForm.isValid {
		m.invalidateCache()
		return m, nil
	}

	if len(m.filteredContacts) == 0 {
		return m, nil
	}

	// Update existing contact
	contact := m.filteredContacts[m.selectedContact]

	name := strings.TrimSpace(m.editForm.name.Value())
	address := strings.TrimSpace(m.editForm.address.Value())
	description := strings.TrimSpace(m.editForm.description.Value())

	if err := contact.Update(name, address, description, nil, "", ""); err != nil {
		return m, func() tea.Msg {
			return ErrorMsg{Err: fmt.Errorf("failed to update contact: %w", err)}
		}
	}
	contact.Category = m.editForm.category
	contact.IsFavorite = m.editForm.isFavorite

	// Update tags
	contact.Tags = []string{} // Clear existing tags
	tagsStr := strings.TrimSpace(m.editForm.tags.Value())
	if tagsStr != "" {
		tags := strings.Split(tagsStr, ",")
		for _, tag := range tags {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				if err := contact.AddTag(tag, nil, "", ""); err != nil {
					return m, func() tea.Msg {
						return ErrorMsg{Err: fmt.Errorf("failed to add tag: %w", err)}
					}
				}
			}
		}
	}

	return m, m.saveContact(&contact)
}

// Validation helper methods
func (m *ContactsModel) isValidVeChainAddress(address string) bool {
	// Basic VeChain address validation
	if len(address) != 42 {
		return false
	}
	if !strings.HasPrefix(strings.ToLower(address), "0x") {
		return false
	}

	// Check if remaining characters are valid hex
	hex := address[2:]
	for _, char := range hex {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
			return false
		}
	}

	return true
}

func (m *ContactsModel) isDuplicateAddress(address, excludeID string) bool {
	for _, contact := range m.contacts {
		if contact.ID != excludeID && strings.EqualFold(contact.Address, address) {
			return true
		}
	}
	return false
}

func (m *ContactsModel) createContact(contact *models.Contact) tea.Cmd {
	return func() tea.Msg {
		// Check if we have a session manager (simplified check)
		if m.sessionManager == nil {
			return ErrorMsg{Err: fmt.Errorf("session expired")}
		}

		// Load existing contacts
		contactList, err := m.storage.LoadContacts()
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to load contacts: %w", err)}
		}

		// Add new contact
		if err := contactList.Add(contact, nil, "", ""); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to add contact: %w", err)}
		}

		// Save contacts
		err = m.storage.SaveContacts(contactList)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save contact: %w", err)}
		}

		return ContactCreatedMsg{Contact: contact}
	}
}

func (m *ContactsModel) saveContact(contact *models.Contact) tea.Cmd {
	return func() tea.Msg {
		// Check if we have a session manager (simplified check)
		if m.sessionManager == nil {
			return ErrorMsg{Err: fmt.Errorf("session expired")}
		}

		// Update contact in storage
		contactList, err := m.storage.LoadContacts()
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to load contacts: %w", err)}
		}

		// Find and update the contact
		for i, c := range contactList.Contacts {
			if c.ID == contact.ID {
				contactList.Contacts[i] = *contact
				break
			}
		}

		err = m.storage.SaveContacts(contactList)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save contact: %w", err)}
		}

		return ContactUpdatedMsg{Contact: contact}
	}
}

func (m *ContactsModel) deleteContact(contactID string) tea.Cmd {
	return func() tea.Msg {
		// Check if we have a session manager (simplified check)
		if m.sessionManager == nil {
			return ErrorMsg{Err: fmt.Errorf("session expired")}
		}

		contactList, err := m.storage.LoadContacts()
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to load contacts: %w", err)}
		}

		// Remove the contact
		for i, contact := range contactList.Contacts {
			if contact.ID == contactID {
				contactList.Contacts = append(contactList.Contacts[:i], contactList.Contacts[i+1:]...)
				break
			}
		}

		err = m.storage.SaveContacts(contactList)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save contacts: %w", err)}
		}

		return ContactDeletedMsg{ContactID: contactID}
	}
}

// ContactForm methods
func (f *ContactForm) nextField() {
	maxField := int(FormFieldSubmit)
	if !f.showAdvanced {
		maxField = int(FormFieldDescription)
	}

	if f.currentField < maxField {
		f.currentField++
	}
}

func (f *ContactForm) prevField() {
	if f.currentField > 0 {
		f.currentField--
	}
}

func (f *ContactForm) focusCurrentField() tea.Cmd {
	f.name.Blur()
	f.address.Blur()
	f.description.Blur()
	f.tags.Blur()

	switch ContactFormField(f.currentField) {
	case FormFieldName:
		return f.name.Focus()
	case FormFieldAddress:
		return f.address.Focus()
	case FormFieldDescription:
		return f.description.Focus()
	case FormFieldTags:
		return f.tags.Focus()
	}

	return nil
}
