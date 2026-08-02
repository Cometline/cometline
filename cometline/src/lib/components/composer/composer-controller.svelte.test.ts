import { describe, expect, it, vi } from 'vitest';
import type { AgentMode } from '$lib/types';
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
		getAgentMode: (): AgentMode => 'auto',
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
			filePaths: ['/tmp/a.ts'],
			agentMode: 'auto'
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
			reasoningEffort: 'high',
			agentMode: 'auto'
		});
	});

	it('buildSubmitPayload omits an effort unsupported by the selected model', () => {
		const controller = createComposerInputController(
			deps({
				getReasoningEffort: () => 'high',
				getReasoningEffortOptions: () => ['low', 'medium']
			})
		);
		expect(controller.buildSubmitPayload([])).toEqual({ text: 'hello', agentMode: 'auto' });
	});

	it('buildSubmitPayload attaches plan mode when active', () => {
		const controller = createComposerInputController(deps({ getAgentMode: () => 'plan' }));
		expect(controller.buildSubmitPayload([])).toEqual({ text: 'hello', agentMode: 'plan' });
	});

	it('submitDraft sends payload and clears draft', () => {
		const onSend = vi.fn();
		const clearDraft = vi.fn();
		const controller = createComposerInputController(deps({ onSend, getValue: () => 'go', clearDraft }));
		expect(controller.submitDraft([])).toBe(true);
		expect(onSend).toHaveBeenCalledWith({ text: 'go', agentMode: 'auto' });
		expect(clearDraft).toHaveBeenCalledOnce();
	});

	it('sendTurn attaches the active agent mode to string payloads', () => {
		const onSend = vi.fn();
		const controller = createComposerInputController(deps({ onSend, getAgentMode: () => 'plan' }));
		controller.sendTurn('go');
		expect(onSend).toHaveBeenCalledWith({ text: 'go', agentMode: 'plan' });
	});

	it('sendTurn keeps an explicit payload agent mode over the composer state', () => {
		const onSend = vi.fn();
		const controller = createComposerInputController(deps({ onSend, getAgentMode: () => 'auto' }));
		controller.sendTurn({ text: 'go', agentMode: 'plan' });
		expect(onSend).toHaveBeenCalledWith({ text: 'go', agentMode: 'plan' });
	});
});
