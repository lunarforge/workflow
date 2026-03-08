<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { runClient, workflowClient } from '$lib/api/client';
	import StatusBadge from '$lib/components/StatusBadge.svelte';

	const runId = $derived(page.params.id ?? '');

	let run = $state<{
		id: string; workflowId: string; foreignId: string; status: string;
		currentStep: string; startedAt: string; finishedAt: string;
		duration: string; errorMessage: string; metadata: Record<string, string>;
	} | null>(null);

	let steps = $state<{
		id: string; stepName: string; status: string; attempt: number;
		startedAt: string; finishedAt: string; duration: string; errorMessage: string;
		input: string; output: string;
	}[]>([]);

	let timeline = $state<{
		stepName: string; status: string; attempt: number;
		offsetMs: number; durationMs: number; errorMessage: string;
	}[]>([]);
	let totalDurationMs = $state(0);

	let graph = $state<{
		nodes: { id: string; label: string; type: number }[];
		edges: { sourceId: string; targetId: string; label: string }[];
	} | null>(null);

	let loading = $state(true);
	let error = $state('');
	let activeTab = $state<'steps' | 'timeline' | 'graph'>('steps');
	let expandedStep = $state<string | null>(null);

	onMount(() => {
		loadRun();
		loadSteps();
		loadTimeline();
	});

	function statusFromNumber(status: number): string {
		const map: Record<number, string> = {
			0: 'unknown', 1: 'pending', 2: 'running', 3: 'succeeded',
			4: 'failed', 5: 'canceled', 6: 'paused',
		};
		return map[status] ?? 'unknown';
	}

	function stepStatusFromNumber(status: number): string {
		const map: Record<number, string> = {
			0: 'unknown', 1: 'pending', 2: 'running', 3: 'succeeded',
			4: 'failed', 5: 'skipped', 6: 'canceled',
		};
		return map[status] ?? 'unknown';
	}

	function formatDuration(d: { seconds?: bigint; nanos?: number } | undefined): string {
		if (!d) return '-';
		const ms = Number(d.seconds ?? 0n) * 1000 + Math.floor((d.nanos ?? 0) / 1e6);
		if (ms < 1000) return `${ms}ms`;
		if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
		return `${(ms / 60000).toFixed(1)}m`;
	}

	function durationToMs(d: { seconds?: bigint; nanos?: number } | undefined): number {
		if (!d) return 0;
		return Number(d.seconds ?? 0n) * 1000 + Math.floor((d.nanos ?? 0) / 1e6);
	}

	function formatTimestamp(ts: { seconds?: bigint; nanos?: number } | undefined): string {
		if (!ts) return '-';
		return new Date(Number(ts.seconds ?? 0n) * 1000).toLocaleString();
	}

	async function loadRun() {
		try {
			console.log('[API]', 'GetRun', { runId });
			const res = await runClient.getRun({ runId });
			console.log('[API Response]', 'GetRun', { runId });
			if (res.run) {
				run = {
					id: res.run.id,
					workflowId: res.run.workflowId,
					foreignId: res.run.foreignId,
					status: statusFromNumber(res.run.status),
					currentStep: res.run.currentStep,
					startedAt: formatTimestamp(res.run.startedAt),
					finishedAt: formatTimestamp(res.run.finishedAt),
					duration: formatDuration(res.run.duration),
					errorMessage: res.run.error?.message ?? '',
					metadata: res.run.metadata ?? {},
				};
			}
		} catch (e) {
			console.error('[API Error]', 'GetRun', e);
			error = e instanceof Error ? e.message : 'Failed to load run';
		} finally {
			loading = false;
		}
	}

	async function loadSteps() {
		try {
			console.log('[API]', 'GetRunSteps', { runId });
			const res = await runClient.getRunSteps({ runId });
			console.log('[API Response]', 'GetRunSteps', { count: res.steps?.length ?? 0 });
			steps = (res.steps ?? []).map((s) => ({
				id: s.id,
				stepName: s.stepName,
				status: stepStatusFromNumber(s.status),
				attempt: s.attempt,
				startedAt: formatTimestamp(s.startedAt),
				finishedAt: formatTimestamp(s.finishedAt),
				duration: formatDuration(s.duration),
				errorMessage: s.error?.message ?? '',
				input: typeof s.input === 'string' ? s.input : JSON.stringify(s.input ?? '', null, 2),
				output: typeof s.output === 'string' ? s.output : JSON.stringify(s.output ?? '', null, 2),
			}));
		} catch (e) {
			console.error('[API Error]', 'GetRunSteps', e);
		}
	}

	async function loadTimeline() {
		try {
			console.log('[API]', 'GetRunTimeline', { runId });
			const res = await runClient.getRunTimeline({ runId });
			console.log('[API Response]', 'GetRunTimeline', { entries: res.entries?.length ?? 0 });
			totalDurationMs = durationToMs(res.totalDuration);
			timeline = (res.entries ?? []).map((e) => ({
				stepName: e.stepName,
				status: stepStatusFromNumber(e.status),
				attempt: e.attempt,
				offsetMs: durationToMs(e.offset),
				durationMs: durationToMs(e.duration),
				errorMessage: e.error?.message ?? '',
			}));
		} catch (e) {
			console.error('[API Error]', 'GetRunTimeline', e);
		}
	}

	async function loadGraph() {
		if (graph || !run) return;
		try {
			console.log('[API]', 'GetGraph', { workflowId: run.workflowId });
			const res = await workflowClient.getGraph({ workflowId: run.workflowId });
			console.log('[API Response]', 'GetGraph', { nodes: res.graph?.nodes?.length ?? 0 });
			if (res.graph) {
				graph = {
					nodes: res.graph.nodes.map((n) => ({ id: n.id, label: n.label, type: n.type })),
					edges: res.graph.edges.map((e) => ({ sourceId: e.sourceId, targetId: e.targetId, label: e.label })),
				};
			}
		} catch (e) {
			console.error('[API Error]', 'GetGraph', e);
		}
	}

	async function handleAction(action: 'cancel' | 'pause' | 'resume' | 'retry') {
		try {
			console.log('[API]', `RunAction:${action}`, { runId });
			if (action === 'cancel') await runClient.cancelRun({ runId, reason: '' });
			else if (action === 'pause') await runClient.pauseRun({ runId });
			else if (action === 'resume') await runClient.resumeRun({ runId });
			else if (action === 'retry') await runClient.retryRun({ runId });
			console.log('[API Response]', `RunAction:${action}`, 'success');
			await loadRun();
			await loadSteps();
		} catch (e) {
			console.error('[API Error]', `RunAction:${action}`, e);
			error = e instanceof Error ? e.message : `Failed to ${action} run`;
		}
	}

	function selectTab(tab: 'steps' | 'timeline' | 'graph') {
		activeTab = tab;
		if (tab === 'graph') loadGraph();
	}

	function layoutGraph(nodes: { id: string; label: string }[], edges: { sourceId: string; targetId: string }[]) {
		const levels = new Map<string, number>();
		const incoming = new Map<string, Set<string>>();
		for (const n of nodes) incoming.set(n.id, new Set());
		for (const e of edges) incoming.get(e.targetId)?.add(e.sourceId);
		const roots = nodes.filter((n) => (incoming.get(n.id)?.size ?? 0) === 0);
		const queue = roots.map((n) => ({ id: n.id, level: 0 }));
		while (queue.length > 0) {
			const { id, level } = queue.shift()!;
			if (levels.has(id) && (levels.get(id)! >= level)) continue;
			levels.set(id, level);
			for (const e of edges) {
				if (e.sourceId === id) queue.push({ id: e.targetId, level: level + 1 });
			}
		}
		const maxLevel = Math.max(...levels.values(), 0);
		const levelCounts = new Map<number, number>();
		const levelIdx = new Map<string, number>();
		for (const n of nodes) {
			const l = levels.get(n.id) ?? 0;
			const idx = levelCounts.get(l) ?? 0;
			levelIdx.set(n.id, idx);
			levelCounts.set(l, idx + 1);
		}
		return nodes.map((n) => {
			const l = levels.get(n.id) ?? 0;
			const idx = levelIdx.get(n.id) ?? 0;
			const count = levelCounts.get(l) ?? 1;
			return {
				...n,
				x: 60 + (l / Math.max(maxLevel, 1)) * 400,
				y: 40 + (idx - (count - 1) / 2) * 60 + 100,
			};
		});
	}

	const completedStepNames = $derived(new Set(steps.filter((s) => s.status === 'succeeded').map((s) => s.stepName)));
	const failedStepNames = $derived(new Set(steps.filter((s) => s.status === 'failed').map((s) => s.stepName)));
	const runningStepNames = $derived(new Set(steps.filter((s) => s.status === 'running').map((s) => s.stepName)));
