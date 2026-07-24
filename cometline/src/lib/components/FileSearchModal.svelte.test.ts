// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterAll, beforeEach, describe, expect, it, vi } from 'vitest';

const {
	openFilePreviewForActive,
	saveFileSearchSource,
	setWebPanelBrowseSource,
	loadFileSearchOptions,
	browseSource
} = vi.hoisted(() => ({
	openFilePreviewForActive: vi.fn(),
	saveFileSearchSource: vi.fn(async () => {}),
	setWebPanelBrowseSource: vi.fn(),
	loadFileSearchOptions: vi.fn(async () => ['src/app.ts', 'src/lib/foo.ts']),
	browseSource: { value: 'changes' as 'wiki' | 'workspace' | 'changes' }
}));

vi.mock('$lib/stores/shell.svelte', () => ({
	shellStore: {
		get workspacePath() {
			return '/repo';
		},
		get webPanelBrowseSource() {
			return browseSource.value;
		},
		openFilePreviewForActive,
		setWebPanelBrowseSource
	}
}));

vi.mock('$lib/stores/settings.svelte', () => ({
	settingsStore: {
		settings: {
			app: { fileSearchSource: 'workspace' as const }
		},
		saveFileSearchSource
	}
}));

vi.mock('$lib/workspace/file-search', () => ({
	loadFileSearchOptions
}));

import FileSearchModal from './FileSearchModal.svelte';

const originalShowModal = Object.getOwnPropertyDescriptor(HTMLDialogElement.prototype, 'showModal');
const originalClose = Object.getOwnPropertyDescriptor(HTMLDialogElement.prototype, 'close');
const showModal = vi.fn(function (this: HTMLDialogElement) {
	this.open = true;
});

describe('FileSearchModal', () => {
	beforeEach(() => {
		browseSource.value = 'changes';
		showModal.mockClear();
		openFilePreviewForActive.mockClear();
		saveFileSearchSource.mockClear();
		setWebPanelBrowseSource.mockClear();
		loadFileSearchOptions.mockClear();
		loadFileSearchOptions.mockResolvedValue(['src/app.ts', 'src/lib/foo.ts']);
		Object.defineProperty(HTMLDialogElement.prototype, 'showModal', {
			configurable: true,
			value: showModal
		});
		Object.defineProperty(HTMLDialogElement.prototype, 'close', {
			configurable: true,
			value(this: HTMLDialogElement) {
				this.open = false;
			}
		});
	});

	afterAll(() => {
		if (originalShowModal) {
			Object.defineProperty(HTMLDialogElement.prototype, 'showModal', originalShowModal);
		} else {
			Reflect.deleteProperty(HTMLDialogElement.prototype, 'showModal');
		}
		if (originalClose) {
			Object.defineProperty(HTMLDialogElement.prototype, 'close', originalClose);
		} else {
			Reflect.deleteProperty(HTMLDialogElement.prototype, 'close');
		}
	});

	it('loads results and opens the selected file in the web panel', async () => {
		const onClose = vi.fn(() => {});
		render(FileSearchModal, { open: true, onClose });

		expect(showModal).toHaveBeenCalledOnce();
		await waitFor(() => {
			expect(screen.getByRole('button', { name: /app\.ts/ })).toBeTruthy();
		});

		await fireEvent.click(screen.getByRole('button', { name: /app\.ts/ }));
		expect(openFilePreviewForActive).toHaveBeenCalledWith('src/app.ts');
		expect(onClose).toHaveBeenCalledOnce();
	});

	it('persists the wiki/workspace toggle preference and syncs browse source', async () => {
		render(FileSearchModal, { open: true, onClose: () => {} });

		await fireEvent.click(screen.getByRole('button', { name: 'Wiki' }));
		expect(saveFileSearchSource).toHaveBeenCalledWith('wiki');
		expect(setWebPanelBrowseSource).toHaveBeenCalledWith('wiki');
	});

	it('prefers panel browse source over settings fileSearchSource', async () => {
		browseSource.value = 'wiki';
		render(FileSearchModal, { open: true, onClose: () => {} });

		await waitFor(() => {
			expect(loadFileSearchOptions).toHaveBeenCalled();
		});
		const lastCall = loadFileSearchOptions.mock.calls.at(-1) as unknown as
			| [string, ...unknown[]]
			| undefined;
		expect(lastCall?.[0]).toBe('wiki');
	});
});
