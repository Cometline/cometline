import { describe, expect, it } from 'vitest';
import {
	buildCronExpression,
	cronDisplayLabel,
	localDatetimeToMillis,
	millisToLocalDatetime,
	parseSupportedCron,
	summarizeCronParts
} from './jobs-page-cron';

describe('parseSupportedCron', () => {
	it('parses daily, weekly, and monthly schedules', () => {
		expect(parseSupportedCron('5 9 * * *')).toEqual({ frequency: 'daily', time: '09:05' });
		expect(parseSupportedCron('0 18 * * 3')).toEqual({
			frequency: 'weekly',
			time: '18:00',
			weekday: '3'
		});
		expect(parseSupportedCron('30 7 31 * *')).toEqual({
			frequency: 'monthly',
			time: '07:30',
			monthDay: '28'
		});
	});

	it('rejects expressions the picker cannot represent', () => {
		expect(parseSupportedCron('*/5 * * * *')).toBeNull();
		expect(parseSupportedCron('0 9 * 1 *')).toBeNull();
		expect(parseSupportedCron('0 9 1 * 1')).toBeNull();
		expect(parseSupportedCron('0 24 * * *')).toBeNull();
		expect(parseSupportedCron('0 9 * *')).toBeNull();
	});
});

describe('buildCronExpression', () => {
	it('builds expressions per frequency and clamps the time', () => {
		expect(buildCronExpression('09:05', 'daily', '1', '1')).toBe('5 9 * * *');
		expect(buildCronExpression('18:00', 'weekly', '3', '1')).toBe('0 18 * * 3');
		expect(buildCronExpression('07:30', 'monthly', '1', '15')).toBe('30 7 15 * *');
		expect(buildCronExpression('99:99', 'daily', '1', '1')).toBe('59 23 * * *');
	});
});

describe('summarizeCronParts', () => {
	it('describes each frequency', () => {
		expect(summarizeCronParts('daily', '', '1', '1')).toBe('Every day at 09:00');
		expect(summarizeCronParts('weekly', '10:00', '0', '1')).toBe('Every Sunday at 10:00');
		expect(summarizeCronParts('monthly', '10:00', '1', '12')).toBe(
			'Every month on day 12 at 10:00'
		);
	});
});

describe('cronDisplayLabel', () => {
	it('summarizes supported expressions and falls back to the raw cron', () => {
		expect(cronDisplayLabel('0 9 * * 1')).toBe('Every Monday at 09:00');
		expect(cronDisplayLabel('*/5 * * * *')).toBe('cron: */5 * * * *');
	});
});

describe('local datetime conversion', () => {
	it('round-trips through local datetime strings', () => {
		const ms = localDatetimeToMillis('2026-03-04T05:06');
		expect(ms).toBeTypeOf('number');
		expect(millisToLocalDatetime(ms)).toBe('2026-03-04T05:06');
	});

	it('handles empty and invalid input', () => {
		expect(localDatetimeToMillis('')).toBeUndefined();
		expect(localDatetimeToMillis('not a date')).toBeUndefined();
		expect(millisToLocalDatetime(undefined)).toBe('');
	});
});
