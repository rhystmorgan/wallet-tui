package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"rhystmorgan/veWallet/internal/views"
)

func main() {
	app, err := views.NewAppModel()
	if err != nil {
		fmt.Printf("Error initializing application: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(app, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running application: %v\n", err)
		os.Exit(1)
	}
}
