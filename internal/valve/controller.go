package valve

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wyw14/cry-98/internal/model"
)

type Transport interface {
	Execute(context.Context, model.ValveCommand) (model.ValveReceipt, error)
}

type MemoryTransport struct {
	mu       sync.Mutex
	executed map[uuid.UUID]model.ValveReceipt
	now      func() time.Time
}

func NewMemoryTransport(now func() time.Time) *MemoryTransport {
	if now == nil {
		now = time.Now
	}
	return &MemoryTransport{
		executed: make(map[uuid.UUID]model.ValveReceipt),
		now:      now,
	}
}

func (t *MemoryTransport) Execute(ctx context.Context, command model.ValveCommand) (model.ValveReceipt, error) {
	if err := ctx.Err(); err != nil {
		return model.ValveReceipt{}, err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if receipt, ok := t.executed[command.ID]; ok {
		receipt.Duplicate = true
		return receipt, nil
	}
	receipt := model.ValveReceipt{
		CommandID:  command.ID,
		Executed:   true,
		Message:    "accepted",
		ReceivedAt: t.now().UTC(),
	}
	t.executed[command.ID] = receipt
	return receipt, nil
}

type Controller struct {
	transport Transport
	now       func() time.Time
}

func NewController(transport Transport, now func() time.Time) (*Controller, error) {
	if transport == nil {
		return nil, errors.New("valve transport is required")
	}
	if now == nil {
		now = time.Now
	}
	return &Controller{transport: transport, now: now}, nil
}

func (c *Controller) Execute(ctx context.Context, command model.ValveCommand) (model.ValveReceipt, error) {
	receipt, err := c.transport.Execute(ctx, command)
	if err != nil {
		return model.ValveReceipt{}, err
	}
	if err := receipt.Validate(command); err != nil {
		return model.ValveReceipt{}, err
	}
	return receipt, nil
}
