<script lang="ts">
</script>

<svelte:head>
	<title>FlowWatch - Settings</title>
</svelte:head>

<h2 class="text-lg font-semibold text-text-bright mb-4">Settings</h2>

<div class="space-y-6">
	<div class="bg-surface rounded-lg border border-border p-6">
		<h3 class="text-sm font-semibold text-text-bright mb-3">Subsystem Management</h3>
		<p class="text-sm text-text-muted">Configure subsystem definitions, labels, and alert thresholds.</p>
		<p class="mt-3 text-xs text-paused bg-paused-bg border border-paused/20 rounded px-3 py-2">Coming soon — subsystem management will be available in a future release.</p>
	</div>

	<div class="bg-surface rounded-lg border border-border p-6">
		<h3 class="text-sm font-semibold text-text-bright mb-3">Alert Rules</h3>
		<p class="text-sm text-text-muted">Define rules for failure spikes, latency regressions, and stuck runs.</p>
		<p class="mt-3 text-xs text-paused bg-paused-bg border border-paused/20 rounded px-3 py-2">Coming soon — alert configuration will be available in a future release.</p>
	</div>

	<div class="bg-surface rounded-lg border border-border p-6">
		<h3 class="text-sm font-semibold text-text-bright mb-3">Saved Views</h3>
		<p class="text-sm text-text-muted">Manage your saved filter presets for the Runs page.</p>
		<p class="mt-3 text-xs text-paused bg-paused-bg border border-paused/20 rounded px-3 py-2">Coming soon — saved views management will be available in a future release.</p>
	</div>

	<div class="bg-surface rounded-lg border border-border p-6">
		<h3 class="text-sm font-semibold text-text-bright mb-3">Engine Connection</h3>
		<p class="text-sm text-text-muted">View and configure the connected workflow engine.</p>
		<div class="mt-3 grid grid-cols-2 gap-3 text-xs">
			<div class="bg-bg border border-border rounded p-3">
				<span class="block text-text-muted mb-1">API URL</span>
				<span class="font-mono text-text">localhost:8090</span>
			</div>
			<div class="bg-bg border border-border rounded p-3">
				<span class="block text-text-muted mb-1">Engine</span>
				<span class="font-mono text-text">luno/workflow</span>
			</div>
		</div>
	</div>
</div>
