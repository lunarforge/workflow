package adapter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// CachingAdapter wraps an EngineAdapter and materializes analytics results in
// the background. Pre-computed results are served from memory, eliminating the
// expensive per-request record/step pagination.
type CachingAdapter struct {
	inner    EngineAdapter
	interval time.Duration

	mu    sync.RWMutex
	cache map[string]*cacheEntry

	cancel context.CancelFunc
	done   chan struct{}
}

type cacheEntry struct {
	value     any
	expiresAt time.Time
}

// CachingOption configures the CachingAdapter.
type CachingOption func(*CachingAdapter)

// WithRefreshInterval sets how often the background goroutine refreshes analytics.
func WithRefreshInterval(d time.Duration) CachingOption {
	return func(c *CachingAdapter) { c.interval = d }
}

// NewCachingAdapter creates a middleware adapter that caches analytics results.
// Call Start() to begin the background refresh loop, and Stop() to shut it down.
func NewCachingAdapter(inner EngineAdapter, opts ...CachingOption) *CachingAdapter {
	c := &CachingAdapter{
		inner:    inner,
		interval: 5 * time.Second,
		cache:    make(map[string]*cacheEntry),
		done:     make(chan struct{}),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Start begins the background refresh loop. It pre-computes analytics for all
// registered workflows at standard time windows.
func (c *CachingAdapter) Start(ctx context.Context) {
	ctx, c.cancel = context.WithCancel(ctx)
	go c.refreshLoop(ctx)
}

// Stop shuts down the background refresh loop.
func (c *CachingAdapter) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	<-c.done
}

func (c *CachingAdapter) refreshLoop(ctx context.Context) {
	defer close(c.done)

	// Refresh immediately on start.
	c.refresh(ctx)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.refresh(ctx)
		}
	}
}

// Standard time windows to pre-compute. Each has a recommended granularity.
var precomputeWindows = []struct {
	duration    time.Duration
	granularity string
}{
	{1 * time.Hour, "1m"},
	{6 * time.Hour, "5m"},
	{24 * time.Hour, "15m"},
	{7 * 24 * time.Hour, "1h"},
}

func (c *CachingAdapter) refresh(ctx context.Context) {
	workflows, err := c.inner.ListWorkflows(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("flowwatch.cache: failed to list workflows")
		return
	}

	now := time.Now()
	ttl := c.interval * 2 // Cache entries live for 2 refresh cycles.

	// Collect all workflow IDs plus empty string (all-workflows query).
	workflowIDs := []string{""}
	for _, wf := range workflows {
		workflowIDs = append(workflowIDs, wf.ID)
	}

	for _, wfID := range workflowIDs {
		for _, w := range precomputeWindows {
			from := now.Add(-w.duration)
			to := now

			params := AnalyticsParams{
				WorkflowID:  wfID,
				From:        from,
				To:          to,
				Granularity: w.granularity,
			}
			stepParams := StepDurationParams{
				WorkflowID: wfID,
				From:       from,
				To:         to,
			}

			// Compute all 5 analytics in sequence for this workflow+window.
			// Each is fast relative to the full refresh cycle.
			c.computeAndCache(ctx, "throughput", params, ttl, func() (any, error) {
				return c.inner.GetThroughput(ctx, params)
			})
			c.computeAndCache(ctx, "latency", params, ttl, func() (any, error) {
				return c.inner.GetLatency(ctx, params)
			})
			c.computeAndCache(ctx, "failure_rate", params, ttl, func() (any, error) {
				return c.inner.GetFailureRate(ctx, params)
			})
			c.computeAndCache(ctx, "step_duration", stepParams, ttl, func() (any, error) {
				return c.inner.GetStepDuration(ctx, stepParams)
			})
			c.computeAndCache(ctx, "step_heatmap", params, ttl, func() (any, error) {
				return c.inner.GetStepHeatmap(ctx, params)
			})

			if ctx.Err() != nil {
				return
			}
		}
	}
}

