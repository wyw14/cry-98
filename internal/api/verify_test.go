package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wyw14/cry-98/internal/api"
	"github.com/wyw14/cry-98/internal/circuit"
	"github.com/wyw14/cry-98/internal/fill"
	"github.com/wyw14/cry-98/internal/model"
	"github.com/wyw14/cry-98/internal/pressure"
	"github.com/wyw14/cry-98/internal/valve"
	"github.com/wyw14/cry-98/internal/vessel"
)

type failingAppender struct{}

func (failingAppender) Append(event model.Event) (model.Event, error) {
	if event.Kind == "fill.soak_verified" {
		return model.Event{}, errors.New("access denied")
	}
	return event, nil
}

func TestFillCompletesAfterSoakEvidencePersists(t *testing.T) {
	now := time.Now
	vessels := vessel.NewRegistry()
	v, _ := model.NewVessel("V-18", "Vessel 18", "east", now())
	if err := vessels.Register(v); err != nil {
		t.Fatal(err)
	}
	state := vessel.NewStateService(vessels, now)
	circuits := circuit.NewRegistry(circuit.Circuit{ID: "C-A", ValveID: "VA", Enabled: true})
	reservations := circuit.NewManager("C-A")
	transport := valve.NewMemoryTransport(now)
	controller, _ := valve.NewController(transport, now)
	routes := valve.NewRouteService(valve.NewCommandService(controller, now))
	coordinator := fill.NewCoordinator(reservations, circuits, routes, state, now)
	pressureMonitor := pressure.NewMonitor(now)
	if err := pressureMonitor.AddVessel("V-18", 2, 1); err != nil {
		t.Fatal(err)
	}
	completion := fill.NewCompletionService(coordinator, pressureMonitor, failingAppender{}, now)
	service, _ := fill.NewService(coordinator, completion, fill.NewAbortService(coordinator, now), 2.1)
	server := httptest.NewServer(api.NewServer(api.Dependencies{Fills: service, Now: now}).Handler())
	defer server.Close()
	response, err := http.Post(server.URL+"/api/fill-sessions", "application/json", bytes.NewBufferString(`{"vessel_id":"V-18","liters":2.1}`))
	if err != nil {
		t.Fatal(err)
	}
	var session model.FillSession
	if err := json.NewDecoder(response.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	response, err = http.Post(server.URL+"/api/fill-sessions/"+session.ID.String()+"/soak", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	base := now()
	for index, value := range []float64{101, 101.2} {
		if err := pressureMonitor.Add(model.Sample{ProbeID: "P-P18", VesselID: "V-18", Kind: model.ProbePressure, Value: value, CalibrationEpoch: 1, ObservedAt: base.Add(time.Duration(index) * time.Second)}); err != nil {
			t.Fatal(err)
		}
	}
	response, err = http.Post(server.URL+"/api/fill-sessions/"+session.ID.String()+"/complete", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("status=%d", response.StatusCode)
	}
	current, _ := service.Get(session.ID)
	if current.State != model.FillSoaking {
		t.Fatalf("state=%s", current.State)
	}
	if _, occupied := reservations.Get("C-A"); !occupied {
		t.Fatal("circuit was released")
	}
}
