package api_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/wyw14/cry-98/internal/api"
	"github.com/wyw14/cry-98/internal/model"
	"github.com/wyw14/cry-98/internal/probe"
	"github.com/wyw14/cry-98/internal/telemetry"
	"github.com/wyw14/cry-98/internal/vessel"
)

func TestTelemetryStreamUsesStableSnapshot(t *testing.T) {
	now := time.Now
	vessels := vessel.NewRegistry()
	v, _ := model.NewVessel("V-03", "Vessel 03", "north", now())
	if err := vessels.Register(v); err != nil {
		t.Fatal(err)
	}
	states := vessel.NewStateService(vessels, now)
	probes := probe.NewRegistry(64)
	p, _ := model.NewProbe("P-L03", model.ProbeLevel, "V-03", now())
	if err := probes.Register(p); err != nil {
		t.Fatal(err)
	}
	snapshots := telemetry.NewSnapshotStore()
	snapshots.Replace(vessels.List(), now())
	ingestor := telemetry.NewIngestor(probes, states, snapshots)
	stable := ingestor.Snapshot()
	if err := ingestor.Ingest(p.NewSample(20, now())); err != nil {
		t.Fatal(err)
	}
	if stable.Vessels["V-03"].LevelMM != 0 {
		t.Fatal("previous snapshot changed after a write")
	}
	server := httptest.NewServer(api.NewServer(api.Dependencies{Telemetry: ingestor, Now: now}).Handler())
	defer server.Close()
	var wait sync.WaitGroup
	for worker := range 8 {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			for sample := range 100 {
				_ = ingestor.Ingest(p.NewSample(float64(worker*100+sample), now()))
			}
		}(worker)
	}
	for range 20 {
		response, err := http.Get(server.URL + "/api/vessels/stream")
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("status=%d", response.StatusCode)
		}
	}
	wait.Wait()
}
