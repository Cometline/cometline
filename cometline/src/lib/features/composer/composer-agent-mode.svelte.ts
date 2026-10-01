import { updateSession } from '$lib/client/cometmind';
import { sessionStore } from '$lib/stores/session.svelte';
import { normalizeAgentMode } from '$lib/sessions/session-metadata';
import type { AgentMode } from '$lib/types';
import {
	agentModeAnnouncement,
	beginAgentModeRequest,
	completeAgentModeRequest,
	createInitialAgentModeState,
	nextAgentMode,
	sameAgentModeSwitchState,
	bindAgentModeForSession,
	type AgentModeSwitchState
} from '$lib/features/composer/agent-mode-switch';

/**
 * Agent mode is persisted on the session, with a local switch machine so Tab
 * can queue the latest request and stale store snapshots cannot snap back.
 */
export function createComposerAgentModeController(deps: { getSessionId: () => string }) {
	let agentModeState = $state<AgentModeSwitchState>(createInitialAgentModeState());
	let boundAgentModeFromStore = $state(false);
	let modeAnnouncement = $state('');
	const agentMode = $derived(agentModeState.mode);
	const agentModeKnown = $derived(agentModeState.known);

	function applyAgentModeState(next: AgentModeSwitchState) {
		if (sameAgentModeSwitchState(agentModeState, next)) return;
		agentModeState = next;
	}

	function resetStoreBinding() {
		boundAgentModeFromStore = false;
	}

	// Bind persisted mode when entering a session, or when that session first
	// appears in the store. Later store writes do not own the chip.
	function bindFromStore() {
		const id = deps.getSessionId();
		const session = id ? sessionStore.sessions.find((item) => item.id === id) : undefined;
		if (id && !session) {
			boundAgentModeFromStore = false;
			return;
		}
		if (boundAgentModeFromStore) return;
		applyAgentModeState(bindAgentModeForSession(session, id));
		boundAgentModeFromStore = Boolean(id);
	}

	async function persistAgentMode(id: string, next: AgentMode) {
		try {
			const updated = await updateSession(id, { agent_mode: next });
			sessionStore.updateSession(updated);
			if (deps.getSessionId() !== id) return;
			const settled = completeAgentModeRequest(agentModeState, next, {
				ok: true,
				mode: normalizeAgentMode(updated.agent_mode)
			});
			applyAgentModeState(settled.state);
			modeAnnouncement = agentModeAnnouncement(agentModeState.mode);
			if (settled.shouldPersist) {
				void persistAgentMode(id, settled.shouldPersist);
			}
		} catch {
			if (deps.getSessionId() !== id) return;
			const settled = completeAgentModeRequest(agentModeState, next, { ok: false });
			applyAgentModeState(settled.state);
			if (settled.shouldPersist) {
				modeAnnouncement = agentModeAnnouncement(agentModeState.mode);
				void persistAgentMode(id, settled.shouldPersist);
				return;
			}
			modeAnnouncement = 'Failed to change mode';
		}
	}

	function setAgentMode(next: AgentMode) {
		const started = beginAgentModeRequest(agentModeState, next);
		if (started.state === agentModeState && !started.shouldPersist) return;
		applyAgentModeState(started.state);
		modeAnnouncement = agentModeAnnouncement(started.state.mode);
		const id = deps.getSessionId();
		if (!id || !started.shouldPersist) return;
		void persistAgentMode(id, next);
	}

	function cycleAgentMode() {
		setAgentMode(nextAgentMode(agentModeState.mode));
	}

	return {
		get agentMode() {
			return agentMode;
		},
		get agentModeKnown() {
			return agentModeKnown;
		},
		get modeAnnouncement() {
			return modeAnnouncement;
		},
		resetStoreBinding,
		bindFromStore,
		setAgentMode,
		cycleAgentMode
	};
}
