<script lang="ts">
	import { Globe, LoaderCircle, Plus, TriangleAlert, X } from '@lucide/svelte';
	import AudioActivityIcon from './AudioActivityIcon.svelte';
	import type { WebTabStatus } from '$lib/workspace/web-tab-activity.svelte';

	let {
		tabs,
		activeId,
		ariaLabel,
		dirtyById = {},
		labelFor,
		titleFor,
		onActivate,
		onClose,
		onNewTab,
		webStatusFor,
		onToggleMute
	}: {
		tabs: string[];
		activeId: string | null;
		ariaLabel: string;
		dirtyById?: Record<string, boolean>;
		labelFor: (id: string, active: boolean) => string;
		titleFor?: (id: string) => string;
		onActivate: (id: string) => void;
		onClose: (id: string) => void;
		onNewTab?: () => void;
		webStatusFor?: (id: string) => WebTabStatus | undefined;
		onToggleMute?: (id: string) => void;
	} = $props();
	const stripId = $props.id();

	function keepPaneFocus(event: MouseEvent) {
		event.preventDefault();
	}
</script>

<div class="panel-tabs">
	<div class="panel-tab-list" role="tablist" aria-label={ariaLabel}>
		{#each tabs as tabId (tabId)}
			{@const active = tabId === activeId}
			{@const label = labelFor(tabId, active)}
			{@const title = titleFor?.(tabId) ?? tabId}
			{@const dirty = Boolean(dirtyById[tabId])}
			{@const status = webStatusFor?.(tabId)}
			{@const statusLabel =
				status?.loadError || (status?.showLoading ? 'Loading page' : undefined)}
			<div
				class="panel-tab"
				class:active
				class:web-tab={Boolean(webStatusFor)}
				role="presentation"
			>
				<button
					type="button"
					class="panel-tab-button"
					role="tab"
					aria-selected={active}
					aria-describedby={statusLabel ? `${stripId}-status-${tabId}` : undefined}
					{title}
					onmousedown={keepPaneFocus}
					onclick={() => onActivate(tabId)}
				>
					{#if webStatusFor}
						<span
							class="tab-status"
							class:failed={Boolean(status?.loadError)}
							title={statusLabel}
							aria-hidden="true"
						>
							{#if status?.loadError}
								<TriangleAlert size={13} />
							{:else if status?.showLoading}
								<span class="loading-icon"><LoaderCircle size={13} /></span>
							{:else}
								<Globe size={13} />
							{/if}
						</span>
					{/if}
					<span class="panel-tab-label">{label}</span>
					{#if dirty}<span class="dirty-dot" aria-label="Unsaved changes">•</span>{/if}
				</button>
				{#if statusLabel}<span id={`${stripId}-status-${tabId}`} class="status-description"
						>{statusLabel}</span
					>{/if}
				{#if webStatusFor && onToggleMute}
					<span class="audio-slot">
						{#if status?.audible || status?.muted}
							<button
								type="button"
								class="tab-audio"
								class:muted={status.muted}
								disabled={!status.ready}
								aria-label={`${status.muted ? 'Unmute' : 'Mute'} ${label}`}
								aria-pressed={status.muted}
								title={status.muted
									? 'Muted - Click to unmute'
									: 'Playing audio - Click to mute'}
								onmousedown={(event) => {
									event.preventDefault();
									event.stopPropagation();
								}}
								onclick={(event) => {
									event.stopPropagation();
									onToggleMute(tabId);
								}}
							>
								<AudioActivityIcon muted={status.muted} />
							</button>
						{/if}
					</span>
				{/if}
				<button
					type="button"
					class="panel-tab-close"
					aria-label={`Close ${label}`}
					title={active ? 'Close (Cmd/Ctrl+W)' : 'Close'}
					onmousedown={keepPaneFocus}
					onclick={() => onClose(tabId)}
				>
					<X size={12} />
				</button>
			</div>
		{/each}
	</div>
	{#if onNewTab}
		<button
			type="button"
			class="new-tab-button"
			onmousedown={keepPaneFocus}
			onclick={onNewTab}
			aria-label="New tab"
			title="New tab"
		>
			<Plus size={14} />
		</button>
	{/if}
</div>

<style>
	.panel-tabs {
		display: flex;
		align-items: center;
		gap: 6px;
		min-width: 0;
		flex: 1;
		overflow: hidden;
	}

	.panel-tab-list {
		display: flex;
		align-items: center;
		gap: 6px;
		min-width: 0;
		flex: 1 1 auto;
		overflow-x: auto;
		overflow-y: hidden;
	}

	.panel-tab {
		display: flex;
		align-items: center;
		box-sizing: border-box;
		height: 26px;
		min-width: 3rem;
		max-width: 10rem;
		flex: 1 1 auto;
		border: 1px solid
			color-mix(in srgb, var(--hero-composer-glow-color) 22%, var(--border-soft));
		border-radius: 6px;
		background: color-mix(in srgb, var(--hero-composer-glow-color) 6%, transparent);
		box-shadow: none;
	}

	.panel-tab.active {
		border-color: color-mix(in srgb, var(--hero-composer-glow-color) 54%, var(--border-soft));
		background: color-mix(in srgb, var(--hero-composer-glow-color) 18%, var(--panel-bg));
		box-shadow: 0 0 8px var(--hero-composer-glow-soft);
	}
	.panel-tab.web-tab {
		min-width: 6rem;
	}

	.panel-tab-button {
		display: inline-flex;
		align-items: center;
		gap: 2px;
		min-width: 0;
		flex: 1;
		border: 0;
		background: transparent;
		color: var(--text-muted);
		font-size: 12px;
		font-weight: 600;
		padding: 4px 6px;
		cursor: pointer;
	}

	.panel-tab.active .panel-tab-button {
		color: var(--text-main);
	}

	.panel-tab-label {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.tab-status {
		display: inline-flex;
		flex-shrink: 0;
		width: 14px;
		height: 14px;
		margin-right: 3px;
		align-items: center;
		color: var(--text-soft);
	}
	.status-description {
		position: absolute;
		width: 1px;
		height: 1px;
		padding: 0;
		margin: -1px;
		overflow: hidden;
		clip-path: inset(50%);
		white-space: nowrap;
	}
	.tab-status.failed {
		color: var(--status-error);
	}
	.loading-icon {
		display: inline-flex;
		color: var(--accent);
		animation: tab-loading 1s linear infinite;
	}
	.audio-slot {
		display: inline-grid;
		place-items: center;
		width: 22px;
		flex-shrink: 0;
	}
	.tab-audio {
		display: inline-grid;
		place-items: center;
		border: 0;
		border-radius: 4px;
		width: 22px;
		height: 22px;
		padding: 0;
		background: transparent;
		color: var(--accent);
		cursor: pointer;
	}
	.tab-audio.muted {
		color: var(--text-muted);
	}
	.tab-audio:hover {
		background: color-mix(in srgb, var(--text-main) 10%, transparent);
	}
	.tab-audio:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: -2px;
	}
	.tab-audio:disabled {
		opacity: 0.5;
		cursor: default;
	}
	@keyframes tab-loading {
		to {
			transform: rotate(360deg);
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.loading-icon {
			animation: none;
		}
	}

	.panel-tab-close {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		width: 18px;
		height: 18px;
		margin-left: auto;
		margin-right: 4px;
		border: 0;
		border-radius: 4px;
		background: transparent;
		color: var(--text-muted);
		cursor: pointer;
	}

	.panel-tab:not(.active) .panel-tab-close {
		opacity: 0;
	}

	.panel-tab:not(.active):hover .panel-tab-close,
	.panel-tab:not(.active):focus-within .panel-tab-close {
		opacity: 1;
	}

	.panel-tab-close:hover {
		background: color-mix(in srgb, var(--text-main) 12%, transparent);
		color: var(--text-main);
	}

	.dirty-dot {
		color: var(--accent, #2563eb);
		font-weight: 700;
	}

	.new-tab-button {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		align-self: center;
		box-sizing: border-box;
		flex-shrink: 0;
		width: 26px;
		height: 26px;
		border: 1px dashed
			color-mix(in srgb, var(--hero-composer-glow-color) 28%, var(--border-soft));
		border-radius: 6px;
		background: transparent;
		color: var(--text-muted);
		cursor: pointer;
	}

	.new-tab-button:hover {
		color: var(--text-main);
		border-color: color-mix(in srgb, var(--hero-composer-glow-color) 54%, var(--border-soft));
		background: color-mix(in srgb, var(--hero-composer-glow-color) 10%, transparent);
	}
</style>
