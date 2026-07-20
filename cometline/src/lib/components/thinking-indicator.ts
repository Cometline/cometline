import type { TurnStatusPhase } from '$lib/conversation/turn-status';

export type ThinkingIndicatorVariant = 'orbit' | 'eclipse' | 'nova';

export const THINKING_INDICATOR_VARIANTS: readonly ThinkingIndicatorVariant[] = [
	'orbit',
	'eclipse',
	'nova'
] as const;

/** Coarse UI stages — wire phases map here before picking a variant. */
export type ThinkingUiStage = 'thinking' | 'tools' | 'composing';

export const THINKING_VARIANT_STICKY_MS = 750;

const PHASE_STAGE: Record<TurnStatusPhase, ThinkingUiStage> = {
	retrieving_memories: 'thinking',
	compacting_context: 'thinking',
	contacting_model: 'thinking',
	continuing: 'thinking',
	running_tools: 'tools',
	composing_response: 'composing'
};

const STAGE_VARIANT: Record<ThinkingUiStage, ThinkingIndicatorVariant> = {
	thinking: 'orbit',
	tools: 'eclipse',
	composing: 'nova'
};

export function stageForTurnPhase(phase: string | undefined): ThinkingUiStage {
	if (!phase) return 'thinking';
	return PHASE_STAGE[phase as TurnStatusPhase] ?? 'thinking';
}

export function variantForStage(stage: ThinkingUiStage): ThinkingIndicatorVariant {
	return STAGE_VARIANT[stage];
}

export function variantForTurnPhase(phase: string | undefined): ThinkingIndicatorVariant {
	return variantForStage(stageForTurnPhase(phase));
}
