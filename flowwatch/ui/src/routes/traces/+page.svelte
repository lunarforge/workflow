<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { workflowClient } from '$lib/api/client';
	import StatusBadge from '$lib/components/StatusBadge.svelte';

	const traceId = $derived(page.url.searchParams.get('id') ?? '');

	let trace = $state<{
		traceId: string;
		totalDurationMs: number;
		subsystemCount: number;
		spans: {
			runId: string;
			workflowId: string;
			subsystemId: string;
			status: string;
			spanId: string;
			parentSpanId: string;
			startedAt: string;
			duration: string;
			offsetMs: number;
			durationMs: number;
		}[];
	} | null>(null);
	let loading = $state(false);
	let error = $state('');
	let searchInput = $state('');

	const subsystemColorMap: Record<string, string> = {};
	const colorPalette = ['#3b82f6', '#a855f7', '#f59e0b', '#10b981', '#ec4899'];

	function getSubsystemColor(id: string): string {
		if (!subsystemColorMap[id]) {
			subsystemColorMap[id] = colorPalette[Object.keys(subsystemColorMap).length % colorPalette.length];
		}
		return subsystemColorMap[id];
	}

	onMount(() => {
		if (traceId) {
			searchInput = traceId;
			loadTrace(traceId);
		}
	});

	function statusFromNumber(status: number): string {
		const map: Record<number, string> = {
			0: 'unknown', 1: 'pending', 2: 'running', 3: 'succeeded',
			4: 'failed', 5: 'canceled', 6: 'paused',
		};
		return map[status] ?? 'unknown';
	}

	function durationToMs(d: { seconds?: bigint; nanos?: number } | undefined): number {
		if (!d) return 0;
		return Number(d.seconds ?? 0n) * 1000 + Math.floor((d.nanos ?? 0) / 1e6);
	}

	async function loadTrace(id: string) {
		if (!id.trim()) return;
		loading = true;
		error = '';
		trace = null;
		try {
			console.log('[API]', 'GetTrace', { traceId: id.trim() });
			const res = await workflowClient.getTrace({ traceId: id.trim() });
			const totalMs = durationToMs(res.totalDuration);
			trace = {
				traceId: res.traceId,
				totalDurationMs: totalMs,
				subsystemCount: res.subsystemCount,
				spans: (res.spans ?? []).map((s) => ({
					runId: s.run?.id ?? '',
					workflowId: s.run?.workflowId ?? '',
					subsystemId: s.subsystemId,
					status: statusFromNumber(s.run?.status ?? 0),
					spanId: s.spanId,
					parentSpanId: s.parentSpanId,
					startedAt: s.run?.startedAt ? new Date(Number(s.run.startedAt.seconds) * 1000).toLocaleString() : '',
					duration: s.run?.duration ? `${Number(s.run.duration.seconds ?? 0n)}s` : '-',
					offsetMs: durationToMs(s.offset),
					durationMs: durationToMs(s.duration),
				})),
			};
			console.log('[API Response]', 'GetTrace', { spans: trace.spans.length });
		} catch (e) {
			console.error('[API Error]', 'GetTrace', e);
			error = e instanceof Error ? e.message : 'Failed to load trace';
		} finally {
			loading = false;
		}
	}

	function handleSearch() {
		loadTrace(searchInput);
	}

	const subsystemIds = $derived(
		trace ? [...new Set(trace.spans.map((s) => s.subsystemId || 'default'))] : []
	);

	function fmtMs(ms: number): string {
		if (ms < 1000) return `${ms}ms`;
		return `${(ms / 1000).toFixed(1)}s`;
	}
</script>

<svelte:head>
	<title>FlowWatch - Traces</title>
</svelte:head>

<h2 class="text-lg font-semibold text-text-bright mb-4">Trace Waterfall</h2>

<!-- Search bar -->
<div class="mb-6">
	<form onsubmit={(e) => { e.preventDefault(); handleSearch(); }} class="flex gap-2">
		<input
			bind:value={searchInput}
			placeholder="Enter trace ID..."
			class="flex-1 px-4 py-2 text-sm font-mono bg-surface border border-border rounded-lg text-text placeholder-text-muted focus:outline-none focus:ring-1 focus:ring-accent focus:border-accent"
		/>
		<button
			type="submit"
			class="px-4 py-2 text-xs font-semibold bg-accent text-white rounded-lg hover:brightness-110 transition-all"
		>
			Lookup
		</button>
	</form>
</div>

