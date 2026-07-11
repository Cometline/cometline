<script lang="ts">
	import { slide } from 'svelte/transition';
	import { ChevronDown, ChevronRight, Folder, ArrowDown } from '@lucide/svelte';
	import type { Session } from '$lib/types';
	import SessionRow from '$lib/components/sidebar/SessionRow.svelte';

	const WORKSPACE_SESSIONS_SLIDE = { duration: 180 };
	const VISIBLE_LIMIT = 5;

	let {
		label,
		workspacePath,
		sessions,
		collapsed,
		active = false,
		searchActive = false,
		currentSessionId,
		deletingID,
		pinningID,
		onToggle,
		onSelectSession,
		onDeleteSession,
		onPinSession,
		onSessionContextMenu
	}: {
		label: string;
		workspacePath: string;
		sessions: Session[];
		collapsed: boolean;
		active?: boolean;
		searchActive?: boolean;
		currentSessionId: string | null;
		deletingID: string | null;
		pinningID: string | null;
		onToggle: () => void;
		onSelectSession: (session: Session) => void;
		onDeleteSession: (session: Session) => void;
		onPinSession: (session: Session) => void;
		onSessionContextMenu: (session: Session, event: MouseEvent) => void;
	} = $props();

	let overflow = $derived(!searchActive && sessions.length > VISIBLE_LIMIT);
	let hiddenCount = $state(0);
	let scrollEl = $state<HTMLDivElement | null>(null);

	function onScroll() {
		const el = scrollEl;
		if (!el) return;
		const maxScroll = el.scrollHeight - el.clientHeight;
		if (maxScroll <= 0) {
			hiddenCount = 0;
			return;
		}
		const remaining = maxScroll - el.scrollTop;
		if (remaining <= 0) {
			hiddenCount = 0;
			return;
		}
		const totalOverflow = sessions.length - VISIBLE_LIMIT;
		const fractionLeft = remaining / maxScroll;
		hiddenCount = Math.max(1, Math.round(totalOverflow * fractionLeft));
	}

	$effect(() => {
		if (overflow && scrollEl) {
			hiddenCount = sessions.length - VISIBLE_LIMIT;
		}
	});
</script>

