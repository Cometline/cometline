// @vitest-environment jsdom
import { fireEvent, render, screen } from '@testing-library/svelte';
import { afterAll, beforeEach, describe, expect, it, vi } from 'vitest';
import ImageAttachments from './ImageAttachments.svelte';

const originalShowModal = Object.getOwnPropertyDescriptor(HTMLDialogElement.prototype, 'showModal');
const originalClose = Object.getOwnPropertyDescriptor(HTMLDialogElement.prototype, 'close');

describe('ImageAttachments', () => {
	beforeEach(() => {
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

	it('opens the shared image preview when an attachment is clicked', async () => {
		render(ImageAttachments, {
			props: {
				images: [
					{
						id: 'image-1',
						media_type: 'image/png',
						data: 'iVBORw0KGgo=',
						name: 'sample.png'
					}
				],
				onRemove: vi.fn()
			}
		});

		await fireEvent.click(screen.getByRole('button', { name: 'View sample.png' }));

		expect(screen.getByRole('dialog', { name: 'Image preview' })).toHaveProperty('open', true);
	});
});
