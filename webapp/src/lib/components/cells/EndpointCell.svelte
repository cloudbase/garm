<script lang="ts">
	import { resolve } from '$app/paths';
	import { getForgeIcon } from '$lib/utils/common.js';

	export let item: any;
	export let iconSize: string = 'w-5 h-5';
	export let linkTo: string = '';

	$: displayName = item?.endpoint?.name || item?.endpoint_name || item?.endpoint_type || 'Unknown';

	function getLink(): string {
		if (!linkTo || !item?.id) return '';
		switch (linkTo) {
			case 'forge_instance': return resolve(`/forge-instances/${item.id}`);
			default: return '';
		}
	}

	$: href = getLink();
</script>

<div class="flex items-center">
	<div class="flex-shrink-0 mr-2">
		{@html getForgeIcon(item?.endpoint?.endpoint_type || item?.endpoint_type || 'unknown', iconSize)}
	</div>
	{#if href}
		<a {href} class="text-sm font-medium text-blue-600 dark:text-blue-400 hover:text-blue-500 dark:hover:text-blue-300">
			{displayName}
		</a>
	{:else}
		<div class="text-sm text-gray-900 dark:text-white">
			{displayName}
		</div>
	{/if}
</div>