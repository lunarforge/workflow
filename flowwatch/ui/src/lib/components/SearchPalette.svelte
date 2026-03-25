<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { runClient } from '$lib/api/client';

	let open = $state(false);
	let query = $state('');
	let selectedIdx = $state(0);
	let results = $state<{ type: string; id: string; label: string; sublabel: string; status: string; href: string }[]>([]);
	let searching = $state(false);
	let inputEl: HTMLInputElement | undefined = $state();

	const recentSearches = ['status:failed', 'workflow:order-processing', 'timeout'];
	const queryFields = ['status:', 'workflow:', 'step:', 'error:', 'foreign_id:', 'started:', 'duration:'];

	onMount(() => {
		const handler = () => { open = true; };
		document.addEventListener('open-search', handler);
		return () => document.removeEventListener('open-search', handler);
	});

	$effect(() => {
		if (open && inputEl) {
			setTimeout(() => inputEl?.focus(), 50);
		}
	});

	function statusToString(status: number): string {
		const map: Record<number, string> = {
			0: 'unknown', 1: 'pending', 2: 'running', 3: 'succeeded',
			4: 'failed', 5: 'canceled', 6: 'paused',
		};
		return map[status] ?? 'unknown';
	}

	let debounceTimer: ReturnType<typeof setTimeout>;
	function handleInput() {
		clearTimeout(debounceTimer);
		selectedIdx = 0;
		if (query.length < 2) {
			results = [];
			return;
		}
		debounceTimer = setTimeout(search, 300);
	}

	async function search() {
		if (query.length < 2) return;
		searching = true;
		try {
			console.log('[API]', 'Search', { query });
			const filter: Record<string, unknown> = {};
			if (query.startsWith('workflow:')) {
				filter.workflowIds = [query.replace('workflow:', '')];
			} else if (query.startsWith('foreign_id:')) {
				filter.foreignId = query.replace('foreign_id:', '');
			} else {
				filter.foreignId = query;
			}

			const res = await runClient.listRuns({
				filter,
				pagination: { limit: 10, cursor: '' },
			});
			console.log('[API Response]', 'Search', { count: res.runs?.length ?? 0 });

			results = (res.runs ?? []).map((r) => ({
				type: 'run',
				id: r.id,
				label: r.workflowId,
				sublabel: r.foreignId || r.id.slice(0, 8),
				status: statusToString(r.status),
				href: `/runs/${r.id}`,
			}));
		} catch {
			results = [];
		} finally {
			searching = false;
		}
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			close();
		} else if (e.key === 'ArrowDown') {
			e.preventDefault();
			selectedIdx = Math.min(selectedIdx + 1, results.length - 1);
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			selectedIdx = Math.max(selectedIdx - 1, 0);
		} else if (e.key === 'Enter') {
			e.preventDefault();
			if (results[selectedIdx]) {
				navigate(results[selectedIdx].href);
			}
		}
	}

	function navigate(href: string) {
		close();
		goto(href);
	}

	function close() {
		open = false;
		query = '';
		results = [];
		selectedIdx = 0;
	}

	const statusColors: Record<string, string> = {
		succeeded: 'text-succeeded bg-succeeded-bg border-succeeded/20',
		failed: 'text-failed bg-failed-bg border-failed/20',
		running: 'text-running bg-running-bg border-running/20',
		paused: 'text-paused bg-paused-bg border-paused/20',
	};
</script>

{#if open}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="fixed inset-0 z-50 flex justify-center pt-20 bg-black/60 backdrop-blur-sm"
		onkeydown={handleKeydown}
		onclick={(e) => { if (e.target === e.currentTarget) close(); }}
	>
		<div class="relative w-[560px] max-h-[70vh] bg-surface border border-border rounded-xl shadow-2xl overflow-hidden flex flex-col">
			<!-- Input -->
			<div class="flex items-center gap-3 px-4 py-3 border-b border-border">
				<span class="text-text-muted text-sm">⌕</span>
				<input
					bind:this={inputEl}
					bind:value={query}
					oninput={handleInput}
					onkeydown={handleKeydown}
					placeholder="Search runs, workflows, foreign IDs..."
					class="flex-1 bg-transparent border-none outline-none text-sm text-text placeholder-text-muted"
				/>
				{#if searching}
					<span class="text-xs text-text-muted animate-pulse">Searching...</span>
				{/if}
				<kbd class="text-[10px] px-1.5 py-0.5 bg-bg border border-border rounded font-mono text-text-muted">esc</kbd>
			</div>

			<!-- Results -->
			<div class="flex-1 overflow-auto py-2">
				{#if query.length < 2}
					<div class="px-4 py-2">
						<p class="text-[10px] font-bold text-text-muted uppercase tracking-wider mb-2">Recent Searches</p>
						{#each recentSearches as s}
							<button
								onclick={() => { query = s; handleInput(); }}
								class="w-full text-left flex items-center gap-2.5 px-3 py-1.5 rounded text-xs font-mono text-text-muted hover:bg-surface-hover"
							>
								<span class="text-text-muted">↻</span> {s}
							</button>
						{/each}
					</div>
					<div class="px-4 py-2 border-t border-border-subtle">
						<p class="text-[10px] font-bold text-text-muted uppercase tracking-wider mb-2">Query Syntax</p>
						<div class="flex flex-wrap gap-1.5">
							{#each queryFields as f}
								<button
									onclick={() => { query = f; inputEl?.focus(); }}
									class="px-2 py-0.5 text-[11px] font-mono bg-bg border border-border rounded text-accent hover:bg-surface-hover"
								>{f}</button>
							{/each}
						</div>
					</div>
				{:else if results.length === 0 && !searching}
					<div class="px-4 py-6 text-center text-sm text-text-muted">No results for "{query}"</div>
				{:else}
					<p class="px-4 text-[10px] font-bold text-text-muted uppercase tracking-wider mb-1">Runs ({results.length})</p>
					{#each results as result, i}
						<button
							onclick={() => navigate(result.href)}
							class="w-full text-left flex items-center gap-3 px-4 py-2 text-xs transition-colors
								{i === selectedIdx ? 'bg-accent-muted border-l-2 border-l-accent' : 'border-l-2 border-l-transparent hover:bg-surface-hover'}"
						>
							<span class="font-mono text-accent w-14 truncate">{result.id.slice(0, 8)}</span>
							<span class="text-text font-medium truncate">{result.label}</span>
							<span class="font-mono text-text-muted truncate">{result.sublabel}</span>
							<span class="ml-auto text-[10px] px-2 py-0.5 rounded-full border {statusColors[result.status] ?? 'text-text-muted bg-bg border-border'}">{result.status}</span>
						</button>
					{/each}
				{/if}
			</div>

			<!-- Footer -->
			<div class="px-4 py-2 border-t border-border flex gap-4 text-[10px] text-text-muted">
				<span><kbd class="px-1 bg-bg border border-border rounded font-mono">↵</kbd> Open</span>
				<span><kbd class="px-1 bg-bg border border-border rounded font-mono">↑↓</kbd> Navigate</span>
				<span><kbd class="px-1 bg-bg border border-border rounded font-mono">esc</kbd> Close</span>
				{#if results.length > 0}
					<span class="ml-auto">{results.length} results</span>
				{/if}
			</div>
		</div>
	</div>
{/if}
