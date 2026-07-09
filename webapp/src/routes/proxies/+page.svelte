<script lang="ts">
	import { onMount } from 'svelte';
	import { garmApi } from '$lib/api/client.js';
	import type { Proxy, CreateProxyParams, UpdateProxyParams } from '$lib/api/generated/api.js';
	import PageHeader from '$lib/components/PageHeader.svelte';
	import DataTable from '$lib/components/DataTable.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import DeleteModal from '$lib/components/DeleteModal.svelte';
	import Badge from '$lib/components/Badge.svelte';
	import { toastStore } from '$lib/stores/toast.js';
	import { extractAPIError } from '$lib/utils/apiError';
	import { GenericCell, ActionsCell } from '$lib/components/cells';
	import { eagerCache, eagerCacheManager } from '$lib/stores/eager-cache';

	let proxies: Proxy[] = [];
	let error = '';
	let searchTerm = '';

	// Subscribe to eager cache for proxies
	$: {
		// Only use cache data if we're not in direct API mode
		if (!proxies.length || $eagerCache.loaded.proxies) {
			proxies = $eagerCache.proxies;
		}
	}
	$: loading = $eagerCache.loading.proxies;
	$: cacheError = $eagerCache.errorMessages.proxies;

	// Pagination
	let currentPage = 1;
	let perPage = 25;
	let totalPages = 1;

	// Modals
	let showCreateModal = false;
	let showEditModal = false;
	let showDeleteModal = false;
	let selectedProxy: Proxy | null = null;
	let submitting = false;

	// Form state
	let formData = {
		name: '',
		description: '',
		http_proxy: '',
		https_proxy: '',
		no_proxy: '',
		username: '',
		password: ''
	};
	// Track original values for change comparison during updates
	let originalFormData: typeof formData = { ...formData };

	$: filteredProxies = searchTerm
		? proxies.filter(proxy =>
			proxy.name?.toLowerCase().includes(searchTerm.toLowerCase()) ||
			proxy.description?.toLowerCase().includes(searchTerm.toLowerCase()) ||
			proxy.http_proxy?.toLowerCase().includes(searchTerm.toLowerCase()) ||
			proxy.https_proxy?.toLowerCase().includes(searchTerm.toLowerCase())
		)
		: proxies;

	$: {
		totalPages = Math.ceil(filteredProxies.length / perPage);
		if (currentPage > totalPages && totalPages > 0) {
			currentPage = totalPages;
		}
	}

	$: paginatedProxies = filteredProxies.slice(
		(currentPage - 1) * perPage,
		currentPage * perPage
	);

	async function retryLoadProxies() {
		try {
			await eagerCacheManager.retryResource('proxies');
		} catch (err) {
			console.error('Retry failed:', err);
		}
	}

	function resetForm() {
		formData = {
			name: '',
			description: '',
			http_proxy: '',
			https_proxy: '',
			no_proxy: '',
			username: '',
			password: ''
		};
		originalFormData = { ...formData };
	}

	function openCreateModal() {
		resetForm();
		showCreateModal = true;
	}

	function openEditModal(proxy: Proxy) {
		selectedProxy = proxy;
		formData = {
			name: proxy.name || '',
			description: proxy.description || '',
			http_proxy: proxy.http_proxy || '',
			https_proxy: proxy.https_proxy || '',
			no_proxy: proxy.no_proxy || '',
			username: proxy.username || '',
			// Password is write-only and never returned by the API
			password: ''
		};
		originalFormData = { ...formData };
		showEditModal = true;
	}

	function openDeleteModal(proxy: Proxy) {
		selectedProxy = proxy;
		showDeleteModal = true;
	}

	function closeModals() {
		showCreateModal = false;
		showEditModal = false;
		showDeleteModal = false;
		selectedProxy = null;
		resetForm();
	}

	async function handleCreateProxy() {
		try {
			submitting = true;
			const params: CreateProxyParams = {
				name: formData.name.trim(),
				description: formData.description.trim() || undefined,
				http_proxy: formData.http_proxy.trim() || undefined,
				https_proxy: formData.https_proxy.trim() || undefined,
				no_proxy: formData.no_proxy.trim() || undefined,
				username: formData.username.trim() || undefined,
				password: formData.password || undefined
			};

			await garmApi.createProxy(params);
			toastStore.add({
				type: 'success',
				title: 'Proxy created',
				message: `Proxy "${params.name}" has been created successfully.`
			});
			closeModals();
		} catch (err) {
			toastStore.add({
				type: 'error',
				title: 'Failed to create proxy',
				message: extractAPIError(err)
			});
		} finally {
			submitting = false;
		}
	}

	function buildUpdateParams(): UpdateProxyParams {
		const params: UpdateProxyParams = {};

		// Only send fields that changed from their original values
		if (formData.name.trim() !== originalFormData.name) {
			params.name = formData.name.trim();
		}
		if (formData.description !== originalFormData.description) {
			params.description = formData.description;
		}
		if (formData.http_proxy !== originalFormData.http_proxy) {
			params.http_proxy = formData.http_proxy.trim();
		}
		if (formData.https_proxy !== originalFormData.https_proxy) {
			params.https_proxy = formData.https_proxy.trim();
		}
		if (formData.no_proxy !== originalFormData.no_proxy) {
			params.no_proxy = formData.no_proxy.trim();
		}
		if (formData.username !== originalFormData.username) {
			// Clearing the username removes the credentials
			params.username = formData.username.trim();
		}
		// Password is write-only; a blank password keeps the current one
		if (formData.password !== '') {
			params.password = formData.password;
		}

		return params;
	}

	async function handleUpdateProxy() {
		if (!selectedProxy?.id) return;

		try {
			submitting = true;
			const params = buildUpdateParams();

			if (Object.keys(params).length === 0) {
				toastStore.add({
					type: 'info',
					title: 'No changes',
					message: 'No fields were modified.'
				});
				closeModals();
				return;
			}

			await garmApi.updateProxy(selectedProxy.id, params);
			toastStore.add({
				type: 'success',
				title: 'Proxy updated',
				message: `Proxy "${selectedProxy.name}" has been updated successfully.`
			});
			closeModals();
		} catch (err) {
			toastStore.add({
				type: 'error',
				title: 'Failed to update proxy',
				message: extractAPIError(err)
			});
		} finally {
			submitting = false;
		}
	}

	async function handleDeleteProxy() {
		if (!selectedProxy?.id) return;

		try {
			await garmApi.deleteProxy(selectedProxy.id);
			toastStore.add({
				type: 'success',
				title: 'Proxy deleted',
				message: `Proxy "${selectedProxy.name}" has been deleted successfully.`
			});
			showDeleteModal = false;
			selectedProxy = null;
		} catch (err) {
			// Deletion is blocked when the proxy is in use by pools or scale sets
			toastStore.add({
				type: 'error',
				title: 'Failed to delete proxy',
				message: extractAPIError(err)
			});
		}
	}

	$: isFormValid = formData.name.trim() !== '';

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
			key: 'http_proxy',
			title: 'HTTP Proxy',
			cellComponent: GenericCell,
			cellProps: { field: 'http_proxy' }
		},
		{
			key: 'https_proxy',
			title: 'HTTPS Proxy',
			cellComponent: GenericCell,
			cellProps: { field: 'https_proxy' }
		},
		{
			key: 'auth',
			title: 'Auth'
		},
		{
			key: 'actions',
			title: 'Actions',
			align: 'right' as const,
			cellComponent: ActionsCell
		}
	];

	// Mobile card configuration for proxies
	const mobileCardConfig = {
		entityType: 'proxy',
		primaryText: {
			field: 'name',
			isClickable: false
		},
		secondaryText: {
			field: 'description'
		},
		badges: [
			{
				type: 'custom',
				value: (item: any) => item.username
					? { variant: 'success', text: 'Authenticated' }
					: { variant: 'gray', text: 'No auth' }
			}
		],
		actions: [
			{
				type: 'edit',
				handler: (item: any) => openEditModal(item)
			},
			{
				type: 'delete',
				handler: (item: any) => openDeleteModal(item)
			}
		]
	};

	onMount(async () => {
		// Load proxies through eager cache (priority load + background load others)
		try {
			const proxyData = await eagerCacheManager.getProxies();
			// If WebSocket is disconnected, getProxies returns direct API data
			if (proxyData && Array.isArray(proxyData)) {
				proxies = proxyData;
			}
		} catch (err) {
			// Cache error is already handled by the eager cache system
			console.error('Failed to load proxies:', err);
			error = err instanceof Error ? err.message : 'Failed to load proxies';
		}
	});
