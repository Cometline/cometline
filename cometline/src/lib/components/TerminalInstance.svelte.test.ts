// @vitest-environment jsdom

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render } from '@testing-library/svelte';
import { tick } from 'svelte';

const { fitAddonConstructor, terminalConstructor, shellStore, terminalStore } = vi.hoisted(() => ({
	fitAddonConstructor: vi.fn(),
	terminalConstructor: vi.fn(),
	shellStore: {
		setFocusedPane: vi.fn(),
		addWebContextForActive: vi.fn(),
		requestComposerFocus: vi.fn()
	},
	terminalStore: {
		getSnapshot: vi.fn(() => null),
		subscribe: vi.fn(() => () => {}),
		resize: vi.fn(),
		write: vi.fn()
	}
}));

vi.mock('@xterm/xterm', () => ({
	Terminal: class {
		constructor(options: unknown) {
			return terminalConstructor(options);
		}
	}
}));
vi.mock('@xterm/addon-fit', () => ({
	FitAddon: class {
		constructor() {
			return fitAddonConstructor();
		}
	}
}));
vi.mock('@xterm/xterm/css/xterm.css', () => ({}));
vi.mock('$lib/stores/shell.svelte', () => ({ shellStore }));
vi.mock('$lib/stores/terminal.svelte', () => ({ terminalStore }));

import TerminalInstance from './TerminalInstance.svelte';

describe('TerminalInstance', () => {
	beforeEach(() => {
		fitAddonConstructor.mockReset();
		fitAddonConstructor.mockReturnValue({ fit: vi.fn() });
		terminalConstructor.mockReset();
		terminalConstructor.mockImplementation(() => ({
			cols: 80,
			rows: 24,
			loadAddon: vi.fn(),
			open: vi.fn(),
			write: vi.fn(),
			onData: vi.fn(() => ({ dispose: vi.fn() })),
			focus: vi.fn(),
			getSelection: vi.fn(() => ''),
			clearSelection: vi.fn(),
			dispose: vi.fn()
		}));
		shellStore.setFocusedPane.mockReset();
		terminalStore.getSnapshot.mockReset();
		terminalStore.getSnapshot.mockReturnValue(null);
		terminalStore.subscribe.mockReset();
		terminalStore.subscribe.mockReturnValue(() => {});
		terminalStore.resize.mockReset();
		terminalStore.write.mockReset();
		vi.stubGlobal(
			'ResizeObserver',
			class {
				observe() {}
				disconnect() {}
			}
		);
		vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => {
			callback(0);
			return 1;
		});
		vi.stubGlobal('cancelAnimationFrame', () => {});
	});

	it('enables Option-drag selection for mouse-reporting TUIs', async () => {
		render(TerminalInstance, { props: { sessionId: 'session-1', active: true } });
		await tick();

		expect(terminalConstructor).toHaveBeenCalledWith(
			expect.objectContaining({ macOptionClickForcesSelection: true })
		);
	});

	it('tracks native terminal focus without issuing a resize request', async () => {
		const { container } = render(TerminalInstance, {
			props: { sessionId: 'session-1', active: true }
		});
		await tick();
		terminalStore.resize.mockClear();

		await fireEvent.focusIn(container.querySelector('.terminal-instance')!);

		expect(shellStore.setFocusedPane).toHaveBeenCalledWith('terminal');
		expect(terminalStore.resize).not.toHaveBeenCalled();
	});
});
