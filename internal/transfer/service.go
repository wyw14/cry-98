package transfer

import (
	"errors"
	"time"
)

type Transfer struct {
	ProbeID     string    `json:"probe_id"`
	FromVessel  string    `json:"from_vessel"`
	ToVessel    string    `json:"to_vessel"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	Status      string    `json:"status"`
}

type Service struct {
	rebind *RebindService
	now    func() time.Time
}

func NewService(rebind *RebindService, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{rebind: rebind, now: now}
}

func (s *Service) TakeOver(probeID, vesselID string) (Transfer, error) {
	if probeID == "" || vesselID == "" {
		return Transfer{}, errors.New("probe and target vessel are required")
	}
	started := s.now().UTC()
	result, err := s.rebind.Rebind(probeID, vesselID)
	transfer := Transfer{
		ProbeID:     probeID,
		FromVessel:  result.PreviousVessel,
		ToVessel:    vesselID,
		StartedAt:   started,
		CompletedAt: s.now().UTC(),
		Status:      "completed",
	}
	if err != nil {
		transfer.Status = "failed"
	}
	return transfer, err
}
