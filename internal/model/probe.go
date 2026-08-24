package model

import (
	"errors"
	"strings"
	"time"
)

type ProbeKind string

const (
	ProbeLevel       ProbeKind = "level"
	ProbePressure    ProbeKind = "pressure"
	ProbeTemperature ProbeKind = "temperature"
)

type Probe struct {
	ID                string    `json:"id"`
	Kind              ProbeKind `json:"kind"`
	VesselID          string    `json:"vessel_id"`
	CalibrationEpoch  uint64    `json:"calibration_epoch"`
	BindingGeneration uint64    `json:"binding_generation"`
	Connected         bool      `json:"connected"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type Sample struct {
	ProbeID           string    `json:"probe_id"`
	VesselID          string    `json:"vessel_id"`
	Kind              ProbeKind `json:"kind"`
	Value             float64   `json:"value"`
	CalibrationEpoch  uint64    `json:"calibration_epoch"`
	BindingGeneration uint64    `json:"binding_generation"`
	ObservedAt        time.Time `json:"observed_at"`
}

func NewProbe(id string, kind ProbeKind, vesselID string, now time.Time) (Probe, error) {
	id = strings.TrimSpace(id)
	vesselID = strings.TrimSpace(vesselID)
	if id == "" || vesselID == "" {
		return Probe{}, errors.New("probe id and vessel id are required")
	}
	if !kind.Valid() {
		return Probe{}, errors.New("unknown probe kind")
	}
	return Probe{
		ID:                id,
		Kind:              kind,
		VesselID:          vesselID,
		CalibrationEpoch:  1,
		BindingGeneration: 1,
		Connected:         true,
		UpdatedAt:         now.UTC(),
	}, nil
}

func (k ProbeKind) Valid() bool {
	switch k {
	case ProbeLevel, ProbePressure, ProbeTemperature:
		return true
	default:
		return false
	}
}

func (p Probe) Rebind(vesselID string, now time.Time) (Probe, error) {
	vesselID = strings.TrimSpace(vesselID)
	if vesselID == "" {
		return Probe{}, errors.New("target vessel is required")
	}
	p.VesselID = vesselID
	p.BindingGeneration++
	p.Connected = true
	p.UpdatedAt = now.UTC()
	return p, nil
}

func (p Probe) Calibrate(now time.Time) Probe {
	p.CalibrationEpoch++
	p.UpdatedAt = now.UTC()
	return p
}

func (p Probe) Disconnect(now time.Time) Probe {
	p.Connected = false
	p.UpdatedAt = now.UTC()
	return p
}

func (p Probe) NewSample(value float64, observedAt time.Time) Sample {
	return Sample{
		ProbeID:           p.ID,
		VesselID:          p.VesselID,
		Kind:              p.Kind,
		Value:             value,
		CalibrationEpoch:  p.CalibrationEpoch,
		BindingGeneration: p.BindingGeneration,
		ObservedAt:        observedAt.UTC(),
	}
}
