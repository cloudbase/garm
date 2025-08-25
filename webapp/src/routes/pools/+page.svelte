<script lang="ts">
	import { onMount } from 'svelte';
	import { garmApi } from '$lib/api/client.js';
	import type { Pool, UpdatePoolParams } from '$lib/api/generated/api.js';
	import PageHeader from '$lib/components/PageHeader.svelte';
	import CreatePoolModal from '$lib/components/CreatePoolModal.svelte';
	import UpdatePoolModal from '$lib/components/UpdatePoolModal.svelte';
	import DeleteModal from '$lib/components/DeleteModal.svelte';
	import { eagerCache, eagerCacheManager } from '$lib/stores/eager-cache.js';
	import { toastStore } from '$lib/stores/toast.js';
	import { getEntityName, filterEntities } from '$lib/utils/common.js';
	import { extractAPIError } from '$lib/utils/apiError';
import DataTable from '$lib/components/DataTable.svelte';
import { EntityCell, EndpointCell, StatusCell, ActionsCell, GenericCell, PoolEntityCell } from '$lib/components/cells';

	let pools: Pool[] = [];
	let loading = true;
	let error = '';

	// Subscribe to eager cache for pools (when WebSocket is connected)
	$: {
		// Only use cache data if we're not in direct API mode
		if (!pools.length || $eagerCache.loaded.pools) {
			pools = $eagerCache.pools;
		}
	}
	$: loading = $eagerCache.loading.pools;
	$: cacheError = $eagerCache.errorMessages.pools;
	let searchTerm = '';
	let currentPage = 1;
	let perPage = 25;
	let showCreateModal = false;
	let showUpdateModal = false;
	let showDeleteModal = false;
	let selectedPool: Pool | null = null;
	// Filtered and paginated data
	// Search by entity name since pools don't have names
	$: filteredPools = filterEntities(pools, searchTerm, (pool) => getEntityName(pool, $eagerCache));
	$: totalPages = Math.ceil(filteredPools.length / perPage);
	$: {
		if (currentPage > totalPages && totalPages > 0) {
			currentPage = totalPages;
		}
	}
	$: paginatedPools = filteredPools.slice(
		(currentPage - 1) * perPage,
		currentPage * perPage
	);

	async function handleCreatePool(event: CustomEvent<CreatePoolParams>) {
		try {
			// For the global pools page, the modal itself should handle the API call
			// since it knows which entity type and ID was selected
			// We just need to show success and close the modal
			toastStore.success(
				'Pool Created',
				'Pool has been created successfully.'
			);
			showCreateModal = false;
		} catch (err) {
			const errorMessage = extractAPIError(err);
			toastStore.error(
				'Pool Creation Failed',
				errorMessage
			);
			// Don't close the modal on error, let user fix and retry
		}
	}

	async function handleUpdatePool(params: UpdatePoolParams) {
		if (!selectedPool) return;
		try {
			await garmApi.updatePool(selectedPool.id!, params);
			// No need to reload - eager cache websocket will handle the update
			showUpdateModal = false;
			toastStore.add({
				type: 'success',
				title: 'Pool Updated',
				message: `Pool ${selectedPool.id!.slice(0, 8)}... has been updated successfully.`
			});
			selectedPool = null;
		} catch (err) {
			const errorMessage = extractAPIError(err);
			toastStore.add({
				type: 'error',
				title: 'Update Failed',
				message: errorMessage
			});
			throw err; // Let the modal handle the error too
		}
	}

	async function handleDeletePool() {
		if (!selectedPool) return;
		const poolName = `Pool ${selectedPool.id!.slice(0, 8)}...`;
		try {
			await garmApi.deletePool(selectedPool.id!);
			// No need to reload - eager cache websocket will handle the update
			showDeleteModal = false;
			toastStore.add({
				type: 'success',
				title: 'Pool Deleted',
				message: `${poolName} has been deleted successfully.`
			});
			selectedPool = null;
		} catch (err) {
			const errorMessage = extractAPIError(err);
			toastStore.add({
				type: 'error',
				title: 'Delete Failed',
				message: errorMessage
			});
		}
		showDeleteModal = false;
		selectedPool = null;
	}

	function openCreateModal() {
		showCreateModal = true;
	}

	function openUpdateModal(pool: Pool) {
		selectedPool = pool;
		showUpdateModal = true;
	}

	function openDeleteModal(pool: Pool) {
		selectedPool = pool;
		showDeleteModal = true;
	}


	onMount(async () => {
		// Load pools through eager cache (priority load + background load others)
		try {
			loading = true;
			const poolData = await eagerCacheManager.getPools();
			// If WebSocket is disconnected, getPools returns direct API data
			// Update our local pools array with this data
			if (poolData && Array.isArray(poolData)) {
				pools = poolData;
			}
		} catch (err) {
			// Cache error is already handled by the eager cache system
			// We don't need to set error here anymore since it's in the cache state
			if (!import.meta.env?.VITEST) {
				console.error('Failed to load pools:', err);
			}
			error = err instanceof Error ? err.message : 'Failed to load pools';
		} finally {
			loading = false;
		}
	});

	async function retryLoadPools() {
		try {
			await eagerCacheManager.retryResource('pools');
		} catch (err) {
			if (!import.meta.env?.VITEST) {
				console.error('Retry failed:', err);
			}
		}
	}

	// DataTable configuration
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
			cellProps: {
				field: 'image',
				type: 'code',
				showTitle: true
			}
		},
		{ 
			key: 'provider', 
			title: 'Provider',
			// Auto-size to content (widest cell in this column)
			cellComponent: GenericCell,
			cellProps: { field: 'provider_name' }
		},
		{ 
			key: 'flavor', 
			title: 'Flavor',
			// Auto-size to content (widest cell in this column)
			cellComponent: GenericCell,
			cellProps: { field: 'flavor' }
		},
		{ 
			key: 'entity', 
			title: 'Entity',
			// Auto-size to content (widest cell in this column)
			cellComponent: PoolEntityCell
		},
		{ 
			key: 'endpoint', 
			title: 'Endpoint',
			// Auto-size to content (widest cell in this column)
			cellComponent: EndpointCell
		},
		{ 
			key: 'status', 
			title: 'Status',
			// Auto-size to content (widest cell in this column)
			cellComponent: StatusCell,
			cellProps: { statusType: 'enabled' }
		},
		{ 
			key: 'actions', 
			title: 'Actions',
			// Auto-size to content (widest cell in this column)
			align: 'right' as const,
			cellComponent: ActionsCell
		}
	];

	// Mobile card configuration
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
		],
		actions: [
			{
				type: 'edit' as const,
				handler: (item: any) => openUpdateModal(item)
			},
			{
				type: 'delete' as const,
				handler: (item: any) => openDeleteModal(item)
			}
		]
	};

	function handleTableSearch(event: CustomEvent<{ term: string }>) {
		searchTerm = event.detail.term;
		currentPage = 1;
	}

	function handleTablePageChange(event: CustomEvent<{ page: number }>) {
		currentPage = event.detail.page;
	}

	function handleTablePerPageChange(event: CustomEvent<{ perPage: number }>) {
		perPage = event.detail.perPage;
		currentPage = 1;
	}

	function handleEdit(event: CustomEvent<{ item: any }>) {
		openUpdateModal(event.detail.item);
	}

	function handleDelete(event: CustomEvent<{ item: any }>) {
		openDeleteModal(event.detail.item);
	}

	// Pools are now handled by eager cache with websocket subscriptions
