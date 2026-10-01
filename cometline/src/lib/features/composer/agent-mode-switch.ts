import type { AgentMode, Session } from '$lib/types';
import { normalizeAgentMode } from '$lib/sessions/session-metadata';

export interface AgentModeSwitchState {
	sessionId: string;
	mode: AgentMode;
	known: boolean;
	pending: boolean;
	queued: AgentMode | null;
	baseline: AgentMode;
}

export function createInitialAgentModeState(sessionId = ''): AgentModeSwitchState {
	return {
		sessionId,
		mode: 'auto',
		known: false,
		pending: false,
		queued: null,
		baseline: 'auto'
	};
}

export function nextAgentMode(current: AgentMode): AgentMode {
	return current === 'plan' ? 'auto' : 'plan';
}

export function sameAgentModeSwitchState(
	left: AgentModeSwitchState,
	right: AgentModeSwitchState
): boolean {
	return (
		left.sessionId === right.sessionId &&
		left.mode === right.mode &&
		left.known === right.known &&
		left.pending === right.pending &&
		left.queued === right.queued &&
		left.baseline === right.baseline
	);
}

/** Bind persisted mode only when the composer enters a session. */
export function bindAgentModeForSession(
	session: Session | undefined,
	sessionId: string
): AgentModeSwitchState {
	if (!sessionId) return createInitialAgentModeState();
	const mode = normalizeAgentMode(session?.agent_mode);
	return {
		sessionId,
		mode,
		known: true,
		pending: false,
		queued: null,
		baseline: mode
	};
}

export function beginAgentModeRequest(
	state: AgentModeSwitchState,
	next: AgentMode
): { state: AgentModeSwitchState; shouldPersist: boolean } {
	if (state.pending) {
		return {
			state: { ...state, mode: next, queued: next, known: true },
			shouldPersist: false
		};
	}
	if (next === state.mode) {
		return { state, shouldPersist: false };
	}
	return {
		state: { ...state, mode: next, pending: true, queued: null, known: true },
		shouldPersist: true
	};
}

export function completeAgentModeRequest(
	state: AgentModeSwitchState,
	attempted: AgentMode,
	result: { ok: true; mode: AgentMode } | { ok: false }
): { state: AgentModeSwitchState; shouldPersist: AgentMode | null } {
	if (result.ok) {
		const baseline = result.mode;
		if (state.queued && state.queued !== baseline) {
			return {
				state: {
					...state,
					mode: state.queued,
					known: true,
					pending: true,
					queued: null,
					baseline
				},
				shouldPersist: state.queued
			};
		}
		return {
			state: {
				...state,
				mode: baseline,
				known: true,
				pending: false,
				queued: null,
				baseline
			},
			shouldPersist: null
		};
	}

	if (state.queued && state.queued !== attempted) {
		return {
			state: {
				...state,
				mode: state.queued,
				pending: true,
				queued: null,
				known: true
			},
			shouldPersist: state.queued
		};
	}

	return {
		state: {
			...state,
			mode: state.baseline,
			known: true,
			pending: false,
			queued: null,
			baseline: state.baseline
		},
		shouldPersist: null
	};
}

export function agentModeAnnouncement(mode: AgentMode): string {
	return mode === 'plan' ? 'Plan mode enabled' : 'Auto mode enabled';
}
