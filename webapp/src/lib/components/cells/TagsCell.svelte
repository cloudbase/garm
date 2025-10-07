<script lang="ts">
	export let item: any;

	// Get tags from item
	$: tags = item?.tags || [];

	// Show maximum 3 tags, with "+X more" indicator
	const MAX_VISIBLE_TAGS = 3;
	$: visibleTags = tags.slice(0, MAX_VISIBLE_TAGS);
	$: hiddenCount = Math.max(0, tags.length - MAX_VISIBLE_TAGS);
</script>

{#if !tags || tags.length === 0}
	<span class="text-gray-500 dark:text-gray-400">-</span>
{:else}
	<div class="flex flex-wrap gap-1 items-center group relative">
		{#each visibleTags as tag}
			<span class="inline-flex items-center px-2 py-0.5 text-xs font-medium text-blue-700 dark:text-blue-300 bg-blue-100 dark:bg-blue-900 rounded-md">
				{tag}
			</span>
		{/each}
		{#if hiddenCount > 0}
			<span class="inline-flex items-center px-2 py-0.5 text-xs font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-md">
				+{hiddenCount} more
			</span>
		{/if}

		<!-- Tooltip showing all tags on hover -->
		{#if tags.length > MAX_VISIBLE_TAGS}
			<div class="absolute left-0 top-full mt-2 z-10 invisible group-hover:visible opacity-0 group-hover:opacity-100 transition-opacity duration-200 pointer-events-none">
				<div class="bg-gray-900 dark:bg-gray-700 text-white text-xs rounded-md px-3 py-2 max-w-sm shadow-lg">
					<div class="font-semibold mb-1">All tags:</div>
					<div class="flex flex-wrap gap-1">
						{#each tags as tag}
							<span class="inline-flex items-center px-2 py-0.5 text-xs font-medium text-blue-200 bg-blue-800/50 rounded-md">
								{tag}
							</span>
						{/each}
					</div>
				</div>
			</div>
		{/if}
	</div>
{/if}
