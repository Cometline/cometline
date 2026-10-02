import { tick, untrack } from 'svelte';
import type { ChatItem } from '#lib/stores/chat.svelte.js';
import { activeTurnMinHeight, transcriptHasLeadingOrphans } from './thread-turns';
import { buildScrollKey, followUpPinScrollMargin, shouldShowJumpToBottom } from './thread-scroll';
import { THREAD_HYDRATION_FAILSAFE_MS, scrollTopAfterPrepend } from './thread-virtualizer';

const LOAD_OLDER_TOP_PX = 320;
/** Max older pages to auto-fetch after hydration to re-anchor a mid-turn first page. */
const AUTO_ANCHOR_PAGE_CAP = 5;

export interface ThreadScrollDeps {
	getSessionId: () => string;
	getIsSessionSynced: () => boolean;
	getThreadItems: () => readonly ChatItem[];
	getSessionStreaming: () => boolean;
	getLastUserId: () => string | null;
	getUserMessageCount: () => number;
	getIsLoading: () => boolean;
	sessionHasCachedTranscript: (sessionId: string) => boolean;
	/** Optional — when provided, the controller tracks the bound scroller. */
	getScroller?: () => HTMLDivElement | undefined;
	/** Optional keyset pagination — near-top prepend into the same chat store. */
	getHasMoreHistory?: () => boolean;
	getIsLoadingOlder?: () => boolean;
	loadOlderTranscript?: (sessionId: string) => Promise<number>;
	/** Keep virtualization / other listeners aligned with programmatic scrollTop. */
	onScrollTopChange?: (top: number) => void;
}

