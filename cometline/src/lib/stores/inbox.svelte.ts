import {
	dismissInboxMessage,
	getInboxSummary,
	listInboxMessages,
	replyInboxMessage,
	type InboxMessageResource
} from '$lib/client/cometmind';

function createInboxStore() {
	let openCount = $state(0);
	let messages = $state<InboxMessageResource[]>([]);
	let drawerOpen = $state(false);
	let busyId = $state<string | null>(null);
	let error = $state<string | null>(null);
	let loaded = $state(false);

	async function refreshSummary() {
		const summary = await getInboxSummary();
		openCount = summary.open_count;
	}

	async function load() {
		error = null;
		try {
			const [list, summary] = await Promise.all([
				listInboxMessages('open'),
				getInboxSummary()
			]);
			messages = list.messages;
			openCount = summary.open_count;
			loaded = true;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load inbox';
		}
	}

	function applyCreated(_id: string, nextOpenCount: number) {
		openCount = nextOpenCount;
		if (drawerOpen) {
			void load();
		}
	}

	function applyArchived(id: string, nextOpenCount: number) {
		openCount = nextOpenCount;
		messages = messages.filter((m) => m.id !== id);
	}

	async function reply(id: string, content: string) {
		busyId = id;
		error = null;
		try {
			await replyInboxMessage(id, content);
			messages = messages.filter((m) => m.id !== id);
			openCount = Math.max(0, openCount - 1);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to reply';
			throw err;
		} finally {
			busyId = null;
		}
	}

	async function dismiss(id: string) {
		busyId = id;
		error = null;
		try {
			await dismissInboxMessage(id);
			messages = messages.filter((m) => m.id !== id);
			openCount = Math.max(0, openCount - 1);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to dismiss';
			throw err;
		} finally {
			busyId = null;
		}
	}

	function openDrawer() {
		drawerOpen = true;
		void load();
	}

	function closeDrawer() {
		drawerOpen = false;
	}

	function toggleDrawer() {
		if (drawerOpen) closeDrawer();
		else openDrawer();
	}

	return {
		get openCount() {
			return openCount;
		},
		get messages() {
			return messages;
		},
		get drawerOpen() {
			return drawerOpen;
		},
		get busyId() {
			return busyId;
		},
		get error() {
			return error;
		},
		get loaded() {
			return loaded;
		},
		load,
		refreshSummary,
		applyCreated,
		applyArchived,
		reply,
		dismiss,
		openDrawer,
		closeDrawer,
		toggleDrawer
	};
}

export const inboxStore = createInboxStore();
