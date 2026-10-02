import { describe, expect, it, vi } from 'vitest';
import { defaultKeyboardShortcuts } from '$lib/keyboard-shortcuts';
import { handleComposerKeydown, type ComposerKeydownDeps } from './composer-keydown';

function keyEvent(init: { metaKey?: boolean; shiftKey?: boolean }): KeyboardEvent {
	return {
		key: 'Enter',
		code: 'Enter',
		metaKey: init.metaKey ?? false,
		ctrlKey: false,
		altKey: false,
		shiftKey: init.shiftKey ?? false,
		isComposing: false,
		preventDefault: vi.fn()
	} as unknown as KeyboardEvent;
}

function deps(
	shortcuts = defaultKeyboardShortcuts()
): ComposerKeydownDeps & { submit: ReturnType<typeof vi.fn> } {
	return {
		getShortcuts: () => shortcuts,
		cycleReasoningEffort: vi.fn(),
		handleSlashMenuKeydown: () => false,
		handleMentionMenuKeydown: () => false,
		cycleAgentMode: vi.fn(),
		getValue: () => '',
		getImages: () => [],
		getPendingWebContextCount: () => 0,
		removeImage: vi.fn(),
		removeWebContextAt: vi.fn(),
		isBrowsingHistory: () => false,
		isCaretAtStart: () => false,
		isCaretAtEnd: () => false,
		navigateHistory: vi.fn(async () => undefined),
		isStreaming: () => false,
		onStop: vi.fn(),
		submit: vi.fn(async () => undefined)
	};
}

describe('handleComposerKeydown send binding', () => {
	it('submits on Command+Enter when that chord is the send shortcut', () => {
		const composer = deps({
			...defaultKeyboardShortcuts(),
			sendMessage: { key: 'Enter', command: true }
		});
		const event = keyEvent({ metaKey: true });

		handleComposerKeydown(event, composer);

		expect(event.preventDefault).toHaveBeenCalledOnce();
		expect(composer.submit).toHaveBeenCalledOnce();
	});

	it('does not submit a plain Enter when send is Command+Enter', () => {
		const composer = deps({
			...defaultKeyboardShortcuts(),
			sendMessage: { key: 'Enter', command: true }
		});

		handleComposerKeydown(keyEvent({}), composer);

		expect(composer.submit).not.toHaveBeenCalled();
	});
});
