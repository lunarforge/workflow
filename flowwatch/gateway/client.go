package gateway

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	flowwatchv1 "github.com/lunarforge/workflow/flowwatch/gen/flowwatch/v1"
	"github.com/lunarforge/workflow/flowwatch/gen/flowwatch/v1/flowwatchv1connect"
)

// ConnectToGateway opens a persistent registration stream to the gateway and
// handles heartbeats and automatic reconnection. The subsystem must still run
// its own FlowWatch HTTP server for the gateway to proxy requests to.
//
// This function blocks until ctx is cancelled. It reconnects with exponential
// backoff on stream errors.
func ConnectToGateway(ctx context.Context, opts ...ClientOption) error {
	cfg := defaultClientConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	client := flowwatchv1connect.NewGatewayServiceClient(cfg.httpClient, cfg.gatewayAddr)
	backoff := cfg.reconnectInterval

	for {
		err := runRegistrationStream(ctx, client, &cfg)
		if ctx.Err() != nil {
			return ctx.Err()
		}

		log.Warn().
			Err(err).
			Str("subsystem", cfg.subsystemID).
			Dur("backoff", backoff).
			Msg("gateway.client.reconnecting")

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}

		// Exponential backoff with cap.
		backoff = backoff * 2
		if backoff > cfg.maxReconnect {
			backoff = cfg.maxReconnect
		}
	}
}

func runRegistrationStream(ctx context.Context, client flowwatchv1connect.GatewayServiceClient, cfg *clientConfig) error {
	stream := client.Register(ctx)

	// Send RegisterRequest.
	if err := stream.Send(&flowwatchv1.SubsystemMessage{
		Msg: &flowwatchv1.SubsystemMessage_Register{
			Register: &flowwatchv1.RegisterRequest{
				SubsystemId:       cfg.subsystemID,
				SubsystemName:     cfg.subsystemName,
				Description:       cfg.description,
				ApiKey:            cfg.apiKey,
				FlowwatchEndpoint: cfg.localEndpoint,
			},
		},
	}); err != nil {
		return err
	}

	// Read RegisterResponse.
	resp, err := stream.Receive()
	if err != nil {
		return err
	}

	registered := resp.GetRegistered()
	if registered == nil || !registered.Accepted {
		reason := "unknown"
		if registered != nil {
			reason = registered.RejectReason
		}
		log.Error().
			Str("subsystem", cfg.subsystemID).
			Str("reason", reason).
			Msg("gateway.client.registration_rejected")
		return stream.CloseResponse()
	}

	log.Info().
		Str("subsystem", cfg.subsystemID).
		Str("gateway", cfg.gatewayAddr).
		Msg("gateway.client.registered")

	// Enter heartbeat loop: read pings, send pongs.
	for {
		msg, err := stream.Receive()
		if err != nil {
			return err
		}

		if msg.GetPing() != nil {
			if err := stream.Send(&flowwatchv1.SubsystemMessage{
				Msg: &flowwatchv1.SubsystemMessage_Pong{
					Pong: &flowwatchv1.Heartbeat{
						Timestamp: timestamppb.Now(),
					},
				},
			}); err != nil {
				return err
			}
		}

		if msg.GetDisconnect() != nil {
			log.Warn().
				Str("subsystem", cfg.subsystemID).
				Str("reason", msg.GetDisconnect().Reason).
				Msg("gateway.client.disconnect_requested")
			return stream.CloseResponse()
		}
	}
}
