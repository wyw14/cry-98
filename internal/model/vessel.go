package model

import (
	"errors"
	"strings"
	"time"
)

type VesselStatus string

const (
	VesselOffline VesselStatus = "offline"
	VesselReady   VesselStatus = "ready"
	VesselFilling VesselStatus = "filling"
	VesselFault   VesselStatus = "fault"
)

type Vessel struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Zone         string       `json:"zone"`
	Generation   uint64       `json:"generation"`
	Status       VesselStatus `json:"status"`
	LevelMM      float64      `json:"level_mm"`
	PressureKPa  float64      `json:"pressure_kpa"`
	TemperatureC float64      `json:"temperature_c"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

func NewVessel(id, name, zone string, now time.Time) (Vessel, error) {
	id = strings.TrimSpace(id)
	name = strings.TrimSpace(name)
	zone = strings.TrimSpace(zone)
	if id == "" || name == "" || zone == "" {
		return Vessel{}, errors.New("vessel id, name, and zone are required")
	}
	return Vessel{
		ID:         id,
		Name:       name,
		Zone:       zone,
		Generation: 1,
		Status:     VesselReady,
		UpdatedAt:  now.UTC(),
	}, nil
}

func (v Vessel) Validate() error {
	if strings.TrimSpace(v.ID) == "" {
		return errors.New("vessel id is required")
	}
	if v.Generation == 0 {
		return errors.New("vessel generation must be positive")
	}
	switch v.Status {
	case VesselOffline, VesselReady, VesselFilling, VesselFault:
		return nil
	default:
		return errors.New("unknown vessel status")
	}
}

func (v Vessel) WithStatus(status VesselStatus, now time.Time) Vessel {
	v.Status = status
	v.UpdatedAt = now.UTC()
	return v
}

func (v Vessel) WithTelemetry(level, pressure, temperature float64, now time.Time) Vessel {
	v.LevelMM = level
	v.PressureKPa = pressure
	v.TemperatureC = temperature
	v.UpdatedAt = now.UTC()
	return v
}
