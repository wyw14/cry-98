package alarm_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/wyw14/cry-98/internal/alarm"
	"github.com/wyw14/cry-98/internal/journal"
)

func TestAlarmSuppressionDoesNotGrowAfterClockRollback(t *testing.T) {
	current := time.Unix(10_000, 0)
	now := func() time.Time { return current }
	store, err := journal.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.New()
	service := alarm.NewSuppressionService(store, now)
	if err := service.Suppress(id, 10*time.Minute); err != nil {
		t.Fatal(err)
	}
	current = current.Add(5 * time.Minute)
	if err := service.Checkpoint(id); err != nil {
		t.Fatal(err)
	}
	current = current.Add(-25 * time.Minute)
	restored := alarm.NewSuppressionService(store, now)
	if err := restored.Restore(id); err != nil {
		t.Fatal(err)
	}
	remaining := restored.Remaining(id)
	if remaining > 5*time.Minute {
		t.Fatalf("remaining grew to %s", remaining)
	}
	if remaining <= 0 {
		t.Fatalf("remaining=%s", remaining)
	}
}
