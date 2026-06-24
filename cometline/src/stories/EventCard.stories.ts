import type { Meta, StoryObj } from '@storybook/svelte';
import EventCardStory from './wrappers/EventCardStory.svelte';

const meta = {
	title: 'Chat/EventCard',
	component: EventCardStory,
	tags: ['autodocs']
} satisfies Meta<typeof EventCardStory>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
	args: { variant: 'default' }
};

export const Error: Story = {
	args: { variant: 'error' }
};
