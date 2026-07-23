import type { TerminalAppearanceSettings, TerminalThemeId } from '$lib/types';

export const DEFAULT_TERMINAL_APPEARANCE: TerminalAppearanceSettings = {
	fontSize: 12,
	theme: 'cometline-dark'
};

export const DEFAULT_TERMINAL_FONT_FAMILY =
	'"MesloLGS NF", "Meslo LG S", ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace';

export interface TerminalColorTheme {
	background: string;
	foreground: string;
	cursor: string;
	selectionBackground: string;
	black: string;
	red: string;
	green: string;
	yellow: string;
	blue: string;
	magenta: string;
	cyan: string;
	white: string;
	brightBlack: string;
	brightRed: string;
	brightGreen: string;
	brightYellow: string;
	brightBlue: string;
	brightMagenta: string;
	brightCyan: string;
	brightWhite: string;
}

export const TERMINAL_THEME_PRESETS: Record<
	TerminalThemeId,
	{ label: string; colors: TerminalColorTheme }
> = {
	'cometline-dark': {
		label: 'Cometline Dark',
		colors: {
			background: '#171717',
			foreground: '#f4f4f5',
			cursor: '#f4f4f5',
			selectionBackground: '#2563eb66',
			black: '#18181b',
			red: '#ef4444',
			green: '#22c55e',
			yellow: '#eab308',
			blue: '#3b82f6',
			magenta: '#d946ef',
			cyan: '#22d3ee',
			white: '#e4e4e7',
			brightBlack: '#52525b',
			brightRed: '#f87171',
			brightGreen: '#4ade80',
			brightYellow: '#facc15',
			brightBlue: '#60a5fa',
			brightMagenta: '#e879f9',
			brightCyan: '#67e8f9',
			brightWhite: '#fafafa'
		}
	},
	dracula: {
		label: 'Dracula',
		colors: {
			background: '#282a36',
			foreground: '#f8f8f2',
			cursor: '#f8f8f2',
			selectionBackground: '#44475a',
			black: '#21222c',
			red: '#ff5555',
			green: '#50fa7b',
			yellow: '#f1fa8c',
			blue: '#bd93f9',
			magenta: '#ff79c6',
			cyan: '#8be9fd',
			white: '#f8f8f2',
			brightBlack: '#6272a4',
			brightRed: '#ff6e6e',
			brightGreen: '#69ff94',
			brightYellow: '#ffffa5',
			brightBlue: '#d6acff',
			brightMagenta: '#ff92df',
			brightCyan: '#a4ffff',
			brightWhite: '#ffffff'
		}
	},
	'gruvbox-dark': {
		label: 'Gruvbox Dark',
		colors: {
			background: '#282828',
			foreground: '#ebdbb2',
			cursor: '#ebdbb2',
			selectionBackground: '#504945',
			black: '#282828',
			red: '#cc241d',
			green: '#98971a',
			yellow: '#d79921',
			blue: '#458588',
			magenta: '#b16286',
			cyan: '#689d6a',
			white: '#a89984',
			brightBlack: '#928374',
			brightRed: '#fb4934',
			brightGreen: '#b8bb26',
			brightYellow: '#fabd2f',
			brightBlue: '#83a598',
			brightMagenta: '#d3869b',
			brightCyan: '#8ec07c',
			brightWhite: '#ebdbb2'
		}
	},
	'solarized-dark': {
		label: 'Solarized Dark',
		colors: {
			background: '#002b36',
			foreground: '#839496',
			cursor: '#93a1a1',
			selectionBackground: '#073642',
			black: '#073642',
			red: '#dc322f',
			green: '#859900',
			yellow: '#b58900',
			blue: '#268bd2',
			magenta: '#d33682',
			cyan: '#2aa198',
			white: '#eee8d5',
			brightBlack: '#586e75',
			brightRed: '#cb4b16',
			brightGreen: '#586e75',
			brightYellow: '#657b83',
			brightBlue: '#839496',
			brightMagenta: '#6c71c4',
			brightCyan: '#93a1a1',
			brightWhite: '#fdf6e3'
		}
	}
};

const TERMINAL_THEME_IDS: TerminalThemeId[] = [
	'cometline-dark',
	'dracula',
	'gruvbox-dark',
	'solarized-dark'
];
export function normalizeTerminalFontSize(value: unknown): number {
	if (typeof value !== 'number' || !Number.isFinite(value)) {
		return DEFAULT_TERMINAL_APPEARANCE.fontSize;
	}
	return Math.min(32, Math.max(8, Math.round(value)));
}

export function normalizeTerminalAppearance(
	appearance: Partial<TerminalAppearanceSettings> | undefined
): TerminalAppearanceSettings {
	const fontSize = appearance?.fontSize;
	const theme = appearance?.theme;
	return {
		fontSize: normalizeTerminalFontSize(fontSize),
		theme: TERMINAL_THEME_IDS.includes(theme as TerminalThemeId)
			? (theme as TerminalThemeId)
			: DEFAULT_TERMINAL_APPEARANCE.theme
	};
}
