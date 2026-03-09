package lunoworkflow

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/lunarforge/workflow"

	"github.com/lunarforge/workflow/flowwatch/internal/adapter"
)

// parseGranularity converts a granularity string like "1h", "15m", "1d" to a time.Duration.
func parseGranularity(g string) time.Duration {
	switch g {
	case "1m":
		return time.Minute
	case "5m":
		return 5 * time.Minute
	case "15m":
		return 15 * time.Minute
	case "1h":
		return time.Hour
	case "1d":
		return 24 * time.Hour
	default:
		return time.Hour
	}
}

// timeBuckets generates bucket start times from `from` to `to` at the given interval.
func timeBuckets(from, to time.Time, interval time.Duration) []time.Time {
	from = from.Truncate(interval)
	var buckets []time.Time
	for t := from; !t.After(to); t = t.Add(interval) {
		buckets = append(buckets, t)
	}
	return buckets
}

// bucketIndex returns the index of the bucket that the given time falls into.
func bucketIndex(t time.Time, from time.Time, interval time.Duration) int {
	from = from.Truncate(interval)
	if t.Before(from) {
		return 0
	}
	return int(t.Sub(from) / interval)
}

// fetchRecordsInRange loads all records from the store that fall within the time range.
// It pages through the store in batches.
func (a *Adapter) fetchRecordsInRange(ctx context.Context, workflowName string, from, to time.Time) ([]workflow.Record, error) {
	const batchSize = 500
	var all []workflow.Record
	var offset int64

	for {
		records, err := a.recordStore.List(ctx, workflowName, offset, batchSize, workflow.OrderTypeAscending)
		if err != nil {
			return nil, err
		}
		if len(records) == 0 {
			break
		}

		for _, rec := range records {
			if rec.CreatedAt.After(to) {
				// Past the end of range and ascending order, so we can stop.
				return all, nil
			}
			if rec.CreatedAt.Before(from) && rec.UpdatedAt.Before(from) {
				continue
			}
			all = append(all, rec)
		}

		if len(records) < batchSize {
			break
		}
		offset += int64(batchSize)
	}

	return all, nil
}

// fetchAllStepRecords loads step records for all given runs.
func (a *Adapter) fetchAllStepRecords(ctx context.Context, records []workflow.Record) ([]workflow.StepRecord, error) {
	if a.stepStore == nil {
		return nil, nil
	}

	var all []workflow.StepRecord
	seen := make(map[string]bool)
	for _, rec := range records {
		if seen[rec.RunID] {
			continue
		}
		seen[rec.RunID] = true

		steps, err := a.stepStore.List(ctx, rec.WorkflowName, rec.RunID)
		if err != nil {
			return nil, err
		}
		all = append(all, steps...)
	}
	return all, nil
}

// percentile computes the p-th percentile from a sorted slice of durations.
// p should be between 0 and 1 (e.g., 0.5 for p50, 0.95 for p95).
func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func (a *Adapter) GetThroughput(ctx context.Context, p adapter.AnalyticsParams) ([]adapter.ThroughputPoint, error) {
	records, err := a.fetchRecordsInRange(ctx, p.WorkflowID, p.From, p.To)
	if err != nil {
		return nil, fmt.Errorf("get throughput: %w", err)
	}

	interval := parseGranularity(p.Granularity)
	buckets := timeBuckets(p.From, p.To, interval)
	points := make([]adapter.ThroughputPoint, len(buckets))
	for i, t := range buckets {
		points[i].Timestamp = t
	}

	for _, rec := range records {
		// Use UpdatedAt to categorize into a bucket (represents when the state was reached).
		idx := bucketIndex(rec.UpdatedAt, p.From, interval)
		if idx < 0 || idx >= len(points) {
			continue
		}

		points[idx].Total++
		switch rec.RunState {
		case workflow.RunStateCompleted, workflow.RunStateDataDeleted, workflow.RunStateRequestedDataDeleted:
			points[idx].Succeeded++
		case workflow.RunStateCancelled:
			points[idx].Canceled++
		case workflow.RunStateRunning:
			points[idx].Running++
		case workflow.RunStateInitiated:
			points[idx].Running++ // count initiated as running for throughput
		case workflow.RunStatePaused:
			points[idx].Running++
		}
	}

	return points, nil
}

