import { describe, expect, it } from 'vitest';
import {
	miniWindowTimeoutFromInput,
	nonNegativeIntFromInput,
	positiveIntFromInput,
	screenCaptureStatusLabel
} from './general-panel-inputs';

function inputEvent(value: string): Event {
	return { currentTarget: { value } } as unknown as Event;
}

describe('general panel input parsing', () => {
	it('clamps the mini window timeout to 1..1440 minutes', () => {
		expect(miniWindowTimeoutFromInput(inputEvent('0'))).toBe(1);
		expect(miniWindowTimeoutFromInput(inputEvent('12.7'))).toBe(12);
		expect(miniWindowTimeoutFromInput(inputEvent('5000'))).toBe(1440);
		expect(miniWindowTimeoutFromInput(inputEvent('abc'))).toBe(30);
	});

	it('floors non-negative integers and falls back to 0', () => {
		expect(nonNegativeIntFromInput(inputEvent('-3'))).toBe(0);
		expect(nonNegativeIntFromInput(inputEvent('7.9'))).toBe(7);
		expect(nonNegativeIntFromInput(inputEvent('abc'))).toBe(0);
	});

	it('floors positive integers and falls back to 1', () => {
		expect(positiveIntFromInput(inputEvent('0'))).toBe(1);
		expect(positiveIntFromInput(inputEvent('24.5'))).toBe(24);
		expect(positiveIntFromInput(inputEvent('abc'))).toBe(1);
	});
});

describe('screenCaptureStatusLabel', () => {
	it('describes known and unknown statuses', () => {
		expect(screenCaptureStatusLabel('granted')).toBe('System permission: granted');
		expect(screenCaptureStatusLabel('whatever')).toBe('System permission: unknown');
	});
});
