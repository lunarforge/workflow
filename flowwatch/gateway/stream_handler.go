package gateway

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	flowwatchv1 "github.com/lunarforge/workflow/flowwatch/gen/flowwatch/v1"
)

// Server implements the GatewayServiceHandler interface for subsystem
// registration via bidirectional streams.
type Server struct {
	adapter           *Adapter
	apiKey            string
	heartbeatInterval time.Duration
}

// NewServer creates a new gateway registration server.
func NewServer(adapter *Adapter) *Server {
	return &Server{
		adapter:           adapter,
		apiKey:            adapter.config.apiKey,
		heartbeatInterval: adapter.config.heartbeatInterval,
	}
}

// Register handles a bidirectional registration stream from a subsystem.
func (s *Server) Register(ctx context.Context, stream *connect.BidiStream[flowwatchv1.SubsystemMessage, flowwatchv1.GatewayMessage]) error {
	// 1. Read first message — expect RegisterRequest.
	msg, err := stream.Receive()
	if err != nil {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("expected RegisterRequest as first message"))
	}

	reg := msg.GetRegister()
	if reg == nil {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("first message must be RegisterRequest"))
	}

	// 2. Validate API key.
	if s.apiKey != "" && reg.ApiKey != s.apiKey {
		if err := stream.Send(&flowwatchv1.GatewayMessage{
			Msg: &flowwatchv1.GatewayMessage_Registered{
				Registered: &flowwatchv1.RegisterResponse{
					Accepted:     false,
					RejectReason: "invalid API key",
				},
			},
		}); err != nil {
			return err
		}
		return connect.NewError(connect.CodeUnauthenticated, errors.New("invalid API key"))
	}

	if reg.SubsystemId == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("subsystem_id is required"))
	}

	// 3. Register subsystem in the adapter.
	info := SubsystemInfo{
		SubsystemID:   reg.SubsystemId,
		SubsystemName: reg.SubsystemName,
		Description:   reg.Description,
		Endpoint:      reg.FlowwatchEndpoint,
		Workflows:     reg.Workflows,
		Capabilities:  reg.Capabilities,
		Metadata:      reg.Metadata,
	}
	s.adapter.RegisterSubsystem(info)

	log.Info().
		Str("subsystem", reg.SubsystemId).
		Str("endpoint", reg.FlowwatchEndpoint).
		Int("workflows", len(reg.Workflows)).
		Msg("gateway.subsystem_registered")

	// 4. Send RegisterResponse.
	if err := stream.Send(&flowwatchv1.GatewayMessage{
		Msg: &flowwatchv1.GatewayMessage_Registered{
			Registered: &flowwatchv1.RegisterResponse{
				Accepted:          true,
				HeartbeatInterval: durationpb.New(s.heartbeatInterval),
			},
		},
	}); err != nil {
		s.adapter.DeregisterSubsystem(reg.SubsystemId)
		return err
	}

	// 5. Start heartbeat ping sender.
	pingDone := make(chan struct{})
	go func() {
		defer close(pingDone)
		ticker := time.NewTicker(s.heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := stream.Send(&flowwatchv1.GatewayMessage{
					Msg: &flowwatchv1.GatewayMessage_Ping{
						Ping: &flowwatchv1.Heartbeat{
							Timestamp: timestamppb.Now(),
						},
					},
				}); err != nil {
					return
				}
			}
		}
	}()

	// 6. Read pongs until disconnect.
	for {
		msg, err := stream.Receive()
		if err != nil {
			break
		}
		if msg.GetPong() != nil {
			s.adapter.mu.RLock()
			conn, ok := s.adapter.subsystems[reg.SubsystemId]
			s.adapter.mu.RUnlock()
			if ok {
				conn.recordHeartbeat()
			}
		}
	}

	// 7. Stream closed — mark stale, start grace period.
	log.Warn().
		Str("subsystem", reg.SubsystemId).
		Dur("grace_period", s.adapter.config.gracePeriod).
		Msg("gateway.subsystem_disconnected")

	s.adapter.MarkStale(reg.SubsystemId)

	// Grace period cleanup is handled by the health monitor.
	// If the subsystem reconnects, RegisterSubsystem replaces the stale entry.

	<-pingDone
	return nil
}