func (a *Adapter) GetLatency(ctx context.Context, p adapter.AnalyticsParams) ([]adapter.LatencyPoint, error) {
	records, err := a.fetchRecordsInRange(ctx, p.WorkflowID, p.From, p.To)
	if err != nil {
		return nil, fmt.Errorf("get latency: %w", err)
	}

	interval := parseGranularity(p.Granularity)
	buckets := timeBuckets(p.From, p.To, interval)

	// Collect durations per bucket.
	dursByBucket := make(map[int][]time.Duration)
	for _, rec := range records {
		if !rec.RunState.Finished() {
			continue
		}
		dur := rec.UpdatedAt.Sub(rec.CreatedAt)
		idx := bucketIndex(rec.UpdatedAt, p.From, interval)
		if idx < 0 || idx >= len(buckets) {
			continue
		}
		dursByBucket[idx] = append(dursByBucket[idx], dur)
	}

	points := make([]adapter.LatencyPoint, len(buckets))
	for i, t := range buckets {
		points[i].Timestamp = t
		durs := dursByBucket[i]
		if len(durs) == 0 {
			continue
		}
		sort.Slice(durs, func(a, b int) bool { return durs[a] < durs[b] })
		points[i].P50 = percentile(durs, 0.50)
		points[i].P95 = percentile(durs, 0.95)
		points[i].P99 = percentile(durs, 0.99)
	}

	return points, nil
}

func (a *Adapter) GetFailureRate(ctx context.Context, p adapter.AnalyticsParams) ([]adapter.FailureRatePoint, error) {
	records, err := a.fetchRecordsInRange(ctx, p.WorkflowID, p.From, p.To)
	if err != nil {
		return nil, fmt.Errorf("get failure rate: %w", err)
	}

	interval := parseGranularity(p.Granularity)
	buckets := timeBuckets(p.From, p.To, interval)
	points := make([]adapter.FailureRatePoint, len(buckets))
	for i, t := range buckets {
		points[i].Timestamp = t
	}

	for _, rec := range records {
		if !rec.RunState.Finished() {
			continue
		}
		idx := bucketIndex(rec.UpdatedAt, p.From, interval)
		if idx < 0 || idx >= len(points) {
			continue
		}
		points[idx].TotalRuns++
		if rec.RunState == workflow.RunStateCancelled {
			points[idx].FailedRuns++
		}
	}

	for i := range points {
		if points[i].TotalRuns > 0 {
			points[i].Rate = float64(points[i].FailedRuns) / float64(points[i].TotalRuns)
		}
	}

	return points, nil
}

func (a *Adapter) GetStepDuration(ctx context.Context, p adapter.StepDurationParams) (*adapter.StepDurationReport, error) {
	if a.stepStore == nil {
		return nil, fmt.Errorf("step store not configured")
	}

	records, err := a.fetchRecordsInRange(ctx, p.WorkflowID, p.From, p.To)
	if err != nil {
		return nil, fmt.Errorf("get step duration: %w", err)
	}

	steps, err := a.fetchAllStepRecords(ctx, records)
	if err != nil {
		return nil, fmt.Errorf("get step duration: fetch steps: %w", err)
	}

	// Group step durations by step name.
	type stepDurations struct {
		total []time.Duration
		// We don't have separate queue/exec tracking in StepRecord, so total = exec.
	}
	byStep := make(map[string]*stepDurations)
	// Also group by step+day for trend.
	type dayKey struct {
		step string
		day  time.Time
	}
	byStepDay := make(map[dayKey][]time.Duration)

	for _, s := range steps {
		if s.StepStatus != workflow.StepStatusSucceeded && s.StepStatus != workflow.StepStatusFailed {
			continue
		}
		name := s.StatusDescription
		if name == "" {
			name = fmt.Sprintf("status-%d", s.Status)
		}

		sd, ok := byStep[name]
		if !ok {
			sd = &stepDurations{}
			byStep[name] = sd
		}
		sd.total = append(sd.total, s.Duration)

		day := s.StartedAt.Truncate(24 * time.Hour)
		byStepDay[dayKey{step: name, day: day}] = append(byStepDay[dayKey{step: name, day: day}], s.Duration)
	}

	// Compute cross-step median p95 for bottleneck detection.
	var allP95s []time.Duration
	for _, sd := range byStep {
		sorted := make([]time.Duration, len(sd.total))
		copy(sorted, sd.total)
		sort.Slice(sorted, func(a, b int) bool { return sorted[a] < sorted[b] })
		allP95s = append(allP95s, percentile(sorted, 0.95))
	}
	sort.Slice(allP95s, func(a, b int) bool { return allP95s[a] < allP95s[b] })
	medianP95 := percentile(allP95s, 0.50)
	bottleneckThreshold := medianP95 * 2

	// Build entries.
	var entries []adapter.StepDurationEntry
	for name, sd := range byStep {
		sorted := make([]time.Duration, len(sd.total))
		copy(sorted, sd.total)
		sort.Slice(sorted, func(a, b int) bool { return sorted[a] < sorted[b] })

		totalPercentiles := adapter.DurationPercentiles{
			P50: percentile(sorted, 0.50),
			P95: percentile(sorted, 0.95),
			P99: percentile(sorted, 0.99),
		}

		// Build 7-day trend.
		var trend []adapter.StepTrendPoint
		for d := 0; d < 7; d++ {
			day := p.To.AddDate(0, 0, -6+d).Truncate(24 * time.Hour)
			dayDurs := byStepDay[dayKey{step: name, day: day}]
			tp := adapter.StepTrendPoint{Timestamp: day}
			if len(dayDurs) > 0 {
				sortedDay := make([]time.Duration, len(dayDurs))
				copy(sortedDay, dayDurs)
				sort.Slice(sortedDay, func(a, b int) bool { return sortedDay[a] < sortedDay[b] })
				tp.P50 = percentile(sortedDay, 0.50)
				tp.P95 = percentile(sortedDay, 0.95)
				tp.P99 = percentile(sortedDay, 0.99)
			}
			trend = append(trend, tp)
		}

		entries = append(entries, adapter.StepDurationEntry{
			StepName:     name,
			IsBottleneck: totalPercentiles.P95 > bottleneckThreshold,
			QueueWait:    adapter.DurationPercentiles{}, // Not tracked separately
			Execution:    totalPercentiles,
			Total:        totalPercentiles,
			SampleCount:  int64(len(sd.total)),
			Trend:        trend,
		})
	}

	// Sort entries by step name for consistency.
	sort.Slice(entries, func(i, j int) bool { return entries[i].StepName < entries[j].StepName })

	return &adapter.StepDurationReport{
		Steps:               entries,
		BottleneckThreshold: bottleneckThreshold,
	}, nil
}

