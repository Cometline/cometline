import { shellStore } from '$lib/stores/shell.svelte';

type WorkspacePanelModuleComponent =
	typeof import('$lib/features/workspace/components/WorkspacePanel.svelte').default;
type IntroAnimationComponent =
	typeof import('$lib/features/shell/components/IntroAnimation.svelte').default;
type InboxDrawerComponent =
	typeof import('$lib/features/inbox/components/InboxDrawer.svelte').default;

export function createAppShellLazyPanels() {
	let WorkspacePanel = $state<WorkspacePanelModuleComponent | null>(null);
	let workspacePanelLoadPromise: Promise<WorkspacePanelModuleComponent | null> | null = null;
	let workspacePanelLoadFailed = $state(false);
	let Intro = $state<IntroAnimationComponent | null>(null);
	let introLoadPromise: Promise<IntroAnimationComponent | null> | null = null;
	let Inbox = $state<InboxDrawerComponent | null>(null);
	let inboxLoadPromise: Promise<InboxDrawerComponent | null> | null = null;
	let inboxLoadFailed = $state(false);

	function loadWorkspacePanel() {
		if (WorkspacePanel) return Promise.resolve(WorkspacePanel);
		if (!workspacePanelLoadPromise) {
			workspacePanelLoadFailed = false;
			workspacePanelLoadPromise =
				import('$lib/features/workspace/components/WorkspacePanel.svelte')
					.then((module) => {
						WorkspacePanel = module.default;
						return module.default;
					})
					.catch((error) => {
						workspacePanelLoadPromise = null;
						workspacePanelLoadFailed = true;
						console.error('Workspace panel failed to load', error);
						return null;
					});
		}
		return workspacePanelLoadPromise;
	}

	function loadIntroAnimation() {
		if (Intro) return Promise.resolve(Intro);
		if (!introLoadPromise) {
			introLoadPromise = import('$lib/features/shell/components/IntroAnimation.svelte')
				.then((module) => {
					Intro = module.default;
					return module.default;
				})
				.catch((error) => {
					introLoadPromise = null;
					console.error('Intro animation failed to load', error);
					shellStore.closeIntro();
					return null;
				});
		}
		return introLoadPromise;
	}

	function loadInboxDrawer() {
		if (Inbox) return Promise.resolve(Inbox);
		if (!inboxLoadPromise) {
			inboxLoadFailed = false;
			inboxLoadPromise = import('$lib/features/inbox/components/InboxDrawer.svelte')
				.then((module) => {
					Inbox = module.default;
					return module.default;
				})
				.catch((error) => {
					inboxLoadPromise = null;
					inboxLoadFailed = true;
					console.error('Inbox drawer failed to load', error);
					return null;
				});
		}
		return inboxLoadPromise;
	}

	return {
		get WorkspacePanel() {
			return WorkspacePanel;
		},
		get Intro() {
			return Intro;
		},
		get Inbox() {
			return Inbox;
		},
		get workspacePanelLoadFailed() {
			return workspacePanelLoadFailed;
		},
		get inboxLoadFailed() {
			return inboxLoadFailed;
		},
		loadWorkspacePanel,
		loadIntroAnimation,
		loadInboxDrawer
	};
}
