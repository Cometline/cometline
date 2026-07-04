import { tick, untrack } from 'svelte';
import type { ChatItem } from '$lib/stores/chat.svelte';
import { activeTurnMinHeight } from './thread-turns';
import { followUpPinScrollMargin } from './thread-scroll';

export interface ThreadScrollDeps {
	getSessionId: () => string;
	getIsSessionSynced: () => boolean;
	getThreadItems: () => readonly ChatItem[];
	getSessionStreaming: () => boolean;
	getLastUserId: () => string | null;
	getUserMessageCount: () => number;
	getIsLoading: () => boolean;
	sessionHasCachedTranscript: (sessionId: string) => boolean;
}

export function createThreadScroll(deps: ThreadScrollDeps) {
	let scroller = $state<HTMLDivElement | undefined>(undefined);
	let lastScrolledUserId: string | null = null;
	let viewportHeight = $state(0);
	let isInitialTranscriptPaint = $state(true);
	/** Keeps min-height on the latest turn from send until the next user message. */
	let activeTurnCanvas = $state(false);

	const turnMinHeight = $derived.by(() =>
		activeTurnCanvas ? activeTurnMinHeight(viewportHeight) : 0
	);
	const userPinScrollMargin = $derived(followUpPinScrollMargin(viewportHeight));

	function setScroller(element: HTMLDivElement | undefined) {
		scroller = element;
	}

	function scrollUserMessageIntoView(userId: string) {
		if (!scroller) return;
		const target = scroller.querySelector<HTMLElement>(`[data-user-item-id="${userId}"]`);
		target?.scrollIntoView({ block: 'start', behavior: 'auto' });
	}

	function pinUserMessageAfterLayout(userId: string) {
		let frame = 0;
		const settle = () => {
			scrollUserMessageIntoView(userId);
			frame += 1;
			if (frame < 3) requestAnimationFrame(settle);
		};
		requestAnimationFrame(settle);
	}

	$effect(() => {
		void deps.getSessionId();
		untrack(() => {
			lastScrolledUserId = deps.getLastUserId();
			isInitialTranscriptPaint = !deps.sessionHasCachedTranscript(deps.getSessionId());
			activeTurnCanvas = false;
		});
	});

	$effect(() => {
		const sessionId = deps.getSessionId();
		const isSessionSynced = deps.getIsSessionSynced();
		const threadItems = deps.getThreadItems();
		const isLoading = deps.getIsLoading();

		if (!isSessionSynced) {
			isInitialTranscriptPaint = !deps.sessionHasCachedTranscript(sessionId);
			return;
		}
		if (isLoading && threadItems.length === 0) {
			isInitialTranscriptPaint = true;
			return;
		}
		if (threadItems.length === 0) {
			isInitialTranscriptPaint = true;
			return;
		}

		if (!isInitialTranscriptPaint) return;

		let cancelled = false;
		let settleFrame = 0;
		let lastHeight = 0;
		let stableFrames = 0;
		let frameCount = 0;

		const finishHydration = () => {
			if (cancelled) return;
			if (scroller) scroller.scrollTop = scroller.scrollHeight;
			isInitialTranscriptPaint = false;
		};

		const settle = () => {
			if (cancelled) return;
			if (!scroller) {
				settleFrame = requestAnimationFrame(settle);
				return;
			}
			scroller.scrollTop = scroller.scrollHeight;
			const height = scroller.scrollHeight;
			if (height === lastHeight) stableFrames += 1;
			else {
				stableFrames = 0;
				lastHeight = height;
			}
			frameCount += 1;
			if (stableFrames >= 2 || frameCount >= 12) {
				finishHydration();
				return;
			}
			settleFrame = requestAnimationFrame(settle);
		};

		void tick().then(() => {
			if (cancelled) return;
			settleFrame = requestAnimationFrame(settle);
		});

		return () => {
			cancelled = true;
			if (settleFrame) cancelAnimationFrame(settleFrame);
		};
	});

	$effect(() => {
		if (!scroller) return;
		viewportHeight = scroller.clientHeight;
		if (typeof ResizeObserver === 'undefined') return;
		const observer = new ResizeObserver(() => {
			if (scroller) viewportHeight = scroller.clientHeight;
		});
		observer.observe(scroller);
		return () => observer.disconnect();
	});

	$effect(() => {
		const userId = deps.getLastUserId();
		if (!userId) {
			lastScrolledUserId = null;
			return;
		}
		if (userId === lastScrolledUserId) return;
		lastScrolledUserId = userId;
		if (isInitialTranscriptPaint || deps.getUserMessageCount() <= 1) return;
		activeTurnCanvas = true;
		void tick().then(() => {
			pinUserMessageAfterLayout(userId);
		});
	});

	return {
		get activeTurnMinHeight() {
			return turnMinHeight;
		},
		get userPinScrollMargin() {
			return userPinScrollMargin;
		},
		get viewportHeight() {
			return viewportHeight;
		},
		get isInitialTranscriptPaint() {
			return isInitialTranscriptPaint;
		},
		setScroller
	};
}
