import type { Meta, StoryObj } from '@storybook/svelte';
import RuntimeOverlayStory from './wrappers/RuntimeOverlayStory.svelte';

const meta = {
	title: 'UI/RuntimeOverlay',
	component: RuntimeOverlayStory,
	tags: ['autodocs'],
	parameters: {
		layout: 'fullscreen'
	}
} satisfies Meta<typeof RuntimeOverlayStory>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Connecting: Story = {
	args: { mode: 'connecting' }
};

export const Error: Story = {
	args: { mode: 'error' }
};
