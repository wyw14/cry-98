package vessel

import (
	"errors"
	"time"

	"github.com/wyw14/cry-98/internal/model"
)

type StateService struct {
	registry *Registry
	now      func() time.Time
}

func NewStateService(registry *Registry, now func() time.Time) *StateService {
	if now == nil {
		now = time.Now
	}
	return &StateService{registry: registry, now: now}
}

func (s *StateService) Registry() *Registry {
	return s.registry
}

func (s *StateService) BeginFill(id string) (model.Vessel, error) {
	return s.registry.Update(id, func(current model.Vessel) (model.Vessel, error) {
		if current.Status == model.VesselFilling {
			return current, nil
		}
		if current.Status != model.VesselReady {
			return model.Vessel{}, errors.New("vessel is not ready for filling")
		}
		return current.WithStatus(model.VesselFilling, s.now()), nil
	})
}

func (s *StateService) FinishFill(id string) (model.Vessel, error) {
	return s.registry.Update(id, func(current model.Vessel) (model.Vessel, error) {
		if current.Status != model.VesselFilling {
			return model.Vessel{}, errors.New("vessel has no active fill")
		}
		return current.WithStatus(model.VesselReady, s.now()), nil
	})
}

func (s *StateService) ApplyTelemetry(id string, level, pressure, temperature float64, observedAt time.Time) (model.Vessel, error) {
	return s.registry.Update(id, func(current model.Vessel) (model.Vessel, error) {
		if observedAt.Before(current.UpdatedAt.Add(-time.Minute)) {
			return current, nil
		}
		return current.WithTelemetry(level, pressure, temperature, observedAt), nil
	})
}
