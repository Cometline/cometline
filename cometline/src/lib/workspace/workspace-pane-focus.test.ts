// @vitest-environment jsdom

import { describe, expect, it } from 'vitest';
import {
	isWorkspaceOwnedPane,
	resolveWorkspaceFocusTarget,
	shouldClaimChatPaneFromMainPointer
} from './workspace-pane-focus';

describe('isWorkspaceOwnedPane', () => {
	it('treats web and terminal as owned', () => {
		expect(isWorkspaceOwnedPane('web')).toBe(true);
		expect(isWorkspaceOwnedPane('terminal')).toBe(true);
		expect(isWorkspaceOwnedPane('chat')).toBe(false);
	});
});

describe('shouldClaimChatPaneFromMainPointer', () => {
	it('ignores composer context chips', () => {
		const chips = document.createElement('div');
		chips.className = 'message-context-chips';
		const chip = document.createElement('button');
		chips.append(chip);
		expect(shouldClaimChatPaneFromMainPointer(chip)).toBe(false);
		expect(shouldClaimChatPaneFromMainPointer(document.createElement('div'))).toBe(true);
	});
});

describe('resolveWorkspaceFocusTarget', () => {
	const base = {
		focusedPane: 'web',
		showContentSearch: false,
		addressEditing: false,
		isBlankTab: false,
		showWebview: false,
		showBrowseFilter: false,
		hasFilePreview: false
	};

	it('parks on the address bar for a blank or editing search tab', () => {
		expect(
			resolveWorkspaceFocusTarget({
				...base,
				showContentSearch: true,
				isBlankTab: true
			})
		).toBe('address');
		expect(
			resolveWorkspaceFocusTarget({
				...base,
				showContentSearch: true,
				addressEditing: true,
				showWebview: true
			})
		).toBe('address');
	});

	it('parks on the file/panel host when a file tab is open', () => {
		expect(resolveWorkspaceFocusTarget({ ...base, hasFilePreview: true })).toBe('panel');
	});

	it('parks on the panel host for a loaded page, not the guest webview', () => {
		expect(resolveWorkspaceFocusTarget({ ...base, showWebview: true })).toBe('panel');
	});

	it('parks on the filter when browsing a tree', () => {
		expect(resolveWorkspaceFocusTarget({ ...base, showBrowseFilter: true })).toBe('filter');
	});

	it('does not steal the filter while a file preview is up', () => {
		expect(
			resolveWorkspaceFocusTarget({
				...base,
				showBrowseFilter: true,
				hasFilePreview: true
			})
		).toBe('panel');
	});
});
