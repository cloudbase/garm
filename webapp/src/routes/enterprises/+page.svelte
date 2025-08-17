<script lang="ts">
	import { onMount } from 'svelte';
	import { garmApi } from '$lib/api/client.js';
	import type { Enterprise, CreateEnterpriseParams, UpdateEntityParams } from '$lib/api/generated/api.js';
	import { base } from '$app/paths';
	import PageHeader from '$lib/components/PageHeader.svelte';
	import CreateEnterpriseModal from '$lib/components/CreateEnterpriseModal.svelte';
	import UpdateEntityModal from '$lib/components/UpdateEntityModal.svelte';
	import DeleteModal from '$lib/components/DeleteModal.svelte';
	import { eagerCache, eagerCacheManager } from '$lib/stores/eager-cache.js';
	import { toastStore } from '$lib/stores/toast.js';
	import { getForgeIcon, getEntityStatusBadge, filterByName } from '$lib/utils/common.js';
	import { extractAPIError } from '$lib/utils/apiError';
	import Badge from '$lib/components/Badge.svelte';
import DataTable from '$lib/components/DataTable.svelte';
import ActionButton from '$lib/components/ActionButton.svelte';
import { EntityCell, EndpointCell, StatusCell, ActionsCell, GenericCell } from '$lib/components/cells';

	let enterprises: Enterprise[] = [];
	let loading = true;
	let error = '';

	// Subscribe to eager cache for enterprises (when WebSocket is connected)
	$: {
		// Only use cache data if we're not in direct API mode
		if (!enterprises.length || $eagerCache.loaded.enterprises) {
			enterprises = $eagerCache.enterprises;
		}
	}
	$: loading = $eagerCache.loading.enterprises;
	$: cacheError = $eagerCache.errorMessages.enterprises;
	let searchTerm = '';
	let currentPage = 1;
	let perPage = 25;
	let showCreateModal = false;
	let showUpdateModal = false;
	let showDeleteModal = false;
	let selectedEnterprise: Enterprise | null = null;
	// Filtered and paginated data
	$: filteredEnterprises = filterByName(enterprises, searchTerm);
	$: totalPages = Math.ceil(filteredEnterprises.length / perPage);
	$: {
		if (currentPage > totalPages && totalPages > 0) {
			currentPage = totalPages;
		}
	}
	$: paginatedEnterprises = filteredEnterprises.slice(
		(currentPage - 1) * perPage,
		currentPage * perPage
	);

	async function handleCreateEnterprise(params: CreateEnterpriseParams) {
		try {
			error = '';
			await garmApi.createEnterprise(params);
			// No need to reload - eager cache websocket will handle the update
			toastStore.success(
				'Enterprise Created',
				`Enterprise ${params.name} has been created successfully.`
			);
			showCreateModal = false;
		} catch (err) {
			error = extractAPIError(err);
			throw err; // Let the modal handle the error
		}
	}

	async function handleUpdateEnterprise(params: UpdateEntityParams) {
		if (!selectedEnterprise) return;
		try {
			await garmApi.updateEnterprise(selectedEnterprise.id!, params);
			// No need to reload - eager cache websocket will handle the update
			toastStore.success(
				'Enterprise Updated',
				`Enterprise ${selectedEnterprise.name} has been updated successfully.`
			);
			showUpdateModal = false;
			selectedEnterprise = null;
		} catch (err) {
			throw err; // Let the modal handle the error
		}
	}

	async function handleDeleteEnterprise() {
		if (!selectedEnterprise) return;
		try {
			error = '';
			await garmApi.deleteEnterprise(selectedEnterprise.id!);
			// No need to reload - eager cache websocket will handle the update
			toastStore.success(
				'Enterprise Deleted',
				`Enterprise ${selectedEnterprise.name} has been deleted successfully.`
			);
			showDeleteModal = false;
			selectedEnterprise = null;
		} catch (err) {
			const errorMessage = extractAPIError(err);
			toastStore.error('Delete Failed', errorMessage);
		}
	}

	function openCreateModal() {
		showCreateModal = true;
	}

	function openUpdateModal(enterprise: Enterprise) {
		selectedEnterprise = enterprise;
		showUpdateModal = true;
	}

	function openDeleteModal(enterprise: Enterprise) {
		selectedEnterprise = enterprise;
		showDeleteModal = true;
	}


	onMount(async () => {
		// Load enterprises through eager cache (priority load + background load others)
		try {
			loading = true;
			const entData = await eagerCacheManager.getEnterprises();
			// If WebSocket is disconnected, getEnterprises returns direct API data
			// Update our local enterprises array with this data
			if (entData && Array.isArray(entData)) {
				enterprises = entData;
			}
		} catch (err) {
			// Cache error is already handled by the eager cache system
			// We don't need to set error here anymore since it's in the cache state
			console.error('Failed to load enterprises:', err);
			error = err instanceof Error ? err.message : 'Failed to load enterprises';
		} finally {
			loading = false;
		}
	});

	async function retryLoadEnterprises() {
		try {
			await eagerCacheManager.retryResource('enterprises');
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
			cellProps: { entityType: 'enterprise' }
		},
		{ 
			key: 'endpoint', 
			title: 'Endpoint',
			cellComponent: EndpointCell
		},
		{ 
			key: 'credentials', 
			title: 'Credentials',
			cellComponent: GenericCell,
			cellProps: { field: 'credentials_name' }
		},
		{ 
			key: 'status', 
			title: 'Status',
			cellComponent: StatusCell,
			cellProps: { statusType: 'entity' }
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
		entityType: 'enterprise' as const,
		primaryText: {
			field: 'name',
			isClickable: true,
			href: '/enterprises/{id}'
		},
		secondaryText: {
			field: 'credentials_name'
		},
		badges: [
			{ 
				type: 'custom' as const, 
				value: (item: any) => getEntityStatusBadge(item)
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

	// Enterprises are now handled by eager cache with websocket subscriptions
</script>

<svelte:head>
	<title>Enterprises - GARM</title>
</svelte:head>

<div class="space-y-6">
	<!-- Header -->
	<PageHeader
		title="Enterprises"
		description="Manage GitHub enterprises"
		actionLabel="Add Enterprise"
		on:action={openCreateModal}
	/>

	<DataTable
		{columns}
		data={paginatedEnterprises}
		{loading}
		error={cacheError || error}
		{searchTerm}
		searchPlaceholder="Search enterprises..."
		{currentPage}
		{perPage}
		{totalPages}
		totalItems={filteredEnterprises.length}
		itemName="enterprises"
		emptyIconType="building"
		showRetry={!!cacheError}
		{mobileCardConfig}
		on:search={handleTableSearch}
		on:pageChange={handleTablePageChange}
		on:perPageChange={handleTablePerPageChange}
		on:retry={retryLoadEnterprises}
		on:edit={handleEdit}
		on:delete={handleDelete}
	>
		<!-- Mobile card content -->
		<svelte:fragment slot="mobile-card" let:item={enterprise} let:index>
			{@const status = getEntityStatusBadge(enterprise)}
			<div class="flex items-center justify-between">
				<div class="flex-1 min-w-0">
					<a href={`${base}/enterprises/${enterprise.id}`} class="block">
						<p class="text-sm font-medium text-blue-600 dark:text-blue-400 hover:text-blue-500 dark:hover:text-blue-300 truncate">
							{enterprise.name}
						</p>
						<p class="text-sm text-gray-500 dark:text-gray-400 truncate mt-1">
							{enterprise.credentials_name}
						</p>
					</a>
				</div>
				<div class="flex items-center space-x-3 ml-4">
					<Badge variant={status.variant} text={status.text} />
					<div class="flex space-x-2">
						<ActionButton
							action="edit"
							size="sm"
							title="Edit enterprise"
							ariaLabel="Edit enterprise"
							on:click={() => openUpdateModal(enterprise)}
						/>
						<ActionButton
							action="delete"
							size="sm"
							title="Delete enterprise"
							ariaLabel="Delete enterprise"
							on:click={() => openDeleteModal(enterprise)}
						/>
					</div>
				</div>
			</div>
		</svelte:fragment>

	</DataTable>
</div>

<!-- Modals -->
{#if showCreateModal}
	<CreateEnterpriseModal
		on:close={() => showCreateModal = false}
		on:submit={(e) => handleCreateEnterprise(e.detail)}
	/>
{/if}

{#if showUpdateModal && selectedEnterprise}
	<UpdateEntityModal
		entity={selectedEnterprise}
		entityType="enterprise"
		on:close={() => { showUpdateModal = false; selectedEnterprise = null; }}
		on:submit={(e) => handleUpdateEnterprise(e.detail)}
	/>
{/if}

{#if showDeleteModal && selectedEnterprise}
	<DeleteModal
		title="Delete Enterprise"
		message="Are you sure you want to delete this enterprise? This action cannot be undone."
		itemName={selectedEnterprise.name}
		on:close={() => { showDeleteModal = false; selectedEnterprise = null; }}
		on:confirm={handleDeleteEnterprise}
	/>
{/if}