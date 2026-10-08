import { goto } from '$app/navigation';
import { resolve } from '$app/paths';
import { listInboxMessages, listSkillDrafts } from '#lib/client/cometmind.js';
import { openWorkspaceFilePreview } from '#lib/features/workspace/open-file-preview.js';
import type { CometMindJobsNotificationSettings } from '#lib/cometmind-settings.js';
import { gotoJob } from '#lib/routes/job-route.js';
import { sessionDisplayTitle } from '#lib/sessions/session-title.js';
import { appToastStore } from '#lib/stores/app-toasts.svelte.js';
import { inboxStore } from '#lib/stores/inbox.svelte.js';
import type { Session } from '#lib/types.js';

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
		void gotoJob(notice.id);
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

export type SkillReviewNotice = {
	name: string;
	action: string;
	description: string;
};

export function notifySkillReview(skills: SkillReviewNotice[]) {
	if (skills.length === 0) return;
	const first = skills[0];
	const detail = skills.length === 1 ? first.name : `${skills.length} skills updated`;
	appToastStore.success('Skill updated', detail, () => {
		const params = new URLSearchParams({ tab: 'skills', skill: first.name });
		void goto(`${resolve('skills')}?${params.toString()}`);
	});
}

export function notifyWikiReview(paths: string[]) {
	if (paths.length === 0) return;
	const first = paths[0];
	const name = first.split('/').pop() || first;
	const detail = paths.length === 1 ? name : `${paths.length} wiki pages updated`;
	appToastStore.success('Wiki updated', detail, () => {
		openWorkspaceFilePreview(first);
	});
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
			void goto(resolve('skills'));
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
		void goto(resolve('/session/[id]', { id: session.id }));
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
