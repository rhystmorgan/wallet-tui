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

type TransactionHistoryModel struct {
	wallet           *models.Wallet
	blockchainClient *blockchain.Client
	sessionManager   *security.SessionManager
	storage          *storage.Storage

	// Transaction data
	transactionHistory *models.TransactionHistory
	selectedIndex      int

	// UI state
	loading           bool
	error             error
	showDetails       bool
	detailTransaction *models.Transaction

	// Filtering and search state
	filterActive    bool
	currentFilter   *models.TransactionFilter
	searchQuery     string
	filterMenuIndex int

	// Performance
	lastRefresh time.Time
	renderCache string
	cacheValid  bool

	// Dimensions
	width  int
	height int
}

type TransactionHistoryLoadedMsg struct {
	History *models.TransactionHistory
}

type TransactionHistoryErrorMsg struct {
	Err error
}

func NewTransactionHistoryModel(wallet *models.Wallet) *TransactionHistoryModel {
	return &TransactionHistoryModel{
		wallet: wallet,
		transactionHistory: &models.TransactionHistory{
			Transactions:  []models.Transaction{},
			TotalCount:    0,
			CurrentPage:   1,
			PageSize:      20,
			HasMore:       false,
			LastFetch:     time.Time{},
			FilteredCount: 0,
		},
		selectedIndex:   0,
		loading:         false,
		error:           nil,
		showDetails:     false,
		filterActive:    false,
		currentFilter:   &models.TransactionFilter{},
		searchQuery:     "",
		filterMenuIndex: 0,
		lastRefresh:     time.Time{},
		cacheValid:      false,
	}
}

func (m *TransactionHistoryModel) SetBlockchainClient(client *blockchain.Client) {
	m.blockchainClient = client
}

func (m *TransactionHistoryModel) SetSessionManager(sessionManager *security.SessionManager) {
	m.sessionManager = sessionManager
}

func (m *TransactionHistoryModel) SetStorage(storage *storage.Storage) {
	m.storage = storage
}

func (m *TransactionHistoryModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.cacheValid = false
}

func (m TransactionHistoryModel) Init() tea.Cmd {
	return m.loadTransactionHistory()
}

func (m TransactionHistoryModel) Update(msg tea.Msg) (TransactionHistoryModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case TransactionHistoryLoadedMsg:
		m.transactionHistory = msg.History
		m.loading = false
		m.error = nil
		m.lastRefresh = time.Now()
		m.cacheValid = false
		return m, nil

	case TransactionHistoryErrorMsg:
		m.error = msg.Err
		m.loading = false
		return m, nil
	}

	return m, nil
}

