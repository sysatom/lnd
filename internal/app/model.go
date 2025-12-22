package app

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/yourusername/lnd/internal/collector"
	"github.com/yourusername/lnd/internal/ui"
	"github.com/yourusername/lnd/internal/ui/components"
)

const (
	TabInterfaces   = 0
	TabConnectivity = 1
	TabDashboard    = 2
	TabKernel       = 3
)

var tabs = []string{"Interfaces", "Connectivity", "Dashboard", "Kernel"}

type Model struct {
	ActiveTab int
	Width     int
	Height    int
	Ready     bool
	Viewport  viewport.Model

	// Data
	HostInfo     collector.HostInfo
	Connectivity collector.ConnectivityStats
	Traffic      collector.TrafficStats
	Kernel       collector.KernelStats

	// Collectors
	sysCollector     *collector.SystemCollector
	connCollector    *collector.ConnectivityCollector
	trafficCollector *collector.TrafficCollector
	kernelCollector  *collector.KernelCollector

	// Loading states
	LoadingSystem  bool
	LoadingConn    bool
	LoadingTraffic bool
	LoadingKernel  bool
}

func NewModel() Model {
	k, _ := collector.NewKernelCollector() // Handle error gracefully in Collect if nil
	return Model{
		sysCollector:     collector.NewSystemCollector(),
		connCollector:    collector.NewConnectivityCollector(),
		trafficCollector: collector.NewTrafficCollector(),
		kernelCollector:  k,
		LoadingSystem:    true,
		LoadingConn:      true,
		// Traffic and Kernel start as false, will be triggered by Init/Tick
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		fetchSystemInfo(m.sysCollector),
		fetchConnectivity(m.connCollector),
		// Start the tick loop
		tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
			return TickMsg(t)
		}),
	)
}

// Messages
type SystemInfoMsg collector.HostInfo
type ConnectivityMsg collector.ConnectivityStats
type TrafficMsg collector.TrafficStats
type KernelMsg collector.KernelStats
type TickMsg time.Time

// Commands
func fetchSystemInfo(c *collector.SystemCollector) tea.Cmd {
	return func() tea.Msg {
		info, err := c.Collect()
		if err != nil {
			info.Error = err
		}
		return SystemInfoMsg(info)
	}
}

func fetchConnectivity(c *collector.ConnectivityCollector) tea.Cmd {
	return func() tea.Msg {
		stats, err := c.Collect()
		if err != nil {
			// Handle error in stats
		}
		return ConnectivityMsg(stats)
	}
}

func fetchTraffic(c *collector.TrafficCollector) tea.Cmd {
	return func() tea.Msg {
		stats, err := c.Collect()
		if err != nil {
			// Handle error
		}
		return TrafficMsg(stats)
	}
}

func fetchKernel(c *collector.KernelCollector) tea.Cmd {
	return func() tea.Msg {
		if c == nil {
			return KernelMsg{Error: fmt.Errorf("kernel collector not initialized")}
		}
		stats, err := c.Collect()
		if err != nil {
			stats.Error = err
		}
		return KernelMsg(stats)
	}
}

func tickTraffic() tea.Cmd {
	return tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// Removed duplicate tickKernel and tickTraffic usage in Init

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab", "right":
			m.ActiveTab = (m.ActiveTab + 1) % len(tabs)
		case "shift+tab", "left":
			m.ActiveTab = (m.ActiveTab - 1 + len(tabs)) % len(tabs)
		}

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		if !m.Ready {
			m.Viewport = viewport.New(msg.Width, msg.Height-5) // Reserve space for header/footer
			m.Ready = true
		} else {
			m.Viewport.Width = msg.Width
			m.Viewport.Height = msg.Height - 5
		}

	case SystemInfoMsg:
		m.HostInfo = collector.HostInfo(msg)
		m.LoadingSystem = false

	case ConnectivityMsg:
		m.Connectivity = collector.ConnectivityStats(msg)
		m.LoadingConn = false
		// Schedule next update
		cmds = append(cmds, tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
			return fetchConnectivity(m.connCollector)()
		}))

	case TrafficMsg:
		m.LoadingTraffic = false
		m.Traffic = collector.TrafficStats(msg)

	case KernelMsg:
		m.LoadingKernel = false
		m.Kernel = collector.KernelStats(msg)

	case TickMsg:
		// Trigger updates if not already loading
		if !m.LoadingTraffic {
			m.LoadingTraffic = true
			cmds = append(cmds, fetchTraffic(m.trafficCollector))
		}
		if !m.LoadingKernel {
			m.LoadingKernel = true
			cmds = append(cmds, fetchKernel(m.kernelCollector))
		}

		// Schedule next tick
		cmds = append(cmds, tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
			return TickMsg(t)
		}))
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if m.Width < 60 {
		return "Terminal too small, please resize."
	}
	if !m.Ready {
		return "Initializing..."
	}

	// Header
	header := components.Header("LND", "v0.1.0")

	// Tabs
	var tabViews []string
	for i, t := range tabs {
		style := ui.TabStyle
		if i == m.ActiveTab {
			style = ui.ActiveTabStyle
		}
		tabViews = append(tabViews, style.Render(t))
	}
	tabsRow := lipgloss.JoinHorizontal(lipgloss.Top, tabViews...)

	// Content
	var content string
	switch m.ActiveTab {
	case TabInterfaces:
		content = m.renderInterfaces()
	case TabConnectivity:
		content = m.renderConnectivity()
	case TabDashboard:
		content = m.renderDashboard()
	case TabKernel:
		content = m.renderKernel()
	}

	// Footer
	footer := components.Footer("Press 'q' to quit, 'tab' to switch views")

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		tabsRow,
		ui.BoxStyle.Width(m.Width-2).Height(m.Height-5).Render(content),
		footer,
	)
}

