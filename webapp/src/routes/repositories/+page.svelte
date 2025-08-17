<script lang="ts">
	import { onMount } from 'svelte';
	import { garmApi } from '$lib/api/client.js';
	import type { Repository, CreateRepoParams, UpdateEntityParams } from '$lib/api/generated/api.js';
	import CreateRepositoryModal from '$lib/components/CreateRepositoryModal.svelte';
	import UpdateEntityModal from '$lib/components/UpdateEntityModal.svelte';
	import DeleteModal from '$lib/components/DeleteModal.svelte';
	import PageHeader from '$lib/components/PageHeader.svelte';
	import { eagerCache, eagerCacheManager } from '$lib/stores/eager-cache.js';
	import { toastStore } from '$lib/stores/toast.js';
	import { getForgeIcon, changePerPage, getEntityStatusBadge, filterRepositories, paginateItems } from '$lib/utils/common.js';
	import { extractAPIError } from '$lib/utils/apiError';
	import DataTable from '$lib/components/DataTable.svelte';
	import { EntityCell, EndpointCell, StatusCell, ActionsCell, GenericCell } from '$lib/components/cells';

	let repositories: Repository[] = [];
	let loading = true;
	let error = '';
	let searchTerm = '';

	// Modal states
	let showCreateModal = false;
	let showEditModal = false;
	let showDeleteModal = false;
	let editingRepository: Repository | null = null;
	let deletingRepository: Repository | null = null;

	// Subscribe to eager cache for repositories (when WebSocket is connected)
	$: {
		// Only use cache data if we're not in direct API mode
		if (!repositories.length || $eagerCache.loaded.repositories) {
			repositories = $eagerCache.repositories;
		}
	}
	$: loading = $eagerCache.loading.repositories;
	$: cacheError = $eagerCache.errorMessages.repositories;

	// Pagination
	let currentPage = 1;
	let perPage = 25;
	let totalPages = 1;

	$: filteredRepositories = filterRepositories(repositories, searchTerm);

	$: {
		totalPages = Math.ceil(filteredRepositories.length / perPage);
		if (currentPage > totalPages && totalPages > 0) {
			currentPage = totalPages;
		}
	}

	$: paginatedRepositories = paginateItems(filteredRepositories, currentPage, perPage);

	// Repository events are now handled by eager cache

	onMount(async () => {
		// Load repositories through eager cache (priority load + background load others)
		try {
			loading = true;
			const repoData = await eagerCacheManager.getRepositories();
			// If WebSocket is disconnected, getRepositories returns direct API data
			// Update our local repositories array with this data
			if (repoData && Array.isArray(repoData)) {
				repositories = repoData;
			}
		} catch (err) {
			// Cache error is already handled by the eager cache system
			// We don't need to set error here anymore since it's in the cache state
			console.error('Failed to load repositories:', err);
			error = err instanceof Error ? err.message : 'Failed to load repositories';
		} finally {
			loading = false;
		}
	});

	async function retryLoadRepositories() {
		try {
			await eagerCacheManager.retryResource('repositories');
		} catch (err) {
			console.error('Retry failed:', err);
		}
	}

	// Repositories are now loaded through eager cache

	function showEditRepositoryModal(repository: Repository) {
		editingRepository = repository;
		showEditModal = true;
	}

	function showDeleteRepositoryModal(repository: Repository) {
		deletingRepository = repository;
		showDeleteModal = true;
	}

	function closeModals() {
		showCreateModal = false;
		showEditModal = false;
		showDeleteModal = false;
		editingRepository = null;
		deletingRepository = null;
		error = '';
	}

	async function handleCreateRepository(event: CustomEvent<CreateRepoParams & { install_webhook?: boolean; auto_generate_secret?: boolean }>) {
		try {
			error = '';
			
			const data = event.detail;
			const repoParams: CreateRepoParams = {
				name: data.name,
				owner: data.owner,
				credentials_name: data.credentials_name,
				webhook_secret: data.webhook_secret
			};

			const createdRepo = await garmApi.createRepository(repoParams);
			
			// If install_webhook is checked, install the webhook
			if (data.install_webhook && createdRepo.id) {
				try {
					await garmApi.installRepoWebhook(createdRepo.id);
					toastStore.success(
						'Webhook Installed',
						`Webhook for repository ${createdRepo.owner}/${createdRepo.name} has been installed successfully.`
					);
				} catch (webhookError) {
					console.warn('Repository created but webhook installation failed:', webhookError);
					toastStore.error(
						'Webhook Installation Failed',
						webhookError instanceof Error ? webhookError.message : 'Failed to install webhook. You can try installing it manually from the repository details page.'
					);
				}
			}

			// No need to reload - websocket will handle the update
			showCreateModal = false;
			toastStore.success(
				'Repository Created',
				`Repository ${createdRepo.owner}/${createdRepo.name} has been created successfully.`
			);
		} catch (err) {
			error = extractAPIError(err);
			throw err; // Let the modal handle the error display
		}
	}

	async function handleUpdateRepository(params: UpdateEntityParams) {
		if (!editingRepository) return;
		
		try {
			await garmApi.updateRepository(editingRepository.id!, params);
			// No need to reload - websocket will handle the update
			toastStore.success(
				'Repository Updated',
				`Repository ${editingRepository.owner}/${editingRepository.name} has been updated successfully.`
			);
			closeModals();
		} catch (err) {
			throw err; // Let the modal handle the error
		}
	}

	async function handleDeleteRepository() {
		if (!deletingRepository) return;
		
		try {
			error = '';
			await garmApi.deleteRepository(deletingRepository.id!);
			// No need to reload - websocket will handle the update
			toastStore.success(
				'Repository Deleted',
				`Repository ${deletingRepository.owner}/${deletingRepository.name} has been deleted successfully.`
			);
		} catch (err) {
			const errorMessage = extractAPIError(err);
			toastStore.error('Delete Failed', errorMessage);
		} finally {
			closeModals();
		}
	}

	// DataTable configuration
	const columns = [
		{ 
			key: 'repository', 
			title: 'Repository', 
			cellComponent: EntityCell,
			cellProps: { entityType: 'repository', showOwner: true }
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
		entityType: 'repository' as const,
		primaryText: {
			field: 'name',
			isClickable: true,
			href: '/repositories/{id}',
			showOwner: true
		},
		customInfo: [
			{ 
				icon: (item: any) => getForgeIcon(item?.endpoint?.endpoint_type || 'unknown'),
				text: (item: any) => item?.endpoint?.name || 'Unknown'
			}
		],
		badges: [
			{ 
				type: 'custom' as const, 
				value: (item: any) => getEntityStatusBadge(item)
			}
		],
		actions: [
			{ 
				type: 'edit' as const, 
				handler: (item: any) => showEditRepositoryModal(item) 
			},
			{ 
				type: 'delete' as const, 
				handler: (item: any) => showDeleteRepositoryModal(item)
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
		const result = changePerPage(event.detail.perPage);
		perPage = result.newPerPage;
		currentPage = result.newCurrentPage;
	}

	function handleEdit(event: CustomEvent<{ item: any }>) {
		showEditRepositoryModal(event.detail.item);
	}

	function handleDelete(event: CustomEvent<{ item: any }>) {
		showDeleteRepositoryModal(event.detail.item);
	}
</script>

<svelte:head>
	<title>Repositories - GARM</title>
</svelte:head>

<div class="space-y-6">
	<!-- Header -->
	<PageHeader
		title="Repositories"
		description="Manage your GitHub repositories and their runners"
		actionLabel="Add Repository"
		on:action={() => { showCreateModal = true; }}
	/>

	<DataTable
		{columns}
		data={paginatedRepositories}
		{loading}
		error={cacheError || error}
		{searchTerm}
		searchPlaceholder="Search repositories by name or owner..."
		{currentPage}
		{perPage}
		{totalPages}
		totalItems={filteredRepositories.length}
		itemName="repositories"
		emptyIconType="building"
		showRetry={!!cacheError}
		{mobileCardConfig}
		on:search={handleTableSearch}
		on:pageChange={handleTablePageChange}
		on:perPageChange={handleTablePerPageChange}
		on:retry={retryLoadRepositories}
		on:edit={handleEdit}
		on:delete={handleDelete}
	/>
</div>

<!-- Create Repository Modal -->
{#if showCreateModal}
	<CreateRepositoryModal
		on:close={() => showCreateModal = false}
		on:submit={handleCreateRepository}
	/>
{/if}

{#if showEditModal && editingRepository}
	<UpdateEntityModal
		entity={editingRepository}
		entityType="repository"
		on:close={closeModals}
		on:submit={(e) => handleUpdateRepository(e.detail)}
	/>
{/if}

{#if showDeleteModal && deletingRepository}
	<DeleteModal
		title="Delete Repository"
		message="Are you sure you want to delete this repository? This action cannot be undone and will remove all associated pools and runners."
		itemName="{deletingRepository.owner}/{deletingRepository.name}"
		on:close={closeModals}
		on:confirm={handleDeleteRepository}
	/>
{/if}
