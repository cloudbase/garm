<script lang="ts">
	import { base } from '$app/paths';
	import { getEntityName, getEntityType, getEntityUrl } from '$lib/utils/common.js';

	export let item: any;
	export let eagerCache: any = null;
</script>

<div class="flex flex-col">
	<a href={getEntityUrl(item)} class="text-sm font-medium text-blue-600 dark:text-blue-400 hover:text-blue-500 dark:hover:text-blue-300">
		{getEntityName(item, eagerCache)}
	</a>
	<span class="text-xs text-gray-500 dark:text-gray-400 capitalize">
		{getEntityType(item)}
	</span>
</div>