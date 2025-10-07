<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import Button from './Button.svelte';

	export let currentPage: number = 1;
	export let totalPages: number = 1;
	export let totalItems: number = 0;
	export let pageSize: number = 25;
	export let loading: boolean = false;
	export let itemName: string = 'items';

	const dispatch = createEventDispatcher<{
		pageChange: { page: number };
		pageSizeChange: { pageSize: number };
		prefetch: { page: number };
	}>();

	$: hasNext = currentPage < totalPages;
	$: hasPrev = currentPage > 1;
	$: startItem = totalItems === 0 ? 0 : (currentPage - 1) * pageSize + 1;
	$: endItem = Math.min(currentPage * pageSize, totalItems);

	function handlePrevious() {
		if (hasPrev && !loading) {
			dispatch('pageChange', { page: currentPage - 1 });
			// Prefetch the page before previous if it exists
			if (currentPage - 2 > 0) {
				dispatch('prefetch', { page: currentPage - 2 });
			}
		}
	}

	function handleNext() {
		if (hasNext && !loading) {
			dispatch('pageChange', { page: currentPage + 1 });
			// Prefetch the page after next if it exists
			if (currentPage + 2 <= totalPages) {
				dispatch('prefetch', { page: currentPage + 2 });
			}
		}
	}

	function handlePageSizeChange(event: Event) {
		const target = event.target as HTMLSelectElement;
		const newSize = parseInt(target.value);
		dispatch('pageSizeChange', { pageSize: newSize });
	}

	// Prefetch next page when current page loads
	$: if (currentPage < totalPages && !loading) {
		dispatch('prefetch', { page: currentPage + 1 });
	}
</script>

<div class="flex items-center justify-between border-t border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 px-4 py-3 sm:px-6">
	<div class="flex flex-1 justify-between sm:hidden">
		<Button
			variant="secondary"
			size="sm"
			disabled={!hasPrev || loading}
			on:click={handlePrevious}
		>
			Previous
		</Button>
		<Button
			variant="secondary"
			size="sm"
			disabled={!hasNext || loading}
			on:click={handleNext}
		>
			Next
		</Button>
	</div>
	<div class="hidden sm:flex sm:flex-1 sm:items-center sm:justify-between">
		<div class="flex items-center gap-4">
			<p class="text-sm text-gray-700 dark:text-gray-300">
				Showing
				<span class="font-medium">{startItem}</span>
				to
				<span class="font-medium">{endItem}</span>
				of
				<span class="font-medium">{totalItems}</span>
				{itemName}
			</p>
			<div class="flex items-center gap-2">
				<label for="page-size" class="text-sm text-gray-700 dark:text-gray-300">
					Show:
				</label>
				<select
					id="page-size"
					bind:value={pageSize}
					on:change={handlePageSizeChange}
					disabled={loading}
					class="rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white py-1 pl-2 pr-8 text-sm focus:border-blue-500 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed"
				>
					<option value={10}>10</option>
					<option value={25}>25</option>
					<option value={50}>50</option>
					<option value={100}>100</option>
				</select>
			</div>
		</div>
		<div class="flex items-center gap-2">
			<Button
				variant="secondary"
				size="sm"
				disabled={!hasPrev || loading}
				on:click={handlePrevious}
			>
				<svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
				</svg>
			</Button>
			<span class="text-sm text-gray-700 dark:text-gray-300">
				Page <span class="font-medium">{currentPage}</span> of <span class="font-medium">{totalPages}</span>
			</span>
			<Button
				variant="secondary"
				size="sm"
				disabled={!hasNext || loading}
				on:click={handleNext}
			>
				<svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
				</svg>
			</Button>
		</div>
	</div>
</div>
