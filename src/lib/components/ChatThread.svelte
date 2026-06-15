<script lang="ts">
	import { tick } from 'svelte';
	import { fade, fly, slide } from 'svelte/transition';
	import {
		Brain,
		Check,
		ChevronDown,
		CircleCheck,
		Copy,
		LoaderCircle,
		Terminal,
		TriangleAlert
	} from '@lucide/svelte';
	import { chatStore, type ChatItem } from '$lib/stores/chat.svelte';
	import { chatDebug, chatDebugEnabled, summarizeChatItem } from '../debug/chat';
	import AssistantMarkdown from '$lib/components/AssistantMarkdown.svelte';

	const ASSISTANT_ROW_IN = { y: 10, duration: 220 };
	const TOOL_ROW_IN = { y: 8, duration: 200 };
	const STATUS_ROW_IN = { y: 6, duration: 180 };
	const FOLD_IN = { duration: 180 };
	const TRANSCRIPT_IN = { duration: 220 };

	let {
		sessionId,
		awaitingFirstAssistant = false,
		firstTurnFlightDone = false
	}: {
		sessionId: string;
		awaitingFirstAssistant?: boolean;
		firstTurnFlightDone?: boolean;
	} = $props();

	let isInitialTranscriptPaint = $state(true);
	let isSessionSynced = $derived(chatStore.sessionID === sessionId);

	let scroller: HTMLDivElement;
	let scrollFrame = 0;
	let expandedToolOutput = $state(new Set<string>());
	let expandedThinking = $state(new Set<string>());
	let copiedId = $state<string | null>(null);
	let copyResetTimer: ReturnType<typeof setTimeout> | null = null;
	let now = $state(Date.now());
	let threadItems = $derived(chatStore.items);
	let firstAssistantItem = $derived(
		threadItems.find((item) =>
			item.type === 'assistant' &&
			(item.text.trim() || item.reasoning?.text.trim() || item.reasoning?.pending)
		) as Extract<ChatItem, { type: 'assistant' }> | undefined
	);
	let firstAssistantId = $derived(firstAssistantItem?.id ?? null);
	let streamingAssistantId = $derived(
		chatStore.isStreaming
			? (threadItems.findLast((item) => item.type === 'assistant')?.id ?? null)
			: null
	);
	let firstUserId = $derived(threadItems.find((item) => item.type === 'user')?.id ?? null);
	let showMessages = $derived(
		isSessionSynced &&
			(threadItems.length > 0 || (awaitingFirstAssistant && !firstUserId))
	);
	let transcriptFadeIn = $derived(awaitingFirstAssistant ? { duration: 0 } : TRANSCRIPT_IN);

	type ThinkingBlock = {
		reasoning?: { text: string; pending?: boolean };
		tools: Extract<ChatItem, { type: 'tool' }>[];
	};

	let thinkingForAssistant = $derived.by(() => {
		const map = new Map<string, ThinkingBlock>();
		const toolIdsInBuffer = new Set<string>();
		let currentAssistantId: string | null = null;

		for (const item of threadItems) {
			if (item.type === 'user' || item.type === 'status' || item.type === 'error') {
				currentAssistantId = null;
				continue;
			}
			if (item.type === 'assistant') {
				currentAssistantId = item.id;
				const existing = map.get(item.id);
				if (!existing) {
					map.set(item.id, { reasoning: item.reasoning, tools: [] });
				} else if (item.reasoning && !existing.reasoning) {
					existing.reasoning = item.reasoning;
				}
			} else if (item.type === 'tool' && currentAssistantId) {
				const block = map.get(currentAssistantId);
				if (block) {
					block.tools.push(item);
					toolIdsInBuffer.add(item.id);
				}
			}
		}

		return { map, toolIdsInBuffer };
	});

	function isToolInBuffer(item: Extract<ChatItem, { type: 'tool' }>) {
		return thinkingForAssistant.toolIdsInBuffer.has(item.id);
	}

	function thinkingExpanded(id: string) {
		return expandedThinking.has(id);
	}

	function toggleThinking(id: string) {
		expandedThinking = toggleExpanded(expandedThinking, id);
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

	function thinkingPending(block: ThinkingBlock) {
		return block.reasoning?.pending === true || block.tools.some((tool) => tool.pending);
	}
	let scrollKey = $derived(
		`${chatStore.isStreaming}:${threadItems
			.map((item) => {
				if (item.type === 'tool') {
					return `${item.id}:${item.output?.length ?? 0}:${item.pending ?? false}`;
				}
				if (item.type === 'assistant') {
					return `${item.id}:${item.text.length}:${item.reasoning?.text.length ?? 0}:${item.reasoning?.pending ?? false}:${item.pending ?? false}`;
				}
				if (item.type === 'user') {
					return `${item.id}:${item.text.length}:${item.reveal === false}`;
				}
				return `${item.id}:${'text' in item ? item.text.length : 0}:${item.type}`;
			})
			.join('|')}`
	);
	let renderDebugSnapshot = $derived.by(() => ({
		awaitingFirstAssistant,
		firstTurnFlightDone,
		isStreaming: chatStore.isStreaming,
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
		if (!chatStore.items.some((item) => item.type === 'tool' && item.pending)) return;
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

	function showToolOutputPanel(item: Extract<ChatItem, { type: 'tool' }>) {
		return Boolean(item.output || item.error || item.pending);
	}

	function showAssistantRow(item: Extract<ChatItem, { type: 'assistant' }>) {
		return Boolean(
			item.text ||
			item.reasoning?.text ||
			item.reasoning?.pending ||
			(item.pending && chatStore.isStreaming && !item.reasoning)
		);
	}

	function showTypingBubble(item: Extract<ChatItem, { type: 'assistant' }>) {
		return Boolean(chatStore.isStreaming && !item.text && !item.reasoning?.pending);
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
			showTypingBubble: showTypingBubble(item)
		};
	}

	function speakerFor(item: ChatItem): 'user' | 'assistant' | null {
		if (item.type === 'user') return 'user';
		if (item.type === 'assistant' || item.type === 'tool') return 'assistant';
		return null;
	}

	function showFirstTurnAvatarSlot() {
		if (!awaitingFirstAssistant || !firstUserId) return false;
		if (!firstTurnFlightDone) return true;
		if (!firstAssistantItem) return true;
		return !showAssistantRow(firstAssistantItem);
	}

	function firstAssistantInNormalList(item: Extract<ChatItem, { type: 'assistant' }>) {
		return !(
			awaitingFirstAssistant &&
			item.id === firstAssistantId &&
			firstUserId &&
			!(firstTurnFlightDone && showAssistantRow(item))
		);
	}

	function rowIntro<T extends object>(transition: T) {
		return isInitialTranscriptPaint || awaitingFirstAssistant ? undefined : transition;
	}

	function startsSpeakerRun(index: number, speaker: 'user' | 'assistant') {
		for (let i = index - 1; i >= 0; i--) {
			const previousSpeaker = speakerFor(chatStore.items[i]);
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
			isInitialTranscriptPaint = true;
			return;
		}
		if (chatStore.isLoading) {
			isInitialTranscriptPaint = true;
			return;
		}
		if (threadItems.length === 0) return;
		const frame = requestAnimationFrame(() => {
			isInitialTranscriptPaint = false;
		});
		return () => cancelAnimationFrame(frame);
	});

	$effect(() => {
		void scrollKey;
		if (scrollFrame) cancelAnimationFrame(scrollFrame);
		scrollFrame = requestAnimationFrame(() => {
			void tick().then(() => {
				scrollFrame = 0;
				if (!scroller) return;
				scroller.scrollTo({
					top: scroller.scrollHeight,
					behavior:
						chatStore.isStreaming || isInitialTranscriptPaint ? 'auto' : 'smooth'
				});
			});
		});
		return () => {
			if (scrollFrame) cancelAnimationFrame(scrollFrame);
		};
	});
</script>

{#snippet thinkingBlock(block: ThinkingBlock, hostId: string)}
	<div class="fold-panel thinking-panel">
		<button
			type="button"
			class="fold-toggle thinking-toggle"
			aria-expanded={thinkingExpanded(hostId)}
			onclick={() => toggleThinking(hostId)}
		>
			<Brain size={13} />
			<span>
				{block.tools.length > 0 ? `Thinking · ${block.tools.length} tool${block.tools.length === 1 ? '' : 's'}` : 'Thinking'}
			</span>
			{#if thinkingPending(block)}
				<LoaderCircle size={12} class="spin" />
			{/if}
			<ChevronDown size={13} class={thinkingExpanded(hostId) ? 'expanded' : ''} />
		</button>
		{#if thinkingExpanded(hostId)}
			<div class="fold-body thinking-body" transition:slide={FOLD_IN}>
				{#if block.reasoning}
					<div class="thinking-reasoning">
						<p>{block.reasoning.text || 'Thinking…'}</p>
					</div>
				{/if}
				{#if block.tools.length > 0}
					<div class="thinking-tools">
						{#each block.tools as tool (tool.id)}
							<div class="thinking-tool" class:error={!!tool.error} class:pending={tool.pending}>
								<div class="thinking-tool-header">
									<Terminal size={12} />
									<span class="thinking-tool-name">{tool.toolName}</span>
									{#if tool.pending}
										<LoaderCircle size={11} class="spin" />
									{:else}
										<CircleCheck size={11} />
									{/if}
								</div>
								{#if tool.output || tool.error}
									<div class="thinking-tool-output">
										{#if tool.output}<p>{tool.output}</p>{/if}
										{#if tool.error}<p class="tool-error-text">{tool.error}</p>{/if}
									</div>
								{/if}
							</div>
						{/each}
					</div>
				{/if}
			</div>
		{/if}
	</div>
{/snippet}

{#snippet assistantStack(item: Extract<ChatItem, { type: 'assistant' }>)}
	{@const block = thinkingForAssistant.map.get(item.id)}
	<div class="assistant-stack">
		{#if block && (block.reasoning || block.tools.length > 0)}
			{@render thinkingBlock(block, item.id)}
		{/if}
		{#if item.text}
			<div class="bubble assistant-bubble">
				<AssistantMarkdown source={item.text} streaming={item.id === streamingAssistantId} />
			</div>
			{#if item.id !== streamingAssistantId}
				<div class="message-actions">
					<button
						type="button"
						class="message-action"
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
		{:else if showTypingBubble(item)}
			<div class="bubble assistant-bubble pending">
				<span class="typing"><span></span><span></span><span></span></span>
			</div>
		{/if}
	</div>
{/snippet}

<div class="thread" bind:this={scroller} aria-live="polite">
	<div class="thread-inner">
		{#if showMessages}
			<div class="thread-messages" in:fade={transcriptFadeIn}>
				{#if awaitingFirstAssistant && !firstUserId}
			<div
				class="row assistant-row gap-2.5 md:gap-3 lg:gap-4 flight-placeholder"
				aria-hidden="true"
			>
				<div
					class="avatar-mini size-9 shrink-0 rounded-full border border-gray-400 md:size-10 lg:size-11 xl:size-12"
					class:avatar-flight-hidden={!firstTurnFlightDone}
					data-flight-target="avatar"
				>
					<img
						src="/project_avatar_96.png"
						srcset="/project_avatar_96.png 96w, /project_avatar_192.png 192w, /project_avatar_384.png 384w"
						sizes="(min-width: 1280px) 48px, (min-width: 1024px) 44px, (min-width: 768px) 40px, 36px"
						alt=""
					/>
				</div>
				{#if firstAssistantItem && showAssistantRow(firstAssistantItem)}
					{@render assistantStack(firstAssistantItem)}
				{:else}
					<div class="assistant-stack"></div>
				{/if}
			</div>
		{/if}

		{#each threadItems as item, index (item.id)}
			{#if item.type === 'user'}
				<div class="row user-row" class:continuation-row={!startsSpeakerRun(index, 'user')}>
					<div
						class="bubble user-bubble"
						class:flight-hidden={item.reveal === false}
						data-flight-target={item.reveal === false ? 'user' : undefined}
					>
						<AssistantMarkdown source={item.text} mode="user" />
					</div>
				</div>
				{#if showFirstTurnAvatarSlot() && item.id === firstUserId}
					<div
						class="row assistant-row gap-2.5 md:gap-3 lg:gap-4"
						class:flight-placeholder={!firstAssistantId}
						aria-hidden={!firstAssistantId}
					>
						<div
							class="avatar-mini size-9 shrink-0 rounded-full border border-gray-400 md:size-10 lg:size-11 xl:size-12"
							class:avatar-flight-hidden={!firstTurnFlightDone}
							data-flight-target="avatar"
						>
							<img
								src="/project_avatar_96.png"
								srcset="/project_avatar_96.png 96w, /project_avatar_192.png 192w, /project_avatar_384.png 384w"
								sizes="(min-width: 1280px) 48px, (min-width: 1024px) 44px, (min-width: 768px) 40px, 36px"
								alt=""
							/>
						</div>
						{#if firstAssistantItem && showAssistantRow(firstAssistantItem)}
							{@render assistantStack(firstAssistantItem)}
						{:else}
							<div class="assistant-stack"></div>
						{/if}
					</div>
				{/if}
			{:else if item.type === 'assistant' && showAssistantRow(item) && firstAssistantInNormalList(item)}
				<div
					class="row assistant-row gap-2.5 md:gap-3 lg:gap-4"
					class:continuation-row={!startsSpeakerRun(index, 'assistant')}
					in:fly={rowIntro(ASSISTANT_ROW_IN)}
				>
					{#if startsSpeakerRun(index, 'assistant')}
						<div
							class="avatar-mini size-9 shrink-0 rounded-full border border-gray-400 md:size-10 lg:size-11 xl:size-12"
						>
							<img
								src="/project_avatar_96.png"
								srcset="/project_avatar_96.png 96w, /project_avatar_192.png 192w, /project_avatar_384.png 384w"
								sizes="(min-width: 1280px) 48px, (min-width: 1024px) 44px, (min-width: 768px) 40px, 36px"
								alt=""
							/>
						</div>
					{:else}
						<div
							class="avatar-gutter size-9 shrink-0 md:size-10 lg:size-11 xl:size-12"
							aria-hidden="true"
						></div>
					{/if}
					{@render assistantStack(item)}
				</div>
			{:else if item.type === 'tool' && !isToolInBuffer(item)}
				<div
					class="row tool-row gap-2.5 md:gap-3 lg:gap-4"
					class:continuation-row={!startsSpeakerRun(index, 'assistant')}
					in:fly={rowIntro(TOOL_ROW_IN)}
				>
					<div
						class="avatar-gutter size-9 shrink-0 md:size-10 lg:size-11 xl:size-12"
						aria-hidden="true"
					></div>
					<div class="tool-stack">
						<div class="event-card tool-card" class:error={!!item.error}>
							<div class="tool-header">
								<div class="event-title">
									<Terminal size={14} />
									<span class="tool-name">{item.toolName}</span>
								</div>
								<div class="tool-meta">
									{#if showToolOutputPanel(item)}
										<button
											type="button"
											class="fold-toggle tool-output-toggle"
											aria-expanded={toolOutputExpanded(item)}
											onclick={() => toggleToolOutput(item.id)}
										>
											<span>Output</span>
											<ChevronDown
												size={12}
												class={toolOutputExpanded(item) ? 'expanded' : ''}
											/>
										</button>
									{/if}
									{#if toolDurationLabel(item)}
										<span class="tool-duration">{toolDurationLabel(item)}</span>
									{/if}
									{#if item.pending}
										<LoaderCircle size={13} class="spin" />
									{:else}
										<CircleCheck size={13} />
									{/if}
								</div>
							</div>
							{#if showToolOutputPanel(item) && toolOutputExpanded(item)}
								<div class="tool-output-body" transition:slide={FOLD_IN}>
									{#if item.output}
										<p>{item.output}</p>
									{/if}
									{#if item.error}
										<p class="tool-error-text">{item.error}</p>
									{/if}
									{#if item.pending && !item.output && !item.error}
										<p>Running…</p>
									{/if}
								</div>
							{/if}
						</div>
					</div>
				</div>
			{:else if item.type === 'status'}
				<div class="status" in:fly={rowIntro(STATUS_ROW_IN)}>{usageText(item)}</div>
			{:else if item.type === 'error'}
				<div class="row event-row" in:fly={rowIntro(TOOL_ROW_IN)}>
					<div class="event-card error-card">
						<div class="event-title"><TriangleAlert size={14} /><span>Error</span></div>
						<p>{item.text}</p>
					</div>
				</div>
			{/if}
		{/each}
			</div>
		{/if}
	</div>
</div>

<style>
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
		--chat-assistant-column: calc(var(--chat-content-column) * var(--chat-assistant-fill));
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
	}

	@media (min-width: 768px) {
		.thread {
			padding: 40px var(--chat-gutter) var(--thread-padding-bottom);
		}

		.thread-inner {
			gap: 16px;
		}

		.bubble {
			font-size: 15px;
		}
	}

	@media (min-width: 1024px) {
		.thread {
			padding: 48px var(--chat-gutter) var(--thread-padding-bottom);
		}

		.thread-inner {
			gap: 18px;
		}

		.bubble {
			font-size: 15px;
			padding: 12px 16px;
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
	}

	.continuation-row {
		margin-top: -6px;
	}

	.tool-row.continuation-row {
		margin-top: -16px;
	}

	.user-row {
		justify-content: flex-end;
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

	.avatar-gutter {
		flex: 0 0 auto;
	}

	.tool-stack {
		min-width: 0;
		flex: 1;
		max-width: calc(var(--chat-content-max) * 0.35);
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

	.avatar-flight-hidden {
		visibility: hidden;
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

	.fold-panel {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.fold-toggle {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		align-self: flex-start;
		border: 1px solid var(--border-soft);
		background: rgba(255, 255, 255, 0.72);
		color: var(--text-muted);
		border-radius: 999px;
		padding: 5px 10px;
		font-size: 12px;
		font-weight: 600;
		cursor: pointer;
	}

	.fold-toggle:hover {
		background: rgba(255, 255, 255, 0.92);
		color: var(--text-main);
	}

	.fold-toggle :global(svg.expanded) {
		transform: rotate(180deg);
	}

	.fold-body {
		border: 1px solid var(--border-soft);
		background: rgba(255, 255, 255, 0.68);
		border-radius: 12px;
		padding: 10px 12px;
		color: var(--text-muted);
		box-shadow: 0 6px 18px rgba(15, 23, 42, 0.04);
	}

	.fold-body p {
		margin: 0;
		font-size: 12px;
		line-height: 1.5;
		white-space: pre-wrap;
		word-break: break-word;
		max-height: 220px;
		overflow: auto;
		scrollbar-gutter: stable;
	}

	.thinking-body {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}

	.thinking-reasoning p {
		margin: 0;
		font-size: 12px;
		line-height: 1.5;
		white-space: pre-wrap;
		word-break: break-word;
		max-height: 220px;
		overflow: auto;
		scrollbar-gutter: stable;
		color: var(--text-muted);
	}

	.thinking-tools {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.thinking-tool {
		border: 1px solid var(--border-soft);
		border-radius: 10px;
		padding: 7px 9px;
		background: rgba(15, 23, 42, 0.02);
	}

	.thinking-tool.pending {
		background: rgba(255, 255, 255, 0.5);
	}

	.thinking-tool.error {
		background: rgba(255, 245, 245, 0.72);
		border-color: rgba(244, 63, 94, 0.18);
	}

	.thinking-tool-header {
		display: flex;
		align-items: center;
		gap: 5px;
		font-size: 11px;
		font-weight: 650;
		color: var(--text-main);
	}

	.thinking-tool-header :global(svg) {
		flex-shrink: 0;
		color: var(--text-muted);
	}

	.thinking-tool-name {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.thinking-tool-output {
		margin-top: 6px;
		padding-top: 6px;
		border-top: 1px solid var(--border-soft);
		color: var(--text-muted);
	}

	.thinking-tool-output p {
		margin: 0;
		font-size: 11px;
		line-height: 1.45;
		white-space: pre-wrap;
		word-break: break-word;
		max-height: 160px;
		overflow: auto;
		scrollbar-gutter: stable;
	}

	.thinking-tool-output p + p {
		margin-top: 6px;
		padding-top: 6px;
		border-top: 1px solid var(--border-soft);
	}

	.tool-card {
		padding: 8px 10px;
	}

	.tool-header {
		display: flex;
		align-items: center;
		gap: 10px;
		min-width: 0;
	}

	.tool-header .event-title {
		margin-bottom: 0;
		flex: 1;
		min-width: 0;
	}

	.tool-meta {
		display: flex;
		align-items: center;
		gap: 6px;
		flex-shrink: 0;
	}

	.tool-output-toggle {
		padding: 3px 8px;
		font-size: 11px;
	}

	.tool-duration {
		font-size: 11px;
		font-weight: 500;
		color: var(--text-soft);
		font-variant-numeric: tabular-nums;
	}

	.tool-output-body {
		margin-top: 8px;
		border: 1px solid var(--border-soft);
		background: rgba(15, 23, 42, 0.03);
		border-radius: 10px;
		padding: 8px 10px;
		color: var(--text-muted);
	}

	.tool-output-body p {
		margin: 0;
		font-size: 12px;
		line-height: 1.5;
		white-space: pre-wrap;
		word-break: break-word;
		max-height: 220px;
		overflow: auto;
		scrollbar-gutter: stable;
	}

	.tool-output-body p + p {
		margin-top: 8px;
		padding-top: 8px;
		border-top: 1px solid var(--border-soft);
	}

	.tool-error-text {
		color: #b42318;
	}

	.avatar-mini {
		flex: 0 0 auto;
		aspect-ratio: 1;
		background: linear-gradient(145deg, #ffffff, #eef2f6);
		box-shadow: 0 5px 14px rgba(15, 23, 42, 0.06);
		overflow: hidden;
	}

	@media (min-width: 1024px) {
		.avatar-mini {
			box-shadow: 0 6px 18px rgba(15, 23, 42, 0.08);
		}
	}

	.avatar-mini img {
		width: 100%;
		height: 100%;
		object-fit: cover;
		border-radius: 50%;
		display: block;
	}

	.bubble {
		max-width: 100%;
		border-radius: 18px;
		padding: 11px 14px;
		font-size: 14px;
		line-height: 1.55;
		white-space: pre-wrap;
		word-break: break-word;
	}

	.user-bubble {
		background: var(--user-bubble-bg);
		color: white;
		border-bottom-right-radius: 6px;
		box-shadow: 0 8px 20px var(--user-bubble-shadow);
		max-width: var(--chat-content-column);
	}

	.assistant-bubble {
		width: fit-content;
		max-width: 100%;
		background: rgba(255, 255, 255, 0.82);
		border: 1px solid var(--border-soft);
		border-bottom-left-radius: 6px;
		color: var(--text-main);
	}

	.assistant-bubble.pending {
		padding-block: 13px;
	}

	.message-actions {
		display: flex;
		align-items: center;
		gap: 4px;
		margin-top: -2px;
		opacity: 0;
		transition: opacity var(--duration-fast) var(--ease-smooth);
	}

	/* Reveal on hover/focus of the surrounding assistant stack. */
	.assistant-stack:hover .message-actions,
	.message-actions:focus-within {
		opacity: 1;
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

	.tool-name {
		min-width: 0;
		flex: 1;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.event-title :global(svg:last-child) {
		flex-shrink: 0;
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

	.typing {
		display: inline-flex;
		align-items: center;
		gap: 4px;
	}

	.typing span {
		width: 5px;
		height: 5px;
		border-radius: 50%;
		background: var(--text-soft);
		animation: pulse 1s ease-in-out infinite;
	}

	.typing span:nth-child(2) {
		animation-delay: 0.12s;
	}

	.typing span:nth-child(3) {
		animation-delay: 0.24s;
	}

	@keyframes pulse {
		0%,
		80%,
		100% {
			opacity: 0.35;
			transform: translateY(0);
		}
		40% {
			opacity: 1;
			transform: translateY(-2px);
		}
	}
</style>
