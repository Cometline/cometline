// @vitest-environment jsdom
import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import TimelineEntryRowHarness from './TimelineEntryRow.harness.svelte';

describe('TimelineEntryRow', () => {
	it('renders reasoning entry via ThinkingBlock', () => {
		render(TimelineEntryRowHarness, {
			props: {
				entry: { kind: 'reasoning', segmentIndex: 0, text: 'Let me think…' }
			}
		});
		expect(screen.getByText('Let me think…')).toBeTruthy();
	});

	it('renders tool entry with tool label', () => {
		render(TimelineEntryRowHarness, {
			props: {
				entry: {
					kind: 'tool',
					tool: {
						id: 'tool-1',
						type: 'tool',
						toolName: 'read_file',
						input: '{}',
						output: 'file contents',
						pending: false
					}
				}
			}
		});
		expect(screen.getByText('read_file')).toBeTruthy();
	});
});