func (m TransactionHistoryModel) handleKeyPress(msg tea.KeyMsg) (TransactionHistoryModel, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		if m.showDetails {
			m.showDetails = false
			m.detailTransaction = nil
			m.cacheValid = false
			return m, nil
		}
		return m, tea.Cmd(func() tea.Msg {
			return NavigateMsg{State: ViewWalletDashboard}
		})

	case "up", "k":
		if m.filterActive {
			if m.filterMenuIndex > 0 {
				m.filterMenuIndex--
				m.cacheValid = false
			}
		} else if !m.showDetails && len(m.transactionHistory.Transactions) > 0 {
			if m.selectedIndex > 0 {
				m.selectedIndex--
				m.cacheValid = false
			}
		}
		return m, nil

	case "down", "j":
		if m.filterActive {
			if m.filterMenuIndex < 6 { // 7 filter options (0-6)
				m.filterMenuIndex++
				m.cacheValid = false
			}
		} else if !m.showDetails && len(m.transactionHistory.Transactions) > 0 {
			if m.selectedIndex < len(m.transactionHistory.Transactions)-1 {
				m.selectedIndex++
				m.cacheValid = false
			}
		}
		return m, nil

	case "left", "p":
		// Previous page
		if !m.showDetails && m.transactionHistory.CurrentPage > 1 {
			m.transactionHistory.CurrentPage--
			m.selectedIndex = 0
			m.loading = true
			m.cacheValid = false
			return m, m.loadTransactionHistoryPage(m.transactionHistory.CurrentPage)
		}
		return m, nil

	case "right", "n":
		// Next page
		if !m.showDetails && m.transactionHistory.HasMore {
			m.transactionHistory.CurrentPage++
			m.selectedIndex = 0
			m.loading = true
			m.cacheValid = false
			return m, m.loadTransactionHistoryPage(m.transactionHistory.CurrentPage)
		}
		return m, nil

	case "home", "g":
		// Go to first page
		if !m.showDetails && m.transactionHistory.CurrentPage > 1 {
			m.transactionHistory.CurrentPage = 1
			m.selectedIndex = 0
			m.loading = true
			m.cacheValid = false
			return m, m.loadTransactionHistoryPage(1)
		}
		return m, nil

	case "end", "G":
		// Go to last page
		if !m.showDetails && m.transactionHistory.HasMore {
			totalPages := (m.transactionHistory.TotalCount + m.transactionHistory.PageSize - 1) / m.transactionHistory.PageSize
			if m.transactionHistory.CurrentPage < totalPages {
				m.transactionHistory.CurrentPage = totalPages
				m.selectedIndex = 0
				m.loading = true
				m.cacheValid = false
				return m, m.loadTransactionHistoryPage(totalPages)
			}
		}
		return m, nil

	case "pgup":
		// Previous 5 pages
		if !m.showDetails && m.transactionHistory.CurrentPage > 1 {
			newPage := m.transactionHistory.CurrentPage - 5
			if newPage < 1 {
				newPage = 1
			}
			m.transactionHistory.CurrentPage = newPage
			m.selectedIndex = 0
			m.loading = true
			m.cacheValid = false
			return m, m.loadTransactionHistoryPage(newPage)
		}
		return m, nil

	case "pgdown":
		// Next 5 pages
		if !m.showDetails {
			totalPages := (m.transactionHistory.TotalCount + m.transactionHistory.PageSize - 1) / m.transactionHistory.PageSize
			newPage := m.transactionHistory.CurrentPage + 5
			if newPage > totalPages {
				newPage = totalPages
			}
			if newPage > m.transactionHistory.CurrentPage {
				m.transactionHistory.CurrentPage = newPage
				m.selectedIndex = 0
				m.loading = true
				m.cacheValid = false
				return m, m.loadTransactionHistoryPage(newPage)
			}
		}
		return m, nil

	case "enter":
		if m.filterActive {
			// Handle filter selection
			m.toggleFilterOption()
			m.cacheValid = false
			// Reload with new filter
			m.loading = true
			return m, m.loadTransactionHistoryPage(1)
		} else if !m.showDetails && len(m.transactionHistory.Transactions) > 0 {
			m.detailTransaction = &m.transactionHistory.Transactions[m.selectedIndex]
			m.showDetails = true
			m.cacheValid = false
		}
		return m, nil

	case "r":
		if !m.showDetails {
			m.loading = true
			m.error = nil
			m.cacheValid = false
			return m, m.loadTransactionHistoryPage(m.transactionHistory.CurrentPage)
		}
		return m, nil

	case "f":
		// Toggle filter panel (future implementation)
		m.filterActive = !m.filterActive
		m.cacheValid = false
		return m, nil
	}

	return m, nil
}

func (m *TransactionHistoryModel) toggleFilterOption() {
	switch m.filterMenuIndex {
	case 0: // Direction filter
		switch m.currentFilter.Direction {
		case "":
			m.currentFilter.Direction = models.TransactionDirectionSent
		case models.TransactionDirectionSent:
			m.currentFilter.Direction = models.TransactionDirectionReceived
		case models.TransactionDirectionReceived:
			m.currentFilter.Direction = models.TransactionDirectionSelf
		case models.TransactionDirectionSelf:
			m.currentFilter.Direction = ""
		}
	case 1: // Status filter
		switch m.currentFilter.Status {
		case "":
			m.currentFilter.Status = models.TransactionStatusPending
		case models.TransactionStatusPending:
			m.currentFilter.Status = models.TransactionStatusConfirmed
		case models.TransactionStatusConfirmed:
			m.currentFilter.Status = models.TransactionStatusFailed
		case models.TransactionStatusFailed:
			m.currentFilter.Status = ""
		}
	case 2: // Asset filter
		switch m.currentFilter.Asset {
		case "":
			m.currentFilter.Asset = "VET"
		case "VET":
			m.currentFilter.Asset = "VTHO"
		case "VTHO":
			m.currentFilter.Asset = ""
		}
	case 3: // Date range - Last 7 days
		if m.currentFilter.DateFrom.IsZero() {
			m.currentFilter.DateFrom = time.Now().AddDate(0, 0, -7)
			m.currentFilter.DateTo = time.Now()
		} else {
			m.currentFilter.DateFrom = time.Time{}
			m.currentFilter.DateTo = time.Time{}
		}
	case 4: // Date range - Last 30 days
		if m.currentFilter.DateFrom.IsZero() {
			m.currentFilter.DateFrom = time.Now().AddDate(0, 0, -30)
			m.currentFilter.DateTo = time.Now()
		} else {
			m.currentFilter.DateFrom = time.Time{}
			m.currentFilter.DateTo = time.Time{}
		}
	case 5: // Contacts only
		m.currentFilter.ContactsOnly = !m.currentFilter.ContactsOnly
	case 6: // Clear all filters
		m.currentFilter = &models.TransactionFilter{}
	}
}

