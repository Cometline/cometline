import type { Meta, StoryObj } from '@storybook/svelte';
import ThreadRowStory from './wrappers/ThreadRowStory.svelte';

const meta = {
	title: 'Chat/ThreadRow',
	component: ThreadRowStory,
	tags: ['autodocs']
} satisfies Meta<typeof ThreadRowStory>;

export default meta;
type Story = StoryObj<typeof meta>;

export const User: Story = {
	args: { variant: 'user' }
};

export const Assistant: Story = {
	args: { variant: 'assistant' }
};

export const Event: Story = {
	args: { variant: 'event' }
};

export const Continuation: Story = {
	args: { variant: 'user', continuationRow: true }
};
