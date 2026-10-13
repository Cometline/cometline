export interface ComposerInputRef {
	getFilePaths(): string[];
	getPasteDisplayText?(): string | undefined;
	clear(): void;
	insertText(text: string): void;
	setText(text: string): void;
	focusAsync(options?: { position?: 'start' | 'end' }): Promise<void>;
	insertFileMention(path: string): void;
	isCaretAtStart(): boolean;
	isCaretAtEnd(): boolean;
}