func (m TransactionHistoryModel) applyFilters(transactions []models.Transaction) []models.Transaction {
	if m.currentFilter == nil {
		return transactions
	}

	filtered := make([]models.Transaction, 0)

	for _, tx := range transactions {
		// Direction filter
		if m.currentFilter.Direction != "" && tx.Direction != m.currentFilter.Direction {
			continue
		}

		// Status filter
		if m.currentFilter.Status != "" && tx.Status != m.currentFilter.Status {
			continue
		}

		// Asset filter
		if m.currentFilter.Asset != "" && tx.Asset != m.currentFilter.Asset {
			continue
		}

		// Date range filter
		if !m.currentFilter.DateFrom.IsZero() && !m.currentFilter.DateTo.IsZero() {
			if tx.Timestamp.Before(m.currentFilter.DateFrom) || tx.Timestamp.After(m.currentFilter.DateTo) {
				continue
			}
		}

		// Contacts only filter
		if m.currentFilter.ContactsOnly && tx.ContactName == "" {
			continue
		}

		// Search query filter
		if m.currentFilter.SearchQuery != "" {
			query := strings.ToLower(m.currentFilter.SearchQuery)
			if !strings.Contains(strings.ToLower(tx.Hash), query) &&
				!strings.Contains(strings.ToLower(tx.From), query) &&
				!strings.Contains(strings.ToLower(tx.To), query) &&
				!strings.Contains(strings.ToLower(tx.ContactName), query) {
				continue
			}
		}

		filtered = append(filtered, tx)
	}

	return filtered
}

func (m TransactionHistoryModel) loadTransactionHistory() tea.Cmd {
	return m.loadTransactionHistoryPage(1)
}

func (m TransactionHistoryModel) loadTransactionHistoryPage(page int) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// For now, create mock transaction data
		// In a real implementation, this would fetch from the blockchain with pagination
		allMockTransactions := m.createAllMockTransactions()

		// Apply filters
		filteredTransactions := m.applyFilters(allMockTransactions)

		// Calculate pagination on filtered results
		pageSize := 20
		startIndex := (page - 1) * pageSize
		endIndex := startIndex + pageSize

		var pageTransactions []models.Transaction
		if startIndex < len(filteredTransactions) {
			if endIndex > len(filteredTransactions) {
				endIndex = len(filteredTransactions)
			}
			pageTransactions = filteredTransactions[startIndex:endIndex]
		}

		totalPages := (len(filteredTransactions) + pageSize - 1) / pageSize
		hasMore := page < totalPages

		history := &models.TransactionHistory{
			Transactions:  pageTransactions,
			TotalCount:    len(allMockTransactions),
			CurrentPage:   page,
			PageSize:      pageSize,
			HasMore:       hasMore,
			LastFetch:     time.Now(),
			FilteredCount: len(filteredTransactions),
		}

		return TransactionHistoryLoadedMsg{History: history}
	})
}

