<script lang="ts">
	import { toastStore, type Toast } from '$lib/stores/toast.js';
	import Button from './Button.svelte';

	// Subscribe to toast store
	$: toasts = $toastStore;

	function getToastIcon(type: Toast['type']) {
		switch (type) {
			case 'success':
				return `<svg class="h-5 w-5 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/>
				</svg>`;
			case 'error':
				return `<svg class="h-5 w-5 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z"/>
				</svg>`;
			case 'warning':
				return `<svg class="h-5 w-5 text-yellow-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L3.732 16.5c-.77.833.192 2.5 1.732 2.5z"/>
				</svg>`;
			case 'info':
			default:
				return `<svg class="h-5 w-5 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/>
				</svg>`;
		}
	}

	function getToastClasses(type: Toast['type']) {
		switch (type) {
			case 'success':
				return 'bg-green-50 dark:bg-green-900 border-green-200 dark:border-green-700';
			case 'error':
				return 'bg-red-50 dark:bg-red-900 border-red-200 dark:border-red-700';
			case 'warning':
				return 'bg-yellow-50 dark:bg-yellow-900 border-yellow-200 dark:border-yellow-700';
			case 'info':
			default:
				return 'bg-blue-50 dark:bg-blue-900 border-blue-200 dark:border-blue-700';
		}
	}

	function getTitleClasses(type: Toast['type']) {
		switch (type) {
			case 'success':
				return 'text-green-800 dark:text-green-200';
			case 'error':
				return 'text-red-800 dark:text-red-200';
			case 'warning':
				return 'text-yellow-800 dark:text-yellow-200';
			case 'info':
			default:
				return 'text-blue-800 dark:text-blue-200';
		}
	}

	function getMessageClasses(type: Toast['type']) {
		switch (type) {
			case 'success':
				return 'text-green-700 dark:text-green-300';
			case 'error':
				return 'text-red-700 dark:text-red-300';
			case 'warning':
				return 'text-yellow-700 dark:text-yellow-300';
			case 'info':
			default:
				return 'text-blue-700 dark:text-blue-300';
		}
	}
</script>

<!-- Toast Container -->
<div class="fixed top-4 right-4 z-[60] space-y-4 max-w-sm">
	{#each toasts as toast (toast.id)}
		<div 
			class="relative rounded-lg border p-4 shadow-lg transition-all duration-300 ease-in-out {getToastClasses(toast.type)}"
		>
			<div class="flex">
				<div class="flex-shrink-0">
					{@html getToastIcon(toast.type)}
				</div>
				<div class="ml-3 flex-1">
					<h3 class="text-sm font-medium {getTitleClasses(toast.type)}">
						{toast.title}
					</h3>
					{#if toast.message}
						<div class="mt-1 text-sm {getMessageClasses(toast.type)}">
							{toast.message}
						</div>
					{/if}
				</div>
				<div class="ml-4 flex-shrink-0">
					<Button
						variant="ghost"
						size="sm"
						aria-label="Dismiss notification"
						icon="<path stroke-linecap='round' stroke-linejoin='round' stroke-width='2' d='M6 18L18 6M6 6l12 12'/>"
						class="{toast.type === 'success' ? 'text-green-400 hover:text-green-500 focus:ring-green-500' : toast.type === 'error' ? 'text-red-400 hover:text-red-500 focus:ring-red-500' : toast.type === 'warning' ? 'text-yellow-400 hover:text-yellow-500 focus:ring-yellow-500' : 'text-blue-400 hover:text-blue-500 focus:ring-blue-500'}"
						on:click={() => toastStore.remove(toast.id)}
					>
					</Button>
				</div>
			</div>
		</div>
	{/each}
</div>