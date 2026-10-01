<script lang="ts">
	export type PersistenceTier = 'pending' | 'instant' | 'action';

	let {
		tier,
		detail
	}: {
		tier: PersistenceTier;
		detail?: string;
	} = $props();

	const tierLabel = $derived(
		tier === 'pending' ? 'Save changes' : tier === 'instant' ? 'Instant' : 'Action'
	);

	const tierDescription = $derived(
		tier === 'pending'
			? 'Saved when you click Save changes'
			: tier === 'instant'
				? 'Applies immediately'
				: (detail ?? 'Runs when you click the button')
	);
</script>

<p class="settings-persistence-hint" data-tier={tier}>
	<span class="settings-persistence-tier">{tierLabel}</span>
	<span class="settings-persistence-copy">
		{tierDescription}{#if detail && tier !== 'action'}
			— {detail}{/if}
	</span>
</p>
