package views

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"rhystmorgan/veWallet/internal/blockchain"
	"rhystmorgan/veWallet/internal/config"
	"rhystmorgan/veWallet/internal/models"
	"rhystmorgan/veWallet/internal/security"
	"rhystmorgan/veWallet/internal/storage"
	"rhystmorgan/veWallet/internal/utils"
)

type ViewState int

const (
	ViewWalletSelector ViewState = iota
	ViewWalletCreate
	ViewWalletImport
	ViewWalletDashboard
	ViewSendTransaction
	ViewTransactionHistory
	ViewContacts
	ViewSettings
)

type AppModel struct {
	state            ViewState
	width            int
	height           int
	storage          *storage.Storage
	config           *storage.Config
	blockchainClient *blockchain.Client
	networkStatus    blockchain.NetworkStatus
	currentWallet    *models.Wallet
	wallets          []storage.EncryptedWallet
	contacts         *models.ContactList

	// Session management
	sessionManager  *security.SessionManager
	securityManager *security.SecurityManager
	activityMonitor *security.ActivityMonitor

	walletSelector     *WalletSelectorModel
	walletCreate       *WalletCreateModel
	walletImport       *WalletImportModel
	walletDashboard    *WalletDashboardModel
	sendTransaction    *SendTransactionModel
	transactionHistory *TransactionHistoryModel
	contactsView       *ContactsModel

	err error
}

type NavigateMsg struct {
	State ViewState
	Data  interface{}
}

type ErrorMsg struct {
	Err error
}

type WalletLoadedMsg struct {
	Wallet *models.Wallet
}

func NewAppModel() (*AppModel, error) {
	storage, err := storage.NewStorage()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	storageConfig, err := storage.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	contacts, err := storage.LoadContacts()
	if err != nil {
		return nil, fmt.Errorf("failed to load contacts: %w", err)
	}

	wallets, err := storage.ListWallets()
	if err != nil {
		return nil, fmt.Errorf("failed to list wallets: %w", err)
	}

	// Initialize blockchain client
	blockchainConfig, err := config.LoadBlockchainConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load blockchain config: %w", err)
	}

	blockchainClient, err := blockchain.NewClient(blockchainConfig.ToBlockchainConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize blockchain client: %w", err)
	}

	// Initialize session management
	sessionManager := security.NewSessionManager(storage)
	securityManager := security.NewSecurityManager(sessionManager)
	activityMonitor := security.NewActivityMonitor(sessionManager)

	app := &AppModel{
		state:            ViewWalletSelector,
		storage:          storage,
		config:           storageConfig,
		blockchainClient: blockchainClient,
		networkStatus:    blockchainClient.GetStatus(),
		contacts:         contacts,
		wallets:          wallets,
		sessionManager:   sessionManager,
		securityManager:  securityManager,
		activityMonitor:  activityMonitor,
	}

	app.walletSelector = NewWalletSelectorModel(wallets)
	app.walletCreate = NewWalletCreateModel()
	app.walletImport = NewWalletImportModel()

	return app, nil
}