</script>

<svelte:head>
	<title>Proxies - GARM</title>
</svelte:head>

<PageHeader
	title="Proxies"
	description="Manage proxy definitions runners can use to reach GARM, the forge and other resources. Proxies can be set on pools or scale sets."
	actionLabel="Create Proxy"
	showAction={true}
	on:action={openCreateModal}
/>

<DataTable
	{columns}
	data={paginatedProxies}
	{loading}
	error={cacheError || error}
	showRetry={!!(cacheError || error)}
	on:retry={retryLoadProxies}
	{searchTerm}
	searchPlaceholder="Search proxies by name, description or URL..."
	{currentPage}
	{perPage}
	{totalPages}
	totalItems={filteredProxies.length}
	itemName="proxies"
	{mobileCardConfig}
	on:search={(e) => { searchTerm = e.detail.term; currentPage = 1; }}
	on:pageChange={(e) => currentPage = e.detail.page}
	on:perPageChange={(e) => { perPage = e.detail.perPage; currentPage = 1; }}
	on:edit={(e) => openEditModal(e.detail.item)}
	on:delete={(e) => openDeleteModal(e.detail.item)}
	emptyMessage="No proxies found"
>
	<svelte:fragment slot="cell" let:item let:column>
		{#if column.key === 'auth'}
			{#if item.username}
				<Badge variant="success" text="Authenticated" />
			{:else}
				<Badge variant="gray" text="No auth" />
			{/if}
		{/if}
	</svelte:fragment>
</DataTable>

<!-- Create/Edit Proxy Modal -->
{#if showCreateModal || (showEditModal && selectedProxy)}
	<Modal on:close={closeModals}>
		<div class="max-w-2xl w-full max-h-[90vh] overflow-y-auto">
			<div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
				<h2 class="text-xl font-semibold text-gray-900 dark:text-white">
					{showCreateModal ? 'Create Proxy' : `Edit Proxy ${selectedProxy?.name}`}
				</h2>
			</div>

			<form on:submit|preventDefault={showCreateModal ? handleCreateProxy : handleUpdateProxy} class="p-6 space-y-4">
				<div>
					<label for="proxy-name" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
						Name <span class="text-red-500">*</span>
					</label>
					<input
						id="proxy-name"
						type="text"
						bind:value={formData.name}
						required
						placeholder="e.g., corporate-proxy"
						class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
					/>
				</div>

				<div>
					<label for="proxy-description" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
						Description
					</label>
					<input
						id="proxy-description"
						type="text"
						bind:value={formData.description}
						placeholder="Optional description"
						class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
					/>
				</div>

				<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
					<div>
						<label for="proxy-http" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
							HTTP Proxy
						</label>
						<input
							id="proxy-http"
							type="text"
							bind:value={formData.http_proxy}
							placeholder="e.g., http://proxy.example.com:3128"
							class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
						/>
					</div>
					<div>
						<label for="proxy-https" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
							HTTPS Proxy
						</label>
						<input
							id="proxy-https"
							type="text"
							bind:value={formData.https_proxy}
							placeholder="e.g., http://proxy.example.com:3128"
							class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
						/>
					</div>
				</div>

				<div>
					<label for="proxy-no-proxy" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
						No Proxy
					</label>
					<input
						id="proxy-no-proxy"
						type="text"
						bind:value={formData.no_proxy}
						placeholder="e.g., localhost,127.0.0.1,.internal.example.com"
						class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
					/>
					<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
						Comma separated list of hosts, domains or CIDRs for which the proxy should be bypassed.
					</p>
				</div>

				<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
					<div>
						<label for="proxy-username" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
							Username
						</label>
						<input
							id="proxy-username"
							type="text"
							bind:value={formData.username}
							placeholder="Optional proxy username"
							autocomplete="off"
							class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
						/>
					</div>
					<div>
						<label for="proxy-password" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
							Password
						</label>
						<input
							id="proxy-password"
							type="password"
							bind:value={formData.password}
							placeholder={showEditModal ? 'Leave blank to keep current password' : 'Optional proxy password'}
							autocomplete="new-password"
							class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
						/>
					</div>
				</div>
				{#if showEditModal}
					<p class="text-xs text-gray-500 dark:text-gray-400">
						Leaving the password blank keeps the current password. Clearing the username removes the credentials.
					</p>
				{/if}

				<div class="flex justify-end space-x-3 pt-4 border-t border-gray-200 dark:border-gray-700">
					<button
						type="button"
						on:click={closeModals}
						class="px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 cursor-pointer"
					>
						Cancel
					</button>
					<button
						type="submit"
						disabled={submitting || !isFormValid}
						class="px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
					>
						{#if submitting}
							<div class="flex items-center">
								<div class="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
								{showCreateModal ? 'Creating...' : 'Updating...'}
							</div>
						{:else}
							{showCreateModal ? 'Create Proxy' : 'Update Proxy'}
						{/if}
					</button>
				</div>
			</form>
		</div>
	</Modal>
{/if}

<!-- Delete Proxy Modal -->
{#if showDeleteModal && selectedProxy}
	<DeleteModal
		title="Delete Proxy"
		message="Are you sure you want to delete this proxy? Deletion will fail if the proxy is still in use by pools or scale sets. This action cannot be undone."
		itemName={selectedProxy.name}
		on:close={() => { showDeleteModal = false; selectedProxy = null; }}
		on:confirm={handleDeleteProxy}
	/>
{/if}
