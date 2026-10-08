<script lang="ts">
	import { slide } from 'svelte/transition';
	import { ArrowDown } from '@lucide/svelte';
	import type { Session } from '#lib/types.js';
	import SessionRow from '#lib/features/sidebar/components/SessionRow.svelte';

	const WORKSPACE_SESSIONS_SLIDE = { duration: 180 };
	const VISIBLE_LIMIT = 5;

	let {
		sessions,
		collapsed,
		searchActive = false,
		currentSessionId,
		deletingID,
		pinningID,
		onSelectSession,
		onDeleteSession,
		onPinSession,
		onRenameSession,
		onSessionContextMenu
	}: {
		sessions: Session[];
		collapsed: boolean;
		searchActive?: boolean;
		currentSessionId: string | null;
		deletingID: string | null;
		pinningID: string | null;
		onSelectSession: (session: Session) => void;
		onDeleteSession: (session: Session) => void;
		onPinSession: (session: Session) => void;
		onRenameSession: (session: Session) => void;
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

{#if !collapsed}
	<div class="workspace-sessions" class:overflow transition:slide={WORKSPACE_SESSIONS_SLIDE}>
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
					onRename={() => onRenameSession(session)}
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

<style>
	.workspace-sessions {
		display: flex;
		flex-direction: column;
		gap: 2px;
		--session-group-color: var(
			--workspace-group-color,
			var(--workspace-inactive-color, var(--workspace-group-color))
		);
	}

	:global(.workspace-group.active) .workspace-sessions {
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
		font-size: 11px;
		font-weight: 600;
	}
</style>
