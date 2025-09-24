package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"sync"
	"time"
)

type SecurityManager struct {
	sessionManager *SessionManager
	attemptTracker *AttemptTracker
	encryptionKey  []byte
	secureRandom   io.Reader
}

type AttemptTracker struct {
	attempts        map[string]*AttemptRecord
	lockoutDuration time.Duration
	maxAttempts     int
	mu              sync.RWMutex
}

type AttemptRecord struct {
	Count       int
	LastAttempt time.Time
	LockedUntil time.Time
	IPAddress   string
}

func NewSecurityManager(sessionManager *SessionManager) *SecurityManager {
	attemptTracker := &AttemptTracker{
		attempts:        make(map[string]*AttemptRecord),
		lockoutDuration: 15 * time.Minute,
		maxAttempts:     3,
	}

	encryptionKey := make([]byte, 32)
	if _, err := rand.Read(encryptionKey); err != nil {
		panic(fmt.Sprintf("failed to generate encryption key: %v", err))
	}

	return &SecurityManager{
		sessionManager: sessionManager,
		attemptTracker: attemptTracker,
		encryptionKey:  encryptionKey,
		secureRandom:   rand.Reader,
	}
}

func (sm *SecurityManager) GenerateSessionToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := sm.secureRandom.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate secure random bytes: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

