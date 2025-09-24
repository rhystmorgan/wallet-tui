package security

import (
	"sync"
	"time"
)

type ActivityMonitor struct {
	sessionManager      *SessionManager
	activityLog         []SessionActivity
	suspiciousThreshold int
	logRetention        time.Duration
	mu                  sync.RWMutex
}

type ActivityPattern struct {
	WalletID        string
	ViewTransitions []string
	ActionFrequency map[string]int
	TimePattern     []time.Time
	RiskScore       int
}

func NewActivityMonitor(sessionManager *SessionManager) *ActivityMonitor {
	am := &ActivityMonitor{
		sessionManager:      sessionManager,
		activityLog:         make([]SessionActivity, 0),
		suspiciousThreshold: 10,
		logRetention:        24 * time.Hour,
	}

	go am.startCleanupRoutine()
	return am
}

func (am *ActivityMonitor) RecordActivity(walletID, action, view string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	activity := SessionActivity{
		WalletID:  walletID,
		Action:    action,
		Timestamp: time.Now(),
		ViewName:  view,
	}

	am.activityLog = append(am.activityLog, activity)

	if am.shouldExtendSession(walletID, action) {
		am.ExtendSessionOnActivity(walletID)
	}

	if am.DetectSuspiciousActivity(walletID) {
		am.handleSuspiciousActivity(walletID)
	}
}

func (am *ActivityMonitor) ExtendSessionOnActivity(walletID string) {
	if err := am.sessionManager.ExtendSession(walletID); err != nil {
		return
	}
}

func (am *ActivityMonitor) DetectSuspiciousActivity(walletID string) bool {
	am.mu.RLock()
	defer am.mu.RUnlock()

	pattern := am.analyzeActivityPattern(walletID)
	return pattern.RiskScore > am.suspiciousThreshold
}

func (am *ActivityMonitor) GetActivitySummary(walletID string) *ActivityPattern {
	am.mu.RLock()
	defer am.mu.RUnlock()

	return am.analyzeActivityPattern(walletID)
}

func (am *ActivityMonitor) CleanupOldLogs() {
	am.mu.Lock()
	defer am.mu.Unlock()

	cutoff := time.Now().Add(-am.logRetention)
	var filteredLog []SessionActivity

	for _, activity := range am.activityLog {
		if activity.Timestamp.After(cutoff) {
			filteredLog = append(filteredLog, activity)
		}
	}

	am.activityLog = filteredLog
}

func (am *ActivityMonitor) GetRecentActivity(walletID string, duration time.Duration) []SessionActivity {
	am.mu.RLock()
	defer am.mu.RUnlock()

	cutoff := time.Now().Add(-duration)
	var recentActivity []SessionActivity

	for _, activity := range am.activityLog {
		if activity.WalletID == walletID && activity.Timestamp.After(cutoff) {
			recentActivity = append(recentActivity, activity)
		}
	}

	return recentActivity
}

func (am *ActivityMonitor) GetActivityCount(walletID string, duration time.Duration) int {
	return len(am.GetRecentActivity(walletID, duration))
}

func (am *ActivityMonitor) GetMostActiveWallet(duration time.Duration) string {
	am.mu.RLock()
	defer am.mu.RUnlock()

	cutoff := time.Now().Add(-duration)
	activityCount := make(map[string]int)

	for _, activity := range am.activityLog {
		if activity.Timestamp.After(cutoff) {
			activityCount[activity.WalletID]++
		}
	}

	var mostActive string
	var maxCount int

	for walletID, count := range activityCount {
		if count > maxCount {
			maxCount = count
			mostActive = walletID
		}
	}

	return mostActive
}

func (am *ActivityMonitor) analyzeActivityPattern(walletID string) *ActivityPattern {
	recentActivity := am.GetRecentActivity(walletID, 1*time.Hour)

	pattern := &ActivityPattern{
		WalletID:        walletID,
		ViewTransitions: make([]string, 0),
		ActionFrequency: make(map[string]int),
		TimePattern:     make([]time.Time, 0),
		RiskScore:       0,
	}

	for _, activity := range recentActivity {
		pattern.ViewTransitions = append(pattern.ViewTransitions, activity.ViewName)
		pattern.ActionFrequency[activity.Action]++
		pattern.TimePattern = append(pattern.TimePattern, activity.Timestamp)
	}

	pattern.RiskScore = am.calculateRiskScore(pattern)
	return pattern
}

