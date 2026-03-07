<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { runClient } from '$lib/api/client';
	import StatusBadge from '$lib/components/StatusBadge.svelte';

	let runs = $state<{ id: string; workflowId: string; foreignId: string; status: string; currentStep: string; startedAt: string }[]>([]);
	let loading = $state(true);
	let error = $state('');
	let cursor = $state('');
	let hasMore = $state(false);

	const workflowFilter = $derived(page.url.searchParams.get('workflow') ?? '');

	onMount(() => loadRuns());

	async function loadRuns() {
		loading = true;
		try {
			const filter: Record<string, unknown> = {};
			if (workflowFilter) {
				filter.workflowIds = [workflowFilter];
			}

			const res = await runClient.listRuns({
				filter,
				pagination: { limit: 25, cursor },
			});

			const mapped = (res.runs ?? []).map((r) => ({
				id: r.id,
				workflowId: r.workflowId,
				foreignId: r.foreignId,
				status: statusToString(r.status),
				currentStep: r.currentStep,
				startedAt: r.startedAt ? new Date(Number(r.startedAt.seconds) * 1000).toLocaleString() : '',
			}));

			runs = [...runs, ...mapped];
			hasMore = (res.pagination?.nextCursor ?? '') !== '';
			cursor = res.pagination?.nextCursor ?? '';

			console.log('[API]', 'ListRuns', { count: mapped.length, hasMore });
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load runs';
			console.error('[API Error]', 'ListRuns', e);
		} finally {
			loading = false;
		}
	}

	function statusToString(status: number): string {
		const map: Record<number, string> = {
			0: 'unknown', 1: 'pending', 2: 'running', 3: 'succeeded',
			4: 'failed', 5: 'canceled', 6: 'paused',
		};
		return map[status] ?? 'unknown';
	}
</script>

<svelte:head>
	<title>FlowWatch - Runs</title>
</svelte:head>

<div class="flex items-center justify-between mb-4">
	<h2 class="text-xl font-semibold">
		Runs
		{#if workflowFilter}
			<span class="text-sm font-normal text-muted ml-2">({workflowFilter})</span>
		{/if}
	</h2>
</div>

{#if error}
	<div class="bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-300 p-4 rounded-lg mb-4">{error}</div>
{/if}

<div class="bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-800 overflow-hidden">
	<table class="w-full text-sm">
		<thead>
			<tr class="border-b border-gray-200 dark:border-gray-800 bg-gray-50 dark:bg-gray-800/50">
				<th class="text-left px-4 py-2 font-medium">Run ID</th>
				<th class="text-left px-4 py-2 font-medium">Workflow</th>
				<th class="text-left px-4 py-2 font-medium">Foreign ID</th>
				<th class="text-left px-4 py-2 font-medium">Status</th>
				<th class="text-left px-4 py-2 font-medium">Step</th>
				<th class="text-left px-4 py-2 font-medium">Started</th>
			</tr>
		</thead>
		<tbody>
			{#each runs as run}
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
					<td class="px-4 py-2 text-xs text-muted">{run.startedAt}</td>
				</tr>
			{:else}
				<tr>
					<td colspan="6" class="px-4 py-8 text-center text-muted">
						{loading ? 'Loading...' : 'No runs found'}
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>

{#if hasMore && !loading}
	<div class="mt-4 text-center">
		<button
			onclick={loadRuns}
			class="px-4 py-2 text-sm bg-gray-100 dark:bg-gray-800 rounded-md hover:bg-gray-200 dark:hover:bg-gray-700"
		>
			Load More
		</button>
	</div>
{/if}

{#if loading && runs.length > 0}
	<p class="mt-4 text-center text-sm text-muted">Loading more...</p>
{/if}
