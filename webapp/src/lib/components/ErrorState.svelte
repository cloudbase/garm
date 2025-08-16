<script lang="ts">
	import Button from './Button.svelte';

	export let title: string = 'Error';
	export let message: string;
	export let showRetry: boolean = false;
	export let onRetry: (() => void) | undefined = undefined;
</script>

<div class="p-6">
	<div class="rounded-md bg-red-50 dark:bg-red-900 p-4">
		<div class="flex">
			<div class="flex-shrink-0">
				<svg class="h-5 w-5 text-red-400" viewBox="0 0 20 20" fill="currentColor">
					<path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
				</svg>
			</div>
			<div class="ml-3 flex-1">
				<h3 class="text-sm font-medium text-red-800 dark:text-red-200">{title}</h3>
				<p class="mt-2 text-sm text-red-700 dark:text-red-300">{message}</p>
				{#if showRetry && onRetry}
					<div class="mt-3">
						<Button
							variant="secondary"
							size="sm"
							on:click={onRetry}
							icon="<path stroke-linecap='round' stroke-linejoin='round' stroke-width='2' d='M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15' />"
							class="text-red-700 dark:text-red-200 bg-red-100 dark:bg-red-800 hover:bg-red-200 dark:hover:bg-red-700 focus:outline-none focus:bg-red-200 dark:focus:bg-red-700"
						>
							Retry
						</Button>
					</div>
				{/if}
			</div>
		</div>
	</div>
</div>