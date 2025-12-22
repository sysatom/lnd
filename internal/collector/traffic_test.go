package collector

import (
	"testing"
	"time"
)

func TestTrafficCollector_Collect(t *testing.T) {
	c := NewTrafficCollector()

	// First collection
	stats1, err := c.Collect()
	if err != nil {
		t.Fatalf("First Collect() error = %v", err)
	}
	if stats1.Timestamp.IsZero() {
		t.Error("Timestamp is zero")
	}

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	// Second collection (to test rate calculation logic, though rates might be 0)
	stats2, err := c.Collect()
	if err != nil {
		t.Fatalf("Second Collect() error = %v", err)
	}

	if stats2.Timestamp.Before(stats1.Timestamp) {
		t.Error("Timestamp went backwards")
	}
}
