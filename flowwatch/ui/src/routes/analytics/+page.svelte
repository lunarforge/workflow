<script lang="ts">
	let timeRange = $state('24h');

	const throughputData = Array.from({ length: 24 }, (_, i) => ({
		hour: `${String(i).padStart(2, '0')}:00`,
		succeeded: Math.floor(Math.random() * 80 + 20),
		failed: Math.floor(Math.random() * 8),
		running: Math.floor(Math.random() * 15 + 5),
	}));

	const maxThroughput = $derived(
		Math.max(...throughputData.map((d) => d.succeeded + d.failed + d.running))
	);

	const latencyData = Array.from({ length: 24 }, (_, i) => ({
		hour: `${String(i).padStart(2, '0')}:00`,
		p50: Math.floor(Math.random() * 200 + 100),
		p95: Math.floor(Math.random() * 800 + 400),
		p99: Math.floor(Math.random() * 2000 + 1000),
	}));

	const maxLatency = $derived(Math.max(...latencyData.map((d) => d.p99)));

	const failureData = Array.from({ length: 24 }, (_, i) => ({
		hour: `${String(i).padStart(2, '0')}:00`,
		rate: Math.random() * 0.08,
	}));

	const stepDurations = [
		{ name: 'charge-card', p50: 340, p95: 1200, p99: 3800, bottleneck: true, samples: 1283 },
		{ name: 'face-match', p50: 220, p95: 580, p99: 1100, bottleneck: false, samples: 892 },
		{ name: 'validate-input', p50: 45, p95: 90, p99: 180, bottleneck: false, samples: 2104 },
		{ name: 'send-notification', p50: 30, p95: 60, p99: 120, bottleneck: false, samples: 1847 },
		{ name: 'reserve-inventory', p50: 150, p95: 420, p99: 890, bottleneck: false, samples: 1050 },
	];

	const heatmapSteps = ['validate', 'check-balance', 'charge', 'reserve', 'ship', 'notify'];
	const heatmapBuckets = Array.from({ length: 8 }, (_, i) => `${String(i * 3).padStart(2, '0')}:00`);
	const heatmapCells = heatmapSteps.flatMap((step) =>
		heatmapBuckets.map((bucket) => ({
			step,
			bucket,
			rate: step === 'charge' && (bucket === '03:00' || bucket === '06:00')
				? Math.random() * 0.3 + 0.2
				: Math.random() * 0.05,
		}))
	);

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

<div class="mb-3 text-xs text-paused bg-paused-bg border border-paused/20 rounded-lg px-4 py-2">
	Analytics API not yet connected — displaying mock data for layout preview.
</div>

