package fill_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wyw14/cry-98/internal/circuit"
	"github.com/wyw14/cry-98/internal/fill"
	"github.com/wyw14/cry-98/internal/model"
	"github.com/wyw14/cry-98/internal/valve"
	"github.com/wyw14/cry-98/internal/vessel"
)

type closeFailureTransport struct {
	now time.Time
}

func (t closeFailureTransport) Execute(_ context.Context, command model.ValveCommand) (model.ValveReceipt, error) {
	if command.Action == model.ValveClose {
		return model.ValveReceipt{}, errors.New("close inlet timeout")
	}
	return model.ValveReceipt{CommandID: command.ID, Executed: true, ReceivedAt: t.now}, nil
}

func TestFailedValveCloseRetainsCircuitReservation(t *testing.T) {
	now := time.Now
	vessels := vessel.NewRegistry()
	for _, id := range []string{"V-09", "V-12"} {
		item, _ := model.NewVessel(id, id, "south", now())
		if err := vessels.Register(item); err != nil {
			t.Fatal(err)
		}
	}
	states := vessel.NewStateService(vessels, now)
	circuits := circuit.NewRegistry(circuit.Circuit{ID: "C-A", ValveID: "VA", Enabled: true})
	reservations := circuit.NewManager("C-A")
	controller, _ := valve.NewController(closeFailureTransport{now: now()}, now)
	routes := valve.NewRouteService(valve.NewCommandService(controller, now))
	coordinator := fill.NewCoordinator(reservations, circuits, routes, states, now)
	service, _ := fill.NewService(coordinator, nil, fill.NewAbortService(coordinator, now), 2.1)
	session, err := service.StartManual(context.Background(), "V-09", 2.1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Abort(context.Background(), session.ID, "pressure fluctuation"); err == nil {
		t.Fatal("close failure was hidden")
	}
	reservation, occupied := reservations.Get("C-A")
	if !occupied || !reservation.Faulted {
		t.Fatalf("occupied=%v faulted=%v", occupied, reservation.Faulted)
	}
	if _, err := service.StartManual(context.Background(), "V-12", 2.1); !errors.Is(err, circuit.ErrCircuitBusy) {
		t.Fatalf("second start error=%v", err)
	}
}
