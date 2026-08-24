package pressure

import (
	"testing"
	"time"

	"github.com/wyw14/cry-98/internal/model"
)

func TestSoakWindowRestartsAfterProbeCalibration(t *testing.T) {
	window, err := NewWindow("V-07", 5, 1.0)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Unix(1000, 0)
	for i, value := range []float64{101.0, 101.2, 101.1} {
		window.Add(model.Sample{ProbeID: "P-P07", VesselID: "V-07", Kind: model.ProbePressure, Value: value, CalibrationEpoch: 1, ObservedAt: base.Add(time.Duration(i) * time.Second)})
	}
	for i, value := range []float64{101.3, 101.2} {
		window.Add(model.Sample{ProbeID: "P-P07", VesselID: "V-07", Kind: model.ProbePressure, Value: value, CalibrationEpoch: 2, ObservedAt: base.Add(time.Duration(i+3) * time.Second)})
	}
	if window.Stable() {
		t.Fatal("two current-epoch samples completed the five-sample window")
	}
	samples := window.Samples()
	if len(samples) != 2 {
		t.Fatalf("window retained %d samples", len(samples))
	}
	for _, sample := range samples {
		if sample.CalibrationEpoch != 2 {
			t.Fatalf("retained epoch %d", sample.CalibrationEpoch)
		}
	}
}
