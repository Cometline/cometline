import type { Meta, StoryObj } from '@storybook/svelte';
import UserMessageRowStory from './wrappers/UserMessageRowStory.svelte';

const meta = {
	title: 'Chat/UserMessageRow',
	component: UserMessageRowStory,
	tags: ['autodocs']
} satisfies Meta<typeof UserMessageRowStory>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const Copied: Story = {
	args: { copied: true }
};

export const LongMessage: Story = {
	args: {
		text: Array.from(
			{ length: 18 },
			(_, index) =>
				`Paragraph ${index + 1}: Explain how a long user message should stay readable.`
		).join('\n')
	}
};

export const LongUnbrokenMessage: Story = {
	args: {
		text: `A long token should still stay inside the bubble: ${'cometline'.repeat(90)}`
	}
};
