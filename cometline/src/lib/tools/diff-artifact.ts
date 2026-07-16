/**
 * DiffArtifact wire contract — keep markers in sync with
 * cometmind/internal/tools/diffartifact/diffartifact.go.
 */
export const DIFF_BEGIN_MARKER = '*** Begin Diff';
export const DIFF_END_MARKER = '*** End Diff';

export const AGENT_LABEL_RESEARCH = 'cometmind';
export const AGENT_LABEL_CODING = 'cometmind-coding';

export const SESSION_KIND_RESEARCH = 'general';
export const SESSION_KIND_CODING = 'coding';
export const SESSION_KIND_ACP = 'acp';

export type DiffLineKind = 'meta' | 'hunk' | 'add' | 'del' | 'ctx' | 'other';

export type ParsedDiffLine = {
	kind: DiffLineKind;
	text: string;
};

/** Structured edit_file diff (UI side of DiffArtifact). */
export type DiffArtifact = {
	summary: string;
	path?: string;
	added?: number;
	deleted?: number;
	lines: ParsedDiffLine[];
};

/** @deprecated Use DiffArtifact */
export type ParsedEditDiff = DiffArtifact;

/** Display agentName for a persisted subagent_kind. */
export function agentLabelForSessionKind(kind: string | undefined | null): string {
	switch (kind) {
		case SESSION_KIND_CODING:
			return AGENT_LABEL_CODING;
		case SESSION_KIND_RESEARCH:
		case '':
		case undefined:
		case null:
			return AGENT_LABEL_RESEARCH;
		default:
			return '';
	}
}

export function looksLikeDiffArtifact(output: string | undefined | null): boolean {
	if (!output) return false;
	return output.includes(DIFF_BEGIN_MARKER);
}
