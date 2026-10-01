// @vitest-environment jsdom
import { render, screen } from '@testing-library/svelte';
import { tick } from 'svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import SessionPhaseHarness from './SessionPhaseHarness.svelte';
import { chatStore } from '$lib/stores/chat.svelte';
import { shellStore } from '$lib/stores/shell.svelte';

vi.mock('$lib/client/cometmind', () => ({
	getSession: vi.fn(),
	getSessionMessages: vi.fn(),
	listChildSessions: vi.fn()
}));

async function settle() {
	await tick();
	await Promise.resolve();
	await tick();
}

function phaseEl() {
	return screen.getByTestId('session-phase');
}

describe('createSessionPhase', () => {
	beforeEach(() => {
		chatStore.clear();
		shellStore.dockComposer();
	});

	it('centers an empty session and hides the conversation', async () => {
		const view = render(SessionPhaseHarness, { props: { sessionId: 'empty' } });
		await settle();

		expect(phaseEl().dataset.visible).toBe('false');
		expect(phaseEl().dataset.flightDone).toBe('false');
		expect(phaseEl().dataset.synced).toBe('true');
		expect(shellStore.composerPhase).toBe('centered');
		view.unmount();
	});

	it('marks flight done when the cache already has a user turn', async () => {
		chatStore.bindSession('full');
		chatStore.stageUserForSession('full', 'hello');
		chatStore.revealStagedUserForSession('full');
		shellStore.centerComposer();

		const view = render(SessionPhaseHarness, { props: { sessionId: 'full' } });
		await settle();

		expect(phaseEl().dataset.visible).toBe('true');
		expect(phaseEl().dataset.flightDone).toBe('true');
		expect(shellStore.composerPhase).toBe('docked');
		view.unmount();
	});

	it('does not count a status-only cache as a visible conversation', async () => {
		chatStore.bindSession('status-only');
		const view = render(SessionPhaseHarness, { props: { sessionId: 'status-only' } });
		await settle();

		expect(chatStore.hasCachedConversationTurns('status-only')).toBe(false);
		expect(phaseEl().dataset.visible).toBe('false');
		expect(shellStore.composerPhase).toBe('centered');
		view.unmount();
	});

	it('docks when the conversation becomes visible', async () => {
		const view = render(SessionPhaseHarness, { props: { sessionId: 'swap' } });
		await settle();
		expect(shellStore.composerPhase).toBe('centered');

		chatStore.bindSession('swap');
		chatStore.stageUserForSession('swap', 'now visible');
		chatStore.revealStagedUserForSession('swap');
		await settle();

		expect(phaseEl().dataset.visible).toBe('true');
		expect(shellStore.composerPhase).toBe('docked');
		view.unmount();
	});
});
