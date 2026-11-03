export interface ComposerInputRef {
	getFilePaths(): string[];
	getPasteBodies?(): string[];
	getPasteDisplayText?(): string | undefined;
	clear(): void;
	insertText(text: string): void;
	setText(text: string, pastes?: readonly string[], display?: string): void;
	focusAsync(options?: { position?: 'start' | 'end' }): Promise<void>;
	insertFileMention(path: string): void;
	isCaretAtStart(): boolean;
	isCaretAtEnd(): boolean;
}