func (sm *SecurityManager) EncryptSessionData(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(sm.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := sm.secureRandom.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

func (sm *SecurityManager) DecryptSessionData(encryptedData []byte) ([]byte, error) {
	block, err := aes.NewCipher(sm.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(encryptedData) < nonceSize {
		return nil, fmt.Errorf("encrypted data too short")
	}

	nonce, ciphertext := encryptedData[:nonceSize], encryptedData[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

func (sm *SecurityManager) ValidateSessionToken(token string) bool {
	if len(token) != 64 {
		return false
	}

	_, err := hex.DecodeString(token)
	return err == nil
}

func (sm *SecurityManager) IsAccountLocked(walletID string) bool {
	sm.attemptTracker.mu.RLock()
	defer sm.attemptTracker.mu.RUnlock()

	record, exists := sm.attemptTracker.attempts[walletID]
	if !exists {
		return false
	}

	return time.Now().Before(record.LockedUntil)
}

func (sm *SecurityManager) RecordFailedAttempt(walletID string) {
	sm.attemptTracker.mu.Lock()
	defer sm.attemptTracker.mu.Unlock()

	now := time.Now()
	record, exists := sm.attemptTracker.attempts[walletID]

	if !exists {
		record = &AttemptRecord{
			Count:       0,
			LastAttempt: now,
		}
		sm.attemptTracker.attempts[walletID] = record
	}

	record.Count++
	record.LastAttempt = now

	if record.Count >= sm.attemptTracker.maxAttempts {
		lockoutDuration := sm.calculateLockoutDuration(record.Count)
		record.LockedUntil = now.Add(lockoutDuration)
	}
}

func (sm *SecurityManager) RecordSuccessfulAttempt(walletID string) {
	sm.attemptTracker.mu.Lock()
	defer sm.attemptTracker.mu.Unlock()

	delete(sm.attemptTracker.attempts, walletID)
}

func (sm *SecurityManager) GetRemainingLockoutTime(walletID string) time.Duration {
	sm.attemptTracker.mu.RLock()
	defer sm.attemptTracker.mu.RUnlock()

	record, exists := sm.attemptTracker.attempts[walletID]
	if !exists {
		return 0
	}

	remaining := time.Until(record.LockedUntil)
	if remaining < 0 {
		return 0
	}

	return remaining
}

func (sm *SecurityManager) GetFailedAttemptCount(walletID string) int {
	sm.attemptTracker.mu.RLock()
	defer sm.attemptTracker.mu.RUnlock()

	record, exists := sm.attemptTracker.attempts[walletID]
	if !exists {
		return 0
	}

	return record.Count
}

func (sm *SecurityManager) CleanupExpiredLockouts() {
	sm.attemptTracker.mu.Lock()
	defer sm.attemptTracker.mu.Unlock()

	now := time.Now()
	for walletID, record := range sm.attemptTracker.attempts {
		if now.After(record.LockedUntil) && record.Count >= sm.attemptTracker.maxAttempts {
			delete(sm.attemptTracker.attempts, walletID)
		}
	}
}

func (sm *SecurityManager) RotateEncryptionKey() error {
	newKey := make([]byte, 32)
	if _, err := sm.secureRandom.Read(newKey); err != nil {
		return fmt.Errorf("failed to generate new encryption key: %w", err)
	}

	sm.encryptionKey = newKey
	return nil
}

func (sm *SecurityManager) SecureZeroMemory(data []byte) {
	for i := range data {
		data[i] = 0
	}
}

func (sm *SecurityManager) calculateLockoutDuration(attemptCount int) time.Duration {
	switch {
	case attemptCount <= 3:
		return 1 * time.Minute
	case attemptCount <= 5:
		return 5 * time.Minute
	case attemptCount <= 7:
		return 15 * time.Minute
	default:
		return 1 * time.Hour
	}
}

func (sm *SecurityManager) UpdateAttemptTrackerConfig(maxAttempts int, lockoutDuration time.Duration) {
	sm.attemptTracker.mu.Lock()
	defer sm.attemptTracker.mu.Unlock()

	sm.attemptTracker.maxAttempts = maxAttempts
	sm.attemptTracker.lockoutDuration = lockoutDuration
}

func (sm *SecurityManager) GetSecurityMetrics() SecurityMetrics {
	sm.attemptTracker.mu.RLock()
	defer sm.attemptTracker.mu.RUnlock()

	metrics := SecurityMetrics{
		TotalFailedAttempts: 0,
		ActiveLockouts:      0,
		WalletsWithAttempts: len(sm.attemptTracker.attempts),
	}

	now := time.Now()
	for _, record := range sm.attemptTracker.attempts {
		metrics.TotalFailedAttempts += record.Count
		if now.Before(record.LockedUntil) {
			metrics.ActiveLockouts++
		}
	}

	return metrics
}

type SecurityMetrics struct {
	TotalFailedAttempts int
	ActiveLockouts      int
	WalletsWithAttempts int
}

type SecurityLevel string

const (
	SecurityLow    SecurityLevel = "low"
	SecurityMedium SecurityLevel = "medium"
	SecurityHigh   SecurityLevel = "high"
	SecurityMax    SecurityLevel = "max"
)

func (sm *SecurityManager) ApplySecurityLevel(level SecurityLevel) {
	switch level {
	case SecurityLow:
		sm.sessionManager.UpdateConfig(&SessionConfig{
			DefaultTimeout:  30 * time.Minute,
			MaxSessions:     10,
			CleanupInterval: 60 * time.Second,
			InactivityLimit: 30 * time.Minute,
			RequirePassword: false,
		})
		sm.UpdateAttemptTrackerConfig(5, 1*time.Minute)
	case SecurityMedium:
		sm.sessionManager.UpdateConfig(&SessionConfig{
			DefaultTimeout:  15 * time.Minute,
			MaxSessions:     5,
			CleanupInterval: 30 * time.Second,
			InactivityLimit: 15 * time.Minute,
			RequirePassword: true,
		})
		sm.UpdateAttemptTrackerConfig(3, 5*time.Minute)
	case SecurityHigh:
		sm.sessionManager.UpdateConfig(&SessionConfig{
			DefaultTimeout:  5 * time.Minute,
			MaxSessions:     3,
			CleanupInterval: 15 * time.Second,
			InactivityLimit: 5 * time.Minute,
			RequirePassword: true,
		})
		sm.UpdateAttemptTrackerConfig(3, 15*time.Minute)
	case SecurityMax:
		sm.sessionManager.UpdateConfig(&SessionConfig{
			DefaultTimeout:  2 * time.Minute,
			MaxSessions:     1,
			CleanupInterval: 10 * time.Second,
			InactivityLimit: 2 * time.Minute,
			RequirePassword: true,
		})
		sm.UpdateAttemptTrackerConfig(1, 1*time.Hour)
	}
}
