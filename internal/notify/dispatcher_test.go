package notify

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wyw14/cry-98/internal/model"
)

// stubChannel is a controllable notification channel used to reproduce the
// production scenario where the siren relay is unavailable while the
// duty-terminal still receives the message.
type stubChannel struct {
	name     string
	required bool
	sendErr  error
}

func (c *stubChannel) Name() string    { return c.name }
func (c *stubChannel) Required() bool  { return c.required }
func (c *stubChannel) Send(_ context.Context, _ model.Alarm) error {
	return c.sendErr
}

func fixedNow() time.Time { return time.Unix(100, 0) }

func TestDispatchSurfacesRequiredChannelFailure(t *testing.T) {
	siren := &stubChannel{name: "siren", required: true, sendErr: errors.New("siren relay unavailable")}
	terminal := &stubChannel{name: "duty-terminal", required: true}
	dispatcher, err := NewDispatcher(fixedNow, siren, terminal)
	if err != nil {
		t.Fatal(err)
	}

	alarm, err := model.NewAlarm("V-03", model.AlarmLeak, "nitrogen leak detected", fixedNow())
	if err != nil {
		t.Fatal(err)
	}

	results, dispatchErr := dispatcher.Dispatch(context.Background(), alarm)
	if dispatchErr == nil {
		t.Fatal("dispatch did not surface the failed required channel")
	}

	// The alarm as a whole must not be marked delivered when a required
	// channel failed; it must remain partial so it stays eligible for retry.
	alarm = alarm.WithChannels(results, fixedNow())
	if alarm.State != model.AlarmPartial {
		t.Fatalf("state=%s, want %s", alarm.State, model.AlarmPartial)
	}

	var sirenResult *model.ChannelResult
	for i := range alarm.Channels {
		if alarm.Channels[i].Channel == "siren" {
			sirenResult = &alarm.Channels[i]
			break
		}
	}
	if sirenResult == nil || sirenResult.Delivered {
		t.Fatalf("expected siren channel to remain undelivered, got %+v", sirenResult)
	}
}

func TestDispatchSucceedsWhenOnlyOptionalChannelFails(t *testing.T) {
	optional := &stubChannel{name: "audit-log", required: false, sendErr: errors.New("audit backlog")}
	dispatcher, err := NewDispatcher(fixedNow, optional)
	if err != nil {
		t.Fatal(err)
	}

	alarm, err := model.NewAlarm("V-03", model.AlarmLeak, "nitrogen leak detected", fixedNow())
	if err != nil {
		t.Fatal(err)
	}

	results, dispatchErr := dispatcher.Dispatch(context.Background(), alarm)
	if dispatchErr != nil {
		t.Fatalf("optional channel failure blocked delivery: %v", dispatchErr)
	}
	alarm = alarm.WithChannels(results, fixedNow())
	if alarm.State != model.AlarmDelivered {
		t.Fatalf("state=%s, want %s", alarm.State, model.AlarmDelivered)
	}
}

// TestRetryKeepsFailedRequiredChannelPending locks in the retry half of the
// fix: after a required channel fails, RunDue must keep the alarm in the
// retry queue until that channel finally succeeds.
func TestRetryKeepsFailedRequiredChannelPending(t *testing.T) {
	clock := &steppingClock{t: time.Unix(100, 0), step: time.Second}
	siren := &stubChannel{name: "siren", required: true, sendErr: errors.New("siren relay unavailable")}
	terminal := &stubChannel{name: "duty-terminal", required: true}
	dispatcher, err := NewDispatcher(clock.now, siren, terminal)
	if err != nil {
		t.Fatal(err)
	}

	alarm, err := model.NewAlarm("V-03", model.AlarmLeak, "nitrogen leak detected", clock.now())
	if err != nil {
		t.Fatal(err)
	}

	results, dispatchErr := dispatcher.Dispatch(context.Background(), alarm)
	if dispatchErr == nil {
		t.Fatal("expected dispatch to surface the failed required channel")
	}
	alarm = alarm.WithChannels(results, clock.now())

	queue := NewRetryQueue(time.Second, clock.now)
	queue.Schedule(alarm)
	if len(queue.Pending()) != 1 {
		t.Fatalf("pending=%d, want 1", len(queue.Pending()))
	}

	// Advance the clock past the scheduled retry so RunDue picks it up.
	clock.advance(time.Second)

	// First retry: siren is still unavailable, the alarm must stay queued.
	if errs := queue.RunDue(context.Background(), dispatcher); len(errs) == 0 {
		t.Fatal("expected RunDue to report the still-failing required channel")
	}
	if len(queue.Pending()) != 1 {
		t.Fatalf("alarm dropped from retry queue while a required channel still failed, pending=%d", len(queue.Pending()))
	}

	// Second retry: siren relay recovers, the alarm is fully delivered and
	// removed from the retry queue.
	clock.advance(time.Second)
	siren.sendErr = nil
	if errs := queue.RunDue(context.Background(), dispatcher); len(errs) != 0 {
		t.Fatalf("unexpected retry errors: %v", errs)
	}
	if len(queue.Pending()) != 0 {
		t.Fatalf("pending=%d, want 0 after all required channels delivered", len(queue.Pending()))
	}
}

// steppingClock is a deterministic clock that advances only when asked, so a
// scheduled retry does not fire until the test moves time forward.
type steppingClock struct {
	t    time.Time
	step time.Duration
}

func (c *steppingClock) now() time.Time { return c.t }

func (c *steppingClock) advance(d time.Duration) { c.t = c.t.Add(d) }
