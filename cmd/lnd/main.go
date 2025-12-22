package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/yourusername/lnd/internal/app"
)

func main() {
	// Root Check
	if os.Geteuid() != 0 {
		fmt.Println("Warning: LND is running without Root privileges.")
		fmt.Println("Some features (Ping, Kernel Stats, Ethtool) may be limited or unavailable.")
		fmt.Println("Press Enter to continue or Ctrl+C to abort...")
		fmt.Scanln()
	}

	p := tea.NewProgram(app.NewModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