func (m AppModel) Init() tea.Cmd {
	return nil
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			if m.state != ViewWalletSelector {
				return m.navigateTo(ViewWalletSelector, nil)
			}
		}

	case NavigateMsg:
		return m.navigateTo(msg.State, msg.Data)

	case ErrorMsg:
		m.err = msg.Err
		return m, nil

	case WalletLoadedMsg:
		m.currentWallet = msg.Wallet
		m.walletDashboard = NewWalletDashboardModel(msg.Wallet)
		m.walletDashboard.SetBlockchainClient(m.blockchainClient)
		return m.navigateTo(ViewWalletDashboard, nil)

	case WalletCreatedMsg:
		m.currentWallet = msg.Wallet
		m.walletDashboard = NewWalletDashboardModel(msg.Wallet)
		m.walletDashboard.SetBlockchainClient(m.blockchainClient)
		// Refresh wallet list
		wallets, err := m.storage.ListWallets()
		if err == nil {
			m.wallets = wallets
			m.walletSelector = NewWalletSelectorModel(wallets)
		}
		return m.navigateTo(ViewWalletDashboard, nil)

	case WalletImportedMsg:
		m.currentWallet = msg.Wallet
		m.walletDashboard = NewWalletDashboardModel(msg.Wallet)
		m.walletDashboard.SetBlockchainClient(m.blockchainClient)
		// Refresh wallet list
		wallets, err := m.storage.ListWallets()
		if err == nil {
			m.wallets = wallets
			m.walletSelector = NewWalletSelectorModel(wallets)
		}
		return m.navigateTo(ViewWalletDashboard, nil)
	}

	switch m.state {
	case ViewWalletSelector:
		if m.walletSelector != nil {
			*m.walletSelector, cmd = m.walletSelector.Update(msg)
		}
	case ViewWalletCreate:
		if m.walletCreate != nil {
			*m.walletCreate, cmd = m.walletCreate.Update(msg)
		}
	case ViewWalletImport:
		if m.walletImport != nil {
			*m.walletImport, cmd = m.walletImport.Update(msg)
		}
	case ViewWalletDashboard:
		if m.walletDashboard != nil {
			*m.walletDashboard, cmd = m.walletDashboard.Update(msg)
		}
	case ViewSendTransaction:
		if m.sendTransaction != nil {
			*m.sendTransaction, cmd = m.sendTransaction.Update(msg)
		}
	case ViewTransactionHistory:
		if m.transactionHistory != nil {
			*m.transactionHistory, cmd = m.transactionHistory.Update(msg)
		}
	case ViewContacts:
		if m.contactsView != nil {
			model, updateCmd := m.contactsView.Update(msg)
			if contactsModel, ok := model.(*ContactsModel); ok {
				m.contactsView = contactsModel
				cmd = updateCmd
			}
		}
	}

	return m, cmd
}

func (m AppModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	var content string

	switch m.state {
	case ViewWalletSelector:
		if m.walletSelector != nil {
			content = m.walletSelector.View()
		}
	case ViewWalletCreate:
		if m.walletCreate != nil {
			content = m.walletCreate.View()
		}
	case ViewWalletImport:
		if m.walletImport != nil {
			content = m.walletImport.View()
		}
	case ViewWalletDashboard:
		if m.walletDashboard != nil {
			content = m.walletDashboard.View()
		}
	case ViewSendTransaction:
		if m.sendTransaction != nil {
			content = m.sendTransaction.View()
		}
	case ViewTransactionHistory:
		if m.transactionHistory != nil {
			content = m.transactionHistory.View()
		}
	case ViewContacts:
		if m.contactsView != nil {
			content = m.contactsView.View()
		}
	default:
		content = "Unknown view"
	}

	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(utils.Colours.Red)).
			Bold(true).
			Padding(1)
		content += "\n" + errorStyle.Render(fmt.Sprintf("Error: %s", m.err.Error()))
	}

	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Render(content)
}

