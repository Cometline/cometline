<script lang="ts">
	// Prose styles for rendered markdown. Global so they apply inside AssistantMarkdown.
</script>

<style>
	:global(.markdown blockquote) {
		position: relative;
		margin: 0 0 0.6em;
		padding: 0.45em 0.9em 0.45em 2.1em;
		border: 1px solid color-mix(in srgb, var(--border-soft) 72%, transparent);
		border-radius: 10px;
		background: color-mix(in srgb, var(--border-soft) 22%, transparent);
		color: var(--text-muted);
	}

	:global(.markdown blockquote::before) {
		content: '“';
		position: absolute;
		left: 0.65em;
		top: 0.2em;
		font-family: Georgia, serif;
		font-size: 1.35em;
		line-height: 1;
		color: color-mix(in srgb, var(--text-soft) 70%, transparent);
	}

	:global(.markdown hr) {
		border: none;
		border-top: 1px solid var(--border-soft);
		margin: 0.9em 0;
	}

	/* Inline code */
	:global(.markdown code) {
		font-family: 'SF Mono', ui-monospace, 'Menlo', monospace;
		font-size: 0.88em;
		background: rgba(15, 23, 42, 0.06);
		padding: 0.12em 0.36em;
		border-radius: 5px;
	}

	:global(.markdown kbd) {
		font-family: 'SF Mono', ui-monospace, 'Menlo', monospace;
		font-size: 0.8em;
		line-height: 1;
		padding: 0.2em 0.45em;
		border: 1px solid var(--border-soft);
		border-bottom-width: 2px;
		border-radius: 5px;
		background: var(--color-fafafa);
		color: var(--text-main);
		white-space: nowrap;
	}

	:global(.markdown mark) {
		background: var(--color-fff3a3);
		color: inherit;
		padding: 0.05em 0.2em;
		border-radius: 3px;
	}

	/* Block math: allow horizontal scroll for wide equations. */
	:global(.markdown .katex-display) {
		margin: 0.6em 0;
		overflow-x: auto;
		overflow-y: hidden;
		padding: 0.2em 0;
	}

	:global(.markdown .math-error) {
		color: var(--status-error);
		font-family: 'SF Mono', ui-monospace, 'Menlo', monospace;
		font-size: 0.88em;
	}

	/* Fenced code: wrapper + copy button from render.ts.
	   Icons match Lucide Copy/Check used by message-action. */
	:global(.markdown .md-code-block) {
		position: relative;
		margin: 0 0 0.6em;
	}

	:global(.markdown .md-code-copy) {
		--md-copy-icon: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='24' height='24' viewBox='0 0 24 24' fill='none' stroke='black' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Crect width='14' height='14' x='8' y='8' rx='2' ry='2'/%3E%3Cpath d='M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2'/%3E%3C/svg%3E");
		--md-check-icon: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='24' height='24' viewBox='0 0 24 24' fill='none' stroke='black' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='M20 6 9 17l-5-5'/%3E%3C/svg%3E");
		position: absolute;
		top: 0.4em;
		right: 0.4em;
		z-index: 1;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 1.75rem;
		height: 1.75rem;
		padding: 0;
		border: 1px solid transparent;
		border-radius: 7px;
		background: rgba(255, 255, 255, 0.92);
		color: var(--text-soft);
		cursor: pointer;
		transition:
			color var(--duration-fast, 120ms) var(--ease-smooth, ease),
			background var(--duration-fast, 120ms) var(--ease-smooth, ease),
			border-color var(--duration-fast, 120ms) var(--ease-smooth, ease);
	}

	:global(.markdown .md-code-copy::before) {
		content: '';
		display: block;
		width: 13px;
		height: 13px;
		background-color: currentColor;
		mask: var(--md-copy-icon) center / contain no-repeat;
		-webkit-mask: var(--md-copy-icon) center / contain no-repeat;
	}

	:global(.markdown .md-code-copy:hover) {
		color: var(--text-main);
		border-color: var(--border-soft);
		background: var(--panel-bg);
	}

	:global(.markdown .md-code-copy:focus-visible) {
		outline: 2px solid var(--accent);
		outline-offset: 1px;
	}

	:global(.markdown .md-code-copy.is-copied) {
		color: var(--status-success);
	}

	:global(.markdown .md-code-copy.is-copied::before) {
		mask: var(--md-check-icon) center / contain no-repeat;
		-webkit-mask: var(--md-check-icon) center / contain no-repeat;
	}

	:global(.markdown pre) {
		margin: 0 0 0.6em;
		padding: 0.7em 0.85em;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		overflow-x: auto;
		background: var(--panel-bg);
		font-size: 0.86em;
		line-height: 1.5;
	}

	:global(.markdown .md-code-block > pre) {
		margin: 0;
		padding-right: 2.4em;
	}

	:global(.markdown pre.shiki) {
		background: var(--panel-bg) !important;
	}

	:global(.markdown pre code) {
		display: block;
		background: transparent;
		padding: 0;
		border-radius: 0;
		font-size: inherit;
		white-space: pre;
	}

	:global(.markdown table) {
		border-collapse: collapse;
		margin: 0 0 0.6em;
		font-size: 0.92em;
		display: block;
		max-width: 100%;
		overflow-x: auto;
	}

	:global(.markdown th),
	:global(.markdown td) {
		border: 1px solid var(--border-soft);
		padding: 0.35em 0.6em;
		text-align: left;
	}

	:global(.markdown th) {
		background: rgba(15, 23, 42, 0.03);
		font-weight: 650;
	}

	:global(.markdown img) {
		max-width: 100%;
		border-radius: 8px;
	}
</style>