func (c *CachingAdapter) computeAndCache(ctx context.Context, method string, params any, ttl time.Duration, compute func() (any, error)) {
	if ctx.Err() != nil {
		return
	}

	result, err := compute()
	if err != nil {
		log.Debug().Err(err).Str("method", method).Msg("flowwatch.cache: compute failed")
		return
	}

	key := cacheKey(method, params)
	c.mu.Lock()
	c.cache[key] = &cacheEntry{
		value:     result,
		expiresAt: time.Now().Add(ttl),
	}
	c.mu.Unlock()
}

func (c *CachingAdapter) get(key string) (any, bool) {
	c.mu.RLock()
	entry, ok := c.cache[key]
	c.mu.RUnlock()

	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.value, true
}

func cacheKey(method string, params any) string {
	raw := fmt.Sprintf("%s:%+v", method, params)
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:16])
}

// findClosestCacheEntry looks for a cached result that covers the requested
// time range. This handles the case where the UI sends a slightly different
// "now" timestamp than what was pre-computed.
func (c *CachingAdapter) findClosestAnalytics(method string, params AnalyticsParams) (any, bool) {
	// First try exact match.
	if v, ok := c.get(cacheKey(method, params)); ok {
		return v, true
	}

	// Try matching against pre-computed windows by duration.
	duration := params.To.Sub(params.From)
	for _, w := range precomputeWindows {
		// Allow 10% tolerance on the time window duration.
		if absDuration(duration-w.duration) > w.duration/10 {
			continue
		}
		// Try the standard granularity for this window.
		candidate := AnalyticsParams{
			WorkflowID:  params.WorkflowID,
			From:        time.Now().Add(-w.duration),
			To:          time.Now(),
			Granularity: w.granularity,
		}
		if v, ok := c.get(cacheKey(method, candidate)); ok {
			return v, true
		}
	}

	return nil, false
}

