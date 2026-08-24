package journal

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode"
)

func partitionFileName(partition string) string {
	return safePartition(partition) + ".events.jsonl"
}

func partitionSnapshotName(partition string) string {
	return safePartition(partition) + ".snapshot.json"
}

func safePartition(partition string) string {
	partition = strings.TrimSpace(partition)
	if partition == "" {
		return "default"
	}
	var builder strings.Builder
	for _, r := range partition {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			builder.WriteRune(r)
			continue
		}
		builder.WriteByte('-')
	}
	value := strings.Trim(builder.String(), "-")
	if value == "" {
		digest := sha256.Sum256([]byte(partition))
		return "partition-" + hex.EncodeToString(digest[:6])
	}
	if len(value) > 80 {
		digest := sha256.Sum256([]byte(value))
		return value[:60] + "-" + hex.EncodeToString(digest[:6])
	}
	return value
}

func (s *Store) loadSequences() error {
	entries, err := s.partitionEntries()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		partition := strings.TrimSuffix(entry.Name(), ".events.jsonl")
		events, err := s.Events(partition, 0)
		if err != nil {
			return err
		}
		if len(events) > 0 {
			s.sequences[partition] = events[len(events)-1].Sequence
		}
	}
	return nil
}
