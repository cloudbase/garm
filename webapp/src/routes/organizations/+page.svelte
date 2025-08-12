<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { garmApi } from '$lib/api/client.js';
	import type { Organization, CreateOrgParams, UpdateEntityParams } from '$lib/api/generated/api.js';
	import { base } from '$app/paths';
	import CreateOrganizationModal from '$lib/components/CreateOrganizationModal.svelte';
	import UpdateEntityModal from '$lib/components/UpdateEntityModal.svelte';
	import DeleteModal from '$lib/components/DeleteModal.svelte';
	import PageHeader from '$lib/components/PageHeader.svelte';
	import { eagerCache, eagerCacheManager } from '$lib/stores/eager-cache.js';
	import { toastStore } from '$lib/stores/toast.js';
	import { getForgeIcon, getEntityStatusBadge, filterByName } from '$lib/utils/common.js';
	import Badge from '$lib/components/Badge.svelte';
import DataTable from '$lib/components/DataTable.svelte';
import ActionButton from '$lib/components/ActionButton.svelte';
import { EntityCell, EndpointCell, StatusCell, ActionsCell, GenericCell } from '$lib/components/cells';

	let organizations: Organization[] = [];
	let loading = true;
	let error = '';

	// Subscribe to eager cache for organizations (when WebSocket is connected)
	$: {
		// Only use cache data if we're not in direct API mode
		if (!organizations.length || $eagerCache.loaded.organizations) {
			organizations = $eagerCache.organizations;
		}
	}
	$: loading = $eagerCache.loading.organizations;
	$: cacheError = $eagerCache.errorMessages.organizations;
	let searchTerm = '';
	let currentPage = 1;
	let perPage = 25;
	let showCreateModal = false;
	let showUpdateModal = false;
	let showDeleteModal = false;
	let selectedOrganization: Organization | null = null;
	// Filtered and paginated data
	$: filteredOrganizations = filterByName(organizations, searchTerm);
	$: totalPages = Math.ceil(filteredOrganizations.length / perPage);
	$: {
		if (currentPage > totalPages && totalPages > 0) {
			currentPage = totalPages;
		}
	}
	$: paginatedOrganizations = filteredOrganizations.slice(
		(currentPage - 1) * perPage,
		currentPage * perPage
	);


	async function handleCreateOrganization(event: CustomEvent<CreateOrgParams & { install_webhook?: boolean; auto_generate_secret?: boolean }>) {
		try {
			error = '';
			
			const data = event.detail;
			const orgParams: CreateOrgParams = {
				name: data.name,
				credentials_name: data.credentials_name,
				webhook_secret: data.webhook_secret,
				pool_balancer_type: data.pool_balancer_type
			};
			
			const createdOrg = await garmApi.createOrganization(orgParams);
			
			// If install_webhook is checked, install the webhook
			if (data.install_webhook && createdOrg.id) {
				try {
					await garmApi.installOrganizationWebhook(createdOrg.id);
					toastStore.success(
						'Webhook Installed',
						`Webhook for organization ${createdOrg.name} has been installed successfully.`
					);
				} catch (webhookError) {
					console.warn('Organization created but webhook installation failed:', webhookError);
					toastStore.error(
						'Webhook Installation Failed',
						webhookError instanceof Error ? webhookError.message : 'Failed to install webhook. You can try installing it manually from the organization details page.'
					);
				}
			}
			
			// No need to reload - eager cache websocket will handle the update
			toastStore.success(
				'Organization Created',
				`Organization ${createdOrg.name} has been created successfully.`
			);
			showCreateModal = false;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create organization';
			throw err; // Let the modal handle the error
		}
	}

	async function handleUpdateOrganization(params: UpdateEntityParams) {
		if (!selectedOrganization) return;
		try {
			await garmApi.updateOrganization(selectedOrganization.id!, params);
			// No need to reload - eager cache websocket will handle the update
			toastStore.success(
				'Organization Updated',
				`Organization ${selectedOrganization.name} has been updated successfully.`
			);
			showUpdateModal = false;
			selectedOrganization = null;
		} catch (err) {
			throw err; // Let the modal handle the error
		}
	}

	async function handleDeleteOrganization() {
		if (!selectedOrganization) return;
		try {
			error = '';
			await garmApi.deleteOrganization(selectedOrganization.id!);
			// No need to reload - eager cache websocket will handle the update
			toastStore.success(
				'Organization Deleted',
				`Organization ${selectedOrganization.name} has been deleted successfully.`
			);
			showDeleteModal = false;
			selectedOrganization = null;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to delete organization';
		}
	}

	function openCreateModal() {
		showCreateModal = true;
	}

	function openUpdateModal(organization: Organization) {
		selectedOrganization = organization;
		showUpdateModal = true;
	}

	function openDeleteModal(organization: Organization) {
		selectedOrganization = organization;
		showDeleteModal = true;
	}



	onMount(async () => {
		// Load organizations through eager cache (priority load + background load others)
		try {
			loading = true;
			const orgData = await eagerCacheManager.getOrganizations();
			// If WebSocket is disconnected, getOrganizations returns direct API data
			// Update our local organizations array with this data
			if (orgData && Array.isArray(orgData)) {
				organizations = orgData;
			}
		} catch (err) {
			// Cache error is already handled by the eager cache system
			// We don't need to set error here anymore since it's in the cache state
			console.error('Failed to load organizations:', err);
			error = err instanceof Error ? err.message : 'Failed to load organizations';
		} finally {
			loading = false;
		}
	});

	async function retryLoadOrganizations() {
		try {
			await eagerCacheManager.retryResource('organizations');
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
			cellProps: { entityType: 'organization' }
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
		entityType: 'organization' as const,
		primaryText: {
			field: 'name',
			isClickable: true,
			href: '/organizations/{id}'
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

	// Organizations are now handled by eager cache with websocket subscriptions
</script>

<svelte:head>
	<title>Organizations - GARM</title>
</svelte:head>

<div class="space-y-6">
	<!-- Header -->
	<PageHeader
		title="Organizations"
		description="Manage GitHub and Gitea organizations"
		actionLabel="Add Organization"
		on:action={openCreateModal}
	/>

	<DataTable
		{columns}
		data={paginatedOrganizations}
		{loading}
		error={cacheError || error}
		{searchTerm}
		searchPlaceholder="Search organizations..."
		{currentPage}
		{perPage}
		{totalPages}
		totalItems={filteredOrganizations.length}
		itemName="organizations"
		emptyIconType="building"
		showRetry={!!cacheError}
		{mobileCardConfig}
		on:search={handleTableSearch}
		on:pageChange={handleTablePageChange}
		on:perPageChange={handleTablePerPageChange}
		on:retry={retryLoadOrganizations}
		on:edit={handleEdit}
		on:delete={handleDelete}
	>
		<!-- Mobile card content -->
		<svelte:fragment slot="mobile-card" let:item={organization} let:index>
			{@const status = getEntityStatusBadge(organization)}
			<div class="flex items-center justify-between">
				<div class="flex-1 min-w-0">
					<a href={`${base}/organizations/${organization.id}`} class="block">
						<p class="text-sm font-medium text-blue-600 dark:text-blue-400 hover:text-blue-500 dark:hover:text-blue-300 truncate">
							{organization.name}
						</p>
						<div class="flex items-center mt-1 space-x-2">
							<div class="flex items-center text-xs text-gray-500 dark:text-gray-400">
								{@html getForgeIcon(organization.endpoint?.endpoint_type || 'unknown')}
								<span class="ml-1">{organization.endpoint?.name || 'Unknown'}</span>
							</div>
						</div>
					</a>
				</div>
				<div class="flex items-center space-x-3 ml-4">
					<Badge variant={status.variant} text={status.text} />
					<div class="flex space-x-2">
						<ActionButton
							action="edit"
							size="sm"
							title="Edit organization"
							ariaLabel="Edit organization"
							on:click={() => openUpdateModal(organization)}
						/>
						<ActionButton
							action="delete"
							size="sm"
							title="Delete organization"
							ariaLabel="Delete organization"
							on:click={() => openDeleteModal(organization)}
						/>
					</div>
				</div>
			</div>
		</svelte:fragment>

	</DataTable>
</div>

<!-- Modals -->
{#if showCreateModal}
	<CreateOrganizationModal
		on:close={() => showCreateModal = false}
		on:submit={handleCreateOrganization}
	/>
{/if}

{#if showUpdateModal && selectedOrganization}
	<UpdateEntityModal
		entity={selectedOrganization}
		entityType="organization"
		on:close={() => { showUpdateModal = false; selectedOrganization = null; }}
		on:submit={(e) => handleUpdateOrganization(e.detail)}
	/>
{/if}

{#if showDeleteModal && selectedOrganization}
	<DeleteModal
		title="Delete Organization"
		message="Are you sure you want to delete this organization? This action cannot be undone."
		itemName={selectedOrganization.name}
		on:close={() => { showDeleteModal = false; selectedOrganization = null; }}
		on:confirm={handleDeleteOrganization}
	/>
{/if}