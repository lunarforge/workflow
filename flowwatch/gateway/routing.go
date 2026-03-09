package gateway

import "sync"

// runRouter maintains an LRU-like cache mapping run IDs to subsystem IDs.
// This enables efficient routing of per-run requests (GetRun, RetryRun, etc.)
// without broadcasting to all subsystems.
type runRouter struct {
	mu      sync.RWMutex
	cache   map[string]string // runID -> subsystemID
	order   []string          // insertion order for eviction
	maxSize int
}

func newRunRouter(maxSize int) *runRouter {
	if maxSize <= 0 {
		maxSize = 100_000
	}
	return &runRouter{
		cache:   make(map[string]string, maxSize),
		order:   make([]string, 0, maxSize),
		maxSize: maxSize,
	}
}

// Record stores a run-to-subsystem mapping. If the cache is full, the oldest
// entry is evicted.
func (r *runRouter) Record(runID, subsystemID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.cache[runID]; ok {
		// Already cached, update the subsystem (might have moved).
		r.cache[runID] = subsystemID
		return
	}

	// Evict oldest if at capacity.
	for len(r.order) >= r.maxSize {
		oldest := r.order[0]
		r.order = r.order[1:]
		delete(r.cache, oldest)
	}

	r.cache[runID] = subsystemID
	r.order = append(r.order, runID)
}

// RecordBatch stores multiple run-to-subsystem mappings efficiently.
func (r *runRouter) RecordBatch(subsystemID string, runIDs []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, runID := range runIDs {
		if _, ok := r.cache[runID]; ok {
			r.cache[runID] = subsystemID
			continue
		}

		for len(r.order) >= r.maxSize {
			oldest := r.order[0]
			r.order = r.order[1:]
			delete(r.cache, oldest)
		}

		r.cache[runID] = subsystemID
		r.order = append(r.order, runID)
	}
}

// Lookup returns the subsystem that owns the given run ID.
func (r *runRouter) Lookup(runID string) (subsystemID string, ok bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	subsystemID, ok = r.cache[runID]
	return
}

// RemoveSubsystem removes all cached entries for a given subsystem.
// Called when a subsystem is deregistered after grace period expiry.
func (r *runRouter) RemoveSubsystem(subsystemID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	filtered := r.order[:0]
	for _, runID := range r.order {
		if r.cache[runID] == subsystemID {
			delete(r.cache, runID)
		} else {
			filtered = append(filtered, runID)
		}
	}
	r.order = filtered
}
