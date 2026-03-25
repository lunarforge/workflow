<script lang="ts">
	import '../app.css';
	import { page } from '$app/state';
	import SearchPalette from '$lib/components/SearchPalette.svelte';
	import { workflowClient } from '$lib/api/client';
	import { onMount } from 'svelte';
	import { density, subsystemScope } from '$lib/stores/ui';
	import { subsystems } from '$lib/stores/subsystems';

	let { children } = $props();
	let collapsed = $state(false);

	const navItems = [
		{ href: '/', label: 'Dashboard', icon: '◫' },
		{ href: '/subsystems', label: 'Subsystems', icon: '⬡' },
		{ href: '/workflows', label: 'Workflows', icon: '◇' },
		{ href: '/runs', label: 'Runs', icon: '▤' },
		{ href: '/traces', label: 'Traces', icon: '⑆' },
		{ href: '/analytics', label: 'Analytics', icon: '◈' },
		{ href: '/settings', label: 'Settings', icon: '⚙' },
	];

	const currentPath = $derived(page.url.pathname);

	const breadcrumbs = $derived.by(() => {
		const parts = currentPath.split('/').filter(Boolean);
		const crumbs: { label: string; href: string }[] = [{ label: 'FlowWatch', href: '/' }];
		let path = '';
		for (const p of parts) {
			path += '/' + p;
			const nav = navItems.find((n) => n.href === path);
			crumbs.push({ label: nav?.label ?? p, href: path });
		}
		return crumbs;
	});

	onMount(async () => {
		try {
			console.log('[API]', 'ListSubsystems', 'loading for layout');
			const res = await workflowClient.listSubsystems({});
			const mapped = (res.subsystems ?? []).map((s) => ({
				id: s.subsystem?.id ?? '',
				name: s.subsystem?.name ?? s.subsystem?.id ?? '',
			}));
			subsystems.set(mapped);
			console.log('[API Response]', 'ListSubsystems', { count: mapped.length });
		} catch (e) {
			console.error('[API Error]', 'ListSubsystems', e);
		}
	});

	function toggleDensity() {
		density.update((d) => d === 'comfortable' ? 'compact' : 'comfortable');
	}
</script>

<svelte:window onkeydown={(e) => {
	if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
		e.preventDefault();
		document.dispatchEvent(new CustomEvent('open-search'));
	}
}} />

<div class="flex h-screen bg-bg text-text {$density === 'compact' ? 'text-sm' : ''}">
	<!-- Sidebar -->
	<nav class="shrink-0 border-r border-border bg-surface flex flex-col transition-all duration-200 {collapsed ? 'w-14' : 'w-52'}">
		<button
			onclick={() => collapsed = !collapsed}
			class="p-4 border-b border-border flex items-center gap-2.5 hover:bg-surface-hover transition-colors text-left"
		>
			<span class="text-lg text-accent shrink-0">⬡</span>
			{#if !collapsed}
				<div>
					<h1 class="text-sm font-semibold tracking-tight text-text-bright">FlowWatch</h1>
					<p class="text-[10px] text-text-muted">Workflow Monitor</p>
				</div>
			{/if}
		</button>

		<!-- Search trigger -->
		<div class="px-2 pt-2">
			<button
				onclick={() => document.dispatchEvent(new CustomEvent('open-search'))}
				class="w-full flex items-center gap-2.5 px-2.5 py-2 rounded-md text-sm text-text-muted border border-dashed border-border hover:border-accent hover:text-accent transition-colors"
			>
				<span class="text-sm shrink-0">⌕</span>
				{#if !collapsed}
					<span class="flex-1 text-left text-xs">Search</span>
					<kbd class="text-[10px] px-1.5 py-0.5 bg-bg border border-border rounded font-mono text-text-muted">⌘K</kbd>
				{/if}
			</button>
		</div>

		<ul class="flex-1 p-2 space-y-0.5">
			{#each navItems as item}
				<li>
					<a
						href={item.href}
						class="flex items-center gap-2.5 px-2.5 py-2 rounded-md text-sm transition-colors
							{currentPath === item.href || (item.href !== '/' && currentPath.startsWith(item.href))
								? 'bg-accent-muted text-accent font-medium'
								: 'text-text-muted hover:bg-surface-hover hover:text-text'}"
						title={collapsed ? item.label : undefined}
					>
						<span class="text-base shrink-0 w-5 text-center">{item.icon}</span>
						{#if !collapsed}
							{item.label}
						{/if}
					</a>
				</li>
			{/each}
		</ul>

		<!-- Density toggle -->
		{#if !collapsed}
			<div class="p-2 border-t border-border">
				<button
					onclick={toggleDensity}
					class="w-full flex items-center gap-2 px-2.5 py-1.5 rounded-md text-xs text-text-muted hover:bg-surface-hover transition-colors"
				>
					<span class="shrink-0 w-5 text-center">{$density === 'compact' ? '▤' : '▥'}</span>
					{$density === 'compact' ? 'Compact' : 'Comfortable'}
				</button>
			</div>
		{/if}
	</nav>

	<!-- Main content -->
	<div class="flex-1 flex flex-col overflow-hidden">
		<!-- Top bar -->
		<header class="flex items-center justify-between px-6 py-2 border-b border-border bg-surface shrink-0">
			<nav class="flex items-center gap-1 text-xs text-text-muted">
				{#each breadcrumbs as crumb, i}
					{#if i > 0}
						<span class="mx-1 text-border">/</span>
					{/if}
					{#if i === breadcrumbs.length - 1}
						<span class="text-text-bright font-medium">{crumb.label}</span>
					{:else}
						<a href={crumb.href} class="hover:text-accent">{crumb.label}</a>
					{/if}
				{/each}
			</nav>
			<div class="flex items-center gap-3">
				{#if $subsystems.length > 0}
					<select
						bind:value={$subsystemScope}
						class="text-xs bg-surface border border-border rounded px-2 py-1 text-text-muted"
					>
						<option value="">All Subsystems</option>
						{#each $subsystems as ss}
							<option value={ss.id}>{ss.name}</option>
						{/each}
					</select>
				{/if}
			</div>
		</header>

		<main class="flex-1 overflow-auto">
			<div class="p-6 max-w-7xl mx-auto">
				{@render children()}
			</div>
		</main>
	</div>
</div>

<SearchPalette />
