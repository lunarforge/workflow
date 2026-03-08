<script lang="ts">
	let { status, size = 'sm' }: { status: string; size?: 'sm' | 'md' } = $props();

	const config: Record<string, { icon: string; label: string; color: string; bg: string; border: string }> = {
		running: { icon: '◉', label: 'Running', color: 'text-running', bg: 'bg-running-bg', border: 'border-running/20' },
		succeeded: { icon: '✓', label: 'Succeeded', color: 'text-succeeded', bg: 'bg-succeeded-bg', border: 'border-succeeded/20' },
		failed: { icon: '✗', label: 'Failed', color: 'text-failed', bg: 'bg-failed-bg', border: 'border-failed/20' },
		paused: { icon: '⏸', label: 'Paused', color: 'text-paused', bg: 'bg-paused-bg', border: 'border-paused/20' },
		pending: { icon: '○', label: 'Pending', color: 'text-pending', bg: 'bg-bg', border: 'border-pending/20' },
		canceled: { icon: '⊘', label: 'Canceled', color: 'text-canceled', bg: 'bg-bg', border: 'border-canceled/20' },
		skipped: { icon: '—', label: 'Skipped', color: 'text-text-muted', bg: 'bg-bg', border: 'border-text-muted/20' },
		unknown: { icon: '?', label: 'Unknown', color: 'text-text-muted', bg: 'bg-bg', border: 'border-text-muted/20' },
	};

	const cfg = $derived(config[status] ?? config.unknown);
	const sz = $derived(size === 'sm' ? 'px-1.5 py-0.5 text-[11px]' : 'px-2 py-1 text-[13px]');
</script>

<span class="inline-flex items-center gap-1 font-mono font-semibold uppercase tracking-wider rounded border {sz} {cfg.color} {cfg.bg} {cfg.border}">
	<span class={size === 'sm' ? 'text-[10px]' : 'text-[13px]'}>{cfg.icon}</span>
	{cfg.label}
</span>
