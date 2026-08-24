package alarm

import (
	"encoding/json"

	"github.com/wyw14/cry-98/internal/journal"
	"github.com/wyw14/cry-98/internal/model"
)

type Recovery struct {
	store   *journal.Store
	service *Service
}

func NewRecovery(store *journal.Store, service *Service) *Recovery {
	return &Recovery{store: store, service: service}
}

func (r *Recovery) Restore() error {
	events, err := r.store.Events("alarms", 0)
	if err != nil {
		return err
	}
	latest := make(map[string]model.Alarm)
	for _, event := range events {
		if event.Kind != "alarm.delivery" {
			continue
		}
		var alarm model.Alarm
		if err := json.Unmarshal(event.Payload, &alarm); err != nil {
			return err
		}
		latest[alarm.ID.String()] = alarm
	}
	for _, alarm := range latest {
		r.service.Restore(alarm)
	}
	return nil
}
