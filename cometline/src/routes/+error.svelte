<script lang="ts">
	import { page } from '$app/state';

	let message = $derived(page.error?.message ?? 'Something went wrong');
	let status = $derived(page.status ?? 500);
</script>

<div class="error-page" role="alert">
	<div class="error-card">
		<p class="error-status">{status}</p>
		<h1>Unable to load this page</h1>
		<p class="error-message">{message}</p>
		<a class="home-link" href="/">Return home</a>
	</div>
</div>

<style>
	.error-page {
		min-height: 100vh;
		display: grid;
		place-items: center;
		padding: 2rem;
		background: var(--app-bg);
		color: var(--text-main);
	}

	.error-card {
		max-width: 420px;
		padding: 28px 32px;
		background: var(--panel-bg);
		border: 1px solid var(--border-soft);
		border-radius: var(--radius-card);
		box-shadow: var(--shadow-card);
		text-align: center;
	}

	.error-status {
		margin: 0 0 8px;
		font-size: 12px;
		font-weight: 700;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		color: var(--status-error);
	}

	h1 {
		margin: 0 0 10px;
		font-size: 20px;
		font-weight: 650;
	}

	.error-message {
		margin: 0 0 18px;
		font-size: 14px;
		line-height: 1.5;
		color: var(--text-muted);
	}

	.home-link {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 10px 16px;
		border-radius: 10px;
		background: var(--accent);
		color: #fff;
		font-size: 13px;
		font-weight: 600;
		text-decoration: none;
	}

	.home-link:hover {
		opacity: 0.92;
	}
</style>