</script>

<svelte:head>
	<title>Pools - GARM</title>
</svelte:head>

<div class="space-y-6">
	<!-- Header -->
	<PageHeader
		title="Pools"
		description="Manage runner pools across all entities"
		actionLabel="Add Pool"
		on:action={openCreateModal}
	/>

	<DataTable
		{columns}
		data={paginatedPools}
		{loading}
		error={cacheError || error}
		{searchTerm}
		searchPlaceholder="Search by entity name..."
		{currentPage}
		{perPage}
		{totalPages}
		totalItems={filteredPools.length}
		itemName="pools"
		emptyIconType="cog"
		showRetry={!!cacheError}
		{mobileCardConfig}
		on:search={handleTableSearch}
		on:pageChange={handleTablePageChange}
		on:perPageChange={handleTablePerPageChange}
		on:retry={retryLoadPools}
		on:edit={handleEdit}
		on:delete={handleDelete}
	/>
</div>

<!-- Modals -->
{#if showCreateModal}
	<CreatePoolModal
		on:close={() => showCreateModal = false}
		on:submit={handleCreatePool}
	/>
{/if}

{#if showUpdateModal && selectedPool}
	<UpdatePoolModal
		pool={selectedPool}
		on:close={() => { showUpdateModal = false; selectedPool = null; }}
		on:submit={(e) => handleUpdatePool(e.detail)}
	/>
{/if}

{#if showDeleteModal && selectedPool}
	<DeleteModal
		title="Delete Pool"
		message="Are you sure you want to delete this pool? This action cannot be undone and will remove all associated runners."
		itemName={`Pool ${selectedPool.id!.slice(0, 8)}... (${getEntityName(selectedPool, $eagerCache)})`}
		on:close={() => { showDeleteModal = false; selectedPool = null; }}
		on:confirm={handleDeletePool}
	/>
{/if}