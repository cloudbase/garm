<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import Modal from './Modal.svelte';
	import Button from './Button.svelte';

	export let title: string;
	export let message: string;
	export let itemName: string = '';
	export let loading: boolean = false;

	const dispatch = createEventDispatcher<{
		close: void;
		confirm: void;
	}>();

	function handleConfirm() {
		dispatch('confirm');
	}
</script>

<Modal on:close={() => dispatch('close')}>
	<div class="max-w-xl w-full p-6">
		<div class="mx-auto flex items-center justify-center h-12 w-12 rounded-full bg-red-100 dark:bg-red-900 mb-4">
			<svg class="h-6 w-6 text-red-600 dark:text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L3.732 16.5c-.77.833.192 2.5 1.732 2.5z"></path>
			</svg>
		</div>
		
		<div class="text-center">
			<h3 class="text-lg leading-6 font-medium text-gray-900 dark:text-white mb-2">{title}</h3>
			<div class="text-sm text-gray-500 dark:text-gray-400">
				<p>{message}</p>
				{#if itemName}
					<p class="mt-1 font-medium text-gray-900 dark:text-white">{itemName}</p>
				{/if}
			</div>
		</div>

		<div class="mt-6 flex justify-end space-x-3">
			<Button
				variant="secondary"
				on:click={() => dispatch('close')}
				disabled={loading}
			>
				Cancel
			</Button>
			<Button
				variant="danger"
				on:click={handleConfirm}
				disabled={loading}
				loading={loading}
			>
				{loading ? 'Deleting...' : 'Delete'}
			</Button>
		</div>
	</div>
</Modal>