{#if error}
	<div class="bg-failed-bg border border-failed/20 text-failed p-4 rounded-lg mb-4">{error}</div>
{/if}

{#if loading}
	<p class="text-center text-text-muted py-8">Loading trace...</p>
{:else if !trace && !error}
	<div class="bg-surface rounded-lg border border-border p-8 text-center">
		<p class="text-text-muted text-sm">Enter a trace ID to view the cross-subsystem waterfall.</p>
		<p class="text-text-muted text-xs mt-2">Traces correlate workflow runs that were triggered together across subsystems.</p>
	</div>
{:else if trace}
	<div class="text-xs text-text-muted mb-4 flex items-center gap-3">
		<span>Trace: <span class="font-mono text-accent">{trace.traceId}</span></span>
		<span>{trace.spans.length} run{trace.spans.length !== 1 ? 's' : ''}</span>
		<span>{trace.subsystemCount} subsystem{trace.subsystemCount !== 1 ? 's' : ''}</span>
		<span>Duration: <span class="font-mono text-text">{fmtMs(trace.totalDurationMs)}</span></span>
	</div>

	<!-- Gantt Waterfall -->
	<div class="bg-surface rounded-lg border border-border overflow-hidden">
		{#each subsystemIds as ssId}
			{@const ssSpans = trace.spans.filter((s) => (s.subsystemId || 'default') === ssId)}
			{@const barColor = getSubsystemColor(ssId)}
			<div class="border-b border-border last:border-b-0">
				<div class="px-4 py-2 bg-bg/50 flex items-center gap-2">
					<span class="w-2.5 h-2.5 rounded-sm" style="background: {barColor};"></span>
					<span class="text-xs font-semibold text-text-bright">{ssId}</span>
					<span class="text-xs text-text-muted">({ssSpans.length} run{ssSpans.length !== 1 ? 's' : ''})</span>
				</div>
				{#each ssSpans as span}
					{@const left = trace.totalDurationMs > 0 ? (span.offsetMs / trace.totalDurationMs) * 100 : 0}
					{@const width = trace.totalDurationMs > 0 ? Math.max((span.durationMs / trace.totalDurationMs) * 100, 1) : 100}
					<div class="grid grid-cols-[180px_1fr] items-center px-4 py-2.5 border-b border-border-subtle last:border-b-0 hover:bg-surface-hover">
						<div class="flex items-center gap-1.5 {span.parentSpanId ? 'pl-5' : ''}">
							{#if span.parentSpanId}
								<span class="text-border text-[10px]">|--</span>
							{/if}
							<a href="/runs/{span.runId}" class="text-xs font-mono text-accent hover:underline truncate">
								{span.workflowId || span.runId.slice(0, 8)}
							</a>
							<StatusBadge status={span.status} />
						</div>
						<!-- Gantt bar -->
						<div class="flex items-center gap-2">
							<div class="flex-1 h-5 rounded relative" style="background: rgba(30, 42, 58, 0.4);">
								<div
									class="absolute h-full rounded {span.status === 'running' ? 'animate-bar-pulse' : ''}"
									style="left: {left}%; width: {width}%;
										background: {span.status === 'failed'
											? `repeating-linear-gradient(90deg, #ef4444, #ef4444 4px, #ef444480 4px, #ef444480 8px)`
											: barColor};
										opacity: 0.85;"
									title="{span.workflowId}: {fmtMs(span.durationMs)} (offset: {fmtMs(span.offsetMs)})"
								></div>
							</div>
							<span class="text-[10px] text-text-muted font-mono w-14 text-right shrink-0">{fmtMs(span.durationMs)}</span>
						</div>
					</div>
				{/each}
			</div>
		{/each}

		<!-- Time axis -->
		{#if trace.totalDurationMs > 0}
			<div class="px-4 py-1.5 bg-bg/50 flex justify-between text-[10px] text-text-muted font-mono" style="padding-left: 196px">
				<span>0ms</span>
				<span>{fmtMs(trace.totalDurationMs / 4)}</span>
				<span>{fmtMs(trace.totalDurationMs / 2)}</span>
				<span>{fmtMs(trace.totalDurationMs * 3 / 4)}</span>
				<span>{fmtMs(trace.totalDurationMs)}</span>
			</div>
		{/if}
	</div>

	<!-- Legend -->
	<div class="flex gap-4 mt-3 text-[10px] text-text-muted">
		{#each subsystemIds as ssId}
			<span class="flex items-center gap-1">
				<span class="w-2.5 h-2 rounded-sm" style="background: {getSubsystemColor(ssId)};"></span> {ssId}
			</span>
		{/each}
		<span class="flex items-center gap-1"><span class="w-2.5 h-2 rounded-sm bg-failed"></span> failed</span>
	</div>
{/if}
