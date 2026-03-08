<script lang="ts">
	let { label, value, accentColor = 'border-accent' }: {
		label: string;
		value: string | number;
		accentColor?: string;
	} = $props();
</script>

<div class="bg-surface border border-border rounded-lg p-4 pb-3.5 border-b-3 {accentColor} cursor-pointer transition-all duration-150 hover:bg-surface-hover hover:-translate-y-px text-left">
	<div class="text-[11px] text-text-muted uppercase tracking-[1.2px] font-semibold mb-2">{label}</div>
	<span class="text-[32px] font-bold text-text-bright font-mono leading-none">{value}</span>
</div>
