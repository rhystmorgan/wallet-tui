package utils

import (
	"time"
)

// Spinner represents a loading spinner animation
type Spinner struct {
	frames []string
	index  int
	last   time.Time
	speed  time.Duration
}

// NewSpinner creates a new spinner with default settings
func NewSpinner() *Spinner {
	return &Spinner{
		frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		speed:  100 * time.Millisecond,
	}
}

// View returns the current frame of the spinner
func (s *Spinner) View() string {
	now := time.Now()
	if now.Sub(s.last) >= s.speed {
		s.index = (s.index + 1) % len(s.frames)
		s.last = now
	}
	return s.frames[s.index]
}
