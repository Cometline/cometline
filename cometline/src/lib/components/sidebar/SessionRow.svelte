<script lang="ts">
	import { Pin, PinOff, Trash2 } from '@lucide/svelte';
	import type { Session } from '$lib/types';
	import { workspaceLabel, gatewaySessionLabel } from '$lib/sessions/group-by-workspace';
	import { chatStore } from '$lib/stores/chat.svelte';
	import { terminalStore } from '$lib/stores/terminal.svelte';

	let {
		session,
		selected = false,
		deleting = false,
		pinning = false,
		showWorkspaceLabel = false,
		showGatewayLabel = false,
		showPin = true,
		showActions = true,
		onSelect,
		onDelete,
		onPin,
		onRename,
		onContextMenu
	}: {
		session: Session;
		selected?: boolean;
		deleting?: boolean;
		pinning?: boolean;
		showWorkspaceLabel?: boolean;
		showGatewayLabel?: boolean;
		showPin?: boolean;
		showActions?: boolean;
		onSelect: () => void;
		onDelete: () => void;
		onPin: () => void;
		onRename?: () => void;
		onContextMenu: (event: MouseEvent) => void;
	} = $props();

	let streaming = $derived(
		chatStore.isStreamingFor(session.id) || chatStore.hasInFlightTurn(session.id)
	);
	let terminalRunning = $derived(terminalStore.isRunning(session.id));

	function handleContextMenu(event: MouseEvent) {
		event.preventDefault();
		onContextMenu(event);
	}

	function handleDblClick(event: MouseEvent) {
		if (!onRename) return;
		event.preventDefault();
		onRename();
	}
</script>

<div
	class="session-row-wrap group relative flex items-stretch"
	class:selected
	class:streaming={streaming && !selected}
	class:has-actions={showActions}
	role="group"
	oncontextmenu={handleContextMenu}
>
	<button class="session-row" onclick={onSelect} ondblclick={handleDblClick}>
		<span class="session-title-row">
			<span
				class="session-activity"
				aria-label={terminalRunning
					? streaming
						? 'Terminal running and responding'
						: 'Terminal running'
					: streaming
						? 'Responding'
						: undefined}
			>
				<span
					class:active={streaming}
					class:terminal={terminalRunning}
					class="session-streaming"
					title={terminalRunning ? 'Terminal running' : streaming ? 'Responding' : undefined}
				>{#if terminalRunning}<span class="terminal-marker">t</span>{/if}</span>
			</span>
			<span class="session-title">{session.title || 'Untitled'}</span>
		</span>
		{#if showGatewayLabel}
			<span class="session-workspace session-gateway">{gatewaySessionLabel(session)}</span>
		{:else if showWorkspaceLabel}
			<span class="session-workspace">{workspaceLabel(session.workspace_path)}</span>
		{/if}
	</button>
	{#if showActions}
		<div class="session-actions">
			{#if showPin}
				<button
					class="pin-session"
					class:active={session.pinned}
					disabled={pinning}
					onclick={onPin}
					aria-label={session.pinned
						? `Unpin ${session.title || 'Untitled'}`
						: `Pin ${session.title || 'Untitled'}`}
					title={session.pinned ? 'Unpin session' : 'Pin session'}
				>
					{#if session.pinned}
						<Pin size={13} stroke-width={2} />
					{:else}
						<PinOff size={13} stroke-width={1.9} />
					{/if}
				</button>
			{/if}
			<button
				class="delete-session"
				disabled={deleting}
				onclick={onDelete}
				aria-label={`Delete ${session.title || 'Untitled'}`}
				title="Delete session"
			>
				<Trash2 size={13} stroke-width={1.9} />
			</button>
		</div>
	{/if}
</div>

<style>
	.session-row-wrap {
		border-radius: 8px;
		padding-left: 0;
		transition:
			background-color var(--duration-fast) var(--ease-smooth),
			box-shadow var(--duration-fast) var(--ease-smooth);
	}

	.session-row-wrap:hover {
		background: var(--session-row-bg-hover);
		box-shadow: inset 0 0 0 1px var(--session-row-ring);
	}

	.session-row-wrap.streaming:not(.selected) {
		background: var(--session-row-bg-streaming);
		box-shadow: inset 0 0 0 1px var(--session-row-ring);
	}

	.session-row-wrap.streaming:not(.selected):hover {
		background: var(--session-row-bg-hover);
	}

	.session-row-wrap.selected {
		background: var(--session-row-bg-active);
		box-shadow:
			inset 0 0 0 1px var(--session-row-ring-active),
			0 8px 18px color-mix(in srgb, var(--session-group-color) 8%, transparent);
	}

	.session-row-wrap.selected:hover {
		background: var(--session-row-bg-active-hover);
	}

	.session-row {
		width: 100%;
		text-align: left;
		padding: 6px 8px;
		padding-right: 8px;
		border: none;
		background: transparent;
		color: var(--text-main);
		font-size: 13px;
		line-height: 1.35;
		font-weight: 450;
		cursor: pointer;
	}

	.session-row-wrap.has-actions .session-row {
		padding-right: 58px;
	}

	.session-row-wrap.selected .session-title {
		font-weight: 500;
	}

	.session-title-row {
		display: flex;
		align-items: center;
		gap: 6px;
		min-width: 0;
	}

	.session-title {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		display: block;
		flex: 1;
		min-width: 0;
	}

	.session-activity {
		position: relative;
		flex-shrink: 0;
		width: 10px;
		height: 10px;
		display: inline-grid;
		place-items: center;
	}

	.session-streaming {
		width: 10px;
		height: 10px;
		border-radius: 50%;
		background: var(--text-soft);
		opacity: 0.45;
	}

	.session-streaming.active {
		background: var(--session-group-color, var(--accent));
		opacity: 1;
		animation: session-streaming-pulse 1.2s ease-in-out infinite;
	}

	.session-streaming.terminal {
		display: inline-grid;
		place-items: center;
		font-size: 7px;
		font-weight: 800;
		line-height: 1;
		color: white;
		text-transform: uppercase;
	}

	@keyframes session-streaming-pulse {
		0%,
		100% {
			opacity: 0.35;
			transform: scale(0.85);
		}
		50% {
			opacity: 1;
			transform: scale(1);
		}
	}

	.session-workspace {
		display: block;
		margin-top: 1px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 10px;
		font-weight: 500;
		line-height: 1.3;
		color: var(--text-muted);
	}

	.session-actions {
		position: absolute;
		right: 4px;
		top: 50%;
		transform: translateY(-50%);
		display: flex;
		align-items: center;
		gap: 2px;
	}

	.pin-session,
	.delete-session {
		width: 24px;
		height: 24px;
		border: none;
		border-radius: 6px;
		background: transparent;
		color: var(--text-soft);
		display: grid;
		place-items: center;
		opacity: 0;
		cursor: pointer;
	}

	.session-row-wrap:hover .session-actions button,
	.session-row-wrap:focus-within .session-actions button {
		opacity: 1;
	}

	.pin-session.active {
		color: var(--pinned-group-color, #b45309);
	}

	.pin-session:hover:not(:disabled),
	.delete-session:hover:not(:disabled) {
		background: rgba(0, 0, 0, 0.06);
		color: var(--text-main);
	}

	.pin-session.active:hover:not(:disabled) {
		color: var(--pinned-group-color, #b45309);
	}

	.delete-session:hover:not(:disabled) {
		background: rgba(180, 35, 24, 0.08);
		color: var(--status-error);
	}

	.pin-session:disabled,
	.delete-session:disabled {
		opacity: 0.35;
	}
</style>
