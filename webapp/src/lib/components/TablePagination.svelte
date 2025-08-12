<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import Button from './Button.svelte';
	
	export let currentPage: number = 1;
	export let totalPages: number = 1;
	export let perPage: number = 25;
	export let totalItems: number = 0;
	export let itemName: string = 'results';
	
	const dispatch = createEventDispatcher<{
		pageChange: { page: number };
	}>();
	
	function changePage(targetPage: number) {
		if (targetPage >= 1 && targetPage <= totalPages && targetPage !== currentPage) {
			dispatch('pageChange', { page: targetPage });
		}
	}
	
	$: startItem = totalItems === 0 ? 0 : (currentPage - 1) * perPage + 1;
	$: endItem = Math.min(currentPage * perPage, totalItems);
</script>

{#if totalPages > 1}
	<div class="bg-white dark:bg-gray-800 px-4 py-3 flex items-center justify-between border-t border-gray-200 dark:border-gray-700 sm:px-6">
		<!-- Mobile pagination -->
		<div class="flex-1 flex justify-between sm:hidden">
			<Button
				variant="secondary"
				on:click={() => changePage(currentPage - 1)}
				disabled={currentPage === 1}
			>
				Previous
			</Button>
			<Button
				variant="secondary"
				on:click={() => changePage(currentPage + 1)}
				disabled={currentPage === totalPages}
				class="ml-3"
			>
				Next
			</Button>
		</div>
		
		<!-- Desktop pagination -->
		<div class="hidden sm:flex-1 sm:flex sm:items-center sm:justify-between">
			<div>
				<p class="text-sm text-gray-700 dark:text-gray-300">
					{#if totalItems === 0}
						No {itemName}
					{:else}
						Showing <span class="font-medium">{startItem}</span>
						to <span class="font-medium">{endItem}</span>
						of <span class="font-medium">{totalItems}</span> {itemName}
					{/if}
				</p>
			</div>
			<div>
				<nav class="relative z-0 inline-flex rounded-md shadow-sm -space-x-px">
					<Button
						variant="secondary"
						size="sm"
						on:click={() => changePage(currentPage - 1)}
						disabled={currentPage === 1}
						class="rounded-r-none"
						aria-label="Previous page"
						icon="<path stroke-linecap='round' stroke-linejoin='round' stroke-width='2' d='M15 19l-7-7 7-7'></path>"
					>
					</Button>
					
					{#each Array(totalPages) as _, i}
						{@const page = i + 1}
						<Button
							variant={page === currentPage ? 'primary' : 'secondary'}
							size="sm"
							on:click={() => changePage(page)}
							class="rounded-none border-l-0 first:border-l first:rounded-l-md"
						>
							{page}
						</Button>
					{/each}

					<Button
						variant="secondary"
						size="sm"
						on:click={() => changePage(currentPage + 1)}
						disabled={currentPage === totalPages}
						class="rounded-l-none"
						aria-label="Next page"
						icon="<path stroke-linecap='round' stroke-linejoin='round' stroke-width='2' d='M9 5l7 7-7 7'></path>"
					>
					</Button>
				</nav>
			</div>
		</div>
	</div>
{/if}