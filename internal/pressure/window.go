package pressure

import (
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/wyw14/cry-98/internal/model"
)

type WindowSample struct {
	Value            float64   `json:"value"`
	CalibrationEpoch uint64    `json:"calibration_epoch"`
	ObservedAt       time.Time `json:"observed_at"`
}

type Window struct {
	mu       sync.RWMutex
	vesselID string
	epoch    uint64
	capacity int
	samples  []WindowSample
	spread   float64
}

func NewWindow(vesselID string, capacity int, spread float64) (*Window, error) {
	if vesselID == "" || capacity < 2 || spread < 0 {
		return nil, errors.New("invalid pressure window")
	}
	return &Window{vesselID: vesselID, capacity: capacity, spread: spread}, nil
}

func (w *Window) Reset(epoch uint64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.epoch = epoch
	w.samples = nil
}

func (w *Window) Add(sample model.Sample) bool {
	if sample.Kind != model.ProbePressure || sample.VesselID != w.vesselID {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	switch {
	case w.epoch == 0:
		// The first pressure observation seeds the calibration generation.
		w.epoch = sample.CalibrationEpoch
	case sample.CalibrationEpoch > w.epoch:
		// The probe was recalibrated or replaced mid-soak. Stale readings
		// from the previous generation can no longer testify to stability,
		// so discard them and re-accumulate a full window for the new
		// generation starting with this sample.
		w.epoch = sample.CalibrationEpoch
		w.samples = nil
	case sample.CalibrationEpoch < w.epoch:
		// An obsolete-generation sample (e.g. buffered before a
		// recalibration) must not contribute to a stability decision.
		return false
	}
	w.samples = append(w.samples, WindowSample{
		Value:            sample.Value,
		CalibrationEpoch: sample.CalibrationEpoch,
		ObservedAt:       sample.ObservedAt,
	})
	if len(w.samples) > w.capacity {
		w.samples = w.samples[len(w.samples)-w.capacity:]
	}
	return true
}

func (w *Window) Stable() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	// A full window of the current calibration generation must accumulate
	// before pressure stability can be declared. Stale-generation readings
	// never count toward that quorum, so a recalibration mid-soak forces
	// the window to refill from the new generation.
	if len(w.samples) < w.capacity || w.epoch == 0 {
		return false
	}
	minimum, maximum := w.samples[0].Value, w.samples[0].Value
	for _, sample := range w.samples[1:] {
		if sample.CalibrationEpoch != w.epoch {
			return false
		}
		minimum = minFloat(minimum, sample.Value)
		maximum = maxFloat(maximum, sample.Value)
	}
	return maximum-minimum <= w.spread
}

func (w *Window) Samples() []WindowSample {
	w.mu.RLock()
	defer w.mu.RUnlock()
	result := append([]WindowSample(nil), w.samples...)
	sort.Slice(result, func(i, j int) bool { return result[i].ObservedAt.Before(result[j].ObservedAt) })
	return result
}

func (w *Window) Epoch() uint64 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.epoch
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
