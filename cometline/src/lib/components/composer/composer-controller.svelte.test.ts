import { describe, expect, it, vi } from 'vitest';
import { createComposerInputController } from './composer-controller.svelte';

function deps(overrides: Partial<Parameters<typeof createComposerInputController>[0]> = {}) {
	return {
		onSend: vi.fn(),
		getValue: () => 'hello',
		getImages: () => [],
		getDisabled: () => false,
		getHasSelectedModel: () => true,
		getReasoningEffort: () => '',
		getReasoningEffortOptions: () => [],
		clearDraft: vi.fn(),
		...overrides
	};
}

describe('createComposerInputController', () => {
	it('canSubmit is true when text or images exist', () => {
		const controller = createComposerInputController(deps());
		expect(controller.canSubmit()).toBe(true);
	});

	it('buildSubmitPayload returns null when disabled or no model', () => {
		const controller = createComposerInputController(deps({ getDisabled: () => true }));
		expect(controller.buildSubmitPayload([])).toBeNull();
	});

	it('buildSubmitPayload includes text and file paths', () => {
		const controller = createComposerInputController(deps({ getValue: () => '  run tests  ' }));
		expect(controller.buildSubmitPayload(['/tmp/a.ts'])).toEqual({
			text: 'run tests',
			filePaths: ['/tmp/a.ts']
		});
	});

	it('buildSubmitPayload attaches the reasoning effort when set', () => {
		const controller = createComposerInputController(
			deps({
				getReasoningEffort: () => 'high',
				getReasoningEffortOptions: () => ['low', 'medium', 'high']
			})
		);
		expect(controller.buildSubmitPayload([])).toEqual({
			text: 'hello',
			reasoningEffort: 'high'
		});
	});

	it('buildSubmitPayload omits an effort unsupported by the selected model', () => {
		const controller = createComposerInputController(
			deps({
				getReasoningEffort: () => 'high',
				getReasoningEffortOptions: () => ['low', 'medium']
			})
		);
		expect(controller.buildSubmitPayload([])).toEqual({ text: 'hello' });
	});

	it('submitDraft sends payload and clears draft', () => {
		const onSend = vi.fn();
		const clearDraft = vi.fn();
		const controller = createComposerInputController(deps({ onSend, getValue: () => 'go', clearDraft }));
		expect(controller.submitDraft([])).toBe(true);
		expect(onSend).toHaveBeenCalledWith({ text: 'go' });
		expect(clearDraft).toHaveBeenCalledOnce();
	});
});
