import type { Meta, StoryObj } from '@storybook/svelte';
import ErrorEventRowStory from './wrappers/ErrorEventRowStory.svelte';

const meta = {
	title: 'Chat/ErrorEventRow',
	component: ErrorEventRowStory,
	tags: ['autodocs']
} satisfies Meta<typeof ErrorEventRowStory>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
