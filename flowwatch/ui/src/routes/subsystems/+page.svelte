<script lang="ts">
	import { onMount } from 'svelte';
	import { workflowClient } from '$lib/api/client';

	let subsystems = $state<{
		id: string;
		name: string;
		workflowCount: number;
		activeRuns: number;
		failedLastHour: number;
		errorRate: number;
	}[]>([]);
	let loading = $state(true);
	let error = $state('');

	onMount(() => loadSubsystems());

	async function loadSubsystems() {
		try {
			console.log('[API]', 'ListSubsystems', 'loading');
			const res = await workflowClient.listSubsystems({});
			subsystems = (res.subsystems ?? []).map((s) => ({
				id: s.subsystem?.id ?? '',
				name: s.subsystem?.name ?? s.subsystem?.id ?? '',
				workflowCount: s.workflowCount,
				activeRuns: s.activeRuns,
				failedLastHour: s.failedLastHour,
				errorRate: s.errorRateLastHour,
			}));
			console.log('[API Response]', 'ListSubsystems', { count: subsystems.length });
		} catch (e) {
			console.error('[API Error]', 'ListSubsystems', e);
			error = e instanceof Error ? e.message : 'Failed to load subsystems';
		} finally {
			loading = false;
		}
	}

	function healthStatus(failed: number) {
		if (failed > 3) return { label: 'Degraded', color: 'text-failed' };
		if (failed > 0) return { label: 'Warning', color: 'text-paused' };
		return { label: 'Healthy', color: 'text-succeeded' };
	}
</script>

<svelte:head>
	<title>FlowWatch - Subsystems</title>
</svelte:head>

<h2 class="text-lg font-semibold text-text-bright mb-4">Subsystems</h2>

{#if error}
	<div class="bg-failed-bg border border-failed/20 text-failed p-4 rounded-lg mb-4">{error}</div>
{/if}

{#if loading}
	<p class="text-center text-text-muted py-8">Loading subsystems...</p>
{:else if subsystems.length === 0}
	<p class="text-center text-text-muted py-8">No subsystems found</p>
{:else}
	<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
		{#each subsystems as ss}
			{@const health = healthStatus(ss.failedLastHour)}
			<a
				href="/subsystems/{ss.id}"
				class="block bg-surface rounded-lg border border-border p-5 hover:bg-surface-hover transition-all duration-150 hover:-translate-y-px"
			>
				<div class="flex justify-between items-center mb-3">
					<span class="text-sm font-semibold text-text-bright">{ss.name}</span>
					<span class="text-xs font-semibold {health.color}">{health.label}</span>
				</div>

				<div class="grid grid-cols-3 gap-3 text-xs">
					<div>
						<span class="block text-text-muted uppercase tracking-wider text-[10px] mb-1">Running</span>
						<span class="font-mono font-semibold text-running">{ss.activeRuns}</span>
					</div>
					<div>
						<span class="block text-text-muted uppercase tracking-wider text-[10px] mb-1">Failed/1h</span>
						<span class="font-mono font-semibold {ss.failedLastHour > 0 ? 'text-failed' : 'text-text-muted'}">{ss.failedLastHour}</span>
					</div>
					<div>
						<span class="block text-text-muted uppercase tracking-wider text-[10px] mb-1">Error Rate</span>
						<span class="font-mono font-semibold {ss.errorRate > 0.05 ? 'text-failed' : 'text-text-muted'}">{(ss.errorRate * 100).toFixed(1)}%</span>
					</div>
				</div>

				<div class="mt-3 pt-3 border-t border-border-subtle text-xs text-text-muted">
					{ss.workflowCount} workflow{ss.workflowCount !== 1 ? 's' : ''}
				</div>
			</a>
		{/each}
	</div>
{/if}
