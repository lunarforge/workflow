<script lang="ts">
	import { onMount } from 'svelte';
	import { workflowClient } from '$lib/api/client';

	let workflows = $state<{ id: string; name: string; engine: string; subsystemId: string }[]>([]);
	let loading = $state(true);
	let error = $state('');

	onMount(async () => {
		try {
			const res = await workflowClient.listWorkflows({});
			workflows = (res.workflows ?? []).map((w) => ({
				id: w.id,
				name: w.name,
				engine: w.engine,
				subsystemId: w.subsystemId,
			}));
			console.log('[API]', 'ListWorkflows', { count: workflows.length });
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load workflows';
			console.error('[API Error]', 'ListWorkflows', e);
		} finally {
			loading = false;
		}
	});
</script>

<svelte:head>
	<title>FlowWatch - Workflows</title>
</svelte:head>

<h2 class="text-xl font-semibold mb-4">Workflows</h2>

{#if loading}
	<p class="text-muted">Loading...</p>
{:else if error}
	<div class="bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-300 p-4 rounded-lg">{error}</div>
{:else}
	<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
		{#each workflows as wf}
			<div class="bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-800 p-4">
				<h3 class="font-medium text-base">{wf.name}</h3>
				<p class="text-xs text-muted mt-1">Engine: {wf.engine}</p>
				{#if wf.subsystemId}
					<p class="text-xs text-muted">Subsystem: {wf.subsystemId}</p>
				{/if}
				<div class="mt-3 flex gap-2">
					<a href="/runs?workflow={wf.id}" class="text-xs text-blue-600 dark:text-blue-400 hover:underline">
						View Runs
					</a>
				</div>
			</div>
		{:else}
			<p class="text-muted col-span-full">No workflows registered</p>
		{/each}
	</div>
{/if}
