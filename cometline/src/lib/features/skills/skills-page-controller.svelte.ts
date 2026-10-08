import {
	deleteSkill,
	getSkill,
	getSkillDraft,
	listSkillDrafts,
	listSkills,
	promoteSkillDraft,
	rejectSkillDraft,
	updateSkill,
	updateSkillDraft,
	type SkillDetailResponse,
	type SkillDraft,
	type SkillDraftDetailResponse
} from '#lib/client/cometmind.js';
import { filterSkills } from '#lib/features/skills/skills-page-filter.js';
import { skillDraftsStore } from '#lib/stores/skill-drafts.svelte.js';
import { shellStore } from '#lib/stores/shell.svelte.js';
import type { SkillResource } from '#lib/types.js';

export type SkillsTab = 'skills' | 'drafts';

export type SkillsPageController = ReturnType<typeof createSkillsPageController>;

export function createSkillsPageController(deps: {
	getTab: () => SkillsTab;
	getRequestedSkill?: () => string;
}) {
	let drafts = $state<SkillDraft[]>([]);
	let selectedDraft = $state<SkillDraftDetailResponse | null>(null);
	let selectedDraftName = $state('');
	let draftContent = $state('');
	let skills = $state<SkillResource[]>([]);
	let skillErrors = $state<string[]>([]);
	let selectedSkill = $state<SkillDetailResponse | null>(null);
	let selectedSkillName = $state('');
	let skillContent = $state('');
	let skillSearch = $state('');
	let busy = $state(false);
	let contentBusy = $state(false);
	let saveBusy = $state(false);
	let deletePending = $state<SkillResource | null>(null);
	let pendingSkillName = $state('');
	let status = $state('');
	let skillRequestId = 0;
	const selectedDraftId = $derived(selectedDraft?.draft.name ?? '');
	const draftDirty = $derived(selectedDraft !== null && draftContent !== selectedDraft.content);
	const selectedSkillId = $derived(selectedSkill?.skill.name ?? '');
	const skillDirty = $derived(selectedSkill !== null && skillContent !== selectedSkill.content);
	const canEditSkill = $derived(selectedSkill?.skill.can_edit ?? false);
	const canDeleteSkill = $derived(selectedSkill?.skill.can_delete ?? false);
	const filteredSkills = $derived(filterSkills(skills, skillSearch));

	async function load() {
		await refreshDrafts();
		await refreshSkills();
	}

	async function refreshDrafts(options: { keepSelection?: boolean } = {}) {
		busy = true;
		status = '';
		try {
			const nextDrafts = await listSkillDrafts();
			drafts = nextDrafts;
			skillDraftsStore.setCount(nextDrafts.length);
			if (nextDrafts.length === 0) {
				selectedDraft = null;
				selectedDraftName = '';
				return;
			}
			const preferred =
				options.keepSelection && selectedDraftName
					? nextDrafts.find((draft) => draft.name === selectedDraftName)?.name
					: '';
			await openDraft(preferred || nextDrafts[0].name);
		} catch (err) {
			status = err instanceof Error ? err.message : 'Failed to load skill drafts';
		} finally {
			busy = false;
		}
	}

	async function refreshSkills(options: { keepSelection?: boolean } = {}) {
		busy = true;
		status = '';
		try {
			const result = await listSkills(shellStore.workspacePath);
			skills = result.skills ?? [];
			skillErrors = result.errors ?? [];
			if (skills.length === 0) {
				selectedSkill = null;
				selectedSkillName = '';
				return;
			}
			const requested = deps.getRequestedSkill?.().trim() ?? '';
			const preferred =
				requested ||
				(options.keepSelection && selectedSkillName
					? skills.find((skill) => skill.name === selectedSkillName)?.name
					: '') ||
				'';
			if (preferred && skillDirty && !requested) return;
			await openSkill(preferred || skills[0].name, { force: true });
		} catch (err) {
			status = err instanceof Error ? err.message : 'Failed to load skills';
		} finally {
			busy = false;
		}
	}

	async function refreshCurrent(options: { keepSelection?: boolean } = {}) {
		if (deps.getTab() === 'drafts') {
			await refreshDrafts(options);
			return;
		}
		await refreshSkills(options);
	}

	async function openDraft(name: string) {
		selectedDraftName = name;
		contentBusy = true;
		try {
			selectedDraft = await getSkillDraft(name);
			draftContent = selectedDraft.content;
		} catch (err) {
			selectedDraft = null;
			draftContent = '';
			status = err instanceof Error ? err.message : 'Failed to load draft';
		} finally {
			contentBusy = false;
		}
	}

	async function selectSkill(name: string) {
		const trimmed = name.trim();
		if (!trimmed) return;
		await openSkill(trimmed, { force: true });
	}

	async function openSkill(name: string, options: { force?: boolean } = {}) {
		if (!options.force && name === selectedSkillName && selectedSkill) return;
		if (!options.force && skillDirty) {
			pendingSkillName = name;
			return;
		}
		const requestId = ++skillRequestId;
		selectedSkillName = name;
		deletePending = null;
		contentBusy = true;
		try {
			const detail = await getSkill(name, shellStore.workspacePath);
			if (requestId !== skillRequestId || selectedSkillName !== name) return;
			selectedSkill = detail;
			skillContent = detail.content;
		} catch (err) {
			if (requestId !== skillRequestId || selectedSkillName !== name) return;
			selectedSkill = null;
			skillContent = '';
			status = err instanceof Error ? err.message : 'Failed to load skill';
		} finally {
			if (requestId === skillRequestId) contentBusy = false;
		}
	}

	function discardSkillChanges() {
		const name = pendingSkillName;
		pendingSkillName = '';
		if (name) void openSkill(name, { force: true });
	}

	function cancelSkillSwitch() {
		pendingSkillName = '';
	}

	function requestDeleteSelectedSkill() {
		if (selectedSkill) deletePending = selectedSkill.skill;
	}

	function cancelDeleteSkill() {
		deletePending = null;
	}

	async function saveDraft(name: string) {
		saveBusy = true;
		status = '';
		try {
			selectedDraft = await updateSkillDraft(name, draftContent);
			draftContent = selectedDraft.content;
			status = `Saved draft ${name}.`;
			await refreshDrafts({ keepSelection: true });
		} catch (err) {
			status = err instanceof Error ? err.message : 'Failed to save draft';
		} finally {
			saveBusy = false;
		}
	}

	async function saveSkill(name: string) {
		saveBusy = true;
		status = '';
		const submittedContent = skillContent;
		try {
			const updated = await updateSkill(name, submittedContent, shellStore.workspacePath);
			if (selectedSkillName === name) {
				selectedSkill = updated;
				if (skillContent === submittedContent) skillContent = updated.content;
			}
			status = `Saved skill ${name}.`;
			await refreshSkills({ keepSelection: true });
		} catch (err) {
			status = err instanceof Error ? err.message : 'Failed to save skill';
		} finally {
			saveBusy = false;
		}
	}

	async function promoteDraft(name: string) {
		busy = true;
		status = '';
		try {
			await promoteSkillDraft(name);
			status = `Promoted draft ${name}.`;
			await refreshDrafts({ keepSelection: true });
			void refreshSkills({ keepSelection: true });
		} catch (err) {
			status = err instanceof Error ? err.message : 'Failed to promote draft';
		} finally {
			busy = false;
		}
	}

	async function confirmDeleteSkill() {
		const skill = deletePending;
		if (!skill) return;
		busy = true;
		status = '';
		try {
			await deleteSkill(skill.name, shellStore.workspacePath);
			deletePending = null;
			status = `Deleted skill ${skill.name}.`;
			if (selectedSkillName === skill.name) {
				selectedSkill = null;
				selectedSkillName = '';
				skillContent = '';
			}
			await refreshSkills({ keepSelection: true });
		} catch (err) {
			status = err instanceof Error ? err.message : 'Failed to delete skill';
		} finally {
			busy = false;
		}
	}

	async function rejectDraft(name: string) {
		busy = true;
		status = '';
		try {
			await rejectSkillDraft(name);
			status = `Rejected draft ${name}.`;
			await refreshDrafts({ keepSelection: true });
		} catch (err) {
			status = err instanceof Error ? err.message : 'Failed to reject draft';
		} finally {
			busy = false;
		}
	}

	return {
		get drafts() {
			return drafts;
		},
		get selectedDraft() {
			return selectedDraft;
		},
		get selectedDraftName() {
			return selectedDraftName;
		},
		get draftContent() {
			return draftContent;
		},
		set draftContent(value: string) {
			draftContent = value;
		},
		get skills() {
			return skills;
		},
		get skillErrors() {
			return skillErrors;
		},
		get selectedSkill() {
			return selectedSkill;
		},
		get selectedSkillName() {
			return selectedSkillName;
		},
		get skillContent() {
			return skillContent;
		},
		set skillContent(value: string) {
			skillContent = value;
		},
		get skillSearch() {
			return skillSearch;
		},
		set skillSearch(value: string) {
			skillSearch = value;
		},
		get busy() {
			return busy;
		},
		get contentBusy() {
			return contentBusy;
		},
		get saveBusy() {
			return saveBusy;
		},
		get deletePending() {
			return deletePending;
		},
		get pendingSkillName() {
			return pendingSkillName;
		},
		get status() {
			return status;
		},
		get selectedDraftId() {
			return selectedDraftId;
		},
		get draftDirty() {
			return draftDirty;
		},
		get selectedSkillId() {
			return selectedSkillId;
		},
		get skillDirty() {
			return skillDirty;
		},
		get canEditSkill() {
			return canEditSkill;
		},
		get canDeleteSkill() {
			return canDeleteSkill;
		},
		get filteredSkills() {
			return filteredSkills;
		},
		load,
		refreshCurrent,
		openDraft,
		openSkill,
		selectSkill,
		discardSkillChanges,
		cancelSkillSwitch,
		requestDeleteSelectedSkill,
		cancelDeleteSkill,
		saveDraft,
		saveSkill,
		promoteDraft,
		confirmDeleteSkill,
		rejectDraft
	};
}
