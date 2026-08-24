package journal

import (
	"encoding/json"
	"time"

	"github.com/wyw14/cry-98/internal/model"
)

type SuppressionState struct {
	AlarmID        string        `json:"alarm_id"`
	StartedAt      time.Time     `json:"started_at"`
	Duration       time.Duration `json:"duration"`
	Remaining      time.Duration `json:"remaining"`
	CheckpointedAt time.Time     `json:"checkpointed_at"`
}

func NewSuppressionState(alarmID string, startedAt time.Time, duration time.Duration) SuppressionState {
	return SuppressionState{
		AlarmID:        alarmID,
		StartedAt:      startedAt.UTC(),
		Duration:       duration,
		Remaining:      duration,
		CheckpointedAt: startedAt.UTC(),
	}
}

func (s SuppressionState) Checkpoint(now time.Time) SuppressionState {
	elapsed := now.Sub(s.CheckpointedAt)
	if elapsed < 0 {
		elapsed = 0
	}
	if elapsed >= s.Remaining {
		s.Remaining = 0
	} else {
		s.Remaining -= elapsed
	}
	s.CheckpointedAt = now.UTC()
	return s
}

func (s *Store) SaveSuppression(state SuppressionState, now time.Time) error {
	event, err := model.NewEvent("alarms", "alarm.suppression", state.AlarmID, state, now)
	if err != nil {
		return err
	}
	_, err = s.Append(event)
	return err
}

func (s *Store) LoadSuppression(alarmID string) (SuppressionState, bool, error) {
	events, err := s.Events("alarms", 0)
	if err != nil {
		return SuppressionState{}, false, err
	}
	var result SuppressionState
	found := false
	for _, event := range events {
		if event.Kind != "alarm.suppression" || event.AggregateID != alarmID {
			continue
		}
		if err := json.Unmarshal(event.Payload, &result); err != nil {
			return SuppressionState{}, false, err
		}
		found = true
	}
	return result, found, nil
}
