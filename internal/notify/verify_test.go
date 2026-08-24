package notify_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wyw14/cry-98/internal/alarm"
	"github.com/wyw14/cry-98/internal/journal"
	"github.com/wyw14/cry-98/internal/model"
	"github.com/wyw14/cry-98/internal/notify"
)

type testChannel struct {
	name     string
	required bool
	err      error
	sent     int
}

func (c *testChannel) Name() string   { return c.name }
func (c *testChannel) Required() bool { return c.required }
func (c *testChannel) Send(context.Context, model.Alarm) error {
	if c.err == nil {
		c.sent++
	}
	return c.err
}

func TestAlarmRetainsFailedRequiredChannel(t *testing.T) {
	now := time.Now
	siren := &testChannel{name: "siren", required: true, err: errors.New("siren relay unavailable")}
	terminal := &testChannel{name: "terminal", required: true}
	dispatcher, err := notify.NewDispatcher(now, siren, terminal)
	if err != nil {
		t.Fatal(err)
	}
	queue := notify.NewRetryQueue(time.Second, now)
	store, err := journal.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	delivery, err := alarm.NewDeliveryService(dispatcher, queue, store, now)
	if err != nil {
		t.Fatal(err)
	}
	service := alarm.NewService(delivery, now)
	result, err := service.Raise(context.Background(), "V-03", model.AlarmLeak, "nitrogen leak")
	if err == nil {
		t.Fatal("required siren failure was hidden")
	}
	if result.State != model.AlarmPartial {
		t.Fatalf("state=%s", result.State)
	}
	if len(queue.Pending()) != 1 {
		t.Fatalf("pending retries=%d", len(queue.Pending()))
	}
	if siren.sent != 0 || terminal.sent != 1 {
		t.Fatal("unexpected channel delivery counts")
	}
}
