package app

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sysatom/lnd/internal/build"
	"github.com/sysatom/lnd/internal/collector"
	"github.com/sysatom/lnd/internal/config"
	"github.com/sysatom/lnd/internal/ui"
	"github.com/sysatom/lnd/internal/ui/components"
)

const (
	TabInterfaces   = 0
	TabConnectivity = 1
	TabDashboard    = 2
	TabKernel       = 3
	TabDNS          = 4
	TabAbout        = 5
)

var tabs = []string{"Interfaces", "Connectivity", "Dashboard", "Kernel", "DNS", "About"}

var dnsRecordTypes = []collector.DNSRecordType{
	"Auto", collector.RecordA, collector.RecordAAAA, collector.RecordCNAME, collector.RecordMX,
	collector.RecordTXT, collector.RecordNS, collector.RecordPTR, collector.RecordSRV, collector.RecordCAA,
}

var dnsProtocols = []collector.DNSProtocol{
	collector.ProtoUDP, collector.ProtoTCP, collector.ProtoDoT, collector.ProtoDoH,
}

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
	NatInfo      []collector.NatInfo
	DNSResult    *collector.DNSLookupResult
	DNSPing      *collector.PingResult

	// Collectors
	sysCollector     *collector.SystemCollector
	connCollector    *collector.ConnectivityCollector
	trafficCollector *collector.TrafficCollector
	kernelCollector  *collector.KernelCollector
	natCollector     *collector.NatCollector
	dnsCollector     *collector.DNSCollector

	// DNS UI State
	DNSServers         []collector.DNSServer
	DNSInput           textinput.Model
	DNSServerInput     textinput.Model
	DNSFocus           int // 0: Domain, 1: Server
	SelectedDNSServer  int
	SelectedRecordType int
	SelectedProtocol   int // 0: UDP, 1: TCP, 2: DoT, 3: DoH

	// Loading states
	LoadingSystem  bool
	LoadingConn    bool
	LoadingTraffic bool
	LoadingKernel  bool
	LoadingNat     bool
	LoadingDNS     bool
	LoadingDNSPing bool
}

