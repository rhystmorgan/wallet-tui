package security

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"rhystmorgan/veWallet/internal/storage"
)

type SessionStore struct {
	storage       *storage.Storage
	encryptionKey []byte
	sessionFile   string
}

type PersistedSession struct {
	WalletID      string    `json:"wallet_id"`
	SessionToken  string    `json:"session_token"`
	CreatedAt     time.Time `json:"created_at"`
	ExpiresAt     time.Time `json:"expires_at"`
	EncryptedData []byte    `json:"encrypted_data"`
	Checksum      string    `json:"checksum"`
}

type SessionData struct {
	WalletID     string    `json:"wallet_id"`
	LastActivity time.Time `json:"last_activity"`
	IsActive     bool      `json:"is_active"`
}

func NewSessionStore(storage *storage.Storage, encryptionKey []byte) *SessionStore {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Sprintf("failed to get home directory: %v", err))
	}

	sessionFile := filepath.Join(homeDir, ".veterm", "sessions.enc")

	return &SessionStore{
		storage:       storage,
		encryptionKey: encryptionKey,
		sessionFile:   sessionFile,
	}
}

func (ss *SessionStore) SaveSessions(sessions map[string]*WalletSession) error {
	var persistedSessions []PersistedSession

	for walletID, session := range sessions {
		if !session.IsActive || time.Now().After(session.ExpiresAt) {
			continue
		}

		sessionData := SessionData{
			WalletID:     session.WalletID,
			LastActivity: session.LastActivity,
			IsActive:     session.IsActive,
		}

		dataBytes, err := json.Marshal(sessionData)
		if err != nil {
			return fmt.Errorf("failed to marshal session data for wallet %s: %w", walletID, err)
		}

		encryptedData, err := ss.encryptData(dataBytes)
		if err != nil {
			return fmt.Errorf("failed to encrypt session data for wallet %s: %w", walletID, err)
		}

		checksum := ss.calculateChecksum(encryptedData)

		persistedSession := PersistedSession{
			WalletID:      session.WalletID,
			SessionToken:  session.SessionToken,
			CreatedAt:     session.CreatedAt,
			ExpiresAt:     session.ExpiresAt,
			EncryptedData: encryptedData,
			Checksum:      checksum,
		}

		persistedSessions = append(persistedSessions, persistedSession)
	}

	return ss.writeSessionsToFile(persistedSessions)
}

func (ss *SessionStore) LoadSessions() (map[string]*WalletSession, error) {
	persistedSessions, err := ss.readSessionsFromFile()
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]*WalletSession), nil
		}
		return nil, fmt.Errorf("failed to read sessions from file: %w", err)
	}

	sessions := make(map[string]*WalletSession)
	now := time.Now()

	for _, persistedSession := range persistedSessions {
		if now.After(persistedSession.ExpiresAt) {
			continue
		}

		if !ss.ValidateSessionIntegrity(&persistedSession) {
			continue
		}

		sessionData, err := ss.decryptSessionData(persistedSession.EncryptedData)
		if err != nil {
			continue
		}

		session := &WalletSession{
			WalletID:       persistedSession.WalletID,
			UnlockedWallet: nil,
			CreatedAt:      persistedSession.CreatedAt,
			LastActivity:   sessionData.LastActivity,
			ExpiresAt:      persistedSession.ExpiresAt,
			SessionToken:   persistedSession.SessionToken,
			IsActive:       sessionData.IsActive,
		}

		sessions[persistedSession.WalletID] = session
	}

	return sessions, nil
}

func (ss *SessionStore) ValidateSessionIntegrity(session *PersistedSession) bool {
	expectedChecksum := ss.calculateChecksum(session.EncryptedData)
	return expectedChecksum == session.Checksum
}

func (ss *SessionStore) CleanupExpiredSessions() error {
	persistedSessions, err := ss.readSessionsFromFile()
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read sessions for cleanup: %w", err)
	}

	var validSessions []PersistedSession
	now := time.Now()

	for _, session := range persistedSessions {
		if now.Before(session.ExpiresAt) && ss.ValidateSessionIntegrity(&session) {
			validSessions = append(validSessions, session)
		}
	}

	return ss.writeSessionsToFile(validSessions)
}

func (ss *SessionStore) SecureDelete() error {
	if _, err := os.Stat(ss.sessionFile); os.IsNotExist(err) {
		return nil
	}

	file, err := os.OpenFile(ss.sessionFile, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("failed to open session file for secure deletion: %w", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	zeros := make([]byte, fileInfo.Size())
	if _, err := file.WriteAt(zeros, 0); err != nil {
		return fmt.Errorf("failed to overwrite file: %w", err)
	}

	if err := file.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}

	return os.Remove(ss.sessionFile)
}

func (ss *SessionStore) GetSessionCount() (int, error) {
	persistedSessions, err := ss.readSessionsFromFile()
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to read sessions: %w", err)
	}

	validCount := 0
	now := time.Now()

	for _, session := range persistedSessions {
		if now.Before(session.ExpiresAt) && ss.ValidateSessionIntegrity(&session) {
			validCount++
		}
	}

	return validCount, nil
}

func (ss *SessionStore) BackupSessions() error {
	backupFile := ss.sessionFile + ".backup"

	sourceData, err := os.ReadFile(ss.sessionFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read source file: %w", err)
	}

	return os.WriteFile(backupFile, sourceData, 0600)
}

func (ss *SessionStore) RestoreFromBackup() error {
	backupFile := ss.sessionFile + ".backup"

	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		return fmt.Errorf("backup file does not exist")
	}

	backupData, err := os.ReadFile(backupFile)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}

	return os.WriteFile(ss.sessionFile, backupData, 0600)
}

