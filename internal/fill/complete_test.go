package fill

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wyw14/cry-98/internal/circuit"
	"github.com/wyw14/cry-98/internal/model"
	"github.com/wyw14/cry-98/internal/pressure"
	"github.com/wyw14/cry-98/internal/valve"
	"github.com/wyw14/cry-98/internal/vessel"
)

// failingAppender rejects every Append with a fixed error, simulating the
// "append soak result failed, access denied" journal failure.
type failingAppender struct{ err error }

func (f failingAppender) Append(model.Event) (model.Event, error) {
	return model.Event{}, f.err
}

// recordingAppender accepts every Append and remembers the events it observed.
type recordingAppender struct{ events []model.Event }

func (r *recordingAppender) Append(event model.Event) (model.Event, error) {
	r.events = append(r.events, event)
	return event, nil
}

func testNow() time.Time { return time.Unix(1_700_000_000, 0) }

func newCompletionFixture(t *testing.T) (*Coordinator, *pressure.Monitor, *vessel.StateService, *circuit.Manager, string) {
	t.Helper()
	now := testNow

	registry := vessel.NewRegistry()
	vesselState, err := model.NewVessel("V-18", "Cryogenic vessel 18", "east", now())
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(vesselState); err != nil {
		t.Fatal(err)
	}
	state := vessel.NewStateService(registry, now)

	monitor := pressure.NewMonitor(now)
	if err := monitor.AddVessel("V-18", 5, 1.5); err != nil {
		t.Fatal(err)
	}

	circuitRegistry := circuit.NewRegistry(circuit.Circuit{
		ID:      "C-A",
		Zone:    "east",
		ValveID: "VALVE-A",
		Enabled: true,
	})
	manager := circuit.NewManager("C-A")

	transport := valve.NewMemoryTransport(now)
	controller, err := valve.NewController(transport, now)
	if err != nil {
		t.Fatal(err)
	}
	commands := valve.NewCommandService(controller, now)
	routes := valve.NewRouteService(commands)

	coordinator := NewCoordinator(manager, circuitRegistry, routes, state, now)
	return coordinator, monitor, state, manager, "C-A"
}

func stablePressure(t *testing.T, monitor *pressure.Monitor, vesselID string) {
	t.Helper()
	probe, err := model.NewProbe("P-P18", model.ProbePressure, vesselID, time.Unix(1_700_000_000, 0))
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 5; index++ {
		if err := monitor.Add(probe.NewSample(100.0, time.Unix(1_700_000_000+int64(index), 0))); err != nil {
			t.Fatal(err)
		}
	}
	result, err := monitor.Evaluate(vesselID)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Stable {
		t.Fatal("pressure window was not stable")
	}
}

func TestCompleteKeepsSoakingWhenEvidenceNotPersisted(t *testing.T) {
	coordinator, monitor, state, manager, circuitID := newCompletionFixture(t)
	ctx := context.Background()

	session, err := coordinator.StartManual(ctx, "V-18", 2.1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := coordinator.BeginSoak(session.ID); err != nil {
		t.Fatal(err)
	}
	stablePressure(t, monitor, "V-18")

	completion := NewCompletionService(coordinator, monitor, failingAppender{err: errors.New("access denied")}, testNow)
	if _, err := completion.Complete(ctx, session.ID); err == nil {
		t.Fatal("completion succeeded despite journal append failure")
	}

	// The completed status must never be published while the proof is missing.
	current, ok := coordinator.Get(session.ID)
	if !ok {
		t.Fatal("fill session was dropped before completion was durable")
	}
	if current.State != model.FillSoaking {
		t.Fatalf("state=%s, want soaking so completion can be retried", current.State)
	}

	// The shared supply circuit must stay reserved so a later device cannot grab it.
	if _, occupied := manager.Get(circuitID); !occupied {
		t.Fatal("circuit was released before the soak evidence was saved")
	}

	// The vessel must still report an active fill, not be recycled back to ready.
	v, ok := state.Registry().Get("V-18")
	if !ok {
		t.Fatal("vessel not found")
	}
	if v.Status != model.VesselFilling {
		t.Fatalf("vessel status=%s, want filling", v.Status)
	}
}

func TestCompletePersistsEvidenceBeforeReleasingCircuit(t *testing.T) {
	coordinator, monitor, state, manager, circuitID := newCompletionFixture(t)
	ctx := context.Background()

	session, err := coordinator.StartManual(ctx, "V-18", 2.1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := coordinator.BeginSoak(session.ID); err != nil {
		t.Fatal(err)
	}
	stablePressure(t, monitor, "V-18")

	appender := &recordingAppender{}
	completion := NewCompletionService(coordinator, monitor, appender, testNow)
	completed, err := completion.Complete(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if completed.State != model.FillCompleted {
		t.Fatalf("state=%s, want completed", completed.State)
	}
	if len(appender.events) != 1 || appender.events[0].Kind != "fill.soak_verified" {
		t.Fatalf("events=%v, want one fill.soak_verified", appender.events)
	}
	if _, occupied := manager.Get(circuitID); occupied {
		t.Fatal("circuit was not released after durable completion")
	}
	v, ok := state.Registry().Get("V-18")
	if !ok {
		t.Fatal("vessel not found")
	}
	if v.Status != model.VesselReady {
		t.Fatalf("vessel status=%s, want ready", v.Status)
	}
}
