package journal

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/wyw14/cry-98/internal/model"
)

type Store struct {
	mu        sync.Mutex
	root      string
	sequences map[string]uint64
}

func NewStore(root string) (*Store, error) {
	if root == "" {
		return nil, errors.New("journal root is required")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	store := &Store{root: root, sequences: make(map[string]uint64)}
	if err := store.loadSequences(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) Append(event model.Event) (model.Event, error) {
	if err := event.Validate(); err != nil {
		return model.Event{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	event.Sequence = s.sequences[event.Partition] + 1
	path := filepath.Join(s.root, partitionFileName(event.Partition))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return model.Event{}, err
	}
	encoder := json.NewEncoder(file)
	if err := encoder.Encode(event); err != nil {
		_ = file.Close()
		return model.Event{}, err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return model.Event{}, err
	}
	if err := file.Close(); err != nil {
		return model.Event{}, err
	}
	s.sequences[event.Partition] = event.Sequence
	return event, nil
}

func (s *Store) Events(partition string, after uint64) ([]model.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.root, partitionFileName(partition))
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return []model.Event{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	result := make([]model.Event, 0)
	scanner := bufio.NewScanner(file)
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, 4*1024*1024)
	for scanner.Scan() {
		var event model.Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, err
		}
		if event.Sequence > after {
			result = append(result, event)
		}
	}
	return result, scanner.Err()
}

func (s *Store) Snapshot(partition string, value any, now time.Time) (model.SnapshotEnvelope, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot, err := model.NewSnapshotEnvelope(partition, s.sequences[partition], value, now)
	if err != nil {
		return model.SnapshotEnvelope{}, err
	}
	path := filepath.Join(s.root, partitionSnapshotName(partition))
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return model.SnapshotEnvelope{}, err
	}
	temporary := path + ".new"
	if err := os.WriteFile(temporary, append(data, '\n'), 0o644); err != nil {
		return model.SnapshotEnvelope{}, err
	}
	if err := os.Rename(temporary, path); err != nil {
		return model.SnapshotEnvelope{}, err
	}
	return snapshot, nil
}

func (s *Store) Root() string {
	return s.root
}
