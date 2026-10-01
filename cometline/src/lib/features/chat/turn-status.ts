export type TurnStatusPhase =
	| 'retrieving_memories'
	| 'compacting_context'
	| 'contacting_model'
	| 'composing_response'
	| 'running_tools'
	| 'continuing';

export const TURN_STATUS_LABELS: Record<TurnStatusPhase, string> = {
	retrieving_memories: 'Retrieving relevant memories…',
	compacting_context: 'Summarizing earlier context…',
	contacting_model: 'Thinking…',
	composing_response: 'Composing response…',
	running_tools: 'Running tools…',
	continuing: 'Continuing…'
};

export function turnStatusLabel(
	phase: string | undefined,
	message: string | undefined
): string | undefined {
	const custom = String(message ?? '').trim();
	if (custom) return custom;
	if (!phase) return undefined;
	return TURN_STATUS_LABELS[phase as TurnStatusPhase] ?? phase;
}
