import type { Meta, StoryObj } from '@storybook/svelte';
import ThinkingBlockStory from './wrappers/ThinkingBlockStory.svelte';

const meta = {
	title: 'Chat/ThinkingBlock',
	component: ThinkingBlockStory,
	tags: ['autodocs']
} satisfies Meta<typeof ThinkingBlockStory>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Collapsed: Story = {
	args: { expanded: false }
};

export const Expanded: Story = {
	args: { expanded: true }
};
