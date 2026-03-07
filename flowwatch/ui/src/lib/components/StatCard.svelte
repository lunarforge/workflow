<script lang="ts">
	let { label, value, color = 'text-gray-900 dark:text-gray-100' }: {
		label: string;
		value: string | number;
		color?: string;
	} = $props();
</script>

<div class="bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-800 p-4">
	<p class="text-sm text-muted">{label}</p>
	<p class="text-2xl font-semibold mt-1 {color}">{value}</p>
</div>
