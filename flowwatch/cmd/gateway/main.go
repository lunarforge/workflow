package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"connectrpc.com/connect"
	connectcors "connectrpc.com/cors"
	"github.com/rs/cors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/lunarforge/workflow/flowwatch/gateway"
	"github.com/lunarforge/workflow/flowwatch/gen/flowwatch/v1/flowwatchv1connect"
	"github.com/lunarforge/workflow/flowwatch/internal/server"
)

func main() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.Kitchen}).
		With().Timestamp().Logger()

	// Configuration.
	addr := envOr("GATEWAY_ADDR", ":8091")
	apiKey := envOr("GATEWAY_API_KEY", "")
	gracePeriod := durationOr("GATEWAY_GRACE_PERIOD", 30*time.Second)
	heartbeat := durationOr("GATEWAY_HEARTBEAT", 10*time.Second)
	runCacheSize := intOr("GATEWAY_RUN_CACHE_SIZE", 100_000)
	uiOrigin := envOr("UI_ORIGIN", "http://localhost:5173")

	// Create gateway adapter.
	ga := gateway.NewAdapter(
		gateway.WithAPIKey(apiKey),
		gateway.WithGracePeriod(gracePeriod),
		gateway.WithHeartbeatInterval(heartbeat),
		gateway.WithRunCacheSize(runCacheSize),
	)

	// Create registration stream server.
	gwServer := gateway.NewServer(ga)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start health monitor.
	health := gateway.NewHealthMonitor(ga)
	health.Start(ctx)
	defer health.Stop()

	mux := http.NewServeMux()

	// Mount gateway registration service (bidi stream for subsystems).
	interceptors := connect.WithInterceptors()
	gwPath, gwHandler := flowwatchv1connect.NewGatewayServiceHandler(gwServer, interceptors)
	mux.Handle(gwPath, gwHandler)

	// Mount standard FlowWatch services (UI-facing) backed by the gateway adapter.
	server.RegisterAll(mux, ga)

	// Health check.
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "ok")
	})

	// CORS for FlowWatch UI.
	handler := cors.New(cors.Options{
		AllowedOrigins: []string{uiOrigin},
		AllowedMethods: connectcors.AllowedMethods(),
		AllowedHeaders: connectcors.AllowedHeaders(),
		ExposedHeaders: connectcors.ExposedHeaders(),
	}).Handler(mux)

	srv := &http.Server{
		Addr:    addr,
		Handler: h2c.NewHandler(handler, &http2.Server{}),
	}

	go func() {
		fmt.Printf("FlowWatch Gateway running on http://localhost%s\n", addr)
		fmt.Printf("  Subsystem registration: %s\n", flowwatchv1connect.GatewayServiceRegisterProcedure)
		fmt.Printf("  FlowWatch UI:           %s\n", uiOrigin)
		fmt.Println()
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	<-ctx.Done()
	fmt.Println("\nShutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func durationOr(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		d, err := time.ParseDuration(v)
		if err == nil {
			return d
		}
		log.Warn().Str("key", key).Str("value", v).Msg("invalid duration, using default")
	}
	return fallback
}

func intOr(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			return n
		}
		log.Warn().Str("key", key).Str("value", v).Msg("invalid integer, using default")
	}
	return fallback
}
