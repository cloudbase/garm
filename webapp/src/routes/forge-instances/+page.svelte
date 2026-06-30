<script lang="ts">
	import { onMount } from 'svelte';
	import { garmApi } from '$lib/api/client.js';
	import type { ForgeInstance } from '$lib/api/generated/api.js';
	import PageHeader from '$lib/components/PageHeader.svelte';
	import DeleteModal from '$lib/components/DeleteModal.svelte';
	import UpdateEntityModal from '$lib/components/UpdateEntityModal.svelte';
	import CreateForgeInstanceModal from '$lib/components/CreateForgeInstanceModal.svelte';
	import type { CreateForgeInstanceParams } from '$lib/api/generated/api.js';
	import { eagerCache, eagerCacheManager } from '$lib/stores/eager-cache.js';
	import { toastStore } from '$lib/stores/toast.js';
	import { filterEntities } from '$lib/utils/common.js';
	import { extractAPIError } from '$lib/utils/apiError';
	import DataTable from '$lib/components/DataTable.svelte';
	import ActionButton from '$lib/components/ActionButton.svelte';
	import { ActionsCell, GenericCell, EntityCell, EndpointCell, StatusCell } from '$lib/components/cells';

	let forgeInstances: ForgeInstance[] = [];
	let loading = true;
	let error = '';

	// Subscribe to eager cache for forge instances (when WebSocket is connected)
	$: {
		if (!forgeInstances.length || $eagerCache.loaded.forgeInstances) {
			forgeInstances = $eagerCache.forgeInstances;
		}
	}
	$: loading = $eagerCache.loading.forgeInstances;
	$: cacheError = $eagerCache.errorMessages.forgeInstances;
	let searchTerm = '';
	let currentPage = 1;
	let perPage = 25;
	let showCreateModal = false;
	let showDeleteModal = false;
	let showUpdateModal = false;
	let selectedForgeInstance: ForgeInstance | null = null;

	// Filtered and paginated data
	$: filteredForgeInstances = filterEntities(forgeInstances, searchTerm, (fi: ForgeInstance) => {
		return [fi.endpoint?.name, fi.credentials_name, fi.id].filter(Boolean).join(' ');
	});
	$: totalPages = Math.ceil(filteredForgeInstances.length / perPage);
	$: {
		if (currentPage > totalPages && totalPages > 0) {
			currentPage = totalPages;
		}
	}
	$: paginatedForgeInstances = filteredForgeInstances.slice(
		(currentPage - 1) * perPage,
		currentPage * perPage
	);

	async function handleDeleteForgeInstance() {
		if (!selectedForgeInstance) return;
		try {
			error = '';
			await garmApi.deleteForgeInstance(selectedForgeInstance.id!);
			toastStore.success(
				'Forge Instance Deleted',
				`Forge instance ${selectedForgeInstance.credentials_name || selectedForgeInstance.id} has been deleted successfully.`
			);
			showDeleteModal = false;
			selectedForgeInstance = null;
		} catch (err) {
			const errorMessage = extractAPIError(err);
			toastStore.error('Delete Failed', errorMessage);
		}
	}

	function openCreateModal() {
		showCreateModal = true;
	}

	async function handleCreateForgeInstance(params: CreateForgeInstanceParams) {
		try {
			await garmApi.createForgeInstance(params);
			toastStore.success(
				'Forge Instance Created',
				`Forge instance has been created successfully.`
			);
			showCreateModal = false;
		} catch (err) {
			toastStore.error('Create Failed', extractAPIError(err));
		}
	}

	function openDeleteModal(forgeInstance: ForgeInstance) {
		selectedForgeInstance = forgeInstance;
		showDeleteModal = true;
	}

	function openUpdateModal(forgeInstance: ForgeInstance) {
		selectedForgeInstance = forgeInstance;
		showUpdateModal = true;
	}

	async function handleUpdateForgeInstance(params: any) {
		if (!selectedForgeInstance) return;
		try {
			await garmApi.updateForgeInstance(selectedForgeInstance.id!, params);
			toastStore.success(
				'Forge Instance Updated',
				`Forge instance has been updated successfully.`
			);
			showUpdateModal = false;
			selectedForgeInstance = null;
		} catch (err) {
			toastStore.error('Update Failed', extractAPIError(err));
		}
	}

	onMount(async () => {
		try {
			loading = true;
			const data = await eagerCacheManager.getForgeInstances();
			if (data && Array.isArray(data)) {
				forgeInstances = data;
			}
		} catch (err) {
			console.error('Failed to load forge instances:', err);
			error = err instanceof Error ? err.message : 'Failed to load forge instances';
		} finally {
			loading = false;
		}
	});

	async function retryLoadForgeInstances() {
		try {
			await eagerCacheManager.retryResource('forgeInstances');
		} catch (err) {
			console.error('Retry failed:', err);
		}
	}

	// DataTable configuration
	const columns = [
		{
			key: 'endpoint',
			title: 'Endpoint',
			cellComponent: EndpointCell,
			cellProps: { linkTo: 'forge_instance' }
		},
		{
			key: 'credentials_name',
			title: 'Credentials',
			cellComponent: GenericCell,
			cellProps: { field: 'credentials_name' }
		},
		{
			key: 'pool_balancing_type',
			title: 'Pool Balancing',
			cellComponent: GenericCell,
			cellProps: { field: 'pool_balancing_type' }
		},
		{
			key: 'status',
			title: 'Status',
			cellComponent: StatusCell,
			cellProps: { statusField: 'pool_manager_status' }
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
		entityType: 'forge_instance' as const,
		primaryText: {
			field: 'endpoint.name',
			isClickable: true,
			entityType: 'forge_instance' as const
		},
		secondaryText: {
			field: 'credentials_name'
		},
		badges: [
			{
				field: 'pool_manager_status.is_running',
				trueLabel: 'Running',
				falseLabel: 'Stopped',
				trueColor: 'green' as const,
				falseColor: 'red' as const
			}
		],
		actions: [
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
</script>

<svelte:head>
	<title>Forge Instances - GARM</title>
</svelte:head>

<div class="space-y-6">
	<!-- Header -->
	<PageHeader
		title="Forge Instances"
		description="Manage forge instances"
		actionLabel="Add Forge Instance"
		on:action={openCreateModal}
	/>

	<DataTable
		{columns}
		data={paginatedForgeInstances}
		{loading}
		error={cacheError || error}
		{searchTerm}
		searchPlaceholder="Search forge instances..."
		{currentPage}
		{perPage}
		{totalPages}
		totalItems={filteredForgeInstances.length}
		itemName="forge instances"
		emptyIconType="building"
		showRetry={!!cacheError}
		{mobileCardConfig}
		on:search={handleTableSearch}
		on:pageChange={handleTablePageChange}
		on:perPageChange={handleTablePerPageChange}
		on:retry={retryLoadForgeInstances}
		on:edit={handleEdit}
		on:delete={handleDelete}
	>
		<!-- Mobile card content -->
		<svelte:fragment slot="mobile-card" let:item={forgeInstance} let:index>
			<div class="flex items-center justify-between">
				<div class="flex-1 min-w-0">
					<p class="text-sm font-medium text-gray-900 dark:text-white truncate">
						{forgeInstance.credentials_name}
					</p>
					<p class="text-sm text-gray-500 dark:text-gray-400 truncate mt-1">
						{forgeInstance.id}
					</p>
				</div>
				<div class="flex items-center space-x-3 ml-4">
					<div class="flex space-x-2">
						<ActionButton
							action="delete"
							size="sm"
							title="Delete forge instance"
							ariaLabel="Delete forge instance"
							on:click={() => openDeleteModal(forgeInstance)}
						/>
					</div>
				</div>
			</div>
		</svelte:fragment>
	</DataTable>
</div>

<!-- Modals -->
{#if showCreateModal}
	<CreateForgeInstanceModal
		on:close={() => showCreateModal = false}
		on:submit={(e) => handleCreateForgeInstance(e.detail)}
	/>
{/if}

{#if showUpdateModal && selectedForgeInstance}
	<UpdateEntityModal
		entity={selectedForgeInstance}
		entityType="forge_instance"
		on:close={() => { showUpdateModal = false; selectedForgeInstance = null; }}
		on:submit={(e) => handleUpdateForgeInstance(e.detail)}
	/>
{/if}

{#if showDeleteModal && selectedForgeInstance}
	<DeleteModal
		title="Delete Forge Instance"
		message="Are you sure you want to delete this forge instance? This action cannot be undone."
		itemName={selectedForgeInstance.endpoint?.name || selectedForgeInstance.id}
		on:close={() => { showDeleteModal = false; selectedForgeInstance = null; }}
		on:confirm={handleDeleteForgeInstance}
	/>
{/if}
