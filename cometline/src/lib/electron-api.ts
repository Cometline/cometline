// Type-only bridge to the preload contract, so renderer code never imports from electron/.
export type {
	DeleteCustomPersonaResult,
	ElectronAPI,
	MiniWindowState,
	RuntimeReloadOutcome,
	SaveCustomPersonaResult,
	UpdateState
} from '../../electron/src/shared/api';
