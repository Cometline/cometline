import { tick, untrack } from 'svelte';
import type { ChatItem } from '$lib/stores/chat.svelte';
import {
	buildScrollKey,
	offsetTopRelativeTo,
	shouldShowJumpToBottom,
	userMessageScrollTop
} from './thread-scroll';

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
	let showJumpToBottom = $state(false);
	let lastScrolledUserId: string | null = null;
	let viewportHeight = $state(0);
	let scrollFrame = 0;
	let isInitialTranscriptPaint = $state(true);

	const scrollKey = $derived(buildScrollKey(deps.getThreadItems(), deps.getSessionStreaming()));

	function setScroller(element: HTMLDivElement | undefined) {
		scroller = element;
	}

	function updateJumpToBottom() {
		if (!scroller) {
			showJumpToBottom = false;
			return;
		}
		showJumpToBottom = shouldShowJumpToBottom(scroller);
	}

	function onScroll() {
		updateJumpToBottom();
	}

	function jumpToBottom() {
		if (!scroller) return;
		scroller.scrollTo({ top: scroller.scrollHeight, behavior: 'smooth' });
		showJumpToBottom = false;
	}

	function scrollUserMessageToTop(userId: string) {
		if (!scroller) return;
		const target = scroller.querySelector<HTMLElement>(`[data-user-item-id="${userId}"]`);
		if (!target) return;
		const absoluteTop = offsetTopRelativeTo(target, scroller);
		const top = userMessageScrollTop(absoluteTop, deps.getUserMessageCount(), viewportHeight);
		scroller.scrollTo({ top, behavior: 'smooth' });
		updateJumpToBottom();
	}

	$effect(() => {
		void deps.getSessionId();
		untrack(() => {
			lastScrolledUserId = deps.getLastUserId();
			isInitialTranscriptPaint = !deps.sessionHasCachedTranscript(deps.getSessionId());
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
			updateJumpToBottom();
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
		void scrollKey;
		if (scrollFrame) cancelAnimationFrame(scrollFrame);
		scrollFrame = requestAnimationFrame(() => {
			void tick().then(() => {
				scrollFrame = 0;
				if (!scroller) return;
				if (isInitialTranscriptPaint) return;
				updateJumpToBottom();
			});
		});
		return () => {
			if (scrollFrame) cancelAnimationFrame(scrollFrame);
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
		if (isInitialTranscriptPaint) {
			lastScrolledUserId = userId;
			return;
		}
		lastScrolledUserId = userId;
		void tick().then(() => {
			requestAnimationFrame(() => scrollUserMessageToTop(userId));
		});
	});

	return {
		get showJumpToBottom() {
			return showJumpToBottom;
		},
		get viewportHeight() {
			return viewportHeight;
		},
		get isInitialTranscriptPaint() {
			return isInitialTranscriptPaint;
		},
		setScroller,
		onScroll,
		jumpToBottom
	};
}
