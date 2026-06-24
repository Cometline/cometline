import { turnStatusLabel, type TurnStatusPhase } from './turn-status';

export type AssistantThinkingWaitStatus = {
	label: string;
	detail: string;
};

const PHASE_DETAIL: Partial<Record<TurnStatusPhase, string>> = {
	retrieving_memories: 'Retrieving relevant memories…',
	compacting_context: 'Summarizing earlier context…',
	contacting_model: 'Thinking…',
	composing_response: 'Composing response…',
	running_tools: 'Running tools…',
	continuing: 'Continuing…'
};

function phaseDetail(
	phase: string | undefined,
	activityMessage: string | undefined
): string | undefined {
	const custom = String(activityMessage ?? '').trim();
	if (custom) return custom;
	if (!phase) return undefined;
	return PHASE_DETAIL[phase as TurnStatusPhase] ?? turnStatusLabel(phase, activityMessage);
}

/** User-facing copy while the assistant turn has not produced visible output yet. */
export function assistantThinkingWaitStatus(
	phase: string | undefined,
	activityMessage: string | undefined,
	seconds: number
): AssistantThinkingWaitStatus {
	const statusDetail = phaseDetail(phase, activityMessage);
	if (statusDetail) {
		return { label: 'Thinking', detail: statusDetail };
	}

	if (seconds >= 90) {
		return {
			label: 'Still thinking',
			detail: `This is taking longer than usual (${seconds}s). It may time out soon.`
		};
	}
	if (seconds >= 30) {
		return {
			label: 'Still thinking',
			detail: `Working through a slower response (${seconds}s).`
		};
	}
	if (seconds >= 8) {
		return {
			label: 'Still thinking',
			detail: seconds > 0 ? `${seconds}s` : 'Thinking…'
		};
	}

	return { label: 'Thinking', detail: 'Thinking…' };
}