func (m TransactionHistoryModel) createAllMockTransactions() []models.Transaction {
	// Create a larger set of mock transactions for pagination testing
	transactions := []models.Transaction{}

	// Add the original mock transactions
	originalTransactions := m.createMockTransactions()
	transactions = append(transactions, originalTransactions...)

	// Add more mock transactions to test pagination
	for i := 5; i <= 50; i++ {
		tx := models.Transaction{
			ID:             fmt.Sprintf("tx_%03d", i),
			Hash:           fmt.Sprintf("0x%064d", i),
			From:           m.wallet.Address,
			To:             fmt.Sprintf("0x%040d", i),
			Amount:         big.NewInt(int64(i * 100000000000000000)), // Variable amounts
			Asset:          "VET",
			Timestamp:      time.Now().Add(-time.Duration(i) * time.Hour),
			Status:         models.TransactionStatusConfirmed,
			Confirmations:  i,
			GasUsed:        big.NewInt(21000),
			GasPrice:       big.NewInt(1000000000000000),
			BlockNumber:    uint64(15234567 - i),
			BlockHash:      fmt.Sprintf("0x%064d", 15234567-i),
			TransactionFee: big.NewInt(21000000000000000),
			Direction:      models.TransactionDirectionSent,
			ContactName:    fmt.Sprintf("Contact %d", i),
		}

		// Vary some properties for diversity
		if i%3 == 0 {
			tx.Direction = models.TransactionDirectionReceived
			tx.From, tx.To = tx.To, tx.From
		}
		if i%5 == 0 {
			tx.Asset = "VTHO"
			tx.Amount = func() *big.Int {
				val, _ := new(big.Int).SetString(fmt.Sprintf("%d000000000000000000", i*10), 10)
				return val
			}()
		}
		if i%7 == 0 {
			tx.Status = models.TransactionStatusPending
			tx.Confirmations = 0
			tx.BlockNumber = 0
			tx.BlockHash = ""
		}

		transactions = append(transactions, tx)
	}

	return transactions
}

func (m TransactionHistoryModel) createMockTransactions() []models.Transaction {
	// Create some mock transactions for demonstration
	transactions := []models.Transaction{
		{
			ID:             "tx_001",
			Hash:           "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			From:           m.wallet.Address,
			To:             "0x9876543210987654321098765432109876543210",
			Amount:         big.NewInt(1000000000000000000), // 1 VET
			Asset:          "VET",
			Timestamp:      time.Now().Add(-2 * time.Hour),
			Status:         models.TransactionStatusConfirmed,
			Confirmations:  12,
			GasUsed:        big.NewInt(21000),
			GasPrice:       big.NewInt(1000000000000000),
			BlockNumber:    15234567,
			BlockHash:      "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			TransactionFee: big.NewInt(21000000000000000),
			Direction:      models.TransactionDirectionSent,
			ContactName:    "Alice",
		},
		{
			ID:             "tx_002",
			Hash:           "0x2345678901bcdef12345678901bcdef12345678901bcdef12345678901bcdef1",
			From:           "0x8765432109876543210987654321098765432109",
			To:             m.wallet.Address,
			Amount:         big.NewInt(500000000000000000), // 0.5 VET
			Asset:          "VET",
			Timestamp:      time.Now().Add(-4 * time.Hour),
			Status:         models.TransactionStatusConfirmed,
			Confirmations:  24,
			GasUsed:        big.NewInt(21000),
			GasPrice:       big.NewInt(1000000000000000),
			BlockNumber:    15234550,
			BlockHash:      "0xbcdef12345678901bcdef12345678901bcdef12345678901bcdef12345678901",
			TransactionFee: big.NewInt(21000000000000000),
			Direction:      models.TransactionDirectionReceived,
			ContactName:    "Bob",
		},
		{
			ID:             "tx_003",
			Hash:           "0x3456789012cdef123456789012cdef123456789012cdef123456789012cdef12",
			From:           m.wallet.Address,
			To:             "0x7654321098765432109876543210987654321098",
			Amount:         big.NewInt(250000000000000000), // 0.25 VET
			Asset:          "VET",
			Timestamp:      time.Now().Add(-6 * time.Hour),
			Status:         models.TransactionStatusPending,
			Confirmations:  0,
			GasUsed:        big.NewInt(21000),
			GasPrice:       big.NewInt(1000000000000000),
			BlockNumber:    0,
			BlockHash:      "",
			TransactionFee: big.NewInt(21000000000000000),
			Direction:      models.TransactionDirectionSent,
			ContactName:    "Charlie",
		},
		{
			ID:             "tx_004",
			Hash:           "0x456789013def1234567890123def1234567890123def1234567890123def123",
			From:           m.wallet.Address,
			To:             "0x6543210987654321098765432109876543210987",
			Amount:         func() *big.Int { val, _ := new(big.Int).SetString("75000000000000000000", 10); return val }(), // 75 VTHO
			Asset:          "VTHO",
			Timestamp:      time.Now().Add(-8 * time.Hour),
			Status:         models.TransactionStatusConfirmed,
			Confirmations:  36,
			GasUsed:        big.NewInt(80000),
			GasPrice:       big.NewInt(1000000000000000),
			BlockNumber:    15234520,
			BlockHash:      "0xcdef123456789012cdef123456789012cdef123456789012cdef123456789012",
			TransactionFee: big.NewInt(80000000000000000),
			Direction:      models.TransactionDirectionSent,
			ContactName:    "David",
		},
	}

	return transactions
}

