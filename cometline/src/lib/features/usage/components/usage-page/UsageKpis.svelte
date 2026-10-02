<script lang="ts">
	import type { UsageSummaryResponse } from '#lib/client/cometmind.js';
	import { formatTokens, formatUSD } from '#lib/features/usage/format.js';

	let {
		totals,
		cacheReadTotal,
		cacheHit
	}: {
		totals: UsageSummaryResponse['totals'];
		cacheReadTotal: number;
		cacheHit: string;
	} = $props();
</script>

<section class="kpis">
	<article>
		<strong>{formatTokens(totals.tokens)}</strong>
		<span>Total Tokens</span>
	</article>
	<article>
		<strong>{formatUSD(totals.estimated_usd)}</strong>
		<span>Estimated</span>
	</article>
	<article>
		<strong>{cacheReadTotal > 0 ? `${formatTokens(cacheReadTotal)} · ${cacheHit}` : '—'}</strong
		>
		<span>Cache read</span>
	</article>
	<article>
		<strong>{formatTokens(totals.unpriced_tokens)}</strong>
		<span>Unpriced</span>
	</article>
</section>

<style>
	.kpis {
		width: 100%;
		min-width: 0;
		box-sizing: border-box;
		display: grid;
		grid-template-columns: repeat(4, minmax(0, 1fr));
		gap: 12px;
	}

	.kpis article {
		min-width: 0;
		padding: 16px;
		border: 1px solid var(--border-soft);
		border-radius: 14px;
		background: var(--panel-bg);
	}

	.kpis strong {
		display: block;
		font-size: 28px;
		letter-spacing: -0.03em;
	}

	.kpis span {
		color: var(--text-muted);
		font-size: 12px;
	}

	@container main-pane (max-width: 760px) {
		.kpis {
			grid-template-columns: repeat(2, minmax(0, 1fr));
			gap: 8px;
		}

		.kpis article {
			padding: 12px;
		}

		.kpis strong {
			font-size: 22px;
		}
	}
</style>