func (m AppModel) navigateTo(state ViewState, data interface{}) (tea.Model, tea.Cmd) {
	m.state = state
	m.err = nil

	// Record navigation activity
	viewName := m.getViewName(state)
	m.RecordActivity("navigate", viewName)

	switch state {
	case ViewWalletCreate:
		if m.walletCreate == nil {
			m.walletCreate = NewWalletCreateModel()
		}
		m.walletCreate.SetStorage(m.storage)
	case ViewWalletImport:
		if m.walletImport == nil {
			m.walletImport = NewWalletImportModel()
		}
		m.walletImport.SetStorage(m.storage)
	case ViewWalletDashboard:
		if m.walletDashboard != nil {
			m.walletDashboard.SetSessionManager(m.sessionManager)
		}
	case ViewSendTransaction:
		if m.sendTransaction == nil && m.currentWallet != nil {
			m.sendTransaction = NewSendTransactionModel(m.currentWallet)
			m.sendTransaction.SetBlockchainClient(m.blockchainClient)
			m.sendTransaction.SetStorage(m.storage)
		}
		if m.sendTransaction != nil {
			m.sendTransaction.SetSessionManager(m.sessionManager)
		}
	case ViewTransactionHistory:
		if m.transactionHistory == nil && m.currentWallet != nil {
			m.transactionHistory = NewTransactionHistoryModel(m.currentWallet)
			m.transactionHistory.SetBlockchainClient(m.blockchainClient)
			m.transactionHistory.SetStorage(m.storage)
		}
		if m.transactionHistory != nil {
			m.transactionHistory.SetSessionManager(m.sessionManager)
			m.transactionHistory.SetSize(m.width, m.height)
		}
	case ViewContacts:
		if m.contactsView == nil && m.currentWallet != nil {
			m.contactsView = NewContactsModel(m.sessionManager, m.storage, m.currentWallet)
		}
		if m.contactsView != nil {
			m.contactsView.width = m.width
			m.contactsView.height = m.height
		}
	}

	return m, nil
}

func (m *AppModel) getViewName(state ViewState) string {
	switch state {
	case ViewWalletSelector:
		return "wallet_selector"
	case ViewWalletCreate:
		return "wallet_create"
	case ViewWalletImport:
		return "wallet_import"
	case ViewWalletDashboard:
		return "wallet_dashboard"
	case ViewSendTransaction:
		return "send_transaction"
	case ViewTransactionHistory:
		return "transaction_history"
	case ViewContacts:
		return "contacts"
	case ViewSettings:
		return "settings"
	default:
		return "unknown"
	}
}

func NavigateTo(state ViewState, data interface{}) tea.Cmd {
	return func() tea.Msg {
		return NavigateMsg{State: state, Data: data}
	}
}

func ShowError(err error) tea.Cmd {
	return func() tea.Msg {
		return ErrorMsg{Err: err}
	}
}

func LoadWallet(wallet *models.Wallet) tea.Cmd {
	return func() tea.Msg {
		return WalletLoadedMsg{Wallet: wallet}
	}
}

func (m *AppModel) GetBlockchainClient() *blockchain.Client {
	return m.blockchainClient
}

func (m *AppModel) GetNetworkStatus() blockchain.NetworkStatus {
	return m.networkStatus
}

func (m *AppModel) UpdateNetworkStatus() {
	if m.blockchainClient != nil {
		m.networkStatus = m.blockchainClient.GetStatus()
	}
}

// Session management methods
func (m *AppModel) GetSessionManager() *security.SessionManager {
	return m.sessionManager
}

func (m *AppModel) GetSecurityManager() *security.SecurityManager {
	return m.securityManager
}

func (m *AppModel) GetActivityMonitor() *security.ActivityMonitor {
	return m.activityMonitor
}

func (m *AppModel) RecordActivity(action, view string) {
	if m.currentWallet != nil && m.activityMonitor != nil {
		m.activityMonitor.RecordActivity(m.currentWallet.ID, action, view)
	}
}

func (m *AppModel) IsWalletSessionActive() bool {
	if m.currentWallet == nil || m.sessionManager == nil {
		return false
	}
	_, active := m.sessionManager.GetSession(m.currentWallet.ID)
	return active
}

func (m *AppModel) CreateWalletSession(wallet *models.Wallet) error {
	if m.sessionManager == nil {
		return fmt.Errorf("session manager not initialized")
	}
	_, err := m.sessionManager.CreateSession(wallet)
	return err
}

func (m *AppModel) CloseCurrentSession() error {
	if m.currentWallet == nil || m.sessionManager == nil {
		return nil
	}
	return m.sessionManager.CloseSession(m.currentWallet.ID)
}

func (m *AppModel) Shutdown() {
	if m.sessionManager != nil {
		m.sessionManager.Shutdown()
	}
}
