<script lang="ts">
	import { slide } from 'svelte/transition';
	import { ChevronDown, ChevronRight, Pin } from '@lucide/svelte';
	import type { Session } from '$lib/types';
	import SessionRow from '$lib/components/sidebar/SessionRow.svelte';

	const PINNED_SESSIONS_SLIDE = { duration: 180 };

	let {
		sessions,
		collapsed,
		currentSessionId,
		deletingID,
		pinningID,
		onToggle,
		onSelectSession,
		onDeleteSession,
		onPinSession,
		onSessionContextMenu
	}: {
		sessions: Session[];
		collapsed: boolean;
		currentSessionId: string | null;
		deletingID: string | null;
		pinningID: string | null;
		onToggle: () => void;
		onSelectSession: (session: Session) => void;
		onDeleteSession: (session: Session) => void;
		onPinSession: (session: Session) => void;
		onSessionContextMenu: (session: Session, event: MouseEvent) => void;
	} = $props();
</script>

<div class="pinned-entry">
	<div class="pinned-group">
		<button
			class="pinned-header"
			aria-expanded={!collapsed}
			onclick={onToggle}
			title="Pinned sessions"
		>
			<span class="pinned-chevron">
				{#if collapsed}
					<ChevronRight size={13} stroke-width={2} />
				{:else}
					<ChevronDown size={13} stroke-width={2} />
				{/if}
			</span>
			<Pin size={13} stroke-width={2} class="pinned-icon" />
			<span class="pinned-label">Pinned</span>
			<span class="pinned-count">{sessions.length}</span>
		</button>

		{#if !collapsed}
			<div class="pinned-sessions" transition:slide={PINNED_SESSIONS_SLIDE}>
				{#each sessions as session (session.id)}
					<SessionRow
						{session}
						showWorkspaceLabel
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
		{/if}
	</div>
</div>

<style>
	.pinned-entry {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.pinned-group {
		display: flex;
		flex-direction: column;
		gap: 4px;
		border-radius: 8px;
		padding: 2px;
		border: 1px solid color-mix(in srgb, var(--pinned-group-color, #b45309) 20%, transparent);
		background: linear-gradient(
			135deg,
			var(--pinned-group-bg, color-mix(in srgb, #b45309 10%, transparent)),
			color-mix(in srgb, var(--pinned-group-color, #b45309) 4%, transparent)
		);
		box-shadow: 0 8px 22px
			color-mix(in srgb, var(--pinned-group-color, #b45309) 7%, transparent);
		transition:
			background var(--duration-fast) var(--ease-smooth),
			border-color var(--duration-fast) var(--ease-smooth),
			box-shadow var(--duration-fast) var(--ease-smooth);
	}

	.pinned-group:hover {
		border-color: color-mix(in srgb, var(--pinned-group-color, #b45309) 30%, transparent);
		background: linear-gradient(
			135deg,
			var(--pinned-group-bg-hover, color-mix(in srgb, #b45309 16%, transparent)),
			color-mix(in srgb, var(--pinned-group-color, #b45309) 7%, transparent)
		);
	}

	.pinned-header {
		display: flex;
		align-items: center;
		gap: 6px;
		width: 100%;
		padding: 6px 8px;
		border: none;
		border-radius: 7px;
		background: transparent;
		color: var(--pinned-group-color, #b45309);
		font-size: 11px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.02em;
		cursor: pointer;
		text-align: left;
	}

	.pinned-chevron {
		display: grid;
		place-items: center;
		flex-shrink: 0;
		color: var(--pinned-group-color, #b45309);
	}

	.pinned-header :global(.pinned-icon) {
		flex-shrink: 0;
		color: var(--pinned-group-color, #b45309);
	}

	.pinned-label {
		min-width: 0;
		flex: 1;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		color: var(--text-main);
	}

	.pinned-count {
		flex-shrink: 0;
		font-size: 10px;
		font-weight: 600;
		color: var(--text-soft);
		background: rgba(15, 23, 42, 0.06);
		border-radius: 999px;
		padding: 1px 6px;
	}

	.pinned-sessions {
		display: flex;
		flex-direction: column;
		gap: 2px;
		--session-group-color: var(--pinned-group-color);
	}
</style>