func (ss *SessionStore) encryptData(data []byte) ([]byte, error) {
	encryptedData, err := storage.Encrypt(data, hex.EncodeToString(ss.encryptionKey))
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt data: %w", err)
	}

	// Convert EncryptedData struct to bytes for storage
	encBytes, err := json.Marshal(encryptedData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal encrypted data: %w", err)
	}

	return encBytes, nil
}

func (ss *SessionStore) decryptSessionData(encryptedData []byte) (*SessionData, error) {
	// Unmarshal the EncryptedData struct
	var encData storage.EncryptedData
	if err := json.Unmarshal(encryptedData, &encData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal encrypted data: %w", err)
	}

	decryptedData, err := storage.Decrypt(&encData, hex.EncodeToString(ss.encryptionKey))
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt session data: %w", err)
	}

	var sessionData SessionData
	if err := json.Unmarshal(decryptedData, &sessionData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session data: %w", err)
	}

	return &sessionData, nil
}

func (ss *SessionStore) calculateChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func (ss *SessionStore) writeSessionsToFile(sessions []PersistedSession) error {
	if err := os.MkdirAll(filepath.Dir(ss.sessionFile), 0700); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	data, err := json.Marshal(sessions)
	if err != nil {
		return fmt.Errorf("failed to marshal sessions: %w", err)
	}

	tempFile := ss.sessionFile + ".tmp"
	if err := os.WriteFile(tempFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write temporary session file: %w", err)
	}

	if err := os.Rename(tempFile, ss.sessionFile); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to rename session file: %w", err)
	}

	return nil
}

func (ss *SessionStore) readSessionsFromFile() ([]PersistedSession, error) {
	data, err := os.ReadFile(ss.sessionFile)
	if err != nil {
		return nil, err
	}

	var sessions []PersistedSession
	if err := json.Unmarshal(data, &sessions); err != nil {
		return nil, fmt.Errorf("failed to unmarshal sessions: %w", err)
	}

	return sessions, nil
}

type SessionRecoveryManager struct {
	sessionManager *SessionManager
	sessionStore   *SessionStore
	backupSessions map[string]*WalletSession
	recoveryLog    []RecoveryEvent
	mu             sync.RWMutex
}

type RecoveryEvent struct {
	Timestamp time.Time `json:"timestamp"`
	EventType string    `json:"event_type"`
	WalletID  string    `json:"wallet_id"`
	Action    string    `json:"action"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
}

func NewSessionRecoveryManager(sessionManager *SessionManager, sessionStore *SessionStore) *SessionRecoveryManager {
	return &SessionRecoveryManager{
		sessionManager: sessionManager,
		sessionStore:   sessionStore,
		backupSessions: make(map[string]*WalletSession),
		recoveryLog:    make([]RecoveryEvent, 0),
	}
}

func (srm *SessionRecoveryManager) RecoverFromCorruption() error {
	srm.mu.Lock()
	defer srm.mu.Unlock()

	event := RecoveryEvent{
		Timestamp: time.Now(),
		EventType: "corruption_recovery",
		Action:    "attempting_backup_restore",
	}

	err := srm.sessionStore.RestoreFromBackup()
	if err != nil {
		event.Success = false
		event.Error = err.Error()
		srm.recoveryLog = append(srm.recoveryLog, event)
		return fmt.Errorf("failed to restore from backup: %w", err)
	}

	sessions, err := srm.sessionStore.LoadSessions()
	if err != nil {
		event.Success = false
		event.Error = err.Error()
		srm.recoveryLog = append(srm.recoveryLog, event)
		return fmt.Errorf("failed to load restored sessions: %w", err)
	}

	for walletID, session := range sessions {
		srm.sessionManager.sessions[walletID] = session
	}

	event.Success = true
	srm.recoveryLog = append(srm.recoveryLog, event)
	return nil
}

func (srm *SessionRecoveryManager) HandleCrashRecovery() error {
	srm.mu.Lock()
	defer srm.mu.Unlock()

	event := RecoveryEvent{
		Timestamp: time.Now(),
		EventType: "crash_recovery",
		Action:    "loading_persisted_sessions",
	}

	sessions, err := srm.sessionStore.LoadSessions()
	if err != nil {
		event.Success = false
		event.Error = err.Error()
		srm.recoveryLog = append(srm.recoveryLog, event)
		return fmt.Errorf("failed to load sessions after crash: %w", err)
	}

	validSessions := 0
	for walletID, session := range sessions {
		if session.IsActive && time.Now().Before(session.ExpiresAt) {
			srm.sessionManager.sessions[walletID] = session
			validSessions++
		}
	}

	event.Success = true
	event.Action = fmt.Sprintf("recovered_%d_sessions", validSessions)
	srm.recoveryLog = append(srm.recoveryLog, event)

	return nil
}

func (srm *SessionRecoveryManager) CreateSessionBackup() error {
	srm.mu.Lock()
	defer srm.mu.Unlock()

	for walletID, session := range srm.sessionManager.sessions {
		if session.IsActive {
			sessionCopy := *session
			sessionCopy.UnlockedWallet = nil
			srm.backupSessions[walletID] = &sessionCopy
		}
	}

	return srm.sessionStore.BackupSessions()
}

func (srm *SessionRecoveryManager) GetRecoveryLog() []RecoveryEvent {
	srm.mu.RLock()
	defer srm.mu.RUnlock()

	logCopy := make([]RecoveryEvent, len(srm.recoveryLog))
	copy(logCopy, srm.recoveryLog)
	return logCopy
}

func (srm *SessionRecoveryManager) ClearRecoveryLog() {
	srm.mu.Lock()
	defer srm.mu.Unlock()

	srm.recoveryLog = srm.recoveryLog[:0]
}
