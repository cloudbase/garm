<script lang="ts">
	import { onMount } from 'svelte';
	import { garmApi } from '$lib/api/client.js';
	import type { ForgeCredentials, ForgeEndpoint, CreateGithubCredentialsParams, CreateGiteaCredentialsParams, GithubPAT, GithubApp } from '$lib/api/generated/api.js';
	// AuthType constants
	const AuthType = {
		PAT: 'pat',
		APP: 'app'
	} as const;
	import PageHeader from '$lib/components/PageHeader.svelte';
	import ForgeTypeSelector from '$lib/components/ForgeTypeSelector.svelte';
	import ActionButton from '$lib/components/ActionButton.svelte';
	import DataTable from '$lib/components/DataTable.svelte';
	import { eagerCache, eagerCacheManager } from '$lib/stores/eager-cache.js';
	import { toastStore } from '$lib/stores/toast.js';
	import { getForgeIcon, filterCredentials, changePerPage, paginateItems, getAuthTypeBadge } from '$lib/utils/common.js';
	import { extractAPIError } from '$lib/utils/apiError';
	import Badge from '$lib/components/Badge.svelte';
	import { EntityCell, EndpointCell, StatusCell, ActionsCell, GenericCell } from '$lib/components/cells';

	let loading = true;
	let credentials: ForgeCredentials[] = [];
	let endpoints: ForgeEndpoint[] = []; // Only used for modal dropdowns
	let error = '';
	let searchTerm = '';

	// Pagination
	let currentPage = 1;
	let perPage = 25;
	let totalPages = 1;

	// Subscribe to eager cache for credentials and endpoints (when WebSocket is connected)
	$: {
		// Only use cache data if we're not in direct API mode
		if (!credentials.length || $eagerCache.loaded.credentials) {
			credentials = $eagerCache.credentials;
		}
	}
	$: loading = $eagerCache.loading.credentials;
	$: cacheError = $eagerCache.errorMessages.credentials;
	$: {
		// Only use cache data if we're not in direct API mode
		if (!endpoints.length || $eagerCache.loaded.endpoints) {
			endpoints = $eagerCache.endpoints;
		}
	}

	$: filteredCredentials = filterCredentials(credentials, searchTerm);

	$: {
		totalPages = Math.ceil(filteredCredentials.length / perPage);
		if (currentPage > totalPages && totalPages > 0) {
			currentPage = totalPages;
		}
	}

	$: paginatedCredentials = paginateItems(filteredCredentials, currentPage, perPage);
	let showCreateModal = false;
	let showEditModal = false;
	let showDeleteModal = false;
	let selectedAuthType: typeof AuthType[keyof typeof AuthType] = AuthType.PAT;
	let editingCredential: ForgeCredentials | null = null;
	let deletingCredential: ForgeCredentials | null = null;
	// Form state
	let formData: {
		name: string;
		description: string;
		endpoint: string;
		auth_type: typeof AuthType[keyof typeof AuthType];
		oauth2_token: string;
		app_id: string;
		installation_id: string;
		private_key_bytes: string;
	} = {
		name: '',
		description: '',
		endpoint: '',
		auth_type: AuthType.PAT,
		oauth2_token: '',
		app_id: '',
		installation_id: '',
		private_key_bytes: ''
	};
	// Track original values for comparison during updates
	let originalFormData: typeof formData = { ...formData };
	// Checkbox to control whether to update credentials
	let wantToChangeCredentials = false;


	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape' && (showCreateModal || showEditModal || showDeleteModal)) {
			closeModals();
		}
	}

	onMount(async () => {
		// Load credentials and endpoints through eager cache (priority load + background load others)
		try {
			loading = true;
			const [credData, endpointData] = await Promise.all([
				eagerCacheManager.getCredentials(),
				eagerCacheManager.getEndpoints()
			]);
			// If WebSocket is disconnected, these return direct API data
			// Update our local arrays with this data
			if (credData && Array.isArray(credData)) {
				credentials = credData;
			}
			if (endpointData && Array.isArray(endpointData)) {
				endpoints = endpointData;
			}
		} catch (err) {
			// Cache error is already handled by the eager cache system
			// We don't need to set error here anymore since it's in the cache state
			console.error('Failed to load credentials:', err);
			error = err instanceof Error ? err.message : 'Failed to load credentials';
		} finally {
			loading = false;
		}
	});

	async function retryLoadCredentials() {
		try {
			await eagerCacheManager.retryResource('credentials');
		} catch (err) {
			console.error('Retry failed:', err);
		}
	}

	// Credentials are now handled by eager cache with websocket subscriptions


	// Endpoints are now loaded through eager cache

	async function showCreateCredentialsModal() {
		resetForm();
		// Endpoints are already loaded through eager cache
		showCreateModal = true;
		selectedForgeType = 'github'; // Default to github
		formData.auth_type = AuthType.PAT; // Ensure auth_type is set
	}

	// Add forge type selection state
	let selectedForgeType: 'github' | 'gitea' | '' = '';

	function handleForgeTypeSelect(event: CustomEvent<'github' | 'gitea'>) {
		selectedForgeType = event.detail;
		// Reset form when forge type changes
		resetForm();
	}

	async function showEditCredentialsModal(credential: ForgeCredentials) {
		editingCredential = credential;
		formData = {
			name: credential.name || '',
			description: credential.description || '',
			endpoint: credential.endpoint?.name || '',
			auth_type: (credential['auth-type'] as typeof AuthType[keyof typeof AuthType]) || AuthType.PAT,
			oauth2_token: '',
			app_id: '',
			installation_id: '',
			private_key_bytes: ''
		};
		selectedAuthType = (credential['auth-type'] as typeof AuthType[keyof typeof AuthType]) || AuthType.PAT;
		// Store original values for comparison
		originalFormData = { ...formData };
		// Reset checkbox state
		wantToChangeCredentials = false;
		// Endpoints are already loaded through eager cache
		showEditModal = true;
	}

	function showDeleteCredentialsModal(credential: ForgeCredentials) {
		deletingCredential = credential;
		showDeleteModal = true;
	}

	function resetForm() {
		formData = {
			name: '',
			description: '',
			endpoint: '',
			auth_type: AuthType.PAT,
			oauth2_token: '',
			app_id: '',
			installation_id: '',
			private_key_bytes: ''
		};
		originalFormData = { ...formData };
		selectedAuthType = AuthType.PAT;
		wantToChangeCredentials = false;
	}

	function closeModals() {
		showCreateModal = false;
		showEditModal = false;
		showDeleteModal = false;
		editingCredential = null;
		deletingCredential = null;
		selectedForgeType = '';
		resetForm();
	}

	function handleAuthTypeChange(authType: typeof AuthType[keyof typeof AuthType]) {
		selectedAuthType = authType;
		formData.auth_type = authType;
	}

	function buildUpdateParams() {
		const updateParams: any = {};
		
		// Only include name and description if they have changed from original values
		if (formData.name !== originalFormData.name) {
			if (formData.name.trim() !== '') {
				updateParams.name = formData.name.trim();
			}
		}
		
		if (formData.description !== originalFormData.description) {
			if (formData.description.trim() !== '') {
				updateParams.description = formData.description.trim();
			}
		}
		
		// Only include credential fields if the checkbox is checked and fields have values
		if (wantToChangeCredentials && editingCredential) {
			if (editingCredential['auth-type'] === AuthType.PAT) {
				// PAT credentials
				if (formData.oauth2_token.trim() !== '') {
					updateParams.pat = {
						oauth2_token: formData.oauth2_token.trim()
					};
				}
			} else {
				// App credentials
				const appUpdate: any = {};
				let hasAppChanges = false;
				
				if (formData.app_id.trim() !== '') {
					appUpdate.app_id = parseInt(formData.app_id.trim());
					hasAppChanges = true;
				}
				
				if (formData.installation_id.trim() !== '') {
					appUpdate.installation_id = parseInt(formData.installation_id.trim());
					hasAppChanges = true;
				}
				
				if (formData.private_key_bytes !== '') {
					// Convert base64 string to byte array for API
					try {
						const bytes = atob(formData.private_key_bytes);
						appUpdate.private_key_bytes = Array.from(bytes, char => char.charCodeAt(0));
						hasAppChanges = true;
					} catch (e) {
						// Invalid base64, ignore
					}
				}
				
				if (hasAppChanges) {
					updateParams.app = appUpdate;
				}
			}
		}
		
		return updateParams;
	}

	async function handleCreateCredentials() {
		try {
			// Use selected forge type to determine which API to call
			if (selectedForgeType === 'github') {
				// Build the correct nested structure for GitHub credentials
				const githubParams: CreateGithubCredentialsParams = {
					name: formData.name.trim(),
					description: formData.description.trim(),
					endpoint: formData.endpoint.trim(),
					auth_type: formData.auth_type
				};

				if (formData.auth_type === AuthType.PAT) {
					githubParams.pat = {
						oauth2_token: formData.oauth2_token.trim()
					};
					githubParams.app = {}; // Empty app object for PAT
				} else {
					githubParams.app = {
						app_id: parseInt(formData.app_id.trim()),
						installation_id: parseInt(formData.installation_id.trim()),
						private_key_bytes: Array.from(atob(formData.private_key_bytes), char => char.charCodeAt(0))
					};
					githubParams.pat = {}; // Empty PAT object for App
				}

				await garmApi.createGithubCredentials(githubParams);
			} else if (selectedForgeType === 'gitea') {
				// Build the correct nested structure for Gitea credentials (PAT only)
				const giteaParams: CreateGiteaCredentialsParams = {
					name: formData.name.trim(),
					description: formData.description.trim(),
					endpoint: formData.endpoint.trim(),
					auth_type: AuthType.PAT, // Gitea only supports PAT
					pat: {
						oauth2_token: formData.oauth2_token.trim()
					},
					app: {} // Empty app object since Gitea only supports PAT
				};

				await garmApi.createGiteaCredentials(giteaParams);
			} else {
				throw new Error('Please select a forge type');
			}
			// No need to reload - websocket will handle the update
			toastStore.success(
				'Credentials Created',
				`Credentials ${formData.name} have been created successfully.`
			);
			closeModals();
		} catch (err) {
			error = extractAPIError(err);
		}
	}

	async function handleUpdateCredentials() {
		if (!editingCredential || !editingCredential.id) return;
		
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
			
			const endpointType = editingCredential.forge_type;
			if (endpointType === 'github') {
				await garmApi.updateGithubCredentials(editingCredential.id, updateParams);
			} else {
				await garmApi.updateGiteaCredentials(editingCredential.id, updateParams);
			}
			// No need to reload - websocket will handle the update
			toastStore.success(
				'Credentials Updated',
				`Credentials ${editingCredential?.name || 'Unknown'} have been updated successfully.`
			);
			closeModals();
		} catch (err) {
			error = extractAPIError(err);
		}
	}

	async function handleDeleteCredentials() {
		if (!deletingCredential || !deletingCredential.id) return;
		
		try {
			const endpointType = deletingCredential.forge_type;
			if (endpointType === 'github') {
				await garmApi.deleteGithubCredentials(deletingCredential.id);
			} else {
				await garmApi.deleteGiteaCredentials(deletingCredential.id);
			}
			// No need to reload - websocket will handle the update
			toastStore.success(
				'Credentials Deleted',
				`Credentials ${deletingCredential?.name || 'Unknown'} have been deleted successfully.`
			);
		} catch (err) {
			const errorMessage = extractAPIError(err);
			toastStore.error('Delete Failed', errorMessage);
		} finally {
			closeModals();
		}
	}

	function handlePrivateKeyUpload(event: Event) {
		const target = event.target as HTMLInputElement;
		const file = target.files?.[0];
		
		if (!file) {
			formData.private_key_bytes = '';
			return;
		}

		const reader = new FileReader();
		reader.onload = (e) => {
			const content = e.target?.result as string;
			formData.private_key_bytes = btoa(content);
		};
		reader.readAsText(file);
	}

	// Reactive form validation
	$: isFormValid = (() => {
		if (!formData.name || !formData.endpoint) return false;
		
		if (formData.auth_type === AuthType.PAT) {
			return !!formData.oauth2_token;
		} else {
			return !!formData.app_id && !!formData.installation_id && !!formData.private_key_bytes;
		}
	})();
	
	function isEditFormValid() {
		// For updates, basic fields are required
		if (!formData.name.trim()) return false;
		
		// If checkbox is checked, validate credential fields
		if (wantToChangeCredentials && editingCredential) {
			if (editingCredential['auth-type'] === AuthType.PAT) {
				return !!formData.oauth2_token.trim();
			} else {
				return !!formData.app_id.trim() && !!formData.installation_id.trim() && !!formData.private_key_bytes;
			}
		}
		
		// If checkbox is not checked, just need basic fields
		return true;
	}

	function getEndpointForgeType(endpointName: string): string {
		const endpoint = endpoints.find(e => e.name === endpointName);
		return endpoint?.endpoint_type || '';
	}

	function isGiteaEndpoint(endpointName: string): boolean {
		return getEndpointForgeType(endpointName) === 'gitea';
	}

	// Get filtered endpoints based on selected forge type
	$: filteredEndpoints = selectedForgeType ? endpoints.filter(e => e.endpoint_type === selectedForgeType) : endpoints;

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
			cellProps: { field: 'description', type: 'description' }
		},
		{ 
			key: 'endpoint', 
			title: 'Endpoint',
			cellComponent: EndpointCell
		},
		{ 
			key: 'auth_type', 
			title: 'Auth Type',
			cellComponent: StatusCell,
			cellProps: { statusType: 'custom', statusField: 'auth-type' }
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
		entityType: 'credential' as const,
		primaryText: {
			field: 'name',
			isClickable: false
		},
		secondaryText: {
			field: 'description'
		},
		customInfo: [
			{ 
				icon: (item: any) => getForgeIcon(item?.forge_type || 'unknown'),
				text: (item: any) => item?.endpoint?.name || 'Unknown'
			}
		],
		badges: [
			{ 
				type: 'auth' as const, 
				field: 'auth-type'
			}
		],
		actions: [
			{ 
				type: 'edit' as const, 
				handler: (item: any) => showEditCredentialsModal(item) 
			},
			{ 
				type: 'delete' as const, 
				handler: (item: any) => showDeleteCredentialsModal(item)
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
		showEditCredentialsModal(event.detail.item);
	}

	function handleDelete(event: CustomEvent<{ item: any }>) {
		showDeleteCredentialsModal(event.detail.item);
	}
</script>

<svelte:window on:keydown={handleKeydown} />

<svelte:head>
	<title>Credentials - GARM</title>
</svelte:head>

<div class="space-y-6">
	<!-- Header -->
	<PageHeader
		title="Credentials"
		description="Manage authentication credentials for your GitHub and Gitea endpoints."
		actionLabel="Add Credentials"
		on:action={showCreateCredentialsModal}
	/>

	<DataTable
		{columns}
		data={paginatedCredentials}
		{loading}
		error={cacheError || error}
		{searchTerm}
		searchPlaceholder="Search credentials by name, description, or endpoint..."
		{currentPage}
		{perPage}
		{totalPages}
		totalItems={filteredCredentials.length}
		itemName="credentials"
		emptyIconType="key"
		showRetry={!!cacheError}
		{mobileCardConfig}
		on:search={handleTableSearch}
		on:pageChange={handleTablePageChange}
		on:perPageChange={handleTablePerPageChange}
		on:retry={retryLoadCredentials}
		on:edit={handleEdit}
		on:delete={handleDelete}
	>
		<!-- Mobile card layout -->
		<svelte:fragment slot="mobile-card" let:item={credential}>
			<div class="flex items-center justify-between">
				<div class="flex-1 min-w-0">
					<div class="block">
						<p class="text-sm font-medium text-gray-900 dark:text-white truncate">
							{credential.name}
						</p>
						<p class="text-sm text-gray-500 dark:text-gray-400 truncate mt-1">
							{credential.description}
						</p>
						<div class="flex items-center mt-2 space-x-3">
							<div class="flex items-center">
								{@html getForgeIcon(credential.forge_type || 'unknown')}
								<span class="ml-1 text-xs text-gray-500 dark:text-gray-400">{credential.endpoint?.name || 'Unknown'}</span>
							</div>
						</div>
					</div>
				</div>
				<div class="flex items-center space-x-3 ml-4">
					{#if (credential['auth-type'] || 'pat') === 'pat'}
						<Badge variant="success" text="PAT" />
					{:else}
						<Badge variant="info" text="App" />
					{/if}
					<div class="flex space-x-2">
						<ActionButton
							action="edit"
							size="sm"
							title="Edit credentials"
							ariaLabel="Edit credentials"
							on:click={() => showEditCredentialsModal(credential)}
						/>
						<ActionButton
							action="delete"
							size="sm"
							title="Delete credentials"
							ariaLabel="Delete credentials"
							on:click={() => showDeleteCredentialsModal(credential)}
						/>
					</div>
				</div>
			</div>
		</svelte:fragment>
		
		<!-- Desktop table cells -->
	</DataTable>
</div>

<!-- Create Modal -->
{#if showCreateModal}
	<div class="fixed inset-0 flex items-center justify-center z-50" role="dialog" aria-modal="true">
		<button class="fixed inset-0 w-full h-full bg-black/30 dark:bg-black/50 cursor-default" on:click={closeModals} aria-label="Close modal"></button>
		<div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-md w-full mx-4 max-h-screen overflow-y-auto relative z-10" role="document">
			<div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700 flex justify-between items-center">
				<div>
					<h3 class="text-lg font-semibold text-gray-900 dark:text-white">
						Add Credentials
					</h3>
					<p class="text-sm text-gray-600 dark:text-gray-400 mt-1">
						Create new authentication credentials
					</p>
				</div>
				<button on:click={closeModals} class="text-gray-400 hover:text-gray-600 dark:text-gray-300 dark:hover:text-gray-100 cursor-pointer" aria-label="Close modal">
					<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
					</svg>
				</button>
			</div>
			
				<form on:submit|preventDefault={handleCreateCredentials} class="p-6 space-y-4">
					<!-- Forge Type Selection -->
					<ForgeTypeSelector 
						bind:selectedForgeType 
						on:select={handleForgeTypeSelect}
					/>

					<div>
						<label for="name" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
							Credentials Name <span class="text-red-500">*</span>
						</label>
						<input
							type="text"
							id="name"
							bind:value={formData.name}
							required
							autocomplete="off"
							class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
							placeholder="e.g., my-github-credentials"
						/>
					</div>

				<div>
					<label for="description" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						Description
					</label>
					<textarea
						id="description"
						bind:value={formData.description}
						rows="2"
						autocomplete="off"
						class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
						placeholder="Brief description of these credentials"
					></textarea>
				</div>

				<div>
					<label for="endpoint" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						Endpoint <span class="text-red-500">*</span>
					</label>
					<select
						id="endpoint"
						bind:value={formData.endpoint}
						required
						autocomplete="off"
						class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
					>
						<option value="">Select an endpoint</option>
						{#each filteredEndpoints as endpoint}
							<option value={endpoint.name}>
								{endpoint.name} ({endpoint.endpoint_type})
							</option>
						{/each}
					</select>
					{#if selectedForgeType}
						<p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
							Showing only {selectedForgeType} endpoints
						</p>
					{/if}
				</div>

				<!-- Authentication Type Selection -->
				<div role="group" aria-labelledby="auth-type-heading">
					<div id="auth-type-heading" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
						Authentication Type <span class="text-red-500">*</span>
					</div>
					<div class="flex space-x-4">
						<button
							type="button"
							on:click={() => handleAuthTypeChange(AuthType.PAT)}
							class="flex-1 py-2 px-4 text-sm font-medium rounded-md border focus:outline-none focus:ring-2 focus:ring-blue-500 cursor-pointer
								{selectedAuthType === AuthType.PAT 
									? 'bg-blue-600 text-white border-blue-600' 
									: 'bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-300 border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-600'}
								{formData.endpoint && isGiteaEndpoint(formData.endpoint) ? '' : ''}"
						>
							PAT
						</button>
						<button
							type="button"
							on:click={() => handleAuthTypeChange(AuthType.APP)}
							disabled={selectedForgeType === 'gitea'}
							class="flex-1 py-2 px-4 text-sm font-medium rounded-md border focus:outline-none focus:ring-2 focus:ring-blue-500
								{selectedAuthType === AuthType.APP 
									? 'bg-blue-600 text-white border-blue-600' 
									: 'bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-300 border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-600'}
								{selectedForgeType === 'gitea' ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}"
						>
							App
						</button>
					</div>
					{#if selectedForgeType === 'gitea'}
						<p class="text-xs text-gray-500 dark:text-gray-400 mt-1">Gitea only supports PAT authentication</p>
					{/if}
				</div>

				<!-- PAT Fields -->
				{#if selectedAuthType === AuthType.PAT}
					<div>
						<label for="oauth2_token" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
							Personal Access Token <span class="text-red-500">*</span>
						</label>
						<input
							type="password"
							id="oauth2_token"
							bind:value={formData.oauth2_token}
							required
							autocomplete="off"
							class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
							placeholder="ghp_xxxxxxxxxxxxxxxxxxxx"
						/>
					</div>
				{/if}

				<!-- App Fields -->
				{#if selectedAuthType === AuthType.APP}
					<div>
						<label for="app_id" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
							App ID <span class="text-red-500">*</span>
						</label>
						<input
							type="text"
							id="app_id"
							bind:value={formData.app_id}
							required
							autocomplete="off"
							class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
							placeholder="123456"
						/>
					</div>

					<div>
						<label for="installation_id" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
							App Installation ID <span class="text-red-500">*</span>
						</label>
						<input
							type="text"
							id="installation_id"
							bind:value={formData.installation_id}
							required
							autocomplete="off"
							class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
							placeholder="12345678"
						/>
					</div>

					<div>
						<label for="private_key" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
							Private Key <span class="text-red-500">*</span>
						</label>
						<div class="border-2 border-dashed border-gray-300 dark:border-gray-600 rounded-lg p-4 text-center hover:border-blue-400 dark:hover:border-blue-400 transition-colors">
							<input
								type="file"
								id="private_key"
								accept=".pem,.key"
								on:change={handlePrivateKeyUpload}
								class="hidden"
							/>
							<div class="space-y-2">
								<svg class="mx-auto h-8 w-8 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
								</svg>
								<p class="text-sm text-gray-600 dark:text-gray-400">
									<button type="button" on:click={() => document.getElementById('private_key')?.click()} class="text-gray-900 dark:text-white hover:text-gray-700 dark:hover:text-gray-300 hover:underline cursor-pointer">
										Choose a file
									</button>
									or drag and drop
								</p>
								<p class="text-xs text-gray-500 dark:text-gray-400">PEM, KEY files only</p>
							</div>
						</div>
					</div>
				{/if}

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
						disabled={!isFormValid}
						class="px-4 py-2 text-sm font-medium text-white rounded-md focus:outline-none focus:ring-2 focus:ring-offset-2 transition-colors
							{isFormValid ? 'bg-blue-600 hover:bg-blue-700 focus:ring-blue-500 cursor-pointer' : 'bg-gray-400 cursor-not-allowed'}"
					>
						Create Credentials
					</button>
				</div>
			</form>
		</div>
	</div>
{/if}

<!-- Edit Modal -->
{#if showEditModal && editingCredential}
	<div class="fixed inset-0 flex items-center justify-center z-50" role="dialog" aria-modal="true">
		<button class="fixed inset-0 w-full h-full bg-black/30 dark:bg-black/50 cursor-default" on:click={closeModals} aria-label="Close modal"></button>
		<div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-2xl w-full mx-4 max-h-screen overflow-y-auto relative z-10" role="document">
			<div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700 flex justify-between items-center">
				<div>
					<h3 class="text-lg font-semibold text-gray-900 dark:text-white">
						Edit Credentials
					</h3>
					<p class="text-sm text-gray-600 dark:text-gray-400 mt-1">
						Update credentials for {editingCredential?.name || 'Unknown'}
					</p>
				</div>
				<button on:click={closeModals} class="text-gray-400 hover:text-gray-600 dark:text-gray-300 dark:hover:text-gray-100 cursor-pointer" aria-label="Close modal">
					<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
					</svg>
				</button>
			</div>
			
			<form on:submit|preventDefault={handleUpdateCredentials} class="p-6 space-y-4">
				<div>
					<label for="edit_name" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						Credentials Name <span class="text-red-500">*</span>
					</label>
					<input
						type="text"
						id="edit_name"
						bind:value={formData.name}
						required
						autocomplete="off"
						class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
					/>
				</div>

				<div>
					<label for="edit_description" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						Description
					</label>
					<textarea
						id="edit_description"
						bind:value={formData.description}
						rows="2"
						autocomplete="off"
						class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
					></textarea>
				</div>

				<div>
					<label for="edit-endpoint" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						Endpoint
					</label>
					<input
						type="text"
						id="edit-endpoint"
						value={formData.endpoint}
						disabled
						class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-gray-100 dark:bg-gray-600 text-gray-500 dark:text-gray-400 cursor-not-allowed"
					/>
					<p class="text-xs text-gray-500 dark:text-gray-400 mt-1">Endpoint cannot be changed after creation</p>
				</div>

				<!-- Authentication Type Display -->
				<div>
					<span class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						Authentication Type
					</span>
					<div class="px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-gray-100 dark:bg-gray-600">
						<span class="text-sm font-medium text-gray-700 dark:text-gray-300">
							{(editingCredential?.['auth-type'] || AuthType.PAT) === AuthType.PAT ? 'Personal Access Token (PAT)' : 'GitHub App'}
						</span>
					</div>
					<p class="text-xs text-gray-500 dark:text-gray-400 mt-1">Authentication type cannot be changed after creation</p>
				</div>

				<!-- Credentials Update Checkbox -->
				<div class="border-t border-gray-200 dark:border-gray-700 pt-4">
					<div class="flex items-center">
						<input
							id="change_credentials_checkbox"
							type="checkbox"
							bind:checked={wantToChangeCredentials}
							class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 dark:border-gray-600 rounded"
						/>
						<label for="change_credentials_checkbox" class="ml-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
							I want to change credentials
						</label>
					</div>
					<p class="text-xs text-gray-500 dark:text-gray-400 mt-1">Check this box to update authentication credentials</p>
				</div>

				<!-- Conditional Credential Fields -->
				{#if wantToChangeCredentials}
					<!-- PAT Fields -->
					{#if editingCredential['auth-type'] === AuthType.PAT}
						<div>
							<label for="edit_oauth2_token" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
								New Personal Access Token <span class="text-red-500">*</span>
							</label>
							<input
								type="password"
								id="edit_oauth2_token"
								bind:value={formData.oauth2_token}
								required
								autocomplete="off"
								class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
								placeholder="Enter new token"
							/>
						</div>
					{/if}

					<!-- App Fields -->
					{#if editingCredential['auth-type'] === AuthType.APP}
						<div>
							<label for="edit_app_id" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
								App ID <span class="text-red-500">*</span>
							</label>
							<input
								type="text"
								id="edit_app_id"
								bind:value={formData.app_id}
								required
								autocomplete="off"
								class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
								placeholder="Enter new App ID"
							/>
						</div>

						<div>
							<label for="edit_installation_id" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
								App Installation ID <span class="text-red-500">*</span>
							</label>
							<input
								type="text"
								id="edit_installation_id"
								bind:value={formData.installation_id}
								required
								autocomplete="off"
								class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
								placeholder="Enter new Installation ID"
							/>
						</div>

						<div>
							<label for="edit_private_key" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
								Private Key <span class="text-red-500">*</span>
							</label>
							<div class="border-2 border-dashed border-gray-300 dark:border-gray-600 rounded-lg p-4 text-center hover:border-blue-400 dark:hover:border-blue-400 transition-colors">
								<input
									type="file"
									id="edit_private_key"
									accept=".pem,.key"
									on:change={handlePrivateKeyUpload}
									class="hidden"
								/>
								<div class="space-y-2">
									<svg class="mx-auto h-8 w-8 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
									</svg>
									<p class="text-sm text-gray-600 dark:text-gray-400">
										<button type="button" on:click={() => document.getElementById('edit_private_key')?.click()} class="text-gray-900 dark:text-white hover:text-gray-700 dark:hover:text-gray-300 hover:underline cursor-pointer">
											Choose a new file
										</button>
										or drag and drop
									</p>
									<p class="text-xs text-gray-500 dark:text-gray-400">PEM, KEY files only. Upload new private key.</p>
								</div>
							</div>
						</div>
					{/if}
				{/if}

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
						disabled={!isEditFormValid()}
						class="px-4 py-2 text-sm font-medium text-white rounded-md focus:outline-none focus:ring-2 focus:ring-offset-2 transition-colors
							{isEditFormValid() ? 'bg-blue-600 hover:bg-blue-700 focus:ring-blue-500 cursor-pointer' : 'bg-gray-400 cursor-not-allowed'}"
					>
						Update Credentials
					</button>
				</div>
			</form>
		</div>
	</div>
{/if}

<!-- Delete Modal -->
{#if showDeleteModal && deletingCredential}
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
						<h3 class="text-lg font-medium text-gray-900 dark:text-white">Delete Credentials</h3>
						<p class="mt-2 text-sm text-gray-500 dark:text-gray-300">
							Are you sure you want to delete the credentials "{deletingCredential?.name || 'Unknown'}"? This action cannot be undone.
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
					on:click={handleDeleteCredentials}
					class="px-4 py-2 text-sm font-medium text-white bg-red-600 hover:bg-red-700 rounded-md focus:outline-none focus:ring-2 focus:ring-red-500 focus:ring-offset-2 cursor-pointer"
				>
					Delete
				</button>
			</div>
		</div>
	</div>
{/if}