func (m *TransactionHistoryModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	if m.cacheValid && m.renderCache != "" {
		return m.renderCache
	}

	var content string

	if m.showDetails && m.detailTransaction != nil {
		content = m.renderTransactionDetail()
	} else if m.filterActive {
		content = m.renderFilterPanel()
	} else {
		content = m.renderTransactionList()
	}

	m.renderCache = content
	m.cacheValid = true
	return content
}

func (m TransactionHistoryModel) renderTransactionList() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Blue)).
		Bold(true).
		Padding(0, 1)

	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Bold(true).
		Padding(0, 1)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Padding(0, 1)

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Blue)).
		Background(lipgloss.Color(utils.Colours.Surface0)).
		Bold(true).
		Padding(0, 1)

	pendingStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Yellow)).
		Padding(0, 1)

	confirmedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Green)).
		Padding(0, 1)

	failedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Red)).
		Padding(0, 1)

	var content strings.Builder

	// Title
	title := fmt.Sprintf("Transaction History - %s", m.wallet.Name)
	content.WriteString(titleStyle.Render(title))
	content.WriteString("\n\n")

	// Loading state
	if m.loading {
		content.WriteString(normalStyle.Render("Loading transactions..."))
		content.WriteString("\n\n")
	}

	// Error state
	if m.error != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(utils.Colours.Red)).
			Bold(true).
			Padding(0, 1)
		content.WriteString(errorStyle.Render(fmt.Sprintf("Error: %s", m.error.Error())))
		content.WriteString("\n\n")
	}

	// Transaction list
	if len(m.transactionHistory.Transactions) == 0 && !m.loading {
		content.WriteString(normalStyle.Render("No transactions found"))
		content.WriteString("\n\n")
	} else {
		// Header
		header := fmt.Sprintf("%-20s %-10s %-15s %-20s %-15s", "Date", "Direction", "Amount", "Asset", "Contact")
		content.WriteString(headerStyle.Render(header))
		content.WriteString("\n")

		// Separator
		separator := strings.Repeat("─", m.width-2)
		content.WriteString(normalStyle.Render(separator))
		content.WriteString("\n")

		// Transaction items
		for i, tx := range m.transactionHistory.Transactions {
			style := normalStyle
			if i == m.selectedIndex {
				style = selectedStyle
			}

			// Status indicator
			statusIndicator := "●"
			statusStyle := confirmedStyle
			switch tx.Status {
			case models.TransactionStatusPending:
				statusIndicator = "○"
				statusStyle = pendingStyle
			case models.TransactionStatusFailed, models.TransactionStatusReverted:
				statusIndicator = "✗"
				statusStyle = failedStyle
			}

			// Format transaction data
			date := tx.Timestamp.Format("2006-01-02 15:04")
			direction := string(tx.Direction)
			amount := utils.FormatAmount(tx.Amount, 18)
			asset := tx.Asset
			contact := tx.ContactName
			if contact == "" {
				if tx.Direction == models.TransactionDirectionSent {
					contact = utils.FormatAddress(tx.To, 6, 4)
				} else {
					contact = utils.FormatAddress(tx.From, 6, 4)
				}
			}

			line := fmt.Sprintf("%-20s %-10s %-15s %-20s %-15s",
				date, direction, amount, asset, contact)

			// Combine status indicator with transaction line
			statusPart := statusStyle.Render(statusIndicator)
			linePart := style.Render(line)
			content.WriteString(statusPart + " " + linePart)
			content.WriteString("\n")
		}
	}

	// Pagination info and controls
	if m.transactionHistory.TotalCount > 0 {
		content.WriteString("\n")
		totalPages := (m.transactionHistory.TotalCount + m.transactionHistory.PageSize - 1) / m.transactionHistory.PageSize

		// Pagination info
		paginationInfo := fmt.Sprintf("Page %d of %d (%d transactions)",
			m.transactionHistory.CurrentPage,
			totalPages,
			m.transactionHistory.TotalCount)
		content.WriteString(normalStyle.Render(paginationInfo))
		content.WriteString("\n")

		// Pagination controls
		paginationStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(utils.Colours.Blue)).
			Padding(0, 1)

		var paginationControls strings.Builder

		// Previous page indicator
		if m.transactionHistory.CurrentPage > 1 {
			paginationControls.WriteString("[←]")
		} else {
			paginationControls.WriteString(" ← ")
		}

		// Page numbers (show current and nearby pages)
		startPage := m.transactionHistory.CurrentPage - 2
		if startPage < 1 {
			startPage = 1
		}
		endPage := startPage + 4
		if endPage > totalPages {
			endPage = totalPages
			startPage = endPage - 4
			if startPage < 1 {
				startPage = 1
			}
		}

		for i := startPage; i <= endPage; i++ {
			if i == m.transactionHistory.CurrentPage {
				paginationControls.WriteString(fmt.Sprintf(" [%d] ", i))
			} else {
				paginationControls.WriteString(fmt.Sprintf(" %d ", i))
			}
		}

		// Next page indicator
		if m.transactionHistory.HasMore {
			paginationControls.WriteString("[→]")
		} else {
			paginationControls.WriteString(" → ")
		}

		content.WriteString(paginationStyle.Render(paginationControls.String()))
		content.WriteString("\n")
	}

	// Help text
	content.WriteString("\n")
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Overlay0)).
		Italic(true).
		Padding(0, 1)

	helpText := "↑/↓: navigate • ←/→: pages • Enter: details • r: refresh • f: filter • q/Esc: back"
	content.WriteString(helpStyle.Render(helpText))

	return content.String()
}

