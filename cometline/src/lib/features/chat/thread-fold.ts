/** Pure auto-fold bookkeeping for a streaming assistant turn. */

export type StreamingFoldState = {
	autoExpandedTurns: Set<string>;
	autoCollapsedTurns: Set<string>;
	lastStreamingTurnId: string | null;
};

export function createStreamingFoldState(): StreamingFoldState {
	return {
		autoExpandedTurns: new Set(),
		autoCollapsedTurns: new Set(),
		lastStreamingTurnId: null
	};
}

export type StreamingFoldOverride = {
	turnId: string;
	expanded: boolean;
};

/**
 * Given the current streaming turn and whether the session is streaming,
 * returns at most one override to apply (expand-once while streaming,
 * collapse-once when streaming ends for the remembered turn).
 */
export function nextStreamingFoldOverride(
	state: StreamingFoldState,
	streamingAssistantId: string | null,
	sessionStreaming: boolean
): StreamingFoldOverride | null {
	if (streamingAssistantId && sessionStreaming) {
		state.lastStreamingTurnId = streamingAssistantId;
		if (!state.autoExpandedTurns.has(streamingAssistantId)) {
			state.autoExpandedTurns.add(streamingAssistantId);
			return { turnId: streamingAssistantId, expanded: true };
		}
		return null;
	}

	const finished = state.lastStreamingTurnId;
	if (finished && !state.autoCollapsedTurns.has(finished)) {
		state.autoCollapsedTurns.add(finished);
		state.lastStreamingTurnId = null;
		return { turnId: finished, expanded: false };
	}

	state.lastStreamingTurnId = null;
	return null;
}

export function resetStreamingFoldState(): StreamingFoldState {
	return createStreamingFoldState();
}

function toggleExpanded(set: Set<string>, id: string) {
	const next = new Set(set);
	if (next.has(id)) next.delete(id);
	else next.add(id);
	return next;
}

function toggleMapOverride(map: Map<string, boolean>, id: string, current: boolean) {
	const next = new Map(map);
	next.set(id, !current);
	return next;
}

export { toggleExpanded, toggleMapOverride };
