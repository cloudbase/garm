<svelte:window on:keydown={handleKeydown} />

<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { garmApi } from '$lib/api/client.js';
	import type { ForgeEndpoint } from '$lib/api/generated/api.js';
	import PageHeader from '$lib/components/PageHeader.svelte';
	import ForgeTypeSelector from '$lib/components/ForgeTypeSelector.svelte';
	import ActionButton from '$lib/components/ActionButton.svelte';
	import Button from '$lib/components/Button.svelte';
	import SearchBar from '$lib/components/SearchBar.svelte';
	import LoadingState from '$lib/components/LoadingState.svelte';
	import ErrorState from '$lib/components/ErrorState.svelte';
	import EmptyState from '$lib/components/EmptyState.svelte';
	import { eagerCache, eagerCacheManager } from '$lib/stores/eager-cache.js';
	import { toastStore } from '$lib/stores/toast.js';
	import { getForgeIcon, filterEndpoints, changePage, changePerPage, paginateItems } from '$lib/utils/common.js';
import DataTable from '$lib/components/DataTable.svelte';
import { EntityCell, EndpointCell, StatusCell, ActionsCell, GenericCell } from '$lib/components/cells';

	let loading = true;
	let endpoints: ForgeEndpoint[] = [];
	let error = '';
	let searchTerm = '';

	// Pagination
	let currentPage = 1;
	let perPage = 25;
	let totalPages = 1;

	// Subscribe to eager cache for endpoints (when WebSocket is connected)
	$: {
		// Only use cache data if we're not in direct API mode
		if (!endpoints.length || $eagerCache.loaded.endpoints) {
			endpoints = $eagerCache.endpoints;
		}
	}
	$: loading = $eagerCache.loading.endpoints;
	$: cacheError = $eagerCache.errorMessages.endpoints;

	$: filteredEndpoints = filterEndpoints(endpoints, searchTerm);

	$: {
		totalPages = Math.ceil(filteredEndpoints.length / perPage);
		if (currentPage > totalPages && totalPages > 0) {
			currentPage = totalPages;
		}
	}

	$: paginatedEndpoints = paginateItems(filteredEndpoints, currentPage, perPage);
	let showCreateModal = false;
	let showEditModal = false;
	let showDeleteModal = false;
	let selectedForgeType: 'github' | 'gitea' | '' = 'github';
	let editingEndpoint: ForgeEndpoint | null = null;
	let deletingEndpoint: ForgeEndpoint | null = null;
	// Form state
	let formData = {
		name: '',
		description: '',
		endpoint_type: '',
		base_url: '',
		api_base_url: '',
		upload_base_url: '',
		ca_cert_bundle: ''
	};
	// Track original values for comparison during updates
	let originalFormData: typeof formData = { ...formData };


	onMount(async () => {
		// Load endpoints through eager cache (priority load + background load others)
		try {
			loading = true;
			const endpointData = await eagerCacheManager.getEndpoints();
			// If WebSocket is disconnected, getEndpoints returns direct API data
			// Update our local endpoints array with this data
			if (endpointData && Array.isArray(endpointData)) {
				endpoints = endpointData;
			}
		} catch (err) {
			// Cache error is already handled by the eager cache system
			// We don't need to set error here anymore since it's in the cache state
			console.error('Failed to load endpoints:', err);
			error = err instanceof Error ? err.message : 'Failed to load endpoints';
		} finally {
			loading = false;
		}
	});

	async function retryLoadEndpoints() {
		try {
			await eagerCacheManager.retryResource('endpoints');
		} catch (err) {
			console.error('Retry failed:', err);
		}
	}

	// DataTable configuration
	const columns = [
		{ 
			key: 'name', 
			title: 'Name',
			cellComponent: GenericCell,
			cellProps: { field: 'name' }
		},
		{ 
			key: 'description', 
			title: 'Description',
			cellComponent: GenericCell,
			cellProps: { field: 'description' }
		},
		{ 
			key: 'api_url', 
			title: 'API URL',
			cellComponent: GenericCell,
			cellProps: { field: 'api_base_url', fallbackField: 'base_url' }
		},
		{ 
			key: 'forge_type', 
			title: 'Forge Type',
			cellComponent: EndpointCell
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
		entityType: 'endpoint' as const,
		primaryText: {
			field: 'name',
			isClickable: false
		},
		secondaryText: {
			field: 'description'
		},
		customInfo: [
			{ 
				icon: (item: any) => getForgeIcon(item?.endpoint_type || 'unknown'),
				text: (item: any) => item?.api_base_url || 'Unknown'
			}
		],
		actions: [
			{ 
				type: 'edit' as const, 
				handler: (item: any) => showEditEndpointModal(item) 
			},
			{ 
				type: 'delete' as const, 
				handler: (item: any) => showDeleteEndpointModal(item)
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
		showEditEndpointModal(event.detail.item);
	}

	function handleDelete(event: CustomEvent<{ item: any }>) {
		showDeleteEndpointModal(event.detail.item);
	}

	// Endpoints are now handled by eager cache with websocket subscriptions



	function showCreateEndpointModal() {
		selectedForgeType = 'github';
		resetForm();
		showCreateModal = true;
	}

	function handleForgeTypeSelect(event: CustomEvent<'github' | 'gitea'>) {
		selectedForgeType = event.detail;
		formData.endpoint_type = event.detail;
	}

	function showEditEndpointModal(endpoint: ForgeEndpoint) {
		editingEndpoint = endpoint;
		formData = {
			name: endpoint.name || '',
			description: endpoint.description || '',
			endpoint_type: endpoint.endpoint_type || '',
			base_url: endpoint.base_url || '',
			api_base_url: endpoint.api_base_url || '',
			upload_base_url: endpoint.upload_base_url || '',
			ca_cert_bundle: typeof endpoint.ca_cert_bundle === 'string' ? endpoint.ca_cert_bundle : ''
		};
		// Store original values for comparison
		originalFormData = { ...formData };
		showEditModal = true;
	}

	function showDeleteEndpointModal(endpoint: ForgeEndpoint) {
		deletingEndpoint = endpoint;
		showDeleteModal = true;
	}

	function resetForm() {
		formData = {
			name: '',
			description: '',
			endpoint_type: '',
			base_url: '',
			api_base_url: '',
			upload_base_url: '',
			ca_cert_bundle: ''
		};
		originalFormData = { ...formData };
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape' && (showCreateModal || showEditModal || showDeleteModal)) {
			closeModals();
		}
	}

	function closeModals() {
		showCreateModal = false;
		showEditModal = false;
		showDeleteModal = false;
		selectedForgeType = 'github';
		editingEndpoint = null;
		deletingEndpoint = null;
		resetForm();
	}

	function buildUpdateParams() {
		const updateParams: any = {};
		
		// Only include fields that have changed from original values
		if (formData.description !== originalFormData.description) {
			// Only set if not empty or if it was intentionally cleared
			if (formData.description.trim() !== '' || originalFormData.description !== '') {
				updateParams.description = formData.description.trim();
			}
		}
		
		if (formData.base_url !== originalFormData.base_url) {
			if (formData.base_url.trim() !== '') {
				updateParams.base_url = formData.base_url.trim();
			}
		}
		
		if (formData.api_base_url !== originalFormData.api_base_url) {
			// For Gitea, api_base_url is optional, so allow empty
			// For GitHub, it's required so only set if not empty
			if (formData.api_base_url.trim() !== '' || originalFormData.api_base_url !== '') {
				updateParams.api_base_url = formData.api_base_url.trim();
			}
		}
		
		// GitHub-only field
		if (editingEndpoint?.endpoint_type === 'github' && formData.upload_base_url !== originalFormData.upload_base_url) {
			if (formData.upload_base_url.trim() !== '' || originalFormData.upload_base_url !== '') {
				updateParams.upload_base_url = formData.upload_base_url.trim();
			}
		}
		
		if (formData.ca_cert_bundle !== originalFormData.ca_cert_bundle) {
			// CA cert can be cleared by setting to empty
			if (formData.ca_cert_bundle !== '') {
				// Convert base64 string to byte array for API
				try {
					const bytes = atob(formData.ca_cert_bundle);
					updateParams.ca_cert_bundle = Array.from(bytes, char => char.charCodeAt(0));
				} catch (e) {
					// If not valid base64, treat as empty
					if (originalFormData.ca_cert_bundle !== '') {
						updateParams.ca_cert_bundle = [];
					}
				}
			} else if (originalFormData.ca_cert_bundle !== '') {
				// User intentionally cleared the CA cert
				updateParams.ca_cert_bundle = [];
			}
		}
		
		return updateParams;
	}

	async function handleCreateEndpoint() {
		try {
			// Prepare the params with proper ca_cert_bundle conversion
			const endpointParams: any = {
				name: formData.name,
				description: formData.description,
				endpoint_type: formData.endpoint_type,
				base_url: formData.base_url,
				api_base_url: formData.api_base_url,
				upload_base_url: formData.upload_base_url
			};
			
			// Convert ca_cert_bundle from base64 string to byte array if provided
			if (formData.ca_cert_bundle && formData.ca_cert_bundle.trim() !== '') {
				try {
					const bytes = atob(formData.ca_cert_bundle);
					endpointParams.ca_cert_bundle = Array.from(bytes, char => char.charCodeAt(0));
				} catch (e) {
					// If not valid base64, omit the field entirely
					// endpointParams.ca_cert_bundle will remain undefined
				}
			}

			if (formData.endpoint_type === 'github') {
				await garmApi.createGithubEndpoint(endpointParams);
			} else {
				await garmApi.createGiteaEndpoint(endpointParams);
			}
			// No need to reload - websocket will handle the update
			toastStore.success(
				'Endpoint Created',
				`Endpoint ${formData.name} has been created successfully.`
			);
			closeModals();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create endpoint';
		}
	}

	async function handleUpdateEndpoint() {
		if (!editingEndpoint) return;
		
		try {
			const updateParams = buildUpdateParams();
			
			// Only proceed if there are changes to apply
			if (Object.keys(updateParams).length === 0) {
				toastStore.info(
					'No Changes',
					'No fields were modified.'
				);
				closeModals();
				return;
			}
			
			if (editingEndpoint.endpoint_type === 'github') {
				await garmApi.updateGithubEndpoint(editingEndpoint.name!, updateParams);
			} else {
				await garmApi.updateGiteaEndpoint(editingEndpoint.name!, updateParams);
			}
			// No need to reload - websocket will handle the update
			toastStore.success(
				'Endpoint Updated',
				`Endpoint ${editingEndpoint.name} has been updated successfully.`
			);
			closeModals();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to update endpoint';
		}
	}

	async function handleDeleteEndpoint() {
		if (!deletingEndpoint) return;
		
		try {
			if (deletingEndpoint.endpoint_type === 'github') {
				await garmApi.deleteGithubEndpoint(deletingEndpoint.name!);
			} else {
				await garmApi.deleteGiteaEndpoint(deletingEndpoint.name!);
			}
			// No need to reload - websocket will handle the update
			toastStore.success(
				'Endpoint Deleted',
				`Endpoint ${deletingEndpoint.name} has been deleted successfully.`
			);
			closeModals();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to delete endpoint';
		}
	}

	function handleFileUpload(event: Event) {
		const target = event.target as HTMLInputElement;
		const file = target.files?.[0];
		
		if (!file) {
			formData.ca_cert_bundle = '';
			return;
		}

		const reader = new FileReader();
		reader.onload = (e) => {
			const content = e.target?.result as string;
			formData.ca_cert_bundle = btoa(content);
		};
		reader.readAsText(file);
	}

	function isFormValid() {
		if (!formData.name || !formData.description || !formData.base_url) return false;
		if (formData.endpoint_type === 'github' && !formData.api_base_url) return false;
		return true;
	}
</script>

<svelte:head>
	<title>Endpoints - GARM</title>
</svelte:head>

<div class="space-y-6">
	<!-- Header -->
	<PageHeader
		title="Endpoints"
		description="Manage your GitHub and Gitea endpoints for runner management."
		actionLabel="Add Endpoint"
		on:action={showCreateEndpointModal}
	/>

	<DataTable
		{columns}
		data={paginatedEndpoints}
		{loading}
		error={cacheError || error}
		{searchTerm}
		searchPlaceholder="Search endpoints by name, description, or URL..."
		{currentPage}
		{perPage}
		{totalPages}
		totalItems={filteredEndpoints.length}
		itemName="endpoints"
		emptyIconType="settings"
		showRetry={!!cacheError}
		{mobileCardConfig}
		on:search={handleTableSearch}
		on:pageChange={handleTablePageChange}
		on:perPageChange={handleTablePerPageChange}
		on:retry={retryLoadEndpoints}
		on:edit={handleEdit}
		on:delete={handleDelete}
	>
		<!-- Mobile card content -->
		<svelte:fragment slot="mobile-card" let:item={endpoint} let:index>
			<div class="flex items-center justify-between">
				<div class="flex-1 min-w-0">
					<div class="block">
						<p class="text-sm font-medium text-gray-900 dark:text-white truncate">
							{endpoint.name}
						</p>
						<p class="text-sm text-gray-500 dark:text-gray-400 truncate mt-1">
							{endpoint.description}
						</p>
						<div class="flex items-center mt-2">
							{@html getForgeIcon(endpoint.endpoint_type || '', 'w-5 h-5')}
							<span class="ml-2 text-xs text-gray-500 dark:text-gray-400 capitalize">{endpoint.endpoint_type}</span>
						</div>
					</div>
				</div>
				<div class="flex space-x-2 ml-4">
					<ActionButton
						action="edit"
						size="sm"
						title="Edit endpoint"
						ariaLabel="Edit endpoint"
						on:click={() => showEditEndpointModal(endpoint)}
					/>
					<ActionButton
						action="delete"
						size="sm"
						title="Delete endpoint"
						ariaLabel="Delete endpoint"
						on:click={() => showDeleteEndpointModal(endpoint)}
					/>
				</div>
			</div>
		</svelte:fragment>

	</DataTable>
</div>

<!-- Create Modal -->
{#if showCreateModal}
	<div class="fixed inset-0 flex items-center justify-center z-50" role="dialog" aria-modal="true">
		<button class="fixed inset-0 w-full h-full bg-black/30 dark:bg-black/50 cursor-default" on:click={closeModals} aria-label="Close modal"></button>
		<div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-2xl w-full mx-4 max-h-screen overflow-y-auto relative z-10" role="document">
			<div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700 flex justify-between items-center">
				<div>
					<h3 class="text-lg font-semibold text-gray-900 dark:text-white">
						Add Endpoint
					</h3>
					<p class="text-sm text-gray-600 dark:text-gray-400 mt-1">
						Connect to GitHub or Gitea for runner management
					</p>
				</div>
				<button on:click={closeModals} class="text-gray-400 hover:text-gray-600 dark:text-gray-300 dark:hover:text-gray-100 cursor-pointer" aria-label="Close modal">
					<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
					</svg>
				</button>
			</div>
			
			<form on:submit|preventDefault={handleCreateEndpoint} class="p-6 space-y-4">
				<!-- Forge Type Selection -->
				<ForgeTypeSelector 
					bind:selectedForgeType 
					on:select={handleForgeTypeSelect}
				/>
				<div>
					<label for="name" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						Endpoint Name <span class="text-red-500">*</span>
					</label>
					<input
						type="text"
						id="name"
						bind:value={formData.name}
						required
						class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
						placeholder={selectedForgeType === 'github' ? 'e.g., github-enterprise or github-com' : 'e.g., gitea-main or my-gitea'}
					/>
				</div>

				<div>
					<label for="description" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						Description <span class="text-red-500">*</span>
					</label>
					<textarea
						id="description"
						bind:value={formData.description}
						rows="2"
						required
						class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
						placeholder="Brief description of this endpoint"
					></textarea>
				</div>

				<div>
					<label for="base_url" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						Base URL <span class="text-red-500">*</span>
					</label>
					<input
						type="url"
						id="base_url"
						bind:value={formData.base_url}
						required
						class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
						placeholder={selectedForgeType === 'github' ? 'https://github.com or https://github.example.com' : 'https://gitea.example.com'}
					/>
				</div>

				{#if selectedForgeType === 'github'}
					<div>
						<label for="api_base_url" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
							API Base URL <span class="text-red-500">*</span>
						</label>
						<input
							type="url"
							id="api_base_url"
							bind:value={formData.api_base_url}
							required
							class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
							placeholder="https://api.github.com or https://github.example.com/api/v3"
						/>
					</div>

					<div>
						<label for="upload_base_url" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
							Upload Base URL
						</label>
						<input
							type="url"
							id="upload_base_url"
							bind:value={formData.upload_base_url}
							class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
							placeholder="https://uploads.github.com"
						/>
					</div>
				{:else}
					<div>
						<label for="api_base_url" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
							API Base URL <span class="text-xs text-gray-500">(optional)</span>
						</label>
						<input
							type="url"
							id="api_base_url"
							bind:value={formData.api_base_url}
							class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
							placeholder="https://gitea.example.com/api/v1 (leave empty to use Base URL)"
						/>
						<p class="text-xs text-gray-500 dark:text-gray-400 mt-1">If empty, Base URL will be used as API Base URL</p>
					</div>
				{/if}

				<!-- CA Certificate Upload -->
				<div class="space-y-3 border-t border-gray-200 dark:border-gray-700 pt-4">
					<label for="ca_cert_file" class="block text-sm font-medium text-gray-700 dark:text-gray-300">CA Certificate Bundle (Optional)</label>
					<div class="border-2 border-dashed border-gray-300 dark:border-gray-600 rounded-lg p-4 text-center hover:border-blue-400 dark:hover:border-blue-400 transition-colors">
						<input
							type="file"
							id="ca_cert_file"
							accept=".pem,.crt,.cer,.cert"
							on:change={handleFileUpload}
							class="hidden"
						/>
						<div class="space-y-2">
							<svg class="mx-auto h-8 w-8 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
							</svg>
							<p class="text-sm text-gray-600 dark:text-gray-400">
								<button type="button" on:click={() => document.getElementById('ca_cert_file')?.click()} class="text-gray-900 dark:text-white hover:text-gray-700 dark:hover:text-gray-300 hover:underline cursor-pointer">
									Choose a file
								</button>
								or drag and drop
							</p>
							<p class="text-xs text-gray-500 dark:text-gray-400">PEM, CRT, CER, CERT files only</p>
						</div>
					</div>
				</div>

				<div class="flex justify-end space-x-3 pt-4 border-t border-gray-200 dark:border-gray-700">
					<button
						type="button"
						on:click={closeModals}
						class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-600 hover:bg-gray-200 dark:hover:bg-gray-500 rounded-md focus:outline-none focus:ring-2 focus:ring-gray-500 focus:ring-offset-2 cursor-pointer"
					>
						Cancel
					</button>
					<button
						type="submit"
						disabled={!isFormValid()}
						class="px-4 py-2 text-sm font-medium text-white rounded-md focus:outline-none focus:ring-2 focus:ring-offset-2 transition-colors
							{isFormValid() ? 'bg-blue-600 hover:bg-blue-700 focus:ring-blue-500 cursor-pointer' : 'bg-gray-400 cursor-not-allowed'}"
					>
						Create Endpoint
					</button>
				</div>
			</form>
		</div>
	</div>
{/if}

<!-- Edit Modal -->
{#if showEditModal && editingEndpoint}
	<div class="fixed inset-0 flex items-center justify-center z-50" role="dialog" aria-modal="true">
		<button class="fixed inset-0 w-full h-full bg-black/30 dark:bg-black/50 cursor-default" on:click={closeModals} aria-label="Close modal"></button>
		<div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-2xl w-full mx-4 max-h-screen overflow-y-auto relative z-10" role="document">
			<div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700 flex justify-between items-center">
				<div>
					<h3 class="text-lg font-semibold text-gray-900 dark:text-white">
						Edit {editingEndpoint.endpoint_type === 'github' ? 'GitHub' : 'Gitea'} Endpoint
					</h3>
					<p class="text-sm text-gray-600 dark:text-gray-400 mt-1">
						Update endpoint configuration
					</p>
				</div>
				<button on:click={closeModals} class="text-gray-400 hover:text-gray-600 dark:text-gray-300 dark:hover:text-gray-100 cursor-pointer" aria-label="Close modal">
					<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
					</svg>
				</button>
			</div>
			
			<form on:submit|preventDefault={handleUpdateEndpoint} class="p-6 space-y-4">
				<div>
					<label for="edit_name" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						Endpoint Name <span class="text-red-500">*</span>
					</label>
					<input
						type="text"
						id="edit_name"
						bind:value={formData.name}
						required
						class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
					/>
				</div>

				<div>
					<label for="edit_description" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						Description <span class="text-red-500">*</span>
					</label>
					<textarea
						id="edit_description"
						bind:value={formData.description}
						rows="2"
						required
						class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
					></textarea>
				</div>

				<div>
					<label for="edit_base_url" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						Base URL <span class="text-red-500">*</span>
					</label>
					<input
						type="url"
						id="edit_base_url"
						bind:value={formData.base_url}
						required
						class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
					/>
				</div>

				{#if editingEndpoint.endpoint_type === 'github'}
					<div>
						<label for="edit_api_base_url" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
							API Base URL <span class="text-red-500">*</span>
						</label>
						<input
							type="url"
							id="edit_api_base_url"
							bind:value={formData.api_base_url}
							required
							class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
						/>
					</div>

					<div>
						<label for="edit_upload_base_url" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
							Upload Base URL
						</label>
						<input
							type="url"
							id="edit_upload_base_url"
							bind:value={formData.upload_base_url}
							class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
						/>
					</div>
				{:else}
					<div>
						<label for="edit_api_base_url" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
							API Base URL <span class="text-xs text-gray-500">(optional)</span>
						</label>
						<input
							type="url"
							id="edit_api_base_url"
							bind:value={formData.api_base_url}
							class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
						/>
						<p class="text-xs text-gray-500 dark:text-gray-400 mt-1">If empty, Base URL will be used as API Base URL</p>
					</div>
				{/if}

				<!-- CA Certificate Upload -->
				<div class="space-y-3 border-t border-gray-200 dark:border-gray-700 pt-4">
					<label for="edit_ca_cert_file" class="block text-sm font-medium text-gray-700 dark:text-gray-300">CA Certificate Bundle (Optional)</label>
					<div class="border-2 border-dashed border-gray-300 dark:border-gray-600 rounded-lg p-4 text-center hover:border-blue-400 dark:hover:border-blue-400 transition-colors">
						<input
							type="file"
							id="edit_ca_cert_file"
							accept=".pem,.crt,.cer,.cert"
							on:change={handleFileUpload}
							class="hidden"
						/>
						<div class="space-y-2">
							<svg class="mx-auto h-8 w-8 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
							</svg>
							<p class="text-sm text-gray-600 dark:text-gray-400">
								<button type="button" on:click={() => document.getElementById('edit_ca_cert_file')?.click()} class="text-gray-900 dark:text-white hover:text-gray-700 dark:hover:text-gray-300 hover:underline cursor-pointer">
									Choose a file
								</button>
								or drag and drop
							</p>
							<p class="text-xs text-gray-500 dark:text-gray-400">PEM, CRT, CER, CERT files only</p>
						</div>
					</div>
				</div>

				<div class="flex justify-end space-x-3 pt-4 border-t border-gray-200 dark:border-gray-700">
					<button
						type="button"
						on:click={closeModals}
						class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-600 hover:bg-gray-200 dark:hover:bg-gray-500 rounded-md focus:outline-none focus:ring-2 focus:ring-gray-500 focus:ring-offset-2 cursor-pointer"
					>
						Cancel
					</button>
					<button
						type="submit"
						disabled={!isFormValid()}
						class="px-4 py-2 text-sm font-medium text-white rounded-md focus:outline-none focus:ring-2 focus:ring-offset-2 transition-colors
							{isFormValid() ? 'bg-blue-600 hover:bg-blue-700 focus:ring-blue-500 cursor-pointer' : 'bg-gray-400 cursor-not-allowed'}"
					>
						Update Endpoint
					</button>
				</div>
			</form>
		</div>
	</div>
{/if}

<!-- Delete Modal -->
{#if showDeleteModal && deletingEndpoint}
	<div class="fixed inset-0 flex items-center justify-center z-50" role="dialog" aria-modal="true">
		<button class="fixed inset-0 w-full h-full bg-black/30 dark:bg-black/50 cursor-default" on:click={closeModals} aria-label="Close modal"></button>
		<div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-xl w-full mx-4 relative z-10" role="document">
			<div class="px-6 py-4">
				<div class="flex items-center">
					<div class="flex-shrink-0">
						<svg class="h-6 w-6 text-red-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L3.732 16.5c-.77.833.192 2.5 1.732 2.5z"></path>
						</svg>
					</div>
					<div class="ml-3">
						<h3 class="text-lg font-medium text-gray-900 dark:text-white">Delete Endpoint</h3>
						<p class="mt-2 text-sm text-gray-500 dark:text-gray-300">
							Are you sure you want to delete the endpoint "{deletingEndpoint.name}"? This action cannot be undone.
						</p>
					</div>
				</div>
			</div>
			<div class="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex justify-end space-x-3">
				<button
					type="button"
					on:click={closeModals}
					class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-600 hover:bg-gray-200 dark:hover:bg-gray-500 rounded-md focus:outline-none focus:ring-2 focus:ring-gray-500 focus:ring-offset-2 cursor-pointer"
				>
					Cancel
				</button>
				<button
					type="button"
					on:click={handleDeleteEndpoint}
					class="px-4 py-2 text-sm font-medium text-white bg-red-600 hover:bg-red-700 rounded-md focus:outline-none focus:ring-2 focus:ring-red-500 focus:ring-offset-2 cursor-pointer"
				>
					Delete
				</button>
			</div>
		</div>
	</div>
{/if}