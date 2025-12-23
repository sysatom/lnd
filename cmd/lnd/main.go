package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sysatom/lnd/internal/app"
	"github.com/sysatom/lnd/internal/config"
)

func main() {
	configPath := flag.String("config", "", "Path to configuration file (default: ~/.lnd.yaml)")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Root Check
	if os.Geteuid() != 0 {
		fmt.Println("Warning: LND is running without Root privileges.")
		fmt.Println("Some features (Ping, Kernel Stats, Ethtool) may be limited or unavailable.")
		fmt.Println("Press Enter to continue or Ctrl+C to abort...")
		fmt.Scanln()
	}

	p := tea.NewProgram(app.NewModel(cfg), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
