<script lang="ts">
	import { getForgeIcon } from '$lib/utils/common.js';

	export let item: any;
	export let iconSize: string = 'w-5 h-5';
</script>

<div class="flex items-center">
	<div class="flex-shrink-0 mr-2">
		{@html getForgeIcon(item?.endpoint?.endpoint_type || item?.endpoint_type || 'unknown', iconSize)}
	</div>
	<div class="text-sm text-gray-900 dark:text-white">
		{item?.endpoint?.name || item?.endpoint_name || item?.endpoint_type ||'Unknown'}
	</div>
</div>