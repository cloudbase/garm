<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import Button from './Button.svelte';
	import Icons from './Icons.svelte';

	const dispatch = createEventDispatcher<{
		action: void;
	}>();

	export let title: string;
	export let description: string;
	export let actionLabel: string | null = null;
	export let showAction: boolean = true;

	function handleAction() {
		dispatch('action');
	}
</script>

<!-- Page Header -->
<div class="sm:flex sm:items-center sm:justify-between">
	<div>
		<h1 class="text-2xl font-bold text-gray-900 dark:text-white">{title}</h1>
		<p class="mt-2 text-sm text-gray-700 dark:text-gray-300">
			{description}
		</p>
	</div>
	{#if $$slots.actions}
		<div class="mt-4 sm:mt-0 flex items-center space-x-4">
			<slot name="actions" />
		</div>
	{:else if showAction && actionLabel || $$slots['secondary-actions']}
		<div class="mt-4 sm:mt-0 flex items-center space-x-3">
			{#if $$slots['secondary-actions']}
				<slot name="secondary-actions" />
			{/if}
			{#if showAction && actionLabel}
				<Button
					variant="primary"
					icon='<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6" />'
					on:click={handleAction}
				>
					{actionLabel}
				</Button>
			{/if}
		</div>
	{/if}
</div>