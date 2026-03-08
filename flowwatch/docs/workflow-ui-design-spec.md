# Workflow Lifecycle Monitor — UI/UX Design Specification

> **Version:** 1.0 Draft  
> **Date:** March 2026  
> **Scope:** Engine-agnostic, real-time workflow monitoring & management dashboard  
> **Scale:** Thousands of runs/day, streaming updates, mixed technical/non-technical users

---

## 1. Design Philosophy

### Guiding Principles

1. **Glanceability first** — The most critical information (how many workflows are healthy right now?) must be answerable in under 3 seconds without any interaction.
2. **Progressive disclosure** — Non-technical users see status and actions. Technical users can drill into step-level detail, payloads, and logs. The same screen serves both by layering depth.
3. **Action-oriented** — Every problem surfaced should have an adjacent action. A failed step shows a "Retry" button, not just a red badge.
4. **Engine-agnostic abstraction** — The UI consumes a normalized API. Whether the backend is `go-workflow`, `luno/workflow`, Temporal, or a custom engine, the data model is the same.
5. **Real-time by default** — Live updates via WebSocket/SSE. No manual refresh. Stale data is the enemy of trust.

### Design Language

- **Color system:** Semantic, not decorative. Green=healthy, amber=warning/running-long, red=failed, blue=running, gray=pending/skipped.
- **Typography:** Monospace for IDs/timestamps/payloads. Sans-serif for labels and navigation. Clear hierarchy via weight, not size proliferation.
- **Density toggle:** Compact mode for ops/SRE (more rows, smaller text). Comfortable mode for product users (more whitespace, larger touch targets).
- **Dark/light mode:** Required. Ops users work in dark mode on wall displays; product users prefer light.

---

## 2. Information Architecture

### Site Map

```
┌─ Dashboard (home)
│   ├─ Global health summary cards
│   ├─ Subsystem health cards (per-subsystem stats)
│   ├─ Live run ticker
│   └─ Alert feed
│
├─ Subsystems (top-level grouping)
│   ├─ Subsystem list with health indicators
│   └─ [Subsystem Detail]
│       ├─ Subsystem-scoped stats (throughput, latency, error rate)
│       ├─ Workflows in this subsystem
│       └─ Recent runs (filtered to subsystem)
│
├─ Workflows (catalog)
│   ├─ Workflow definition list (grouped by subsystem)
│   └─ [Workflow Detail]
│       ├─ Definition (DAG/state-machine visualization)
│       ├─ Active runs
│       └─ Analytics
│
├─ Runs (execution log)
│   ├─ Filterable run list (paginated, streaming)
│   │   ├─ Subsystem filter (dropdown / sidebar dimension)
│   │   └─ Trace ID filter
│   ├─ Saved views / presets
│   └─ [Run Detail]
│       ├─ Timeline view
│       ├─ Step list view
│       ├─ DAG visualization (live)
│       ├─ Trace link → Trace waterfall view
│       ├─ Payload inspector
│       └─ Action panel (retry, pause, cancel, skip)
│
├─ Traces
│   ├─ Trace lookup (by trace ID)
│   └─ [Trace Waterfall View]
│       ├─ Cross-subsystem span waterfall (Gantt-style)
│       ├─ Subsystem swimlanes
│       ├─ Per-span run detail (expandable)
│       └─ Latency breakdown (queue vs exec per span)
│
├─ Analytics
│   ├─ Global or subsystem-scoped (toggle)
│   ├─ Throughput over time
│   ├─ Latency distributions
│   ├─ Step duration & latency breakdown
│   ├─ Failure rate trends
│   └─ Step-level heatmap
│
└─ Settings
    ├─ Subsystem management
    ├─ Alert rules
    ├─ Saved views management
    ├─ API keys
    └─ Engine connections
```

### Navigation Model

