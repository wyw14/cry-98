package telemetry

import (
	"time"
)

type Health struct {
	Accepted  uint64    `json:"accepted"`
	Dropped   uint64    `json:"dropped"`
	UpdatedAt time.Time `json:"updated_at"`
	Healthy   bool      `json:"healthy"`
}

func (i *Ingestor) Health() Health {
	accepted, dropped := i.Stats()
	return Health{
		Accepted:  accepted,
		Dropped:   dropped,
		UpdatedAt: i.LastUpdated(),
		Healthy:   dropped < accepted+10,
	}
}