func (am *ActivityMonitor) calculateRiskScore(pattern *ActivityPattern) int {
	riskScore := 0

	if len(pattern.ViewTransitions) > 50 {
		riskScore += 5
	}

	rapidTransitions := 0
	for i := 1; i < len(pattern.TimePattern); i++ {
		if pattern.TimePattern[i].Sub(pattern.TimePattern[i-1]) < 500*time.Millisecond {
			rapidTransitions++
		}
	}

	if rapidTransitions > 10 {
		riskScore += 3
	}

	if pattern.ActionFrequency["password_attempt"] > 5 {
		riskScore += 8
	}

	if pattern.ActionFrequency["failed_transaction"] > 3 {
		riskScore += 4
	}

	repeatedViews := 0
	viewCount := make(map[string]int)
	for _, view := range pattern.ViewTransitions {
		viewCount[view]++
		if viewCount[view] > 10 {
			repeatedViews++
		}
	}

	if repeatedViews > 2 {
		riskScore += 2
	}

	return riskScore
}

func (am *ActivityMonitor) shouldExtendSession(walletID, action string) bool {
	extendableActions := map[string]bool{
		"view_balance":       true,
		"send_transaction":   true,
		"view_history":       true,
		"manage_contacts":    true,
		"navigate":           false,
		"password_attempt":   false,
		"failed_transaction": false,
	}

	extend, exists := extendableActions[action]
	return exists && extend
}

func (am *ActivityMonitor) handleSuspiciousActivity(walletID string) {
	am.sessionManager.RecordActivity(walletID, "suspicious_activity_detected", "security_monitor")

	session, exists := am.sessionManager.GetSession(walletID)
	if exists && session.IsActive {
		am.sessionManager.CloseSession(walletID)
	}
}

func (am *ActivityMonitor) startCleanupRoutine() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		am.CleanupOldLogs()
	}
}

func (am *ActivityMonitor) SetSuspiciousThreshold(threshold int) {
	am.mu.Lock()
	defer am.mu.Unlock()

	am.suspiciousThreshold = threshold
}

func (am *ActivityMonitor) SetLogRetention(duration time.Duration) {
	am.mu.Lock()
	defer am.mu.Unlock()

	am.logRetention = duration
}

func (am *ActivityMonitor) GetActivityStats() ActivityStats {
	am.mu.RLock()
	defer am.mu.RUnlock()

	stats := ActivityStats{
		TotalActivities:   len(am.activityLog),
		UniqueWallets:     make(map[string]bool),
		MostCommonAction:  "",
		MostCommonView:    "",
		SuspiciousWallets: 0,
	}

	actionCount := make(map[string]int)
	viewCount := make(map[string]int)

	for _, activity := range am.activityLog {
		stats.UniqueWallets[activity.WalletID] = true
		actionCount[activity.Action]++
		viewCount[activity.ViewName]++
	}

	stats.UniqueWalletCount = len(stats.UniqueWallets)

	var maxActionCount, maxViewCount int
	for action, count := range actionCount {
		if count > maxActionCount {
			maxActionCount = count
			stats.MostCommonAction = action
		}
	}

	for view, count := range viewCount {
		if count > maxViewCount {
			maxViewCount = count
			stats.MostCommonView = view
		}
	}

	for walletID := range stats.UniqueWallets {
		if am.DetectSuspiciousActivity(walletID) {
			stats.SuspiciousWallets++
		}
	}

	return stats
}

type ActivityStats struct {
	TotalActivities   int
	UniqueWallets     map[string]bool
	UniqueWalletCount int
	MostCommonAction  string
	MostCommonView    string
	SuspiciousWallets int
}