</script>

<svelte:head>
	<title>FlowWatch - Run {runId.slice(0, 8)}...</title>
</svelte:head>

{#if loading}
	<p class="text-center text-text-muted py-8">Loading run...</p>
{:else if error}
	<div class="bg-failed-bg border border-failed/20 text-failed p-4 rounded-lg mb-4">{error}</div>
{:else if run}
	<div class="flex items-center justify-between mb-4">
		<div>
			<a href="/runs" class="text-sm text-text-muted hover:text-accent">← Runs</a>
			<h2 class="text-lg font-semibold text-text-bright mt-1 flex items-center gap-3">
				<span class="font-mono">{run.id.slice(0, 8)}...</span>
				<StatusBadge status={run.status} size="md" />
			</h2>
			<p class="text-sm text-text-muted mt-1">
				<a href="/runs?workflow={run.workflowId}" class="hover:text-accent">{run.workflowId}</a>
				{#if run.foreignId}
					<span class="mx-1 text-border">·</span>
					<span class="font-mono">{run.foreignId}</span>
				{/if}
			</p>
		</div>
		<div class="flex items-center gap-2">
			{#if run.status === 'running'}
				<button onclick={() => handleAction('pause')} class="px-3.5 py-1.5 text-xs font-semibold rounded bg-transparent text-paused border border-border hover:bg-surface-hover transition-all">Pause</button>
				<button onclick={() => handleAction('cancel')} class="px-3.5 py-1.5 text-xs font-semibold rounded bg-transparent text-failed border border-border hover:bg-surface-hover transition-all">Cancel</button>
			{/if}
			{#if run.status === 'paused'}
				<button onclick={() => handleAction('resume')} class="px-3.5 py-1.5 text-xs font-semibold rounded bg-accent text-white border border-accent hover:brightness-110 transition-all">Resume</button>
			{/if}
			{#if run.status === 'failed'}
				<button onclick={() => handleAction('retry')} class="px-3.5 py-1.5 text-xs font-semibold rounded bg-accent text-white border border-accent hover:brightness-110 transition-all">Retry</button>
			{/if}
		</div>
	</div>

	<!-- Run metadata -->
	<div class="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
		<div class="bg-surface rounded-lg border border-border p-4 border-b-3 border-b-accent">
			<p class="text-[10px] text-text-muted uppercase tracking-wider mb-1">Current Step</p>
			<p class="text-text-bright font-medium font-mono">{run.currentStep || '-'}</p>
		</div>
		<div class="bg-surface rounded-lg border border-border p-4 border-b-3 border-b-accent">
			<p class="text-[10px] text-text-muted uppercase tracking-wider mb-1">Duration</p>
			<p class="text-text-bright font-medium font-mono">{run.duration}</p>
		</div>
		<div class="bg-surface rounded-lg border border-border p-4 border-b-3 border-b-accent">
			<p class="text-[10px] text-text-muted uppercase tracking-wider mb-1">Started</p>
			<p class="text-text text-sm">{run.startedAt}</p>
		</div>
		<div class="bg-surface rounded-lg border border-border p-4 border-b-3 border-b-accent">
			<p class="text-[10px] text-text-muted uppercase tracking-wider mb-1">Finished</p>
			<p class="text-text text-sm">{run.finishedAt}</p>
		</div>
	</div>

	{#if run.errorMessage}
		<div class="bg-failed-bg border border-failed/20 rounded-lg p-4 mb-6">
			<p class="text-xs font-semibold text-failed mb-1">Error</p>
			<p class="text-sm text-failed font-mono">{run.errorMessage}</p>
		</div>
	{/if}

	<!-- Tabs -->
	<div class="flex border-b border-border mb-4">
		{#each [
			{ key: 'steps', label: `Steps (${steps.length})` },
			{ key: 'timeline', label: 'Timeline' },
			{ key: 'graph', label: 'Graph' },
		] as tab}
			<button
				onclick={() => selectTab(tab.key as typeof activeTab)}
				class="px-5 py-3 text-xs font-semibold uppercase tracking-wider border-b-2 transition-colors
					{activeTab === tab.key ? 'text-accent border-accent' : 'text-text-muted border-transparent hover:text-text'}"
			>{tab.label}</button>
		{/each}
	</div>

	{#if activeTab === 'steps'}
		<div class="bg-surface rounded-lg border border-border overflow-hidden">
			<table class="w-full text-sm">
				<thead>
					<tr class="border-b border-border text-[10px] text-text-muted uppercase tracking-wider">
						<th class="text-left px-4 py-2 font-bold">Step</th>
						<th class="text-left px-4 py-2 font-bold">Status</th>
						<th class="text-left px-4 py-2 font-bold">Attempt</th>
						<th class="text-left px-4 py-2 font-bold">Duration</th>
						<th class="text-left px-4 py-2 font-bold">Started</th>
						<th class="text-left px-4 py-2 font-bold">Error</th>
					</tr>
				</thead>
				<tbody>
					{#each steps as step}
						<tr
							class="border-b border-border-subtle hover:bg-surface-hover cursor-pointer"
							onclick={() => expandedStep = expandedStep === step.id ? null : step.id}
						>
							<td class="px-4 py-2 font-mono text-text font-medium">{step.stepName}</td>
							<td class="px-4 py-2"><StatusBadge status={step.status} /></td>
							<td class="px-4 py-2 text-text-muted">{step.attempt}</td>
							<td class="px-4 py-2 font-mono text-xs text-text-muted">{step.duration}</td>
							<td class="px-4 py-2 text-xs text-text-muted">{step.startedAt}</td>
							<td class="px-4 py-2 text-xs text-failed max-w-xs truncate">{step.errorMessage}</td>
						</tr>
						{#if expandedStep === step.id}
							<tr class="bg-bg">
								<td colspan="6" class="px-4 py-3">
									<div class="grid grid-cols-2 gap-4 text-xs">
										<div>
											<p class="font-semibold text-text-muted mb-1">Input</p>
											<pre class="bg-surface border border-border rounded p-2 font-mono text-[11px] max-h-32 overflow-auto whitespace-pre-wrap text-text">{step.input || '(none)'}</pre>
										</div>
										<div>
											<p class="font-semibold text-text-muted mb-1">Output</p>
											<pre class="bg-surface border border-border rounded p-2 font-mono text-[11px] max-h-32 overflow-auto whitespace-pre-wrap text-text">{step.output || '(none)'}</pre>
										</div>
									</div>
									{#if step.errorMessage}
										<div class="mt-3">
											<p class="font-semibold text-text-muted mb-1 text-xs">Error Detail</p>
											<pre class="bg-failed-bg border border-failed/20 rounded p-2 font-mono text-[11px] text-failed whitespace-pre-wrap">{step.errorMessage}</pre>
										</div>
									{/if}
									<div class="mt-2 flex gap-2">
										<span class="text-[10px] text-text-muted">Started: {step.startedAt}</span>
										<span class="text-[10px] text-text-muted">Finished: {step.finishedAt}</span>
									</div>
								</td>
							</tr>
						{/if}
					{:else}
						<tr>
							<td colspan="6" class="px-4 py-8 text-center text-text-muted">No steps recorded</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}

	{#if activeTab === 'timeline'}
		<div class="bg-surface rounded-lg border border-border p-4">
			{#if timeline.length === 0}
				<p class="text-center text-text-muted py-8">No timeline data available</p>
			{:else}
				<div class="space-y-2">
					{#each timeline as entry}
						{@const left = totalDurationMs > 0 ? (entry.offsetMs / totalDurationMs) * 100 : 0}
						{@const width = totalDurationMs > 0 ? Math.max((entry.durationMs / totalDurationMs) * 100, 1) : 100}
						{@const isFailed = entry.status === 'failed'}
						{@const isRunning = entry.status === 'running'}
						<div class="flex items-center gap-3">
							<div class="w-32 text-xs truncate shrink-0">
								<span class="font-mono text-text font-medium">{entry.stepName}</span>
								{#if entry.attempt > 1}
									<span class="text-xs text-text-muted ml-1">#{entry.attempt}</span>
								{/if}
							</div>
							<div class="flex-1 h-[22px] rounded relative" style="background: rgba(30, 42, 58, 0.4);">
								<div
									class="absolute h-full rounded {isRunning ? 'animate-bar-pulse' : ''}"
									style="left: {left}%; width: {width}%;
										background: {isFailed
											? `repeating-linear-gradient(90deg, #ef4444, #ef4444 4px, #ef444480 4px, #ef444480 8px)`
											: isRunning
											? `linear-gradient(90deg, #3b82f6, #3b82f680)`
											: entry.status === 'succeeded' ? '#10b981' : '#5c6b7e'};"
									title="{entry.stepName}: {entry.durationMs}ms"
								></div>
							</div>
							<div class="w-20 text-[11px] text-text-muted font-mono text-right shrink-0">
								{entry.durationMs < 1000 ? `${entry.durationMs}ms` : `${(entry.durationMs / 1000).toFixed(1)}s`}
							</div>
						</div>
					{/each}
				</div>
				<p class="text-xs text-text-muted mt-3 text-right font-mono">Total: {formatDuration({ seconds: BigInt(Math.floor(totalDurationMs / 1000)), nanos: (totalDurationMs % 1000) * 1e6 })}</p>
			{/if}
		</div>
	{/if}

	{#if activeTab === 'graph'}
		<div class="bg-surface rounded-lg border border-border p-4">
			{#if !graph}
				<p class="text-center text-text-muted py-8">Loading graph...</p>
			{:else if graph.nodes.length === 0}
				<p class="text-center text-text-muted py-8">No graph data available</p>
			{:else}
				{@const positioned = layoutGraph(graph.nodes, graph.edges)}
				<svg viewBox="0 0 520 {Math.max(positioned.length * 50, 200) + 40}" class="w-full" style="max-height: 500px">
					<defs>
						<marker id="arrowhead" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
							<polygon points="0 0, 8 3, 0 6" fill="#5c6b7e" />
						</marker>
					</defs>
					{#each graph.edges as edge}
						{@const source = positioned.find((n) => n.id === edge.sourceId)}
						{@const target = positioned.find((n) => n.id === edge.targetId)}
						{#if source && target}
							<line
								x1={source.x + 50} y1={source.y}
								x2={target.x - 50} y2={target.y}
								stroke="#5c6b7e"
								stroke-width="1.5"
								marker-end="url(#arrowhead)"
							/>
							{#if edge.label}
								<text
									x={(source.x + target.x) / 2}
									y={(source.y + target.y) / 2 - 6}
									text-anchor="middle"
									fill="#5c6b7e"
									class="text-[9px]"
								>{edge.label}</text>
							{/if}
						{/if}
					{/each}
					{#each positioned as node}
						{@const isCompleted = completedStepNames.has(node.label) || completedStepNames.has(node.id)}
						{@const isFailed = failedStepNames.has(node.label) || failedStepNames.has(node.id)}
						{@const isRunning = runningStepNames.has(node.label) || runningStepNames.has(node.id)}
						{@const nodeColor = isFailed ? '#ef4444' : isRunning ? '#3b82f6' : isCompleted ? '#10b981' : '#5c6b7e'}
						{@const nodeBg = isFailed ? '#1f0a0a' : isRunning ? '#0c1a2e' : isCompleted ? '#0a1f18' : '#111820'}
						<g>
							<rect
								x={node.x - 48} y={node.y - 16}
								width="96" height="32" rx="6"
								fill={nodeBg}
								stroke={nodeColor}
								stroke-width="2"
							/>
							{#if isRunning}
								<rect
									x={node.x - 48} y={node.y - 16}
									width="96" height="32" rx="6"
									fill="none" stroke="#3b82f6" stroke-width="2"
								>
									<animate attributeName="opacity" values="0.5;1;0.5" dur="1.5s" repeatCount="indefinite" />
								</rect>
							{/if}
							<text
								x={node.x} y={node.y + 4}
								text-anchor="middle"
								fill={nodeColor}
								class="text-[10px] font-medium"
								style="font-family: 'JetBrains Mono', monospace;"
							>{node.label || node.id}</text>
						</g>
					{/each}
				</svg>
				<div class="flex gap-4 mt-3 text-[10px] text-text-muted">
					<span class="flex items-center gap-1"><span class="w-3 h-2 rounded-sm" style="background: #0a1f18; border: 1px solid #10b981;"></span> Completed</span>
					<span class="flex items-center gap-1"><span class="w-3 h-2 rounded-sm" style="background: #0c1a2e; border: 1px solid #3b82f6;"></span> Running</span>
					<span class="flex items-center gap-1"><span class="w-3 h-2 rounded-sm" style="background: #1f0a0a; border: 1px solid #ef4444;"></span> Failed</span>
					<span class="flex items-center gap-1"><span class="w-3 h-2 rounded-sm" style="background: #111820; border: 1px solid #5c6b7e;"></span> Pending</span>
				</div>
			{/if}
		</div>
	{/if}
{/if}
