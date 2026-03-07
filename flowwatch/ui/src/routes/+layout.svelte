<script lang="ts">
	import '../app.css';

	let { children } = $props();

	const navItems = [
		{ href: '/', label: 'Dashboard' },
		{ href: '/workflows', label: 'Workflows' },
		{ href: '/runs', label: 'Runs' },
	];
</script>

<div class="flex h-screen bg-gray-50 dark:bg-gray-950 text-gray-900 dark:text-gray-100">
	<!-- Sidebar -->
	<nav class="w-56 shrink-0 border-r border-gray-200 dark:border-gray-800 bg-white dark:bg-gray-900 flex flex-col">
		<div class="p-4 border-b border-gray-200 dark:border-gray-800">
			<h1 class="text-lg font-semibold tracking-tight">FlowWatch</h1>
			<p class="text-xs text-muted">Workflow Monitor</p>
		</div>
		<ul class="flex-1 p-2 space-y-1">
			{#each navItems as item}
				<li>
					<a
						href={item.href}
						class="block px-3 py-2 rounded-md text-sm hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
					>
						{item.label}
					</a>
				</li>
			{/each}
		</ul>
	</nav>

	<!-- Main content -->
	<main class="flex-1 overflow-auto">
		<div class="p-6 max-w-7xl mx-auto">
			{@render children()}
		</div>
	</main>
</div>
