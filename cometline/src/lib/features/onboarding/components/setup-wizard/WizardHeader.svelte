<script lang="ts">
	import { Sparkles, X } from '@lucide/svelte';

	let { subtitle, onClose }: { subtitle: string; onClose: () => void } = $props();
</script>

<header>
	<div class="title-mark"><Sparkles size={16} /></div>
	<div>
		<h2 id="wizard-title">Welcome to Cometline</h2>
		<p>{subtitle}</p>
	</div>
	<button class="close-btn" aria-label="Close" onclick={onClose}><X size={16} /></button>
</header>

<style>
	header {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 18px 20px;
		border-bottom: 1px solid var(--border-soft);
	}

	.title-mark {
		width: 32px;
		height: 32px;
		border-radius: 11px;
		background: rgba(0, 102, 204, 0.09);
		color: var(--accent);
		display: grid;
		place-items: center;
		flex-shrink: 0;
	}

	header h2 {
		margin: 0;
		font-size: 17px;
		font-weight: 600;
		color: var(--text-main);
	}

	header p {
		margin: 2px 0 0;
		font-size: 12px;
		color: var(--text-muted);
	}

	.close-btn {
		margin-left: auto;
		border: none;
		background: transparent;
		color: var(--text-muted);
		cursor: pointer;
		padding: 4px;
		border-radius: 6px;
		display: grid;
		place-items: center;
	}

	.close-btn:hover {
		background: rgba(15, 23, 42, 0.05);
		color: var(--text-main);
	}
</style>
