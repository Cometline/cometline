import type { InboxMessageResource } from '$lib/client/cometmind';
import {
	jobLinkKey,
	sessionLinkKey,
	type LinkAvailabilityMap
} from '$lib/features/inbox/link-availability';
import { matchesShortcut } from '$lib/keyboard-shortcuts';
import { appToastStore } from '$lib/stores/app-toasts.svelte';
import { settingsStore } from '$lib/stores/settings.svelte';

export type InboxDrawerController = ReturnType<typeof createInboxDrawerController>;

export function createInboxDrawerController(deps: {
	getOpen: () => boolean;
	getMessages: () => InboxMessageResource[];
	getBusyId: () => string | null;
	canOpenJob: () => boolean;
	canOpenSession: () => boolean;
	reply: (id: string, content: string) => void | Promise<void>;
	dismiss: (id: string) => void | Promise<void>;
}) {
	let selectedId = $state<string | null>(null);
	let replyDraft = $state('');
	let replyForId = $state<string | null>(null);
	let linkAvailability: LinkAvailabilityMap = $state({});

	/** Prefer the user's pick; otherwise auto-select the first open message. */
	const selected = $derived.by(() => {
		if (!deps.getOpen() || deps.getMessages().length === 0) return null;
		const messages = deps.getMessages();
		if (selectedId) {
			const match = messages.find((m) => m.id === selectedId);
			if (match) return match;
		}
		return messages[0] ?? null;
	});

	const activeReply = $derived(selected && replyForId === selected.id ? replyDraft : '');

	const jobLinkStatus = $derived.by(() => {
		const id = selected?.job_id?.trim();
		if (!id) return null;
		return linkAvailability[jobLinkKey(id)] ?? 'unknown';
	});

	const sessionLinkStatus = $derived.by(() => {
		const id = selected?.session_id?.trim();
		if (!id) return null;
		return linkAvailability[sessionLinkKey(id)] ?? 'unknown';
	});

	const showJobLink = $derived(
		jobLinkStatus === 'missing' || (jobLinkStatus === 'available' && deps.canOpenJob())
	);

	const showSessionLink = $derived(
		sessionLinkStatus === 'missing' ||
			(sessionLinkStatus === 'available' && deps.canOpenSession())
	);

	const showDetailLinks = $derived(showJobLink || showSessionLink);

	function selectMessage(id: string) {
		selectedId = id;
		replyDraft = '';
		replyForId = id;
	}

	function setActiveReply(value: string) {
		if (!selected) return;
		replyForId = selected.id;
		replyDraft = value;
	}

	function setLinkAvailability(next: LinkAvailabilityMap) {
		linkAvailability = next;
	}

	function handleReplyKeydown(event: KeyboardEvent) {
		const shortcuts = settingsStore.settings.shortcuts;
		// Match composer: newline before send so Shift+Enter wins if bindings overlap.
		if (!event.isComposing && matchesShortcut(event, shortcuts.insertNewline)) {
			return;
		}
		if (!event.isComposing && matchesShortcut(event, shortcuts.sendMessage)) {
			event.preventDefault();
			void submitReply();
		}
	}

	async function submitReply() {
		if (!selected || !activeReply.trim() || deps.getBusyId()) return;
		const id = selected.id;
		const title = selected.title?.trim() || 'Message';
		const content = activeReply.trim();
		await deps.reply(id, content);
		appToastStore.success('Reply sent', title);
		replyDraft = '';
		replyForId = null;
		selectedId = null;
	}

	async function dismissSelected() {
		if (!selected || deps.getBusyId()) return;
		const id = selected.id;
		replyDraft = '';
		replyForId = null;
		selectedId = null;
		await deps.dismiss(id);
	}

	return {
		get selected() {
			return selected;
		},
		get activeReply() {
			return activeReply;
		},
		get jobLinkStatus() {
			return jobLinkStatus;
		},
		get sessionLinkStatus() {
			return sessionLinkStatus;
		},
		get showJobLink() {
			return showJobLink;
		},
		get showSessionLink() {
			return showSessionLink;
		},
		get showDetailLinks() {
			return showDetailLinks;
		},
		selectMessage,
		setActiveReply,
		setLinkAvailability,
		handleReplyKeydown,
		submitReply,
		dismissSelected
	};
}
