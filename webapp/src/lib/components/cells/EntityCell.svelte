<script lang="ts">
	import { resolve } from '$app/paths';
	import { onMount, onDestroy } from 'svelte';

	export let item: any;
	export let entityType: 'repository' | 'organization' | 'enterprise' | 'pool' | 'scaleset' | 'instance' | 'template' | 'object' | 'credentials' = 'repository';
	export let showOwner: boolean = false;
	export let showId: boolean = false;
	export let fontMono: boolean = false;

	$: entityName = getEntityName();
	$: entityUrl = getEntityUrl();

	// Tooltip state for fixed positioning
	let iconElement: SVGSVGElement | null = null;
	let showTooltip = false;
	let tooltipX = 0;
	let tooltipY = 0;
	let positionAbove = false;

	function updateTooltipPosition() {
		if (iconElement) {
			const rect = iconElement.getBoundingClientRect();
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

	function getEntityName(): string {
		// Safety check for undefined item
		if (!item) return 'Unknown';

		switch (entityType) {
			case 'repository':
				return showOwner ? `${item.owner || 'Unknown'}/${item.name || 'Unknown'}` : (item.name || 'Unknown');
			case 'organization':
			case 'enterprise':
				return item.name || 'Unknown';
			case 'pool':
				return showId ? (item.id || 'Unknown') : (item.name || 'Unknown');
			case 'scaleset':
				return item.name || 'Unknown';
			case 'instance':
				return item.name || 'Unknown';
			case 'template':
				return item.name || 'Unknown';
			case 'object':
				return item.name || 'Unknown';
			case 'credentials':
				return item.name || 'Unknown';
			default:
				return item.name || item.id || 'Unknown';
		}
	}

	function getEntityUrl(): string {
		// Safety check for undefined item
		if (!item) return '#';

		let entityId;
		switch (entityType) {
			case 'instance':
				// For instances, always use name, not ID
				entityId = item.name;
				break;
			default:
				// For other entities, use ID first, then name as fallback
				entityId = item.id || item.name;
				break;
		}

		if (!entityId) return '#';

		switch (entityType) {
			case 'repository':
				return resolve(`/repositories/${entityId}`);
			case 'organization':
				return resolve(`/organizations/${entityId}`);
			case 'enterprise':
				return resolve(`/enterprises/${entityId}`);
			case 'pool':
				return resolve(`/pools/${entityId}`);
			case 'scaleset':
				return resolve(`/scalesets/${entityId}`);
			case 'instance':
				return resolve(`/instances/${encodeURIComponent(entityId)}`);
			case 'template':
				return resolve(`/templates/${entityId}`);
			case 'object':
				return resolve(`/objects/${entityId}`);
			case 'credentials':
				return resolve(`/credentials/${entityId}`);
			default:
				return '#';
		}
	}
</script>

<div class="w-full min-w-0 text-sm font-medium">
	<div class="flex items-center gap-1.5">
		<a
			href={entityUrl}
			class="truncate text-blue-600 dark:text-blue-400 hover:text-blue-500 dark:hover:text-blue-300 {fontMono ? 'font-mono' : ''}"
			title={entityName}
		>{entityName}</a>
		{#if entityType === 'object' && item?.description}
			<div class="flex-shrink-0">
				<svg
					bind:this={iconElement}
					on:mouseenter={handleMouseEnter}
					on:mouseleave={handleMouseLeave}
					role="img"
					aria-label="Has description"
					class="w-4 h-4 text-gray-400 dark:text-gray-500 cursor-help"
					fill="none"
					stroke="currentColor"
					viewBox="0 0 24 24"
				>
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
				</svg>
			</div>
		{/if}
	</div>
	{#if entityType === 'instance' && item?.provider_id}
		<div class="text-sm text-gray-500 dark:text-gray-400 truncate">
			{item.provider_id}
		</div>
	{/if}
</div>

<!-- Tooltip portal - uses fixed positioning to escape all containers -->
{#if entityType === 'object' && item?.description && showTooltip}
	<div
		class="fixed z-50 w-64 pointer-events-none"
		style="left: {tooltipX}px; top: {tooltipY}px; transform: translateY({positionAbove ? '-100%' : '0'});"
	>
		<div class="bg-gray-900 dark:bg-gray-700 text-white text-xs rounded-md px-3 py-2 shadow-lg">
			<div class="font-semibold mb-1">Description:</div>
			<div class="whitespace-pre-wrap break-words max-h-32 overflow-y-auto">
				{item.description}
			</div>
		</div>
	</div>
{/if}