import { tick, untrack } from 'svelte';
import type { ChatItem } from '$lib/stores/chat.svelte';
import { activeTurnMinHeight } from './thread-turns';
import { buildScrollKey, followUpPinScrollMargin, shouldShowJumpToBottom } from './thread-scroll';

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
	let lastSessionId: string | null = null;
	let lastScrolledUserId: string | null = null;
	let viewportHeight = $state(0);
	let scrollFrame = 0;
	let scrollScheduleVersion = 0;
	let isInitialTranscriptPaint = $state(true);
	/** The follow-up user row pinned while its response is in flight. */
	let activePinnedUserId = $state<string | null>(null);

	const scrollKey = $derived(buildScrollKey(deps.getThreadItems(), deps.getSessionStreaming()));
	const turnMinHeight = $derived.by(() =>
		activePinnedUserId ? activeTurnMinHeight(viewportHeight) : 0
	);
	const userPinScrollMargin = $derived(followUpPinScrollMargin(viewportHeight));

	function setScroller(element: HTMLDivElement | undefined) {
		scroller = element;
	}

	function latestSentinel() {
		return scroller?.querySelector<HTMLElement>('[data-thread-latest-sentinel]') ?? null;
	}

	function updateJumpToBottom() {
		if (!scroller) {
			showJumpToBottom = false;
			return;
		}
		showJumpToBottom = shouldShowJumpToBottom(scroller, latestSentinel());
	}

	function onScroll() {
		updateJumpToBottom();
	}

	function cancelScheduledScrollUpdate() {
		scrollScheduleVersion += 1;
		if (scrollFrame) {
			cancelAnimationFrame(scrollFrame);
			scrollFrame = 0;
		}
	}

	function scheduleScrollUpdate() {
		cancelScheduledScrollUpdate();
		const version = scrollScheduleVersion;
		scrollFrame = requestAnimationFrame(() => {
			void tick().then(() => {
				if (version !== scrollScheduleVersion) return;
				scrollFrame = 0;
				if (!scroller || isInitialTranscriptPaint) return;
				updateJumpToBottom();
			});
		});
	}

	function jumpToBottom() {
		const latest = latestSentinel();
		if (latest) {
			latest.scrollIntoView({ block: 'end', behavior: 'smooth' });
			showJumpToBottom = false;
			return;
		}
		if (!scroller) return;
		scroller.scrollTo({ top: scroller.scrollHeight, behavior: 'smooth' });
		showJumpToBottom = false;
	}

	function scrollUserMessageIntoView(userId: string) {
		if (!scroller) return;
		const target = scroller.querySelector<HTMLElement>(`[data-user-item-id="${userId}"]`);
		target?.scrollIntoView({ block: 'start', behavior: 'auto' });
		updateJumpToBottom();
	}

	function pinUserMessageAfterLayout(userId: string) {
		let frame = 0;
		const settle = () => {
			if (activePinnedUserId !== userId || !deps.getSessionStreaming()) return;
			scrollUserMessageIntoView(userId);
			frame += 1;
			if (frame < 3) requestAnimationFrame(settle);
		};
		requestAnimationFrame(settle);
	}

	$effect(() => {
		const sessionId = deps.getSessionId();
		if (sessionId === lastSessionId) return;
		lastSessionId = sessionId;
		untrack(() => {
			lastScrolledUserId = deps.getLastUserId();
			isInitialTranscriptPaint = true;
			activePinnedUserId = null;
			showJumpToBottom = false;
		});
	});

	$effect.pre(() => {
		const streaming = deps.getSessionStreaming();
		if (streaming) return;
		untrack(() => {
			activePinnedUserId = null;
		});
	});

	$effect(() => {
		const isSessionSynced = deps.getIsSessionSynced();
		const threadItems = deps.getThreadItems();
		const isLoading = deps.getIsLoading();

		if (!isSessionSynced) {
			isInitialTranscriptPaint = true;
			return;
		}
		if (isLoading && threadItems.length === 0) {
			isInitialTranscriptPaint = true;
			return;
		}
		if (threadItems.length === 0) {
			// Once an empty transcript is fully synchronized, the next rows are a
			// live first turn rather than historical content being hydrated. This
			// distinction matters when /clear empties the current session without
			// changing its id: keeping the hydration flag set would hide the next
			// user flight and assistant handoff behind the transcript paint state.
			isInitialTranscriptPaint = false;
			lastScrolledUserId = null;
			activePinnedUserId = null;
			showJumpToBottom = false;
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
		scheduleScrollUpdate();
		return cancelScheduledScrollUpdate;
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
		if (!deps.getSessionStreaming()) return;
		if (isInitialTranscriptPaint || deps.getUserMessageCount() <= 1) return;
		activePinnedUserId = userId;
		void tick().then(() => {
			pinUserMessageAfterLayout(userId);
		});
	});

	return {
		get showJumpToBottom() {
			return showJumpToBottom;
		},
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
		setScroller,
		onScroll,
		jumpToBottom
	};
}
