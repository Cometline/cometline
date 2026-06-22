import { describe, expect, it } from 'vitest';
import { parseJobProposal } from './parse-job-proposal';

describe('parseJobProposal', () => {
	it('parses tool input object', () => {
		const proposal = parseJobProposal(
			{ description: 'Fix auth', definition_of_done: 'Tests pass' },
			'{"status":"awaiting_workspace","default_workspace":"/tmp/ws"}'
		);
		expect(proposal).toEqual({
			description: 'Fix auth',
			definitionOfDone: 'Tests pass',
			defaultWorkspace: '/tmp/ws'
		});
	});

	it('returns null when description missing', () => {
		expect(parseJobProposal({ definition_of_done: 'x' })).toBeNull();
	});

	it('parses output-only JSON', () => {
		const proposal = parseJobProposal(null, '{"description":"Task","default_workspace":"/a"}');
		expect(proposal?.description).toBe('Task');
		expect(proposal?.defaultWorkspace).toBe('/a');
	});
});
