import type { AssistantStackContext } from './assistant-stack-props';
import type { ThinkingAttribution } from './thinking-attribution';

const emptyThinking: ThinkingAttribution = {
	map: new Map(),
	toolIdsInBuffer: new Set(),
	subagentIdsInBuffer: new Set(),
	memoryIdsInBuffer: new Set()
};

export function mockChatTurnContext(
	overrides?: Partial<AssistantStackContext>
): AssistantStackContext {
	const foldExpanded = new Map<string, boolean>();

	return {
		threadItems: [],
		thinkingForAssistant: emptyThinking,
		streamingAssistantId: null,
		sessionStreaming: false,
		sessionId: 'session-test',
		now: Date.now(),
		heroGlowColor: '#6366f1',
		copiedId: null,
		fold: {
			thinkingExpanded: () => false,
			toggleThinking: () => {},
			activityGroupExpanded: () => false,
			toggleActivityGroup: () => {},
			memoryInThinkingExpanded: () => false,
			toggleMemoryInThinking: () => {},
			toolOutputExpanded: () => foldExpanded.get('tool') ?? false,
			toggleToolOutput: (id) => {
				foldExpanded.set('tool', !foldExpanded.get('tool'));
				void id;
			},
			subagentExpanded: () => false,
			toggleSubagent: () => {}
		},
		toolFoldLabel: (tool) => tool.toolName ?? 'Tool',
		onCopyMessage: () => {},
		...overrides
	};
}
