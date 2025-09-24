package security

import (
	"fmt"
	"testing"
	"time"

	"rhystmorgan/veWallet/internal/models"
	"rhystmorgan/veWallet/internal/storage"
)

func TestSessionManager_CreateSession(t *testing.T) {
	storage, err := storage.NewStorage()
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	sm := NewSessionManager(storage)
	defer sm.Shutdown()

	wallet := &models.Wallet{
		ID:      "test-wallet-1",
		Name:    "Test Wallet",
		Address: "0x1234567890abcdef1234567890abcdef12345678",
	}

	session, err := sm.CreateSession(wallet)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	if session.WalletID != wallet.ID {
		t.Errorf("Expected wallet ID %s, got %s", wallet.ID, session.WalletID)
	}

	if session.SessionToken == "" {
		t.Error("Session token should not be empty")
	}

	if !session.IsActive {
		t.Error("Session should be active")
	}

	if time.Until(session.ExpiresAt) <= 0 {
		t.Error("Session should not be expired")
	}
}

func TestSessionManager_GetSession(t *testing.T) {
	storage, err := storage.NewStorage()
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	sm := NewSessionManager(storage)
	defer sm.Shutdown()

	wallet := &models.Wallet{
		ID:      "test-wallet-2",
		Name:    "Test Wallet 2",
		Address: "0x1234567890abcdef1234567890abcdef12345679",
	}

	// Test getting non-existent session
	_, exists := sm.GetSession(wallet.ID)
	if exists {
		t.Error("Session should not exist")
	}

	// Create session
	createdSession, err := sm.CreateSession(wallet)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Test getting existing session
	retrievedSession, exists := sm.GetSession(wallet.ID)
	if !exists {
		t.Error("Session should exist")
	}

	if retrievedSession.WalletID != createdSession.WalletID {
		t.Errorf("Expected wallet ID %s, got %s", createdSession.WalletID, retrievedSession.WalletID)
	}

	if retrievedSession.SessionToken != createdSession.SessionToken {
		t.Error("Session tokens should match")
	}
}

func TestSessionManager_ValidateSession(t *testing.T) {
	storage, err := storage.NewStorage()
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	sm := NewSessionManager(storage)
	defer sm.Shutdown()

	wallet := &models.Wallet{
		ID:      "test-wallet-3",
		Name:    "Test Wallet 3",
		Address: "0x1234567890abcdef1234567890abcdef12345680",
	}

	session, err := sm.CreateSession(wallet)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Test valid session
	if !sm.ValidateSession(wallet.ID, session.SessionToken) {
		t.Error("Session validation should succeed")
	}

	// Test invalid token
	if sm.ValidateSession(wallet.ID, "invalid-token") {
		t.Error("Session validation should fail with invalid token")
	}

	// Test invalid wallet ID
	if sm.ValidateSession("invalid-wallet", session.SessionToken) {
		t.Error("Session validation should fail with invalid wallet ID")
	}
}

func TestSessionManager_ExtendSession(t *testing.T) {
	storage, err := storage.NewStorage()
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	sm := NewSessionManager(storage)
	defer sm.Shutdown()

	wallet := &models.Wallet{
		ID:      "test-wallet-4",
		Name:    "Test Wallet 4",
		Address: "0x1234567890abcdef1234567890abcdef12345681",
	}

	session, err := sm.CreateSession(wallet)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	originalExpiry := session.ExpiresAt

	// Wait a moment to ensure time difference
	time.Sleep(10 * time.Millisecond)

	err = sm.ExtendSession(wallet.ID)
	if err != nil {
		t.Fatalf("Failed to extend session: %v", err)
	}

	updatedSession, exists := sm.GetSession(wallet.ID)
	if !exists {
		t.Fatal("Session should still exist after extension")
	}

	if !updatedSession.ExpiresAt.After(originalExpiry) {
		t.Error("Session expiry should be extended")
	}
}

func TestSessionManager_CloseSession(t *testing.T) {
	storage, err := storage.NewStorage()
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	sm := NewSessionManager(storage)
	defer sm.Shutdown()

	wallet := &models.Wallet{
		ID:      "test-wallet-5",
		Name:    "Test Wallet 5",
		Address: "0x1234567890abcdef1234567890abcdef12345682",
	}

	session, err := sm.CreateSession(wallet)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Verify session exists
	_, exists := sm.GetSession(wallet.ID)
	if !exists {
		t.Fatal("Session should exist before closing")
	}

	// Close session
	err = sm.CloseSession(wallet.ID)
	if err != nil {
		t.Fatalf("Failed to close session: %v", err)
	}

	// Verify session no longer exists
	_, exists = sm.GetSession(wallet.ID)
	if exists {
		t.Error("Session should not exist after closing")
	}

	// Verify session token is cleared
	if session.SessionToken != "" {
		t.Error("Session token should be cleared after closing")
	}
}

