import {
	clampWorkspacePanelWidth,
	resolveWorkspacePanelRatio,
	widthFromRatio,
	widthToRatio
} from '#lib/layout/workspace-panel-width.js';
import { settingsStore } from '#lib/stores/settings.svelte.js';
import { shellStore } from '#lib/stores/shell.svelte.js';

const FALLBACK_SIDEBAR_DURATION = 360;

export function createAppShellLayout(deps: {
	getContentRow: () => HTMLDivElement | null;
	getIsUtilityPage: () => boolean;
}) {
	let resizing = $state(false);
	let sidebarAnimating = $state(false);
	let ratioFrame = 0;
	let resizeStartX = 0;
	let resizeStartWidth = 0;

	const panelSizePrefs = $derived({
		workspacePanelRatio: settingsStore.settings.app.workspacePanelRatio,
		workspacePanelWidth: settingsStore.settings.app.workspacePanelWidth
	});
	let preferredRatio = $derived(resolveWorkspacePanelRatio(panelSizePrefs, contentRowWidth()));

	function parseDuration(value: string) {
		const trimmed = value.trim();
		if (!trimmed) return FALLBACK_SIDEBAR_DURATION;
		if (trimmed.endsWith('ms'))
			return Number(trimmed.slice(0, -2)) || FALLBACK_SIDEBAR_DURATION;
		if (trimmed.endsWith('s')) return (Number(trimmed.slice(0, -1)) || 0) * 1000;
		return Number(trimmed) || FALLBACK_SIDEBAR_DURATION;
	}

	function sidebarTransitionDuration() {
		if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return 0;
		return parseDuration(
			getComputedStyle(document.documentElement).getPropertyValue('--duration-sidebar')
		);
	}

	function panelChrome(sidebarOpen = shellStore.sidebarOpen) {
		return {
			sidebarOpen: sidebarOpen || deps.getIsUtilityPage(),
			fullscreen: shellStore.fullscreen
		};
	}

	function contentRowWidth() {
		return deps.getContentRow()?.clientWidth ?? window.innerWidth;
	}

	function sidebarWidthPx() {
		const raw = getComputedStyle(document.documentElement)
			.getPropertyValue('--sidebar-width')
			.trim();
		const px = Number.parseFloat(raw);
		return Number.isFinite(px) ? px : 250;
	}

	function contentRowWidthAfterSidebarToggle(open: boolean) {
		const shellW =
			deps.getContentRow()?.parentElement?.clientWidth ??
			document.querySelector<HTMLElement>('.app-shell')?.clientWidth ??
			window.innerWidth;
		const narrow = window.matchMedia('(max-width: 900px)').matches;
		const side = !narrow && open ? sidebarWidthPx() : 0;
		return Math.max(0, shellW - side);
	}

	function currentPanelWidth() {
		const raw = getComputedStyle(document.documentElement)
			.getPropertyValue('--workspace-panel-width')
			.trim();
		const px = Number.parseFloat(raw);
		if (raw.endsWith('px') && Number.isFinite(px)) return px;
		const inner = document.querySelector<HTMLElement>('.workspace-panel-inner');
		if (inner) return inner.getBoundingClientRect().width;
		return Math.round(window.innerWidth * 0.5);
	}

	function applyWidthFromPreferredRatio() {
		const next = widthFromRatio(preferredRatio, contentRowWidth(), panelChrome());
		document.documentElement.style.setProperty('--workspace-panel-width', `${next}px`);
		return next;
	}

	function applyWidthForSidebarEndState(open: boolean) {
		const endRow = contentRowWidthAfterSidebarToggle(open);
		const next = widthFromRatio(preferredRatio, endRow, panelChrome(open));
		document.documentElement.style.setProperty('--workspace-panel-width', `${next}px`);
		return next;
	}

	function setPanelWidthPx(width: number) {
		const display = clampWorkspacePanelWidth(width, contentRowWidth(), panelChrome());
		document.documentElement.style.setProperty('--workspace-panel-width', `${display}px`);
		return display;
	}

	function applyPreferredRatioToLayout() {
		if (!shellStore.workspacePanelOpen || resizing || sidebarAnimating) return;
		if (ratioFrame) return;
		ratioFrame = requestAnimationFrame(() => {
			ratioFrame = 0;
			if (!shellStore.workspacePanelOpen || resizing || sidebarAnimating) return;
			applyWidthFromPreferredRatio();
		});
	}

	function onResizePointerDown(event: PointerEvent) {
		if (event.button !== 0) return;
		event.preventDefault();
		resizing = true;
		resizeStartX = event.clientX;
		resizeStartWidth = currentPanelWidth();
		(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
		document.body.classList.add('panel-resizing');
	}

	function onResizePointerMove(event: PointerEvent) {
		if (!resizing) return;
		const raw = resizeStartWidth - (event.clientX - resizeStartX);
		setPanelWidthPx(raw);
	}

	function endResize(event: PointerEvent) {
		if (!resizing) return;
		resizing = false;
		const target = event.currentTarget as HTMLElement;
		if (target.hasPointerCapture(event.pointerId)) {
			target.releasePointerCapture(event.pointerId);
		}
		document.body.classList.remove('panel-resizing');
		const width = currentPanelWidth();
		preferredRatio = widthToRatio(width, contentRowWidth());
		void settingsStore.saveWorkspacePanelLayout(width, preferredRatio);
	}

	function onResizeKeydown(event: KeyboardEvent) {
		const step = event.shiftKey ? 64 : 16;
		let next: number | null = null;
		if (event.key === 'ArrowLeft') next = currentPanelWidth() + step;
		else if (event.key === 'ArrowRight') next = currentPanelWidth() - step;
		if (next === null) return;
		event.preventDefault();
		const width = setPanelWidthPx(next);
		preferredRatio = widthToRatio(width, contentRowWidth());
		void settingsStore.saveWorkspacePanelLayout(width, preferredRatio);
	}

	$effect(() => {
		window.electronAPI?.setSidebarOpen?.({
			open: shellStore.sidebarOpen,
			duration: sidebarTransitionDuration()
		});
	});

	$effect(() => {
		void shellStore.sidebarOpen;
		if (typeof document === 'undefined') return;
		sidebarAnimating = true;
		document.body.classList.add('sidebar-animating');
		const duration = sidebarTransitionDuration();
		if (shellStore.workspacePanelOpen && !resizing) {
			applyWidthForSidebarEndState(shellStore.sidebarOpen);
		}
		const timeout = window.setTimeout(() => {
			sidebarAnimating = false;
			document.body.classList.remove('sidebar-animating');
			if (shellStore.workspacePanelOpen && !resizing) {
				applyWidthFromPreferredRatio();
			}
		}, duration);
		return () => {
			window.clearTimeout(timeout);
			sidebarAnimating = false;
			document.body.classList.remove('sidebar-animating');
		};
	});

	$effect(() => {
		void shellStore.fullscreen;
		void shellStore.workspacePanelOpen;
		void preferredRatio;
		if (!shellStore.workspacePanelOpen) return;
		queueMicrotask(() => {
			if (!shellStore.workspacePanelOpen || resizing || sidebarAnimating) return;
			applyWidthFromPreferredRatio();
		});
	});

	function install() {
		let resizeObserver: ResizeObserver | null = null;
		function updateFullScreen(isFullScreen: boolean) {
			shellStore.setFullscreen(isFullScreen);
		}
		void window.electronAPI?.getFullScreen?.().then(updateFullScreen);
		const unsubscribeFullScreen = window.electronAPI?.onFullScreenChange?.(updateFullScreen);
		function onDomFullScreenChange() {
			updateFullScreen(Boolean(document.fullscreenElement));
		}
		document.addEventListener('fullscreenchange', onDomFullScreenChange);
		function onWindowResize() {
			applyPreferredRatioToLayout();
		}
		window.addEventListener('resize', onWindowResize);
		const row = deps.getContentRow();
		if (row) {
			resizeObserver = new ResizeObserver(() => {
				applyPreferredRatioToLayout();
			});
			resizeObserver.observe(row);
		}
		return () => {
			unsubscribeFullScreen?.();
			document.removeEventListener('fullscreenchange', onDomFullScreenChange);
			window.removeEventListener('resize', onWindowResize);
			if (ratioFrame) cancelAnimationFrame(ratioFrame);
			ratioFrame = 0;
			resizeObserver?.disconnect();
		};
	}

	return {
		get resizing() {
			return resizing;
		},
		onResizePointerDown,
		onResizePointerMove,
		endResize,
		onResizeKeydown,
		install
	};
}
