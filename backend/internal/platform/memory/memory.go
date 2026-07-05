package memory

import (
	"context"
	"runtime"
	"sync"
	"time"
)

type PressureMonitor struct {
	highWater uint64
	lowWater  uint64
	mu        sync.RWMutex
	under     bool
}

func NewPressureMonitor(highWaterMB, lowWaterMB int) *PressureMonitor {
	if highWaterMB <= 0 {
		highWaterMB = 1024
	}
	if lowWaterMB <= 0 || lowWaterMB >= highWaterMB {
		lowWaterMB = highWaterMB * 3 / 4
	}
	return &PressureMonitor{highWater: uint64(highWaterMB) * 1024 * 1024, lowWater: uint64(lowWaterMB) * 1024 * 1024}
}

func (m *PressureMonitor) Start(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	m.sample()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.sample()
		}
	}
}

func (m *PressureMonitor) sample() {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.under && stats.Alloc >= m.highWater {
		m.under = true
		return
	}
	if m.under && stats.Alloc <= m.lowWater {
		m.under = false
	}
}

func (m *PressureMonitor) IsUnderPressure() bool {
	if m == nil {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.under
}
