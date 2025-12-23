package collector

import (
	"testing"
)

func TestNatCollector_Collect(t *testing.T) {
	// This test requires internet access and a valid STUN server.
	// In a CI environment without internet, this might fail or hang.
	// We can skip it if needed, or mock the STUN server.
	// For now, let's do a basic integration test but skip if short mode.

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	targets := []StunTarget{
		{Host: "stun.l.google.com", Port: 19302},
	}

	c := NewNatCollector(targets)
	results, err := c.Collect()

	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	res := results[0]
	if res.Error != nil {
		// It's possible STUN fails due to network, but we shouldn't crash.
		t.Logf("STUN probe failed (expected in offline env): %v", res.Error)
	} else {
		if res.PublicIP == "" {
			t.Error("Expected PublicIP to be set")
		}
		if res.LocalIP == "" {
			t.Error("Expected LocalIP to be set")
		}
		if res.NatType == NatUnknown {
			t.Error("Expected NatType to be determined")
		}
		t.Logf("NAT Type: %s, Public: %s, Local: %s", res.NatType, res.PublicIP, res.LocalIP)
	}
}
