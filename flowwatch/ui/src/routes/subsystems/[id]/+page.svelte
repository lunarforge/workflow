<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { workflowClient, runClient } from '$lib/api/client';
	import StatusBadge from '$lib/components/StatusBadge.svelte';

	const subsystemId = $derived(page.params.id ?? '');

	let subsystem = $state<{
		id: string;
		name: string;
		workflowCount: number;
		activeRuns: number;
		failedLastHour: number;
		errorRate: number;
	} | null>(null);
	let workflows = $state<{ id: string; name: string; engine: string }[]>([]);
	let recentRuns = $state<{ id: string; workflowId: string; foreignId: string; status: string; currentStep: string; startedAt: string }[]>([]);
	let loading = $state(true);
	let error = $state('');

	function statusToString(status: number): string {
		const map: Record<number, string> = {
			0: 'unknown', 1: 'pending', 2: 'running', 3: 'succeeded',
			4: 'failed', 5: 'canceled', 6: 'paused',
		};
		return map[status] ?? 'unknown';
	}

	function healthStatus(failed: number) {
		if (failed > 3) return { label: 'Degraded', color: 'text-failed' };
		if (failed > 0) return { label: 'Warning', color: 'text-paused' };
		return { label: 'Healthy', color: 'text-succeeded' };
	}

	onMount(async () => {
		try {
			console.log('[API]', 'GetSubsystem', { subsystemId });
			const [ssRes, runsRes] = await Promise.all([
				workflowClient.getSubsystem({ subsystemId }),
				runClient.listRuns({ filter: { subsystemIds: [subsystemId] }, pagination: { limit: 20 } }),
			]);
			console.log('[API Response]', 'GetSubsystem', { subsystemId });

			if (ssRes.subsystem) {
				subsystem = {
					id: ssRes.subsystem.subsystem?.id ?? '',
					name: ssRes.subsystem.subsystem?.name ?? ssRes.subsystem.subsystem?.id ?? '',
					workflowCount: ssRes.subsystem.workflowCount,
					activeRuns: ssRes.subsystem.activeRuns,
					failedLastHour: ssRes.subsystem.failedLastHour,
					errorRate: ssRes.subsystem.errorRateLastHour,
				};
			}

			workflows = (ssRes.workflows ?? []).map((w) => ({
				id: w.id,
				name: w.name,
				engine: w.engine,
			}));

			recentRuns = (runsRes.runs ?? []).map((r) => ({
				id: r.id,
				workflowId: r.workflowId,
				foreignId: r.foreignId,
				status: statusToString(r.status),
				currentStep: r.currentStep,
				startedAt: r.startedAt ? new Date(Number(r.startedAt.seconds) * 1000).toLocaleString() : '',
			}));
		} catch (e) {
			console.error('[API Error]', 'GetSubsystem', e);
			error = e instanceof Error ? e.message : 'Failed to load subsystem';
		} finally {
			loading = false;
		}
	});
</script>

<svelte:head>
	<title>FlowWatch - {subsystemId}</title>
</svelte:head>

{#if loading}
	<p class="text-center text-text-muted py-8">Loading subsystem...</p>
{:else if error}
	<div class="bg-failed-bg border border-failed/20 text-failed p-4 rounded-lg mb-4">{error}</div>
{:else if subsystem}
	{@const health = healthStatus(subsystem.failedLastHour)}
	<div class="flex items-center justify-between mb-4">
		<div>
			<h2 class="text-lg font-semibold text-text-bright">{subsystem.name}</h2>
			<p class="text-sm text-text-muted mt-0.5">
				<a href="/subsystems" class="hover:text-accent">Subsystems</a> / {subsystem.id}
			</p>
		</div>
		<span class="px-3 py-1 rounded-full text-xs font-semibold {health.color} bg-surface border border-border">{health.label}</span>
	</div>

	<!-- Stats row -->
	<div class="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
		<div class="bg-surface rounded-lg border border-border p-4 border-b-3 border-b-running">
			<p class="text-[10px] text-text-muted uppercase tracking-wider mb-1">Active Runs</p>
			<p class="text-2xl font-bold font-mono text-running">{subsystem.activeRuns}</p>
		</div>
		<div class="bg-surface rounded-lg border border-border p-4 border-b-3 border-b-failed">
			<p class="text-[10px] text-text-muted uppercase tracking-wider mb-1">Failed / 1h</p>
			<p class="text-2xl font-bold font-mono {subsystem.failedLastHour > 0 ? 'text-failed' : 'text-text-muted'}">{subsystem.failedLastHour}</p>
		</div>
		<div class="bg-surface rounded-lg border border-border p-4 border-b-3 border-b-paused">
			<p class="text-[10px] text-text-muted uppercase tracking-wider mb-1">Error Rate</p>
			<p class="text-2xl font-bold font-mono {subsystem.errorRate > 0.05 ? 'text-failed' : 'text-text-muted'}">{(subsystem.errorRate * 100).toFixed(1)}%</p>
		</div>
		<div class="bg-surface rounded-lg border border-border p-4 border-b-3 border-b-accent">
			<p class="text-[10px] text-text-muted uppercase tracking-wider mb-1">Workflows</p>
			<p class="text-2xl font-bold font-mono text-text-bright">{subsystem.workflowCount}</p>
		</div>
	</div>

	<!-- Workflows -->
	{#if workflows.length > 0}
		<h3 class="text-[11px] font-bold text-text-muted uppercase tracking-[1px] mb-3">Workflows</h3>
		<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3 mb-6">
			{#each workflows as wf}
				<a
					href="/runs?workflow={wf.id}"
					class="bg-surface rounded-lg border border-border p-4 hover:bg-surface-hover transition-all"
				>
					<span class="text-sm font-medium text-text-bright">{wf.name}</span>
					<span class="block text-xs text-text-muted font-mono mt-1">Engine: {wf.engine}</span>
				</a>
			{/each}
		</div>
	{/if}

	<!-- Recent runs -->
	<h3 class="text-[11px] font-bold text-text-muted uppercase tracking-[1px] mb-3">Recent Runs</h3>
	<div class="bg-surface rounded-lg border border-border overflow-hidden">
		<table class="w-full text-sm">
			<thead>
				<tr class="border-b border-border text-[10px] text-text-muted uppercase tracking-wider">
					<th class="text-left px-4 py-2 font-bold">Run ID</th>
					<th class="text-left px-4 py-2 font-bold">Workflow</th>
					<th class="text-left px-4 py-2 font-bold">Status</th>
					<th class="text-left px-4 py-2 font-bold">Step</th>
					<th class="text-left px-4 py-2 font-bold">Started</th>
				</tr>
			</thead>
			<tbody>
				{#each recentRuns as run}
					<tr class="border-b border-border-subtle hover:bg-surface-hover">
						<td class="px-4 py-2 font-mono text-xs">
							<a href="/runs/{run.id}" class="text-accent hover:underline">{run.id.slice(0, 8)}...</a>
						</td>
						<td class="px-4 py-2 text-text">{run.workflowId}</td>
						<td class="px-4 py-2"><StatusBadge status={run.status} /></td>
						<td class="px-4 py-2 text-text-muted">{run.currentStep}</td>
						<td class="px-4 py-2 text-xs text-text-muted">{run.startedAt}</td>
					</tr>
				{:else}
					<tr>
						<td colspan="5" class="px-4 py-8 text-center text-text-muted">No runs found</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}
