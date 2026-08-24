package fill

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wyw14/cry-98/internal/circuit"
	"github.com/wyw14/cry-98/internal/model"
	"github.com/wyw14/cry-98/internal/valve"
	"github.com/wyw14/cry-98/internal/vessel"
)

// countingTransport is a valve transport that records how many distinct open
// pulses reach the valve, so concurrent-start scenarios can assert that the
// supply valve is opened at most once per vessel.
type countingTransport struct {
	mu       sync.Mutex
	executed map[string]model.ValveReceipt
	opens    atomic.Int64
	now      func() time.Time
}

func (t *countingTransport) Execute(ctx context.Context, command model.ValveCommand) (model.ValveReceipt, error) {
	if err := ctx.Err(); err != nil {
		return model.ValveReceipt{}, err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	key := command.ID.String()
	if receipt, ok := t.executed[key]; ok {
		receipt.Duplicate = true
		return receipt, nil
	}
	if command.Action == model.ValveOpen {
		t.opens.Add(1)
	}
	receipt := model.ValveReceipt{
		CommandID:  command.ID,
		Executed:   true,
		Message:    "accepted",
		ReceivedAt: t.now().UTC(),
	}
	t.executed[key] = receipt
	return receipt, nil
}

// TestStartConcurrentAutomaticAndManualSharesSingleSlot reproduces the
// V-12 overrun: a low-level automatic refill and an operator manual refill
// fired near-simultaneously both returned "started" and the supply valve
// received two open pulses. The fix makes both entries contend for a single
// session slot, so exactly one Start succeeds and the other observes
// ErrActiveSession with the valve opened only once.
func TestStartConcurrentAutomaticAndManualSharesSingleSlot(t *testing.T) {
	now := func() time.Time { return time.Unix(1000, 0) }
	vesselRegistry := vessel.NewRegistry()
	v, err := model.NewVessel("V-12", "Vessel 12", "south", now())
	if err != nil {
		t.Fatal(err)
	}
	if err := vesselRegistry.Register(v); err != nil {
		t.Fatal(err)
	}
	vessels := vessel.NewStateService(vesselRegistry, now)
	circuitRegistry := circuit.NewRegistry(
		circuit.Circuit{ID: "C-A", Zone: "north", ValveID: "VALVE-A", Enabled: true},
		circuit.Circuit{ID: "C-B", Zone: "south", ValveID: "VALVE-B", Enabled: true},
	)
	circuitManager := circuit.NewManager("C-A", "C-B")
	transport := &countingTransport{executed: make(map[string]model.ValveReceipt), now: now}
	controller, err := valve.NewController(transport, now)
	if err != nil {
		t.Fatal(err)
	}
	commands := valve.NewCommandService(controller, now)
	routes := valve.NewRouteService(commands)
	coordinator := NewCoordinator(circuitManager, circuitRegistry, routes, vessels, now)
	retry, err := NewRetryService(commands, RetryPolicy{Attempts: 1, Delay: 0})
	if err != nil {
		t.Fatal(err)
	}
	coordinator.SetRetryService(retry)

	var wg sync.WaitGroup
	var autoSession, manualSession model.FillSession
	var autoErr, manualErr error
	startBarrier := make(chan struct{})

	run := func(fn func(context.Context, string, float64) (model.FillSession, error), out *model.FillSession, errOut *error) {
		defer wg.Done()
		<-startBarrier
		session, err := fn(context.Background(), "V-12", 2.1)
		*out = session
		*errOut = err
	}

	wg.Add(2)
	go run(coordinator.StartAutomatic, &autoSession, &autoErr)
	go run(coordinator.StartManual, &manualSession, &manualErr)
	close(startBarrier)
	wg.Wait()

	succeeded := 0
	var winner model.FillSession
	for _, res := range []struct {
		session model.FillSession
		err     error
	}{
		{autoSession, autoErr},
		{manualSession, manualErr},
	} {
		switch {
		case res.err == nil:
			succeeded++
			winner = res.session
		case !errors.Is(res.err, ErrActiveSession):
			t.Fatalf("unexpected error %v", res.err)
		}
	}

	if succeeded != 1 {
		t.Fatalf("expected exactly one successful start, got %d (autoErr=%v, manualErr=%v, auto=%v, manual=%v)",
			succeeded, autoErr, manualErr, autoSession.ID, manualSession.ID)
	}

	// The original symptom was two open pulses to the supply valve.
	if got := transport.opens.Load(); got != 1 {
		t.Fatalf("expected one valve open pulse, got %d", got)
	}

	if sessions := coordinator.List(); len(sessions) != 1 {
		t.Fatalf("expected one fill session, got %d", len(sessions))
	}

	// The winning session advances normally, confirming the slot stays consistent.
	if _, err := coordinator.BeginSoak(winner.ID); err != nil {
		t.Fatalf("begin soak: %v", err)
	}
}
