<script lang="ts">
	import { formatDate } from '$lib/utils/common.js';
	
	export let item: any;
	export let field: string;
	export let type: 'text' | 'code' | 'truncated' | 'description' | 'date' = 'text';
	export let truncateLength: number = 50;
	export let showTitle: boolean = false;

	$: value = getValue();
	$: displayValue = getDisplayValue();

	function getValue() {
		// Safety check for undefined item
		if (!item) return '';
		
		// Support nested field access like 'endpoint.name'
		return field.split('.').reduce((obj, key) => obj?.[key], item) || '';
	}

	function getDisplayValue() {
		if (type === 'date') {
			return formatDate(value);
		}
		if (type === 'truncated' && value.length > truncateLength) {
			return `${value.slice(0, truncateLength)}...`;
		}
		return value;
	}

	function getClasses() {
		switch (type) {
			case 'code':
				return 'inline-block max-w-full truncate bg-gray-100 dark:bg-gray-700 px-2 py-1 rounded text-xs font-mono';
			case 'description':
				return 'block w-full truncate text-sm text-gray-500 dark:text-gray-300';
			case 'date':
				return 'block w-full truncate text-sm text-gray-900 dark:text-white font-mono';
			default:
				return 'block w-full truncate text-sm text-gray-900 dark:text-white';
		}
	}
</script>

{#if type === 'code'}
	<code 
		class="{getClasses()} {showTitle ? 'cursor-default' : ''}"
		title={showTitle ? value : ''}
	>
		{displayValue}
	</code>
{:else}
	<span 
		class="{getClasses()} {showTitle ? 'cursor-default' : ''}"
		title={showTitle ? value : ''}
	>
		{displayValue}
	</span>
{/if}