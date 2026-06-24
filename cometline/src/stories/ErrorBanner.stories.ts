import type { Meta, StoryObj } from '@storybook/svelte';
import ErrorBannerStory from './wrappers/ErrorBannerStory.svelte';

const meta = {
	title: 'UI/ErrorBanner',
	component: ErrorBannerStory,
	tags: ['autodocs']
} satisfies Meta<typeof ErrorBannerStory>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
	args: { message: 'Failed to save settings. Try again.' }
};

export const Dismissible: Story = {
	args: { message: 'Sidecar unreachable.', dismissible: true }
};
