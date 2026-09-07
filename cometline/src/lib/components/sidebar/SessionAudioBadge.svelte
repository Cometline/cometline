<script lang="ts">
	import { tick } from 'svelte';
	import type { Session } from '$lib/types';
	import { navigateToSession } from '$lib/actions/navigate-to-session';
	import { sessionStore } from '$lib/stores/session.svelte';
	import { shellStore } from '$lib/stores/shell.svelte';
	import { isNarrowViewport } from '$lib/layout/narrow-viewport';
	import { webTabActivity, type WebTabActivity } from '$lib/workspace/web-tab-activity.svelte';
	import AudioActivityIcon from '../AudioActivityIcon.svelte';
	import { portal } from '../portal';
	import { clampTooltipPosition } from '../tooltip-position';

	let { session, tabs }: { session: Session; tabs: WebTabActivity[] } = $props();
	let trigger = $state<HTMLButtonElement | null>(null);
	let menu = $state<HTMLDivElement | null>(null);
	let open = $state(false);
	let placed = $state(false);
	let top = $state(0);
	let left = $state(0);
	let error = $state('');
	const menuId = $props.id();
	const allMuted = $derived(tabs.every((tab) => tab.surface.pageState.muted));
	const label = (tab: WebTabActivity) =>
		tab.surface.pageState.title || tab.surface.pageState.url || 'Web page';

	function close(restoreFocus = false) {
		open = false;
		if (restoreFocus && trigger?.isConnected) trigger.focus({ preventScroll: true });
	}

	async function showChooser() {
		open = true;
		placed = false;
		await tick();
		if (!open || !menu || !trigger) return;
		const position = clampTooltipPosition({
			anchor: trigger.getBoundingClientRect(),
			tip: menu.getBoundingClientRect(),
			viewport: { width: window.innerWidth, height: window.innerHeight }
		});
		top = position.top;
		left = position.left;
		placed = true;
		menu.querySelector<HTMLButtonElement>('button')?.focus({ preventScroll: true });
	}

	async function reveal(tab: WebTabActivity) {
		const key = `${tab.sessionId}:${tab.tabId}`;
		if (webTabActivity.get(key) !== tab) return;
		error = '';
		try {
			await navigateToSession(session);
			if (sessionStore.current?.id === tab.sessionId && webTabActivity.get(key) === tab) {
				shellStore.activateUrlTabForActive(tab.tabId);
				if (isNarrowViewport()) shellStore.closeSidebar();
			}
			close();
		} catch {
			error = 'Could not open this tab. Try again.';
			await showChooser();
		}
	}

	function activate() {
		if (open) close();
		else if (tabs.length === 1) void reveal(tabs[0]);
		else void showChooser();
	}

	function onOutside(event: Event) {
		const target = event.target;
		if (open && target instanceof Node && !menu?.contains(target) && !trigger?.contains(target))
			close();
	}

	$effect(() => {
		if (!open) return;
		const onKey = (event: KeyboardEvent) => {
			if (event.key === 'Escape') {
				event.preventDefault();
				close(true);
			}
		};
		const onResize = () => close();
		window.addEventListener('pointerdown', onOutside);
		window.addEventListener('focusin', onOutside);
		window.addEventListener('keydown', onKey);
		window.addEventListener('resize', onResize);
		window.addEventListener('scroll', onOutside, true);
		return () => {
			window.removeEventListener('pointerdown', onOutside);
			window.removeEventListener('focusin', onOutside);
			window.removeEventListener('keydown', onKey);
			window.removeEventListener('resize', onResize);
			window.removeEventListener('scroll', onOutside, true);
		};
	});
</script>

<button
	bind:this={trigger}
	type="button"
	class="audio-badge"
	class:muted={allMuted}
	aria-label={`Audio tabs for ${session.title || 'New Chat'}`}
	aria-haspopup={tabs.length > 1 || open ? 'dialog' : undefined}
	aria-expanded={open}
	aria-controls={open ? menuId : undefined}
	title={`${allMuted ? 'Muted audio' : 'Playing audio'}: ${tabs.map(label).join(', ')}`}
	onmousedown={(event) => {
		event.preventDefault();
		event.stopPropagation();
	}}
	onclick={(event) => {
		event.stopPropagation();
		activate();
	}}
>
	<AudioActivityIcon muted={allMuted} />
</button>

{#if open}
	<div
		use:portal
		bind:this={menu}
		id={menuId}
		class="audio-chooser"
		role="dialog"
		aria-label="Audio tabs"
		style:top={`${top}px`}
		style:left={`${left}px`}
		style:visibility={placed ? 'visible' : 'hidden'}
	>
		<p class="heading">Audio tabs</p>
		{#if error}<p class="error" role="alert">{error}</p>{/if}
		{#each tabs as tab (tab.tabId)}
			<div class="audio-row">
				<button
					type="button"
					class="reveal"
					onclick={() => void reveal(tab)}
					title={tab.surface.pageState.url}
				>
					<span class="page-title">{label(tab)}</span>
					<span class="page-status"
						>{tab.surface.pageState.muted ? 'Muted' : 'Playing audio'}</span
					>
				</button>
				<button
					type="button"
					class="mute-button"
					disabled={!tab.surface.pageState.ready}
					aria-label={`${tab.surface.pageState.muted ? 'Unmute' : 'Mute'} ${label(tab)}`}
					aria-pressed={tab.surface.pageState.muted}
					title={tab.surface.pageState.muted ? 'Unmute tab' : 'Mute tab'}
					onclick={() => tab.surface.toggleAudioMuted()}
				>
					<AudioActivityIcon muted={tab.surface.pageState.muted} />
				</button>
			</div>
		{/each}
	</div>
{/if}

<style>
	.audio-badge,
	.mute-button {
		display: inline-grid;
		place-items: center;
		flex-shrink: 0;
		width: 24px;
		height: 24px;
		padding: 0;
		border: 0;
		border-radius: 6px;
		color: var(--accent);
		background: transparent;
		cursor: pointer;
	}
	.audio-badge.muted {
		color: var(--text-muted);
	}
	.audio-badge:hover,
	.mute-button:hover,
	.reveal:hover {
		background: color-mix(in srgb, var(--text-main) 8%, transparent);
	}
	button:focus-visible {
		outline: 2px solid var(--accent);
		outline-offset: -2px;
	}
	button:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.audio-chooser {
		position: fixed;
		z-index: 100;
		width: min(280px, calc(100vw - 16px));
		max-height: min(320px, calc(100dvh - 16px));
		overflow-y: auto;
		box-sizing: border-box;
		padding: 8px;
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		background: var(--panel-bg);
		box-shadow: var(--shadow-card);
	}
	.heading {
		margin: 2px 8px 6px;
		font-size: 11px;
		font-weight: 600;
		color: var(--text-muted);
	}
	.audio-row {
		display: flex;
		align-items: center;
		gap: 4px;
	}
	.reveal {
		min-width: 0;
		flex: 1;
		display: grid;
		gap: 2px;
		padding: 8px;
		border: 0;
		border-radius: 6px;
		text-align: left;
		color: var(--text-main);
		background: transparent;
		cursor: pointer;
	}
	.page-title {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 12px;
	}
	.page-status {
		font-size: 10px;
		color: var(--text-muted);
	}
	.error {
		color: var(--status-error);
		font-size: 12px;
		margin: 8px;
	}
</style>
