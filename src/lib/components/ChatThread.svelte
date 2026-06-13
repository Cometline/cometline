<script lang="ts">
	import { tick } from 'svelte';
	import { fade, fly } from 'svelte/transition';
	import { Brain, CheckCircle2, ChevronDown, Loader2, Terminal, TriangleAlert } from '@lucide/svelte';
	import { chatStore, type ChatItem } from '$lib/stores/chat.svelte';

	let { awaitingFirstAssistant = false, firstTurnFlightDone = false }: {
		awaitingFirstAssistant?: boolean;
		firstTurnFlightDone?: boolean;
	} = $props();

	let scroller: HTMLDivElement;
	let scrollFrame = 0;
	let expandedReasoning = $state(new Set<string>());
	let expandedToolOutput = $state(new Set<string>());
	let firstAssistantId = $derived(
		chatStore.items.find((item) => item.type === 'assistant')?.id ?? null
	);
	let firstUserId = $derived(
		chatStore.items.find((item) => item.type === 'user')?.id ?? null
	);
	let firstAssistantItem = $derived(
		chatStore.items.find((item) => item.type === 'assistant') as
			| Extract<ChatItem, { type: 'assistant' }>
			| undefined
	);
	let scrollKey = $derived(
		`${chatStore.isStreaming}:${chatStore.items
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

	function pretty(value: unknown) {
		if (value == null) return '';
		if (typeof value === 'string') return value;
		try {
			return JSON.stringify(value, null, 2);
		} catch {
			return String(value);
		}
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

	function toggleReasoning(id: string) {
		expandedReasoning = toggleExpanded(expandedReasoning, id);
	}

	function toggleToolOutput(id: string) {
		expandedToolOutput = toggleExpanded(expandedToolOutput, id);
	}

	function reasoningExpanded(item: Extract<ChatItem, { type: 'assistant' }>) {
		return Boolean(
			item.reasoning &&
				(expandedReasoning.has(item.id) || (item.reasoning.pending && chatStore.isStreaming))
		);
	}

	function toolOutputExpanded(item: Extract<ChatItem, { type: 'tool' }>) {
		return expandedToolOutput.has(item.id) || (item.pending && chatStore.isStreaming);
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
		return Boolean(
			chatStore.isStreaming && !item.text && !item.reasoning?.pending
		);
	}

	$effect(() => {
		scrollKey;
		if (scrollFrame) cancelAnimationFrame(scrollFrame);
		scrollFrame = requestAnimationFrame(() => {
			void tick().then(() => {
				scrollFrame = 0;
				if (!scroller) return;
				scroller.scrollTo({
					top: scroller.scrollHeight,
					behavior: chatStore.isStreaming ? 'auto' : 'smooth'
				});
			});
		});
		return () => {
			if (scrollFrame) cancelAnimationFrame(scrollFrame);
		};
	});
</script>

{#snippet assistantStack(item: Extract<ChatItem, { type: 'assistant' }>)}
	<div class="assistant-stack">
		{#if item.text}
			<div class="bubble assistant-bubble">
				{item.text}
			</div>
		{:else if showTypingBubble(item)}
			<div class="bubble assistant-bubble pending">
				<span class="typing"><span></span><span></span><span></span></span>
			</div>
		{/if}
		{#if item.reasoning}
			<div class="fold-panel">
				<button
					type="button"
					class="fold-toggle"
					aria-expanded={reasoningExpanded(item)}
					onclick={() => toggleReasoning(item.id)}
				>
					<Brain size={13} />
					<span>Reasoning</span>
					{#if item.reasoning.pending && chatStore.isStreaming}
						<Loader2 size={12} class="spin" />
					{/if}
					<ChevronDown size={13} class={reasoningExpanded(item) ? 'expanded' : ''} />
				</button>
				{#if reasoningExpanded(item)}
					<div class="fold-body" transition:fade={{ duration: 120 }}>
						<p>{item.reasoning.text || 'Thinking…'}</p>
					</div>
				{/if}
			</div>
		{/if}
	</div>
{/snippet}

<div class="thread" bind:this={scroller} aria-live="polite">
	<div class="thread-inner">
		{#if chatStore.isLoading && chatStore.items.length === 0}
			<div class="loading" transition:fade={{ duration: 140 }}>
				<Loader2 size={15} class="spin" />
				<span>Loading conversation…</span>
			</div>
		{/if}

		{#if awaitingFirstAssistant && !firstUserId}
			<div
				class="row assistant-row gap-2.5 md:gap-3 lg:gap-4 flight-placeholder"
				aria-hidden="true"
			>
				<div
					class="avatar-mini size-9 shrink-0 md:size-10 lg:size-11 xl:size-12"
					class:avatar-flight-hidden={!firstTurnFlightDone}
					data-flight-target="avatar"
				>
					<img src="/project_icon.png" alt="" />
				</div>
				<div class="assistant-stack"></div>
			</div>
		{/if}

		{#each chatStore.items as item (item.id)}
			{#if item.type === 'user'}
				<div
					class="row user-row"
					transition:fly={item.reveal === false ? undefined : { y: 8, duration: 160 }}
				>
					<div
						class="bubble user-bubble"
						class:flight-hidden={item.reveal === false}
						data-flight-target={item.reveal === false ? 'user' : undefined}
					>
						{item.text}
					</div>
				</div>
				{#if awaitingFirstAssistant && item.id === firstUserId}
					<div
						class="row assistant-row gap-2.5 md:gap-3 lg:gap-4"
						class:flight-placeholder={!firstAssistantId}
						aria-hidden={!firstAssistantId}
					>
						<div
							class="avatar-mini size-9 shrink-0 md:size-10 lg:size-11 xl:size-12"
							class:avatar-flight-hidden={!firstTurnFlightDone}
							data-flight-target="avatar"
						>
							<img src="/project_icon.png" alt="" />
						</div>
						{#if firstAssistantItem && showAssistantRow(firstAssistantItem)}
							{@render assistantStack(firstAssistantItem)}
						{:else}
							<div class="assistant-stack"></div>
						{/if}
					</div>
				{/if}
			{:else if item.type === 'assistant' && showAssistantRow(item) && !(awaitingFirstAssistant && item.id === firstAssistantId)}
				<div
					class="row assistant-row gap-2.5 md:gap-3 lg:gap-4"
					transition:fly={item.id === firstAssistantId ? undefined : { y: 8, duration: 180 }}
				>
					<div class="avatar-mini size-9 shrink-0 md:size-10 lg:size-11 xl:size-12">
						<img src="/project_icon.png" alt="" />
					</div>
					{@render assistantStack(item)}
				</div>
			{:else if item.type === 'tool'}
				<div class="row tool-row gap-2.5 md:gap-3 lg:gap-4" transition:fly={{ y: 6, duration: 160 }}>
					<div class="avatar-gutter size-9 shrink-0 md:size-10 lg:size-11 xl:size-12" aria-hidden="true"></div>
					<div class="tool-stack">
						<div class="event-card tool-card" class:error={!!item.error}>
							<div class="event-title">
								<Terminal size={14} />
								<span class="tool-name">{item.toolName}</span>
								{#if item.pending}
									<Loader2 size={13} class="spin" />
								{:else}
									<CheckCircle2 size={13} />
								{/if}
							</div>
							{#if pretty(item.input)}
								<pre class="tool-input">{pretty(item.input)}</pre>
							{/if}
							{#if showToolOutputPanel(item)}
								<div class="fold-panel tool-output-panel">
									<button
										type="button"
										class="fold-toggle"
										aria-expanded={toolOutputExpanded(item)}
										onclick={() => toggleToolOutput(item.id)}
									>
										<span>Output</span>
										{#if item.pending}
											<Loader2 size={12} class="spin" />
										{/if}
										<ChevronDown size={13} class={toolOutputExpanded(item) ? 'expanded' : ''} />
									</button>
									{#if toolOutputExpanded(item)}
										<div class="fold-body tool-output-body" transition:fade={{ duration: 120 }}>
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
							{/if}
						</div>
					</div>
				</div>
			{:else if item.type === 'status'}
				<div class="status" transition:fade={{ duration: 140 }}>{usageText(item)}</div>
			{:else if item.type === 'error'}
				<div class="row event-row" transition:fly={{ y: 6, duration: 160 }}>
					<div class="event-card error-card">
						<div class="event-title"><TriangleAlert size={14} /><span>Error</span></div>
						<p>{item.text}</p>
					</div>
				</div>
			{/if}
		{/each}
	</div>
</div>

<style>
	.thread {
		position: absolute;
		inset: 0 0 178px;
		overflow-y: auto;
		padding: 32px 20px 20px;
		scrollbar-width: thin;
	}

	.thread-inner {
		width: min(760px, 100%);
		margin: 0 auto;
		display: flex;
		flex-direction: column;
		gap: 14px;
	}

	@media (min-width: 768px) {
		.thread {
			padding: 40px 32px 24px;
		}

		.thread-inner {
			gap: 16px;
		}

		.assistant-stack {
			max-width: min(640px, 84%);
		}

		.bubble {
			font-size: 15px;
		}
	}

	@media (min-width: 1024px) {
		.thread {
			padding: 48px 40px 24px;
		}

		.thread-inner {
			gap: 18px;
			width: min(800px, 100%);
		}

		.assistant-stack {
			max-width: min(680px, 86%);
		}

		.bubble {
			font-size: 15px;
			padding: 12px 16px;
		}
	}

	@media (min-width: 1280px) {
		.thread {
			padding: 56px 48px 28px;
		}

		.thread-inner {
			width: min(840px, 100%);
		}
	}

	.loading,
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
		max-width: min(620px, 82%);
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
		max-width: min(620px, 82%);
		min-width: 0;
		flex: 1;
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
	}

	.fold-body p + p {
		margin-top: 8px;
		padding-top: 8px;
		border-top: 1px solid var(--border-soft);
	}

	.tool-output-panel {
		margin-top: 8px;
	}

	.tool-error-text {
		color: #b42318;
	}

	.avatar-mini {
		flex: 0 0 auto;
		aspect-ratio: 1;
		border-radius: 50%;
		background: linear-gradient(145deg, #ffffff, #eef2f6);
		border: 1px solid var(--border-soft);
		display: grid;
		place-items: center;
		box-shadow: 0 5px 14px rgba(15, 23, 42, 0.06);
		overflow: hidden;
		padding: 3px;
	}

	@media (min-width: 1024px) {
		.avatar-mini {
			padding: 4px;
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
		background: #1f2933;
		color: white;
		border-bottom-right-radius: 6px;
		box-shadow: 0 8px 20px rgba(31, 41, 51, 0.12);
	}

	.assistant-bubble {
		background: rgba(255, 255, 255, 0.82);
		border: 1px solid var(--border-soft);
		border-bottom-left-radius: 6px;
		color: var(--text-main);
	}

	.assistant-bubble.pending {
		padding-block: 13px;
	}

	.event-row .event-card {
		max-width: min(620px, 82%);
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
		margin-left: auto;
	}

	.error-card p {
		margin: 0;
		font-size: 12px;
		line-height: 1.5;
		white-space: pre-wrap;
	}

	.tool-card .tool-input {
		margin: 0 0 8px;
		padding: 8px 10px;
		border-radius: 9px;
		background: rgba(15, 23, 42, 0.04);
		font-size: 11px;
		line-height: 1.45;
		white-space: pre-wrap;
		word-break: break-word;
		overflow-x: auto;
		color: var(--text-muted);
	}

	.tool-card .tool-input:last-child {
		margin-bottom: 0;
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

	:global(.spin) {
		animation: spin 0.8s linear infinite;
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

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	@media (max-width: 767px) {
		.thread {
			inset: 0 0 158px;
		}

		.bubble,
		.assistant-stack,
		.tool-stack {
			max-width: 92%;
		}
	}
</style>