func TestSessionManager_SessionTimeout(t *testing.T) {
	storage, err := storage.NewStorage()
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	sm := NewSessionManager(storage)
	defer sm.Shutdown()

	// Set very short timeout for testing
	sm.UpdateConfig(&SessionConfig{
		DefaultTimeout:  100 * time.Millisecond,
		MaxSessions:     5,
		CleanupInterval: 50 * time.Millisecond,
		InactivityLimit: 100 * time.Millisecond,
		RequirePassword: true,
	})

	wallet := &models.Wallet{
		ID:      "test-wallet-6",
		Name:    "Test Wallet 6",
		Address: "0x1234567890abcdef1234567890abcdef12345683",
	}

	_, err = sm.CreateSession(wallet)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Wait for session to expire
	time.Sleep(200 * time.Millisecond)

	// Session should be expired
	_, exists := sm.GetSession(wallet.ID)
	if exists {
		t.Error("Session should be expired and cleaned up")
	}
}

func TestSessionManager_MaxSessions(t *testing.T) {
	storage, err := storage.NewStorage()
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	sm := NewSessionManager(storage)
	defer sm.Shutdown()

	// Set max sessions to 2
	sm.UpdateConfig(&SessionConfig{
		DefaultTimeout:  5 * time.Minute,
		MaxSessions:     2,
		CleanupInterval: 30 * time.Second,
		InactivityLimit: 5 * time.Minute,
		RequirePassword: true,
	})

	// Create first session
	wallet1 := &models.Wallet{
		ID:      "test-wallet-7",
		Name:    "Test Wallet 7",
		Address: "0x1234567890abcdef1234567890abcdef12345684",
	}

	_, err = sm.CreateSession(wallet1)
	if err != nil {
		t.Fatalf("Failed to create first session: %v", err)
	}

	// Create second session
	wallet2 := &models.Wallet{
		ID:      "test-wallet-8",
		Name:    "Test Wallet 8",
		Address: "0x1234567890abcdef1234567890abcdef12345685",
	}

	_, err = sm.CreateSession(wallet2)
	if err != nil {
		t.Fatalf("Failed to create second session: %v", err)
	}

	// Try to create third session (should fail)
	wallet3 := &models.Wallet{
		ID:      "test-wallet-9",
		Name:    "Test Wallet 9",
		Address: "0x1234567890abcdef1234567890abcdef12345686",
	}

	_, err = sm.CreateSession(wallet3)
	if err == nil {
		t.Error("Creating third session should fail due to max sessions limit")
	}
}

func TestSessionManager_GetActiveSessions(t *testing.T) {
	storage, err := storage.NewStorage()
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	sm := NewSessionManager(storage)
	defer sm.Shutdown()

	// Initially no active sessions
	activeSessions := sm.GetActiveSessions()
	if len(activeSessions) != 0 {
		t.Errorf("Expected 0 active sessions, got %d", len(activeSessions))
	}

	// Create sessions
	wallet1 := &models.Wallet{
		ID:      "test-wallet-10",
		Name:    "Test Wallet 10",
		Address: "0x1234567890abcdef1234567890abcdef12345687",
	}

	wallet2 := &models.Wallet{
		ID:      "test-wallet-11",
		Name:    "Test Wallet 11",
		Address: "0x1234567890abcdef1234567890abcdef12345688",
	}

	_, err = sm.CreateSession(wallet1)
	if err != nil {
		t.Fatalf("Failed to create first session: %v", err)
	}

	_, err = sm.CreateSession(wallet2)
	if err != nil {
		t.Fatalf("Failed to create second session: %v", err)
	}

	// Check active sessions
	activeSessions = sm.GetActiveSessions()
	if len(activeSessions) != 2 {
		t.Errorf("Expected 2 active sessions, got %d", len(activeSessions))
	}

	// Close one session
	err = sm.CloseSession(wallet1.ID)
	if err != nil {
		t.Fatalf("Failed to close session: %v", err)
	}

	// Check active sessions again
	activeSessions = sm.GetActiveSessions()
	if len(activeSessions) != 1 {
		t.Errorf("Expected 1 active session, got %d", len(activeSessions))
	}

	if activeSessions[0] != wallet2.ID {
		t.Errorf("Expected active session %s, got %s", wallet2.ID, activeSessions[0])
	}
}

func TestSessionManager_GetSessionStatus(t *testing.T) {
	storage, err := storage.NewStorage()
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	sm := NewSessionManager(storage)
	defer sm.Shutdown()

	wallet := &models.Wallet{
		ID:      "test-wallet-12",
		Name:    "Test Wallet 12",
		Address: "0x1234567890abcdef1234567890abcdef12345689",
	}

	// Test inactive status
	status := sm.GetSessionStatus(wallet.ID)
	if status != SessionStatusInactive {
		t.Errorf("Expected status %s, got %s", SessionStatusInactive, status)
	}

	// Create session
	_, err = sm.CreateSession(wallet)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Test active status
	status = sm.GetSessionStatus(wallet.ID)
	if status != SessionStatusActive {
		t.Errorf("Expected status %s, got %s", SessionStatusActive, status)
	}

	// Test expiring status
	sm.UpdateConfig(&SessionConfig{
		DefaultTimeout:  1 * time.Minute,
		MaxSessions:     5,
		CleanupInterval: 30 * time.Second,
		InactivityLimit: 1 * time.Minute,
		RequirePassword: true,
	})

	// Manually set expiry to trigger expiring status
	if session, exists := sm.GetSession(wallet.ID); exists {
		session.ExpiresAt = time.Now().Add(30 * time.Second)
	}

	status = sm.GetSessionStatus(wallet.ID)
	if status != SessionStatusExpiring {
		t.Errorf("Expected status %s, got %s", SessionStatusExpiring, status)
	}
}

