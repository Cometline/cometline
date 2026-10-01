import { describe, expect, it } from 'vitest';
import { dateInputValue, usageCsvFilename, usageEventsCsv } from './usage-page-csv';

const event = {
	created_at: Date.UTC(2026, 0, 2, 3, 4, 5),
	call_kind: 'agent_step',
	model_id: 'claude "opus"',
	input_tokens: 10,
	cache_read: 2,
	cache_write: 1,
	output_tokens: 5,
	priced: true,
	estimated_usd: 0.0123456789
};

describe('usageEventsCsv', () => {
	it('quotes every cell and escapes embedded quotes', () => {
		expect(usageEventsCsv([event])).toBe(
			[
				'"Date","Kind","Model","Input","Cache read","Cache write","Output","Cost"',
				'"2026-01-02T03:04:05.000Z","agent_step","claude ""opus""","10","2","1","5","0.012346"'
			].join('\n')
		);
	});

	it('leaves the cost empty for unpriced events', () => {
		const csv = usageEventsCsv([{ ...event, priced: false }]);
		expect(csv.split('\n')[1].endsWith(',""')).toBe(true);
	});
});

describe('usageCsvFilename', () => {
	it('uses local calendar dates for the range bounds', () => {
		const from = new Date(2026, 2, 4).getTime();
		const to = new Date(2026, 10, 25).getTime();
		expect(dateInputValue(from)).toBe('2026-03-04');
		expect(usageCsvFilename(from, to)).toBe('cometline-usage-2026-03-04-2026-11-25.csv');
	});
});