- **Primary nav:** Left sidebar, collapsible to icons. Items: Dashboard, Subsystems, Workflows, Runs, Traces, Analytics, Settings.
- **Subsystem context:** A global subsystem selector in the top bar scopes the entire UI to a single subsystem. When set, all pages (Dashboard, Runs, Analytics) filter to that subsystem. "All Subsystems" is the default.
- **Contextual breadcrumb:** Top bar shows `Subsystems > order-fulfillment > Workflows > order-processing > Run abc123 > Step: validate-payment`.
- **Trace navigation:** Clicking a trace ID anywhere in the UI (run detail, search results, logs) opens the Trace Waterfall View.
- **Quick search:** Global `Cmd+K` palette. Search by run ID, workflow name, foreign ID, trace ID, subsystem, or error text.
- **Saved views:** Pinned filters in a collapsible sub-nav within the Runs page (inspired by Temporal's Saved Views). Subsystem scope is persisted in saved views.

---

## 3. Screen Designs — Wireframes & Specifications

### 3.1 Dashboard (Home)

**Purpose:** Answer "is everything okay?" in 3 seconds.

```
┌─────────────────────────────────────────────────────────────────┐
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ │
│  │ RUNNING │ │SUCCEEDED│ │ FAILED  │ │ PAUSED  │ │  RATE   │ │
│  │   47    │ │  1,283  │ │    3    │ │    5    │ │ 82/min  │ │
│  │  ↑ 12%  │ │  ↑ 5%   │ │  ↓ 40% │ │  ── 0% │ │  ↑ 8%  │ │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘ │
│                                                                 │
│  ┌─ LIVE RUN TICKER ──────────────────────────────────────────┐ │
│  │ 🟢 order-proc #a3f2  step:ship        2s ago   Succeeded  │ │
│  │ 🔵 kyc-verify  #b891  step:face-match  now      Running   │ │
│  │ 🔴 payment    #c4d7  step:charge      5s ago   Failed     │ │
│  │ 🔵 onboarding #d123  step:send-email   now      Running   │ │
│  │ 🟢 order-proc #e456  step:confirm     8s ago   Succeeded  │ │
│  │                                    [View All Runs →]       │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                 │
│  ┌─ ALERT FEED ──────────┐  ┌─ THROUGHPUT (24h) ────────────┐ │
│  │ ⚠ payment: 3 failures │  │  ┌──╮   ╭──╮                  │ │
│  │   in last 5 min        │  │  │  ╰───╯  ╰──╮   ╭──        │ │
│  │   [View] [Retry All]   │  │  │             ╰───╯          │ │
│  │                        │  │  └────────────────────────→   │ │
│  │ ⚠ kyc-verify: avg      │  │   00:00        12:00   now   │ │
│  │   latency > 30s        │  │                               │ │
│  │   [View]               │  │  succeeded ── failed ──       │ │
│  └────────────────────────┘  └───────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

**Key behaviors:**
- Summary cards are clickable — clicking "FAILED: 3" navigates to Runs filtered by `status=failed`.
- Live run ticker streams via WebSocket, most recent at top. Auto-scrolls unless user hovers (pause scroll).
- Alert feed is rule-based (configured in Settings). Surfaces anomalies: failure spikes, latency regressions, stuck runs.
- Throughput chart is a sparkline for the last 24h, updated every 10s.

### 3.2 Runs List

**Purpose:** Find, filter, and act on workflow runs at scale.

```
┌─────────────────────────────────────────────────────────────────┐
│  Saved Views: [All] [Running] [Failed] [Stuck >5m] [+Custom]   │
│                                                                 │
│  ┌─ FILTERS ──────────────────────────────────────────────────┐ │
│  │ Workflow: [Any ▾]  Status: [Any ▾]  Time: [Last 1h ▾]     │ │
│  │ Search: [run ID, foreign ID, or workflow name...      🔍]  │ │
│  │ [+ Add filter]                         [Save as View 💾]   │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                 │
│  ┌─ RESULTS (2,847 runs) ─────── sorted by: Started ↓ ──────┐ │
│  │ Status │ Run ID   │ Workflow      │ Step          │ Start  │ │
│  │────────│──────────│──────────────│───────────────│────────│ │
│  │ 🔴 Fail│ c4d7…   │ payment      │ charge        │ 2s ago │ │
│  │        │          │              │ ↳ Retry (2/3) │        │ │
│  │ 🔵 Run │ b891…   │ kyc-verify   │ face-match    │ 5s ago │ │
│  │ 🔵 Run │ d123…   │ onboarding   │ send-email    │ 8s ago │ │
│  │ 🟢 Done│ e456…   │ order-proc   │ ✓ (4/4 steps) │ 12s ago│ │
│  │ ⏸ Pause│ f789…   │ refund       │ await-approval│ 3m ago │ │
│  │ 🔵 Run │ g012…   │ order-proc   │ validate      │ 15s ago│ │
│  │                                                            │ │
│  │ [Load more...]        Streaming: 🟢 Live  [Pause stream]  │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                 │
│  Bulk actions: □ Select all  [Retry Selected] [Cancel Selected]│
└─────────────────────────────────────────────────────────────────┘
```

**Key behaviors:**
- **Streaming pagination:** New runs appear at the top in real-time (WebSocket). Historical runs load via cursor-based pagination on scroll.
- **Saved views:** Predefined system views (All, Running, Failed, Stuck) plus user-created custom views with arbitrary filter combos.
- **Inline context:** The "Step" column shows the current step for running workflows or the failed step for failed ones. Avoids needing to click into every run.
- **Bulk actions:** Multi-select with checkboxes. Retry/cancel/pause in batch.
- **Row click** → opens Run Detail as a slide-over panel (not full navigation), so the list context is preserved.

### 3.3 Run Detail

**Purpose:** Understand exactly what happened in a single run, and take action.

```
┌─────────────────────────────────────────────────────────────────┐
│  ← Back to Runs                                                 │
│                                                                 │
│  Run: c4d7…f2a1          Workflow: payment                     │
│  Foreign ID: order-9382  Status: 🔴 Failed                     │
│  Started: 2025-03-07 14:23:01   Duration: 4.2s                │
│                                                                 │
│  [Retry] [Retry from Failed] [Cancel] [Pause] [View Payload]   │
│                                                                 │
│  ┌─ VIEW TOGGLE ─────────────────────────────────────────────┐ │
│  │ [Timeline ◉] [Steps ○] [Graph ○]                          │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                 │
│  ── TIMELINE VIEW ──────────────────────────────────────────── │
│                                                                 │
│  validate-input  ████████░░░░░░░░░░░░░░░░░░  120ms  ✓        │
│  check-balance   ░░░░░░░░████████░░░░░░░░░░  340ms  ✓        │
│  charge-card     ░░░░░░░░░░░░░░░░█████████▓  1.2s   ✗        │
│                                   ↑ retry 1  ↑ retry 2 (fail)│
│  notify-user     ░░░░░░░░░░░░░░░░░░░░░░░░░░  ──     skipped  │
│  ├──────────────────────────────────────────────────────────┤ │
│  0s              1s              2s              3s     4.2s  │
│                                                                 │
│  ── SELECTED: charge-card ─────────────────────────────────── │
│  │ Status: Failed (after 2 retries)                           │ │
│  │ Error: "insufficient_funds" (code: 402)                    │ │
│  │ Input:  {"card_id": "visa_8832", "amount": 149.99}        │ │
│  │ Output: null                                               │ │
│  │ Duration: 1.2s (attempt 1: 580ms, attempt 2: 620ms)       │ │
│  │ [Retry This Step] [View Full Payload] [Copy Error]         │ │
│  └────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

**Three view modes:**

1. **Timeline** (default) — Gantt-style horizontal bars showing step duration and parallelism. Inspired by Temporal's timeline view. Color-coded by outcome. Retry attempts shown as segments within the bar.

2. **Steps** — Vertical list, each step expandable to show input/output/error/duration. Best for sequential debugging.

3. **Graph** — DAG or state-machine visualization. Nodes are steps, edges are dependencies/transitions. Live animation: current step pulses, completed steps are solid, failed are red-outlined. For state machines, shows the path taken through the state graph.

**Action panel:**
- **Retry** — re-run the entire workflow from scratch
- **Retry from Failed** — re-run starting from the first failed step (requires engine support)
- **Cancel** — terminate the run
- **Pause/Resume** — suspend execution (for engines that support it)
- **Skip** — mark a failed step as skipped and continue

### 3.4 Analytics

**Purpose:** Spot trends and systemic issues.

```
┌─────────────────────────────────────────────────────────────────┐
│  Time range: [Last 24h ▾]  Workflow: [All ▾]  Granularity: [5m]│
│                                                                 │
│  ┌─ THROUGHPUT ──────────────────────────────────────────────┐ │
│  │  Stacked area chart: succeeded / failed / running          │ │
│  │  Y-axis: runs/minute    X-axis: time                       │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                 │
│  ┌─ LATENCY (p50/p95/p99) ───┐  ┌─ FAILURE RATE ────────────┐ │
│  │  Line chart, 3 percentile  │  │  Line chart, % of runs    │ │
│  │  lines with fill between   │  │  that failed per interval │ │
│  └────────────────────────────┘  └───────────────────────────┘ │
│                                                                 │
│  ┌─ STEP DURATION & LATENCY ────────────────────────────────┐ │
│  │  Scope: [All workflows ▾ / Single workflow]                │ │
│  │                                                            │ │
│  │  Step Name      │ Queue│ Exec │ p50  │ p95  │ p99  │Trend │ │
│  │  ────────────────│──────│──────│──────│──────│──────│──────│ │
│  │  charge-card  🔴│▒▒▒   │██████│ 340  │1,200 │3,800 │ ╱╲╱  │ │
│  │  face-match     │▒▒    │████  │ 220  │ 580  │1,100 │ ──╱  │ │
│  │  validate       │▒     │██    │  45  │  90  │ 180  │ ───  │ │
│  │  notify         │▒     │█     │  30  │  60  │ 120  │ ───  │ │
│  │                                                            │ │
│  │  ▒▒ = queue wait    ██ = execution     🔴 = bottleneck     │ │
│  │                                                            │ │
│  │  ┌─ SELECTED: charge-card  trend (7d) ──────────────────┐ │ │
│  │  │  p50 ───   p95 ─·─   p99 ···                         │ │ │
│  │  │     ╭──╮                                              │ │ │
│  │  │  ───╯  ╰──╮   ╭─·─·─╮                                │ │ │
│  │  │            ╰───╯     ╰──                              │ │ │
│  │  │  Mon   Tue   Wed   Thu   Fri   Sat   Sun              │ │ │
│  │  └───────────────────────────────────────────────────────┘ │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                 │
│  ┌─ STEP HEATMAP ────────────────────────────────────────────┐ │
│  │  Rows: step names     Columns: time buckets                │ │
│  │  Cell color: failure rate (green → yellow → red)           │ │
│  │  Cell tooltip: count, avg duration, error breakdown        │ │
│  │                                                            │ │
│  │  validate    [  ] [  ] [  ] [  ] [  ] [  ] [██] [  ]      │ │
│  │  check-bal   [  ] [  ] [  ] [  ] [  ] [  ] [  ] [  ]      │ │
│  │  charge      [  ] [██] [██] [▓▓] [  ] [  ] [  ] [  ]      │ │
│  │  notify      [  ] [  ] [  ] [  ] [  ] [  ] [  ] [  ]      │ │
│  └────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

#### Step Duration & Latency Card — Detail

This is the most data-rich card in the analytics suite. It combines four metrics in a single interactive view:

**Layout:** A combo table-chart hybrid. Each row represents a step (sorted by p95 descending — bottlenecks first). Columns:

| Column | Content | Visual |
|--------|---------|--------|
| **Step Name** | Name + bottleneck indicator (🔴 if p95 > 2× average across steps) | Text + optional badge |
| **Queue / Exec bars** | Stacked horizontal bar split into queue wait (hatched) and execution time (solid) | Proportional to p50 duration |
| **p50 / p95 / p99** | Numeric values in ms or seconds | Monospace, color-coded (green < amber < red thresholds) |
| **Trend sparkline** | 7-day p50 trend as an inline sparkline | Mini line chart, ~80px wide |

**Interactions:**
- **Click a row** → Expands an inline detail panel below showing a full time-series chart for that step (p50/p95/p99 lines over the selected time range). This replaces navigation — the user stays in context.
- **Hover on queue/exec bar** → Tooltip shows exact breakdown: "Queue: 45ms (13%) · Exec: 295ms (87%)"
- **Hover on sparkline** → Crosshair tooltip shows the value at that point in time.
- **Sort controls** → Click any column header to re-sort (by p50, p95, p99, or trend direction).
- **Bottleneck auto-detection** — Steps where p95 > 2× the cross-step median are flagged with 🔴 and pushed to the top regardless of sort.

**Scope toggle:**
- **All workflows (aggregate)** — Steps are aggregated across all workflows. Useful for spotting systemic infra issues (e.g., all "notify" steps are slow because the email service is degraded).
- **Single workflow (dropdown)** — Shows only steps for the selected workflow. Useful for optimizing a specific pipeline.

**Percentile color thresholds** (configurable per deployment):

| Threshold | Color | Meaning |
|-----------|-------|---------|
| p95 < 500ms | Green | Healthy |
| p95 500ms–2s | Amber | Elevated |
| p95 > 2s | Red | Degraded |

**Queue wait vs execution time:**
- **Queue wait** = time between "step scheduled" and "step started" (time spent waiting for a worker/goroutine slot).
- **Execution time** = time between "step started" and "step completed/failed" (actual work).
- A high queue-to-exec ratio signals backpressure or concurrency starvation. The UI highlights this with a warning icon when queue > 50% of total.
```

**Step heatmap** is the most valuable analytics view — it immediately shows which step, at what time, is causing problems. Clicking a hot cell drills into the Runs list pre-filtered by that step + time range.

### 3.5 Subsystem Health (Dashboard Cards)

The Dashboard includes a row of subsystem health cards below the global stats, showing per-subsystem operational posture at a glance.

```
┌─ SUBSYSTEM HEALTH ────────────────────────────────────────────┐
│                                                                │
│  ┌─ order-fulfillment ─┐  ┌─ billing ──────────┐  ┌─ auth ─┐ │
│  │  🟢 Healthy          │  │  🔴 Degraded        │  │  🟢 OK  │ │
│  │  Running: 12         │  │  Running: 3          │  │  Run: 2 │ │
│  │  Failed/1h: 0        │  │  Failed/1h: 7        │  │  Fail: 0│ │
│  │  Rate: 34/min        │  │  Rate: 12/min        │  │  8/min  │ │
│  │  Err: 0.0%           │  │  Err: 14.2% ▲        │  │  0.0%   │ │
│  │  Workflows: 4        │  │  Workflows: 3        │  │  WFs: 2 │ │
│  │  [View →]            │  │  [View →]            │  │  [→]    │ │
│  └──────────────────────┘  └──────────────────────┘  └────────┘ │
└────────────────────────────────────────────────────────────────┘
```

**Health status logic:**
- 🟢 **Healthy** — error rate < 1% in last hour
- 🟡 **Warning** — error rate 1–5% or avg latency > 2× baseline
- 🔴 **Degraded** — error rate > 5% or any step fully failing
- ⚫ **Offline** — no runs in last 15 minutes (unexpected for active subsystems)

Clicking a subsystem card navigates to the Subsystem Detail page, which shows subsystem-scoped analytics + workflow list + recent runs.

### 3.6 Trace Waterfall View

**Purpose:** Visualize a distributed transaction across multiple workflow runs and subsystems.

```
┌─────────────────────────────────────────────────────────────────┐
│  Trace: 4bf92f3577b34da6a3ce929d0e0e4736                        │
│  Duration: 2.4s  ·  3 subsystems  ·  5 runs  ·  14 steps       │
│                                                                 │
│  ── WATERFALL ──────────────────────────────────────────────── │
│                                                                 │
│  Subsystem         Run / Step           0ms     500    1000   2400│
│  ─────────────     ─────────────        │       │      │      │ │
│                                         │       │      │      │ │
│  order-fulfillment                      │       │      │      │ │
│  ├─ order-proc     ████████████████░░░░░│░░░░░░░│░░░░░░│      │ │
│  │  ├─ validate    ███░░░░░░░░░░░░░░░░░░│░░░░░░░│░░░░░░│      │ │
│  │  ├─ reserve     ░░░████░░░░░░░░░░░░░░│░░░░░░░│░░░░░░│      │ │
│  │  └─ confirm     ░░░░░░░██░░░░░░░░░░░░│░░░░░░░│░░░░░░│      │ │
│  │                                      │       │      │      │ │
│  billing                                │       │      │      │ │
│  ├─ payment-charge ░░░░░░░░░████████████│██░░░░░│░░░░░░│      │ │
│  │  ├─ fraud-check ░░░░░░░░░████░░░░░░░░│░░░░░░░│░░░░░░│      │ │
│  │  ├─ authorize   ░░░░░░░░░░░░░████░░░░│░░░░░░░│░░░░░░│      │ │
│  │  └─ capture     ░░░░░░░░░░░░░░░░░████│██░░░░░│░░░░░░│      │ │
│  │                                      │       │      │      │ │
│  notifications                          │       │      │      │ │
│  └─ send-notifs    ░░░░░░░░░░░░░░░░░░░░░│░░█████│██████│████  │ │
│     ├─ email       ░░░░░░░░░░░░░░░░░░░░░│░░████░│░░░░░░│      │ │
│     └─ push        ░░░░░░░░░░░░░░░░░░░░░│░░░░░░░│█████░│      │ │
│                                         │       │      │      │ │
│  ─────────────────────────────────────────────────────────────│ │
│  0ms               500ms         1000ms          2000ms  2400ms│ │
│                                                                 │
│  Legend: ████ execution  ▒▒▒▒ queue wait  🔴 failed step       │
│  Color by subsystem: 🔵 order  🟣 billing  🟠 notifications    │
└─────────────────────────────────────────────────────────────────┘
```

**Behaviors:**
- **Swimlanes by subsystem** — each subsystem gets a distinct color and visual grouping. Runs are nested under their subsystem.
- **Parent-child nesting** — runs triggered by other runs are indented and connected with a dashed dependency line.
- **Step-level drill-down** — clicking a run in the waterfall expands its steps inline (shown above). Click again to collapse.
- **Hover tooltip** — shows exact timing: start, end, duration, queue wait, status.
- **Zoom + pan** — for long traces (>5s), horizontal scroll with pinch-to-zoom. A minimap at the bottom shows the full trace with a viewport indicator.
- **Critical path highlighting** — the longest sequential chain of dependencies is highlighted with a thicker bar outline, showing what actually determines the trace's total duration.

**Entry points to Trace Waterfall:**
- Click a trace ID badge on any run in the Runs list
- Click "View Trace" in the Run Detail panel
- Search for a trace ID via `Cmd+K` or `trace:4bf92f...` in the query language
- Trace links in log entries and error details

---

## 4. Abstract API Contract

### 4.1 Data Model

```
WorkflowDefinition {
  id:          string          // "order-processing"
  name:        string          // "Order Processing"
  engine:      string          // "go-workflow" | "luno-workflow" | "temporal"
  version:     string
  steps:       StepDefinition[]
  edges:       Edge[]          // for DAG engines
  transitions: Transition[]    // for state-machine engines
  created_at:  timestamp
}

Run {
  id:           string         // unique run ID
  workflow_id:  string
  foreign_id:   string         // business entity ID (e.g., order ID)
  status:       RunStatus      // pending|running|succeeded|failed|canceled|paused
  current_step: string | null
  started_at:   timestamp
  finished_at:  timestamp | null
  duration_ms:  int | null
  error:        string | null
  metadata:     map<string, string>
}

StepExecution {
  id:           string
  run_id:       string
  step_name:    string
  status:       StepStatus     // pending|running|succeeded|failed|skipped|canceled
  attempt:      int            // retry attempt number
  started_at:   timestamp
  finished_at:  timestamp | null
  duration_ms:  int | null
  input:        json | null
  output:       json | null
  error:        ErrorDetail | null
}

ErrorDetail {
  message:     string
  code:        string | null
  stack_trace: string | null
}

SearchResult {
  id:            string                // result ID (run ID, step ID, or log line ref)
  type:          ResultType            // run|step|log|payload
  run_id:        string
  workflow:      string
  score:         float                 // relevance score
  highlight:     map<string, string[]> // field → highlighted fragments
  context: {                           // surrounding context for the match
    run_status:  RunStatus
    step_name:   string | null
    timestamp:   timestamp
  }
}

SearchResponse {
  results:     SearchResult[]
  facets: {                            // aggregated counts for filter sidebar
    status:    map<string, int>        // { "failed": 24, "running": 3 }
    workflow:  map<string, int>
    step:      map<string, int>
    error:     map<string, int>        // top error codes/messages
  }
  total:       int
  cursor:      string | null
  query_ms:    int                     // search latency for display
}

SearchQuery {
  raw:         string                  // raw query string
  parsed: {                            // structured representation
    text:      string | null           // free-text portion
    filters:   Filter[]                // { field, op, value }
    sort:      SortField | null
    time_range: TimeRange | null
  }
}

LogEntry {
  run_id:      string
  step_name:   string
  line_number: int
  timestamp:   timestamp
  level:       string                  // debug|info|warn|error
  message:     string
  source:      string                  // stdout|stderr
}
```

### 4.2 API Layer — gRPC + Connect-Go + OpenAPI

The FlowWatch API is **gRPC-first**, defined in Protocol Buffers and served via
[Connect-Go](https://connectrpc.com). This provides three transport options from
a single proto definition:

| Transport | Protocol | Use Case |
|-----------|----------|----------|
| **gRPC** | HTTP/2, binary protobuf | Backend service-to-service |
| **gRPC-Web** | HTTP/1.1 | Browser clients without proxy |
| **Connect** | HTTP/1.1, JSON or binary | Simplest for `curl` / `fetch` / SSE |

Server-streaming RPCs (StreamService) are exposed as **Server-Sent Events (SSE)**
over the Connect protocol, making them consumable from browsers via native
`EventSource` or `fetch()` — no WebSocket library needed.

#### Proto Package Structure

```
proto/flowwatch/v1/
├── types.proto              # shared enums + domain messages
├── workflow_service.proto   # definitions, graph, capabilities
├── run_service.proto        # CRUD, actions, bulk ops, logs
├── search_service.proto     # full-text search, suggest, explain
├── analytics_service.proto  # throughput, latency, step-duration, heatmap
└── stream_service.proto     # real-time server-streaming RPCs
```

#### Services

| Service | RPCs | Description |
|---------|------|-------------|
| **WorkflowService** | `ListWorkflows`, `GetWorkflow`, `GetGraph`, `GetCapabilities` | Workflow definitions + engine feature negotiation |
| **RunService** | `ListRuns`, `GetRun`, `GetRunSteps`, `GetRunTimeline`, `GetRunLogs`, `RetryRun`, `RetryFromStep`, `CancelRun`, `PauseRun`, `ResumeRun`, `SkipStep`, `BulkRetry`, `BulkCancel` | Full run lifecycle management |
| **SearchService** | `Search`, `Suggest`, `Explain`, `Reindex`, `GetIndexStats` | Full-text search across runs/steps/payloads/logs |
| **AnalyticsService** | `GetThroughput`, `GetLatency`, `GetFailureRate`, `GetStepDuration`, `GetStepHeatmap` | Pre-aggregated metrics for dashboards |
| **StreamService** | `WatchRuns` (stream), `WatchRun` (stream), `WatchStats` (stream) | Real-time event subscriptions via SSE |

#### OpenAPI Documentation

The proto definitions are converted to an OpenAPI v3 spec and served via Redocly:

```
make proto      # generate Go + Connect-Go stubs
make openapi    # convert proto → OpenAPI v3
make serve-docs # start Redocly dev server on :8088
```

The OpenAPI spec is auto-generated from protos using `buf generate` with the
`grpc-ecosystem/openapiv2` plugin, then converted to v3 via `swagger2openapi`.
Redocly serves the spec with a themed, interactive doc site matching the
FlowWatch brand palette.

#### Connect-Go Endpoint Convention

All RPCs are available at predictable HTTP paths:

```
POST /flowwatch.v1.RunService/ListRuns          # Connect protocol (JSON)
POST /flowwatch.v1.RunService/RetryRun          # Connect protocol (JSON)
POST /flowwatch.v1.SearchService/Search         # Connect protocol (JSON)
GET  /flowwatch.v1.StreamService/WatchRuns      # SSE stream (Connect)
GET  /flowwatch.v1.StreamService/WatchStats     # SSE stream (Connect)
```

The UI consumes these as standard `fetch()` calls with JSON bodies. No gRPC
client library is needed in the browser.

---

## 5. Component Architecture

### 5.1 Component Tree

```
<App>
  <ThemeProvider>                          // dark/light mode
  <WebSocketProvider>                      // shared WS connection
  <Router>
    <Shell>
      <Sidebar />                          // primary navigation
      <Breadcrumb />                       // contextual path
      <SearchPalette />                    // Cmd+K global quick-search
        <SearchInput />                    //   debounced text input
        <RecentSearches />                 //   last 10 searches from localStorage
        <SearchResultGroups />             //   grouped: runs, steps, logs
          <SearchResultItem />             //   single result with highlight
        <SearchFooter />                   //   keyboard hints + timing

      <DashboardPage>
        <StatsCardRow />                   // 5 summary metric cards
        <LiveRunTicker />                  // streaming run feed
        <AlertFeed />                      // rule-based anomaly alerts
        <ThroughputSparkline />            // 24h mini chart
      </DashboardPage>

      <RunsPage>
        <SavedViewBar />                   // saved filter presets
        <AdvancedSearchPanel>              // full search + filter system
          <SearchBox />                    //   full-text search input
          <FacetFilterRow />               //   visual dropdown filters
            <FacetDropdown />              //   per-field filter (status, workflow, etc.)
          <ActiveFilterPills />            //   removable filter badges
          <SearchModeToggle />             //   visual ↔ raw query switch
          <RawQueryEditor />               //   single-line query with syntax highlight
            <QueryAutoComplete />          //   field + value suggestions
            <QueryValidation />            //   inline error underlines
          <SearchMeta />                   //   result count + latency + save button
        </AdvancedSearchPanel>
        <RunsTable />                      // virtualized, streaming
          <RunRow />                       // single run row
          <MatchContextColumn />           // shown when search active — highlights
        <BulkActionBar />                  // multi-select actions
        <RunDetailPanel />                 // slide-over detail
          <RunHeader />                    // ID, status, actions
          <ViewToggle />                   // timeline/steps/graph
          <TimelineView />                 // Gantt-style bars
          <StepListView />                 // expandable step list
          <GraphView />                    // DAG/FSM visualization
          <StepDetail />                   // input/output/error
          <ActionPanel />                  // retry/cancel/pause/skip
          <PayloadInspector />             // JSON tree viewer
      </RunsPage>

      <WorkflowsPage>
        <WorkflowList />
        <WorkflowDetail>
          <GraphEditor />                  // read-only DAG/FSM view
          <ActiveRunsSummary />
          <WorkflowAnalytics />
        </WorkflowDetail>
      </WorkflowsPage>

      <AnalyticsPage>
        <TimeRangeSelector />
        <ThroughputChart />                // stacked area
        <LatencyChart />                   // percentile lines
        <FailureRateChart />               // trend line
        <StepHeatmap />                    // failure heatmap grid
      </AnalyticsPage>
    </Shell>
  </Router>
  </WebSocketProvider>
  </ThemeProvider>
</App>
```

### 5.2 Key Technical Decisions

| Concern | Approach | Rationale |
|---------|----------|-----------|
| **Run list at scale** | Virtualized table (e.g., TanStack Virtual) | Thousands of rows; only render visible |
| **Real-time updates** | WebSocket with reconnect + exponential backoff | SSE alternative for simpler deployments |
| **DAG rendering** | Dagre layout + SVG/Canvas renderer | Works for both DAG and state-machine topologies |
| **Timeline view** | Custom canvas-based Gantt | HTML/DOM too slow for 50+ step timelines |
| **JSON payloads** | Lazy-loaded, collapsible tree viewer | Payloads can be large; don't fetch until expanded |
| **Filtering** | Server-side with cursor pagination | Client-side filtering impossible at scale |
| **State management** | Reactive store with WS merge | WS events patch local state; no full refetch |
| **URL routing** | Deep-linkable filters and views | Share a URL that reproduces exact filter state |

---

## 6. Engine Adapter Pattern (Backend)

The Go backend implements engine-specific adapters behind a unified interface:

```go
// Core interface — each engine implements this
type EngineAdapter interface {
    // Queries
    ListWorkflows(ctx context.Context) ([]WorkflowDefinition, error)
    ListRuns(ctx context.Context, filter RunFilter, cursor string, limit int) ([]Run, string, error)
    GetRun(ctx context.Context, runID string) (*RunDetail, error)
    GetRunSteps(ctx context.Context, runID string) ([]StepExecution, error)
    GetGraph(ctx context.Context, workflowID string) (*Graph, error)

    // Actions
    RetryRun(ctx context.Context, runID string) error
    RetryFromStep(ctx context.Context, runID, stepName string) error
    CancelRun(ctx context.Context, runID string) error
    PauseRun(ctx context.Context, runID string) error
    ResumeRun(ctx context.Context, runID string) error
    SkipStep(ctx context.Context, runID, stepName string) error

    // Streaming
    SubscribeRuns(ctx context.Context, filter RunFilter) (<-chan RunEvent, error)

    // Analytics
    GetThroughput(ctx context.Context, params AnalyticsParams) ([]TimeseriesPoint, error)
    GetLatency(ctx context.Context, params AnalyticsParams) ([]PercentilesPoint, error)
    GetStepHeatmap(ctx context.Context, params AnalyticsParams) ([]HeatmapCell, error)
    GetStepDuration(ctx context.Context, params StepDurationParams) (*StepDurationReport, error)
}

// SearchIndex is a separate interface — not engine-specific.
// The search index sits alongside the engine adapter and is fed
// by an async ingestion pipeline that reads engine events.
type SearchIndex interface {
    // Query executes a parsed search query against the index
    Search(ctx context.Context, query SearchQuery, cursor string, limit int) (*SearchResponse, error)

    // Suggest returns auto-complete suggestions for a field
    Suggest(ctx context.Context, field string, prefix string, limit int) ([]string, error)

    // Explain parses a raw query string into a structured AST
    // Used for visual ↔ raw query conversion
    Explain(ctx context.Context, raw string) (*SearchQuery, error)

    // Ingest adds or updates a document in the index
    IngestRun(ctx context.Context, run *RunDetail, steps []StepExecution, logs []LogEntry) error

    // Admin
    Reindex(ctx context.Context, filter ReindexFilter) error
    Stats(ctx context.Context) (*IndexStats, error)
}

// Implementations:
// - BleveSearchIndex    — embedded, zero-infra, good for <100k runs
// - MeilisearchIndex    — standalone, fast, good for <1M runs
// - ElasticsearchIndex  — distributed, good for >1M runs

// Ingestion pipeline reads engine events and feeds the search index:
//
//   EngineAdapter.SubscribeRuns()
//       │
//       ▼
//   SearchIngester (goroutine)
//       │ — on run.completed / run.failed:
//       │     fetch steps + payloads + logs
//       │     call SearchIndex.IngestRun()
//       │
//       ▼
//   SearchIndex (Bleve / Meilisearch / Elasticsearch)

// Implementations
type GoWorkflowAdapter struct { ... }    // wraps Azure/go-workflow
type LunoWorkflowAdapter struct { ... }  // wraps luno/workflow RecordStore + EventStreamer
type TemporalAdapter struct { ... }      // wraps Temporal gRPC client
```

### Capability Negotiation

Not all engines support all actions. The API exposes capabilities per engine:

```json
GET /api/v1/capabilities
{
  "retry_run": true,
  "retry_from_step": false,     // go-workflow doesn't support this
  "cancel_run": true,
  "pause_run": false,           // go-workflow doesn't support this
  "skip_step": true,
  "streaming": true,
  "analytics": true,
  "search": {
    "enabled": true,
    "engine": "bleve",          // bleve | meilisearch | elasticsearch
    "scopes": ["runs", "steps", "payloads", "logs"],
    "query_language": true,     // raw query syntax supported
    "max_results": 10000,
    "payload_indexed": true,
    "log_indexed": true,
    "log_retention_days": 7,
    "payload_retention_days": 30
  }
}
```

The UI disables/hides action buttons based on capabilities. No dead buttons.

---

## 7. Interaction Patterns

### 7.1 Real-Time Run List

- New runs appear at the top with a subtle slide-in animation.
- Status changes trigger an in-place badge color transition (no full row re-render).
- If the user is scrolled down (viewing historical runs), new runs queue in a "12 new runs" floating pill at the top. Clicking it scrolls to top and flushes the queue.
- User can pause live streaming to investigate a specific set of runs without the list shifting under them.

### 7.2 Run Detail Slide-Over

- Opens as a right-side panel (60% width) over the run list. List remains visible and interactive on the left.
- Keyboard navigation: `↑`/`↓` to move between runs in the list while the detail panel stays open (updates to show the newly selected run).
- `Esc` closes the panel.
- Deep-linkable: URL updates to `/runs/c4d7` when panel opens.

### 7.3 Retry Flows

- **Single retry:** Click "Retry" → confirmation dialog with run summary → execute → run list updates in real-time showing new run.
- **Retry from step:** Click failed step → "Retry from here" → confirmation showing which steps will re-execute → execute.
- **Bulk retry:** Select multiple failed runs → "Retry Selected (7)" → confirmation → progress indicator showing "3/7 retried..."

### 7.4 Full-Text Search System

The search system operates in two tiers: a **global quick-search** accessible from anywhere via `Cmd+K`, and an **advanced inline search** embedded in the Runs page with faceted filtering and a raw query language.

#### Search Scope — What Is Indexed

All searchable content is ingested into a full-text search index (Bleve, Meilisearch, or Elasticsearch depending on deployment scale).

| Layer | Fields Indexed | Examples |
|-------|---------------|----------|
| **Run metadata** | run ID, workflow name, foreign ID, status, tags, metadata keys/values | `ord-9382`, `payment-charge`, `failed` |
| **Step-level data** | step name, step status, error message, error code | `charge-card`, `insufficient_funds`, `timeout` |
| **Payloads** | step input JSON, step output JSON (flattened key paths + values) | `card_id:visa_8832`, `amount:149.99`, `user.email:jane@...` |
| **Logs** | stdout/stderr log lines attached to step executions | `WARN: retrying connection`, `panic: nil pointer` |

Payload and log indexing is **async** — content is ingested into the search index after run completion to avoid impacting execution latency. A configurable TTL controls how long deep content remains indexed (default: 30 days for payloads, 7 days for logs).

#### Tier 1: Global Quick-Search (`Cmd+K` Palette)

A modal overlay accessible from any page via `Cmd+K` (Mac) / `Ctrl+K` (Windows/Linux).

```
┌─────────────────────────────────────────────────────────────┐
│  🔍  Search runs, workflows, errors...              ⌘K     │
│─────────────────────────────────────────────────────────────│
│                                                             │
│  RECENT SEARCHES                                            │
│  ○  status:failed workflow:payment                          │
│  ○  insufficient_funds                                      │
│  ○  ord-9382                                                │
│                                                             │
│  ─────── after typing "insuffic" ───────                    │
│                                                             │
│  RUNS (3 matches)                                           │
│  🔴 c4d7…  payment-charge    "insufficient_funds"  2m ago  │
│  🔴 a891…  payment-charge    "insufficient_funds"  18m ago │
│  🔴 f234…  refund-process    "insufficient_funds"  1h ago  │
│                                                             │
│  STEPS (5 matches)                                          │
│  ↳ charge-card in c4d7…  error: insufficient_funds          │
│  ↳ charge-card in a891…  error: insufficient_funds          │
│                                                             │
│  LOGS (12 matches)                                          │
│  ↳ c4d7… charge-card  "ERR insufficient_funds for visa_88" │
│  ↳ a891… charge-card  "ERR insufficient_funds for mc_2201" │
│                                                             │
│  [Enter] to open  ·  [↑↓] to navigate  ·  [⇥] Advanced    │
│  [Esc] to close   ·  Results: 20 in 42ms                   │
└─────────────────────────────────────────────────────────────┘
```

**Behaviors:**
- **Debounced search** — Fires after 150ms of no typing. Shows a skeleton loader during fetch.
- **Grouped results** — Results are categorized: Runs, Steps, Logs, Workflows. Each group shows top 3, with a "View all N matches" link.
- **Result actions** — `Enter` opens the selected result (run detail, step detail, or log viewer). `Tab` pivots to the advanced search on the Runs page with the current query pre-filled.
- **Recent + saved queries** — Last 10 searches are persisted in localStorage. Starred queries persist across sessions.
- **Keyboard-first** — Full `↑`/`↓` navigation, `Enter` to select, `Esc` to dismiss. No mouse required.
- **Highlight matching** — Matched text fragments are highlighted in the results with a subtle accent background.

#### Tier 2: Advanced Inline Search (Runs Page)

Embedded in the Runs page filter panel. Combines a text search box, visual faceted filters, and a raw query toggle for power users.

```
┌─────────────────────────────────────────────────────────────────┐
│  Saved Views: [All] [Running] [Failed] [Stuck >5m] [+Custom]   │
│                                                                 │
│  ┌─ SEARCH & FILTERS ────────────────────────────────────────┐ │
│  │                                                            │ │
│  │  🔍 [Full-text search across runs, steps, payloads, logs] │ │
│  │                                                            │ │
│  │  Facets:                                                   │ │
│  │  [Status ▾ Any]  [Workflow ▾ Any]  [Time ▾ Last 1h]       │ │
│  │  [Step ▾ Any]    [Error contains ▾]  [+ Add facet]        │ │
│  │                                                            │ │
│  │  Active filters:  [status:failed ×] [workflow:payment ×]  │ │
│  │                                                            │ │
│  │  ┌──── Mode: [Visual ◉] [Query ○] ────────────────────┐  │ │
│  │  │ (toggle to raw query language)                       │  │ │
│  │  └─────────────────────────────────────────────────────┘  │ │
│  │                                                            │ │
│  │  Results: 2,847 runs  ·  Searched in 38ms   [Save View 💾]│ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                 │
│  ┌─ RESULTS ─────────────────────────────────────────────────┐ │
│  │  (same RunsTable as before, but filtered by search)       │ │
│  │                                                            │ │
│  │  When a text search is active, a "Match Context" column   │ │
│  │  appears showing the matched fragment:                    │ │
│  │                                                            │ │
│  │  Status │ ID   │ Workflow │ Match Context          │ Time │ │
│  │  🔴 Fail│ c4d7 │ payment  │ error:"insuffic…funds" │ 2m   │ │
│  │  🔴 Fail│ a891 │ payment  │ log:"ERR insuffic…"    │ 18m  │ │
│  │  🔴 Fail│ f234 │ refund   │ output.reason:"insuf…" │ 1h   │ │
│  └────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

**Visual mode (default):** Faceted filters render as dropdown selectors. Each selected filter becomes a removable pill. Filters compose with AND logic. The text search box performs full-text search across all indexed content. Visual mode is designed for non-technical users.

**Raw query mode (toggle):** Replaces the visual filters with a single-line query editor supporting a structured query language:

```
# Query language examples

# Simple text search (searches all fields)
insufficient_funds

# Field-scoped search
error:timeout
step:charge-card
workflow:payment
log:"connection refused"
payload.user.email:jane@example.com

# Boolean operators
status:failed AND workflow:payment
error:timeout OR error:rate_limited
NOT status:succeeded

# Time ranges
started:>2025-03-07T10:00:00Z
duration:>5000ms
started:last_1h

# Numeric comparisons on payload fields
payload.amount:>100.00

# Wildcards
error:insuffic*
workflow:order-*

# Combining everything
status:failed AND workflow:payment AND error:insuffic* AND started:last_24h
```

**Query language features:**
- Auto-complete on field names (`status:`, `workflow:`, `step:`, `error:`, `log:`, `payload.`)
- Auto-complete on known values (workflow names, status enums, step names)
- Syntax highlighting in the query editor (field names in accent color, strings in green, operators in bold)
- Inline validation — invalid syntax underlined in red with tooltip explanation
- One-click conversion between visual filters and raw query (bidirectional)

#### Search Result Highlighting

When a text search is active anywhere in the UI, matched content is highlighted:

- **Runs table:** A "Match Context" column appears showing the specific fragment that matched, with the matching text wrapped in a highlight span. The column shows the source layer (error, log, payload, metadata).
- **Run detail:** When opened from a search result, the matching step is auto-expanded and the matched text is highlighted within the payload inspector or log viewer.
- **Log viewer:** Matched log lines are pinned to the top of the log view with surrounding context lines (3 lines above/below). Non-matching lines are dimmed but visible.

#### Search Persistence & Sharing

- **URL serialization:** The full search query is encoded in the URL query string: `/runs?q=status:failed AND error:timeout&view=custom`. Shareable across team members.
- **Saved views include search:** When saving a view, the full query (text search + facets) is persisted as part of the view definition.
- **Export search results:** A "Download CSV" button exports the current filtered/searched result set (metadata only, not payloads/logs, to avoid data exfiltration concerns).

### 7.5 Accessibility

- Full keyboard navigation for all interactive elements.
- ARIA labels on status badges (color is not the only indicator — icons + text labels).
- Screen-reader-friendly table with proper `role` attributes.
- High-contrast mode for color-blind users (status indicated by shape: ✓, ✗, ◉, ⏸, —).

---

## 8. Performance Budget

| Metric | Target | Approach |
|--------|--------|----------|
| Initial load (TTI) | < 1.5s | Code-split per page; lazy-load analytics charts |
| Run list render (1000 rows) | < 100ms | Virtualized table; only render ~30 visible rows |
| WebSocket latency | < 200ms | Server-side filtering; send only relevant events |
| Run detail open | < 300ms | Pre-fetch step data on row hover |
| DAG render (50 nodes) | < 500ms | Canvas-based renderer; layout computed in Web Worker |
| Search results | < 200ms | Server-side search with indexed fields |
| Search suggestions | < 80ms | Prefix trie / Meilisearch suggest endpoint |
| Query parse + validate | < 20ms | Client-side AST parser with server fallback |
| Deep payload search | < 500ms | Async-indexed, flattened JSON key paths |
| Log search (7d window) | < 800ms | Time-bounded index partitions |
| Analytics charts | < 1s | Pre-aggregated data; no raw event processing client-side |

---

## 9. Implementation Roadmap

### Phase 1: Core Monitoring (Weeks 1–3)
- Dashboard with stats cards + live ticker
- Runs list with filtering, pagination, streaming
- Run detail with step list view
- Single-engine adapter (your primary engine)

### Phase 2: Actions & Detail Views (Weeks 4–5)
- Retry, cancel, pause, skip actions
- Timeline view for run detail
- Bulk actions
- Saved views

### Phase 3: Full-Text Search (Weeks 6–7)
- Search index integration (Bleve for embedded, Meilisearch for standalone)
- Async ingestion pipeline (run metadata + steps + payloads + logs)
- Global quick-search (Cmd+K palette) with grouped results
- Advanced inline search on Runs page with faceted filters
- Raw query language parser + syntax highlighting + auto-complete
- Visual ↔ raw query bidirectional conversion
- Match context column + result highlighting
- URL serialization of search queries
- Saved views with search queries

### Phase 4: Visualization & Analytics (Weeks 8–9)
- DAG/state-machine graph view
- Analytics page (throughput, latency, failure rate)
- Step heatmap

### Phase 5: Polish & Multi-Engine (Weeks 10–11)
- Second engine adapter
- Dark mode
- Density toggle
- Keyboard navigation
- Alert rules configuration
- Search admin (reindex, stats, retention policies)
