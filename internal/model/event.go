package model

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Event struct {
	ID          uuid.UUID       `json:"id"`
	Partition   string          `json:"partition"`
	Sequence    uint64          `json:"sequence"`
	Kind        string          `json:"kind"`
	AggregateID string          `json:"aggregate_id"`
	OccurredAt  time.Time       `json:"occurred_at"`
	Payload     json.RawMessage `json:"payload"`
}

func NewEvent(partition, kind, aggregateID string, payload any, now time.Time) (Event, error) {
	if partition == "" || kind == "" || aggregateID == "" {
		return Event{}, errors.New("event identity is incomplete")
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return Event{}, err
	}
	return Event{
		ID:          uuid.New(),
		Partition:   partition,
		Kind:        kind,
		AggregateID: aggregateID,
		OccurredAt:  now.UTC(),
		Payload:     encoded,
	}, nil
}

func (e Event) Validate() error {
	if e.ID == uuid.Nil || e.Partition == "" || e.Kind == "" || e.AggregateID == "" {
		return errors.New("invalid event")
	}
	if !json.Valid(e.Payload) {
		return errors.New("event payload is not valid json")
	}
	return nil
}

type SnapshotEnvelope struct {
	Partition string          `json:"partition"`
	Sequence  uint64          `json:"sequence"`
	SavedAt   time.Time       `json:"saved_at"`
	Payload   json.RawMessage `json:"payload"`
}

func NewSnapshotEnvelope(partition string, sequence uint64, value any, now time.Time) (SnapshotEnvelope, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return SnapshotEnvelope{}, err
	}
	return SnapshotEnvelope{
		Partition: partition,
		Sequence:  sequence,
		SavedAt:   now.UTC(),
		Payload:   payload,
	}, nil
}
