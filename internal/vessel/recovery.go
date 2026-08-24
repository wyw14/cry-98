package vessel

import (
	"encoding/json"
	"errors"

	"github.com/wyw14/cry-98/internal/model"
)

type Recovery struct {
	registry *Registry
}

func NewRecovery(registry *Registry) *Recovery {
	return &Recovery{registry: registry}
}

func (r *Recovery) Restore(events []model.Event) error {
	for _, event := range events {
		if event.Kind != "vessel.snapshot" {
			continue
		}
		var vessel model.Vessel
		if err := json.Unmarshal(event.Payload, &vessel); err != nil {
			return err
		}
		if _, exists := r.registry.Get(vessel.ID); exists {
			_, err := r.registry.Update(vessel.ID, func(current model.Vessel) (model.Vessel, error) {
				if vessel.Generation < current.Generation {
					return model.Vessel{}, errors.New("recovered vessel generation moved backward")
				}
				return vessel, nil
			})
			if err != nil {
				return err
			}
			continue
		}
		if err := r.registry.Register(vessel); err != nil {
			return err
		}
	}
	return nil
}

func (r *Recovery) Snapshot() []model.Vessel {
	return r.registry.List()
}
