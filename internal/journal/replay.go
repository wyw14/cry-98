package journal

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/wyw14/cry-98/internal/model"
)

type Replay struct {
	store *Store
}

func NewReplay(store *Store) *Replay {
	return &Replay{store: store}
}

func (r *Replay) Partition(partition string, apply func(model.Event) error) error {
	events, err := r.store.Events(partition, 0)
	if err != nil {
		return err
	}
	for _, event := range events {
		if err := apply(event); err != nil {
			return err
		}
	}
	return nil
}

func (r *Replay) Snapshot(partition string, destination any) (uint64, error) {
	path := filepath.Join(r.store.root, partitionSnapshotName(partition))
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var envelope model.SnapshotEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return 0, err
	}
	if destination != nil {
		if err := json.Unmarshal(envelope.Payload, destination); err != nil {
			return 0, err
		}
	}
	return envelope.Sequence, nil
}

func (s *Store) partitionEntries() ([]os.DirEntry, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, err
	}
	result := make([]os.DirEntry, 0)
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".events.jsonl") {
			result = append(result, entry)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name() < result[j].Name() })
	return result, nil
}

func (s *Store) Partitions() ([]string, error) {
	entries, err := s.partitionEntries()
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		result = append(result, strings.TrimSuffix(entry.Name(), ".events.jsonl"))
	}
	return result, nil
}
