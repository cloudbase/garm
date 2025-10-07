<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import Icons from './Icons.svelte';

	export let value: string = '';
	export let placeholder: string = 'Search...';
	export let disabled: boolean = false;

	const dispatch = createEventDispatcher<{
		input: string;
	}>();

	function handleInput() {
		dispatch('input', value);
	}
</script>

<div class="relative">
	<div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
		<Icons name="search" class="h-5 w-5 text-gray-400" />
	</div>
	<input
		bind:value
		on:input={handleInput}
		on:keydown
		type="text"
		class="block w-full pl-10 pr-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md leading-5 bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-500 dark:placeholder-gray-400 focus:outline-none focus:placeholder-gray-400 focus:ring-1 focus:ring-blue-500 focus:border-blue-500 sm:text-sm disabled:opacity-50 disabled:cursor-not-allowed"
		{placeholder}
		{disabled}
	/>
</div>