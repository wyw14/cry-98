package model

import (
	"testing"
	"time"
)

func TestFillSessionTransitionsThroughOperationalStates(t *testing.T) {
	now := time.Unix(100, 0)
	session, err := NewFillSession("V-01", FillManual, 1, 2.1, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range []FillState{FillPrechecking, FillFilling, FillSoaking, FillVerified, FillCompleted} {
		session, err = session.Transition(state, now.Add(time.Second))
		if err != nil {
			t.Fatalf("transition to %s failed: %v", state, err)
		}
	}
	if session.Active() {
		t.Fatal("completed fill remained active")
	}
}

func TestAlarmChannelSummaryPreservesRequiredFailure(t *testing.T) {
	now := time.Unix(100, 0)
	alarm, err := NewAlarm("V-01", AlarmLeak, "leak", now)
	if err != nil {
		t.Fatal(err)
	}
	alarm = alarm.WithChannels([]ChannelResult{
		{Channel: "terminal", Required: true, Delivered: true},
		{Channel: "siren", Required: true, Delivered: false, Error: "offline"},
	}, now)
	if alarm.State != AlarmPartial {
		t.Fatalf("state=%s", alarm.State)
	}
}
