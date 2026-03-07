package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	connectcors "connectrpc.com/cors"
	"github.com/rs/cors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/luno/workflow/flowwatch/internal/server"
)

func main() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()

	addr := envOr("FLOWWATCH_ADDR", ":8090")

	mux := http.NewServeMux()

	// Register all Connect-Go service handlers on the mux.
	// Each handler registers its own path prefix (e.g., /flowwatch.v1.RunService/).
	// TODO(phase3): Replace nil with a real EngineAdapter implementation.
	server.RegisterAll(mux, nil)

	// Serve the OpenAPI spec for Redocly dev mode.
	mux.Handle("/docs/", http.StripPrefix("/docs/", http.FileServer(http.Dir("docs"))))

	// Health check
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	// Wrap with CORS for browser Connect/gRPC-Web clients.
	handler := withCORS(mux)

	// h2c allows HTTP/2 without TLS (for local dev + gRPC).
	srv := &http.Server{
		Addr:    addr,
		Handler: h2c.NewHandler(handler, &http2.Server{}),
	}

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info().Str("addr", addr).Msg("flowwatch server starting")
		log.Info().Str("url", fmt.Sprintf("http://localhost%s/docs/", addr)).Msg("API docs available")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("server error")
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	log.Info().Msg("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("shutdown error")
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// withCORS wraps a handler to allow browser-based Connect and gRPC-Web requests.
func withCORS(h http.Handler) http.Handler {
	return cors.New(cors.Options{
		AllowedMethods: connectcors.AllowedMethods(),
		AllowedHeaders: connectcors.AllowedHeaders(),
		ExposedHeaders: connectcors.ExposedHeaders(),
	}).Handler(h)
}
