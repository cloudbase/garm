<script lang="ts">
	import { onMount } from 'svelte';
	import { garmApi } from '$lib/api/client.js';
	import type { ScaleSet, CreateScaleSetParams } from '$lib/api/generated/api.js';
	import PageHeader from '$lib/components/PageHeader.svelte';
	import CreateScaleSetModal from '$lib/components/CreateScaleSetModal.svelte';
	import UpdateScaleSetModal from '$lib/components/UpdateScaleSetModal.svelte';
	import DeleteModal from '$lib/components/DeleteModal.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import LoadingState from '$lib/components/LoadingState.svelte';
	import { eagerCache, eagerCacheManager } from '$lib/stores/eager-cache.js';
	import { toastStore } from '$lib/stores/toast.js';
	import { getEntityName, filterEntities } from '$lib/utils/common.js';
	import { extractAPIError } from '$lib/utils/apiError';
	import DataTable from '$lib/components/DataTable.svelte';
	import { EntityCell, EndpointCell, StatusCell, ActionsCell, GenericCell, PoolEntityCell } from '$lib/components/cells';

	let scaleSets: ScaleSet[] = [];
	let loading = true;
	let error = '';

	// Subscribe to eager cache for scale sets (when WebSocket is connected)
	$: {
		// Only use cache data if we're not in direct API mode
		if (!scaleSets.length || $eagerCache.loaded.scalesets) {
			scaleSets = $eagerCache.scalesets;
		}
	}
	$: loading = $eagerCache.loading.scalesets;
	$: cacheError = $eagerCache.errorMessages.scalesets;
	let searchTerm = '';
	let currentPage = 1;
	let perPage = 25;
	let showCreateModal = false;
	let showUpdateModal = false;
	let showDeleteModal = false;
	let selectedScaleSet: ScaleSet | null = null;
	let loadingScaleSetDetails = false;
	// Filtered and paginated data
	$: filteredScaleSets = filterEntities(scaleSets, searchTerm, (scaleSet) => getEntityName(scaleSet));
	$: totalPages = Math.ceil(filteredScaleSets.length / perPage);
	$: {
		if (currentPage > totalPages && totalPages > 0) {
			currentPage = totalPages;
		}
	}
	$: paginatedScaleSets = filteredScaleSets.slice(
		(currentPage - 1) * perPage,
		currentPage * perPage
	);

	async function handleCreateScaleSet(params: CreateScaleSetParams) {
		try {
			error = '';
			// The actual creation will be handled by the modal based on entity type
			// No need to reload - eager cache websocket will handle the update
			showCreateModal = false;
			// Note: We don't have access to the created scale set data here
			toastStore.success(
				'Scale Set Created',
				'Scale set has been created successfully.'
			);
		} catch (err) {
			error = extractAPIError(err);
			throw err; // Let the modal handle the error
		}
	}

	async function handleUpdateScaleSet(params: Partial<CreateScaleSetParams>) {
		if (!selectedScaleSet) return;
		try {
			await garmApi.updateScaleSet(selectedScaleSet.id!, params);
			// No need to reload - eager cache websocket will handle the update
			toastStore.success(
				'Scale Set Updated',
				`Scale set ${selectedScaleSet.name} has been updated successfully.`
			);
			showUpdateModal = false;
			selectedScaleSet = null;
		} catch (err) {
			throw err; // Let the modal handle the error
		}
	}

	async function handleDeleteScaleSet() {
		if (!selectedScaleSet) return;
		try {
			await garmApi.deleteScaleSet(selectedScaleSet.id!);
			// No need to reload - websocket will handle the update
			toastStore.success(
				'Scale Set Deleted',
				`Scale set ${selectedScaleSet.name} has been deleted successfully.`
			);
		} catch (err) {
			const errorMessage = extractAPIError(err);
			toastStore.error('Delete Failed', errorMessage);
		} finally {
			showDeleteModal = false;
			selectedScaleSet = null;
		}
	}

	function openCreateModal() {
		showCreateModal = true;
	}

	async function openUpdateModal(scaleSet: ScaleSet) {
		try {
			loadingScaleSetDetails = true;
			// Fetch complete scale set data including extra_specs from API
			// The scale set list data omits extra_specs to reduce payload size
			const completeScaleSet = await garmApi.getScaleSet(scaleSet.id!);
			selectedScaleSet = completeScaleSet;
			showUpdateModal = true;
		} catch (err) {
			const errorMessage = extractAPIError(err);
			toastStore.error(
				'Failed to Load Scale Set Details',
				errorMessage
			);
		} finally {
			loadingScaleSetDetails = false;
		}
	}

	function openDeleteModal(scaleSet: ScaleSet) {
		selectedScaleSet = scaleSet;
		showDeleteModal = true;
	}


	onMount(async () => {
		// Load scale sets through eager cache (priority load + background load others)
		try {
			loading = true;
			const scaleSetData = await eagerCacheManager.getScaleSets();
			// If WebSocket is disconnected, getScaleSets returns direct API data
			// Update our local scaleSets array with this data
			if (scaleSetData && Array.isArray(scaleSetData)) {
				scaleSets = scaleSetData;
			}
		} catch (err) {
			// Cache error is already handled by the eager cache system
			// We don't need to set error here anymore since it's in the cache state
			if (!import.meta.env?.VITEST) {
				console.error('Failed to load scale sets:', err);
			}
			error = extractAPIError(err);
		} finally {
			loading = false;
		}
	});

	async function retryLoadScaleSets() {
		try {
			await eagerCacheManager.retryResource('scalesets');
		} catch (err) {
			console.error('Retry failed:', err);
		}
	}

	// DataTable configuration
	const columns = [
		{ 
			key: 'name', 
			title: 'Name',
			cellComponent: EntityCell,
			cellProps: { entityType: 'scaleset' }
		},
		{ 
			key: 'image', 
			title: 'Image',
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
			cellComponent: GenericCell,
			cellProps: { field: 'provider_name' }
		},
		{ 
			key: 'flavor', 
			title: 'Flavor',
			cellComponent: GenericCell,
			cellProps: { field: 'flavor' }
		},
		{ 
			key: 'entity', 
			title: 'Entity',
			cellComponent: PoolEntityCell
		},
		{ 
			key: 'endpoint', 
			title: 'Endpoint',
			cellComponent: EndpointCell
		},
		{ 
			key: 'status', 
			title: 'Status',
			cellComponent: StatusCell,
			cellProps: { statusType: 'enabled' }
		},
		{ 
			key: 'actions', 
			title: 'Actions', 
			align: 'right' as const,
			cellComponent: ActionsCell
		}
	];

	// Mobile card configuration
	const mobileCardConfig = {
		entityType: 'scaleset' as const,
		primaryText: {
			field: 'name',
			isClickable: true,
			href: '/scalesets/{id}'
		},
		secondaryText: {
			field: 'entity_name',
			computedValue: (item: any) => getEntityName(item)
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

	// Scale sets are now handled by eager cache with websocket subscriptions
</script>

<svelte:head>
	<title>Scale Sets - GARM</title>
</svelte:head>

<div class="space-y-6">
	<!-- Header -->
	<PageHeader
		title="Scale Sets"
		description="Manage GitHub runner scale sets"
		actionLabel="Add Scale Set"
		on:action={openCreateModal}
	/>

	<DataTable
		{columns}
		data={paginatedScaleSets}
		{loading}
		error={cacheError || error}
		{searchTerm}
		searchPlaceholder="Search by entity name..."
		{currentPage}
		{perPage}
		{totalPages}
		totalItems={filteredScaleSets.length}
		itemName="scale sets"
		emptyIconType="cog"
		showRetry={!!cacheError}
		{mobileCardConfig}
		on:search={handleTableSearch}
		on:pageChange={handleTablePageChange}
		on:perPageChange={handleTablePerPageChange}
		on:retry={retryLoadScaleSets}
		on:edit={handleEdit}
		on:delete={handleDelete}
	/>
</div>

<!-- Modals -->
{#if showCreateModal}
	<CreateScaleSetModal
		on:close={() => showCreateModal = false}
		on:submit={(e) => handleCreateScaleSet(e.detail)}
	/>
{/if}

{#if showUpdateModal && selectedScaleSet}
	<UpdateScaleSetModal
		scaleSet={selectedScaleSet}
		on:close={() => { showUpdateModal = false; selectedScaleSet = null; }}
		on:submit={(e) => handleUpdateScaleSet(e.detail)}
	/>
{/if}

{#if showDeleteModal && selectedScaleSet}
	<DeleteModal
		title="Delete Scale Set"
		message="Are you sure you want to delete this scale set? This action cannot be undone and will remove all associated runners."
		itemName={`Scale Set ${selectedScaleSet.name} (${getEntityName(selectedScaleSet)})`}
		on:close={() => { showDeleteModal = false; selectedScaleSet = null; }}
		on:confirm={handleDeleteScaleSet}
	/>
{/if}

<!-- Loading Modal for Scale Set Details -->
{#if loadingScaleSetDetails}
	<Modal on:close={() => {/* Prevent closing during load */}}>
		<div class="p-6">
			<LoadingState message="Loading scale set details..." />
		</div>
	</Modal>
{/if}