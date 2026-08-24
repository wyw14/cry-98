package valve

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/wyw14/cry-98/internal/model"
)

type CommandService struct {
	controller *Controller
	now        func() time.Time
}

func NewCommandService(controller *Controller, now func() time.Time) *CommandService {
	if now == nil {
		now = time.Now
	}
	return &CommandService{controller: controller, now: now}
}

func (s *CommandService) Send(ctx context.Context, commandID, sessionID uuid.UUID, circuitGeneration uint64, valveID string, action model.ValveAction) (model.ValveReceipt, error) {
	command, err := model.NewValveCommand(
		commandID,
		sessionID,
		circuitGeneration,
		valveID,
		action,
		s.now(),
	)
	if err != nil {
		return model.ValveReceipt{}, err
	}
	return s.controller.Execute(ctx, command)
}

func (s *CommandService) Open(ctx context.Context, commandID, sessionID uuid.UUID, circuitGeneration uint64, valveID string) (model.ValveReceipt, error) {
	return s.Send(ctx, commandID, sessionID, circuitGeneration, valveID, model.ValveOpen)
}

func (s *CommandService) Close(ctx context.Context, commandID, sessionID uuid.UUID, circuitGeneration uint64, valveID string) (model.ValveReceipt, error) {
	return s.Send(ctx, commandID, sessionID, circuitGeneration, valveID, model.ValveClose)
}
