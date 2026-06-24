import { untrack } from 'svelte';
import type { ChatItem } from '$lib/stores/chat.svelte';
import { defaultActivityGroupExpanded, defaultThinkingExpanded } from './thinking-attribution';
import {
	createStreamingFoldState,
	nextStreamingFoldOverride,
	resetStreamingFoldState,
	toggleExpanded,
	toggleMapOverride
} from './thread-fold';
import { isJobProposalDismissed } from '$lib/jobs/job-proposal-dismissals';
import { parseJobProposal } from '$lib/jobs/parse-job-proposal';

export interface FoldControllerDeps {
	getSessionId: () => string;
	getIsSessionSynced: () => boolean;
	getItems: () => readonly ChatItem[];
	getStreamingAssistantId: () => string | null;
	getSessionStreaming: () => boolean;
}

export function createFoldController(deps: FoldControllerDeps) {
	let thinkingOverrides = $state(new Map<string, boolean>());
	let activityGroupOverrides = $state(new Map<string, boolean>());
	let expandedToolOutput = $state(new Set<string>());
	let proposeJobAutoExpanded = $state(new Set<string>());
	let expandedMemoryInThinking = $state(new Set<string>());
	let subagentFold = $state(new Map<string, boolean>());

	let streamingFoldState = createStreamingFoldState();

	function setActivityGroupOverride(turnId: string, value: boolean) {
		const next = new Map(activityGroupOverrides);
		next.set(turnId, value);
		activityGroupOverrides = next;
	}

	function thinkingExpanded(
		assistant: Extract<ChatItem, { type: 'assistant' }>,
		segmentKey: string,
		segmentIndex: number,
		pending?: boolean
	) {
		const override = thinkingOverrides.get(segmentKey);
		if (override !== undefined) return override;
		return defaultThinkingExpanded(
			segmentIndex,
			pending,
			assistant,
			deps.getStreamingAssistantId(),
			deps.getSessionStreaming()
		);
	}

	function toggleThinking(
		assistant: Extract<ChatItem, { type: 'assistant' }>,
		segmentKey: string,
		segmentIndex: number,
		pending?: boolean
	) {
		thinkingOverrides = toggleMapOverride(
			thinkingOverrides,
			segmentKey,
			thinkingExpanded(assistant, segmentKey, segmentIndex, pending)
		);
	}

	function activityGroupExpanded(
		assistantId: string,
		item: Extract<ChatItem, { type: 'assistant' }>
	) {
		const override = activityGroupOverrides.get(assistantId);
		if (override !== undefined) return override;
		return defaultActivityGroupExpanded(
			item,
			deps.getStreamingAssistantId(),
			deps.getSessionStreaming()
		);
	}

	function toggleActivityGroup(
		assistantId: string,
		item: Extract<ChatItem, { type: 'assistant' }>
	) {
		activityGroupOverrides = toggleMapOverride(
			activityGroupOverrides,
			assistantId,
			activityGroupExpanded(assistantId, item)
		);
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
		subagentFold = toggleMapOverride(subagentFold, id, subagentExpanded(id));
	}

	function toolOutputExpanded(item: Extract<ChatItem, { type: 'tool' }>) {
		return expandedToolOutput.has(item.id);
	}

	function toggleToolOutput(id: string) {
		expandedToolOutput = toggleExpanded(expandedToolOutput, id);
	}

	function resetForSession() {
		thinkingOverrides = new Map();
		activityGroupOverrides = new Map();
		expandedToolOutput = new Set();
		proposeJobAutoExpanded = new Set();
		streamingFoldState = resetStreamingFoldState();
	}

	$effect(() => {
		if (!deps.getIsSessionSynced()) return;
		const items = deps.getItems();
		const sessionId = deps.getSessionId();
		let nextExpanded: Set<string> | null = null;
		let nextAutoExpanded: Set<string> | null = null;
		for (const item of items) {
			if (
				item.type !== 'tool' ||
				item.toolName !== 'propose_job' ||
				item.pending ||
				item.error ||
				proposeJobAutoExpanded.has(item.id)
			) {
				continue;
			}
			nextAutoExpanded ??= new Set(proposeJobAutoExpanded);
			nextAutoExpanded.add(item.id);
			const proposal = parseJobProposal(item.input, item.output);
			if (proposal && sessionId && isJobProposalDismissed(sessionId, proposal)) {
				continue;
			}
			if (!expandedToolOutput.has(item.id)) {
				nextExpanded ??= new Set(expandedToolOutput);
				nextExpanded.add(item.id);
			}
		}
		if (nextAutoExpanded) proposeJobAutoExpanded = nextAutoExpanded;
		if (nextExpanded) expandedToolOutput = nextExpanded;
	});

	$effect(() => {
		void deps.getSessionId();
		untrack(() => resetForSession());
	});

	$effect(() => {
		const id = deps.getStreamingAssistantId();
		const streaming = deps.getSessionStreaming();
		untrack(() => {
			const override = nextStreamingFoldOverride(streamingFoldState, id, streaming);
			if (override) setActivityGroupOverride(override.turnId, override.expanded);
		});
	});

	return {
		get thinkingOverrides() {
			return thinkingOverrides;
		},
		get activityGroupOverrides() {
			return activityGroupOverrides;
		},
		get expandedToolOutput() {
			return expandedToolOutput;
		},
		get expandedMemoryInThinking() {
			return expandedMemoryInThinking;
		},
		get subagentFold() {
			return subagentFold;
		},
		thinkingExpanded,
		toggleThinking,
		activityGroupExpanded,
		toggleActivityGroup,
		memoryInThinkingExpanded,
		toggleMemoryInThinking,
		subagentExpanded,
		toggleSubagent,
		toolOutputExpanded,
		toggleToolOutput
	};
}
