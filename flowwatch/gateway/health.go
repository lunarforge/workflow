package gateway

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
)

// HealthMonitor periodically checks subsystem connections and removes
// subsystems that have been stale beyond the grace period.
type HealthMonitor struct {
	adapter  *Adapter
	interval time.Duration
	done     chan struct{}
}

// NewHealthMonitor creates a new health monitor. The check interval defaults
// to half the heartbeat interval for timely stale detection.
func NewHealthMonitor(a *Adapter) *HealthMonitor {
	return &HealthMonitor{
		adapter:  a,
		interval: a.config.heartbeatInterval / 2,
		done:     make(chan struct{}),
	}
}

// Start begins the background health check loop.
func (h *HealthMonitor) Start(ctx context.Context) {
	go h.run(ctx)
}

// Stop waits for the health monitor to finish.
func (h *HealthMonitor) Stop() {
	<-h.done
}

func (h *HealthMonitor) run(ctx context.Context) {
	defer close(h.done)

	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.check()
		}
	}
}

func (h *HealthMonitor) check() {
	now := time.Now()
	gracePeriod := h.adapter.config.gracePeriod
	heartbeatTimeout := h.adapter.config.heartbeatInterval * 2

	h.adapter.mu.RLock()
	subsystems := make([]*subsystemConn, 0, len(h.adapter.subsystems))
	ids := make([]string, 0, len(h.adapter.subsystems))
	for id, conn := range h.adapter.subsystems {
		subsystems = append(subsystems, conn)
		ids = append(ids, id)
	}
	h.adapter.mu.RUnlock()

	for i, conn := range subsystems {
		conn.mu.RLock()
		state := conn.connState
		lastHB := conn.lastHeartbeat
		staleSince := conn.staleSince
		conn.mu.RUnlock()

		subID := ids[i]

		switch state {
		case connStateConnected:
			// Check if heartbeat is overdue.
			if now.Sub(lastHB) > heartbeatTimeout {
				log.Warn().
					Str("subsystem", subID).
					Dur("since_heartbeat", now.Sub(lastHB)).
					Msg("gateway.heartbeat_timeout")
				conn.markStale()
			}

		case connStateStale:
			// Check if grace period has expired.
			if staleSince != nil && now.Sub(*staleSince) > gracePeriod {
				log.Warn().
					Str("subsystem", subID).
					Msg("gateway.subsystem_removed")
				h.adapter.DeregisterSubsystem(subID)
			}
		}
	}
}
