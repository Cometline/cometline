export {
	cloneCometMindSettings,
	defaultCometMindAutonomousJobsSettings,
	defaultCometMindJobsSettings,
	defaultCometMindMCPSettings,
	defaultCometMindSettings,
	normalizeCometMindSettings,
	type CometMindACPSettings,
	type CometMindAutonomousJobsSettings,
	type CometMindDiscordGatewaySettings,
	type CometMindJobsNotificationSettings,
	type CometMindJobsSettings,
	type CometMindMCPSettings,
	type CometMindMemorySettings,
	type CometMindSettings,
	type CometMindSkillsSettings,
	type CometMindStorageSettings,
	type MCPOAuthSettings,
	type MCPServerConfig,
	type MCPTransport
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
