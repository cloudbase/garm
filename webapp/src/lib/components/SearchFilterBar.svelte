<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import SearchBar from './SearchBar.svelte';
	
	export let searchTerm: string = '';
	export let perPage: number = 25;
	export let placeholder: string = 'Search...';
	export let showPerPageSelector: boolean = true;
	export let perPageOptions: number[] = [25, 50, 100];
	
	const dispatch = createEventDispatcher<{
		search: { term: string };
		perPageChange: { perPage: number };
	}>();
	
	function handleSearchInput() {
		dispatch('search', { term: searchTerm });
	}
	
	function handlePerPageChange() {
		dispatch('perPageChange', { perPage });
	}
</script>

<div class="bg-white dark:bg-gray-800 shadow rounded-lg p-4">
	<div class="sm:flex sm:items-center sm:justify-between">
		<div class="flex-1 min-w-0">
			<div class="max-w-md">
				<label for="search" class="sr-only">Search</label>
				<SearchBar
					bind:value={searchTerm}
					on:input={handleSearchInput}
					{placeholder}
				/>
			</div>
		</div>
		{#if showPerPageSelector}
			<div class="mt-4 sm:mt-0 sm:ml-4 flex items-center space-x-4">
				<div class="flex items-center space-x-2">
					<label for="per-page" class="text-sm text-gray-700 dark:text-gray-300">Show:</label>
					<select
						id="per-page"
						bind:value={perPage}
						on:change={handlePerPageChange}
						class="block w-20 pl-2 pr-8 py-1 text-sm border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white rounded-md focus:outline-none focus:ring-1 focus:ring-blue-500 focus:border-blue-500"
					>
						{#each perPageOptions as option}
							<option value={option}>{option}</option>
						{/each}
					</select>
				</div>
			</div>
		{/if}
	</div>
</div>