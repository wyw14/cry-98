package pressure

import (
	"errors"
	"time"
)

type Policy struct {
	RequiredSamples int           `json:"required_samples"`
	MaximumSpread   float64       `json:"maximum_spread"`
	MinimumDuration time.Duration `json:"minimum_duration"`
}

func DefaultPolicy() Policy {
	return Policy{RequiredSamples: 5, MaximumSpread: 1.5, MinimumDuration: 4 * time.Second}
}

func (p Policy) Validate() error {
	if p.RequiredSamples < 2 {
		return errors.New("pressure policy requires at least two samples")
	}
	if p.MaximumSpread < 0 {
		return errors.New("pressure spread cannot be negative")
	}
	if p.MinimumDuration < 0 {
		return errors.New("pressure duration cannot be negative")
	}
	return nil
}

func (p Policy) NewWindow(vesselID string) (*Window, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return NewWindow(vesselID, p.RequiredSamples, p.MaximumSpread)
}
