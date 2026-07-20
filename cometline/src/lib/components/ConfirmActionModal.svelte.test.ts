// @vitest-environment jsdom
import { fireEvent, render, screen } from '@testing-library/svelte';
import { afterAll, beforeEach, describe, expect, it, vi } from 'vitest';
import ConfirmActionModal from './ConfirmActionModal.svelte';

const originalShowModal = Object.getOwnPropertyDescriptor(HTMLDialogElement.prototype, 'showModal');
const originalClose = Object.getOwnPropertyDescriptor(HTMLDialogElement.prototype, 'close');
const showModal = vi.fn(function (this: HTMLDialogElement) {
	this.open = true;
});

function renderModal(onCancel = vi.fn()) {
	return {
		onCancel,
		...render(ConfirmActionModal, {
			open: true,
			title: 'Terminate terminal?',
			description: 'This cannot be undone.',
			confirmLabel: 'Terminate terminal',
			onCancel,
			onConfirm: vi.fn()
		})
	};
}

describe('ConfirmActionModal', () => {
	beforeEach(() => {
		showModal.mockClear();
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

	it('opens as a native dialog', () => {

		renderModal();

		expect(showModal).toHaveBeenCalledOnce();
		expect(screen.getByRole('dialog')).toHaveProperty('open', true);
	});

	it('cancels when the native dialog is dismissed', async () => {
		const { onCancel } = renderModal();

		await fireEvent(screen.getByRole('dialog'), new Event('cancel', { cancelable: true }));

		expect(onCancel).toHaveBeenCalledOnce();
	});
});
