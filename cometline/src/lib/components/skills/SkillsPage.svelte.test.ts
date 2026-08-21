// @vitest-environment jsdom
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterAll, beforeEach, describe, expect, it, vi } from 'vitest';
import SkillsPage from './SkillsPage.svelte';

const api = vi.hoisted(() => ({
	deleteSkill: vi.fn(),
	getSkill: vi.fn(),
	getSkillDraft: vi.fn(),
	listSkillDrafts: vi.fn(),
	listSkills: vi.fn(),
	promoteSkillDraft: vi.fn(),
	rejectSkillDraft: vi.fn(),
	updateSkill: vi.fn(),
	updateSkillDraft: vi.fn()
}));

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('$app/state', () => ({
	page: { url: new URL('http://localhost/skills?tab=skills') }
}));
vi.mock('$lib/client/cometmind', () => api);
vi.mock('$lib/stores/skill-drafts.svelte', () => ({
	skillDraftsStore: { count: 0, hasDrafts: false, setCount: vi.fn() }
}));
vi.mock('$lib/stores/shell.svelte', () => ({
	shellStore: { workspacePath: '/workspace' }
}));

type Skill = {
	name: string;
	description: string;
	path: string;
	source: string;
	internal: boolean;
	is_symlink: boolean;
	can_delete: boolean;
	can_export: boolean;
	can_edit: boolean;
};

const alpha = skill('alpha');
const beta = skill('beta');
const originalShowModal = Object.getOwnPropertyDescriptor(HTMLDialogElement.prototype, 'showModal');
const originalClose = Object.getOwnPropertyDescriptor(HTMLDialogElement.prototype, 'close');

function skill(name: string): Skill {
	return {
		name,
		description: `${name} description`,
		path: `/skills/${name}`,
		source: `/skills/${name}`,
		internal: false,
		is_symlink: false,
		can_delete: false,
		can_export: true,
		can_edit: true
	};
}

function detail(value: Skill, content = `${value.name} content`) {
	return { skill: value, content };
}

function deferred<T>() {
	let resolve!: (value: T) => void;
	const promise = new Promise<T>((next) => {
		resolve = next;
	});
	return { promise, resolve };
}

function editor(): HTMLTextAreaElement {
	const textarea = document.querySelector<HTMLTextAreaElement>('.item-markdown');
	if (!textarea) throw new Error('skill editor not found');
	return textarea;
}

describe('SkillsPage skill editor', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		api.listSkillDrafts.mockResolvedValue([]);
		api.listSkills.mockResolvedValue({ skills: [alpha, beta], errors: [] });
		api.getSkill.mockImplementation((name: string) =>
			Promise.resolve(detail(name === alpha.name ? alpha : beta))
		);
		Object.defineProperty(HTMLDialogElement.prototype, 'showModal', {
			configurable: true,
			value(this: HTMLDialogElement) {
				this.open = true;
			}
		});
		Object.defineProperty(HTMLDialogElement.prototype, 'close', {
			configurable: true,
			value(this: HTMLDialogElement) {
				this.open = false;
			}
		});
	});

	afterAll(() => {
		if (originalShowModal) {
			Object.defineProperty(HTMLDialogElement.prototype, 'showModal', originalShowModal);
		} else {
			Reflect.deleteProperty(HTMLDialogElement.prototype, 'showModal');
		}
		if (originalClose) {
			Object.defineProperty(HTMLDialogElement.prototype, 'close', originalClose);
		} else {
			Reflect.deleteProperty(HTMLDialogElement.prototype, 'close');
		}
	});

	it('asks before switching away from unsaved changes', async () => {
		render(SkillsPage);
		await waitFor(() => expect(editor()).toHaveValue('alpha content'));

		await fireEvent.input(editor(), { target: { value: 'unsaved alpha' } });
		await fireEvent.click(screen.getByRole('button', { name: /beta/ }));

		expect(screen.getByRole('dialog')).toHaveTextContent('Discard unsaved changes?');
		expect(editor()).toHaveValue('unsaved alpha');

		await fireEvent.click(screen.getByRole('button', { name: 'Discard' }));
		await waitFor(() => expect(editor()).toHaveValue('beta content'));
	});

	it('ignores a stale skill response after a faster selection completes', async () => {
		const alphaRequest = deferred<ReturnType<typeof detail>>();
		api.getSkill.mockImplementation((name: string) =>
			name === alpha.name ? alphaRequest.promise : Promise.resolve(detail(beta))
		);
		render(SkillsPage);

		await fireEvent.click(await screen.findByRole('button', { name: /beta/ }));
		await waitFor(() => expect(editor()).toHaveValue('beta content'));
		alphaRequest.resolve(detail(alpha));

		await waitFor(() => expect(editor()).toHaveValue('beta content'));
	});

	it('preserves edits typed while a save request is in flight', async () => {
		const updateRequest = deferred<ReturnType<typeof detail>>();
		api.updateSkill.mockReturnValue(updateRequest.promise);
		render(SkillsPage);
		await waitFor(() => expect(editor()).toHaveValue('alpha content'));

		await fireEvent.input(editor(), { target: { value: 'submitted alpha' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Save' }));
		await fireEvent.input(editor(), { target: { value: 'newer alpha edit' } });
		updateRequest.resolve(detail(alpha, 'submitted alpha'));

		await waitFor(() => expect(api.listSkills).toHaveBeenCalledTimes(2));
		expect(editor()).toHaveValue('newer alpha edit');
	});
});