func (c *CachingAdapter) findClosestStepDuration(params StepDurationParams) (any, bool) {
	if v, ok := c.get(cacheKey("step_duration", params)); ok {
		return v, true
	}

	duration := params.To.Sub(params.From)
	for _, w := range precomputeWindows {
		if absDuration(duration-w.duration) > w.duration/10 {
			continue
		}
		candidate := StepDurationParams{
			WorkflowID: params.WorkflowID,
			From:       time.Now().Add(-w.duration),
			To:         time.Now(),
		}
		if v, ok := c.get(cacheKey("step_duration", candidate)); ok {
			return v, true
		}
	}
	return nil, false
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// ─── EngineAdapter implementation ───
// Analytics methods serve from cache; all others delegate to the inner adapter.

func (c *CachingAdapter) EngineName() string              { return c.inner.EngineName() }
func (c *CachingAdapter) Capabilities() *EngineCapabilities { return c.inner.Capabilities() }

func (c *CachingAdapter) ListSubsystems(ctx context.Context) ([]SubsystemSummary, error) {
	return c.inner.ListSubsystems(ctx)
}
func (c *CachingAdapter) GetSubsystem(ctx context.Context, id string) (*SubsystemDetail, error) {
	return c.inner.GetSubsystem(ctx, id)
}
func (c *CachingAdapter) ListWorkflows(ctx context.Context) ([]WorkflowDef, error) {
	return c.inner.ListWorkflows(ctx)
}
func (c *CachingAdapter) GetWorkflow(ctx context.Context, id string) (*WorkflowDef, error) {
	return c.inner.GetWorkflow(ctx, id)
}
func (c *CachingAdapter) GetGraph(ctx context.Context, id string) (*GraphData, error) {
	return c.inner.GetGraph(ctx, id)
}
func (c *CachingAdapter) GetTrace(ctx context.Context, id string) (*TraceData, error) {
	return c.inner.GetTrace(ctx, id)
}
func (c *CachingAdapter) ListRuns(ctx context.Context, f RunFilter, cursor string, limit int) ([]RunData, string, error) {
	return c.inner.ListRuns(ctx, f, cursor, limit)
}
func (c *CachingAdapter) GetRun(ctx context.Context, id string) (*RunData, error) {
	return c.inner.GetRun(ctx, id)
}
func (c *CachingAdapter) GetRunSteps(ctx context.Context, id string) ([]StepData, error) {
	return c.inner.GetRunSteps(ctx, id)
}
func (c *CachingAdapter) GetRunTimeline(ctx context.Context, id string) (*TimelineData, error) {
	return c.inner.GetRunTimeline(ctx, id)
}
func (c *CachingAdapter) GetRunLogs(ctx context.Context, runID, stepName, cursor string, limit int) ([]LogData, string, error) {
	return c.inner.GetRunLogs(ctx, runID, stepName, cursor, limit)
}
func (c *CachingAdapter) RetryRun(ctx context.Context, id string) (string, error) {
	return c.inner.RetryRun(ctx, id)
}
func (c *CachingAdapter) RetryFromStep(ctx context.Context, runID, stepName string) (string, error) {
	return c.inner.RetryFromStep(ctx, runID, stepName)
}
func (c *CachingAdapter) CancelRun(ctx context.Context, id, reason string) error {
	return c.inner.CancelRun(ctx, id, reason)
}
func (c *CachingAdapter) PauseRun(ctx context.Context, id string) error {
	return c.inner.PauseRun(ctx, id)
}
func (c *CachingAdapter) ResumeRun(ctx context.Context, id string) error {
	return c.inner.ResumeRun(ctx, id)
}
func (c *CachingAdapter) SkipStep(ctx context.Context, runID, stepName, reason string) error {
	return c.inner.SkipStep(ctx, runID, stepName, reason)
}
func (c *CachingAdapter) SubscribeRuns(ctx context.Context, f RunFilter) (<-chan RunEvent, error) {
	return c.inner.SubscribeRuns(ctx, f)
}
func (c *CachingAdapter) SubscribeRun(ctx context.Context, id string) (<-chan RunDetailEvent, error) {
	return c.inner.SubscribeRun(ctx, id)
}

// ─── Cached analytics methods ───

func (c *CachingAdapter) GetThroughput(ctx context.Context, p AnalyticsParams) ([]ThroughputPoint, error) {
	if v, ok := c.findClosestAnalytics("throughput", p); ok {
		return v.([]ThroughputPoint), nil
	}
	return c.inner.GetThroughput(ctx, p)
}

func (c *CachingAdapter) GetLatency(ctx context.Context, p AnalyticsParams) ([]LatencyPoint, error) {
	if v, ok := c.findClosestAnalytics("latency", p); ok {
		return v.([]LatencyPoint), nil
	}
	return c.inner.GetLatency(ctx, p)
}

func (c *CachingAdapter) GetFailureRate(ctx context.Context, p AnalyticsParams) ([]FailureRatePoint, error) {
	if v, ok := c.findClosestAnalytics("failure_rate", p); ok {
		return v.([]FailureRatePoint), nil
	}
	return c.inner.GetFailureRate(ctx, p)
}

func (c *CachingAdapter) GetStepDuration(ctx context.Context, p StepDurationParams) (*StepDurationReport, error) {
	if v, ok := c.findClosestStepDuration(p); ok {
		return v.(*StepDurationReport), nil
	}
	return c.inner.GetStepDuration(ctx, p)
}

func (c *CachingAdapter) GetStepHeatmap(ctx context.Context, p AnalyticsParams) ([]HeatmapPoint, error) {
	if v, ok := c.findClosestAnalytics("step_heatmap", p); ok {
		return v.([]HeatmapPoint), nil
	}
	return c.inner.GetStepHeatmap(ctx, p)
}

// Verify interface compliance.
var _ EngineAdapter = (*CachingAdapter)(nil)
