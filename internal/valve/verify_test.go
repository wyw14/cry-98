package valve_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/wyw14/cry-98/internal/fill"
	"github.com/wyw14/cry-98/internal/model"
	"github.com/wyw14/cry-98/internal/valve"
)

type lossyTransport struct {
	executed map[uuid.UUID]model.ValveReceipt
	pulses   int
	first    bool
	clock    func() time.Time
}

func (t *lossyTransport) Execute(_ context.Context, command model.ValveCommand) (model.ValveReceipt, error) {
	if receipt, ok := t.executed[command.ID]; ok {
		receipt.Duplicate = true
		return receipt, nil
	}
	t.pulses++
	receipt := model.ValveReceipt{CommandID: command.ID, Executed: true, ReceivedAt: t.clock()}
	t.executed[command.ID] = receipt
	if t.first {
		t.first = false
		return model.ValveReceipt{}, errors.New("response timeout")
	}
	return receipt, nil
}

func TestValveRetryReusesLogicalCommandIdentity(t *testing.T) {
	now := time.Now
	transport := &lossyTransport{executed: make(map[uuid.UUID]model.ValveReceipt), first: true, clock: now}
	controller, err := valve.NewController(transport, now)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := fill.NewRetryService(valve.NewCommandService(controller, now), fill.RetryPolicy{Attempts: 2})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := retry.Open(context.Background(), uuid.New(), 7, "VALVE-A")
	if err != nil {
		t.Fatal(err)
	}
	if !receipt.Duplicate {
		t.Fatal("retry did not recover the existing device result")
	}
	if transport.pulses != 1 {
		t.Fatalf("physical pulses=%d", transport.pulses)
	}
}
