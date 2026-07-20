import { describe, expect, it } from 'vitest';
import source from './Tooltip.svelte?raw';

describe('Tooltip', () => {
	it('exposes a tooltip role and shortcut kbd slot', () => {
		expect(source).toContain('role="tooltip"');
		expect(source).toContain('<kbd>{kbd}</kbd>');
		expect(source).toContain('aria-describedby');
		expect(source).toContain('position: fixed');
		expect(source).toContain('clampTooltipPosition');
		expect(source).toContain('use:portal');
		expect(source).toContain('text-transform: none');
	});
});
