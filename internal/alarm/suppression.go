package alarm

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wyw14/cry-98/internal/journal"
)

type SuppressionService struct {
	mu     sync.RWMutex
	store  *journal.Store
	states map[uuid.UUID]journal.SuppressionState
	now    func() time.Time
}

func NewSuppressionService(store *journal.Store, now func() time.Time) *SuppressionService {
	if now == nil {
		now = time.Now
	}
	return &SuppressionService{store: store, states: make(map[uuid.UUID]journal.SuppressionState), now: now}
}

func (s *SuppressionService) Suppress(alarmID uuid.UUID, duration time.Duration) error {
	if alarmID == uuid.Nil || duration <= 0 {
		return errors.New("invalid alarm suppression")
	}
	state := journal.NewSuppressionState(alarmID.String(), s.now(), duration)
	if err := s.store.SaveSuppression(state, s.now()); err != nil {
		return err
	}
	s.mu.Lock()
	s.states[alarmID] = state
	s.mu.Unlock()
	return nil
}

func (s *SuppressionService) Checkpoint(alarmID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.states[alarmID]
	if !ok {
		return errors.New("suppression not found")
	}
	state = state.Checkpoint(s.now())
	if err := s.store.SaveSuppression(state, s.now()); err != nil {
		return err
	}
	s.states[alarmID] = state
	return nil
}

func (s *SuppressionService) Remaining(alarmID uuid.UUID) time.Duration {
	s.mu.RLock()
	state, ok := s.states[alarmID]
	s.mu.RUnlock()
	if !ok {
		return 0
	}
	remaining := state.Checkpoint(s.now()).Remaining
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (s *SuppressionService) Restore(alarmID uuid.UUID) error {
	state, ok, err := s.store.LoadSuppression(alarmID.String())
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("persisted suppression not found")
	}
	wallRemaining := state.StartedAt.Add(state.Duration).Sub(s.now())
	if wallRemaining < 0 {
		wallRemaining = 0
	}
	state.Remaining = wallRemaining
	state.CheckpointedAt = s.now().UTC()
	s.mu.Lock()
	s.states[alarmID] = state
	s.mu.Unlock()
	return nil
}
