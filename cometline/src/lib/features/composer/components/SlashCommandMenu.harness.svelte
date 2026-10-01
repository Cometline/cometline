<script lang="ts">
	import SlashCommandMenu from './SlashCommandMenu.svelte';
</script>

<SlashCommandMenu ariaLabel="Slash commands">
	<button
		type="button"
		class="skill-command-option highlighted"
		role="option"
		aria-selected="true">/model</button
	>
	<button type="button" class="skill-command-option" role="option" aria-selected="false"
		>/change</button
	>
</SlashCommandMenu>
