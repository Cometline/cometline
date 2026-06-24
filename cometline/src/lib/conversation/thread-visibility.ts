import {
	buildAssistantTimeline,
	pinnedJobProposalsForAssistant,
	type ThinkingAttribution
} from './thinking-attribution';
import { hasReasoning } from './reasoning';
import type { ChatItem } from '$lib/stores/chat.svelte';

type AssistantItem = Extract<ChatItem, { type: 'assistant' }>;

export interface ThreadVisibilityContext {
	threadItems: readonly ChatItem[];
	thinkingForAssistant: ThinkingAttribution;
	streamingAssistantId: string | null;
	sessionStreaming: boolean;
	awaitingFirstAssistant: boolean;
	firstTurnFlightDone: boolean;
	firstTurnHandoffPending: boolean;
	firstUserId: string | null;
	firstAssistantRowId: string | null;
	firstAssistantItem: AssistantItem | undefined;
}

export function hasVisibleThinkingBlock(
	itemId: string,
	threadItems: readonly ChatItem[],
	thinkingForAssistant: ThinkingAttribution
) {
	return buildAssistantTimeline(itemId, threadItems, thinkingForAssistant).length > 0;
}

export function showAssistantActivitySpinner(
	item: AssistantItem,
	streamingAssistantId: string | null,
	sessionStreaming: boolean
) {
	return sessionStreaming && item.id === streamingAssistantId;
}

export function showAssistantPending(
	item: AssistantItem,
	ctx: Pick<
		ThreadVisibilityContext,
		'threadItems' | 'thinkingForAssistant' | 'streamingAssistantId' | 'sessionStreaming'
	>
) {
	if (!ctx.sessionStreaming || item.id !== ctx.streamingAssistantId) return false;
	if (item.text?.trim()) return false;
	return !hasVisibleThinkingBlock(item.id, ctx.threadItems, ctx.thinkingForAssistant);
}

export function showAssistantRow(item: AssistantItem, ctx: ThreadVisibilityContext) {
	return Boolean(
		item.text ||
		hasReasoning(item) ||
		hasVisibleThinkingBlock(item.id, ctx.threadItems, ctx.thinkingForAssistant) ||
		pinnedJobProposalsForAssistant(item.id, ctx.threadItems).length > 0 ||
		showAssistantPending(item, ctx) ||
		showAssistantActivitySpinner(item, ctx.streamingAssistantId, ctx.sessionStreaming)
	);
}

export function showFirstTurnAvatarSlot(ctx: ThreadVisibilityContext) {
	if (!ctx.firstUserId) return false;
	if (ctx.firstTurnHandoffPending) return true;
	if (!ctx.awaitingFirstAssistant) return false;
	if (!ctx.firstTurnFlightDone) return true;
	if (!ctx.firstAssistantItem) return true;
	return !showAssistantRow(ctx.firstAssistantItem, ctx);
}

export function firstAssistantInNormalList(item: AssistantItem, ctx: ThreadVisibilityContext) {
	if (showFirstTurnAvatarSlot(ctx)) return false;
	return !(
		ctx.awaitingFirstAssistant &&
		item.id === ctx.firstAssistantRowId &&
		ctx.firstUserId &&
		!(ctx.firstTurnFlightDone && showAssistantRow(item, ctx))
	);
}

export function hideAssistantAvatarForFirstTurn(
	item: AssistantItem,
	firstTurnHandoffPending: boolean,
	firstAssistantRowId: string | null
) {
	return firstTurnHandoffPending && item.id === firstAssistantRowId;
}

export function hideFirstTurnDestination(firstTurnHandoffPending: boolean) {
	return firstTurnHandoffPending;
}
