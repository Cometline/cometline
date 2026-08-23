// @vitest-environment jsdom
import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/svelte';
import SelectionAddToChat from './SelectionAddToChat.svelte';

describe('SelectionAddToChat', () => {
	function renderPopup() {
		const onAdd = vi.fn();
		const onDismiss = vi.fn();
		render(SelectionAddToChat, {
			props: { position: { top: 20, left: 30 }, onAdd, onDismiss }
		});
		return { onAdd, onDismiss };
	}

	it('dismisses on the first pointer down outside the popup', async () => {
		const { onDismiss } = renderPopup();

		await fireEvent.pointerDown(document.body);

		expect(onDismiss).toHaveBeenCalledOnce();
	});

	it('clears the old DOM range before dismissing outside the popup', async () => {
		const { onDismiss } = renderPopup();
		const selectionHost = document.createElement('p');
		selectionHost.textContent = 'Selected response';
		document.body.append(selectionHost);
		const selection = window.getSelection();
		const range = document.createRange();
		range.selectNodeContents(selectionHost);
		selection?.removeAllRanges();
		selection?.addRange(range);

		try {
			expect(selection?.toString()).toBe('Selected response');

			await fireEvent.pointerDown(document.body);

			expect(selection?.rangeCount).toBe(0);
			expect(onDismiss).toHaveBeenCalledOnce();
		} finally {
			selection?.removeAllRanges();
			selectionHost.remove();
		}
	});

	it('keeps the popup open through its own pointer down and adds context on click', async () => {
		const { onAdd, onDismiss } = renderPopup();
		const button = screen.getByRole('button', { name: 'Add to chat' });

		await fireEvent.pointerDown(button);
		await fireEvent.click(button);

		expect(onDismiss).not.toHaveBeenCalled();
		expect(onAdd).toHaveBeenCalledOnce();
	});

	it('dismisses on Escape, scroll, and resize', async () => {
		const { onDismiss } = renderPopup();

		await fireEvent.keyDown(window, { key: 'Escape' });
		await fireEvent.scroll(document);
		await fireEvent.resize(window);

		expect(onDismiss).toHaveBeenCalledTimes(3);
	});
});
