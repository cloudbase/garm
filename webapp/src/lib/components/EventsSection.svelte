<script lang="ts">
	import type { EntityEvent } from '$lib/api/generated/api.js';
	import { formatDate } from '$lib/utils/common.js';
	import Badge from './Badge.svelte';

	export let events: EntityEvent[] | undefined;
	export let eventsContainer: HTMLElement | undefined = undefined;
</script>

{#if events && events.length > 0}
	<div class="bg-white dark:bg-gray-800 shadow rounded-lg">
		<div class="px-4 py-5 sm:p-6">
			<h2 class="text-lg font-medium text-gray-900 dark:text-white mb-4">Events</h2>
			<div bind:this={eventsContainer} class="space-y-3 max-h-96 overflow-y-auto scroll-smooth">
				{#each events as event}
					<div class="bg-gray-50 dark:bg-gray-700 p-4 rounded-lg">
						<div class="flex justify-between items-start">
							<p class="text-sm text-gray-900 dark:text-white flex-1 mr-4">{event.message}</p>
							<div class="flex items-center space-x-2 flex-shrink-0">
								{#if (event.event_level || 'info').toLowerCase() === 'error'}
									<Badge variant="error" text="Error" />
								{:else if (event.event_level || 'info').toLowerCase() === 'warning'}
									<Badge variant="warning" text="Warning" />
								{:else}
									<Badge variant="info" text="Info" />
								{/if}
								<span class="text-xs text-gray-500 dark:text-gray-400">{formatDate(event.created_at)}</span>
							</div>
						</div>
					</div>
				{/each}
			</div>
		</div>
	</div>
{:else}
	<div class="bg-white dark:bg-gray-800 shadow rounded-lg">
		<div class="px-4 py-5 sm:p-6">
			<h2 class="text-lg font-medium text-gray-900 dark:text-white mb-4">Events</h2>
			<div class="text-center py-8">
				<svg class="w-12 h-12 text-gray-400 dark:text-gray-500 mx-auto mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
				</svg>
				<p class="text-sm text-gray-500 dark:text-gray-400">No events available</p>
			</div>
		</div>
	</div>
{/if}