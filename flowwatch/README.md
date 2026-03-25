# FlowWatch

Engine-agnostic workflow monitoring dashboard for [lunarforge/workflow](https://github.com/lunarforge/workflow).

FlowWatch provides a Connect-Go API server and SvelteKit frontend for monitoring workflow runs, viewing step execution details, and performing lifecycle actions (retry, cancel, pause, resume).

## Quickstart

### Standalone — single application

Add FlowWatch monitoring to your workflow application using the public `flowwatch` package:

```go
import (
	"github.com/lunarforge/workflow/flowwatch"
)

// Create the adapter with your existing stores.
fwAdapter := flowwatch.NewAdapter(recordStore, stepStore)

// Register each workflow you want to monitor.
flowwatch.RegisterWorkflow(fwAdapter, wf, flowwatch.WithSubsystem("orders"))

// Mount FlowWatch API handlers on your mux.
mux := http.NewServeMux()
stop := flowwatch.RegisterHandlersWithCache(ctx, mux, fwAdapter)
defer stop()
```

Point the frontend at your application:

```bash
cd flowwatch/ui
bun install
VITE_API_URL=http://localhost:8090 bun run dev
```

Open http://localhost:5173 to view the dashboard.

### Multi-subsystem — Gateway

When you have multiple services each running their own workflows, the **Gateway** aggregates them into a single unified dashboard.

**1. Start the gateway:**

```bash
cd flowwatch
make build-gateway
GATEWAY_API_KEY=secret ./bin/gateway
```

The gateway listens on `:8091` by default and serves the same FlowWatch API, backed by all registered subsystems.

**2. Register subsystems:**

Each service opts in by setting `GATEWAY_ADDR`:

```go
import "github.com/lunarforge/workflow/flowwatch/gateway"

// After FlowWatch setup, register with the gateway (non-blocking).
go gateway.ConnectToGateway(ctx,
	gateway.WithGatewayAddr("http://localhost:8091"),
	gateway.WithClientAPIKey("secret"),
	gateway.WithSubsystemID("order-service"),
	gateway.WithSubsystemName("Order Service"),
	gateway.WithDescription("Order processing workflows"),
	gateway.WithLocalEndpoint("http://localhost:8090"),
)
```

**3. Point the UI at the gateway:**

```bash
cd flowwatch/ui
VITE_API_URL=http://localhost:8091 bun run dev
```

The dashboard now shows all workflows from all registered subsystems.

### Environment variables

#### Standalone FlowWatch

| Variable | Default | Description |
|---|---|---|
| `FLOWWATCH_ADDR` | `:8090` | API server listen address |
| `VITE_API_URL` | `http://localhost:8090` | API URL for the frontend |

#### Gateway

| Variable | Default | Description |
|---|---|---|
| `GATEWAY_ADDR` | `:8091` | Gateway listen address |
| `GATEWAY_API_KEY` | `""` | Shared secret for subsystem registration |
| `GATEWAY_GRACE_PERIOD` | `30s` | Time before removing a disconnected subsystem |
| `GATEWAY_HEARTBEAT` | `10s` | Heartbeat ping interval |
| `GATEWAY_RUN_CACHE_SIZE` | `100000` | LRU cache size for run-to-subsystem routing |
| `UI_ORIGIN` | `http://localhost:5173` | Allowed CORS origin for the frontend |

## Architecture

```
flowwatch/
├── cmd/
│   ├── flowwatch/             # Standalone FlowWatch server
│   └── gateway/               # Gateway aggregation server
├── gateway/                   # Gateway package
│   ├── adapter.go             # EngineAdapter backed by subsystem fan-out
│   ├── client.go              # Subsystem → gateway registration client
│   ├── stream_handler.go      # Bidi gRPC stream handler (registration)
│   ├── health.go              # Health monitor (heartbeat, stale detection)
│   ├── routing.go             # Run ID → subsystem LRU router
│   ├── proxy.go               # Fan-out helpers and proto conversion
│   └── options.go             # Functional options
├── proto/flowwatch/v1/        # Protobuf service definitions
├── gen/                       # Generated Go + Connect-Go stubs
├── internal/
│   ├── adapter/               # EngineAdapter interface + CachingAdapter
│   ├── lunoworkflow/          # lunarforge/workflow adapter implementation
│   └── server/                # Connect-Go service handlers
└── ui/                        # SvelteKit frontend
    ├── src/gen/               # Generated TypeScript stubs
    ├── src/lib/api/           # Connect-Web transport client
    └── src/routes/            # Pages: dashboard, workflows, runs, run detail
```

### EngineAdapter

FlowWatch is engine-agnostic. The `EngineAdapter` interface (`internal/adapter/adapter.go`) defines the contract between the API layer and your workflow engine:

- **`lunoworkflow`** — Built-in adapter for `lunarforge/workflow`, backed by RecordStore and StepStore.
- **`gateway.Adapter`** — Implements EngineAdapter by proxying to registered subsystems. Each API call fans out to all healthy subsystems in parallel, merges results, and returns a unified response.

To support a different engine, implement the `EngineAdapter` interface.

### Gateway

The gateway enables a single FlowWatch UI to monitor workflows across multiple independent services.

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  Service A   │     │  Service B   │     │  Service C   │
│  (FlowWatch) │     │  (FlowWatch) │     │  (FlowWatch) │
└──────┬───────┘     └──────┬───────┘     └──────┬───────┘
       │ Register            │ Register           │ Register
       │ (bidi stream)       │ (bidi stream)      │ (bidi stream)
       ▼                     ▼                    ▼
┌──────────────────────────────────────────────────────┐
│                    Gateway                            │
│  ┌──────────┐  ┌──────────┐  ┌──────────────────┐   │
│  │ Registry  │  │ Health   │  │ EngineAdapter    │   │
│  │ (streams) │  │ Monitor  │  │ (fan-out proxy)  │   │
│  └──────────┘  └──────────┘  └──────────────────┘   │
└──────────────────────┬───────────────────────────────┘
                       │ Connect-Go API
                       ▼
                ┌─────────────┐
                │  FlowWatch  │
                │     UI      │
                └─────────────┘
```

**Registration flow:**

1. Subsystem calls `gateway.ConnectToGateway()` which opens a bidirectional gRPC stream.
2. Subsystem sends a `RegisterRequest` with its ID, name, description, and local FlowWatch endpoint.
3. Gateway validates the API key, creates Connect-Go clients for the subsystem's FlowWatch services, and sends back a `RegisterResponse`.
4. The stream stays open for heartbeat pings — gateway sends `Ping`, subsystem responds with `Pong`.

**Query routing:**

- **List operations** (ListWorkflows, ListRuns, etc.): Fan out to all healthy subsystems in parallel, merge and return.
- **Single-item lookups** (GetRun, GetRunTimeline, etc.): Route by run ID using an LRU cache. On cache miss, broadcast to all subsystems.
- **Analytics**: Additive metrics (throughput, failure rate) are merged across subsystems. Percentile-based metrics (latency, step duration) are scoped to a single subsystem.

**Health monitoring:**

- Gateway pings each subsystem every `heartbeatInterval` (default 10s).
- Subsystems that miss 2 consecutive heartbeats are marked stale and excluded from queries.
- After `gracePeriod` (default 30s) without recovery, the subsystem is removed from the registry.
- Subsystems reconnect automatically with exponential backoff (1s to 30s).

### API

FlowWatch exposes Connect-Go services (compatible with gRPC, gRPC-Web, and Connect protocol):

- **WorkflowService** — List workflows, get graph topology, capabilities, subsystems
- **RunService** — List/get runs, step executions, timeline, and lifecycle actions (retry, cancel, pause, resume, skip)
- **AnalyticsService** — Throughput, latency, failure rate, step duration
- **StreamService** — Real-time run updates (planned)
- **SearchService** — Full-text search across runs (planned)
- **GatewayService** — Bidirectional registration stream (gateway only)

### Frontend pages

| Route | Description |
|---|---|
| `/` | Dashboard with stats and recent runs |
| `/workflows` | Workflow list with status graph info |
| `/runs` | Runs table with pagination and workflow filter |
| `/runs/[id]` | Run detail with metadata, step table, and Gantt timeline |

## Development

```bash
# Build
make build            # Build standalone FlowWatch server
make build-gateway    # Build gateway server

# Run
make run              # Build and run standalone server
make run-gateway      # Build and run gateway

# Test
make test             # Run Go tests

# Proto
make proto            # Regenerate Go + Connect-Go stubs
cd flowwatch/ui && buf generate   # Regenerate TypeScript stubs

# Frontend
cd flowwatch/ui && bun run dev    # Dev server
cd flowwatch/ui && bun run build  # Production build
```
