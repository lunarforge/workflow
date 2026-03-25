# FlowWatch UI

Engine-agnostic workflow monitoring dashboard built with SvelteKit, Svelte 5, and Tailwind CSS v4.

## Design

Dark NOC (Network Operations Center) dashboard aesthetic inspired by mission control interfaces. Key design elements:

- **Color system:** Dark background (`#0a0e14`) with semantic status colors — blue=running, green=succeeded, red=failed, amber=paused, purple=canceled
- **Typography:** JetBrains Mono for data/IDs/timestamps, Inter for UI labels
- **Status badges:** Unicode icons (`◉` running, `✓` succeeded, `✗` failed, `⏸` paused, `○` pending, `⊘` canceled)
- **Animations:** Pulsing dots for live feeds, slide-in for new data, bar-pulse for running timeline bars
- **Stat cards:** Bottom border accent color with hover lift effect
- **Density toggle:** Compact/comfortable modes for different use cases

## Pages

- **Dashboard** — Summary stat cards, live run ticker, alert feed, subsystem health
- **Subsystems** — Health cards with running/failed/error-rate stats
- **Workflows** — Catalog of registered workflow definitions
- **Runs** — Filterable run list with bulk actions, slide-over detail panel (55% width, Esc/arrow key navigation)
- **Run Detail** — Steps table, timeline bars (Gantt-style), and DAG graph view
- **Traces** — Cross-subsystem waterfall visualization
- **Analytics** — Throughput, latency percentiles, failure rate, step duration table, failure heatmap (mock data)
- **Settings** — Placeholder for subsystem management, alert rules, saved views, engine connection

## Tech Stack

- **SvelteKit** with Svelte 5 runes (`$state`, `$derived`, `$props`, `$effect`)
- **Tailwind CSS v4** with custom theme tokens in `app.css`
- **@connectrpc/connect-web** for browser Connect transport to gRPC API
- **Svelte stores** for shared state (`src/lib/stores/`)
- **Console logging** with `[API]`, `[API Response]`, `[API Error]` prefixes

## Development

```sh
bun install
bun run dev
```

## Building

```sh
bun run build
bun run preview
```
