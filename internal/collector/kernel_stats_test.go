package collector

import (
	"os"
	"testing"
)

func TestKernelCollector_Collect(t *testing.T) {
	// Check if /proc/net/snmp exists (skip if not on Linux or inside restricted container)
	if _, err := os.Stat("/proc/net/snmp"); os.IsNotExist(err) {
		t.Skip("/proc/net/snmp not found, skipping kernel stats test")
	}

	c, err := NewKernelCollector()
	if err != nil {
		t.Fatalf("NewKernelCollector() error = %v", err)
	}

	stats, err := c.Collect()
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	// We can't assert specific values, but we can check if it ran without error
	// and populated some fields if the system has activity.
	t.Logf("TCP Retrans Rate: %.2f%%", stats.TCPRetransRate)
	t.Logf("TCP Established: %d", stats.TCPEstablished)
}

func TestParseNetSnmp(t *testing.T) {
	if _, err := os.Stat("/proc/net/snmp"); os.IsNotExist(err) {
		t.Skip("/proc/net/snmp not found")
	}

	data, err := parseNetSnmp()
	if err != nil {
		t.Fatalf("parseNetSnmp() error = %v", err)
	}

	if len(data) == 0 {
		t.Error("Parsed SNMP data is empty")
	}

	if _, ok := data["Tcp"]; !ok {
		t.Error("Missing Tcp stats in SNMP data")
	}
}
