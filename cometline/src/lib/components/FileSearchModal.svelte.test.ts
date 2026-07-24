// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterAll, beforeEach, describe, expect, it, vi } from 'vitest';

const {
	openFilePreviewForActive,
	saveFileSearchSource,
	loadFileSearchOptions
} = vi.hoisted(() => ({
	openFilePreviewForActive: vi.fn(),
	saveFileSearchSource: vi.fn(async () => {}),
	loadFileSearchOptions: vi.fn(async () => ['src/app.ts', 'src/lib/foo.ts'])
}));

vi.mock('$lib/stores/shell.svelte', () => ({
	shellStore: {
		get workspacePath() {
			return '/repo';
		},
		openFilePreviewForActive
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
		showModal.mockClear();
		openFilePreviewForActive.mockClear();
		saveFileSearchSource.mockClear();
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

	it('persists the wiki/workspace toggle preference', async () => {
		render(FileSearchModal, { open: true, onClose: () => {} });

		await fireEvent.click(screen.getByRole('button', { name: 'Wiki' }));
		expect(saveFileSearchSource).toHaveBeenCalledWith('wiki');
	});
});