export function createThreadScroll(deps: ThreadScrollDeps) {
	let scroller = $state<HTMLDivElement | undefined>(undefined);
	let showJumpToBottom = $state(false);
	let lastSessionId: string | null = null;
	/**
	 * One bump per session-id change, including the first bind.
	 * Virtual and fold subscribe to this instead of re-deriving "new session".
	 * Written only inside untrack so this module's other effects do not depend on it.
	 */
	let sessionEpoch = $state(0);
	/** Successfully presented follow-up user id (only advanced when pin runs). */
	let lastScrolledUserId: string | null = null;
	let viewportHeight = $state(0);
	let scrollFrame = 0;
	let scrollScheduleVersion = 0;
	let isInitialTranscriptPaint = $state(true);
	let sessionHadTranscript = false;
	/** The follow-up user row pinned while its reply canvas is active. */
	let activePinnedUserId = $state<string | null>(null);
	/** Covers the post-load tick + scrollTop correction window (store flag clears earlier). */
	let loadOlderInFlight = false;
	/** Auto-anchor (leading orphan → loadOlder) in flight; shared with scroll-near-top gate. */
	let autoAnchorInFlight = false;
	/** Pages already auto-anchored for the current session (reset on session change). */
	let autoAnchorPagesUsed = 0;
	/**
	 * Wall-clock escape from opacity:0. Lives outside the settle $effect so
	 * threadItems / sync churn cannot cancel and re-arm it (#157 miss).
	 */
	let hydrationFailsafeTimer: ReturnType<typeof setTimeout> | 0 = 0;

	const scrollKey = $derived(buildScrollKey(deps.getThreadItems(), deps.getSessionStreaming()));
	const turnMinHeight = $derived.by(() =>
		activePinnedUserId ? activeTurnMinHeight(viewportHeight) : 0
	);
	const userPinScrollMargin = $derived(followUpPinScrollMargin(viewportHeight));

	function notifyScrollTop() {
		if (!scroller) return;
		deps.onScrollTopChange?.(scroller.scrollTop);
	}

	function setScroller(element: HTMLDivElement | undefined) {
		scroller = element;
		if (element) deps.onScrollTopChange?.(element.scrollTop);
	}

	$effect(() => {
		const getScroller = deps.getScroller;
		if (!getScroller) return;
		const element = getScroller();
		scroller = element;
		if (element) deps.onScrollTopChange?.(element.scrollTop);
	});

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

	async function maybeLoadOlderHistory() {
		const loadOlder = deps.loadOlderTranscript;
		if (!loadOlder) return;
		if (loadOlderInFlight || isInitialTranscriptPaint) return;
		if (!deps.getIsSessionSynced()) return;
		if (!deps.getHasMoreHistory?.() || deps.getIsLoadingOlder?.()) return;
		const el = scroller;
		if (!el || el.scrollTop > LOAD_OLDER_TOP_PX) return;
		loadOlderInFlight = true;
		const prevTop = el.scrollTop;
		const prevHeight = el.scrollHeight;
		try {
			const prepended = await loadOlder(deps.getSessionId());
			if (!prepended || !scroller) return;
			// Flush derived virtual spacer height before correcting scrollTop.
			await tick();
			if (!scroller) return;
			const nextHeight = scroller.scrollHeight;
			scroller.scrollTop = scrollTopAfterPrepend(prevTop, prevHeight, nextHeight);
			notifyScrollTop();
		} finally {
			loadOlderInFlight = false;
		}
	}

	/**
	 * After first transcript page apply / hydration, if hasMore and the page starts
	 * mid-turn (leading non-user orphans), keep loading older pages until an
	 * anchoring user appears or AUTO_ANCHOR_PAGE_CAP is hit. Not scroll-gated.
	 */
	async function maybeAutoAnchorLeadingOrphans() {
		const loadOlder = deps.loadOlderTranscript;
		if (!loadOlder) return;
		if (autoAnchorInFlight || loadOlderInFlight || isInitialTranscriptPaint) return;
		if (!deps.getIsSessionSynced()) return;
		if (!deps.getHasMoreHistory?.() || deps.getIsLoadingOlder?.()) return;
		if (autoAnchorPagesUsed >= AUTO_ANCHOR_PAGE_CAP) return;
		if (!transcriptHasLeadingOrphans(deps.getThreadItems())) return;

		autoAnchorInFlight = true;
		loadOlderInFlight = true;
		const el = scroller;
		const prevTop = el?.scrollTop ?? 0;
		const prevHeight = el?.scrollHeight ?? 0;
		try {
			while (
				autoAnchorPagesUsed < AUTO_ANCHOR_PAGE_CAP &&
				deps.getHasMoreHistory?.() &&
				!deps.getIsLoadingOlder?.() &&
				transcriptHasLeadingOrphans(deps.getThreadItems())
			) {
				const prepended = await loadOlder(deps.getSessionId());
				autoAnchorPagesUsed += 1;
				if (!prepended) break;
			}
			if (el && scroller === el && prevHeight > 0) {
				await tick();
				if (!scroller) return;
				const nextHeight = scroller.scrollHeight;
				scroller.scrollTop = scrollTopAfterPrepend(prevTop, prevHeight, nextHeight);
				notifyScrollTop();
			}
		} finally {
			autoAnchorInFlight = false;
			loadOlderInFlight = false;
		}
	}

	function onScroll() {
		updateJumpToBottom();
		notifyScrollTop();
		void maybeLoadOlderHistory();
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
			if (activePinnedUserId !== userId) return;
			scrollUserMessageIntoView(userId);
			frame += 1;
			if (frame < 3) requestAnimationFrame(settle);
		};
		requestAnimationFrame(settle);
	}

	function presentFollowUpTurn(userId: string) {
		activePinnedUserId = userId;
		lastScrolledUserId = userId;
		void tick().then(() => {
			pinUserMessageAfterLayout(userId);
		});
	}

	function clearHydrationFailsafe() {
		if (hydrationFailsafeTimer) {
			clearTimeout(hydrationFailsafeTimer);
			hydrationFailsafeTimer = 0;
		}
	}

	/** Reveal the thread: clear opacity:0 / pointer-events:none. */
	function finishHydrationPaint() {
		clearHydrationFailsafe();
		if (scroller) {
			scroller.scrollTop = scroller.scrollHeight;
			notifyScrollTop();
		}
		isInitialTranscriptPaint = false;
		updateJumpToBottom();
		// Post-hydration kick: re-anchor mid-turn first page (not scroll-only).
		void maybeAutoAnchorLeadingOrphans();
	}

	/**
	 * Arm (or keep) the wall-clock failsafe. Idempotent while already armed so
	 * settle-effect restarts from item churn cannot reset the deadline.
	 */
	function armHydrationFailsafe() {
		if (hydrationFailsafeTimer) return;
		hydrationFailsafeTimer = setTimeout(() => {
			hydrationFailsafeTimer = 0;
			if (!isInitialTranscriptPaint) return;
			finishHydrationPaint();
		}, THREAD_HYDRATION_FAILSAFE_MS);
	}

	function beginHydrationPaint() {
		isInitialTranscriptPaint = true;
		// Idempotent arm: sync/loading flaps must not reset the wall-clock deadline.
		armHydrationFailsafe();
	}

	/** New session (or remount): reset the failsafe deadline for this hydration episode. */
	function beginHydrationPaintFresh() {
		clearHydrationFailsafe();
		isInitialTranscriptPaint = true;
		armHydrationFailsafe();
	}

	$effect(() => {
		const sessionId = deps.getSessionId();
		if (sessionId === lastSessionId) return;
		lastSessionId = sessionId;
		untrack(() => {
			sessionEpoch += 1;
			sessionHadTranscript = deps.sessionHasCachedTranscript(sessionId);
			lastScrolledUserId = deps.getLastUserId();
			beginHydrationPaintFresh();
			activePinnedUserId = null;
			showJumpToBottom = false;
			autoAnchorPagesUsed = 0;
		});
	});

	$effect(() => {
		const isSessionSynced = deps.getIsSessionSynced();
		const threadItems = deps.getThreadItems();
		const isLoading = deps.getIsLoading();

		if (!isSessionSynced) {
			beginHydrationPaint();
			return;
		}
		if (isLoading && threadItems.length === 0) {
			beginHydrationPaint();
			return;
		}
		if (threadItems.length === 0) {
			// Once an empty transcript is fully synchronized, the next rows are a
			// live first turn rather than historical content being hydrated. This
			// distinction matters when /clear empties the current session without
			// changing its id: keeping the hydration flag set would hide the next
			// user flight and assistant handoff behind the transcript paint state.
			clearHydrationFailsafe();
			isInitialTranscriptPaint = false;
			lastScrolledUserId = null;
			activePinnedUserId = null;
			showJumpToBottom = false;
			return;
		}
		if (!sessionHadTranscript && deps.getSessionStreaming()) {
			// A newly-created session can receive its first live turn before its
			// empty transcript request resolves. Do not treat that turn as history.
			// Mark the session as live-ready once so later follow-up pins are not
			// wiped on every streaming effect re-run (common in the mini window).
			sessionHadTranscript = true;
			clearHydrationFailsafe();
			isInitialTranscriptPaint = false;
			lastScrolledUserId = null;
			activePinnedUserId = null;
			showJumpToBottom = false;
			return;
		}

		if (!isInitialTranscriptPaint) return;

		// Ensure failsafe is armed even if we entered hydrating via a path that
		// did not call beginHydrationPaint (e.g. initial controller mount).
		armHydrationFailsafe();

		let cancelled = false;
		let settleFrame = 0;
		let lastHeight = 0;
		let stableFrames = 0;
		let frameCount = 0;

		const finishHydration = () => {
			if (cancelled) return;
			cancelled = true;
			if (settleFrame) {
				cancelAnimationFrame(settleFrame);
				settleFrame = 0;
			}
			finishHydrationPaint();
		};

		const settle = () => {
			if (cancelled) return;
			if (!scroller) {
				settleFrame = requestAnimationFrame(settle);
				return;
			}
			scroller.scrollTop = scroller.scrollHeight;
			notifyScrollTop();
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
			// Cancel rAF settle only — do NOT clear the wall-clock failsafe.
			// threadItems / sync churn re-enters this effect; resetting the
			// failsafe was why #157 still hung opacity:0 on soft-swap/open.
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
		const hydrating = isInitialTranscriptPaint;
		const count = deps.getUserMessageCount();
		const vh = viewportHeight;
		const streaming = deps.getSessionStreaming();

		if (!userId) {
			lastScrolledUserId = null;
			return;
		}
		if (userId === lastScrolledUserId) return;

		// Transcript refresh may remap the pinned user id after the stream ends.
		// Retarget the reply canvas without scrolling again.
		if (activePinnedUserId !== null && !streaming && count > 1 && !hydrating) {
			activePinnedUserId = userId;
			lastScrolledUserId = userId;
			return;
		}

		// Do not consume the id until we can actually present — retry when
		// hydration finishes, viewport is measured, or streaming starts.
		if (hydrating || count <= 1 || vh <= 0 || !streaming) return;

		presentFollowUpTurn(userId);
	});

	// If leading orphans appear after paint (or hasMore flips), keep auto-anchoring.
	$effect(() => {
		if (isInitialTranscriptPaint) return;
		if (!deps.getIsSessionSynced()) return;
		void deps.getThreadItems();
		void deps.getHasMoreHistory?.();
		void maybeAutoAnchorLeadingOrphans();
	});

	return {
		get showJumpToBottom() {
			return showJumpToBottom;
		},
		get activePinnedUserId() {
			return activePinnedUserId;
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
		get sessionEpoch() {
			return sessionEpoch;
		},
		setScroller,
		onScroll,
		jumpToBottom,
		maybeLoadOlderHistory,
		maybeAutoAnchorLeadingOrphans
	};
}
