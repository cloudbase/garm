<script lang="ts">
	import { base } from '$app/paths';

	export let item: any;
</script>

<div class="w-full min-w-0 text-sm font-medium">
{#if item?.pool_id}
	<a href="{base}/pools/{item.pool_id}" class="text-blue-600 dark:text-blue-400 hover:text-blue-500 dark:hover:text-blue-300 truncate block text-sm" title="Pool: {item.pool_id}">
		Pool: {item.pool_id}
	</a>
{:else if item?.scale_set_id}
	<a href="{base}/scalesets/{item.scale_set_id}" class="text-blue-600 dark:text-blue-400 hover:text-blue-500 dark:hover:text-blue-300 truncate block text-sm" title="Scale Set: {item.scale_set_id}">
		Scale Set: {item.scale_set_id}
	</a>
{:else}
	<span class="text-gray-400 dark:text-gray-500 text-sm">-</span>
{/if}
</div>
