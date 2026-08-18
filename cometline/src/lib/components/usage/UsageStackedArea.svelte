<script lang="ts">
	import { formatTokens } from '$lib/usage/format';
	import { nearestPointIndex, stackedAreaPaths, xLabels, yLabels, type SeriesPoint } from '$lib/usage/chart';

	let {
		points = [],
		keys = []
	}: {
		points?: SeriesPoint[];
		keys?: string[];
	} = $props();

	const height = 220;
	let width = $state(640);
	let hoverIndex = $state(-1);

	const paths = $derived(stackedAreaPaths(points, keys, width, height));
	const xs = $derived(xLabels(points, width));
	const ys = $derived(yLabels(points, keys, height));
	const hover = $derived(hoverIndex >= 0 ? points[hoverIndex] : undefined);
	const summaryLabel = $derived(
		hover
			? `${hover.date}: ${keys.map((key) => `${key} ${formatTokens(hover.cumulative[key] ?? 0)}`).join(', ')}`
			: 'Cumulative token usage. Use arrow keys to inspect each day.'
	);

	function observeSize(node: HTMLDivElement) {
		if (typeof ResizeObserver === 'undefined') return;
		const observer = new ResizeObserver((entries) => {
			const box = entries[0]?.contentRect;
			if (!box) return;
			width = Math.max(240, box.width);
		});
		observer.observe(node);
		return () => observer.disconnect();
	}

	function onMove(event: MouseEvent) {
		const target = event.currentTarget;
		if (!(target instanceof HTMLElement)) return;
		const rect = target.getBoundingClientRect();
		hoverIndex = nearestPointIndex(points, width, event.clientX - rect.left);
	}

	function onKey(event: KeyboardEvent) {
		if (!points.length) return;
		if (event.key === 'ArrowRight' || event.key === 'ArrowLeft') {
			event.preventDefault();
			const delta = event.key === 'ArrowRight' ? 1 : -1;
			const current = hoverIndex < 0 ? (delta > 0 ? -1 : points.length) : hoverIndex;
			hoverIndex = Math.min(points.length - 1, Math.max(0, current + delta));
			return;
		}
		if (event.key === 'Home') {
			event.preventDefault();
			hoverIndex = 0;
			return;
		}
		if (event.key === 'End') {
			event.preventDefault();
			hoverIndex = points.length - 1;
			return;
		}
		if (event.key === 'Escape') {
			hoverIndex = -1;
		}
	}
</script>

<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div
	class="usage-chart"
	{@attach observeSize}
	tabindex="0"
	role="application"
	aria-label={summaryLabel}
	onmousemove={onMove}
	onmouseleave={() => (hoverIndex = -1)}
	onkeydown={onKey}
>
	<svg viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none" aria-hidden="true">
		{#each ys as tick (tick.y)}
			<line class="grid" x1="36" x2={width - 8} y1={tick.y} y2={tick.y} />
			<text class="axis" x="4" y={tick.y + 3}>{formatTokens(tick.label)}</text>
		{/each}
		{#each paths as path, index (path.key)}
			<path class={`area series-${index % 6}`} d={path.d} />
		{/each}
		{#each xs as tick (tick.label + tick.x)}
			<text class="axis x" x={tick.x} y={height - 6}>{tick.label}</text>
		{/each}
	</svg>
	<div class="sr-only" aria-live="polite">{summaryLabel}</div>
	{#if hover}
		<div class="hover">
			<strong>{hover.date}</strong>
			{#each keys as key, index (key)}
				<span class={`swatch series-${index % 6}`}></span>
				<span>{key}: {formatTokens(hover.cumulative[key] ?? 0)}</span>
			{/each}
		</div>
	{/if}
	<div class="sr-only">
		<table>
			<caption>Cumulative token usage by day</caption>
			<thead>
				<tr>
					<th>Date</th>
					{#each keys as key (key)}
						<th>{key}</th>
					{/each}
				</tr>
			</thead>
			<tbody>
				{#each points as point (point.date)}
					<tr>
						<td>{point.date}</td>
						{#each keys as key (point.date + key)}
							<td>{point.cumulative[key] ?? 0}</td>
						{/each}
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
</div>

<style>
	.usage-chart {
		position: relative;
		width: 100%;
		height: 220px;
		overflow: hidden;
	}

	.usage-chart:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: 2px;
		border-radius: 10px;
	}

	svg {
		width: 100%;
		height: 100%;
		display: block;
	}

	.grid {
		stroke: var(--border-soft);
		stroke-width: 1;
	}

	.axis {
		fill: var(--text-muted);
		font-size: 10px;
	}

	.axis.x {
		text-anchor: middle;
	}

	.area {
		opacity: 0.72;
	}

	.area.series-0 {
		fill: var(--status-success);
	}
	.area.series-1 {
		fill: var(--intro-blue);
	}
	.area.series-2 {
		fill: var(--accent);
	}
	.area.series-3 {
		fill: var(--status-warning);
	}
	.area.series-4 {
		fill: var(--text-soft);
	}
	.area.series-5 {
		fill: var(--status-error);
	}

	.swatch.series-0 {
		background: var(--status-success);
	}
	.swatch.series-1 {
		background: var(--intro-blue);
	}
	.swatch.series-2 {
		background: var(--accent);
	}
	.swatch.series-3 {
		background: var(--status-warning);
	}
	.swatch.series-4 {
		background: var(--text-soft);
	}
	.swatch.series-5 {
		background: var(--status-error);
	}

	.hover {
		position: absolute;
		top: 8px;
		right: 8px;
		display: flex;
		flex-wrap: wrap;
		gap: 6px 10px;
		align-items: center;
		padding: 8px 10px;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		background: var(--panel-bg);
		color: var(--text-main);
		font-size: 11px;
		pointer-events: none;
	}

	.swatch {
		width: 8px;
		height: 8px;
		border-radius: 99px;
	}

	.sr-only {
		position: absolute;
		width: 1px;
		height: 1px;
		padding: 0;
		margin: -1px;
		overflow: hidden;
		clip-path: inset(50%);
		white-space: nowrap;
		border: 0;
	}
</style>
