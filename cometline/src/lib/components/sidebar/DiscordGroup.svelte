<script lang="ts">
	import { slide } from 'svelte/transition';
	import { ChevronDown, ChevronRight } from '@lucide/svelte';
	import type { Session } from '$lib/types';
	import SessionRow from '$lib/components/sidebar/SessionRow.svelte';

	const DISCORD_SESSIONS_SLIDE = { duration: 180 };

	let {
		sessions,
		collapsed,
		active = false,
		showDivider = false,
		currentSessionId,
		deletingID,
		onToggle,
		onSelectSession,
		onDeleteSession,
		onSessionContextMenu
	}: {
		sessions: Session[];
		collapsed: boolean;
		active?: boolean;
		showDivider?: boolean;
		currentSessionId: string | null;
		deletingID: string | null;
		onToggle: () => void;
		onSelectSession: (session: Session) => void;
		onDeleteSession: (session: Session) => void;
		onSessionContextMenu: (session: Session, event: MouseEvent) => void;
	} = $props();
</script>

<div class="discord-entry">
	<div class="discord-group" class:active>
		<button
			class="discord-header"
			aria-expanded={!collapsed}
			aria-current={active ? 'true' : undefined}
			onclick={onToggle}
			title="Discord gateway sessions"
		>
			<span class="discord-chevron">
				{#if collapsed}
					<ChevronRight size={13} stroke-width={2} />
				{:else}
					<ChevronDown size={13} stroke-width={2} />
				{/if}
			</span>
			<svg class="discord-icon" viewBox="0 0 24 24" aria-hidden="true">
				<path
					fill="currentColor"
					d="M20.317 4.37a19.791 19.791 0 0 0-4.885-1.515.074.074 0 0 0-.079.037c-.21.375-.444.864-.608 1.25a18.27 18.27 0 0 0-5.487 0 12.64 12.64 0 0 0-.617-1.25.077.077 0 0 0-.079-.037A19.736 19.736 0 0 0 3.677 4.37a.07.07 0 0 0-.032.027C.533 9.046-.32 13.58.099 18.057a.082.082 0 0 0 .031.057 19.9 19.9 0 0 0 5.993 3.03.078.078 0 0 0 .084-.028 14.09 14.09 0 0 0 1.226-1.994.076.076 0 0 0-.041-.106 13.107 13.107 0 0 1-1.872-.892.077.077 0 0 1-.008-.128 10.2 10.2 0 0 0 .372-.292.074.074 0 0 1 .077-.01c3.928 1.793 8.18 1.793 12.062 0a.074.074 0 0 1 .078.01c.12.098.246.198.373.292a.077.077 0 0 1-.006.127 12.299 12.299 0 0 1-1.873.892.077.077 0 0 0-.041.107c.36.698.772 1.362 1.225 1.993a.076.076 0 0 0 .084.028 19.839 19.839 0 0 0 6.002-3.03.077.077 0 0 0 .032-.054c.5-5.177-.838-9.864-3.549-13.66a.061.061 0 0 0-.031-.03zM8.02 15.33c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.956-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.956 2.418-2.157 2.418zm7.975 0c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.955-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.946 2.418-2.157 2.418z"
				/>
			</svg>
			<span class="discord-label">Discord</span>
			<span class="discord-count">{sessions.length}</span>
		</button>

		{#if !collapsed}
			<div class="discord-sessions" transition:slide={DISCORD_SESSIONS_SLIDE}>
				{#each sessions as session (session.id)}
					<SessionRow
						{session}
						showGatewayLabel
						showPin={false}
						selected={currentSessionId === session.id}
						deleting={deletingID === session.id}
						onSelect={() => onSelectSession(session)}
						onDelete={() => onDeleteSession(session)}
						onPin={() => {}}
						onContextMenu={(event) => onSessionContextMenu(session, event)}
					/>
				{/each}
			</div>
		{/if}
	</div>
	{#if showDivider}
		<div class="discord-divider" role="separator" aria-hidden="true"></div>
	{/if}
</div>

<style>
	.discord-entry {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.discord-group {
		display: flex;
		flex-direction: column;
		gap: 4px;
		border-radius: 8px;
		padding: 2px;
		padding-left: 4px;
		margin-left: -6px;
		margin-right: -6px;
		border-left: 3px solid var(--workspace-inactive-color, #9a9a9f);
		background: var(--discord-group-bg, color-mix(in srgb, #5865f2 10%, transparent));
		transition:
			background var(--duration-fast) var(--ease-smooth),
			border-color var(--duration-fast) var(--ease-smooth);
	}

	.discord-group:hover {
		background: var(--discord-group-bg-hover, color-mix(in srgb, #5865f2 16%, transparent));
	}

	.discord-group.active {
		border-left-color: var(--discord-group-color, #5865f2);
	}

	.discord-group.active .discord-label {
		color: var(--text-main);
	}

	.discord-group.active .discord-chevron,
	.discord-group.active .discord-icon {
		color: var(--discord-group-color, #5865f2);
	}

	.discord-header {
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

	.discord-group:hover .discord-header {
		color: var(--text-muted);
	}

	.discord-chevron {
		display: grid;
		place-items: center;
		flex-shrink: 0;
		color: var(--workspace-inactive-color, #9a9a9f);
		transition: color var(--duration-fast) var(--ease-smooth);
	}

	.discord-icon {
		width: 13px;
		height: 13px;
		flex-shrink: 0;
		color: var(--workspace-inactive-color, #9a9a9f);
		transition: color var(--duration-fast) var(--ease-smooth);
	}

	.discord-label {
		min-width: 0;
		flex: 1;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		transition: color var(--duration-fast) var(--ease-smooth);
	}

	.discord-count {
		flex-shrink: 0;
		font-size: 10px;
		font-weight: 600;
		color: var(--text-soft);
		background: rgba(15, 23, 42, 0.06);
		border-radius: 999px;
		padding: 1px 6px;
	}

	.discord-sessions {
		display: flex;
		flex-direction: column;
		gap: 2px;
		padding-left: 6px;
	}

	.discord-divider {
		height: 2px;
		margin: 8px 6px 6px;
		background: rgba(15, 23, 42, 0.16);
		border-radius: 1px;
		flex-shrink: 0;
	}
</style>
