import { shellStore } from '$lib/stores/shell.svelte';
import {
	isWorkspaceOwnedPane,
	resolveWorkspaceFocusTarget
} from '$lib/features/workspace/workspace-pane-focus';
import { isBlankTabUrl } from '$lib/features/workspace/workspace-panel-state';
import type { WorkspacePanelView } from '$lib/features/workspace/workspace-panel-view.svelte';

export function createWorkspacePanelFocus(view: WorkspacePanelView) {
	let addressInputEl = $state<HTMLInputElement | null>(null);
	let panelFocusEl = $state<HTMLDivElement | null>(null);
	let fileTreeFilterInputEl = $state<HTMLInputElement | null>(null);
	let satisfiedFilterFocusRequestId = 0;

	function applyOwnedFocus() {
		if (!view.panelOpen || !isWorkspaceOwnedPane(shellStore.focusedPane)) return;
		const target = resolveWorkspaceFocusTarget({
			focusedPane: shellStore.focusedPane,
			showContentSearch: view.showContentSearch,
			addressEditing: view.addressEditing,
			isBlankTab: isBlankTabUrl(view.webSearchUrl),
			showWebview: view.showWebview,
			showBrowseFilter: view.showBrowseFilter,
			hasFilePreview: view.showFilePreview
		});
		if (target === 'address' && addressInputEl) {
			addressInputEl.focus({ preventScroll: true });
			return;
		}
		if (target === 'filter' && fileTreeFilterInputEl) {
			fileTreeFilterInputEl.focus({ preventScroll: true });
			return;
		}
		panelFocusEl?.focus({ preventScroll: true });
	}

	// Tracks the focus request id we have already satisfied, so a remounting
	// input (or a late effect run) doesn't refocus twice and a brand-new request
	// always wins regardless of which path observes it first.
	let satisfiedFocusRequestId = 0;

	function applyAddressFocus(force = false) {
		const requestId = shellStore.addressBarFocusRequestId;
		if (!requestId || (!force && requestId === satisfiedFocusRequestId)) return;
		if (!shellStore.workspacePanelOpen) return;
		const el = addressInputEl;
		if (!el) return;
		satisfiedFocusRequestId = requestId;
		shellStore.setFocusedPane('web');
		el.focus({ preventScroll: true });
		el.select();
	}

	function trackAddressInput(node: HTMLInputElement) {
		addressInputEl = node;
		// The input may mount *after* a focus request was issued (panel reopen
		// rebuilds the URL field). Focus straight from mount so no request is lost.
		applyAddressFocus();
		return {
			destroy() {
				if (addressInputEl === node) addressInputEl = null;
			}
		};
	}

	function applyFileTreeFilterFocus(force = false) {
		const requestId = shellStore.fileTreeFilterFocusRequestId;
		if (!requestId || (!force && requestId === satisfiedFilterFocusRequestId)) return;
		if (!shellStore.workspacePanelOpen || !view.showBrowseFilter) return;
		const el = fileTreeFilterInputEl;
		if (!el) return;
		satisfiedFilterFocusRequestId = requestId;
		shellStore.setFocusedPane('web');
		el.focus({ preventScroll: true });
		el.select();
	}

	function trackFileTreeFilterInput(node: HTMLInputElement) {
		fileTreeFilterInputEl = node;
		applyFileTreeFilterFocus();
		return {
			destroy() {
				if (fileTreeFilterInputEl === node) fileTreeFilterInputEl = null;
			}
		};
	}

	return {
		get addressInputEl() {
			return addressInputEl;
		},
		get fileTreeFilterInputEl() {
			return fileTreeFilterInputEl;
		},
		get panelFocusEl() {
			return panelFocusEl;
		},
		set panelFocusEl(next: HTMLDivElement | null) {
			panelFocusEl = next;
		},
		applyOwnedFocus,
		applyAddressFocus,
		applyFileTreeFilterFocus,
		trackAddressInput,
		trackFileTreeFilterInput
	};
}
