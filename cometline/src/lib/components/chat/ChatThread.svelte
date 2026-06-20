<script lang="ts">
	import { tick, untrack } from 'svelte';
	import { fade } from 'svelte/transition';
	import { Brain, Check, Copy, TriangleAlert } from '@lucide/svelte';
	import { chatStore, type ChatItem } from '$lib/stores/chat.svelte';
	import { settingsStore } from '$lib/stores/settings.svelte';
	import { chatDebug, chatDebugEnabled, summarizeChatItem } from '$lib/debug/chat';
	import AssistantMarkdown from '$lib/components/AssistantMarkdown.svelte';
	import ThinkingIndicator from '$lib/components/ThinkingIndicator.svelte';
	import ThinkingBlock from '$lib/components/chat/ThinkingBlock.svelte';
	import ToolFoldPanel from '$lib/components/chat/ToolFoldPanel.svelte';
	import SubagentPanel from '$lib/components/chat/SubagentPanel.svelte';
	import JumpToBottom from '$lib/components/chat/JumpToBottom.svelte';
	import ThreadAvatar from '$lib/components/chat/ThreadAvatar.svelte';
	import { imageDataURL } from '$lib/files/images';
	import { memoryUpdateHint, memoryUpdateTooltip } from '$lib/memory-updates';
	import {
		buildAssistantTimeline,
		buildThinkingAttribution,
		type InjectedMemory,
		type TimelineEntry
	} from '$lib/conversation/thinking-attribution';
	import {
		anyReasoningPending,
		hasReasoning,
		reasoningTextLength
	} from '$lib/conversation/reasoning';

	const TRANSCRIPT_IN = { duration: 140 };

	let {
		sessionId,
		awaitingFirstAssistant = false,
		firstTurnFlightDone = false
	}: {
		sessionId: string;
		awaitingFirstAssistant?: boolean;
		firstTurnFlightDone?: boolean;
	} = $props();

	let iconVariant = $derived(settingsStore.settings.app.iconVariant);

	let isInitialTranscriptPaint = $state(true);
	let isSessionSynced = $derived(chatStore.sessionID === sessionId);
	let sessionStreaming = $derived(chatStore.isStreamingFor(sessionId));

	let snapshotItems = $state.raw<ChatItem[]>([]);

	$effect(() => {
		if (chatStore.sessionID !== sessionId) return;
		snapshotItems = chatStore.items;
	});

	let scroller: HTMLDivElement;
	let scrollFrame = 0;
	// We never auto-scroll the thread. Instead, when the content overflows the
	// viewport and the user is not already near the bottom, we show a
	// jump-to-bottom button so they can opt in to jumping to the latest content.
	let showJumpToBottom = $state(false);
	const SCROLL_PIN_THRESHOLD = 96;
	// Tracks the most recently sent user message so we can scroll it near the
	// top of the viewport once (revealing the assistant avatar/response below).
	let lastScrolledUserId: string | null = null;
	// Gap left above the freshly-sent user message. The first turn pins close to
	// the top; follow-up turns sit a little lower (upper-middle) so the message
	// reads naturally and leaves the bulk of the screen for the reply below.
	const USER_SEND_TOP_OFFSET = 24;
	const USER_SEND_FOLLOWUP_OFFSET = 128;
	// ChatGPT-style: reserve empty space below the latest turn so a freshly-sent
	// user message can always scroll up to the top, leaving room for the
	// assistant avatar/response to appear below it (never clipped).
	let viewportHeight = $state(0);
	let expandedToolOutput = $state(new Set<string>());
	let thinkingOverrides = $state(new Map<string, boolean>());
	let expandedMemoryInThinking = $state(new Set<string>());
	let subagentFold = $state(new Map<string, boolean>());
	let copiedId = $state<string | null>(null);
	let copyResetTimer: ReturnType<typeof setTimeout> | null = null;
	let now = $state(Date.now());
	let threadItems = $derived(isSessionSynced ? chatStore.items : snapshotItems);
	let firstAssistantItem = $derived(
		threadItems.find(
			(item) => item.type === 'assistant' && (item.text.trim() || hasReasoning(item))
		) as Extract<ChatItem, { type: 'assistant' }> | undefined
	);
	let firstAssistantId = $derived(firstAssistantItem?.id ?? null);
	let streamingAssistantId = $derived.by(() => {
		if (!sessionStreaming) return null;
		const last = threadItems.at(-1);
		if (last?.type === 'user') return null;
		return threadItems.findLast((item) => item.type === 'assistant')?.id ?? null;
	});
	let firstUserId = $derived(threadItems.find((item) => item.type === 'user')?.id ?? null);
	let lastUserId = $derived(threadItems.findLast((item) => item.type === 'user')?.id ?? null);
	let userMessageCount = $derived(
		threadItems.reduce((count, item) => (item.type === 'user' ? count + 1 : count), 0)
	);
	// Reserve a viewport-tall gap below the conversation right after the user
	// sends a message, so the latest user message can scroll up toward the
	// top-right area. Once the assistant starts responding (assistant/tool/
	// subagent items appear), the spacer collapses so the reply grows upward
	// naturally instead of leaving a big empty gap. Only applies once a
	// follow-up turn exists (the first turn keeps its natural layout).
	let bottomSpacerHeight = $derived(
		userMessageCount > 1 && viewportHeight > 0 && threadItems.at(-1)?.type === 'user'
			? Math.max(0, viewportHeight - 240)
			: 0
	);
	let showMessages = $derived(
		threadItems.length > 0 || (isSessionSynced && awaitingFirstAssistant && !firstUserId)
	);
	let transcriptFadeIn = $derived(
		awaitingFirstAssistant || isInitialTranscriptPaint ? { duration: 0 } : TRANSCRIPT_IN
	);

	function sessionHasCachedTranscript(targetSessionId: string) {
		if (chatStore.sessionID === targetSessionId && chatStore.items.length > 0) return true;
		if (
			chatStore.isStreamingFor(targetSessionId) ||
			chatStore.hasInFlightTurn(targetSessionId)
		) {
			return true;
		}
		return chatStore.getCachedItemCount(targetSessionId) > 0;
	}

	let thinkingForAssistant = $derived(buildThinkingAttribution(threadItems));

	function isToolInBuffer(item: Extract<ChatItem, { type: 'tool' }>) {
		return thinkingForAssistant.toolIdsInBuffer.has(item.id);
	}

	function isSubagentInBuffer(item: Extract<ChatItem, { type: 'subagent' }>) {
		return thinkingForAssistant.subagentIdsInBuffer.has(item.id);
	}

	function isMemoryInBuffer(item: Extract<ChatItem, { type: 'memory' }>) {
		return thinkingForAssistant.memoryIdsInBuffer.has(item.id);
	}

	// A thinking block auto-expands while reasoning is actively streaming and
	// folds when it is done. Tool execution does NOT expand the block. The user
	// can override the default by toggling; the override is keyed per segment.
	function thinkingExpanded(segmentKey: string, pending?: boolean) {
		const override = thinkingOverrides.get(segmentKey);
		if (override !== undefined) return override;
		return pending === true;
	}

	function toggleThinking(segmentKey: string, pending?: boolean) {
		const next = new Map(thinkingOverrides);
		next.set(segmentKey, !thinkingExpanded(segmentKey, pending));
		thinkingOverrides = next;
	}

	function memoryInThinkingExpanded(segmentKey: string) {
		return expandedMemoryInThinking.has(segmentKey);
	}

	function toggleMemoryInThinking(segmentKey: string) {
		expandedMemoryInThinking = toggleExpanded(expandedMemoryInThinking, segmentKey);
	}

	function subagentExpanded(id: string) {
		const override = subagentFold.get(id);
		if (override !== undefined) return override;
		return false;
	}

	function toggleSubagent(id: string) {
		const next = new Map(subagentFold);
		next.set(id, !subagentExpanded(id));
		subagentFold = next;
	}

	async function copyMessage(id: string, text: string) {
		try {
			await navigator.clipboard.writeText(text);
		} catch {
			return;
		}
		copiedId = id;
		if (copyResetTimer) clearTimeout(copyResetTimer);
		copyResetTimer = setTimeout(() => {
			copiedId = null;
			copyResetTimer = null;
		}, 1600);
	}

	// Drives auto-expand: only the reasoning stream should expand the block.
	function thinkingActive(pending?: boolean) {
		return pending === true;
	}

	function isNearBottom(element: HTMLElement) {
		return (
			element.scrollHeight - element.scrollTop - element.clientHeight <= SCROLL_PIN_THRESHOLD
		);
	}

	// Show the jump-to-bottom button only when the content overflows the viewport
	// and the user is not already near the bottom. No auto-scrolling happens.
	function updateJumpToBottom() {
		if (!scroller) {
			showJumpToBottom = false;
			return;
		}
		const overflowing = scroller.scrollHeight - scroller.clientHeight > SCROLL_PIN_THRESHOLD;
		showJumpToBottom = overflowing && !isNearBottom(scroller);
	}

	function onThreadScroll() {
		updateJumpToBottom();
	}

	function jumpToBottom() {
		if (!scroller) return;
		scroller.scrollTo({ top: scroller.scrollHeight, behavior: 'smooth' });
		showJumpToBottom = false;
	}

	$effect(() => {
		void sessionId;
		thinkingOverrides = new Map();
		// Treat the session's existing latest user message as already-positioned so
		// switching sessions doesn't trigger the send auto-scroll.
		lastScrolledUserId = untrack(() => lastUserId);
		if (sessionHasCachedTranscript(sessionId)) {
			isInitialTranscriptPaint = false;
			return;
		}
		isInitialTranscriptPaint = true;
	});

	let scrollKey = $derived.by(() => {
		if (!sessionStreaming) {
			const last = threadItems.at(-1);
			return `idle:${threadItems.length}:${last?.id ?? ''}`;
		}
		for (let i = threadItems.length - 1; i >= 0; i--) {
			const item = threadItems[i];
			if (item.type === 'assistant') {
				return `stream:assistant:${item.id}:${item.text.length}:${reasoningTextLength(item)}:${anyReasoningPending(item)}:${item.pending ?? false}`;
			}
			if (item.type === 'tool') {
				return `stream:tool:${item.id}:${item.output?.length ?? 0}:${item.pending ?? false}`;
			}
		}
		return `stream:empty:${threadItems.length}`;
	});
	let renderDebugSnapshot = $derived.by(() => ({
		awaitingFirstAssistant,
		firstTurnFlightDone,
		isStreaming: sessionStreaming,
		firstUserId,
		firstAssistantId,
		firstAssistantItem: firstAssistantItem ? summarizeChatItem(firstAssistantItem) : null,
		firstAssistantVisible: firstAssistantItem ? showAssistantRow(firstAssistantItem) : false,
		items: threadItems.map(summarizeRenderItem)
	}));

	$effect(() => {
		return () => {
			if (copyResetTimer) clearTimeout(copyResetTimer);
		};
	});

	$effect(() => {
		const hasTimedPending = chatStore.items.some(
			(item) =>
				(item.type === 'tool' && item.pending) ||
				(item.type === 'assistant' && item.pending && item.pendingStartedAt != null)
		);
		if (!hasTimedPending) return;
		const timer = setInterval(() => {
			now = Date.now();
		}, 100);
		return () => clearInterval(timer);
	});

	function formatToolDuration(ms: number) {
		if (ms < 1000) return `${Math.max(1, Math.round(ms))}ms`;
		if (ms < 10000) return `${(ms / 1000).toFixed(1)}s`;
		return `${Math.round(ms / 1000)}s`;
	}

	function toolDurationLabel(item: Extract<ChatItem, { type: 'tool' }>) {
		if (item.durationMs != null) return formatToolDuration(item.durationMs);
		if (item.pending && item.startedAt != null) return formatToolDuration(now - item.startedAt);
		return '';
	}

	function usageText(item: Extract<ChatItem, { type: 'status' }>) {
		const usage = item.usage;
		if (!usage) return item.text;
		return `${item.text} · ${usage.input_tokens} in / ${usage.output_tokens} out`;
	}

	function toggleExpanded(set: Set<string>, id: string) {
		const next = new Set(set);
		if (next.has(id)) next.delete(id);
		else next.add(id);
		return next;
	}

	function toggleToolOutput(id: string) {
		expandedToolOutput = toggleExpanded(expandedToolOutput, id);
	}

	function toolOutputExpanded(item: Extract<ChatItem, { type: 'tool' }>) {
		return expandedToolOutput.has(item.id);
	}

	function toolFoldLabel(item: Extract<ChatItem, { type: 'tool' }>) {
		const status = item.pending ? 'running' : item.error ? 'fail' : 'success';
		const duration = toolDurationLabel(item);
		return duration
			? `${item.toolName} → ${status} · ${duration}`
			: `${item.toolName} → ${status}`;
	}

	let heroGlowColor = $derived(settingsStore.settings.appearance.heroComposer.glowColor);

	function showAssistantActivitySpinner(item: Extract<ChatItem, { type: 'assistant' }>) {
		return sessionStreaming && item.id === streamingAssistantId;
	}

	function assistantWaitSeconds(item: Extract<ChatItem, { type: 'assistant' }>) {
		if (item.pendingStartedAt == null) return 0;
		return Math.max(0, Math.floor((now - item.pendingStartedAt) / 1000));
	}

	function assistantActivityMessage(item: Extract<ChatItem, { type: 'assistant' }> | undefined) {
		const seconds = item ? assistantWaitSeconds(item) : 0;
		if (seconds >= 90) {
			return `Still waiting for the provider after ${seconds}s. This request may time out soon.`;
		}
		if (seconds >= 30) {
			return `The model has not started streaming after ${seconds}s. The provider may be queued or slow.`;
		}
		if (seconds >= 8) {
			return `Still waiting for the model (${seconds}s).`;
		}
		return 'Contacting model...';
	}

	function hasVisibleThinkingBlock(itemId: string) {
		return buildAssistantTimeline(itemId, threadItems, thinkingForAssistant).length > 0;
	}

	function showAssistantPending(item: Extract<ChatItem, { type: 'assistant' }>) {
		if (!sessionStreaming || item.id !== streamingAssistantId) return false;
		if (item.text?.trim()) return false;
		return !hasVisibleThinkingBlock(item.id);
	}

	function showAssistantRow(item: Extract<ChatItem, { type: 'assistant' }>) {
		return Boolean(
			item.text ||
			hasReasoning(item) ||
			hasVisibleThinkingBlock(item.id) ||
			showAssistantPending(item) ||
			showAssistantActivitySpinner(item)
		);
	}

	function summarizeRenderItem(item: ChatItem, index: number) {
		if (item.type !== 'assistant') return { index, ...summarizeChatItem(item) };
		return {
			index,
			...summarizeChatItem(item),
			showAssistantRow: showAssistantRow(item),
			isFirstAssistant: item.id === firstAssistantId,
			renderedInFirstTurnSlot: awaitingFirstAssistant && item.id === firstAssistantId,
			excludedFromNormalList: awaitingFirstAssistant && item.id === firstAssistantId,
			showAssistantPending: showAssistantPending(item)
		};
	}

	function speakerFor(item: ChatItem | undefined): 'user' | 'assistant' | null {
		if (!item) return null;
		if (item.type === 'user') return 'user';
		if (item.type === 'assistant' || item.type === 'tool' || item.type === 'subagent')
			return 'assistant';
		return null;
	}

	function showFirstTurnAvatarSlot() {
		if (!awaitingFirstAssistant || !firstUserId) return false;
		if (!firstTurnFlightDone) return true;
		if (!firstAssistantItem) return true;
		return !showAssistantRow(firstAssistantItem);
	}

	function timelineEntryKey(entry: TimelineEntry) {
		if (entry.kind === 'reasoning') return `${entry.kind}-${entry.segmentIndex}`;
		if (entry.kind === 'tool') return `${entry.kind}-${entry.tool.id}`;
		return `${entry.kind}-${entry.subagent.id}`;
	}

	function firstAssistantInNormalList(item: Extract<ChatItem, { type: 'assistant' }>) {
		if (showFirstTurnAvatarSlot()) return false;
		return !(
			awaitingFirstAssistant &&
			item.id === firstAssistantId &&
			firstUserId &&
			!(firstTurnFlightDone && showAssistantRow(item))
		);
	}

	function startsSpeakerRun(index: number, speaker: 'user' | 'assistant') {
		for (let i = index - 1; i >= 0; i--) {
			const previousSpeaker = speakerFor(threadItems[i]);
			if (!previousSpeaker) continue;
			return previousSpeaker !== speaker;
		}
		return true;
	}

	$inspect(renderDebugSnapshot).with((type, snapshot) => {
		if (!chatDebugEnabled()) return;
		chatDebug('thread:$inspect', { type, ...snapshot });
	});

	$effect(() => {
		if (!isSessionSynced) {
			if (sessionHasCachedTranscript(sessionId)) {
				isInitialTranscriptPaint = false;
			} else {
				isInitialTranscriptPaint = true;
			}
			return;
		}
		// Only hide the transcript while loading an empty session. If local items
		// already exist (first-turn flight, streaming), keep them visible even
		// when isLoading is stale.
		if (chatStore.isLoading && threadItems.length === 0) {
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
				// We never auto-scroll on new content. We only re-evaluate whether
				// the jump-to-bottom button should be visible now that the content
				// height (and therefore the distance from the bottom) has changed.
				if (isInitialTranscriptPaint) return;
				updateJumpToBottom();
			});
		});
		return () => {
			if (scrollFrame) cancelAnimationFrame(scrollFrame);
		};
	});

	// Track the scroller's visible height so the bottom spacer can reserve a
	// viewport-tall gap below the latest turn.
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

	// When the user sends a new message, scroll that message close to the top of
	// the viewport so the assistant avatar and its incoming response are visible
	// below it. This is the one deliberate auto-scroll in the thread.
	function scrollUserMessageToTop(userId: string) {
		if (!scroller) return;
		const target = scroller.querySelector<HTMLElement>(`[data-user-item-id="${userId}"]`);
		if (!target) return;
		const offset = userMessageCount > 1 ? USER_SEND_FOLLOWUP_OFFSET : USER_SEND_TOP_OFFSET;
		const top = Math.max(0, target.offsetTop - scroller.offsetTop - offset);
		scroller.scrollTo({ top, behavior: 'smooth' });
		updateJumpToBottom();
	}

	$effect(() => {
		const userId = lastUserId;
		if (!userId) {
			lastScrolledUserId = null;
			return;
		}
		if (userId === lastScrolledUserId) return;
		// Don't fight the initial transcript hydration scroll.
		if (isInitialTranscriptPaint) {
			lastScrolledUserId = userId;
			return;
		}
		lastScrolledUserId = userId;
		void tick().then(() => {
			requestAnimationFrame(() => scrollUserMessageToTop(userId));
		});
	});