func (m TransactionHistoryModel) renderTransactionDetail() string {
	if m.detailTransaction == nil {
		return "No transaction selected"
	}

	tx := m.detailTransaction

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Blue)).
		Bold(true).
		Padding(0, 1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Bold(true).
		Padding(0, 1)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Subtext1)).
		Padding(0, 1)

	var content strings.Builder

	// Title
	content.WriteString(titleStyle.Render("Transaction Details"))
	content.WriteString("\n\n")

	// Transaction Hash
	content.WriteString(labelStyle.Render("Hash:"))
	content.WriteString(valueStyle.Render(tx.Hash))
	content.WriteString("\n")

	// Status
	content.WriteString(labelStyle.Render("Status:"))
	statusText := tx.Status.String()
	if tx.Status == models.TransactionStatusConfirmed && tx.Confirmations > 0 {
		statusText += fmt.Sprintf(" (%d confirmations)", tx.Confirmations)
	}
	content.WriteString(valueStyle.Render(statusText))
	content.WriteString("\n\n")

	// From/To
	content.WriteString(labelStyle.Render("From:"))
	fromText := tx.From
	if tx.From == m.wallet.Address {
		fromText += " (My Wallet)"
	}
	content.WriteString(valueStyle.Render(fromText))
	content.WriteString("\n")

	content.WriteString(labelStyle.Render("To:"))
	toText := tx.To
	if tx.To == m.wallet.Address {
		toText += " (My Wallet)"
	} else if tx.ContactName != "" {
		toText += fmt.Sprintf(" (%s)", tx.ContactName)
	}
	content.WriteString(valueStyle.Render(toText))
	content.WriteString("\n\n")

	// Amount and fees
	content.WriteString(labelStyle.Render("Amount:"))
	content.WriteString(valueStyle.Render(fmt.Sprintf("%s %s", utils.FormatAmount(tx.Amount, 18), tx.Asset)))
	content.WriteString("\n")

	if tx.TransactionFee != nil {
		content.WriteString(labelStyle.Render("Fee:"))
		content.WriteString(valueStyle.Render(fmt.Sprintf("%s VET", utils.FormatAmount(tx.TransactionFee, 18))))
		content.WriteString("\n")
	}

	// Block information
	if tx.BlockNumber > 0 {
		content.WriteString("\n")
		content.WriteString(labelStyle.Render("Block:"))
		content.WriteString(valueStyle.Render(fmt.Sprintf("%d", tx.BlockNumber)))
		content.WriteString("\n")
	}

	// Timestamp
	content.WriteString(labelStyle.Render("Time:"))
	content.WriteString(valueStyle.Render(tx.Timestamp.Format("2006-01-02 15:04:05")))
	content.WriteString("\n")

	// Help text
	content.WriteString("\n")
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Overlay0)).
		Italic(true).
		Padding(0, 1)

	helpText := "Esc: close • q: back to dashboard"
	content.WriteString(helpStyle.Render(helpText))

	return content.String()
}

