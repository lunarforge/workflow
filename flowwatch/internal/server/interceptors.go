package server

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"github.com/rs/zerolog/log"
)

// NewLoggingInterceptor returns a Connect interceptor that logs every RPC call
// with method, duration, and error status.
func NewLoggingInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			start := time.Now()
			resp, err := next(ctx, req)
			dur := time.Since(start)

			l := log.With().
				Str("method", req.Spec().Procedure).
				Int64("duration_ms", dur.Milliseconds()).
				Logger()

			if err != nil {
				l.Warn().Err(err).Msg("rpc.error")
			} else {
				l.Info().Msg("rpc.ok")
			}
			return resp, err
		}
	}
}
