package collector

import (
	"fmt"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/net"
)

type TrafficCollector struct {
	lastTime  time.Time
	lastStats map[string]net.IOCountersStat
	mu        sync.Mutex
}

func NewTrafficCollector() *TrafficCollector {
	return &TrafficCollector{
		lastStats: make(map[string]net.IOCountersStat),
	}
}

func (c *TrafficCollector) Collect() (stats TrafficStats, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in TrafficCollector: %v", r)
		}
	}()

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	stats = TrafficStats{
		Interfaces: make(map[string]InterfaceTraffic),
		Timestamp:  now,
	}

	counters, err := net.IOCounters(true) // per interface
	if err != nil {
		return stats, err
	}

	for _, counter := range counters {
		t := InterfaceTraffic{
			RxBytes:    counter.BytesRecv,
			TxBytes:    counter.BytesSent,
			Drop:       counter.Dropin + counter.Dropout,
			Errors:     counter.Errin + counter.Errout,
			Collisions: 0, // gopsutil might not have collisions in all versions, check struct
		}

		// Calculate Rate
		if !c.lastTime.IsZero() {
			duration := now.Sub(c.lastTime).Seconds()
			if duration > 0 {
				if last, ok := c.lastStats[counter.Name]; ok {
					if counter.BytesRecv >= last.BytesRecv {
						t.RxRate = float64(counter.BytesRecv-last.BytesRecv) / duration
					}
					if counter.BytesSent >= last.BytesSent {
						t.TxRate = float64(counter.BytesSent-last.BytesSent) / duration
					}
				}
			}
		}

		stats.Interfaces[counter.Name] = t
		c.lastStats[counter.Name] = counter
	}

	c.lastTime = now
	return stats, nil
}
