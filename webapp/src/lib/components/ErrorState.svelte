<script lang="ts">
	import Button from './Button.svelte';

	export let title: string = 'Error loading data';
	export let message: string;
	export let showRetry: boolean = false;
	export let onRetry: (() => void) | undefined = undefined;
</script>

<div class="p-6 text-center">
	<svg class="mx-auto h-12 w-12 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
		<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
	</svg>
	<h3 class="mt-2 text-sm font-medium text-gray-900 dark:text-white">{title}</h3>
	<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{message}</p>
	{#if showRetry && onRetry}
		<div class="mt-4">
			<Button
				variant="secondary"
				size="sm"
				on:click={onRetry}
				icon="<path stroke-linecap='round' stroke-linejoin='round' stroke-width='2' d='M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15' />"
			>
				Retry
			</Button>
		</div>
	{/if}
</div>
