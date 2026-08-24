package pressure

import (
	"testing"
	"time"

	"github.com/wyw14/cry-98/internal/model"
)

func TestWindowResetDiscardsPriorGeneration(t *testing.T) {
	now := func() time.Time { return time.Unix(100, 0) }
	monitor := NewMonitor(now)
	if err := monitor.AddVessel("V-07", 5, 1.5); err != nil {
		t.Fatal(err)
	}
	// Three pre-calibration samples (epoch 1) accumulate.
	for _, v := range []float64{100, 100.2, 100.1} {
		if err := monitor.Add(model.Sample{ProbeID: "P-P07", VesselID: "V-07", Kind: model.ProbePressure, Value: v, CalibrationEpoch: 1, ObservedAt: now()}); err != nil {
			t.Fatal(err)
		}
	}
	// Engineer recalibrates -> epoch becomes 2 -> Reset must clear the window.
	if err := monitor.Reset("V-07", 2); err != nil {
		t.Fatal(err)
	}
	// Only two new post-calibration readings arrive.
	for _, v := range []float64{200, 200.1} {
		if err := monitor.Add(model.Sample{ProbeID: "P-P07", VesselID: "V-07", Kind: model.ProbePressure, Value: v, CalibrationEpoch: 2, ObservedAt: now()}); err != nil {
			t.Fatal(err)
		}
	}
	res, err := monitor.Evaluate("V-07")
	if err != nil {
		t.Fatal(err)
	}
	if res.Stable {
		t.Fatalf("heat balance passed early with only %d samples", res.Samples)
	}
	if res.Samples != 2 {
		t.Fatalf("expected 2 samples in window, got %d", res.Samples)
	}
}

// TestWindowAddReaccumulatesOnCalibrationGenerationAdvance reproduces the
// reported incident: the probe is recalibrated (epoch 1 -> 2) and the window
// learns about the new generation from the incoming samples themselves.
// Window.Add must discard the stale-generation readings instead of silently
// relabelling the epoch, so two new readings cannot satisfy the quorum while
// three pre-calibration samples linger.
func TestWindowAddReaccumulatesOnCalibrationGenerationAdvance(t *testing.T) {
	now := func() time.Time { return time.Unix(100, 0) }
	monitor := NewMonitor(now)
	if err := monitor.AddVessel("V-07", 5, 1.5); err != nil {
		t.Fatal(err)
	}
	// Three pre-calibration samples (epoch 1), all within spread.
	for _, v := range []float64{100, 100.1, 100.2} {
		if err := monitor.Add(model.Sample{ProbeID: "P-P07", VesselID: "V-07", Kind: model.ProbePressure, Value: v, CalibrationEpoch: 1, ObservedAt: now()}); err != nil {
			t.Fatal(err)
		}
	}
	// Recalibration happens; the window is NOT explicitly reset here - it only
	// observes the new generation from the next samples (the Add fallback).
	// Only two post-calibration readings arrive, both near the old values.
	for _, v := range []float64{100.05, 100.15} {
		if err := monitor.Add(model.Sample{ProbeID: "P-P07", VesselID: "V-07", Kind: model.ProbePressure, Value: v, CalibrationEpoch: 2, ObservedAt: now()}); err != nil {
			t.Fatal(err)
		}
	}
	res, err := monitor.Evaluate("V-07")
	if err != nil {
		t.Fatal(err)
	}
	if res.Stable {
		t.Fatalf("heat balance passed early with mixed generations: %d samples, stable=%v", res.Samples, res.Stable)
	}
	if res.Samples != 2 {
		t.Fatalf("expected window to hold only 2 current-generation samples, got %d", res.Samples)
	}
	if res.Epoch != 2 {
		t.Fatalf("expected epoch 2, got %d", res.Epoch)
	}
}

// TestWindowStableRequiresFullCurrentGenerationWindow pins the reported
// incident: after a recalibration the new readings happen to sit within the
// configured spread, yet with fewer than a full window of current-generation
// samples the stability check must stay false. Pre-calibration samples can
// never be recycled to satisfy the quorum, and stability returns once a full
// window of the new generation accumulates.
func TestWindowStableRequiresFullCurrentGenerationWindow(t *testing.T) {
	now := func() time.Time { return time.Unix(100, 0) }
	monitor := NewMonitor(now)
	if err := monitor.AddVessel("V-07", 5, 1.5); err != nil {
		t.Fatal(err)
	}
	// Fill the window with five pre-calibration readings within spread.
	for _, v := range []float64{100.0, 100.3, 100.2, 100.4, 100.1} {
		if err := monitor.Add(model.Sample{ProbeID: "P-P07", VesselID: "V-07", Kind: model.ProbePressure, Value: v, CalibrationEpoch: 1, ObservedAt: now()}); err != nil {
			t.Fatal(err)
		}
	}
	if res, _ := monitor.Evaluate("V-07"); !res.Stable {
		t.Fatalf("precondition: window should be stable before recalibration, got stable=%v", res.Stable)
	}
	// Recalibrate -> epoch 2. Only two new readings arrive, within spread.
	for _, v := range []float64{100.2, 100.5} {
		if err := monitor.Add(model.Sample{ProbeID: "P-P07", VesselID: "V-07", Kind: model.ProbePressure, Value: v, CalibrationEpoch: 2, ObservedAt: now()}); err != nil {
			t.Fatal(err)
		}
	}
	res, err := monitor.Evaluate("V-07")
	if err != nil {
		t.Fatal(err)
	}
	if res.Stable {
		t.Fatalf("heat balance passed with only %d current-generation samples", res.Samples)
	}
	if res.Samples != 2 {
		t.Fatalf("expected window to hold only 2 current-generation samples, got %d", res.Samples)
	}
	// Once a full window of the new generation accumulates, stability returns.
	for _, v := range []float64{100.1, 100.2, 100.0} {
		if err := monitor.Add(model.Sample{ProbeID: "P-P07", VesselID: "V-07", Kind: model.ProbePressure, Value: v, CalibrationEpoch: 2, ObservedAt: now()}); err != nil {
			t.Fatal(err)
		}
	}
	if res, _ := monitor.Evaluate("V-07"); !res.Stable {
		t.Fatalf("expected stability after full window of current generation, got stable=%v", res.Stable)
	}
}
