<script lang="ts">
	import { onMount } from 'svelte';
	import { workflowClient } from '$lib/api/client';

	let workflows = $state<{ id: string; name: string; engine: string; subsystemId: string }[]>([]);
	let loading = $state(true);
	let error = $state('');

	onMount(async () => {
		try {
			console.log('[API]', 'ListWorkflows', 'loading');
			const res = await workflowClient.listWorkflows({});
			workflows = (res.workflows ?? []).map((w) => ({
				id: w.id,
				name: w.name,
				engine: w.engine,
				subsystemId: w.subsystemId,
			}));
			console.log('[API Response]', 'ListWorkflows', { count: workflows.length });
		} catch (e) {
			console.error('[API Error]', 'ListWorkflows', e);
			error = e instanceof Error ? e.message : 'Failed to load workflows';
		} finally {
			loading = false;
		}
	});
</script>

<svelte:head>
	<title>FlowWatch - Workflows</title>
</svelte:head>

<h2 class="text-lg font-semibold text-text-bright mb-4">Workflows</h2>

{#if loading}
	<p class="text-text-muted text-center py-8">Loading...</p>
{:else if error}
	<div class="bg-failed-bg border border-failed/20 text-failed p-4 rounded-lg">{error}</div>
{:else}
	<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
		{#each workflows as wf}
			<div class="bg-surface rounded-lg border border-border p-4 hover:bg-surface-hover transition-all duration-150 hover:-translate-y-px">
				<h3 class="font-medium text-text-bright">{wf.name}</h3>
				<p class="text-xs text-text-muted mt-1 font-mono">Engine: {wf.engine}</p>
				{#if wf.subsystemId}
					<p class="text-xs text-text-muted font-mono">Subsystem: {wf.subsystemId}</p>
				{/if}
				<div class="mt-3 pt-2 border-t border-border-subtle">
					<a href="/runs?workflow={wf.id}" class="text-xs text-accent hover:underline">View Runs</a>
				</div>
			</div>
		{:else}
			<p class="text-text-muted col-span-full text-center py-8">No workflows registered</p>
		{/each}
	</div>
{/if}
