import {
	THINKING_VARIANT_STICKY_MS,
	stageForTurnPhase,
	variantForStage,
	type ThinkingIndicatorVariant,
	type ThinkingUiStage
} from './thinking-indicator';

export type StickyThinkingVariantResult = {
	variant: ThinkingIndicatorVariant;
	stage: ThinkingUiStage;
	/** Ms until a pending stage can commit; 0 if nothing pending. */
	commitInMs: number;
};

/**
 * Holds the displayed thinking variant until a new UI stage has been
 * stable for `holdMs` (default 750). First observe snaps immediately.
 */
export class StickyThinkingVariant {
	private displayedStage: ThinkingUiStage = 'thinking';
	private pendingStage: ThinkingUiStage | null = null;
	private pendingAt: number | null = null;
	private primed = false;

	constructor(
		private readonly holdMs: number = THINKING_VARIANT_STICKY_MS,
		private readonly now: () => number = () => Date.now()
	) {}

	get variant(): ThinkingIndicatorVariant {
		return variantForStage(this.displayedStage);
	}

	get stage(): ThinkingUiStage {
		return this.displayedStage;
	}

	observe(phase: string | undefined): StickyThinkingVariantResult {
		const target = stageForTurnPhase(phase);
		const at = this.now();

		if (!this.primed) {
			this.primed = true;
			this.displayedStage = target;
			this.pendingStage = null;
			this.pendingAt = null;
			return { variant: this.variant, stage: this.displayedStage, commitInMs: 0 };
		}

		if (target === this.displayedStage) {
			this.pendingStage = null;
			this.pendingAt = null;
			return { variant: this.variant, stage: this.displayedStage, commitInMs: 0 };
		}

		if (this.pendingStage !== target) {
			this.pendingStage = target;
			this.pendingAt = at;
			return { variant: this.variant, stage: this.displayedStage, commitInMs: this.holdMs };
		}

		const elapsed = at - (this.pendingAt ?? at);
		if (elapsed >= this.holdMs) {
			this.displayedStage = target;
			this.pendingStage = null;
			this.pendingAt = null;
			return { variant: this.variant, stage: this.displayedStage, commitInMs: 0 };
		}

		return {
			variant: this.variant,
			stage: this.displayedStage,
			commitInMs: this.holdMs - elapsed
		};
	}
}
