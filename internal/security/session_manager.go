package security

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"rhystmorgan/veWallet/internal/models"
	"rhystmorgan/veWallet/internal/storage"
)

type SessionManager struct {
	sessions        map[string]*WalletSession
	storage         *storage.Storage
	config          *SessionConfig
	mu              sync.RWMutex
	cleanupTicker   *time.Ticker
	activityChannel chan SessionActivity
	stopCleanup     chan bool
}

type WalletSession struct {
	WalletID       string
	UnlockedWallet *models.Wallet
	CreatedAt      time.Time
	LastActivity   time.Time
	ExpiresAt      time.Time
	SessionToken   string
	IsActive       bool
}

type SessionConfig struct {
	DefaultTimeout  time.Duration
	MaxSessions     int
	CleanupInterval time.Duration
	InactivityLimit time.Duration
	RequirePassword bool
}

type SessionActivity struct {
	WalletID  string
	Action    string
	Timestamp time.Time
	ViewName  string
}

func NewSessionManager(storage *storage.Storage) *SessionManager {
	config := &SessionConfig{
		DefaultTimeout:  5 * time.Minute,
		MaxSessions:     5,
		CleanupInterval: 30 * time.Second,
		InactivityLimit: 5 * time.Minute,
		RequirePassword: true,
	}

	sm := &SessionManager{
		sessions:        make(map[string]*WalletSession),
		storage:         storage,
		config:          config,
		activityChannel: make(chan SessionActivity, 100),
		stopCleanup:     make(chan bool),
	}

	sm.startCleanupRoutine()
	sm.startActivityMonitor()

	return sm
}

func (sm *SessionManager) CreateSession(wallet *models.Wallet) (*WalletSession, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if len(sm.sessions) >= sm.config.MaxSessions {
		return nil, fmt.Errorf("maximum number of sessions reached")
	}

	token, err := sm.generateSessionToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate session token: %w", err)
	}

	now := time.Now()
	session := &WalletSession{
		WalletID:       wallet.ID,
		UnlockedWallet: wallet,
		CreatedAt:      now,
		LastActivity:   now,
		ExpiresAt:      now.Add(sm.config.DefaultTimeout),
		SessionToken:   token,
		IsActive:       true,
	}

	sm.sessions[wallet.ID] = session
	return session, nil
}

func (sm *SessionManager) GetSession(walletID string) (*WalletSession, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[walletID]
	if !exists || !session.IsActive || time.Now().After(session.ExpiresAt) {
		return nil, false
	}

	return session, true
}

func (sm *SessionManager) ValidateSession(walletID, token string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[walletID]
	if !exists || !session.IsActive {
		return false
	}

	if time.Now().After(session.ExpiresAt) {
		return false
	}

	return session.SessionToken == token
}

func (sm *SessionManager) ExtendSession(walletID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[walletID]
	if !exists || !session.IsActive {
		return fmt.Errorf("session not found or inactive")
	}

	now := time.Now()
	session.LastActivity = now
	session.ExpiresAt = now.Add(sm.config.DefaultTimeout)

	return nil
}

func (sm *SessionManager) RecordActivity(walletID, action, viewName string) {
	activity := SessionActivity{
		WalletID:  walletID,
		Action:    action,
		Timestamp: time.Now(),
		ViewName:  viewName,
	}

	select {
	case sm.activityChannel <- activity:
	default:
	}
}

func (sm *SessionManager) CloseSession(walletID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[walletID]
	if !exists {
		return fmt.Errorf("session not found")
	}

	session.IsActive = false
	sm.clearSensitiveData(session)
	delete(sm.sessions, walletID)

	return nil
}

func (sm *SessionManager) CloseAllSessions() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for walletID, session := range sm.sessions {
		session.IsActive = false
		sm.clearSensitiveData(session)
		delete(sm.sessions, walletID)
	}
}

func (sm *SessionManager) GetActiveSessions() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var activeWallets []string
	for walletID, session := range sm.sessions {
		if session.IsActive && time.Now().Before(session.ExpiresAt) {
			activeWallets = append(activeWallets, walletID)
		}
	}

	return activeWallets
}

func (sm *SessionManager) GetSessionStatus(walletID string) SessionStatus {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[walletID]
	if !exists {
		return SessionStatusInactive
	}

	if !session.IsActive {
		return SessionStatusInactive
	}

	if time.Now().After(session.ExpiresAt) {
		return SessionStatusExpired
	}

	timeRemaining := time.Until(session.ExpiresAt)
	if timeRemaining < 2*time.Minute {
		return SessionStatusExpiring
	}

	return SessionStatusActive
}

func (sm *SessionManager) GetTimeRemaining(walletID string) time.Duration {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[walletID]
	if !exists || !session.IsActive {
		return 0
	}

	remaining := time.Until(session.ExpiresAt)
	if remaining < 0 {
		return 0
	}

	return remaining
}

func (sm *SessionManager) UpdateConfig(config *SessionConfig) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.config = config
}

func (sm *SessionManager) Shutdown() {
	close(sm.stopCleanup)
	if sm.cleanupTicker != nil {
		sm.cleanupTicker.Stop()
	}
	sm.CloseAllSessions()
}

func (sm *SessionManager) generateSessionToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (sm *SessionManager) clearSensitiveData(session *WalletSession) {
	if session.UnlockedWallet != nil && session.UnlockedWallet.PrivateKey != nil {
		// Clear the private key's D value (the secret scalar)
		if session.UnlockedWallet.PrivateKey.D != nil {
			session.UnlockedWallet.PrivateKey.D.SetInt64(0)
		}
		session.UnlockedWallet.PrivateKey = nil

		// Clear mnemonic if present
		session.UnlockedWallet.Mnemonic = ""
	}

	// Clear session token
	session.SessionToken = ""
}

func (sm *SessionManager) startCleanupRoutine() {
	sm.cleanupTicker = time.NewTicker(sm.config.CleanupInterval)

	go func() {
		for {
			select {
			case <-sm.cleanupTicker.C:
				sm.cleanupExpiredSessions()
			case <-sm.stopCleanup:
				return
			}
		}
	}()
}

func (sm *SessionManager) cleanupExpiredSessions() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	for walletID, session := range sm.sessions {
		if now.After(session.ExpiresAt) {
			session.IsActive = false
			sm.clearSensitiveData(session)
			delete(sm.sessions, walletID)
		}
	}
}

func (sm *SessionManager) startActivityMonitor() {
	go func() {
		for activity := range sm.activityChannel {
			sm.handleActivity(activity)
		}
	}()
}

func (sm *SessionManager) handleActivity(activity SessionActivity) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[activity.WalletID]
	if !exists || !session.IsActive {
		return
	}

	now := time.Now()
	session.LastActivity = now
	session.ExpiresAt = now.Add(sm.config.DefaultTimeout)
}

type SessionStatus string

const (
	SessionStatusActive   SessionStatus = "active"
	SessionStatusExpiring SessionStatus = "expiring"
	SessionStatusExpired  SessionStatus = "expired"
	SessionStatusInactive SessionStatus = "inactive"
)
