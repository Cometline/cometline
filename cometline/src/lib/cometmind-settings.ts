export {
	cloneCometMindSettings,
	defaultCometMindSettings,
	normalizeCometMindSettings,
	type CometMindACPSettings,
	type CometMindDiscordGatewaySettings,
	type CometMindMemorySettings,
	type CometMindSettings,
	type CometMindSkillsSettings,
	type CometMindStorageSettings
} from './settings/schema';

/** Parse comma- or newline-separated IDs for text inputs. */
export function parseIdList(raw: string): string[] {
	return raw
		.split(/[\n,]+/)
		.map((part) => part.trim())
		.filter(Boolean);
}

export function formatIdList(ids: string[]): string {
	return ids.join('\n');
}
