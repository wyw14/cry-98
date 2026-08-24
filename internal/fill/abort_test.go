package fill

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wyw14/cry-98/internal/circuit"
	"github.com/wyw14/cry-98/internal/model"
	"github.com/wyw14/cry-98/internal/valve"
	"github.com/wyw14/cry-98/internal/vessel"
)

// closeTimeoutTransport accepts open commands but fails close commands,
// simulating a "close inlet timeout" where the inlet valve state is never
// confirmed.
type closeTimeoutTransport struct {
	now time.Time
}

func (t *closeTimeoutTransport) Execute(ctx context.Context, command model.ValveCommand) (model.ValveReceipt, error) {
	if err := ctx.Err(); err != nil {
		return model.ValveReceipt{}, err
	}
	if command.Action == model.ValveClose {
		return model.ValveReceipt{CommandID: command.ID, Executed: false, Message: "close inlet timeout", ReceivedAt: t.now.UTC()}, errors.New("close inlet timeout")
	}
	return model.ValveReceipt{CommandID: command.ID, Executed: true, Message: "accepted", ReceivedAt: t.now.UTC()}, nil
}

func newAbortFixture(t *testing.T, now time.Time) (*AbortService, *Coordinator, *vessel.Registry) {
	t.Helper()
	nowFunc := func() time.Time { return now }

	vesselRegistry := vessel.NewRegistry()
	for _, seed := range []struct{ id, name, zone string }{
		{"V-09", "Cryogenic vessel 09", "south"},
		{"V-12", "Cryogenic vessel 12", "south"},
	} {
		v, err := model.NewVessel(seed.id, seed.name, seed.zone, now)
		if err != nil {
			t.Fatal(err)
		}
		if err := vesselRegistry.Register(v); err != nil {
			t.Fatal(err)
		}
	}
	vesselState := vessel.NewStateService(vesselRegistry, nowFunc)

	// A single shared circuit: the only path for any fill. This matches the
	// "shared circuit" topology where a premature release lets a second vessel
	// re-enter the same still-open branch.
	circuitRegistry := circuit.NewRegistry(
		circuit.Circuit{ID: "C-A", Zone: "south", ValveID: "VALVE-A", Enabled: true},
	)
	circuitManager := circuit.NewManager("C-A")

	transport := &closeTimeoutTransport{now: now}
	controller, err := valve.NewController(transport, nowFunc)
	if err != nil {
		t.Fatal(err)
	}
	commands := valve.NewCommandService(controller, nowFunc)
	routes := valve.NewRouteService(commands)

	coordinator := NewCoordinator(circuitManager, circuitRegistry, routes, vesselState, nowFunc)
	abort := NewAbortService(coordinator, nowFunc)
	return abort, coordinator, vesselRegistry
}

// TestAbortCloseFailureRetainsCircuit asserts the failure-cleanup invariant:
// when the inlet valve cannot be closed, the shared circuit must stay reserved
// and faulted. A subsequent session must not be able to claim it while the
// isolation state is unconfirmed, and explicit recovery remains the only way
// out.
func TestAbortCloseFailureRetainsCircuit(t *testing.T) {
	now := time.Unix(1000, 0)
	abort, coordinator, vesselRegistry := newAbortFixture(t, now)

	ctx := context.Background()
	first, err := coordinator.Start(ctx, "V-09", model.FillAutomatic, 2.1)
	if err != nil {
		t.Fatalf("start first fill: %v", err)
	}

	failed, abortErr := abort.Abort(ctx, first.ID, "pressure surge")
	if abortErr == nil {
		t.Fatal("expected abort to report the close failure")
	}
	if failed.State != model.FillFailed {
		t.Fatalf("expected failed session, got %s", failed.State)
	}
	if failed.Failure == "" {
		t.Fatal("expected failure reason to be recorded")
	}

	// The circuit stays reserved AND faulted, so neither Release nor a new
	// reservation can reach it while isolation is unconfirmed.
	reservation, occupied := coordinator.circuits.Get(first.CircuitID)
	if !occupied {
		t.Fatal("circuit was released while the inlet close was unconfirmed")
	}
	if !reservation.Faulted {
		t.Fatal("circuit was not retained as faulted after the close failure")
	}
	if err := coordinator.circuits.Release(first.CircuitID, first.ID); err == nil {
		t.Fatal("faulted circuit must not be releasable without explicit recovery")
	}

	// A second session (V-12) must not enter the shared circuit through the
	// dangerous intermediate state.
	if _, err := coordinator.Start(ctx, "V-12", model.FillAutomatic, 2.1); err == nil {
		t.Fatal("subsequent fill started on a circuit whose isolation was not confirmed")
	}

	// The faulted vessel itself also stays locked out of new fills because the
	// abort path did not clear its active/vessel-filling state.
	if _, active := coordinator.active["V-09"]; !active {
		t.Fatal("faulted vessel was released from active tracking before recovery")
	}
	v09, _ := vesselRegistry.Get("V-09")
	if v09.Status != model.VesselFilling {
		t.Fatalf("faulted vessel status = %s, want filling", v09.Status)
	}

	// The only safe exit is explicit recovery, which clears the reservation and
	// the vessel's active state together.
	if err := abort.RecoverIsolation(first.ID); err != nil {
		t.Fatalf("recover isolation: %v", err)
	}
	if _, stillOccupied := coordinator.circuits.Get(first.CircuitID); stillOccupied {
		t.Fatal("recovered circuit remained occupied")
	}

	// After recovery the shared circuit is safe to use again.
	second, err := coordinator.Start(ctx, "V-12", model.FillAutomatic, 2.1)
	if err != nil {
		t.Fatalf("start fill after recovery: %v", err)
	}
	if second.CircuitID != first.CircuitID {
		t.Fatalf("post-recovery fill used %s, want %s", second.CircuitID, first.CircuitID)
	}
}
