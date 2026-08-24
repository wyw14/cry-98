package journal

import (
	"testing"
	"time"

	"github.com/wyw14/cry-98/internal/model"
)

func TestStoreAppendsAndReopensPartition(t *testing.T) {
	root := t.TempDir()
	store, err := NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	for index := range 3 {
		event, err := model.NewEvent("fills", "fill.changed", "session-1", map[string]int{"index": index}, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		persisted, err := store.Append(event)
		if err != nil {
			t.Fatal(err)
		}
		if persisted.Sequence != uint64(index+1) {
			t.Fatalf("sequence=%d", persisted.Sequence)
		}
	}
	reopened, err := NewStore(root)
	if err != nil {
		t.Fatal(err)
	}
	events, err := reopened.Events("fills", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Sequence != 2 {
		t.Fatalf("events=%v", events)
	}
}

func TestSuppressionCheckpointNeverAddsElapsedTime(t *testing.T) {
	start := time.Unix(1000, 0)
	state := NewSuppressionState("alarm-1", start, 10*time.Minute)
	state = state.Checkpoint(start.Add(4 * time.Minute))
	state = state.Checkpoint(start.Add(-20 * time.Minute))
	if state.Remaining != 6*time.Minute {
		t.Fatalf("remaining=%s", state.Remaining)
	}
}