</script>

{#snippet assistantActivitySpinner(item?: Extract<ChatItem, { type: 'assistant' }>)}
	<div class="assistant-activity-spinner">
		<ThinkingIndicator color={heroGlowColor} size={24} label="Assistant is responding" />
		<span>{assistantActivityMessage(item)}</span>
	</div>
{/snippet}

{#snippet assistantStack(item: Extract<ChatItem, { type: 'assistant' }>)}
	{@const timeline = buildAssistantTimeline(item.id, threadItems, thinkingForAssistant)}
	<div class="assistant-stack">
		{#each timeline as entry (timelineEntryKey(entry))}
			{#if entry.kind === 'reasoning'}
				{@const segmentKey = `${item.id}-seg-${entry.segmentIndex}`}
				<ThinkingBlock
					text={entry.text}
					pending={entry.pending}
					memories={entry.memories}
					expanded={thinkingExpanded(segmentKey, entry.pending)}
					memoryExpanded={memoryInThinkingExpanded(segmentKey)}
					showSpinner={thinkingActive(entry.pending) &&
						!(item.id === streamingAssistantId && sessionStreaming)}
					onToggle={() => toggleThinking(segmentKey, entry.pending)}
					onToggleMemory={() => toggleMemoryInThinking(segmentKey)}
				/>
			{:else if entry.kind === 'tool'}
				<ToolFoldPanel
					item={entry.tool}
					label={toolFoldLabel(entry.tool)}
					expanded={toolOutputExpanded(entry.tool)}
					onToggle={() => toggleToolOutput(entry.tool.id)}
				/>
			{:else}
				<SubagentPanel
					item={entry.subagent}
					expanded={subagentExpanded(entry.subagent.id)}
					onToggle={() => toggleSubagent(entry.subagent.id)}
				/>
			{/if}
		{/each}
		{#if item.text}
			<div class="bubble assistant-bubble">
				<AssistantMarkdown
					source={item.text}
					streaming={item.id === streamingAssistantId}
				/>
			</div>
			{#if item.id !== streamingAssistantId}
				<div class="message-actions m-1">
					{#if item.memoryUpdates?.length}
						<span
							class="message-action memory-hint"
							title={memoryUpdateTooltip(item.memoryUpdates)}
							aria-label={memoryUpdateTooltip(item.memoryUpdates)}
						>
							{memoryUpdateHint(item.memoryUpdates)}
						</span>
					{/if}
					<button
						type="button"
						class="message-action m-1"
						class:copied={copiedId === item.id}
						title="Copy message"
						aria-label="Copy message"
						onclick={() => copyMessage(item.id, item.text)}
					>
						{#if copiedId === item.id}
							<Check size={13} />
							<span>Copied</span>
						{:else}
							<Copy size={13} />
							<span>Copy</span>
						{/if}
					</button>
				</div>
			{/if}
		{/if}
		{#if showAssistantActivitySpinner(item)}
			{@render assistantActivitySpinner(item)}
		{/if}
	</div>
{/snippet}

<div class="thread-wrap">
	<div class="thread" bind:this={scroller} onscroll={onThreadScroll} aria-live="polite">
		<div class="thread-inner">
			{#if showMessages}
				<div
					class="thread-messages"
					class:hydrating={isInitialTranscriptPaint}
					in:fade={transcriptFadeIn}
				>
					{#if awaitingFirstAssistant && !firstUserId}
						<div class="row assistant-row flight-placeholder" aria-hidden="true">
							<ThreadAvatar
								variant="avatar"
								{iconVariant}
								flightHidden={!firstTurnFlightDone}
								flightTarget="avatar"
							/>
							{#if firstAssistantItem && showAssistantRow(firstAssistantItem)}
								{@render assistantStack(firstAssistantItem)}
							{:else if sessionStreaming}
								<div class="assistant-stack">
									{@render assistantActivitySpinner()}
								</div>
							{:else}
								<div class="assistant-stack"></div>
							{/if}
						</div>
					{/if}

					{#each threadItems as item, index (item.id)}
						{#if item.type === 'user'}
							<div
								class="row user-row"
								class:continuation-row={!startsSpeakerRun(index, 'user')}
								data-user-item-id={item.id}
							>
								<ThreadAvatar variant="gutter" {iconVariant} />
								<div class="user-stack">
									<div
										class="bubble user-bubble"
										class:flight-hidden={item.reveal === false}
										data-flight-target={item.reveal === false
											? 'user'
											: undefined}
									>
										{#if item.images?.length}
											<div
												class="user-images"
												class:text-following={Boolean(item.text)}
											>
												{#each item.images as image, imageIndex (`${item.id}-image-${image.id ?? imageIndex}`)}
													<img
														src={imageDataURL(image)}
														alt={image.name ?? 'Attached image'}
													/>
												{/each}
											</div>
										{/if}
										{#if item.text?.trim()}
											<AssistantMarkdown
												source={item.text.trim()}
												mode="user"
											/>
										{/if}
									</div>
									{#if item.text?.trim()}
										<div class="message-actions user-message-actions">
											<button
												type="button"
												class="message-action m-1"
												class:copied={copiedId === item.id}
												title="Copy message"
												aria-label="Copy message"
												onclick={() =>
													copyMessage(item.id, item.text.trim())}
											>
												{#if copiedId === item.id}
													<Check size={13} />
													<span>Copied</span>
												{:else}
													<Copy size={13} />
													<span>Copy</span>
												{/if}
											</button>
										</div>
									{/if}
								</div>
							</div>
							{#if showFirstTurnAvatarSlot() && item.id === firstUserId}
								<div
									class="row assistant-row"
									class:flight-placeholder={!firstAssistantId}
									aria-hidden={!firstAssistantId}
								>
									<ThreadAvatar
										variant="avatar"
										{iconVariant}
										flightHidden={!firstTurnFlightDone}
										flightTarget="avatar"
									/>
									{#if firstAssistantItem && showAssistantRow(firstAssistantItem)}
										{@render assistantStack(firstAssistantItem)}
									{:else if sessionStreaming}
										<div class="assistant-stack">
											{@render assistantActivitySpinner()}
										</div>
									{:else}
										<div class="assistant-stack"></div>
									{/if}
								</div>
							{/if}
						{:else if item.type === 'assistant' && showAssistantRow(item) && firstAssistantInNormalList(item)}
							<div
								class="row assistant-row"
								class:continuation-row={!startsSpeakerRun(index, 'assistant')}
							>
								{#if startsSpeakerRun(index, 'assistant')}
									<ThreadAvatar variant="avatar" {iconVariant} />
								{:else}
									<ThreadAvatar variant="gutter" {iconVariant} />
								{/if}
								{@render assistantStack(item)}
							</div>
						{:else if item.type === 'tool' && !isToolInBuffer(item)}
							<div
								class="row tool-row"
								class:continuation-row={!startsSpeakerRun(index, 'assistant')}
							>
								<ThreadAvatar variant="gutter" {iconVariant} />
								<div class="tool-stack">
									<ToolFoldPanel
										{item}
										label={toolFoldLabel(item)}
										expanded={toolOutputExpanded(item)}
										onToggle={() => toggleToolOutput(item.id)}
									/>
								</div>
							</div>
						{:else if item.type === 'subagent' && !isSubagentInBuffer(item)}
							<div
								class="row tool-row subagent-row"
								class:continuation-row={!startsSpeakerRun(index, 'assistant')}
							>
								<ThreadAvatar variant="gutter" {iconVariant} />
								<div class="subagent-stack">
									<SubagentPanel
										{item}
										expanded={subagentExpanded(item.id)}
										onToggle={() => toggleSubagent(item.id)}
									/>
								</div>
							</div>
						{:else if item.type === 'memory' && !isMemoryInBuffer(item)}
							<div class="row event-row">
								<div class="event-card memory-card">
									<div class="event-title">
										<Brain size={14} /><span>Memories used</span>
									</div>
									<div class="memory-chips">
										{#each item.memories as mem (mem.id)}
											<span class="memory-chip" title={mem.content}
												>{mem.kind}: {mem.content}</span
											>
										{/each}
									</div>
								</div>
							</div>
						{:else if item.type === 'status'}
							<div class="status">{usageText(item)}</div>
						{:else if item.type === 'error'}
							<div class="row event-row">
								<div class="event-card error-card">
									<div class="event-title">
										<TriangleAlert size={14} /><span>Error</span>
									</div>
									<p>{item.text}</p>
								</div>
							</div>
						{/if}
					{/each}
					{#if bottomSpacerHeight > 0}
						<div
							class="thread-bottom-spacer"
							style:height="{bottomSpacerHeight}px"
							aria-hidden="true"
						></div>
					{/if}
				</div>
			{/if}
		</div>
	</div>
	{#if showJumpToBottom}
		<JumpToBottom onclick={jumpToBottom} />
	{/if}
</div>

<style>
	.thread-wrap {
		position: absolute;
		inset: 0;
	}

	.thread {
		position: absolute;
		inset: 0;
		overflow-y: auto;
		scrollbar-gutter: stable;
		padding: 32px var(--chat-gutter) var(--thread-padding-bottom);
		scrollbar-width: thin;
	}

	.thread-inner {
		--chat-content-column: min(
			var(--chat-content-max),
			calc(100% - var(--chat-avatar-size) - var(--chat-row-gap))
		);
		--chat-assistant-column: var(--chat-content-column);
		width: min(var(--chat-thread-width), 100%);
		margin: 0 auto;
		display: flex;
		flex-direction: column;
		gap: 14px;
	}

	.thread-messages {
		display: flex;
		flex-direction: column;
		gap: 14px;
		transition: opacity var(--duration-session-switch) var(--ease-smooth);
	}

	.thread-messages.hydrating {
		opacity: 0;
		pointer-events: none;
	}

	.thread-bottom-spacer {
		flex: 0 0 auto;
		pointer-events: none;
	}

	@media (min-width: 768px) {
		.thread {
			padding: 40px var(--chat-gutter) var(--thread-padding-bottom);
		}

		.thread-inner {
			gap: 16px;
		}
	}

	@media (min-width: 1024px) {
		.thread {
			padding: 48px var(--chat-gutter) var(--thread-padding-bottom);
		}

		.thread-inner {
			gap: 18px;
		}
	}

	@media (min-width: 1280px) {
		.thread {
			padding: 56px var(--chat-gutter) var(--thread-padding-bottom);
		}
	}

	.status {
		align-self: center;
		display: inline-flex;
		align-items: center;
		gap: 7px;
		padding: 6px 10px;
		border-radius: 999px;
		background: rgba(255, 255, 255, 0.74);
		border: 1px solid var(--border-soft);
		font-size: 12px;
		color: var(--text-muted);
	}

	.row {
		display: flex;
		width: 100%;
		gap: var(--chat-row-gap);
	}

	.continuation-row {
		margin-top: -6px;
	}

	.tool-row.continuation-row {
		margin-top: -16px;
	}

	.user-row {
		justify-content: flex-start;
	}

	.user-stack {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		flex: 1 1 auto;
		min-width: 0;
		max-width: var(--chat-assistant-column);
	}

	.assistant-row,
	.tool-row,
	.event-row {
		justify-content: flex-start;
	}

	.assistant-row,
	.tool-row {
		align-items: flex-start;
	}

	.tool-stack {
		min-width: 0;
		flex: 1;
		max-width: var(--chat-assistant-column);
	}

	.flight-placeholder {
		pointer-events: none;
	}

	.flight-placeholder .assistant-stack {
		min-height: 1px;
	}

	.flight-hidden {
		opacity: 0;
		pointer-events: none;
	}

	.assistant-stack {
		display: flex;
		flex-direction: column;
		gap: 8px;
		max-width: var(--chat-assistant-column);
		min-width: 0;
		flex: 0 1 auto;
		align-items: flex-start;
	}

	.assistant-activity-spinner {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 8px 2px 2px;
	}

	.assistant-activity-spinner span {
		font-size: 12px;
		color: var(--text-soft, rgba(0, 0, 0, 0.55));
	}

	.message-actions {
		display: flex;
		align-items: center;
		gap: 4px;
		margin-top: -2px;
		opacity: 0;
		transition: opacity var(--duration-fast) var(--ease-smooth);
	}

	/* Reveal on hover/focus of the surrounding message stack. */
	.assistant-stack:hover .message-actions,
	.user-stack:hover .message-actions,
	.message-actions:focus-within {
		opacity: 1;
	}

	.user-message-actions {
		justify-content: flex-end;
	}

	.message-action {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		padding: 4px 8px;
		border: 1px solid transparent;
		border-radius: 7px;
		background: transparent;
		color: var(--text-soft);
		font-size: 11px;
		font-weight: 600;
		line-height: 1;
		cursor: pointer;
		transition:
			color var(--duration-fast) var(--ease-smooth),
			background var(--duration-fast) var(--ease-smooth),
			border-color var(--duration-fast) var(--ease-smooth);
	}

	.message-action:hover {
		color: var(--text-main);
		background: rgba(255, 255, 255, 0.92);
		border-color: var(--border-soft);
	}

	.message-action.copied {
		color: #15803d;
	}

	.memory-hint {
		cursor: default;
	}

	.memory-hint:hover {
		background: transparent;
		border-color: transparent;
		color: var(--text-soft);
	}

	.message-action :global(svg) {
		flex-shrink: 0;
	}

	@media (prefers-reduced-motion: reduce) {
		.message-actions {
			transition: none;
		}
	}

	.event-row .event-card {
		max-width: var(--chat-content-column);
	}

	.event-card {
		width: 100%;
		max-width: 100%;
		border: 1px solid var(--border-soft);
		background: rgba(255, 255, 255, 0.68);
		border-radius: 14px;
		padding: 10px 12px;
		color: var(--text-muted);
		box-shadow: 0 6px 18px rgba(15, 23, 42, 0.04);
	}

	.event-title {
		display: flex;
		align-items: center;
		gap: 7px;
		font-size: 12px;
		font-weight: 650;
		color: var(--text-main);
		margin-bottom: 7px;
	}

	.event-title :global(svg:last-child) {
		flex-shrink: 0;
	}

	.memory-chips {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.memory-chip {
		min-width: 0;
		max-width: 100%;
		overflow-wrap: anywhere;
		word-break: break-word;
		white-space: normal;
		padding: 5px 10px;
		border-radius: 10px;
		background: rgba(0, 102, 204, 0.08);
		color: var(--text-main);
		font-size: 11px;
		line-height: 1.45;
	}

	.error-card p {
		margin: 0;
		font-size: 12px;
		line-height: 1.5;
		white-space: pre-wrap;
	}

	.error-card {
		color: #b42318;
	}

	.error-card {
		background: rgba(255, 245, 245, 0.82);
		border-color: rgba(244, 63, 94, 0.18);
	}

	.subagent-row {
		align-items: flex-start;
	}

	.subagent-stack {
		min-width: 0;
		flex: 1;
		max-width: var(--chat-assistant-column);
	}
</style>
