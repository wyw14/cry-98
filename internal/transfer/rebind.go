package transfer

import (
	"errors"
	"time"

	"github.com/wyw14/cry-98/internal/probe"
	"github.com/wyw14/cry-98/internal/telemetry"
)

type RebindResult struct {
	ProbeID        string    `json:"probe_id"`
	PreviousVessel string    `json:"previous_vessel"`
	CurrentVessel  string    `json:"current_vessel"`
	Generation     uint64    `json:"generation"`
	CompletedAt    time.Time `json:"completed_at"`
}

type RebindService struct {
	lifecycle     *probe.Lifecycle
	registry      *probe.Registry
	subscriptions *telemetry.SubscriptionManager
	now           func() time.Time
}

func NewRebindService(lifecycle *probe.Lifecycle, registry *probe.Registry, subscriptions *telemetry.SubscriptionManager, now func() time.Time) *RebindService {
	if now == nil {
		now = time.Now
	}
	return &RebindService{
		lifecycle:     lifecycle,
		registry:      registry,
		subscriptions: subscriptions,
		now:           now,
	}
}

func (s *RebindService) Rebind(probeID, vesselID string) (RebindResult, error) {
	current, ok := s.registry.Get(probeID)
	if !ok {
		return RebindResult{}, errors.New("probe not found")
	}
	if current.VesselID == vesselID {
		return RebindResult{
			ProbeID:        probeID,
			PreviousVessel: current.VesselID,
			CurrentVessel:  current.VesselID,
			Generation:     current.BindingGeneration,
			CompletedAt:    s.now().UTC(),
		}, nil
	}
	s.subscriptions.Stop(probeID)
	updated, err := s.lifecycle.Rebind(probeID, vesselID)
	if err != nil {
		return RebindResult{}, err
	}
	if err := s.subscriptions.Start(probeID); err != nil {
		_, _ = s.lifecycle.Rebind(probeID, current.VesselID)
		_ = s.subscriptions.Start(probeID)
		return RebindResult{}, err
	}
	return RebindResult{
		ProbeID:        probeID,
		PreviousVessel: current.VesselID,
		CurrentVessel:  updated.VesselID,
		Generation:     updated.BindingGeneration,
		CompletedAt:    s.now().UTC(),
	}, nil
}