func (m *TransactionHistoryModel) renderFilterPanel() string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Blue)).
		Bold(true).
		Padding(0, 1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Bold(true).
		Padding(0, 1)

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Blue)).
		Background(lipgloss.Color(utils.Colours.Surface0)).
		Bold(true).
		Padding(0, 1)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Text)).
		Padding(0, 1)

	var content strings.Builder

	// Title
	content.WriteString(titleStyle.Render("Transaction Filters"))
	content.WriteString("\n\n")

	// Filter options
	filterOptions := []struct {
		label string
		value string
	}{
		{"Direction", m.getDirectionFilterText()},
		{"Status", m.getStatusFilterText()},
		{"Asset", m.getAssetFilterText()},
		{"Last 7 days", m.getDateFilterText(7)},
		{"Last 30 days", m.getDateFilterText(30)},
		{"Contacts only", m.getContactsFilterText()},
		{"Clear all filters", ""},
	}

	for i, option := range filterOptions {
		style := normalStyle
		if i == m.filterMenuIndex {
			style = selectedStyle
		}

		line := fmt.Sprintf("%-20s %s", option.label, option.value)
		content.WriteString(style.Render(line))
		content.WriteString("\n")
	}

	// Active filters summary
	activeFilters := m.getActiveFiltersCount()
	if activeFilters > 0 {
		content.WriteString("\n")
		content.WriteString(labelStyle.Render(fmt.Sprintf("Active filters: %d", activeFilters)))
		content.WriteString("\n")
	}

	// Help text
	content.WriteString("\n")
	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(utils.Colours.Overlay0)).
		Italic(true).
		Padding(0, 1)

	helpText := "↑/↓: navigate • Enter: toggle • f/Esc: close • q: back to dashboard"
	content.WriteString(helpStyle.Render(helpText))

	return content.String()
}

func (m *TransactionHistoryModel) getDirectionFilterText() string {
	switch m.currentFilter.Direction {
	case models.TransactionDirectionSent:
		return "[Sent]"
	case models.TransactionDirectionReceived:
		return "[Received]"
	case models.TransactionDirectionSelf:
		return "[Self]"
	default:
		return "[All]"
	}
}

func (m *TransactionHistoryModel) getStatusFilterText() string {
	switch m.currentFilter.Status {
	case models.TransactionStatusPending:
		return "[Pending]"
	case models.TransactionStatusConfirmed:
		return "[Confirmed]"
	case models.TransactionStatusFailed:
		return "[Failed]"
	default:
		return "[All]"
	}
}

func (m *TransactionHistoryModel) getAssetFilterText() string {
	switch m.currentFilter.Asset {
	case "VET":
		return "[VET]"
	case "VTHO":
		return "[VTHO]"
	default:
		return "[All]"
	}
}

func (m *TransactionHistoryModel) getDateFilterText(days int) string {
	if !m.currentFilter.DateFrom.IsZero() {
		daysDiff := int(time.Since(m.currentFilter.DateFrom).Hours() / 24)
		if daysDiff >= days-1 && daysDiff <= days+1 {
			return "[Active]"
		}
	}
	return "[Inactive]"
}

func (m *TransactionHistoryModel) getContactsFilterText() string {
	if m.currentFilter.ContactsOnly {
		return "[Active]"
	}
	return "[Inactive]"
}

func (m *TransactionHistoryModel) getActiveFiltersCount() int {
	count := 0
	if m.currentFilter.Direction != "" {
		count++
	}
	if m.currentFilter.Status != "" {
		count++
	}
	if m.currentFilter.Asset != "" {
		count++
	}
	if !m.currentFilter.DateFrom.IsZero() {
		count++
	}
	if m.currentFilter.ContactsOnly {
		count++
	}
	return count
}
