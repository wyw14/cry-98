package telemetry

import (
	"errors"
	"sync"
	"time"

	"github.com/wyw14/cry-98/internal/model"
	"github.com/wyw14/cry-98/internal/probe"
)

type SubscriptionManager struct {
	probes  *probe.Registry
	ingest  *Ingestor
	mu      sync.Mutex
	workers map[string]contextHandle
}

type contextHandle interface {
	Done() <-chan struct{}
	Cancel()
}

type workerContext struct {
	done chan struct{}
	once sync.Once
}

func (c *workerContext) Done() <-chan struct{} { return c.done }
func (c *workerContext) Cancel()               { c.once.Do(func() { close(c.done) }) }

func NewSubscriptionManager(probes *probe.Registry, ingest *Ingestor) *SubscriptionManager {
	return &SubscriptionManager{probes: probes, ingest: ingest, workers: make(map[string]contextHandle)}
}

func (m *SubscriptionManager) Start(probeID string) error {
	m.mu.Lock()
	if _, exists := m.workers[probeID]; exists {
		m.mu.Unlock()
		return nil
	}
	subscription, ok := m.probes.Subscription(probeID)
	if !ok {
		m.mu.Unlock()
		return errors.New("probe subscription not found")
	}
	worker := &workerContext{done: make(chan struct{})}
	m.workers[probeID] = worker
	m.mu.Unlock()
	go m.consume(probeID, subscription, worker)
	return nil
}

func (m *SubscriptionManager) consume(probeID string, subscription *probe.Subscription, worker contextHandle) {
	for {
		select {
		case <-worker.Done():
			return
		case sample, ok := <-subscription.Samples():
			if !ok {
				return
			}
			_ = m.ingest.Ingest(sample)
		}
	}
}

func (m *SubscriptionManager) Stop(probeID string) {
	m.mu.Lock()
	worker := m.workers[probeID]
	delete(m.workers, probeID)
	m.mu.Unlock()
	if worker != nil {
		worker.Cancel()
	}
}

func (m *SubscriptionManager) Publish(probeID string, value float64, at time.Time) error {
	probeState, ok := m.probes.Get(probeID)
	if !ok {
		return errors.New("probe not found")
	}
	subscription, ok := m.probes.Subscription(probeID)
	if !ok || subscription.Closed() {
		return errors.New("probe subscription is closed")
	}
	if !subscription.Send(probeState.NewSample(value, at)) {
		return errors.New("probe sample queue is full")
	}
	return nil
}

func (m *SubscriptionManager) PublishSample(sample model.Sample) error {
	subscription, ok := m.probes.Subscription(sample.ProbeID)
	if !ok || subscription.Closed() {
		return errors.New("probe subscription is closed")
	}
	if !subscription.Send(sample) {
		return errors.New("probe sample queue is full")
	}
	return nil
}
