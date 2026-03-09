# FlowWatch

Engine-agnostic workflow monitoring dashboard for [luno/workflow](https://github.com/lunarforge/workflow).

FlowWatch provides a Connect-Go API server and SvelteKit frontend for monitoring workflow runs, viewing step execution details, and performing lifecycle actions (retry, cancel, pause, resume).

## Quickstart

### 1. Wire up the adapter in your application

FlowWatch connects to your workflow engine through an adapter. For `luno/workflow`, use the built-in adapter:

```go
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
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/lunarforge/workflow"
	"github.com/lunarforge/workflow/adapters/memrecordstore"
	"github.com/lunarforge/workflow/adapters/memstreamer"
	"github.com/lunarforge/workflow/flowwatch/internal/lunoworkflow"
	"github.com/lunarforge/workflow/flowwatch/internal/server"
)

func main() {
	// Your existing workflow infrastructure.
	recordStore := memrecordstore.New()
	streamer := memstreamer.New()

	// Build your workflow as usual.
	b := workflow.NewBuilder[MyData, MyStatus]("order-processing")
	b.AddStep(StatusCreated, processOrder, StatusCompleted)
	wf := b.Build(streamer, recordStore.Store, recordStore.Latest)

	// Create the FlowWatch adapter and register your workflow(s).
	adapter := lunoworkflow.New(recordStore, nil) // pass a StepStore for step-level tracking
	lunoworkflow.RegisterWorkflow(adapter, wf)

	// Mount FlowWatch API handlers.
	mux := http.NewServeMux()
	server.RegisterAll(mux, adapter)

	// Health check endpoint.
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	// Start the server with CORS and h2c support.
	handler := cors.New(cors.Options{
		AllowedMethods: connectcors.AllowedMethods(),
		AllowedHeaders: connectcors.AllowedHeaders(),
		ExposedHeaders: connectcors.ExposedHeaders(),
	}).Handler(mux)

	srv := &http.Server{
		Addr:    ":8090",
		Handler: h2c.NewHandler(handler, &http2.Server{}),
	}

	// Start the workflow engine.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go wf.Run(ctx)
	defer wf.Stop()

	go func() {
		fmt.Println("FlowWatch API running on http://localhost:8090")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
}
```

### 2. Start the frontend

```bash
cd flowwatch/ui
bun install
VITE_API_URL=http://localhost:8090 bun run dev
```

Open http://localhost:5173 to view the dashboard.

### 3. Environment variables

| Variable | Default | Description |
|---|---|---|
| `FLOWWATCH_ADDR` | `:8090` | API server listen address |
| `VITE_API_URL` | `http://localhost:8090` | API URL for the frontend |

## Architecture

```
flowwatch/
├── cmd/flowwatch/          # Server entry point
├── proto/flowwatch/v1/     # Protobuf service definitions
├── gen/                    # Generated Go + Connect-Go stubs
├── internal/
│   ├── adapter/            # EngineAdapter interface
│   ├── lunoworkflow/       # luno/workflow adapter implementation
│   └── server/             # Connect-Go service handlers
└── ui/                     # SvelteKit frontend
    ├── src/gen/            # Generated TypeScript stubs
    ├── src/lib/api/        # Connect-Web transport client
    └── src/routes/         # Pages: dashboard, workflows, runs, run detail
```

### EngineAdapter

FlowWatch is engine-agnostic. The `EngineAdapter` interface defines the contract between the API layer and your workflow engine. The `lunoworkflow` package provides the built-in adapter for `luno/workflow`.

To support a different engine, implement the `EngineAdapter` interface in `flowwatch/internal/adapter/adapter.go`.

### API

FlowWatch exposes Connect-Go services (compatible with gRPC, gRPC-Web, and Connect protocol):

- **WorkflowService** — List workflows, get graph topology, capabilities, subsystems
- **RunService** — List/get runs, step executions, timeline, and lifecycle actions (retry, cancel, pause, resume, skip)
- **AnalyticsService** — Throughput, latency, failure rate, step duration (planned)
- **StreamService** — Real-time run updates (planned)
- **SearchService** — Full-text search across runs (planned)

### Frontend pages

| Route | Description |
|---|---|
| `/` | Dashboard with stats and recent runs |
| `/workflows` | Workflow list with status graph info |
| `/runs` | Runs table with pagination and workflow filter |
| `/runs/[id]` | Run detail with metadata, step table, and Gantt timeline |

## Development

```bash
# Run Go tests
cd flowwatch && go test -v -race ./...

# Regenerate protobuf stubs
cd flowwatch && buf generate          # Go
cd flowwatch/ui && buf generate       # TypeScript

# Frontend dev server
cd flowwatch/ui && bun run dev

# Build frontend
cd flowwatch/ui && bun run build
```
