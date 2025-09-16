<script lang="ts">
	import { onMount, onDestroy } from 'svelte';

	export let item: any;

	// Get tags from item
	$: tags = item?.tags || [];

	// Show maximum 3 tags, with "+X more" indicator
	const MAX_VISIBLE_TAGS = 3;
	$: visibleTags = tags.slice(0, MAX_VISIBLE_TAGS);
	$: hiddenCount = Math.max(0, tags.length - MAX_VISIBLE_TAGS);

	// Tooltip state for fixed positioning
	let containerElement: HTMLDivElement | null = null;
	let showTooltip = false;
	let tooltipX = 0;
	let tooltipY = 0;
	let positionAbove = false;

	function updateTooltipPosition() {
		if (containerElement) {
			const rect = containerElement.getBoundingClientRect();
			tooltipX = rect.left;

			// Check if there's enough space below (estimate tooltip height ~150px)
			const spaceBelow = window.innerHeight - rect.bottom;
			const tooltipEstimatedHeight = 150;

			if (spaceBelow < tooltipEstimatedHeight) {
				// Not enough space below, position above
				positionAbove = true;
				tooltipY = rect.top;
			} else {
				// Enough space, position below
				positionAbove = false;
				tooltipY = rect.bottom + 4;
			}
		}
	}

	function handleMouseEnter() {
		showTooltip = true;
		updateTooltipPosition();
	}

	function handleMouseLeave() {
		showTooltip = false;
	}

	onMount(() => {
		window.addEventListener('scroll', updateTooltipPosition, true);
		window.addEventListener('resize', updateTooltipPosition);
	});

	onDestroy(() => {
		window.removeEventListener('scroll', updateTooltipPosition, true);
		window.removeEventListener('resize', updateTooltipPosition);
	});
</script>

{#if !tags || tags.length === 0}
	<span class="text-gray-500 dark:text-gray-400">-</span>
{:else}
	<!-- svelte-ignore a11y-no-static-element-interactions -->
	<div
		bind:this={containerElement}
		on:mouseenter={tags.length > MAX_VISIBLE_TAGS ? handleMouseEnter : undefined}
		on:mouseleave={tags.length > MAX_VISIBLE_TAGS ? handleMouseLeave : undefined}
		class="flex flex-wrap gap-1 items-center"
	>
		{#each visibleTags as tag}
			<span class="inline-flex items-center px-2 py-0.5 text-xs font-medium text-blue-700 dark:text-blue-300 bg-blue-100 dark:bg-blue-900 rounded-md">
				{tag}
			</span>
		{/each}
		{#if hiddenCount > 0}
			<span class="inline-flex items-center px-2 py-0.5 text-xs font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-md cursor-help">
				+{hiddenCount} more
			</span>
		{/if}
	</div>
{/if}

<!-- Tooltip portal - uses fixed positioning to escape all containers -->
{#if tags.length > MAX_VISIBLE_TAGS && showTooltip}
	<div
		class="fixed z-50 max-w-sm pointer-events-none"
		style="left: {tooltipX}px; top: {tooltipY}px; transform: translateY({positionAbove ? '-100%' : '0'});"
	>
		<div class="bg-gray-900 dark:bg-gray-700 text-white text-xs rounded-md px-3 py-2 shadow-lg">
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
