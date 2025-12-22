package collector

import (
	"testing"
)

func TestSystemCollector_Collect(t *testing.T) {
	c := NewSystemCollector()
	info, err := c.Collect()
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	if info.Hostname == "" {
		t.Error("Hostname is empty")
	}
	if info.KernelVersion == "" {
		t.Error("KernelVersion is empty")
	}
	if info.Uptime == 0 {
		t.Error("Uptime is 0")
	}
	// Note: Interfaces might be empty in some container environments, so we don't strictly assert len > 0
}
