package collector

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/go-ping/ping"
	"github.com/vishvananda/netlink"
)

type ConnectivityCollector struct {
	Targets []string
}

func NewConnectivityCollector() *ConnectivityCollector {
	return &ConnectivityCollector{
		Targets: []string{"8.8.8.8", "baidu.com"},
	}
}

func (c *ConnectivityCollector) Collect() (stats ConnectivityStats, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in ConnectivityCollector: %v", r)
		}
	}()

	stats = ConnectivityStats{
		Targets: make(map[string]PingResult),
	}

	// Create a local copy of targets to avoid race condition if we modify it
	// Actually, we shouldn't modify c.Targets here.
	// We will build a list of targets to ping for this run.
	targetsToPing := make([]string, len(c.Targets))
	copy(targetsToPing, c.Targets)

	// Add Gateway to targets (locally)
	gw, err := getDefaultGateway()
	if err == nil && gw != "" {
		// Check if gw is already in targets
		found := false
		for _, t := range targetsToPing {
			if t == gw {
				found = true
				break
			}
		}
		if !found {
			targetsToPing = append([]string{gw}, targetsToPing...)
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	// Ping Targets
	for _, target := range targetsToPing {
		wg.Add(1)
		go func(t string) {
			defer wg.Done()
			res := pingTarget(t)
			mu.Lock()
			stats.Targets[t] = res
			mu.Unlock()
		}(target)
	}

	// DNS Check
	wg.Add(1)
	go func() {
		defer wg.Done()
		dnsRes := checkDNS()
		mu.Lock()
		stats.DNS = dnsRes
		mu.Unlock()
	}()

	wg.Wait()
	return stats, nil
}

func getDefaultGateway() (string, error) {
	routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		return "", err
	}

	for _, r := range routes {
		if r.Dst == nil { // Default route
			return r.Gw.String(), nil
		}
	}
	return "", fmt.Errorf("no default gateway found")
}

func (c *ConnectivityCollector) Ping(target string) PingResult {
	return pingTarget(target)
}

func pingTarget(target string) PingResult {
	pinger, err := ping.NewPinger(target)
	if err != nil {
		return PingResult{Target: target, Error: err}
	}

	pinger.Count = 3
	pinger.Timeout = 2 * time.Second
	pinger.SetPrivileged(true) // Try privileged (ICMP)

	// Fallback to unprivileged if needed is handled by library usually,
	// but on Linux usually requires root or sysctl net.ipv4.ping_group_range

	err = pinger.Run()
	if err != nil {
		// Try TCP Ping if ICMP fails or permission denied
		return tcpPing(target)
	}

	stats := pinger.Statistics()
	return PingResult{
		Target:     target,
		PacketLoss: stats.PacketLoss,
		MinRtt:     stats.MinRtt,
		AvgRtt:     stats.AvgRtt,
		MaxRtt:     stats.MaxRtt,
	}
}

func tcpPing(target string) PingResult {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(target, "80"), 2*time.Second)
	if err != nil {
		// Try 443
		conn, err = net.DialTimeout("tcp", net.JoinHostPort(target, "443"), 2*time.Second)
	}

	if err != nil {
		return PingResult{Target: target, Error: err, PacketLoss: 100}
	}
	defer conn.Close()

	rtt := time.Since(start)
	return PingResult{
		Target:     target,
		PacketLoss: 0,
		MinRtt:     rtt,
		AvgRtt:     rtt,
		MaxRtt:     rtt,
	}
}

func checkDNS() DNSResult {
	res := DNSResult{}

	// Local DNS
	start := time.Now()
	_, err := net.LookupHost("google.com")
	res.LocalResolverTime = time.Since(start)
	if err != nil {
		res.Error = err
	}

	// Public DNS (1.1.1.1)
	// We can't easily force a specific DNS server with pure Go net.Resolver without custom Dial
	// So we will simulate it by creating a custom resolver

	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: 2 * time.Second,
			}
			return d.DialContext(ctx, "udp", "1.1.1.1:53")
		},
	}

	start = time.Now()
	_, err = r.LookupHost(context.Background(), "google.com")
	res.PublicResolverTime = time.Since(start)

	return res
}
