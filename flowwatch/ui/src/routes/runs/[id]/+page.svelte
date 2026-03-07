<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { runClient } from '$lib/api/client';
	import StatusBadge from '$lib/components/StatusBadge.svelte';

	const runId = $derived(page.params.id);

	let run = $state<{
		id: string;
		workflowId: string;
		foreignId: string;
		status: string;
		currentStep: string;
		startedAt: string;
		finishedAt: string;
		duration: string;
		errorMessage: string;
	} | null>(null);

	let steps = $state<{
		id: string;
		stepName: string;
		status: string;
		attempt: number;
		startedAt: string;
		finishedAt: string;
		duration: string;
		errorMessage: string;
	}[]>([]);

	let timeline = $state<{
		stepName: string;
		status: string;
		attempt: number;
		offsetMs: number;
		durationMs: number;
		errorMessage: string;
	}[]>([]);
	let totalDurationMs = $state(0);

	let loading = $state(true);
	let error = $state('');
	let activeTab = $state<'steps' | 'timeline'>('steps');

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
			const res = await runClient.getRun({ runId });
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
				};
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load run';
		} finally {
			loading = false;
		}
	}

	async function loadSteps() {
		try {
			const res = await runClient.getRunSteps({ runId });
			steps = (res.steps ?? []).map((s) => ({
				id: s.id,
				stepName: s.stepName,
				status: stepStatusFromNumber(s.status),
				attempt: s.attempt,
				startedAt: formatTimestamp(s.startedAt),
				finishedAt: formatTimestamp(s.finishedAt),
				duration: formatDuration(s.duration),
				errorMessage: s.error?.message ?? '',
			}));
		} catch (e) {
			console.error('[API Error]', 'GetRunSteps', e);
		}
	}

	async function loadTimeline() {
		try {
			const res = await runClient.getRunTimeline({ runId });
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

	async function handleAction(action: 'cancel' | 'pause' | 'resume' | 'retry') {
		try {
			if (action === 'cancel') await runClient.cancelRun({ runId, reason: '' });
			else if (action === 'pause') await runClient.pauseRun({ runId });
			else if (action === 'resume') await runClient.resumeRun({ runId });
			else if (action === 'retry') await runClient.retryRun({ runId });
			await loadRun();
			await loadSteps();
		} catch (e) {
			error = e instanceof Error ? e.message : `Failed to ${action} run`;
		}
	}
</script>

<svelte:head>
	<title>FlowWatch - Run {runId.slice(0, 8)}...</title>
</svelte:head>

{#if loading}
	<p class="text-center text-muted py-8">Loading run...</p>
{:else if error}
	<div class="bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-300 p-4 rounded-lg mb-4">{error}</div>
{:else if run}
	<div class="flex items-center justify-between mb-4">
		<div>
			<h2 class="text-xl font-semibold">Run {run.id.slice(0, 8)}...</h2>
			<p class="text-sm text-muted mt-1">
				<a href="/workflows" class="hover:underline">{run.workflowId}</a>
				{#if run.foreignId}
					&middot; <span class="font-mono">{run.foreignId}</span>
				{/if}
			</p>
		</div>
		<div class="flex items-center gap-2">
			{#if run.status === 'running'}
				<button onclick={() => handleAction('pause')} class="px-3 py-1.5 text-sm bg-yellow-100 dark:bg-yellow-900/30 text-yellow-800 dark:text-yellow-200 rounded-md hover:bg-yellow-200 dark:hover:bg-yellow-900/50">Pause</button>
				<button onclick={() => handleAction('cancel')} class="px-3 py-1.5 text-sm bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-200 rounded-md hover:bg-red-200 dark:hover:bg-red-900/50">Cancel</button>
			{/if}
			{#if run.status === 'paused'}
				<button onclick={() => handleAction('resume')} class="px-3 py-1.5 text-sm bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-200 rounded-md hover:bg-blue-200 dark:hover:bg-blue-900/50">Resume</button>
			{/if}
			{#if run.status === 'failed'}
				<button onclick={() => handleAction('retry')} class="px-3 py-1.5 text-sm bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-200 rounded-md hover:bg-green-200 dark:hover:bg-green-900/50">Retry</button>
			{/if}
		</div>
	</div>

	<!-- Run metadata -->
	<div class="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
		<div class="bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-800 p-4">
			<p class="text-xs text-muted mb-1">Status</p>
			<StatusBadge status={run.status} />
		</div>
		<div class="bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-800 p-4">
			<p class="text-xs text-muted mb-1">Current Step</p>
			<p class="font-medium">{run.currentStep || '-'}</p>
		</div>
		<div class="bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-800 p-4">
			<p class="text-xs text-muted mb-1">Duration</p>
			<p class="font-medium">{run.duration}</p>
		</div>
		<div class="bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-800 p-4">
			<p class="text-xs text-muted mb-1">Started</p>
			<p class="text-sm">{run.startedAt}</p>
		</div>
	</div>

	{#if run.errorMessage}
		<div class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-4 mb-6">
			<p class="text-xs font-medium text-red-800 dark:text-red-200 mb-1">Error</p>
			<p class="text-sm text-red-700 dark:text-red-300 font-mono">{run.errorMessage}</p>
		</div>
	{/if}

	<!-- Tabs -->
	<div class="flex border-b border-gray-200 dark:border-gray-800 mb-4">
		<button
			onclick={() => activeTab = 'steps'}
			class="px-4 py-2 text-sm font-medium border-b-2 -mb-px {activeTab === 'steps' ? 'border-blue-500 text-blue-600 dark:text-blue-400' : 'border-transparent text-muted hover:text-gray-700 dark:hover:text-gray-300'}"
		>Steps ({steps.length})</button>
		<button
			onclick={() => activeTab = 'timeline'}
			class="px-4 py-2 text-sm font-medium border-b-2 -mb-px {activeTab === 'timeline' ? 'border-blue-500 text-blue-600 dark:text-blue-400' : 'border-transparent text-muted hover:text-gray-700 dark:hover:text-gray-300'}"
		>Timeline</button>
	</div>

	{#if activeTab === 'steps'}
		<div class="bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-800 overflow-hidden">
			<table class="w-full text-sm">
				<thead>
					<tr class="border-b border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-800/50">
						<th class="text-left px-4 py-2 font-medium">Step</th>
						<th class="text-left px-4 py-2 font-medium">Status</th>
						<th class="text-left px-4 py-2 font-medium">Attempt</th>
						<th class="text-left px-4 py-2 font-medium">Duration</th>
						<th class="text-left px-4 py-2 font-medium">Started</th>
						<th class="text-left px-4 py-2 font-medium">Error</th>
					</tr>
				</thead>
				<tbody>
					{#each steps as step}
						<tr class="border-b border-gray-100 dark:border-gray-800 hover:bg-gray-50 dark:hover:bg-gray-800/30">
							<td class="px-4 py-2 font-medium">{step.stepName}</td>
							<td class="px-4 py-2"><StatusBadge status={step.status} /></td>
							<td class="px-4 py-2">{step.attempt}</td>
							<td class="px-4 py-2 font-mono text-xs">{step.duration}</td>
							<td class="px-4 py-2 text-xs text-muted">{step.startedAt}</td>
							<td class="px-4 py-2 text-xs text-red-600 dark:text-red-400 max-w-xs truncate">{step.errorMessage}</td>
						</tr>
					{:else}
						<tr>
							<td colspan="6" class="px-4 py-8 text-center text-muted">No steps recorded</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}

	{#if activeTab === 'timeline'}
		<div class="bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-800 p-4">
			{#if timeline.length === 0}
				<p class="text-center text-muted py-8">No timeline data available</p>
			{:else}
				<div class="space-y-2">
					{#each timeline as entry}
						{@const left = totalDurationMs > 0 ? (entry.offsetMs / totalDurationMs) * 100 : 0}
						{@const width = totalDurationMs > 0 ? Math.max((entry.durationMs / totalDurationMs) * 100, 1) : 100}
						<div class="flex items-center gap-3">
							<div class="w-32 text-sm truncate shrink-0">
								<span class="font-medium">{entry.stepName}</span>
								{#if entry.attempt > 1}
									<span class="text-xs text-muted ml-1">#{entry.attempt}</span>
								{/if}
							</div>
							<div class="flex-1 h-6 bg-gray-100 dark:bg-gray-800 rounded relative">
								<div
									class="absolute h-full rounded {entry.status === 'succeeded' ? 'bg-green-500' : entry.status === 'failed' ? 'bg-red-500' : entry.status === 'running' ? 'bg-blue-500' : 'bg-gray-400'}"
									style="left: {left}%; width: {width}%"
									title="{entry.stepName}: {entry.durationMs}ms"
								></div>
							</div>
							<div class="w-20 text-xs text-muted text-right shrink-0">
								{entry.durationMs < 1000 ? `${entry.durationMs}ms` : `${(entry.durationMs / 1000).toFixed(1)}s`}
							</div>
						</div>
					{/each}
				</div>
				<p class="text-xs text-muted mt-3 text-right">Total: {formatDuration({ seconds: BigInt(Math.floor(totalDurationMs / 1000)), nanos: (totalDurationMs % 1000) * 1e6 })}</p>
			{/if}
		</div>
	{/if}
{/if}
