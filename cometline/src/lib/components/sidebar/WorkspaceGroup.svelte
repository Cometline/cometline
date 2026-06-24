<script lang="ts">
	import { slide } from 'svelte/transition';
	import { ChevronDown, ChevronRight, Folder } from '@lucide/svelte';
	import type { Session } from '$lib/types';
	import SessionRow from '$lib/components/sidebar/SessionRow.svelte';

	const WORKSPACE_SESSIONS_SLIDE = { duration: 180 };

	let {
		label,
		workspacePath,
		sessions,
		collapsed,
		active = false,
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
			<div class="workspace-sessions" transition:slide={WORKSPACE_SESSIONS_SLIDE}>
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
		border-left: 2px solid transparent;
		transition:
			background var(--duration-fast) var(--ease-smooth),
			border-color var(--duration-fast) var(--ease-smooth);
	}

	.workspace-group:not(.active) {
		border-left: 3px solid var(--workspace-inactive-color, #9a9a9f);
		padding-left: 4px;
		background: color-mix(in srgb, var(--workspace-inactive-color, #9a9a9f) 15%, transparent);
	}

	.workspace-group:not(.active):hover {
		background: color-mix(in srgb, var(--workspace-inactive-color, #9a9a9f) 30%, transparent);
	}

	.workspace-group.active {
		border-left: 3px solid var(--hero-composer-glow-color, var(--accent));
		padding-left: 4px;
		background: color-mix(
			in srgb,
			var(--hero-composer-glow-color, var(--accent)) 30%,
			transparent
		);
	}

	.workspace-group.active:hover {
		background: color-mix(
			in srgb,
			var(--hero-composer-glow-color, var(--accent)) 40%,
			transparent
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
		/* padding-left: 6px; */
		--session-group-color: var(
			--workspace-group-color,
			var(--workspace-inactive-color, #9a9a9f)
		);
	}

	.workspace-group.active .workspace-sessions {
		--session-group-color: var(--hero-composer-glow-color, var(--accent));
	}
</style>
