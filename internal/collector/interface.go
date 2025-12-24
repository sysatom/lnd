package collector

import (
	"time"
)

// HostInfo contains system and network interface information
type HostInfo struct {
	Hostname             string
	OS                   string // e.g. linux
	Platform             string // e.g. ubuntu
	PlatformFamily       string // e.g. debian
	PlatformVersion      string // e.g. 22.04
	KernelVersion        string
	Arch                 string
	VirtualizationSystem string // e.g. kvm, docker
	VirtualizationRole   string // e.g. guest, host
	Uptime               time.Duration
	Load1                float64
	Load5                float64
	Load15               float64
	MaxOpenFiles         uint64
	FileMax              uint64
	Interfaces           []InterfaceInfo
	SysctlParams         map[string]string
	Error                error
}

// InterfaceInfo contains details about a network interface
type InterfaceInfo struct {
	Name            string
	IP              string
	MAC             string
	MTU             int
	Driver          string
	DriverVersion   string
	FirmwareVersion string
	Offload         map[string]bool // TSO, GSO, LRO
}

// ConnectivityStats contains ping and DNS statistics
type ConnectivityStats struct {
	Targets map[string]PingResult
	DNS     DNSResult
}

type PingResult struct {
	Target     string
	PacketLoss float64
	MinRtt     time.Duration
	AvgRtt     time.Duration
	MaxRtt     time.Duration
	Error      error
}

type DNSResult struct {
	LocalResolverTime  time.Duration
	PublicResolverTime time.Duration
	Error              error
}

// TrafficStats contains bandwidth and physical error counts
type TrafficStats struct {
	Interfaces map[string]InterfaceTraffic
	Timestamp  time.Time
}

type InterfaceTraffic struct {
	RxBytes    uint64
	TxBytes    uint64
	RxRate     float64 // Bytes per second
	TxRate     float64 // Bytes per second
	Drop       uint64
	Errors     uint64
	Collisions uint64
}

// KernelStats contains TCP/UDP kernel statistics
type KernelStats struct {
	TCPRetransRate  float64
	TCPEstablished  uint64
	TCPTimeWait     uint64
	TCPCloseWait    uint64
	UDPRcvbufErrors uint64
	Error           error
}

// Collector defines the interface for data collection
type Collector interface {
	Collect() (interface{}, error)
}
