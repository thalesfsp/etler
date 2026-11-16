package checkpoint

import (
	"context"
	"maps"
	"sync"
)

// Memory is a Store held in process memory. It keeps finished chunks across
// runs in the same process, and loses everything when the process exits.
type Memory struct {
	mu   sync.Mutex
	runs map[string]map[string]Record
}

// NewMemory returns an empty in-memory store.
func NewMemory() *Memory {
	return &Memory{runs: map[string]map[string]Record{}}
}

// Load returns a copy of runID's records.
func (m *Memory) Load(_ context.Context, runID string) (map[string]Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	records := make(map[string]Record, len(m.runs[runID]))
	maps.Copy(records, m.runs[runID])

	return records, nil
}

// Save inserts or replaces record under runID.
func (m *Memory) Save(_ context.Context, runID string, record Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.runs[runID] == nil {
		m.runs[runID] = map[string]Record{}
	}

	m.runs[runID][record.Key] = record

	return nil
}
