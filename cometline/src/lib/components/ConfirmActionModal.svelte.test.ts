// @vitest-environment jsdom
import { fireEvent, render, screen } from '@testing-library/svelte';
import { afterAll, beforeEach, describe, expect, it, vi } from 'vitest';
import ConfirmActionModal from './ConfirmActionModal.svelte';

const originalShowModal = Object.getOwnPropertyDescriptor(HTMLDialogElement.prototype, 'showModal');
const originalClose = Object.getOwnPropertyDescriptor(HTMLDialogElement.prototype, 'close');
const showModal = vi.fn(function (this: HTMLDialogElement) {
	this.open = true;
});

function renderModal(
	overrides: {
		onCancel?: ReturnType<typeof vi.fn>;
		onConfirm?: ReturnType<typeof vi.fn>;
		onSecondary?: ReturnType<typeof vi.fn>;
		secondaryLabel?: string;
	} = {}
) {
	const onCancel = overrides.onCancel ?? vi.fn();
	const onConfirm = overrides.onConfirm ?? vi.fn();
	return {
		onCancel,
		onConfirm,
		onSecondary: overrides.onSecondary,
		...render(ConfirmActionModal, {
			open: true,
			title: 'Terminate terminal?',
			description: 'This cannot be undone.',
			confirmLabel: 'Terminate terminal',
			secondaryLabel: overrides.secondaryLabel,
			onCancel,
			onConfirm,
			onSecondary: overrides.onSecondary
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

	it('confirms on Enter', async () => {
		const { onConfirm } = renderModal();

		await fireEvent.keyDown(window, { key: 'Enter' });

		expect(onConfirm).toHaveBeenCalledOnce();
	});

	it('shows secondary button and fires onSecondary when secondaryLabel is provided', async () => {
		const onSecondary = vi.fn();
		renderModal({ secondaryLabel: 'Always close', onSecondary });

		await fireEvent.click(screen.getByRole('button', { name: 'Always close' }));

		expect(onSecondary).toHaveBeenCalledOnce();
	});

	it('hides secondary button when secondaryLabel is omitted', () => {
		renderModal();

		expect(screen.queryByRole('button', { name: 'Always close' })).toBeNull();
		expect(screen.getByRole('button', { name: 'Cancel' })).toBeTruthy();
		expect(screen.getByRole('button', { name: 'Terminate terminal' })).toBeTruthy();
	});
});