<div class="workspace-entry">
	<div class="workspace-group" class:active>
		<button
			class="workspace-header"
			aria-expanded={!collapsed}
			aria-current={active ? 'true' : undefined}
			onclick={onToggle}
			title={workspacePath}
		>
			<span class="workspace-chevron">
				{#if collapsed}
					<ChevronRight size={13} stroke-width={2} />
				{:else}
					<ChevronDown size={13} stroke-width={2} />
				{/if}
			</span>
			<Folder size={13} stroke-width={1.8} class="workspace-folder" />
			<span class="workspace-label">{label}</span>
			<span class="workspace-count">{sessions.length}</span>
		</button>

		{#if !collapsed}
			<div
				class="workspace-sessions"
				class:overflow
				transition:slide={WORKSPACE_SESSIONS_SLIDE}
			>
				<div
					class="workspace-sessions-scroll scrollbar-none"
					bind:this={scrollEl}
					onscroll={onScroll}
				>
					{#each sessions as session (session.id)}
						<SessionRow
							{session}
							selected={currentSessionId === session.id}
							deleting={deletingID === session.id}
							pinning={pinningID === session.id}
							onSelect={() => onSelectSession(session)}
							onDelete={() => onDeleteSession(session)}
							onPin={() => onPinSession(session)}
							onContextMenu={(event) => onSessionContextMenu(session, event)}
						/>
					{/each}
				</div>

				{#if overflow && hiddenCount > 0}
					<span class="workspace-overflow-indicator" aria-hidden="true">
						<ArrowDown size={12} stroke-width={2.5} />
						<span class="workspace-overflow-count">+{hiddenCount}</span>
					</span>
				{/if}
			</div>
		{/if}
	</div>
</div>

<style>
	.workspace-entry {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.workspace-group {
		display: flex;
		flex-direction: column;
		gap: 4px;
		border-radius: 8px;
		padding: 2px;
		border: 1px solid transparent;
		transition:
			background var(--duration-fast) var(--ease-smooth),
			border-color var(--duration-fast) var(--ease-smooth),
			box-shadow var(--duration-fast) var(--ease-smooth);
	}

	.workspace-group:not(.active) {
		background: linear-gradient(
			135deg,
			color-mix(in srgb, var(--workspace-inactive-color, #9a9a9f) 16%, transparent),
			color-mix(in srgb, var(--workspace-inactive-color, #9a9a9f) 6%, transparent)
		);
		border-color: color-mix(in srgb, var(--workspace-inactive-color, #9a9a9f) 14%, transparent);
	}

	.workspace-group:not(.active):hover {
		background: linear-gradient(
			135deg,
			color-mix(in srgb, var(--workspace-inactive-color, #9a9a9f) 24%, transparent),
			color-mix(in srgb, var(--workspace-inactive-color, #9a9a9f) 9%, transparent)
		);
	}

	.workspace-group.active {
		background: linear-gradient(
			135deg,
			color-mix(in srgb, var(--hero-composer-glow-color, var(--accent)) 31%, transparent),
			color-mix(in srgb, var(--hero-composer-glow-color, var(--accent)) 12%, transparent)
		);
		border-color: color-mix(
			in srgb,
			var(--hero-composer-glow-color, var(--accent)) 26%,
			transparent
		);
		box-shadow: 0 8px 22px
			color-mix(in srgb, var(--hero-composer-glow-color, var(--accent)) 8%, transparent);
	}

	.workspace-group.active:hover {
		background: linear-gradient(
			135deg,
			color-mix(in srgb, var(--hero-composer-glow-color, var(--accent)) 38%, transparent),
			color-mix(in srgb, var(--hero-composer-glow-color, var(--accent)) 16%, transparent)
		);
	}

	.workspace-group.active .workspace-label {
		color: var(--text-main);
	}

	.workspace-group.active .workspace-chevron,
	.workspace-group.active :global(.workspace-folder) {
		color: var(--hero-composer-glow-color, var(--accent));
	}

	.workspace-header {
		display: flex;
		align-items: center;
		gap: 6px;
		width: 100%;
		padding: 6px 8px;
		border: none;
		border-radius: 7px;
		background: transparent;
		color: var(--workspace-inactive-color, #9a9a9f);
		font-size: 11px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.02em;
		cursor: pointer;
		text-align: left;
	}

	.workspace-group:hover .workspace-header {
		color: var(--text-muted);
	}

	.workspace-chevron {
		display: grid;
		place-items: center;
		flex-shrink: 0;
		color: var(--workspace-inactive-color, #9a9a9f);
		transition: color var(--duration-fast) var(--ease-smooth);
	}

	.workspace-header :global(.workspace-folder) {
		flex-shrink: 0;
		color: var(--workspace-inactive-color, #9a9a9f);
		transition: color var(--duration-fast) var(--ease-smooth);
	}

	.workspace-label {
		min-width: 0;
		flex: 1;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		transition: color var(--duration-fast) var(--ease-smooth);
	}

	.workspace-count {
		flex-shrink: 0;
		font-size: 10px;
		font-weight: 600;
		color: var(--text-soft);
		background: rgba(15, 23, 42, 0.06);
		border-radius: 999px;
		padding: 1px 6px;
	}

	.workspace-sessions {
		display: flex;
		flex-direction: column;
		gap: 2px;
		--session-group-color: var(
			--workspace-group-color,
			var(--workspace-inactive-color, #9a9a9f)
		);
	}

	.workspace-group.active .workspace-sessions {
		--session-group-color: var(--hero-composer-glow-color, var(--accent));
	}

	.workspace-sessions.overflow {
		gap: 0;
	}

	.workspace-sessions-scroll {
		display: flex;
		flex-direction: column;
		gap: 2px;
		padding-bottom: 2px;
	}

	.workspace-sessions.overflow .workspace-sessions-scroll {
		max-height: calc(5 * 32px);
		overflow-y: auto;
	}

	.workspace-overflow-indicator {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 3px;
		width: 100%;
		padding: 2px 0;
		color: var(--text-muted);
	}

	.workspace-overflow-count {
		font-size: 9px;
		font-weight: 600;
	}
</style>
