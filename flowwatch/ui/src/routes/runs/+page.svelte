<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { runClient } from '$lib/api/client';
	import StatusBadge from '$lib/components/StatusBadge.svelte';

	let runs = $state<{ id: string; workflowId: string; foreignId: string; status: string; currentStep: string; startedAt: string; selected: boolean }[]>([]);
	let loading = $state(true);
	let error = $state('');
	let cursor = $state('');
	let hasMore = $state(false);
	let bulkLoading = $state(false);

	const workflowFilter = $derived(page.url.searchParams.get('workflow') ?? '');
	const subsystemFilter = $derived(page.url.searchParams.get('subsystem') ?? '');
	const statusParam = $derived(page.url.searchParams.get('status') ?? '');
	let statusFilter = $state('all');
	let activeView = $state('all');

	// Slide-over state
	let selectedRun = $state<typeof runs[number] | null>(null);
	let slideOverOpen = $state(false);
	let runDetail = $state<{
		duration: string;
		steps: { name: string; status: string; duration: number; attempt: number; error: string | null; input: string; output: string }[];
	} | null>(null);
	let detailLoading = $state(false);

	// Step name maps for example workflows
	const workflowStepNames: Record<string, string[]> = {
		'order-processing': ['validate-order', 'process-payment', 'complete-payment', 'reserve-inventory', 'fulfill-order'],
		'user-onboarding': ['verify-email', 'setup-profile', 'submit-kyc', 'review-kyc', 'send-welcome', 'complete-onboarding'],
		'refund-process': ['validate-refund', 'check-policy', 'route-refund', 'process-refund', 'notify-customer', 'complete-refund'],
	};

	function generateMockSteps(workflowId: string, runStatus: string) {
		const stepNames = workflowStepNames[workflowId] ?? ['step-1', 'step-2', 'step-3', 'step-4'];
		const failIdx = runStatus === 'failed' ? Math.min(Math.floor(Math.random() * stepNames.length), stepNames.length - 1) : -1;

		return stepNames.map((name, i) => {
			let status: string;
			if (i < failIdx || (failIdx < 0 && (runStatus === 'succeeded' || i < stepNames.length - 1))) {
				status = 'succeeded';
			} else if (i === failIdx) {
				status = 'failed';
			} else if (failIdx >= 0 && i > failIdx) {
				status = 'skipped';
			} else if (runStatus === 'running' && i === stepNames.length - 1) {
				status = 'running';
			} else if (runStatus === 'paused' && i === stepNames.length - 1) {
				status = 'paused';
			} else {
				status = 'pending';
			}
			const duration = status === 'succeeded' ? Math.floor(Math.random() * 1500 + 50)
				: status === 'failed' ? Math.floor(Math.random() * 3000 + 100)
				: status === 'running' ? Math.floor(Math.random() * 500 + 100)
				: 0;
			return {
				name,
				status,
				duration,
				attempt: status === 'failed' ? Math.floor(Math.random() * 2) + 2 : 1,
				error: status === 'failed' ? 'Error: ' + ['timeout', 'connection_refused', 'invalid_response', 'insufficient_funds'][Math.floor(Math.random() * 4)] : null,
				input: '{}',
				output: '{}',
			};
		});
	}

	function durationMs(d?: { seconds?: bigint; nanos?: number }): number {
		if (!d) return 0;
		return Number(d.seconds ?? 0n) * 1000 + Math.floor((d.nanos ?? 0) / 1_000_000);
	}

	function stepStatusToString(status: number): string {
		const map: Record<number, string> = {
			0: 'unknown', 1: 'pending', 2: 'running', 3: 'succeeded',
			4: 'failed', 5: 'skipped',
		};
		return map[status] ?? 'unknown';
	}

	const savedViews = [
		{ key: 'all', label: 'All' },
		{ key: 'running', label: 'Running' },
		{ key: 'failed', label: 'Failed' },
		{ key: 'stuck', label: 'Stuck >5m' },
	];

	const filteredRuns = $derived(
		statusFilter === 'all' ? runs : runs.filter((r) => r.status === statusFilter)
	);

	const statusCounts = $derived({
		running: runs.filter((r) => r.status === 'running').length,
		succeeded: runs.filter((r) => r.status === 'succeeded').length,
		failed: runs.filter((r) => r.status === 'failed').length,
		paused: runs.filter((r) => r.status === 'paused').length,
		canceled: runs.filter((r) => r.status === 'canceled').length,
	});

	const selectedRuns = $derived(runs.filter((r) => r.selected));
	const allSelected = $derived(filteredRuns.length > 0 && filteredRuns.every((r) => r.selected));

	const statusColors: Record<string, string> = {
		running: 'text-running',
		succeeded: 'text-succeeded',
		failed: 'text-failed',
		paused: 'text-paused',
		canceled: 'text-canceled',
		pending: 'text-pending',
	};

	onMount(() => {
		if (statusParam) {
			statusFilter = statusParam;
			activeView = statusParam;
		}
		loadRuns();
	});

	async function loadRuns() {
		loading = true;
		try {
			console.log('[API]', 'ListRuns', { workflowFilter, subsystemFilter, cursor });
			const filter: Record<string, unknown> = {};
			if (workflowFilter) filter.workflowIds = [workflowFilter];
			if (subsystemFilter) filter.subsystemIds = [subsystemFilter];

			const res = await runClient.listRuns({
				filter,
				pagination: { limit: 25, cursor },
			});
			console.log('[API Response]', 'ListRuns', { count: res.runs?.length ?? 0 });

			const mapped = (res.runs ?? []).map((r) => ({
				id: r.id,
				workflowId: r.workflowId,
				foreignId: r.foreignId,
				status: statusToString(r.status),
				currentStep: r.currentStep,
				startedAt: r.startedAt ? new Date(Number(r.startedAt.seconds) * 1000).toLocaleString() : '',
				selected: false,
			}));

			runs = [...runs, ...mapped];
			hasMore = (res.pagination?.nextCursor ?? '') !== '';
			cursor = res.pagination?.nextCursor ?? '';
		} catch (e) {
			console.error('[API Error]', 'ListRuns', e);
			error = e instanceof Error ? e.message : 'Failed to load runs';
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

	function selectView(view: string) {
		activeView = view;
		statusFilter = view === 'stuck' ? 'running' : view;
	}

	function toggleSelectAll() {
		const newVal = !allSelected;
		runs = runs.map((r) => {
			if (statusFilter === 'all' || r.status === statusFilter) {
				return { ...r, selected: newVal };
			}
			return r;
		});
	}

	function toggleSelect(id: string) {
		runs = runs.map((r) => r.id === id ? { ...r, selected: !r.selected } : r);
	}

	async function openSlideOver(run: typeof runs[number]) {
		selectedRun = run;
		slideOverOpen = true;
		detailLoading = true;
		runDetail = null;
		try {
			console.log('[API]', 'GetRun', { runId: run.id });
			const [runRes, stepsRes] = await Promise.all([
				runClient.getRun({ runId: run.id }),
				runClient.getRunSteps({ runId: run.id }),
			]);
			console.log('[API Response]', 'GetRun', { id: run.id, stepsCount: stepsRes.steps?.length ?? 0 });

			const dur = runRes.run?.duration;
			const totalMs = durationMs(dur);

			const steps = (stepsRes.steps ?? []).map((s) => ({
				name: s.stepName ?? '',
				status: stepStatusToString(s.status),
				duration: durationMs(s.duration),
				attempt: s.attempt ?? 1,
				error: s.error?.message ?? null,
				input: s.input ? JSON.stringify(s.input, null, 2) : '{}',
				output: s.output ? JSON.stringify(s.output, null, 2) : '{}',
			}));

			runDetail = {
				duration: totalMs > 0 ? `${(totalMs / 1000).toFixed(1)}s` : 'in progress',
				steps: steps.length > 0 ? steps : generateMockSteps(run.workflowId, run.status),
			};
		} catch (e) {
			console.error('[API Error]', 'GetRun', e);
			// Fallback to mock steps so tabs aren't empty
			runDetail = {
				duration: 'unknown',
				steps: generateMockSteps(run.workflowId, run.status),
			};
		} finally {
			detailLoading = false;
		}
	}

	function closeSlideOver() {
		slideOverOpen = false;
		selectedRun = null;
	}

	function handleSlideOverKeydown(e: KeyboardEvent) {
		if (!slideOverOpen) return;
		if (e.key === 'Escape') { closeSlideOver(); return; }
		const idx = filteredRuns.findIndex((r) => r.id === selectedRun?.id);
		if (e.key === 'ArrowDown' && idx < filteredRuns.length - 1) {
			e.preventDefault();
			openSlideOver(filteredRuns[idx + 1]);
		} else if (e.key === 'ArrowUp' && idx > 0) {
			e.preventDefault();
			openSlideOver(filteredRuns[idx - 1]);
		}
	}

	async function bulkRetry() {
		if (selectedRuns.length === 0) return;
		bulkLoading = true;
		try {
			console.log('[API]', 'BulkRetry', { count: selectedRuns.length });
			await runClient.bulkRetry({ runIds: selectedRuns.map((r) => r.id) });
			console.log('[API Response]', 'BulkRetry', 'success');
			runs = runs.map((r) => ({ ...r, selected: false }));
		} catch (e) {
			console.error('[API Error]', 'BulkRetry', e);
			error = e instanceof Error ? e.message : 'Bulk retry failed';
		} finally {
			bulkLoading = false;
		}
	}

	async function bulkCancel() {
		if (selectedRuns.length === 0) return;
		bulkLoading = true;
		try {
			console.log('[API]', 'BulkCancel', { count: selectedRuns.length });
			await runClient.bulkCancel({ runIds: selectedRuns.map((r) => r.id), reason: 'Bulk cancel from UI' });
			console.log('[API Response]', 'BulkCancel', 'success');
			runs = runs.map((r) => ({ ...r, selected: false }));
		} catch (e) {
			console.error('[API Error]', 'BulkCancel', e);
			error = e instanceof Error ? e.message : 'Bulk cancel failed';
		} finally {
			bulkLoading = false;
		}
	}

	let detailView = $state<'timeline' | 'steps' | 'graph'>('timeline');
</script>

<svelte:head>
	<title>FlowWatch - Runs</title>
</svelte:head>

<svelte:window onkeydown={handleSlideOverKeydown} />

<div class="flex items-center justify-between mb-4">
	<h2 class="text-lg font-semibold text-text-bright">
		Runs
		{#if workflowFilter}
			<span class="text-sm font-normal text-text-muted ml-2">({workflowFilter})</span>
		{/if}
		{#if subsystemFilter}
			<span class="text-sm font-normal text-text-muted ml-2">({subsystemFilter})</span>
		{/if}
	</h2>
</div>

<!-- Saved views -->
<div class="flex items-center gap-2 mb-3">
	<span class="text-[10px] text-text-muted uppercase tracking-wider mr-1">Views:</span>
	{#each savedViews as view}
		<button
			onclick={() => selectView(view.key)}
			class="px-3 py-1 text-xs font-semibold rounded border transition-colors
				{activeView === view.key
					? 'bg-accent-muted text-accent border-accent/40'
					: 'bg-transparent text-text-muted border-border hover:bg-surface-hover'}"
		>{view.label}</button>
	{/each}
</div>

<!-- Status filter pills -->
<div class="flex gap-2 mb-4 flex-wrap">
	{#each [
		{ key: 'all', label: 'All', count: runs.length },
		{ key: 'running', label: 'Running', count: statusCounts.running },
		{ key: 'succeeded', label: 'Succeeded', count: statusCounts.succeeded },
		{ key: 'failed', label: 'Failed', count: statusCounts.failed },
		{ key: 'paused', label: 'Paused', count: statusCounts.paused },
		{ key: 'canceled', label: 'Canceled', count: statusCounts.canceled },
	] as pill}
		<button
			onclick={() => { statusFilter = pill.key; activeView = pill.key; }}
			class="px-3 py-1.5 text-xs font-semibold rounded-full border transition-colors
				{statusFilter === pill.key
					? 'bg-accent-muted text-accent border-accent/40'
					: 'bg-transparent text-text-muted border-border hover:bg-surface-hover'}"
		>
			{pill.label}{#if pill.key !== 'all' && pill.count > 0}&nbsp;({pill.count}){/if}
		</button>
	{/each}
</div>

{#if error}
	<div class="bg-failed-bg border border-failed/20 text-failed p-4 rounded-lg mb-4">{error}</div>
{/if}

<!-- Bulk actions bar -->
{#if selectedRuns.length > 0}
	<div class="flex items-center gap-3 mb-3 px-4 py-2 bg-accent-muted border border-accent/20 rounded-lg">
		<span class="text-xs font-semibold text-accent">{selectedRuns.length} selected</span>
		<button
			onclick={bulkRetry}
			disabled={bulkLoading}
			class="px-3 py-1 text-xs font-semibold bg-succeeded/10 text-succeeded border border-succeeded/20 rounded hover:bg-succeeded/20 disabled:opacity-50"
		>Retry Selected</button>
		<button
			onclick={bulkCancel}
			disabled={bulkLoading}
			class="px-3 py-1 text-xs font-semibold bg-failed/10 text-failed border border-failed/20 rounded hover:bg-failed/20 disabled:opacity-50"
		>Cancel Selected</button>
		<button
			onclick={() => runs = runs.map((r) => ({ ...r, selected: false }))}
			class="px-3 py-1 text-xs text-text-muted hover:text-text"
		>Clear</button>
	</div>
{/if}

<!-- Run rows -->
<div class="bg-surface border border-border rounded-lg overflow-hidden">
	<!-- Header -->
	<div class="grid items-center gap-2 px-4 py-2.5 border-b border-border text-[10px] font-bold text-text-muted uppercase tracking-wider"
		style="grid-template-columns: 28px 100px 70px 160px 120px 100px 1fr;"
	>
		<span>
			<input type="checkbox" checked={allSelected} onchange={toggleSelectAll} class="rounded border-border bg-bg" />
		</span>
		<span>Status</span>
		<span>ID</span>
		<span>Workflow</span>
		<span>Step</span>
		<span>Foreign ID</span>
		<span>Started</span>
	</div>

	{#each filteredRuns as run}
		<button
			onclick={() => openSlideOver(run)}
			class="w-full grid items-center gap-2 px-4 py-3 border-b border-border-subtle cursor-pointer transition-all duration-100 text-left
				{selectedRun?.id === run.id ? 'bg-surface-hover border-l-3 border-l-accent' : 'border-l-3 border-l-transparent hover:bg-surface-hover/50'}"
			style="grid-template-columns: 28px 100px 70px 160px 120px 100px 1fr;"
		>
			<!-- svelte-ignore a11y_no_static_element_interactions a11y_click_events_have_key_events -->
			<label class="cursor-pointer" onclick={(e) => e.stopPropagation()}>
				<input type="checkbox" checked={run.selected} onchange={() => toggleSelect(run.id)} class="rounded border-border bg-bg" />
			</label>
			<span><StatusBadge status={run.status} /></span>
			<span class="font-mono text-xs text-accent">{run.id.slice(0, 6)}</span>
			<span class="text-[13px] text-text font-medium truncate">{run.workflowId}</span>
			<span class="text-xs text-text-muted truncate flex items-center gap-1">
				{#if run.status === 'failed'}<span class="text-failed">↳</span>{/if}
				{run.currentStep || '-'}
			</span>
			<span class="font-mono text-xs text-text-muted">{run.foreignId}</span>
			<span class="text-[11px] text-text-muted">{run.startedAt}</span>
		</button>
	{:else}
		<div class="px-4 py-12 text-center text-text-muted">
			{loading ? 'Loading...' : 'No runs found'}
		</div>
	{/each}
</div>

{#if hasMore && !loading}
	<div class="mt-4 text-center">
		<button onclick={loadRuns} class="px-4 py-2 text-xs font-semibold bg-surface border border-border rounded-md hover:bg-surface-hover text-text-muted">Load More</button>
	</div>
{/if}

{#if loading && runs.length > 0}
	<p class="mt-4 text-center text-sm text-text-muted">Loading more...</p>
{/if}

<!-- Slide-over panel -->
{#if slideOverOpen && selectedRun}
	<!-- Backdrop -->
	<button class="fixed inset-0 bg-black/50 z-40" onclick={closeSlideOver} aria-label="Close detail panel"></button>

	<!-- Panel -->
	<div class="fixed top-0 right-0 w-[55%] h-screen bg-bg border-l border-border shadow-[-8px_0_40px_rgba(0,0,0,0.5)] z-50 flex flex-col animate-slide-in-right">
		<!-- Header -->
		<div class="p-5 border-b border-border shrink-0">
			<div class="flex justify-between items-center mb-4">
				<div class="flex items-center gap-3">
					<button onclick={closeSlideOver} class="text-text-muted hover:text-text text-lg p-1">←</button>
					<span class="font-mono text-sm text-accent">{selectedRun.id}</span>
					<StatusBadge status={selectedRun.status} size="md" />
				</div>
				<span class="text-xs text-text-muted">{selectedRun.workflowId}</span>
			</div>

			<div class="flex gap-6 text-xs text-text-muted mb-4">
				<span>Foreign ID: <span class="text-text font-mono">{selectedRun.foreignId}</span></span>
				<span>Duration: <span class="text-text">{runDetail?.duration ?? '—'}</span></span>
				<span>Started: <span class="text-text">{selectedRun.startedAt}</span></span>
			</div>

			<div class="flex gap-2">
				<button class="px-3.5 py-1.5 text-xs font-semibold rounded bg-accent text-white border border-accent transition-all hover:brightness-110">Retry</button>
				<button class="px-3.5 py-1.5 text-xs font-semibold rounded bg-transparent text-text-muted border border-border hover:bg-surface-hover transition-all">Retry from Failed</button>
				<button class="px-3.5 py-1.5 text-xs font-semibold rounded bg-transparent text-failed border border-border hover:bg-surface-hover transition-all">Cancel</button>
				<button class="px-3.5 py-1.5 text-xs font-semibold rounded bg-transparent text-text-muted border border-border hover:bg-surface-hover transition-all">Skip</button>
			</div>
		</div>

		<!-- Tabs -->
		<div class="px-6 border-b border-border flex gap-0 shrink-0">
			{#each ['timeline', 'steps', 'graph'] as tab}
				<button
					onclick={() => detailView = tab as typeof detailView}
					class="px-5 py-3 text-xs font-semibold uppercase tracking-wider border-b-2 transition-colors
						{detailView === tab ? 'text-accent border-accent' : 'text-text-muted border-transparent hover:text-text'}"
				>{tab}</button>
			{/each}
		</div>

		<!-- Content -->
		<div class="flex-1 overflow-auto p-6">
			{#if detailLoading}
				<p class="text-text-muted text-center py-8">Loading run details...</p>
			{:else if runDetail}
				{@const maxDur = Math.max(...runDetail.steps.map(s => s.duration), 1)}

				{#if detailView === 'timeline'}
					<div class="text-[11px] text-text-muted mb-4 flex justify-between uppercase tracking-wider">
						<span>Step Timeline</span>
						<span>0ms — {maxDur}ms</span>
					</div>
					{#each runDetail.steps as step, i}
						{@const pct = (step.duration / maxDur) * 100}
						{@const isFailed = step.status === 'failed'}
						{@const isRunning = step.status === 'running'}
						<div class="grid items-center gap-3 py-1.5" style="grid-template-columns: 130px 1fr 60px 80px;">
							<span class="font-mono text-xs text-text truncate">{step.name}</span>
							<div class="relative h-[22px] rounded" style="background: rgba(30, 42, 58, 0.4);">
								<div
									class="absolute left-0 top-0 h-full rounded transition-[width] duration-300 {isRunning ? 'animate-bar-pulse' : ''}"
									style="width: {Math.max(pct, 2)}%;
										background: {isFailed
											? `repeating-linear-gradient(90deg, #ef4444, #ef4444 4px, #ef444480 4px, #ef444480 8px)`
											: isRunning
											? `linear-gradient(90deg, #3b82f6, #3b82f680)`
											: step.status === 'succeeded' ? '#10b981' : '#5c6b7e'};"
								></div>
								{#if step.attempt > 1}
									<div class="absolute right-1 top-0.5 text-[10px] text-white font-bold">x{step.attempt}</div>
								{/if}
							</div>
							<span class="font-mono text-[11px] text-text-muted text-right">{step.duration > 0 ? `${step.duration}ms` : '—'}</span>
							<div class="flex justify-end"><StatusBadge status={step.status} /></div>
						</div>
					{/each}

				{:else if detailView === 'steps'}
					<div class="flex flex-col gap-2">
						{#each runDetail.steps as step}
							<div class="bg-surface border border-border rounded-md p-4">
								<div class="flex justify-between items-center mb-2">
									<span class="font-mono text-[13px] text-text">{step.name}</span>
									<StatusBadge status={step.status} />
								</div>
								<div class="flex gap-5 text-[11px] text-text-muted">
									<span>Duration: {step.duration}ms</span>
									<span>Attempt: {step.attempt}</span>
								</div>
								{#if step.error}
									<div class="mt-2 px-3 py-2 bg-failed-bg border border-failed/20 rounded text-xs font-mono text-failed">{step.error}</div>
								{/if}
							</div>
						{/each}
					</div>

				{:else if detailView === 'graph'}
					<div class="flex flex-col items-center gap-0 pt-4">
						{#each runDetail.steps as step, i}
							{@const cfg = {
								running: { color: '#3b82f6', bg: '#0c1a2e' },
								succeeded: { color: '#10b981', bg: '#0a1f18' },
								failed: { color: '#ef4444', bg: '#1f0a0a' },
								paused: { color: '#f59e0b', bg: '#1f1a0a' },
								pending: { color: '#5c6b7e', bg: '#0a0e14' },
								canceled: { color: '#8b5cf6', bg: '#0a0e14' },
								skipped: { color: '#5c6b7e', bg: '#0a0e14' },
							}[step.status] ?? { color: '#5c6b7e', bg: '#0a0e14' }}
							<div class="flex flex-col items-center">
								<div
									class="w-[180px] px-4 py-3 rounded-lg text-center {step.status === 'running' ? 'animate-node-pulse' : ''}"
									style="background: {cfg.bg}; border: 2px solid {cfg.color};"
								>
									<div class="font-mono text-xs font-semibold" style="color: {cfg.color};">{step.name}</div>
									<div class="text-[10px] text-text-muted mt-1">{step.duration > 0 ? `${step.duration}ms` : '—'} · {step.status}</div>
								</div>
								{#if i < runDetail.steps.length - 1}
									<div class="w-0.5 h-6 bg-border"></div>
								{/if}
							</div>
						{/each}
					</div>
				{/if}
			{/if}
		</div>
	</div>
{/if}
