package probe

import (
	"errors"
	"time"

	"github.com/wyw14/cry-98/internal/model"
)

type Lifecycle struct {
	registry *Registry
	now      func() time.Time
}

func NewLifecycle(registry *Registry, now func() time.Time) *Lifecycle {
	if now == nil {
		now = time.Now
	}
	return &Lifecycle{registry: registry, now: now}
}

func (l *Lifecycle) Disconnect(id string) (model.Probe, error) {
	return l.registry.Update(id, func(current model.Probe) (model.Probe, error) {
		if !current.Connected {
			return current, nil
		}
		return current.Disconnect(l.now()), nil
	})
}

func (l *Lifecycle) Rebind(id, vesselID string) (model.Probe, error) {
	if vesselID == "" {
		return model.Probe{}, errors.New("target vessel is required")
	}
	return l.registry.Update(id, func(current model.Probe) (model.Probe, error) {
		return current.Rebind(vesselID, l.now())
	})
}

func (l *Lifecycle) Detach(id string) error {
	return l.registry.Detach(id, l.now())
}

func (l *Lifecycle) Attach(id string) (*Subscription, error) {
	return l.registry.Attach(id, l.now())
}
