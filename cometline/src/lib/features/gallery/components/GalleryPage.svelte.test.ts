// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterAll, beforeEach, describe, expect, it, vi } from 'vitest';
import GalleryPage from './GalleryPage.svelte';

const api = vi.hoisted(() => ({
	deleteMedia: vi.fn(),
	listMedia: vi.fn()
}));
const settings = vi.hoisted(() => ({
	settings: { app: { confirmBeforeDeletingMedia: true } },
	saveConfirmBeforeDeletingMedia: vi.fn()
}));

vi.mock('$lib/client/cometmind', () => api);
vi.mock('$lib/stores/settings.svelte', () => ({ settingsStore: settings }));

const item = {
	id: 'media-1',
	session_id: 'session-1',
	storage_session_id: 'session-1',
	workspace_id: 'workspace-1',
	kind: 'image',
	media_type: 'image/png',
	alt: 'Generated image',
	source: 'generated',
	status: 'ready',
	session_deleted: false,
	byte_size: 100,
	created_at: 1,
	url: '/api/v1/media/media-1/content'
};
const originalShowModal = Object.getOwnPropertyDescriptor(HTMLDialogElement.prototype, 'showModal');
const originalClose = Object.getOwnPropertyDescriptor(HTMLDialogElement.prototype, 'close');

describe('GalleryPage media deletion', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		settings.settings.app.confirmBeforeDeletingMedia = true;
		settings.saveConfirmBeforeDeletingMedia.mockResolvedValue(undefined);
		api.deleteMedia.mockResolvedValue(item);
		api.listMedia.mockResolvedValue({ items: [item] });
		Object.defineProperty(HTMLDialogElement.prototype, 'showModal', {
			configurable: true,
			value(this: HTMLDialogElement) {
				this.open = true;
			}
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

	it('asks before deleting media by default', async () => {
		render(GalleryPage);
		await screen.findByText('Generated image');

		await fireEvent.click(screen.getByRole('button', { name: 'Delete' }));

		expect(screen.getByRole('dialog')).toBeTruthy();
		expect(api.deleteMedia).not.toHaveBeenCalled();
	});

	it('disables future confirmation and deletes from the secondary action', async () => {
		render(GalleryPage);
		await screen.findByText('Generated image');
		await fireEvent.click(screen.getByRole('button', { name: 'Delete' }));

		await fireEvent.click(screen.getByRole('button', { name: "Don't ask again" }));

		expect(settings.saveConfirmBeforeDeletingMedia).toHaveBeenCalledWith(false);
		await waitFor(() => expect(api.deleteMedia).toHaveBeenCalledWith('media-1'));
	});

	it('deletes immediately when confirmation is disabled', async () => {
		settings.settings.app.confirmBeforeDeletingMedia = false;
		render(GalleryPage);
		await screen.findByText('Generated image');

		await fireEvent.click(screen.getByRole('button', { name: 'Delete' }));

		await waitFor(() => expect(api.deleteMedia).toHaveBeenCalledWith('media-1'));
		expect(screen.queryByRole('dialog')).toBeNull();
	});
});