func TestSessionManager_RecordActivity(t *testing.T) {
	storage, err := storage.NewStorage()
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	sm := NewSessionManager(storage)
	defer sm.Shutdown()

	wallet := &models.Wallet{
		ID:      "test-wallet-13",
		Name:    "Test Wallet 13",
		Address: "0x1234567890abcdef1234567890abcdef12345690",
	}

	session, err := sm.CreateSession(wallet)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	originalExpiry := session.ExpiresAt

	// Wait a moment
	time.Sleep(10 * time.Millisecond)

	// Record activity (should extend session)
	sm.RecordActivity(wallet.ID, "test_action", "test_view")

	// Give the activity monitor time to process
	time.Sleep(50 * time.Millisecond)

	updatedSession, exists := sm.GetSession(wallet.ID)
	if !exists {
		t.Fatal("Session should still exist after activity")
	}

	if !updatedSession.ExpiresAt.After(originalExpiry) {
		t.Error("Session should be extended after activity")
	}
}

func TestSessionManager_CloseAllSessions(t *testing.T) {
	storage, err := storage.NewStorage()
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	sm := NewSessionManager(storage)
	defer sm.Shutdown()

	// Create multiple sessions
	for i := 0; i < 3; i++ {
		wallet := &models.Wallet{
			ID:      fmt.Sprintf("test-wallet-%d", 14+i),
			Name:    fmt.Sprintf("Test Wallet %d", 14+i),
			Address: fmt.Sprintf("0x1234567890abcdef1234567890abcdef1234569%d", i),
		}

		_, err = sm.CreateSession(wallet)
		if err != nil {
			t.Fatalf("Failed to create session %d: %v", i, err)
		}
	}

	// Verify sessions exist
	activeSessions := sm.GetActiveSessions()
	if len(activeSessions) != 3 {
		t.Errorf("Expected 3 active sessions, got %d", len(activeSessions))
	}

	// Close all sessions
	sm.CloseAllSessions()

	// Verify no sessions exist
	activeSessions = sm.GetActiveSessions()
	if len(activeSessions) != 0 {
		t.Errorf("Expected 0 active sessions after closing all, got %d", len(activeSessions))
	}
}

func TestSessionManager_GetTimeRemaining(t *testing.T) {
	storage, err := storage.NewStorage()
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	sm := NewSessionManager(storage)
	defer sm.Shutdown()

	wallet := &models.Wallet{
		ID:      "test-wallet-17",
		Name:    "Test Wallet 17",
		Address: "0x1234567890abcdef1234567890abcdef12345693",
	}

	// Test non-existent session
	remaining := sm.GetTimeRemaining(wallet.ID)
	if remaining != 0 {
		t.Errorf("Expected 0 time remaining for non-existent session, got %v", remaining)
	}

	// Create session
	_, err = sm.CreateSession(wallet)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Test active session
	remaining = sm.GetTimeRemaining(wallet.ID)
	if remaining <= 0 {
		t.Error("Time remaining should be positive for active session")
	}

	if remaining > 5*time.Minute {
		t.Error("Time remaining should not exceed default timeout")
	}
}

// Benchmark tests
func BenchmarkSessionManager_CreateSession(b *testing.B) {
	storage, err := storage.NewStorage()
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}

	sm := NewSessionManager(storage)
	defer sm.Shutdown()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		wallet := &models.Wallet{
			ID:      fmt.Sprintf("bench-wallet-%d", i),
			Name:    fmt.Sprintf("Bench Wallet %d", i),
			Address: fmt.Sprintf("0x1234567890abcdef1234567890abcdef%08d", i),
		}

		_, err := sm.CreateSession(wallet)
		if err != nil {
			b.Fatalf("Failed to create session: %v", err)
		}

		// Clean up to avoid hitting max sessions
		sm.CloseSession(wallet.ID)
	}
}

func BenchmarkSessionManager_ValidateSession(b *testing.B) {
	storage, err := storage.NewStorage()
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}

	sm := NewSessionManager(storage)
	defer sm.Shutdown()

	wallet := &models.Wallet{
		ID:      "bench-wallet-validate",
		Name:    "Bench Wallet Validate",
		Address: "0x1234567890abcdef1234567890abcdef12345694",
	}

	session, err := sm.CreateSession(wallet)
	if err != nil {
		b.Fatalf("Failed to create session: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sm.ValidateSession(wallet.ID, session.SessionToken)
	}
}
