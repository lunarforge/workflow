<script lang="ts">
	import { analyticsClient } from '$lib/api/client';
	import { timestampFromDate } from '@bufbuild/protobuf/wkt';

	let timeRange = $state('24h');
	let loading = $state(true);
	let error = $state('');

	// --- Data state ---
	let throughputData = $state<{ hour: string; succeeded: number; failed: number; running: number }[]>([]);
	let latencyData = $state<{ hour: string; p50: number; p95: number; p99: number }[]>([]);
	let failureData = $state<{ hour: string; rate: number }[]>([]);
	let stepDurations = $state<{
		name: string;
		queue: { p50: number; p95: number; p99: number };
		exec: { p50: number; p95: number; p99: number };
		total: { p50: number; p95: number; p99: number };
		bottleneck: boolean;
		samples: number;
		trend: number[];
	}[]>([]);
	let heatmapSteps = $state<string[]>([]);
	let heatmapBuckets = $state<string[]>([]);
	let heatmapCells = $state<{ step: string; bucket: string; rate: number }[]>([]);

	function timeRangeMs(range: string): number {
		switch (range) {
			case '1h': return 60 * 60 * 1000;
			case '6h': return 6 * 60 * 60 * 1000;
			case '24h': return 24 * 60 * 60 * 1000;
			case '7d': return 7 * 24 * 60 * 60 * 1000;
			default: return 24 * 60 * 60 * 1000;
		}
	}

	function granularity(range: string): string {
		switch (range) {
			case '1h': return '5m';
			case '6h': return '15m';
			case '24h': return '1h';
			case '7d': return '1d';
			default: return '1h';
		}
	}

	function durationToMs(d: { seconds?: bigint; nanos?: number } | undefined): number {
		if (!d) return 0;
		return Number(d.seconds ?? 0n) * 1000 + Math.round((d.nanos ?? 0) / 1_000_000);
	}

	function fmtTimestamp(ts: { seconds?: bigint; nanos?: number } | undefined): string {
		if (!ts) return '';
		const date = new Date(Number(ts.seconds ?? 0n) * 1000);
		return date.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', hour12: false });
	}

	async function fetchData() {
		loading = true;
		error = '';
		try {
			const now = new Date();
			const from = new Date(now.getTime() - timeRangeMs(timeRange));
			const gran = granularity(timeRange);

			const filter = {
				timeRange: {
					from: timestampFromDate(from),
					to: timestampFromDate(now),
				},
				granularity: gran,
			};

			const [throughputRes, latencyRes, failureRes, stepDurRes, heatmapRes] = await Promise.allSettled([
				analyticsClient.getThroughput({ filter }),
				analyticsClient.getLatency({ filter }),
				analyticsClient.getFailureRate({ filter }),
				analyticsClient.getStepDuration({
					timeRange: { from: timestampFromDate(from), to: timestampFromDate(now) },
				}),
				analyticsClient.getStepHeatmap({ filter }),
			]);

			// Map throughput
			if (throughputRes.status === 'fulfilled') {
				throughputData = throughputRes.value.buckets.map((b) => ({
					hour: fmtTimestamp(b.timestamp),
					succeeded: b.succeeded,
					failed: b.failed,
					running: b.running,
				}));
			}

			// Map latency
			if (latencyRes.status === 'fulfilled') {
				latencyData = latencyRes.value.buckets.map((b) => ({
					hour: fmtTimestamp(b.timestamp),
					p50: durationToMs(b.p50),
					p95: durationToMs(b.p95),
					p99: durationToMs(b.p99),
				}));
			}

			// Map failure rate
			if (failureRes.status === 'fulfilled') {
				failureData = failureRes.value.buckets.map((b) => ({
					hour: fmtTimestamp(b.timestamp),
					rate: b.rate,
				}));
			}

			// Map step durations
			if (stepDurRes.status === 'fulfilled') {
				stepDurations = stepDurRes.value.steps.map((s) => ({
					name: s.stepName,
					queue: {
						p50: durationToMs(s.queueWait?.p50),
						p95: durationToMs(s.queueWait?.p95),
						p99: durationToMs(s.queueWait?.p99),
					},
					exec: {
						p50: durationToMs(s.execution?.p50),
						p95: durationToMs(s.execution?.p95),
						p99: durationToMs(s.execution?.p99),
					},
					total: {
						p50: durationToMs(s.total?.p50),
						p95: durationToMs(s.total?.p95),
						p99: durationToMs(s.total?.p99),
					},
					bottleneck: s.isBottleneck,
					samples: Number(s.sampleCount),
					trend: s.trend.map((t) => durationToMs(t.p50)),
				}));
			}

			// Map heatmap
			if (heatmapRes.status === 'fulfilled') {
				heatmapSteps = heatmapRes.value.stepNames;
				heatmapBuckets = heatmapRes.value.bucketTimestamps.map((ts) => fmtTimestamp(ts));
				heatmapCells = heatmapRes.value.cells.map((c) => ({
					step: c.stepName,
					bucket: fmtTimestamp(c.bucketStart),
					rate: c.failureRate,
				}));
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to fetch analytics';
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		// Re-fetch when time range changes.
		timeRange;
		fetchData();
	});

	const maxThroughput = $derived(
		Math.max(1, ...throughputData.map((d) => d.succeeded + d.failed + d.running))
	);

	const maxLatency = $derived(Math.max(1, ...latencyData.map((d) => d.p99)));

	const days = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];
	let scope = $state<'all' | 'single'>('all');
	let sortBy = $state<'p50' | 'p95' | 'p99'>('p95');
	let selectedStep = $state<string | null>(null);

	const sortedSteps = $derived(
		[...stepDurations].sort((a, b) => {
			if (a.bottleneck && !b.bottleneck) return -1;
			if (!a.bottleneck && b.bottleneck) return 1;
			return b.total[sortBy] - a.total[sortBy];
		})
	);

	const maxTotal = $derived(Math.max(1, ...sortedSteps.map((s) => s.total.p50)));

	function pctColor(value: number): string {
		if (value < 500) return 'text-succeeded';
		if (value < 2000) return 'text-paused';
		return 'text-failed';
	}

	function sparklinePoints(data: number[], w = 80, h = 24): string {
		if (data.length === 0) return '';
		const max = Math.max(...data);
		const min = Math.min(...data);
		const range = max - min || 1;
		return data.map((v, i) => `${(i / (data.length - 1)) * w},${h - ((v - min) / range) * (h - 4) - 2}`).join(' ');
	}

	function sparklineColor(data: number[]): string {
		if (data.length < 2) return '#3b82f6';
		return data[data.length - 1] > data[0] * 1.1 ? '#ef4444' : '#3b82f6';
	}

	function toggleStep(name: string) {
		selectedStep = selectedStep === name ? null : name;
	}

	function heatColor(rate: number): string {
		if (rate > 0.15) return 'bg-failed';
		if (rate > 0.05) return 'bg-paused';
		if (rate > 0.01) return 'bg-succeeded/40';
		return 'bg-border/40';
	}

	function fmtMs(ms: number): string {
		if (ms < 1000) return `${ms}ms`;
		return `${(ms / 1000).toFixed(1)}s`;
	}
