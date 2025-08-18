<script lang="ts">
	import type { Pool } from '$lib/api/generated/api.js';
	import { resolve } from '$app/paths';
	import { getEnabledStatusBadge, getEntityName } from '$lib/utils/common.js';
	import { eagerCache } from '$lib/stores/eager-cache.js';
	import DataTable from './DataTable.svelte';
	import Button from './Button.svelte';
	import { EntityCell, StatusCell, GenericCell } from './cells';
	import { createEventDispatcher } from 'svelte';

	export let pools: Pool[];
	export let entityType: 'repository' | 'organization' | 'enterprise';
	export let entityId: string = '';
	export let entityName: string = '';

	const dispatch = createEventDispatcher<{
		addPool: {
			entityType: 'repository' | 'organization' | 'enterprise';
			entityId: string;
			entityName: string;
		};
	}>();

	function handleAddPool() {
		dispatch('addPool', {
			entityType,
			entityId,
			entityName
		});
	}

	// DataTable configuration for pools section
	const columns = [
		{ 
			key: 'id', 
			title: 'ID',
			flexible: true, // Share remaining space equally with Image column
			cellComponent: EntityCell,
			cellProps: { entityType: 'pool', showId: true, fontMono: true }
		},
		{ 
			key: 'image', 
			title: 'Image',
			flexible: true, // Share remaining space equally with ID column
			cellComponent: GenericCell,
			cellProps: { field: 'image', type: 'code', showTitle: true }
		},
		{ 
			key: 'provider', 
			title: 'Provider',
			// Auto-size to content (widest cell in this column)
			cellComponent: GenericCell,
			cellProps: { field: 'provider_name' }
		},
		{ 
			key: 'status', 
			title: 'Status',
			// Auto-size to content (widest cell in this column)
			cellComponent: StatusCell,
			cellProps: { statusType: 'enabled' }
		}
	];

	// Mobile card configuration for pools section
	const mobileCardConfig = {
		entityType: 'pool' as const,
		primaryText: {
			field: 'id',
			isClickable: true,
			href: '/pools/{id}',
			useId: true,
			isMonospace: true
		},
		secondaryText: {
			field: 'entity_name',
			computedValue: (item: any) => getEntityName(item, $eagerCache)
		},
		badges: [
			{
				type: 'custom' as const,
				value: (item: any) => ({
					variant: item.enabled ? 'success' : 'error',
					text: item.enabled ? 'Enabled' : 'Disabled'
				})
			}
		]
	};
</script>

<div class="bg-white dark:bg-gray-800 shadow rounded-lg">
	<div class="px-4 py-5 sm:p-6">
		<div class="flex items-center justify-between mb-4">
			<h2 class="text-lg font-medium text-gray-900 dark:text-white">Pools ({pools.length})</h2>
			<a href={resolve('/pools')} class="text-sm text-blue-600 dark:text-blue-400 hover:text-blue-500 dark:hover:text-blue-300">View all pools</a>
		</div>
		{#if pools.length === 0}
			<!-- Custom empty state with Add Pool button -->
			<div class="p-6 text-center">
				<svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
				</svg>
				<h3 class="mt-2 text-sm font-medium text-gray-900 dark:text-white">No pools configured</h3>
				<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">No pools configured for this {entityType}.</p>
				<div class="mt-4">
					<Button
						variant="primary"
						size="sm"
						on:click={handleAddPool}
					>
						Add Pool
					</Button>
				</div>
			</div>
		{:else}
			<DataTable
				{columns}
				data={pools}
				loading={false}
				error=""
				searchTerm=""
				showSearch={false}
				showPagination={false}
				currentPage={1}
				perPage={pools.length}
				totalPages={1}
				totalItems={pools.length}
				itemName="pools"
				emptyTitle="No pools configured"
				emptyMessage="No pools configured for this {entityType}."
				emptyIconType="cog"
				{mobileCardConfig}
			/>
		{/if}
	</div>
</div>