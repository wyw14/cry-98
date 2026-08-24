package monitor

import (
	"sort"
	"sync"
	"time"

	"github.com/wyw14/cry-98/internal/model"
	"github.com/wyw14/cry-98/internal/probe"
)

type ProbeHealth struct {
	ProbeID    string    `json:"probe_id"`
	VesselID   string    `json:"vessel_id"`
	Connected  bool      `json:"connected"`
	LastSample time.Time `json:"last_sample,omitempty"`
	Stale      bool      `json:"stale"`
	CheckedAt  time.Time `json:"checked_at"`
}

type ProbeMonitor struct {
	registry   *probe.Registry
	mu         sync.RWMutex
	last       map[string]time.Time
	staleAfter time.Duration
	now        func() time.Time
}

func NewProbeMonitor(registry *probe.Registry, staleAfter time.Duration, now func() time.Time) *ProbeMonitor {
	if now == nil {
		now = time.Now
	}
	return &ProbeMonitor{registry: registry, last: make(map[string]time.Time), staleAfter: staleAfter, now: now}
}

func (m *ProbeMonitor) Observe(sample model.Sample) {
	m.mu.Lock()
	m.last[sample.ProbeID] = sample.ObservedAt
	m.mu.Unlock()
}

func (m *ProbeMonitor) Health(probeID string) (ProbeHealth, bool) {
	probeState, ok := m.registry.Get(probeID)
	if !ok {
		return ProbeHealth{}, false
	}
	m.mu.RLock()
	last := m.last[probeID]
	m.mu.RUnlock()
	now := m.now()
	stale := last.IsZero() || now.Sub(last) > m.staleAfter
	return ProbeHealth{
		ProbeID:    probeID,
		VesselID:   probeState.VesselID,
		Connected:  probeState.Connected,
		LastSample: last,
		Stale:      stale,
		CheckedAt:  now.UTC(),
	}, true
}

func (m *ProbeMonitor) All() []ProbeHealth {
	probes := m.registry.List()
	result := make([]ProbeHealth, 0, len(probes))
	for _, probeState := range probes {
		if health, ok := m.Health(probeState.ID); ok {
			result = append(result, health)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ProbeID < result[j].ProbeID })
	return result
}

func (m *ProbeMonitor) Disconnected() []ProbeHealth {
	all := m.All()
	result := make([]ProbeHealth, 0)
	for _, health := range all {
		if !health.Connected || health.Stale {
			result = append(result, health)
		}
	}
	return result
}
