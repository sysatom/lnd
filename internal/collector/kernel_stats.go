package collector

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/vishvananda/netlink"
)

// TCP State constants (from include/net/tcp_states.h)
const (
	TCP_ESTABLISHED = 1
	TCP_TIME_WAIT   = 6
	TCP_CLOSE_WAIT  = 8
)

type KernelCollector struct {
	lastRetrans float64
	lastOutSegs float64
	mu          sync.Mutex
}

func NewKernelCollector() (*KernelCollector, error) {
	return &KernelCollector{}, nil
}

func (c *KernelCollector) Collect() (stats KernelStats, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in KernelCollector: %v", r)
			// Ensure we return the stats we have so far, or empty stats with error
			stats.Error = err
		}
	}()

	c.mu.Lock()
	defer c.mu.Unlock()
	// 1. SNMP Stats (TCP Retrans, UDP Errors)
	snmp, err := parseNetSnmp()
	if err != nil {
		return stats, fmt.Errorf("failed to read /proc/net/snmp: %v", err)
	}

	var tcpRetrans, tcpOutSegs float64
	if tcpStats, ok := snmp["Tcp"]; ok {
		if val, ok := tcpStats["RetransSegs"]; ok {
			tcpRetrans = val
		}
		if val, ok := tcpStats["OutSegs"]; ok {
			tcpOutSegs = val
		}
	}

	// Calculate Rate based on delta
	if c.lastOutSegs > 0 {
		deltaRetrans := tcpRetrans - c.lastRetrans
		deltaOut := tcpOutSegs - c.lastOutSegs
		if deltaOut > 0 {
			stats.TCPRetransRate = (deltaRetrans / deltaOut) * 100.0
		}
	} else {
		// First run, use total ratio as fallback or 0
		if tcpOutSegs > 0 {
			stats.TCPRetransRate = (tcpRetrans / tcpOutSegs) * 100.0
		}
	}

	c.lastRetrans = tcpRetrans
	c.lastOutSegs = tcpOutSegs

	// UDP Errors
	if udpStats, ok := snmp["Udp"]; ok {
		if val, ok := udpStats["RcvbufErrors"]; ok {
			stats.UDPRcvbufErrors = uint64(val)
		}
	}

	// 2. TCP States via Netlink (InetDiag)
	diag, err := netlink.SocketDiagTCPInfo(syscall.AF_INET)
	if err == nil {
		for _, info := range diag {
			switch info.InetDiagMsg.State {
			case TCP_ESTABLISHED:
				stats.TCPEstablished++
			case TCP_TIME_WAIT:
				stats.TCPTimeWait++
			case TCP_CLOSE_WAIT:
				stats.TCPCloseWait++
			}
		}
	}

	return stats, nil
}

func parseNetSnmp() (result map[string]map[string]float64, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic parsing snmp: %v", r)
		}
	}()

	file, err := os.Open("/proc/net/snmp")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	result = make(map[string]map[string]float64)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		proto := strings.TrimSuffix(parts[0], ":")

		// Read next line for values
		if !scanner.Scan() {
			break
		}
		valLine := scanner.Text()
		valParts := strings.Fields(valLine)

		// Ensure we have a map for this protocol
		if _, ok := result[proto]; !ok {
			result[proto] = make(map[string]float64)
		}

		// Iterate over parts, but ensure we don't go out of bounds of valParts
		for i := 1; i < len(parts) && i < len(valParts); i++ {
			val, err := strconv.ParseFloat(valParts[i], 64)
			if err == nil {
				result[proto][parts[i]] = val
			}
		}
	}
	return result, nil
}
