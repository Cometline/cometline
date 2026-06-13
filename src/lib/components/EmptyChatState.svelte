<script lang="ts">
</script>

<div class="empty-state">
	<div class="avatar" aria-hidden="true">
		<img src="/project_icon.png" alt="" />
	</div>
	<p class="subtitle">A thought, a task, a file — Cometline continues.</p>
</div>

<style>
	.empty-state {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		text-align: center;
	}

	.avatar {
		width: 82px;
		height: 82px;
		border-radius: 50%;
		background: #fff;
		border: 1px solid var(--border-soft);
		margin-bottom: 24px;
		box-shadow: var(--shadow-card);
		overflow: hidden;
	}

	.avatar img {
		width: 100%;
		height: 100%;
		object-fit: cover;
		border-radius: 50%;
		display: block;
	}

	.subtitle {
		font-size: 14px;
		color: var(--text-muted);
		margin: 0;
	}
</style>
