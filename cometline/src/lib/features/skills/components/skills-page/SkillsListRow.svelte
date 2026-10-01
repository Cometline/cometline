<script lang="ts">
	let {
		name,
		description,
		active,
		badges = [],
		onclick
	}: {
		name: string;
		description: string;
		active: boolean;
		badges?: string[];
		onclick: () => void;
	} = $props();
</script>

<button type="button" class="item-row" class:active {onclick}>
	<strong>
		{name}
		{#each badges as badge (badge)}
			<span class="skill-badge">{badge}</span>
		{/each}
	</strong>
	<p>{description}</p>
</button>

<style>
	.item-row {
		width: 100%;
		text-align: left;
		padding: 10px 12px;
		border: none;
		border-bottom: 1px solid rgba(0, 0, 0, 0.06);
		background: transparent;
		cursor: pointer;
	}

	.item-row:last-child {
		border-bottom: 0;
	}

	.item-row.active {
		background: rgba(0, 102, 204, 0.08);
	}

	.item-row strong {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
		align-items: center;
		font-size: 12px;
		color: var(--text-main);
	}

	.item-row p {
		margin: 4px 0 0;
		font-size: 11px;
		line-height: 1.45;
		color: var(--text-muted);
	}

	.skill-badge {
		font-size: 10px;
		font-weight: 700;
		padding: 1px 6px;
		border-radius: 999px;
		background: rgba(15, 23, 42, 0.08);
		color: var(--text-muted);
	}
</style>
