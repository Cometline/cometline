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
