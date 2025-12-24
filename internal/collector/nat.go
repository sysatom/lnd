package collector

import (
	"fmt"
	"net"
	"time"

	"github.com/pion/stun/v3"
)

type NatType string

const (
	NatOpenInternet       NatType = "Open Internet"
	NatFullCone           NatType = "Full Cone"
	NatRestrictedCone     NatType = "Restricted Cone"
	NatPortRestrictedCone NatType = "Port Restricted Cone"
	NatSymmetric          NatType = "Symmetric NAT"
	NatUdpBlocked         NatType = "UDP Blocked"
	NatUnknown            NatType = "Unknown"
	NatBehindNat          NatType = "Behind NAT (Type Unknown)"
)

type NatInfo struct {
	Target   string
	NatType  NatType
	PublicIP string
	LocalIP  string
	Error    error
}

type StunTarget struct {
	Host string
	Port int
}

type NatCollector struct {
	Targets []StunTarget
}

func NewNatCollector(targets []StunTarget) *NatCollector {
	return &NatCollector{
		Targets: targets,
	}
}

func (c *NatCollector) Collect() ([]NatInfo, error) {
	var results []NatInfo
	// We could run this in parallel
	// For simplicity, let's do it sequentially or with a simple waitgroup if needed.
	// Given UI updates, parallel is better.

	ch := make(chan NatInfo, len(c.Targets))
	for _, t := range c.Targets {
		go func(target StunTarget) {
			ch <- c.probe(target)
		}(t)
	}

	for range c.Targets {
		results = append(results, <-ch)
	}

	return results, nil
}

func (c *NatCollector) probe(target StunTarget) NatInfo {
	info := NatInfo{
		Target:  fmt.Sprintf("%s:%d", target.Host, target.Port),
		NatType: NatUnknown,
	}

	// 1. Resolve and Dial STUN server
	// We use net.Dial to get the local address and ensure we are connected
	serverAddrStr := net.JoinHostPort(target.Host, fmt.Sprintf("%d", target.Port))
	conn, err := net.Dial("udp4", serverAddrStr)
	if err != nil {
		info.Error = fmt.Errorf("dialing stun host: %w", err)
		return info
	}

	// Get Local IP
	localAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		conn.Close()
		info.Error = fmt.Errorf("failed to cast local address to UDPAddr")
		return info
	}
	info.LocalIP = localAddr.IP.String()

	// 3. Create STUN Client
	client, err := stun.NewClient(conn)
	if err != nil {
		conn.Close()
		info.Error = fmt.Errorf("creating stun client: %w", err)
		return info
	}
	defer client.Close()

	// Building the request
	message := stun.MustBuild(stun.TransactionID, stun.BindingRequest)

	var xorAddr stun.XORMappedAddress
	var otherAddr stun.OtherAddress
	var mappedAddr stun.MappedAddress

	// Channel to receive signal when callback is done
	doneCh := make(chan struct{})

	err = client.Do(message, func(res stun.Event) {
		defer close(doneCh)
		if res.Error != nil {
			return
		}

		if getErr := xorAddr.GetFrom(res.Message); getErr == nil {
			info.PublicIP = xorAddr.IP.String()
		} else if getErr := mappedAddr.GetFrom(res.Message); getErr == nil {
			info.PublicIP = mappedAddr.IP.String()
		}

		// Check for OtherAddress (RFC 5780) for further tests
		otherAddr.GetFrom(res.Message)
	})

	if err != nil {
		info.NatType = NatUdpBlocked
		info.Error = fmt.Errorf("stun request failed: %w", err)
		return info
	}

	// Wait for response or timeout
	select {
	case <-doneCh:
		// Callback finished
	case <-time.After(2 * time.Second):
		info.Error = fmt.Errorf("stun request timed out")
		return info
	}

	if info.PublicIP == "" {
		info.NatType = NatUnknown
		if info.Error == nil {
			info.Error = fmt.Errorf("failed to get public ip")
		}
		return info
	}

	// Basic Check: Public IP vs Local IP
	if info.PublicIP == info.LocalIP {
		info.NatType = NatOpenInternet
		return info
	}

	// If we are here, we are behind NAT.
	if otherAddr.IP != nil {
		info.NatType = NatBehindNat
	} else {
		info.NatType = NatBehindNat
	}

	return info
}