</script>

<svelte:head>
	<title>FlowWatch - Analytics</title>
</svelte:head>

<div class="flex items-center justify-between mb-4">
	<h2 class="text-lg font-semibold text-text-bright">Analytics</h2>
	<select
		bind:value={timeRange}
		class="text-xs bg-surface border border-border rounded px-2 py-1 text-text-muted"
	>
		<option value="1h">Last 1h</option>
		<option value="6h">Last 6h</option>
		<option value="24h">Last 24h</option>
		<option value="7d">Last 7d</option>
	</select>
</div>

{#if loading}
	<div class="text-xs text-text-muted bg-surface border border-border rounded-lg px-4 py-3 mb-3">
		Loading analytics data...
	</div>
{:else if error}
	<div class="mb-3 text-xs text-failed bg-failed/10 border border-failed/20 rounded-lg px-4 py-2">
		{error}
	</div>
{/if}

{#if throughputData.length === 0 && !loading}
	<div class="mb-3 text-xs text-text-muted bg-surface border border-border rounded-lg px-4 py-2">
		No analytics data available for the selected time range.
	</div>
{/if}

<!-- Throughput chart -->
{#if throughputData.length > 0}
<div class="bg-surface rounded-lg border border-border p-4 mb-4">
	<h3 class="text-[11px] font-bold text-text-muted uppercase tracking-[1px] mb-3">Throughput (runs/interval)</h3>
	<div class="flex items-end gap-px h-32">
		{#each throughputData as d}
			{@const total = d.succeeded + d.failed + d.running}
			{@const h = total > 0 ? (total / maxThroughput) * 100 : 0}
			{@const failH = total > 0 ? (d.failed / total) * h : 0}
			{@const runH = total > 0 ? (d.running / total) * h : 0}
			{@const succH = h - failH - runH}
			<div class="flex-1 flex flex-col justify-end" title="{d.hour}: {d.succeeded} ok, {d.failed} fail, {d.running} running">
				<div class="bg-failed rounded-t-sm" style="height: {failH}%"></div>
				<div class="bg-running" style="height: {runH}%"></div>
				<div class="bg-succeeded rounded-b-sm" style="height: {succH}%"></div>
			</div>
		{/each}
	</div>
	<div class="flex justify-between mt-1 text-[10px] text-text-muted font-mono">
		<span>{throughputData[0]?.hour ?? ''}</span><span>{throughputData[Math.floor(throughputData.length / 2)]?.hour ?? ''}</span><span>{throughputData[throughputData.length - 1]?.hour ?? ''}</span>
	</div>
	<div class="flex gap-4 mt-2 text-[10px] text-text-muted">
		<span class="flex items-center gap-1"><span class="w-2 h-2 bg-succeeded rounded-sm"></span> Succeeded</span>
		<span class="flex items-center gap-1"><span class="w-2 h-2 bg-running rounded-sm"></span> Running</span>
		<span class="flex items-center gap-1"><span class="w-2 h-2 bg-failed rounded-sm"></span> Failed</span>
	</div>
</div>
{/if}

<!-- Latency + Failure Rate row -->
{#if latencyData.length > 0 || failureData.length > 0}
<div class="grid grid-cols-1 lg:grid-cols-2 gap-4 mb-4">
	{#if latencyData.length > 0}
	<div class="bg-surface rounded-lg border border-border p-4">
		<h3 class="text-[11px] font-bold text-text-muted uppercase tracking-[1px] mb-3">Latency Percentiles</h3>
		<div class="relative h-28">
			<svg viewBox="0 0 240 80" class="w-full h-full" preserveAspectRatio="none">
				{#each [
					{ key: 'p99', color: '#f59e0b' },
					{ key: 'p95', color: '#f97316' },
					{ key: 'p50', color: '#3b82f6' },
				] as line}
					<polyline
						fill="none"
						stroke={line.color}
						stroke-width="1.5"
						points={latencyData.map((d, i) => `${(i / Math.max(latencyData.length - 1, 1)) * 240},${80 - ((d as unknown as Record<string, number>)[line.key] / maxLatency) * 75}`).join(' ')}
					/>
				{/each}
			</svg>
		</div>
		<div class="flex gap-4 mt-2 text-[10px] text-text-muted">
			<span class="flex items-center gap-1"><span class="w-3 h-0.5 bg-accent"></span> p50</span>
			<span class="flex items-center gap-1"><span class="w-3 h-0.5 bg-orange-500"></span> p95</span>
			<span class="flex items-center gap-1"><span class="w-3 h-0.5 bg-paused"></span> p99</span>
		</div>
	</div>
	{/if}

	{#if failureData.length > 0}
	<div class="bg-surface rounded-lg border border-border p-4">
		<h3 class="text-[11px] font-bold text-text-muted uppercase tracking-[1px] mb-3">Failure Rate</h3>
		<div class="relative h-28">
			<svg viewBox="0 0 240 80" class="w-full h-full" preserveAspectRatio="none">
				<polyline
					fill="none"
					stroke="#ef4444"
					stroke-width="1.5"
					points={failureData.map((d, i) => `${(i / Math.max(failureData.length - 1, 1)) * 240},${80 - (d.rate / 0.1) * 75}`).join(' ')}
				/>
				<line x1="0" y1={80 - (0.05 / 0.1) * 75} x2="240" y2={80 - (0.05 / 0.1) * 75} stroke="#f59e0b" stroke-width="0.5" stroke-dasharray="4,4" />
			</svg>
		</div>
		<div class="flex gap-4 mt-2 text-[10px] text-text-muted">
			<span class="flex items-center gap-1"><span class="w-3 h-0.5 bg-failed"></span> Failure rate</span>
			<span class="flex items-center gap-1"><span class="w-3 h-0.5 bg-paused"></span> 5% threshold</span>
		</div>
	</div>
	{/if}
</div>
{/if}

<!-- Step Duration & Latency -->
{#if sortedSteps.length > 0}
<div class="bg-surface rounded-lg border border-border p-5 mb-4">
	<!-- Header with scope toggle -->
	<div class="flex justify-between items-center mb-5">
		<h3 class="text-[11px] font-bold text-text-muted uppercase tracking-[1px]">Step Duration &amp; Latency</h3>
		<div class="flex gap-1.5">
			{#each [{ key: 'all', label: 'All Workflows' }, { key: 'single', label: 'Single \u25be' }] as s}
				<button
					onclick={() => { scope = s.key as 'all' | 'single'; }}
					class="px-3 py-1 text-[11px] font-semibold rounded cursor-pointer border transition-colors
						{scope === s.key ? 'bg-accent-muted text-accent border-accent/40' : 'bg-transparent text-text-muted border-border hover:bg-surface-hover'}"
				>{s.label}</button>
			{/each}
		</div>
	</div>

	<!-- Grid header -->
	<div class="grid grid-cols-[140px_1fr_60px_60px_60px_86px] gap-3 py-2 border-b border-border text-[10px] font-bold text-text-muted uppercase tracking-[0.8px]">
		<span>Step</span>
		<span>Queue + Exec (p50)</span>
		{#each ['p50', 'p95', 'p99'] as p}
			<button
				onclick={() => { sortBy = p as 'p50' | 'p95' | 'p99'; }}
				class="text-right cursor-pointer transition-colors {sortBy === p ? 'text-accent underline' : 'text-text-muted hover:text-text'}"
			>{p} {sortBy === p ? '\u2193' : ''}</button>
		{/each}
		<span class="text-center">7d Trend</span>
	</div>

	<!-- Step rows -->
	{#each sortedSteps as step (step.name)}
		{@const queuePct = step.queue.p50 + step.exec.p50 > 0 ? (step.queue.p50 / (step.queue.p50 + step.exec.p50)) * 100 : 0}
		{@const barPct = maxTotal > 0 ? ((step.queue.p50 + step.exec.p50) / maxTotal) * 100 : 0}
		{@const queueWarning = queuePct > 50}
		{@const isSelected = selectedStep === step.name}
		{@const sparkLastY = (() => { const pts = step.trend; if (pts.length === 0) return 12; const mx = Math.max(...pts); const mn = Math.min(...pts); const rng = mx - mn || 1; return 24 - ((pts[pts.length - 1] - mn) / rng) * 20 - 2; })()}
		<div>
			<button
				onclick={() => toggleStep(step.name)}
				class="w-full grid grid-cols-[140px_1fr_60px_60px_60px_86px] gap-3 py-2.5 border-b border-border-subtle items-center cursor-pointer transition-colors text-left
					{isSelected ? 'bg-accent/[0.03]' : 'hover:bg-surface-hover'}"
			>
				<!-- Step name -->
				<div class="flex items-center gap-1.5">
					{#if step.bottleneck}<span class="text-[10px]" title="Bottleneck: p95 > 2x median">&#x1f534;</span>{/if}
					<span class="font-mono text-xs {step.bottleneck ? 'font-semibold' : 'font-normal'} text-text">{step.name}</span>
				</div>

				<!-- Duration bar -->
				<div
					class="relative h-[18px] rounded-[3px] overflow-hidden"
					style="background: rgba(30,42,58,0.3)"
					title="Queue: {step.queue.p50}ms ({Math.round(queuePct)}%) \u00b7 Exec: {step.exec.p50}ms ({Math.round(100 - queuePct)}%)"
				>
					<div class="absolute left-0 top-0 h-full flex rounded-[3px] overflow-hidden" style="width: {barPct}%">
						<div
							class="h-full"
							style="width: {queuePct}%; background: {queueWarning
								? 'repeating-linear-gradient(90deg, rgba(245,158,11,0.38), rgba(245,158,11,0.38) 3px, rgba(245,158,11,0.19) 3px, rgba(245,158,11,0.19) 6px)'
								: 'repeating-linear-gradient(90deg, rgba(59,130,246,0.25), rgba(59,130,246,0.25) 3px, rgba(59,130,246,0.12) 3px, rgba(59,130,246,0.12) 6px)'}"
						></div>
						<div class="flex-1 h-full bg-accent"></div>
					</div>
					{#if queueWarning}
						<span class="absolute right-1 top-0.5 text-[10px] text-paused">\u26a0</span>
					{/if}
				</div>

				<!-- Percentiles -->
				<span class="text-right font-mono text-[11px] font-medium {pctColor(step.total.p50)}">{fmtMs(step.total.p50)}</span>
				<span class="text-right font-mono text-[11px] font-medium {pctColor(step.total.p95)}">{fmtMs(step.total.p95)}</span>
				<span class="text-right font-mono text-[11px] font-medium {pctColor(step.total.p99)}">{fmtMs(step.total.p99)}</span>

				<!-- Sparkline -->
				<div class="flex justify-center">
					{#if step.trend.length > 1}
						<svg width="80" height="24" class="block">
							<polyline points={sparklinePoints(step.trend)} fill="none" stroke={sparklineColor(step.trend)} stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
							<circle cx="80" cy={sparkLastY} r="2.5" fill={sparklineColor(step.trend)} />
						</svg>
					{:else}
						<span class="text-[10px] text-text-muted">—</span>
					{/if}
				</div>
			</button>

			<!-- Accordion detail panel -->
			{#if isSelected}
				<div class="py-4 pl-5 pr-4 border-b border-border bg-accent/[0.02]" style="animation: slideIn 0.15s ease">
					<div class="text-[11px] font-bold text-text-muted uppercase tracking-[0.8px] mb-3">
						{step.name} — 7-Day Trend \u00b7 {step.samples.toLocaleString()} samples
					</div>

					<!-- Percentile breakdown cards -->
					<div class="flex gap-4 mb-4">
						{#each [
							{ label: 'Queue Wait', data: step.queue, color: '#f59e0b', hatched: true },
							{ label: 'Execution', data: step.exec, color: '#3b82f6', hatched: false },
							{ label: 'Total', data: step.total, color: '#e8edf3', hatched: false },
						] as card}
							<div class="bg-bg border border-border rounded-md px-3.5 py-2.5 min-w-[140px]">
								<div class="text-[10px] text-text-muted uppercase tracking-[0.6px] mb-1.5 flex items-center gap-1.5">
									<span
										class="w-2 h-2 rounded-sm inline-block"
										style="background: {card.hatched
											? `repeating-linear-gradient(90deg, ${card.color}60, ${card.color}60 2px, ${card.color}30 2px, ${card.color}30 4px)`
											: card.color}"
									></span>
									{card.label}
								</div>
								<div class="flex gap-3 text-[11px] font-mono">
									<span class="text-text">p50: <span class={pctColor(card.data.p50)}>{fmtMs(card.data.p50)}</span></span>
									<span class="text-text">p95: <span class={pctColor(card.data.p95)}>{fmtMs(card.data.p95)}</span></span>
									<span class="text-text">p99: <span class={pctColor(card.data.p99)}>{fmtMs(card.data.p99)}</span></span>
								</div>
							</div>
						{/each}
					</div>

					<!-- Trend chart -->
					{#if step.trend.length > 1}
					<div class="bg-bg border border-border rounded-md p-4">
						<div class="flex justify-between mb-2">
							<div class="flex gap-4 text-[10px] text-text-muted">
								<span class="flex items-center gap-1"><span class="w-3 h-0.5 bg-accent inline-block"></span> p50</span>
								<span class="flex items-center gap-1"><span class="w-3 h-0.5 bg-paused inline-block" style="border-top: 1px dashed"></span> p95</span>
								<span class="flex items-center gap-1"><span class="w-3 h-0.5 bg-failed/50 inline-block"></span> p99</span>
							</div>
						</div>
						<svg width="100%" height="120" viewBox="0 0 700 120" preserveAspectRatio="none" class="block">
							<!-- Grid lines -->
							{#each [0, 1, 2, 3] as i}
								<line x1="0" y1={i * 40} x2="700" y2={i * 40} stroke="#1e2a3a" stroke-width="0.5" />
							{/each}
							<!-- p99 dashed -->
							<polyline
								points={step.trend.map((v, i) => {
									const p99r = step.total.p99 / Math.max(step.total.p50, 1);
									return `${i * (700 / Math.max(step.trend.length - 1, 1))},${120 - (v * p99r / (step.total.p99 * 1.2)) * 120}`;
								}).join(' ')}
								fill="none" stroke="#ef4444" stroke-width="1" opacity="0.4" stroke-dasharray="4 3"
							/>
							<!-- p95 dashed -->
							<polyline
								points={step.trend.map((v, i) => {
									const p95r = step.total.p95 / Math.max(step.total.p50, 1);
									return `${i * (700 / Math.max(step.trend.length - 1, 1))},${120 - (v * p95r / (step.total.p99 * 1.2)) * 120}`;
								}).join(' ')}
								fill="none" stroke="#f59e0b" stroke-width="1.5" stroke-dasharray="6 3"
							/>
							<!-- p50 solid -->
							<polyline
								points={step.trend.map((v, i) => `${i * (700 / Math.max(step.trend.length - 1, 1))},${120 - (v / (step.total.p99 * 1.2)) * 120}`).join(' ')}
								fill="none" stroke="#3b82f6" stroke-width="2"
							/>
							<!-- p50 dots -->
							{#each step.trend as v, i}
								<circle
									cx={i * (700 / Math.max(step.trend.length - 1, 1))}
									cy={120 - (v / (step.total.p99 * 1.2)) * 120}
									r="3" fill="#3b82f6" stroke="#0a0e14" stroke-width="1.5"
								/>
							{/each}
						</svg>
						<div class="flex justify-between mt-1">
							{#each days as d}
								<span class="text-[10px] text-text-muted">{d}</span>
							{/each}
						</div>
					</div>
					{/if}
				</div>
			{/if}
		</div>
	{/each}

	<!-- Legend -->
	<div class="flex gap-4 mt-3 text-[10px] text-text-muted">
		<span class="flex items-center gap-1">
			<span class="w-3.5 h-2 rounded-sm inline-block" style="background: repeating-linear-gradient(90deg, rgba(59,130,246,0.25), rgba(59,130,246,0.25) 3px, rgba(59,130,246,0.12) 3px, rgba(59,130,246,0.12) 6px)"></span>
			Queue wait
		</span>
		<span class="flex items-center gap-1"><span class="w-3.5 h-2 bg-accent rounded-sm inline-block"></span> Execution</span>
		<span class="flex items-center gap-1">&#x1f534; Bottleneck (p95 &gt; 2x median)</span>
		<span class="flex items-center gap-1">\u26a0 High queue ratio (&gt;50%)</span>
	</div>
</div>
{/if}

<!-- Step Heatmap -->
{#if heatmapSteps.length > 0}
<div class="bg-surface rounded-lg border border-border p-4">
	<h3 class="text-[11px] font-bold text-text-muted uppercase tracking-[1px] mb-3">Step Failure Heatmap</h3>
	<div class="overflow-x-auto">
		<table class="text-[10px]">
			<thead>
				<tr>
					<th class="text-left py-1 px-2 font-bold text-text-muted w-24"></th>
					{#each heatmapBuckets as bucket}
						<th class="py-1 px-1 font-bold text-text-muted text-center">{bucket}</th>
					{/each}
				</tr>
			</thead>
			<tbody>
				{#each heatmapSteps as step}
					<tr>
						<td class="py-1 px-2 font-mono text-text-muted">{step}</td>
						{#each heatmapBuckets as bucket}
							{@const cell = heatmapCells.find((c) => c.step === step && c.bucket === bucket)}
							<td class="py-1 px-1">
								<div
									class="w-8 h-5 rounded-sm {heatColor(cell?.rate ?? 0)}"
									title="{step} @ {bucket}: {((cell?.rate ?? 0) * 100).toFixed(1)}% failure"
								></div>
							</td>
						{/each}
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
	<div class="flex gap-3 mt-3 text-[10px] text-text-muted">
		<span class="flex items-center gap-1"><span class="w-3 h-2 bg-border/40 rounded-sm"></span> &lt;1%</span>
		<span class="flex items-center gap-1"><span class="w-3 h-2 bg-succeeded/40 rounded-sm"></span> 1-5%</span>
		<span class="flex items-center gap-1"><span class="w-3 h-2 bg-paused rounded-sm"></span> 5-15%</span>
		<span class="flex items-center gap-1"><span class="w-3 h-2 bg-failed rounded-sm"></span> &gt;15%</span>
	</div>
</div>
{/if}