// Render Helpers
func (m Model) renderInterfaces() string {
	if m.LoadingSystem {
		return "Loading System Info..."
	}
	info := m.HostInfo

	s := fmt.Sprintf("Hostname: %s\nKernel: %s\nArch: %s\nUptime: %s\nLoad: %.2f\n",
		info.Hostname, info.KernelVersion, info.Arch, info.Uptime, info.LoadAvg)

	s += fmt.Sprintf("\nLimits:\n  Max Open Files: %d\n  File Max: %d\n", info.MaxOpenFiles, info.FileMax)

	s += "\nSysctl:\n"
	for k, v := range info.SysctlParams {
		s += fmt.Sprintf("  %s: %s\n", k, v)
	}

	s += "\nInterfaces:\n"
	for _, iface := range info.Interfaces {
		s += fmt.Sprintf("  %s: %s (MTU: %d)\n", iface.Name, iface.IP, iface.MTU)
		if iface.Driver != "" {
			s += fmt.Sprintf("    Driver: %s\n", iface.Driver)
		}
	}
	return s
}

func (m Model) renderConnectivity() string {
	if m.LoadingConn {
		return "Probing Connectivity..."
	}
	s := "Ping Targets:\n"
	for target, res := range m.Connectivity.Targets {
		status := "OK"
		style := ui.SubtitleStyle
		if res.PacketLoss > 0 || res.Error != nil {
			status = "FAIL"
			style = ui.ErrorStyle
		}

		rtt := fmt.Sprintf("%.2fms", float64(res.AvgRtt.Microseconds())/1000.0)
		if res.Error != nil {
			rtt = "N/A"
		}

		s += fmt.Sprintf("  %s: %s (Loss: %.0f%%, RTT: %s)\n",
			target, style.Render(status), res.PacketLoss, rtt)
	}

	s += "\nDNS Performance:\n"
	dns := m.Connectivity.DNS
	s += fmt.Sprintf("  Local Resolver: %s\n", dns.LocalResolverTime)
	s += fmt.Sprintf("  Public (1.1.1.1): %s\n", dns.PublicResolverTime)

	return s
}

func (m Model) renderDashboard() string {
	s := "Traffic (Last 1s):\n"
	for name, t := range m.Traffic.Interfaces {
		// Only show active interfaces
		if t.RxRate == 0 && t.TxRate == 0 && t.RxBytes == 0 {
			continue
		}
		s += fmt.Sprintf("  %s:\n", ui.SubtitleStyle.Render(name))
		s += fmt.Sprintf("    RX: %.2f KB/s  TX: %.2f KB/s\n", t.RxRate/1024, t.TxRate/1024)
		s += fmt.Sprintf("    Drops: %d  Errors: %d\n", t.Drop, t.Errors)
	}
	return s
}

func (m Model) renderKernel() string {
	k := m.Kernel
	if k.Error != nil {
		return ui.ErrorStyle.Render(fmt.Sprintf("Error: %v", k.Error))
	}

	s := "TCP Health:\n"
	retransStyle := ui.SubtitleStyle
	if k.TCPRetransRate > 1.0 {
		retransStyle = ui.WarningStyle
	}
	s += fmt.Sprintf("  Retransmission Rate: %s\n", retransStyle.Render(fmt.Sprintf("%.2f%%", k.TCPRetransRate)))

	s += "\nTCP States:\n"
	s += fmt.Sprintf("  ESTABLISHED: %d\n", k.TCPEstablished)
	s += fmt.Sprintf("  TIME_WAIT:   %d\n", k.TCPTimeWait)
	s += fmt.Sprintf("  CLOSE_WAIT:  %d\n", k.TCPCloseWait)

	s += "\nUDP Issues:\n"
	s += fmt.Sprintf("  RcvbufErrors: %d\n", k.UDPRcvbufErrors)

	return s
}
