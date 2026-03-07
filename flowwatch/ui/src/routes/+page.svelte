<script lang="ts">
	import { onMount } from 'svelte';
	import { workflowClient, runClient } from '$lib/api/client';
	import StatCard from '$lib/components/StatCard.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';

	let workflows = $state<{ id: string; name: string; subsystemId: string }[]>([]);
	let recentRuns = $state<{ id: string; workflowId: string; foreignId: string; status: string; currentStep: string }[]>([]);
	let capabilities = $state<{ retryRun: boolean; pauseRun: boolean; cancelRun: boolean } | null>(null);
	let loading = $state(true);
	let error = $state('');

	onMount(async () => {
		try {
			const [wfRes, capsRes, runsRes] = await Promise.all([
				workflowClient.listWorkflows({}),
				workflowClient.getCapabilities({}),
				runClient.listRuns({ pagination: { limit: 10 } }),
			]);

			workflows = (wfRes.workflows ?? []).map((w) => ({
				id: w.id,
				name: w.name,
				subsystemId: w.subsystemId,
			}));

			capabilities = {
				retryRun: capsRes.capabilities?.retryRun ?? false,
				pauseRun: capsRes.capabilities?.pauseRun ?? false,
				cancelRun: capsRes.capabilities?.cancelRun ?? false,
			};

			recentRuns = (runsRes.runs ?? []).map((r) => ({
				id: r.id,
				workflowId: r.workflowId,
				foreignId: r.foreignId,
				status: statusToString(r.status),
				currentStep: r.currentStep,
			}));

			console.log('[API]', 'Dashboard loaded', { workflows: workflows.length, runs: recentRuns.length });
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load dashboard';
			console.error('[API Error]', 'Dashboard', e);
		} finally {
			loading = false;
		}
	});

	function statusToString(status: number): string {
		const map: Record<number, string> = {
			0: 'unknown', 1: 'pending', 2: 'running', 3: 'succeeded',
			4: 'failed', 5: 'canceled', 6: 'paused',
		};
		return map[status] ?? 'unknown';
	}
</script>

<svelte:head>
	<title>FlowWatch - Dashboard</title>
</svelte:head>

<h2 class="text-xl font-semibold mb-4">Dashboard</h2>

{#if loading}
	<p class="text-muted">Loading...</p>
{:else if error}
	<div class="bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-300 p-4 rounded-lg">
		{error}
	</div>
{:else}
	<!-- Stats -->
	<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
		<StatCard label="Workflows" value={workflows.length} />
		<StatCard label="Recent Runs" value={recentRuns.length} />
		<StatCard
			label="Running"
			value={recentRuns.filter((r) => r.status === 'running').length}
			color="text-blue-600 dark:text-blue-400"
		/>
		<StatCard
			label="Failed"
			value={recentRuns.filter((r) => r.status === 'failed').length}
			color="text-red-600 dark:text-red-400"
		/>
	</div>

	<!-- Capabilities -->
	{#if capabilities}
		<div class="mb-6 text-sm text-muted">
			Engine capabilities:
			{#if capabilities.retryRun}<span class="mr-2">Retry</span>{/if}
			{#if capabilities.pauseRun}<span class="mr-2">Pause</span>{/if}
			{#if capabilities.cancelRun}<span class="mr-2">Cancel</span>{/if}
		</div>
	{/if}

	<!-- Recent Runs Table -->
	<h3 class="text-lg font-medium mb-3">Recent Runs</h3>
	<div class="bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-800 overflow-hidden">
		<table class="w-full text-sm">
			<thead>
				<tr class="border-b border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-800/50">
					<th class="text-left px-4 py-2 font-medium">Run ID</th>
					<th class="text-left px-4 py-2 font-medium">Workflow</th>
					<th class="text-left px-4 py-2 font-medium">Foreign ID</th>
					<th class="text-left px-4 py-2 font-medium">Status</th>
					<th class="text-left px-4 py-2 font-medium">Current Step</th>
				</tr>
			</thead>
			<tbody>
				{#each recentRuns as run}
					<tr class="border-b border-gray-100 dark:border-gray-800 hover:bg-gray-50 dark:hover:bg-gray-800/30">
						<td class="px-4 py-2 font-mono text-xs">
							<a href="/runs/{run.id}" class="text-blue-600 dark:text-blue-400 hover:underline">
								{run.id.slice(0, 8)}...
							</a>
						</td>
						<td class="px-4 py-2">{run.workflowId}</td>
						<td class="px-4 py-2 font-mono text-xs">{run.foreignId}</td>
						<td class="px-4 py-2"><StatusBadge status={run.status} /></td>
						<td class="px-4 py-2">{run.currentStep}</td>
					</tr>
				{:else}
					<tr>
						<td colspan="5" class="px-4 py-8 text-center text-muted">No runs found</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}
