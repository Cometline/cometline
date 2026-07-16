import { listSkillDrafts } from '$lib/client/cometmind';

function createSkillDraftsStore() {
	let count = $state(0);
	let loaded = $state(false);

	async function refresh() {
		try {
			const drafts = await listSkillDrafts();
			count = drafts.length;
			loaded = true;
		} catch {
			// Keep the last known count if the sidecar is briefly unavailable.
		}
	}

	function setCount(next: number) {
		count = Math.max(0, next);
		loaded = true;
	}

	return {
		get count() {
			return count;
		},
		get loaded() {
			return loaded;
		},
		get hasDrafts() {
			return count > 0;
		},
		refresh,
		setCount
	};
}

export const skillDraftsStore = createSkillDraftsStore();
