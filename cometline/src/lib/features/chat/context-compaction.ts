import type { ContextCompactionResult } from '#lib/generated/cometmind-api/index.js';

export type CompactionFeedback = {
	tone: 'success' | 'warning' | 'error';
	label: string;
	detail: string;
};

export function compactionFeedback(result: ContextCompactionResult): CompactionFeedback | null {
	if (result.status === 'compacted') {
		return {
			tone: 'success',
			label: 'Context compacted',
			detail: 'Earlier messages were folded into the session summary.'
		};
	}
	if (result.reason === 'cancelled') return null;
	return {
		tone: 'warning',
		label: 'Nothing to compact',
		detail: 'Recent messages already fit beside the summary.'
	};
}
