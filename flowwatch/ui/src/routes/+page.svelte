<script lang="ts">
	import { onMount } from 'svelte';
	import { workflowClient, runClient } from '$lib/api/client';
	import StatCard from '$lib/components/StatCard.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';

	let workflows = $state<{ id: string; name: string; subsystemId: string }[]>([]);
	let recentRuns = $state<{ id: string; workflowId: string; foreignId: string; status: string; currentStep: string; startedAt: string }[]>([]);
	let subsystems = $state<{
		id: string; name: string; activeRuns: number; failedLastHour: number; errorRate: number; workflowCount: number;
	}[]>([]);
	let capabilities = $state<{ retryRun: boolean; pauseRun: boolean; cancelRun: boolean } | null>(null);
	let loading = $state(true);
	let error = $state('');

	onMount(async () => {
		try {
			console.log('[API]', 'Dashboard', 'loading all data');
			const [wfRes, capsRes, runsRes, ssRes] = await Promise.all([
				workflowClient.listWorkflows({}),
				workflowClient.getCapabilities({}),
				runClient.listRuns({ pagination: { limit: 15 } }),
				workflowClient.listSubsystems({}),
			]);
			console.log('[API Response]', 'Dashboard', 'all requests complete');

			workflows = (wfRes.workflows ?? []).map((w) => ({
				id: w.id, name: w.name, subsystemId: w.subsystemId,
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
				startedAt: r.startedAt ? new Date(Number(r.startedAt.seconds) * 1000).toLocaleString() : '',
			}));

			subsystems = (ssRes.subsystems ?? []).map((s) => ({
				id: s.subsystem?.id ?? '',
				name: s.subsystem?.name ?? s.subsystem?.id ?? '',
				activeRuns: s.activeRuns,
				failedLastHour: s.failedLastHour,
				errorRate: s.errorRateLastHour,
				workflowCount: s.workflowCount,
			}));
		} catch (e) {
			console.error('[API Error]', 'Dashboard', e);
			error = e instanceof Error ? e.message : 'Failed to load dashboard';
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

	function healthStatus(failed: number) {
		if (failed > 3) return { label: 'Degraded', color: 'text-failed' };
		if (failed > 0) return { label: 'Warning', color: 'text-paused' };
		return { label: 'Healthy', color: 'text-succeeded' };
	}

	const statusIcon: Record<string, string> = {
		running: '◉', succeeded: '✓', failed: '✗', paused: '⏸', canceled: '⊘', pending: '○',
	};

	const runningCount = $derived(recentRuns.filter((r) => r.status === 'running').length);
	const succeededCount = $derived(recentRuns.filter((r) => r.status === 'succeeded').length);
	const failedCount = $derived(recentRuns.filter((r) => r.status === 'failed').length);
	const pausedCount = $derived(recentRuns.filter((r) => r.status === 'paused').length);

	const alerts = $derived.by(() => {
		const a: { level: 'warning' | 'error'; message: string; href: string }[] = [];
		for (const ss of subsystems) {
			if (ss.failedLastHour > 3) {
				a.push({ level: 'error', message: `${ss.name}: ${ss.failedLastHour} failures in last hour`, href: `/runs?subsystem=${ss.id}` });
			} else if (ss.failedLastHour > 0) {
				a.push({ level: 'warning', message: `${ss.name}: ${ss.failedLastHour} failure${ss.failedLastHour > 1 ? 's' : ''} in last hour`, href: `/runs?subsystem=${ss.id}` });
			}
			if (ss.errorRate > 0.1) {
				a.push({ level: 'error', message: `${ss.name}: error rate ${(ss.errorRate * 100).toFixed(1)}%`, href: `/subsystems/${ss.id}` });
			}
		}
		return a;
	});
</script>

<svelte:head>
	<title>FlowWatch - Dashboard</title>
</svelte:head>

{#if loading}
	<p class="text-text-muted text-center py-12">Loading...</p>
{:else if error}
	<div class="bg-failed-bg border border-failed/20 text-failed p-4 rounded-lg">{error}</div>
{:else}
	<!-- Summary stat cards -->
	<div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-4 mb-6">
		<a href="/runs?status=running"><StatCard label="Running" value={runningCount} accentColor="border-b-running" /></a>
		<a href="/runs?status=succeeded"><StatCard label="Succeeded" value={succeededCount} accentColor="border-b-succeeded" /></a>
		<a href="/runs?status=failed"><StatCard label="Failed" value={failedCount} accentColor="border-b-failed" /></a>
		<a href="/runs?status=paused"><StatCard label="Paused" value={pausedCount} accentColor="border-b-paused" /></a>
		<StatCard label="Workflows" value={workflows.length} accentColor="border-b-accent" />
	</div>

	<!-- Live run ticker + Alert feed -->
	<div class="grid grid-cols-1 lg:grid-cols-3 gap-4 mb-6">
		<!-- Live Run Ticker -->
		<div class="lg:col-span-2 bg-surface border border-border rounded-lg overflow-hidden">
			<div class="px-4 py-3 border-b border-border flex items-center justify-between">
				<div class="flex items-center gap-2">
					<span class="w-2 h-2 rounded-full bg-succeeded animate-pulse-dot"></span>
					<span class="text-[11px] font-bold text-text-muted uppercase tracking-[1px]">Live Feed</span>
				</div>
				<a href="/runs" class="text-[11px] text-text-muted hover:text-accent transition-colors">View All</a>
			</div>
			<div class="max-h-72 overflow-auto">
				{#each recentRuns.slice(0, 8) as run, i}
					<a
						href="/runs/{run.id}"
						class="grid items-center gap-2 px-4 py-2.5 border-b border-border-subtle hover:bg-surface-hover transition-colors {i === 0 ? 'animate-slide-in bg-accent/[0.03]' : ''}"
						style="grid-template-columns: 24px 1fr 140px 80px 90px;"
					>
						<span class="text-sm {run.status === 'running' ? 'text-running' : run.status === 'succeeded' ? 'text-succeeded' : run.status === 'failed' ? 'text-failed' : run.status === 'paused' ? 'text-paused' : 'text-text-muted'}">{statusIcon[run.status] ?? '○'}</span>
						<span class="font-mono text-xs text-text-muted truncate">{run.id.slice(0, 8)}</span>
						<span class="text-xs text-text font-medium truncate">{run.workflowId}</span>
						<span class="text-[11px] text-text-muted truncate">{run.currentStep || '-'}</span>
						<span class="text-[11px] text-text-muted text-right">{run.startedAt}</span>
					</a>
				{:else}
					<p class="px-4 py-8 text-sm text-center text-text-muted">No recent runs</p>
				{/each}
			</div>
		</div>

		<!-- Alert Feed -->
		<div class="bg-surface border border-border rounded-lg overflow-hidden">
			<div class="px-4 py-3 border-b border-border">
				<span class="text-[11px] font-bold text-text-muted uppercase tracking-[1px]">Alert Feed</span>
			</div>
			<div class="max-h-72 overflow-auto">
				{#each alerts as alert}
					<a href={alert.href} class="flex items-start gap-2.5 px-4 py-3 border-b border-border-subtle hover:bg-surface-hover transition-colors">
						<span class="text-xs mt-0.5 {alert.level === 'error' ? 'text-failed' : 'text-paused'}">{alert.level === 'error' ? '✗' : '⚠'}</span>
						<div class="flex-1">
							<p class="text-xs {alert.level === 'error' ? 'text-failed' : 'text-paused'}">{alert.message}</p>
							<span class="text-[10px] text-accent mt-0.5">View details</span>
						</div>
					</a>
				{:else}
					<p class="px-4 py-8 text-sm text-center text-text-muted">No active alerts</p>
				{/each}
			</div>
		</div>
	</div>

	<!-- Subsystem Health Cards -->
	{#if subsystems.length > 0}
		<div class="mb-6">
			<h3 class="text-[11px] font-bold text-text-muted uppercase tracking-[1px] mb-3">Subsystem Health</h3>
			<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
				{#each subsystems as ss}
					{@const health = healthStatus(ss.failedLastHour)}
					<a
						href="/subsystems/{ss.id}"
						class="bg-surface rounded-lg border border-border p-4 hover:bg-surface-hover transition-all duration-150 hover:-translate-y-px"
					>
						<div class="flex justify-between items-center mb-3">
							<span class="text-sm font-semibold text-text-bright">{ss.name}</span>
							<span class="text-xs font-semibold {health.color}">{health.label}</span>
						</div>
						<div class="grid grid-cols-3 gap-2 text-xs">
							<div>
								<span class="block text-text-muted text-[10px] uppercase tracking-wider">Running</span>
								<span class="font-mono font-semibold text-running">{ss.activeRuns}</span>
							</div>
							<div>
								<span class="block text-text-muted text-[10px] uppercase tracking-wider">Failed/1h</span>
								<span class="font-mono font-semibold {ss.failedLastHour > 0 ? 'text-failed' : 'text-text-muted'}">{ss.failedLastHour}</span>
							</div>
							<div>
								<span class="block text-text-muted text-[10px] uppercase tracking-wider">Err Rate</span>
								<span class="font-mono font-semibold {ss.errorRate > 0.05 ? 'text-failed' : 'text-text-muted'}">{(ss.errorRate * 100).toFixed(1)}%</span>
							</div>
						</div>
						<div class="mt-3 pt-2 border-t border-border-subtle text-xs text-text-muted">
							{ss.workflowCount} workflow{ss.workflowCount !== 1 ? 's' : ''}
						</div>
					</a>
				{/each}
			</div>
		</div>
	{/if}

	<!-- Capabilities -->
	{#if capabilities}
		<div class="text-xs text-text-muted">
			Engine capabilities:
			{#if capabilities.retryRun}<span class="mr-2 px-1.5 py-0.5 bg-surface border border-border rounded font-mono">Retry</span>{/if}
			{#if capabilities.pauseRun}<span class="mr-2 px-1.5 py-0.5 bg-surface border border-border rounded font-mono">Pause</span>{/if}
			{#if capabilities.cancelRun}<span class="mr-2 px-1.5 py-0.5 bg-surface border border-border rounded font-mono">Cancel</span>{/if}
		</div>
	{/if}
{/if}
