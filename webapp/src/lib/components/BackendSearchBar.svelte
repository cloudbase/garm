<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import SearchBar from './SearchBar.svelte';
	import Button from './Button.svelte';

	export let value: string = '';
	export let placeholder: string = 'Search...';
	export let disabled: boolean = false;
	export let helpText: string = '';
	export let showButton: boolean = true;

	const dispatch = createEventDispatcher<{
		search: string;
	}>();

	function handleSearch() {
		dispatch('search', value);
	}

	function handleInput() {
		// Dispatch search on every input (parent handles debouncing)
		dispatch('search', value);
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter') {
			handleSearch();
		}
	}
</script>

<div class="bg-white dark:bg-gray-800 shadow rounded-lg p-4">
	<div class="space-y-2">
		<div class="flex gap-2 items-start">
			<div class="flex-1">
				<SearchBar
					bind:value
					{placeholder}
					{disabled}
					on:input={handleInput}
					on:keydown={handleKeydown}
				/>
			</div>
			{#if showButton}
				<Button variant="secondary" on:click={handleSearch} {disabled}>
					Search
				</Button>
			{/if}
		</div>
		{#if helpText}
			<p class="text-sm text-gray-500 dark:text-gray-400">
				{helpText}
			</p>
		{/if}
	</div>
</div>
