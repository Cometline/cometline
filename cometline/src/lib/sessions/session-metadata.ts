import type { AgentMode, Session } from '$lib/types';

/** Session snapshots that are not exactly `plan` behave as Auto. */
export function normalizeAgentMode(value: Session['agent_mode'] | undefined): AgentMode {
	return value === 'plan' ? 'plan' : 'auto';
}

/** Fields a metadata refresh may write. Agent mode is never among them. */
export type SessionMetadataPatch = Pick<Session, 'title' | 'token_usage'> &
	Partial<Pick<Session, 'updated_at'>>;

export function applySessionMetadata(existing: Session, patch: SessionMetadataPatch): Session {
	return {
		...existing,
		title: patch.title,
		token_usage: patch.token_usage,
		updated_at: patch.updated_at ?? existing.updated_at
	};
}
