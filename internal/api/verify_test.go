package api_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/wyw14/cry-98/internal/api"
	"github.com/wyw14/cry-98/internal/circuit"
	"github.com/wyw14/cry-98/internal/fill"
	"github.com/wyw14/cry-98/internal/model"
	"github.com/wyw14/cry-98/internal/valve"
	"github.com/wyw14/cry-98/internal/vessel"
)

func TestAutoAndManualFillCreateOneSession(t *testing.T) {
	now := time.Now
	vessels := vessel.NewRegistry()
	v, err := model.NewVessel("V-12", "Vessel 12", "south", now())
	if err != nil {
		t.Fatal(err)
	}
	if err := vessels.Register(v); err != nil {
		t.Fatal(err)
	}
	state := vessel.NewStateService(vessels, now)
	circuits := circuit.NewRegistry(
		circuit.Circuit{ID: "C-A", ValveID: "VA", Enabled: true},
		circuit.Circuit{ID: "C-B", ValveID: "VB", Enabled: true},
	)
	reservations := circuit.NewManager("C-A", "C-B")
	controller, err := valve.NewController(valve.NewMemoryTransport(now), now)
	if err != nil {
		t.Fatal(err)
	}
	routes := valve.NewRouteService(valve.NewCommandService(controller, now))
	coordinator := fill.NewCoordinator(reservations, circuits, routes, state, now)
	service, err := fill.NewService(coordinator, nil, nil, 2.1)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(api.NewServer(api.Dependencies{Fills: service, Now: now}).Handler())
	defer server.Close()

	start := make(chan struct{})
	statuses := make(chan int, 2)
	var wait sync.WaitGroup
	wait.Add(1)
	go func() {
		defer wait.Done()
		<-start
		response, requestErr := http.Post(server.URL+"/api/fill-sessions", "application/json", bytes.NewBufferString(`{"vessel_id":"V-12","liters":2.1}`))
		if requestErr != nil {
			statuses <- 0
			return
		}
		defer response.Body.Close()
		statuses <- response.StatusCode
	}()
	wait.Add(1)
	go func() {
		defer wait.Done()
		<-start
		_, startErr := service.OnLowLevel(context.Background(), "V-12", now())
		if startErr == nil {
			statuses <- http.StatusCreated
		} else {
			statuses <- http.StatusConflict
		}
	}()
	close(start)
	wait.Wait()
	close(statuses)
	created, rejected := 0, 0
	for status := range statuses {
		if status == http.StatusCreated {
			created++
		}
		if status == http.StatusConflict {
			rejected++
		}
	}
	if created != 1 || rejected != 1 {
		t.Fatalf("created=%d rejected=%d", created, rejected)
	}
	if sessions := service.List(); len(sessions) != 1 {
		t.Fatalf("sessions=%d", len(sessions))
	}
}
