import type { ModelOption } from '#lib/stores/model.svelte.js';

export function groupModelCommandOptions(options: ModelOption[]) {
	const groups: {
		providerId: string;
		providerName: string;
		providerMethod: string;
		options: ModelOption[];
	}[] = [];
	for (const option of options) {
		let group = groups.find((item) => item.providerId === option.providerId);
		if (!group) {
			group = {
				providerId: option.providerId,
				providerName: option.providerName,
				providerMethod: option.providerMethod,
				options: []
			};
			groups.push(group);
		}
		group.options.push(option);
	}
	return groups;
}
