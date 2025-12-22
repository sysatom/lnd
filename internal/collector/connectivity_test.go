package collector

import (
	"testing"
)

func TestConnectivityCollector_Collect(t *testing.T) {
	c := NewConnectivityCollector()
	// Override targets to localhost for faster/reliable testing
	c.Targets = []string{"127.0.0.1"}

	stats, err := c.Collect()
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	if len(stats.Targets) == 0 {
		t.Error("No targets pinged")
	}

	if res, ok := stats.Targets["127.0.0.1"]; ok {
		if res.Error != nil {
			t.Logf("Ping 127.0.0.1 failed (expected in some envs): %v", res.Error)
		} else {
			if res.PacketLoss == 100 {
				t.Log("100% packet loss to localhost")
			}
		}
	} else {
		t.Error("127.0.0.1 result missing")
	}

	// DNS check might fail without internet, but shouldn't panic
	if stats.DNS.Error != nil {
		t.Logf("DNS check failed: %v", stats.DNS.Error)
	}
}
