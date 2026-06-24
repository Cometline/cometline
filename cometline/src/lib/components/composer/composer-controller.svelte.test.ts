import { describe, expect, it, vi } from 'vitest';
import { createComposerInputController } from './composer-controller.svelte';

describe('createComposerInputController', () => {
	it('canSubmit is true when text or images exist', () => {
		const controller = createComposerInputController({
			onSend: vi.fn(),
			getValue: () => 'hello',
			getImages: () => [],
			getDisabled: () => false,
			getHasSelectedModel: () => true,
			clearDraft: vi.fn()
		});
		expect(controller.canSubmit()).toBe(true);
	});

	it('buildSubmitPayload returns null when disabled or no model', () => {
		const controller = createComposerInputController({
			onSend: vi.fn(),
			getValue: () => 'hello',
			getImages: () => [],
			getDisabled: () => true,
			getHasSelectedModel: () => true,
			clearDraft: vi.fn()
		});
		expect(controller.buildSubmitPayload([])).toBeNull();
	});

	it('buildSubmitPayload includes text and file paths', () => {
		const controller = createComposerInputController({
			onSend: vi.fn(),
			getValue: () => '  run tests  ',
			getImages: () => [],
			getDisabled: () => false,
			getHasSelectedModel: () => true,
			clearDraft: vi.fn()
		});
		expect(controller.buildSubmitPayload(['/tmp/a.ts'])).toEqual({
			text: 'run tests',
			filePaths: ['/tmp/a.ts']
		});
	});

	it('submitDraft sends payload and clears draft', () => {
		const onSend = vi.fn();
		const clearDraft = vi.fn();
		const controller = createComposerInputController({
			onSend,
			getValue: () => 'go',
			getImages: () => [],
			getDisabled: () => false,
			getHasSelectedModel: () => true,
			clearDraft
		});
		expect(controller.submitDraft([])).toBe(true);
		expect(onSend).toHaveBeenCalledWith({ text: 'go' });
		expect(clearDraft).toHaveBeenCalledOnce();
	});
});
