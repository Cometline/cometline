import { goto } from '$app/navigation';
import { listInboxMessages, listSkillDrafts } from '$lib/client/cometmind';
import type { CometMindJobsNotificationSettings } from '$lib/cometmind-settings';
import { sessionDisplayTitle } from '$lib/sessions/session-title';
import { appToastStore } from '$lib/stores/app-toasts.svelte';
import { inboxStore } from '$lib/stores/inbox.svelte';
import type { Session } from '$lib/types';

type JobNotice = {
	kind: 'completed' | 'blocked';
	id: string;
	description: string;
};

export function notifyJobActivity(notice: JobNotice, settings: CometMindJobsNotificationSettings) {
	if (!settings.enabled) return;
	if (notice.kind === 'completed' && !settings.onCompleted) return;
	if (notice.kind === 'blocked' && !settings.onBlocked) return;
	const label = notice.kind === 'completed' ? 'Job completed' : 'Job blocked';
	const open = () => {
		void goto(`/jobs?job=${encodeURIComponent(notice.id)}`);
	};
	if (notice.kind === 'blocked') {
		appToastStore.warning(label, notice.description, open);
		return;
	}
	appToastStore.success(label, notice.description, open);
}

export async function notifyNewInboxMessage(id: string) {
	if (inboxStore.drawerOpen) return;
	try {
		const list = await listInboxMessages('open');
		const message = list.messages.find((item) => item.id === id);
		if (!message) return;
		appToastStore.success('New inbox note', message.title, () => {
			inboxStore.openDrawer();
		});
	} catch {
		appToastStore.success('New inbox note', '', () => {
			inboxStore.openDrawer();
		});
	}
}

export function startSkillDraftToastWatch(opts: {
	intervalMs?: number;
	listDrafts?: typeof listSkillDrafts;
	isReviewOpen?: () => boolean;
}): () => void {
	const listDrafts = opts.listDrafts ?? listSkillDrafts;
	const isReviewOpen = opts.isReviewOpen ?? (() => false);
	const known = new Set<string>();
	let primed = false;

	async function poll() {
		let drafts: Awaited<ReturnType<typeof listSkillDrafts>>;
		try {
			drafts = await listDrafts();
		} catch {
			return;
		}
		if (!primed) {
			for (const draft of drafts) known.add(draft.name);
			primed = true;
			return;
		}
		const fresh = drafts.filter((draft) => !known.has(draft.name));
		for (const draft of drafts) known.add(draft.name);
		if (fresh.length === 0 || isReviewOpen()) return;
		const first = fresh[0];
		const detail =
			fresh.length === 1 ? first.description || first.name : `${fresh.length} drafts ready`;
		appToastStore.success('Skill draft ready', detail, () => {
			void goto('/skills');
		});
	}

	void poll();
	const timer = setInterval(() => void poll(), opts.intervalMs ?? 30_000);
	return () => clearInterval(timer);
}

export function isNotifiableChat(session: Pick<Session, 'origin' | 'parent_session_id'>): boolean {
	return session.origin === 'user' && !session.parent_session_id;
}

/** Toast when a user chat the viewer is not looking at finishes. */
export function notifyBackgroundRunFinished(
	session: Pick<Session, 'id' | 'title' | 'origin' | 'parent_session_id'>,
	activeSessionId: string | null
) {
	if (!session.id || session.id === activeSessionId || !isNotifiableChat(session)) return;
	appToastStore.success('Chat finished', sessionDisplayTitle(session.title), () => {
		void goto(`/session/${session.id}`);
	});
}

export function notifyConnectionChange(previous: string, next: string) {
	if (previous === 'ready' && next === 'error') {
		appToastStore.error('CometMind disconnected', 'Reconnecting in the background');
		return;
	}
	if (previous === 'error' && next === 'ready') {
		appToastStore.success('CometMind is back');
	}
}
