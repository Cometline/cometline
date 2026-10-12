const seenKey = 'cometline.skill-updates.seen';

function readSeen(): Set<string> {
	if (typeof localStorage === 'undefined') return new Set();
	try {
		const raw = localStorage.getItem(seenKey);
		const parsed = raw ? (JSON.parse(raw) as unknown) : [];
		return new Set(
			Array.isArray(parsed) ? parsed.filter((name) => typeof name === 'string') : []
		);
	} catch {
		return new Set();
	}
}

function createSkillUpdatesStore() {
	let unseen = $state<string[]>([]);
	const seen = readSeen();

	function persist() {
		if (typeof localStorage === 'undefined') return;
		localStorage.setItem(seenKey, JSON.stringify([...seen]));
	}

	function note(names: string[]) {
		const fresh = names.map((name) => name.trim()).filter((name) => name && !seen.has(name));
		if (fresh.length === 0) return;
		unseen = [...new Set([...unseen, ...fresh])];
	}

	function markSeen(names?: string[]) {
		const clearing = names ?? unseen;
		for (const name of clearing) seen.add(name);
		const drop = new Set(clearing);
		unseen = unseen.filter((name) => !drop.has(name));
		persist();
	}

	return {
		get unseen() {
			return unseen;
		},
		get hasUnseen() {
			return unseen.length > 0;
		},
		note,
		markSeen
	};
}

export const skillUpdatesStore = createSkillUpdatesStore();