<!-- Throughput chart -->
<div class="bg-surface rounded-lg border border-border p-4 mb-4">
	<h3 class="text-[11px] font-bold text-text-muted uppercase tracking-[1px] mb-3">Throughput (runs/interval)</h3>
	<div class="flex items-end gap-px h-32">
		{#each throughputData as d}
			{@const total = d.succeeded + d.failed + d.running}
			{@const h = (total / maxThroughput) * 100}
			{@const failH = (d.failed / total) * h}
			{@const runH = (d.running / total) * h}
			{@const succH = h - failH - runH}
			<div class="flex-1 flex flex-col justify-end" title="{d.hour}: {d.succeeded} ok, {d.failed} fail, {d.running} running">
				<div class="bg-failed rounded-t-sm" style="height: {failH}%"></div>
				<div class="bg-running" style="height: {runH}%"></div>
				<div class="bg-succeeded rounded-b-sm" style="height: {succH}%"></div>
			</div>
		{/each}
	</div>
	<div class="flex justify-between mt-1 text-[10px] text-text-muted font-mono">
		<span>00:00</span><span>06:00</span><span>12:00</span><span>18:00</span><span>now</span>
	</div>
	<div class="flex gap-4 mt-2 text-[10px] text-text-muted">
		<span class="flex items-center gap-1"><span class="w-2 h-2 bg-succeeded rounded-sm"></span> Succeeded</span>
		<span class="flex items-center gap-1"><span class="w-2 h-2 bg-running rounded-sm"></span> Running</span>
		<span class="flex items-center gap-1"><span class="w-2 h-2 bg-failed rounded-sm"></span> Failed</span>
	</div>
</div>

<!-- Latency + Failure Rate row -->
<div class="grid grid-cols-1 lg:grid-cols-2 gap-4 mb-4">
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
						points={latencyData.map((d, i) => `${(i / 23) * 240},${80 - ((d as unknown as Record<string, number>)[line.key] / maxLatency) * 75}`).join(' ')}
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

	<div class="bg-surface rounded-lg border border-border p-4">
		<h3 class="text-[11px] font-bold text-text-muted uppercase tracking-[1px] mb-3">Failure Rate</h3>
		<div class="relative h-28">
			<svg viewBox="0 0 240 80" class="w-full h-full" preserveAspectRatio="none">
				<polyline
					fill="none"
					stroke="#ef4444"
					stroke-width="1.5"
					points={failureData.map((d, i) => `${(i / 23) * 240},${80 - (d.rate / 0.1) * 75}`).join(' ')}
				/>
				<line x1="0" y1={80 - (0.05 / 0.1) * 75} x2="240" y2={80 - (0.05 / 0.1) * 75} stroke="#f59e0b" stroke-width="0.5" stroke-dasharray="4,4" />
			</svg>
		</div>
		<div class="flex gap-4 mt-2 text-[10px] text-text-muted">
			<span class="flex items-center gap-1"><span class="w-3 h-0.5 bg-failed"></span> Failure rate</span>
			<span class="flex items-center gap-1"><span class="w-3 h-0.5 bg-paused"></span> 5% threshold</span>
		</div>
	</div>
</div>

<!-- Step Duration Table -->
<div class="bg-surface rounded-lg border border-border p-4 mb-4">
	<h3 class="text-[11px] font-bold text-text-muted uppercase tracking-[1px] mb-3">Step Duration &amp; Latency</h3>
	<div class="overflow-x-auto">
		<table class="w-full text-xs">
			<thead>
				<tr class="border-b border-border text-[10px] text-text-muted uppercase tracking-wider">
					<th class="text-left py-2 px-3 font-bold">Step</th>
					<th class="text-left py-2 px-3 font-bold">Queue / Exec</th>
					<th class="text-right py-2 px-3 font-bold">p50</th>
					<th class="text-right py-2 px-3 font-bold">p95</th>
					<th class="text-right py-2 px-3 font-bold">p99</th>
					<th class="text-right py-2 px-3 font-bold">Samples</th>
				</tr>
			</thead>
			<tbody>
				{#each stepDurations as step}
					<tr class="border-b border-border-subtle hover:bg-surface-hover">
						<td class="py-2 px-3 font-mono text-text">
							{#if step.bottleneck}<span class="text-failed mr-1" title="Bottleneck">*</span>{/if}
							{step.name}
						</td>
						<td class="py-2 px-3">
							<div class="flex items-center gap-0.5 h-3">
								<div class="bg-text-muted/40 rounded-sm h-full" style="width: {Math.max((step.p50 * 0.15 / step.p95) * 80, 4)}px" title="Queue wait"></div>
								<div class="bg-accent rounded-sm h-full" style="width: {Math.max((step.p50 * 0.85 / step.p95) * 80, 4)}px" title="Execution"></div>
							</div>
						</td>
						<td class="py-2 px-3 text-right font-mono text-succeeded">{fmtMs(step.p50)}</td>
						<td class="py-2 px-3 text-right font-mono {step.p95 > 2000 ? 'text-failed' : step.p95 > 500 ? 'text-paused' : 'text-succeeded'}">{fmtMs(step.p95)}</td>
						<td class="py-2 px-3 text-right font-mono {step.p99 > 2000 ? 'text-failed' : step.p99 > 500 ? 'text-paused' : 'text-succeeded'}">{fmtMs(step.p99)}</td>
						<td class="py-2 px-3 text-right text-text-muted">{step.samples.toLocaleString()}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
	<div class="flex gap-4 mt-3 text-[10px] text-text-muted">
		<span class="flex items-center gap-1"><span class="w-2 h-2 bg-text-muted/40 rounded-sm"></span> Queue wait</span>
		<span class="flex items-center gap-1"><span class="w-2 h-2 bg-accent rounded-sm"></span> Execution</span>
		<span class="flex items-center gap-1"><span class="text-failed">*</span> Bottleneck (p95 &gt; 2x median)</span>
	</div>
</div>

<!-- Step Heatmap -->
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