func NewModel(cfg *config.Config) Model {
	k, _ := collector.NewKernelCollector() // Handle error gracefully in Collect if nil

	var stunTargets []collector.StunTarget
	for _, s := range cfg.StunServers {
		host, portStr, err := net.SplitHostPort(s)
		if err != nil {
			// If split fails, assume it's just a host and use default port
			host = s
			portStr = "3478"
		}
		port, err := strconv.Atoi(portStr)
		if err != nil {
			port = 3478
		}

		stunTargets = append(stunTargets, collector.StunTarget{
			Host: host,
			Port: port,
		})
	}

	// Initialize DNS Servers
	// Start with defaults (excluding Custom)
	var dnsServers []collector.DNSServer
	defaults := collector.DefaultDNSServers
	// Find "Custom" index
	customIdx := -1
	for i, s := range defaults {
		if s.Name == "Custom" {
			customIdx = i
			break
		}
	}

	if customIdx != -1 {
		dnsServers = append(dnsServers, defaults[:customIdx]...)
	} else {
		dnsServers = append(dnsServers, defaults...)
	}

	// Add Configured Servers
	for _, s := range cfg.DNSServers {
		dnsServers = append(dnsServers, collector.DNSServer{
			Name:    s.Name,
			Address: s.Address,
			Proto:   collector.DNSProtocol(s.Proto),
		})
	}

	// Add Custom at the end
	if customIdx != -1 {
		dnsServers = append(dnsServers, defaults[customIdx])
	}

	ti := textinput.New()
	ti.Placeholder = "Enter domain or IP..."
	ti.Focus()
	ti.CharLimit = 255
	ti.Width = 30

	si := textinput.New()
	si.Placeholder = "e.g. 1.1.1.1:853 or https://..."
	si.CharLimit = 255
	si.Width = 30

	m := Model{
		sysCollector:     collector.NewSystemCollector(),
		connCollector:    collector.NewConnectivityCollector(),
		trafficCollector: collector.NewTrafficCollector(),
		kernelCollector:  k,
		natCollector:     collector.NewNatCollector(stunTargets),
		dnsCollector:     collector.NewDNSCollector(),
		DNSServers:       dnsServers,
		DNSInput:         ti,
		DNSServerInput:   si,
		LoadingSystem:    true,
		LoadingConn:      true,
		LoadingNat:       true,
		// Traffic and Kernel start as false, will be triggered by Init/Tick
	}

	// Sync initial protocol
	if len(m.DNSServers) > 0 {
		proto := m.DNSServers[0].Proto
		for i, p := range dnsProtocols {
			if p == proto {
				m.SelectedProtocol = i
				break
			}
		}
	}

	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		fetchSystemInfo(m.sysCollector),
		fetchConnectivity(m.connCollector),
		fetchNatInfo(m.natCollector),
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
type NatMsg []collector.NatInfo
type DNSMsg collector.DNSLookupResult
type DNSPingMsg collector.PingResult
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

func fetchNatInfo(c *collector.NatCollector) tea.Cmd {
	return func() tea.Msg {
		info, err := c.Collect()
		if err != nil {
			// If Collect returns error, it might be a general error, but we changed Collect to return []NatInfo
			// and error is usually nil unless something catastrophic happens.
			// But let's handle it.
			return NatMsg([]collector.NatInfo{{Error: err}})
		}
		return NatMsg(info)
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

func fetchDNS(c *collector.DNSCollector, domain string, recordType collector.DNSRecordType, server collector.DNSServer) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		// Handle Auto type
		if recordType == "Auto" {
			// If IP, use PTR. If Domain, use A.
			if net.ParseIP(domain) != nil {
				recordType = collector.RecordPTR
			} else {
				recordType = collector.RecordA
			}
		}
		return DNSMsg(c.Lookup(ctx, domain, recordType, server))
	}
}

func fetchSinglePing(c *collector.ConnectivityCollector, target string) tea.Cmd {
	return func() tea.Msg {
		return DNSPingMsg(c.Ping(target))
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
		case "tab":
			m.ActiveTab = (m.ActiveTab + 1) % len(tabs)
			return m, nil
		case "shift+tab":
			m.ActiveTab = (m.ActiveTab - 1 + len(tabs)) % len(tabs)
			return m, nil
		}

		if m.ActiveTab == TabDNS {
			isCustom := m.DNSServers[m.SelectedDNSServer].Name == "Custom"

			switch msg.String() {
			case "enter":
				m.LoadingDNS = true
				m.DNSResult = nil // Clear previous result
				m.DNSPing = nil   // Clear previous ping
				server := m.DNSServers[m.SelectedDNSServer]
				if isCustom {
					server.Address = m.DNSServerInput.Value()
				}
				server.Proto = dnsProtocols[m.SelectedProtocol]
				cmds = append(cmds, fetchDNS(m.dnsCollector, m.DNSInput.Value(), dnsRecordTypes[m.SelectedRecordType], server))
				return m, tea.Batch(cmds...)

			case "down":
				m.SelectedDNSServer = (m.SelectedDNSServer + 1) % len(m.DNSServers)
				m.DNSFocus = 0
				m.DNSInput.Focus()
				m.DNSServerInput.Blur()
				// Sync protocol
				proto := m.DNSServers[m.SelectedDNSServer].Proto
				for i, p := range dnsProtocols {
					if p == proto {
						m.SelectedProtocol = i
						break
					}
				}

			case "up":
				m.SelectedDNSServer = (m.SelectedDNSServer - 1 + len(m.DNSServers)) % len(m.DNSServers)
				m.DNSFocus = 0
				m.DNSInput.Focus()
				m.DNSServerInput.Blur()
				// Sync protocol
				proto := m.DNSServers[m.SelectedDNSServer].Proto
				for i, p := range dnsProtocols {
					if p == proto {
						m.SelectedProtocol = i
						break
					}
				}

			case "ctrl+down":
				if isCustom {
					m.DNSFocus = 1
					m.DNSInput.Blur()
					m.DNSServerInput.Focus()
				}

			case "ctrl+up":
				m.DNSFocus = 0
				m.DNSInput.Focus()
				m.DNSServerInput.Blur()

			case "ctrl+t":
				m.SelectedRecordType = (m.SelectedRecordType + 1) % len(dnsRecordTypes)
			case "ctrl+p":
				m.SelectedProtocol = (m.SelectedProtocol + 1) % len(dnsProtocols)
			}
			var cmd tea.Cmd
			if m.DNSFocus == 0 {
				m.DNSInput, cmd = m.DNSInput.Update(msg)
			} else {
				m.DNSServerInput, cmd = m.DNSServerInput.Update(msg)
			}
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}

		switch msg.String() {
		case "right":
			m.ActiveTab = (m.ActiveTab + 1) % len(tabs)
		case "left":
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

	case NatMsg:
		m.NatInfo = []collector.NatInfo(msg)
		m.LoadingNat = false

	case TrafficMsg:
		m.LoadingTraffic = false
		m.Traffic = collector.TrafficStats(msg)

	case KernelMsg:
		m.LoadingKernel = false
		m.Kernel = collector.KernelStats(msg)

	case DNSMsg:
		m.LoadingDNS = false
		res := collector.DNSLookupResult(msg)
		m.DNSResult = &res

		// Trigger Ping if we have a valid result
		if res.Error == nil {
			target := ""
			// If input was IP, ping that IP
			if net.ParseIP(m.DNSInput.Value()) != nil {
				target = m.DNSInput.Value()
			} else if len(res.Records) > 0 {
				// If we got records, check if any are IPs (A/AAAA)
				// Records are strings like "google.com. 300 IN A 1.2.3.4"
				// We need to parse the IP from the record string
				for _, rec := range res.Records {
					parts := strings.Fields(rec)
					if len(parts) > 0 {
						last := parts[len(parts)-1]
						if net.ParseIP(last) != nil {
							target = last
							break
						}
					}
				}
			}

			if target != "" {
				m.LoadingDNSPing = true
				cmds = append(cmds, fetchSinglePing(m.connCollector, target))
			}
		}

	case DNSPingMsg:
		m.LoadingDNSPing = false
		res := collector.PingResult(msg)
		m.DNSPing = &res

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
	header := components.Header("LND", build.Version)

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
	case TabDNS:
		content = m.renderDNS()
	case TabAbout:
		content = m.renderAbout()
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

	s += "\nNAT Status:\n"
	if m.LoadingNat {
		s += "  Probing NAT Type...\n"
	} else {
		for _, info := range m.NatInfo {
			s += fmt.Sprintf("  Target: %s\n", info.Target)
			if info.Error != nil {
				s += fmt.Sprintf("    Error: %v\n", info.Error)
			} else {
				s += fmt.Sprintf("    Type: %s\n", info.NatType)
				s += fmt.Sprintf("    Public IP: %s\n", info.PublicIP)
				s += fmt.Sprintf("    Local IP: %s\n", info.LocalIP)
			}
			s += "\n"
		}
	}

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

func (m Model) renderAbout() string {
	s := ui.TitleStyle.Render("LND - Linux Network Diagnoser") + "\n\n"
	s += fmt.Sprintf("Version:   %s\n", build.Version)
	s += fmt.Sprintf("Commit:    %s\n", build.Commit)
	s += fmt.Sprintf("Date:      %s\n", build.Date)
	s += fmt.Sprintf("Built By:  %s\n", build.BuiltBy)
	s += "\n"
	s += "GitHub:    https://github.com/sysatom/lnd\n"
	s += "License:   MIT\n"
	s += "\n"
	s += "A TUI-based network diagnostic tool for Linux.\n"
	s += "Use 'tab' to switch between views.\n"
	return s
}

func (m Model) renderDNS() string {
	s := ui.TitleStyle.Render("DNS Lookup Tool") + "\n\n"

	// Input
	s += fmt.Sprintf("Domain/IP: %s\n", m.DNSInput.View())

	// Settings
	server := m.DNSServers[m.SelectedDNSServer]
	s += fmt.Sprintf("Server:    %s (Use Up/Down to change)\n", server.Name)

	if server.Name == "Custom" {
		s += fmt.Sprintf("  Address: %s (Ctrl+Down to edit)\n", m.DNSServerInput.View())
	}

	recordType := dnsRecordTypes[m.SelectedRecordType]
	s += fmt.Sprintf("Type:      %s (Use Ctrl+t to change)\n", recordType)

	proto := dnsProtocols[m.SelectedProtocol]
	s += fmt.Sprintf("Protocol:  %s (Use Ctrl+p to change)\n", proto)

	s += "\nPress Enter to Query\n"
	s += ui.DividerStyle.Render(strings.Repeat("-", m.Width-4)) + "\n"

	if m.LoadingDNS {
		s += "\nQuerying...\n"
	} else if m.DNSResult != nil {
		res := m.DNSResult
		if res.Error != nil {
			s += fmt.Sprintf("\nError: %v\n", res.Error)
		} else {
			s += fmt.Sprintf("\nServer: %s (%s)\n", res.Server, res.Protocol)
			s += fmt.Sprintf("Latency: %s\n", res.Latency)
			s += fmt.Sprintf("Response: %s\n", res.ResponseCode)

			if res.CertInfo != nil {
				s += "\nTLS Certificate:\n"
				s += fmt.Sprintf("  Subject: %s\n", res.CertInfo.Subject)
				s += fmt.Sprintf("  Issuer:  %s\n", res.CertInfo.Issuer)
				s += fmt.Sprintf("  Expires: %s\n", res.CertInfo.NotAfter.Format(time.RFC822))
				// s += fmt.Sprintf("  Version: TLS 1.%d\n", res.CertInfo.Version-0x0301+1)
			}

			s += "\nRecords:\n"
			if len(res.Records) == 0 {
				s += "  (No records found)\n"
			}
			for _, rec := range res.Records {
				s += fmt.Sprintf("  %s\n", rec)
			}

			// Ping Result
			s += "\nConnectivity:\n"
			if m.LoadingDNSPing {
				s += "  Checking connectivity...\n"
			} else if m.DNSPing != nil {
				ping := m.DNSPing
				if ping.Error != nil {
					s += fmt.Sprintf("  %s: %s\n", ping.Target, ui.ErrorStyle.Render(fmt.Sprintf("Failed (%v)", ping.Error)))
				} else {
					status := "OK"
					style := ui.SubtitleStyle
					if ping.PacketLoss > 0 {
						status = "Lossy"
						style = ui.WarningStyle
					}
					s += fmt.Sprintf("  %s: %s (Loss: %.0f%%, RTT: %s)\n",
						ping.Target, style.Render(status), ping.PacketLoss, ping.AvgRtt)
				}
			}
		}
	}

	return s
}
