<script lang="ts">
	let { status }: { status: string } = $props();

	const colors: Record<string, string> = {
		running: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200',
		succeeded: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200',
		failed: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200',
		canceled: 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200',
		paused: 'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200',
		pending: 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400',
	};

	const colorClass = $derived(colors[status] ?? colors.pending);
</script>

<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium {colorClass}">
	{status}
</span>
