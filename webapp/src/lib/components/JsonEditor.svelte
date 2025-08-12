<script lang="ts">
	export let value: string = '';
	export let placeholder: string = '{}';
	export let rows: number = 4;
	export let disabled: boolean = false;

	let isValidJson = true;

	// Validate JSON as user types
	$: {
		if (value.trim()) {
			try {
				JSON.parse(value);
				isValidJson = true;
			} catch (e) {
				isValidJson = false;
			}
		} else {
			isValidJson = true;
		}
	}
</script>

<div class="relative">
	<textarea
		bind:value={value}
		{placeholder}
		{rows}
		{disabled}
		class="w-full px-3 py-2 border rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 font-mono text-sm resize-none
			{isValidJson 
				? 'border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-white' 
				: 'border-red-300 dark:border-red-600 bg-red-50 dark:bg-red-900/20 text-red-900 dark:text-red-100'
			}
			{disabled ? 'opacity-50 cursor-not-allowed' : ''}
		"
		style="tab-size: 2;"
		spellcheck="false"
	></textarea>
	
	{#if !isValidJson}
		<div class="absolute top-2 right-2">
			<svg class="w-4 h-4 text-red-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L3.732 16.5c-.77.833.192 2.5 1.732 2.5z"></path>
			</svg>
		</div>
	{/if}
</div>