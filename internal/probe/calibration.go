package probe

import (
	"errors"
	"sync"
	"time"

	"github.com/wyw14/cry-98/internal/model"
)

type CalibrationNotice struct {
	ProbeID   string
	VesselID  string
	OldEpoch  uint64
	NewEpoch  uint64
	ChangedAt time.Time
}

type CalibrationListener func(CalibrationNotice)

type CalibrationService struct {
	registry  *Registry
	mu        sync.RWMutex
	listeners []CalibrationListener
	now       func() time.Time
}

func NewCalibrationService(registry *Registry, now func() time.Time) *CalibrationService {
	if now == nil {
		now = time.Now
	}
	return &CalibrationService{registry: registry, now: now}
}

func (s *CalibrationService) Subscribe(listener CalibrationListener) error {
	if listener == nil {
		return errors.New("calibration listener is required")
	}
	s.mu.Lock()
	s.listeners = append(s.listeners, listener)
	s.mu.Unlock()
	return nil
}

func (s *CalibrationService) Calibrate(probeID string) (model.Probe, error) {
	now := s.now()
	var oldEpoch uint64
	probe, err := s.registry.Update(probeID, func(current model.Probe) (model.Probe, error) {
		oldEpoch = current.CalibrationEpoch
		return current.Calibrate(now), nil
	})
	if err != nil {
		return model.Probe{}, err
	}
	notice := CalibrationNotice{
		ProbeID:   probe.ID,
		VesselID:  probe.VesselID,
		OldEpoch:  oldEpoch,
		NewEpoch:  probe.CalibrationEpoch,
		ChangedAt: now.UTC(),
	}
	s.mu.RLock()
	listeners := append([]CalibrationListener(nil), s.listeners...)
	s.mu.RUnlock()
	for _, listener := range listeners {
		listener(notice)
	}
	return probe, nil
}
