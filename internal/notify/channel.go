package notify

import (
	"context"
	"errors"
	"sync"

	"github.com/wyw14/cry-98/internal/model"
)

type Channel interface {
	Name() string
	Required() bool
	Send(context.Context, model.Alarm) error
}

type LocalChannel struct {
	mu       sync.Mutex
	name     string
	required bool
}

func NewLocalChannel(name string, required bool) (*LocalChannel, error) {
	if name == "" {
		return nil, errors.New("notification channel name is required")
	}
	return &LocalChannel{name: name, required: required}, nil
}

func (c *LocalChannel) Name() string   { return c.name }
func (c *LocalChannel) Required() bool { return c.required }

func (c *LocalChannel) Send(ctx context.Context, alarm model.Alarm) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = alarm
	return nil
}
