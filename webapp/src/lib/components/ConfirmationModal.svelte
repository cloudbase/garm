<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import Modal from './Modal.svelte';
	import Button from './Button.svelte';

	export let title: string;
	export let message: string;
	export let confirmText: string = 'Confirm';
	export let cancelText: string = 'Cancel';
	export let variant: 'danger' | 'warning' | 'info' = 'warning';
	export let loading: boolean = false;

	const dispatch = createEventDispatcher<{
		close: void;
		confirm: void;
	}>();

	function handleConfirm() {
		dispatch('confirm');
	}

	$: iconColor = variant === 'danger' ? 'text-red-600 dark:text-red-400' : 
				   variant === 'warning' ? 'text-yellow-600 dark:text-yellow-400' : 
				   'text-blue-600 dark:text-blue-400';
	
	$: bgColor = variant === 'danger' ? 'bg-red-100 dark:bg-red-900' : 
				 variant === 'warning' ? 'bg-yellow-100 dark:bg-yellow-900' : 
				 'bg-blue-100 dark:bg-blue-900';
</script>

<Modal on:close={() => dispatch('close')}>
	<div class="max-w-xl w-full p-6">
		<div class="mx-auto flex items-center justify-center h-12 w-12 rounded-full {bgColor} mb-4">
			{#if variant === 'danger'}
				<svg class="h-6 w-6 {iconColor}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L3.732 16.5c-.77.833.192 2.5 1.732 2.5z"></path>
				</svg>
			{:else if variant === 'warning'}
				<svg class="h-6 w-6 {iconColor}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.996-.833-2.464 0L3.34 16.5c-.77.833.192 2.5 1.732 2.5z"></path>
				</svg>
			{:else}
				<svg class="h-6 w-6 {iconColor}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
				</svg>
			{/if}
		</div>
		
		<div class="text-center">
			<h3 class="text-lg leading-6 font-medium text-gray-900 dark:text-white mb-2">{title}</h3>
			<div class="text-sm text-gray-500 dark:text-gray-400">
				<p>{message}</p>
			</div>
		</div>

		<div class="mt-6 flex justify-end space-x-3">
			<Button
				variant="secondary"
				on:click={() => dispatch('close')}
				disabled={loading}
			>
				{cancelText}
			</Button>
			<Button
				variant={variant === 'info' ? 'primary' : variant === 'warning' ? 'primary' : variant}
				on:click={handleConfirm}
				disabled={loading}
				loading={loading}
			>
				{confirmText}
			</Button>
		</div>
	</div>
</Modal>