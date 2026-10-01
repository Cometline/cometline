<script lang="ts">
	import { ChevronDown, ChevronRight, Folder, Plus } from '@lucide/svelte';
	import type { Session } from '$lib/types';
	import WorkspaceSessionList from '$lib/features/sidebar/components/sidebar/WorkspaceSessionList.svelte';

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
		onNewSession,
		onSelectSession,
		onDeleteSession,
		onPinSession,
		onRenameSession,
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
		onNewSession?: (workspacePath: string) => void;
		onSelectSession: (session: Session) => void;
		onDeleteSession: (session: Session) => void;
		onPinSession: (session: Session) => void;
		onRenameSession: (session: Session) => void;
		onSessionContextMenu: (session: Session, event: MouseEvent) => void;
	} = $props();
</script>

<div class="workspace-entry">
	<div class="workspace-group" class:active>
		<div class="workspace-header-row">
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
			</button>
			{#if onNewSession}
				<button
					type="button"
					class="workspace-add"
					aria-label={`New chat in ${label}`}
					title={`New chat in ${label}`}
					onclick={() => onNewSession(workspacePath)}
				>
					<Plus size={13} stroke-width={2.2} />
				</button>
			{/if}
			<span class="workspace-count">{sessions.length}</span>
		</div>

		<WorkspaceSessionList
			{sessions}
			{collapsed}
			{searchActive}
			{currentSessionId}
			{deletingID}
			{pinningID}
			{onSelectSession}
			{onDeleteSession}
			{onPinSession}
			{onRenameSession}
			{onSessionContextMenu}
		/>
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

	.workspace-header-row {
		display: flex;
		align-items: center;
		gap: 2px;
		width: 100%;
		min-width: 0;
	}

	.workspace-header {
		display: flex;
		align-items: center;
		gap: 6px;
		min-width: 0;
		flex: 1;
		padding: 6px 4px 6px 8px;
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

	.workspace-group:hover .workspace-header,
	.workspace-group:hover .workspace-add {
		color: var(--text-muted);
	}

	.workspace-add {
		display: grid;
		place-items: center;
		flex-shrink: 0;
		width: 18px;
		height: 18px;
		padding: 0;
		border: none;
		border-radius: 5px;
		background: transparent;
		color: var(--workspace-inactive-color, #9a9a9f);
		cursor: pointer;
	}

	.workspace-add:hover {
		background: rgba(15, 23, 42, 0.08);
		color: var(--text-main);
	}

	.workspace-group.active .workspace-add {
		color: var(--hero-composer-glow-color, var(--accent));
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
		margin-right: 6px;
		font-size: 10px;
		font-weight: 600;
		color: var(--text-soft);
		background: rgba(15, 23, 42, 0.06);
		border-radius: 999px;
		padding: 1px 6px;
	}
</style>