func (a *Adapter) GetStepHeatmap(ctx context.Context, p adapter.AnalyticsParams) ([]adapter.HeatmapPoint, error) {
	if a.stepStore == nil {
		return nil, fmt.Errorf("step store not configured")
	}

	records, err := a.fetchRecordsInRange(ctx, p.WorkflowID, p.From, p.To)
	if err != nil {
		return nil, fmt.Errorf("get step heatmap: %w", err)
	}

	steps, err := a.fetchAllStepRecords(ctx, records)
	if err != nil {
		return nil, fmt.Errorf("get step heatmap: fetch steps: %w", err)
	}

	interval := parseGranularity(p.Granularity)

	// Aggregate by (step, bucket).
	type cellKey struct {
		step   string
		bucket time.Time
	}
	type cellAgg struct {
		total    int
		failed   int
		totalDur time.Duration
	}
	cells := make(map[cellKey]*cellAgg)

	for _, s := range steps {
		if s.StepStatus != workflow.StepStatusSucceeded && s.StepStatus != workflow.StepStatusFailed {
			continue
		}
		name := s.StatusDescription
		if name == "" {
			name = fmt.Sprintf("status-%d", s.Status)
		}

		bucket := s.StartedAt.Truncate(interval)
		key := cellKey{step: name, bucket: bucket}
		c, ok := cells[key]
		if !ok {
			c = &cellAgg{}
			cells[key] = c
		}
		c.total++
		c.totalDur += s.Duration
		if s.StepStatus == workflow.StepStatusFailed {
			c.failed++
		}
	}

	var points []adapter.HeatmapPoint
	for key, c := range cells {
		var rate float64
		if c.total > 0 {
			rate = float64(c.failed) / float64(c.total)
		}
		var avgDur time.Duration
		if c.total > 0 {
			avgDur = c.totalDur / time.Duration(c.total)
		}
		points = append(points, adapter.HeatmapPoint{
			StepName:        key.step,
			BucketStart:     key.bucket,
			FailureRate:     rate,
			TotalExecutions: c.total,
			FailedExecs:     c.failed,
			AvgDuration:     avgDur,
		})
	}

	// Sort by step name, then bucket.
	sort.Slice(points, func(i, j int) bool {
		if points[i].StepName != points[j].StepName {
			return points[i].StepName < points[j].StepName
		}
		return points[i].BucketStart.Before(points[j].BucketStart)
	})

	return points, nil
}
