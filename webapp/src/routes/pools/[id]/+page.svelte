<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { garmApi } from '$lib/api/client.js';
	import type { Pool, UpdatePoolParams } from '$lib/api/generated/api.js';
	import { resolve } from '$app/paths';
	import UpdatePoolModal from '$lib/components/UpdatePoolModal.svelte';
	import DeleteModal from '$lib/components/DeleteModal.svelte';
	import InstancesSection from '$lib/components/InstancesSection.svelte';
	import DetailHeader from '$lib/components/DetailHeader.svelte';
	import { websocketStore, type WebSocketEvent } from '$lib/stores/websocket.js';
	import type { Instance } from '$lib/api/generated/api.js';
	import { toastStore } from '$lib/stores/toast.js';
	import { formatDate, getForgeIcon, getEntityName, getEntityType, getEntityUrl } from '$lib/utils/common.js';
	import { extractAPIError } from '$lib/utils/apiError';
	import ForgeTypeCell from '$lib/components/cells/ForgeTypeCell.svelte';

	let pool: Pool | null = null;
	let loading = true;
	let error = '';
	let showUpdateModal = false;
	let showDeleteModal = false;
	let showDeleteInstanceModal = false;
	let selectedInstance: Instance | null = null;
	let unsubscribeWebsocket: (() => void) | null = null;

	$: poolId = page.params.id;

	async function loadPool() {
		if (!poolId) return;
		
		try {
			loading = true;
			error = '';
			pool = await garmApi.getPool(poolId);
			// Ensure instances array always exists for websocket updates
			if (!pool.instances) {
				pool.instances = [];
			}
		} catch (err) {
			error = extractAPIError(err);
		} finally {
			loading = false;
		}
	}

	async function handleUpdate(params: UpdatePoolParams) {
		if (!pool) return;
		try {
			// Update pool and get the updated object from API response
			const updatedPool = await garmApi.updatePool(pool.id!, params);
			// Update the local state directly instead of re-rendering
			pool = updatedPool;
			showUpdateModal = false;
			toastStore.success(
				'Pool Updated',
				`Pool ${pool.id} has been updated successfully.`
			);
		} catch (err) {
			const errorMessage = extractAPIError(err);
			toastStore.error(
				'Update Failed',
				errorMessage
			);
		}
	}

	async function handleDelete() {
		if (!pool) return;
		try {
			await garmApi.deletePool(pool.id!);
			goto(resolve('/pools'));
		} catch (err) {
			const errorMessage = extractAPIError(err);
			toastStore.error(
				'Delete Failed',
				errorMessage
			);
		}
		showDeleteModal = false;
	}

	async function handleDeleteInstance() {
		if (!selectedInstance) return;
		try {
			await garmApi.deleteInstance(selectedInstance.name!);
			toastStore.success(
				'Instance Deleted',
				`Instance ${selectedInstance.name} has been deleted successfully.`
			);
		} catch (err) {
			const errorMessage = extractAPIError(err);
			toastStore.error(
				'Delete Failed',
				errorMessage
			);
		}
		showDeleteInstanceModal = false;
		selectedInstance = null;
	}

	function openDeleteInstanceModal(instance: Instance) {
		selectedInstance = instance;
		showDeleteInstanceModal = true;
	}

	function formatExtraSpecs(extraSpecs: any): string {
		if (!extraSpecs) return '{}';
		try {
			if (typeof extraSpecs === 'string') {
				const parsed = JSON.parse(extraSpecs);
				return JSON.stringify(parsed, null, 2);
			}
			return JSON.stringify(extraSpecs, null, 2);
		} catch (e) {
			return extraSpecs.toString();
		}
	}

	function handlePoolEvent(event: WebSocketEvent) {
		
		if (event.operation === 'update') {
			const updatedPool = event.payload as Pool;
			// Only update if this is the pool we're viewing
			if (pool && updatedPool.id === pool.id) {
				pool = updatedPool;
			}
		} else if (event.operation === 'delete') {
			const deletedPoolId = event.payload.id || event.payload;
			// If this pool was deleted, redirect to pools list
			if (pool && pool.id === deletedPoolId) {
				goto(resolve('/pools'));
			}
		}
	}

	function handleInstanceEvent(event: WebSocketEvent) {
		
		if (!pool) return;
		
		const instance = event.payload;
		// Only handle instances that belong to this pool
		if (instance.pool_id !== pool.id) return;

		// Ensure instances array exists
		if (!pool.instances) {
			pool.instances = [];
		}

		if (event.operation === 'create') {
			// Add new instance to the list
			pool.instances = [...pool.instances, instance];
		} else if (event.operation === 'update') {
			// Update existing instance
			pool.instances = pool.instances.map(inst => 
				inst.id === instance.id ? instance : inst
			);
		} else if (event.operation === 'delete') {
			// Remove deleted instance
			const instanceId = instance.id || instance;
			pool.instances = pool.instances.filter(inst => inst.id !== instanceId);
		}
		
		// Force reactivity
		pool = pool;
	}

	onMount(() => {
		loadPool();
		
		// Subscribe to pool events
		const unsubscribePool = websocketStore.subscribeToEntity(
			'pool',
			['update', 'delete'],
			handlePoolEvent
		);

		// Subscribe to instance events
		const unsubscribeInstance = websocketStore.subscribeToEntity(
			'instance',
			['create', 'update', 'delete'],
			handleInstanceEvent
		);

		// Combine unsubscribe functions
		unsubscribeWebsocket = () => {
			unsubscribePool();
			unsubscribeInstance();
		};
	});

	onDestroy(() => {
		if (unsubscribeWebsocket) {
			unsubscribeWebsocket();
			unsubscribeWebsocket = null;
		}
	});
</script>

<svelte:head>
	<title>{pool ? `Pool ${pool.id} - Pool Details` : 'Pool Details'} - GARM</title>
</svelte:head>

<div class="space-y-6">
	<!-- Breadcrumbs -->
	<nav class="flex" aria-label="Breadcrumb">
		<ol class="inline-flex items-center space-x-1 md:space-x-3">
			<li class="inline-flex items-center">
				<a href={resolve('/pools')} class="inline-flex items-center text-sm font-medium text-gray-700 hover:text-blue-600 dark:text-gray-400 dark:hover:text-white">
					<svg class="w-3 h-3 mr-2.5" fill="currentColor" viewBox="0 0 20 20">
						<path d="M10.707 2.293a1 1 0 00-1.414 0l-7 7a1 1 0 001.414 1.414L4 10.414V17a1 1 0 001 1h2a1 1 0 001-1v-2a1 1 0 011-1h2a1 1 0 011 1v2a1 1 0 001 1h2a1 1 0 001-1v-6.586l.293.293a1 1 0 001.414-1.414l-7-7z"/>
					</svg>
					Pools
				</a>
			</li>
			<li>
				<div class="flex items-center">
					<svg class="w-3 h-3 text-gray-400 mx-1" fill="currentColor" viewBox="0 0 20 20">
						<path fill-rule="evenodd" d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z" clip-rule="evenodd"/>
					</svg>
					<span class="ml-1 text-sm font-medium text-gray-500 md:ml-2 dark:text-gray-400">
						{pool ? pool.id : 'Loading...'}
					</span>
				</div>
			</li>
		</ol>
	</nav>

	{#if loading}
		<div class="p-6 text-center">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600 mx-auto"></div>
			<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">Loading pool...</p>
		</div>
	{:else if error}
		<div class="rounded-md bg-red-50 dark:bg-red-900 p-4">
			<p class="text-sm font-medium text-red-800 dark:text-red-200">{error}</p>
		</div>
	{:else if pool}
		<!-- Header -->
		<DetailHeader
			title={pool.id!}
			subtitle="Pool for {getEntityName(pool)} ({getEntityType(pool)})"
			forgeIcon={getForgeIcon(pool.endpoint?.endpoint_type || 'unknown')}
			onEdit={() => showUpdateModal = true}
			onDelete={() => showDeleteModal = true}
		/>

		<!-- Pool Details -->
		<div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
			<!-- Basic Information -->
			<div class="bg-white dark:bg-gray-800 shadow rounded-lg">
				<div class="px-4 py-5 sm:p-6">
					<h2 class="text-lg font-medium text-gray-900 dark:text-white mb-4">Basic Information</h2>
					<dl class="space-y-4">
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Pool ID</dt>
							<dd class="mt-1 text-sm text-gray-900 dark:text-white font-mono">{pool.id}</dd>
						</div>
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Provider</dt>
							<dd class="mt-1 text-sm text-gray-900 dark:text-white">{pool.provider_name}</dd>
						</div>
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Forge Type</dt>
							<dd class="mt-1">
								<ForgeTypeCell item={pool} />
							</dd>
						</div>
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Image</dt>
							<dd class="mt-1 text-sm text-gray-900 dark:text-white">
								<code class="bg-gray-100 dark:bg-gray-700 px-2 py-1 rounded text-xs">{pool.image}</code>
							</dd>
						</div>
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Flavor</dt>
							<dd class="mt-1 text-sm text-gray-900 dark:text-white">{pool.flavor}</dd>
						</div>
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Status</dt>
							<dd class="mt-1">
								<span class="inline-flex px-2 py-1 text-xs font-medium rounded-full {pool.enabled ? 'bg-green-100 dark:bg-green-900 text-green-800 dark:text-green-200' : 'bg-red-100 dark:bg-red-900 text-red-800 dark:text-red-200'}">
									{pool.enabled ? 'Enabled' : 'Disabled'}
								</span>
							</dd>
						</div>
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Entity</dt>
							<dd class="mt-1">
								<div class="flex items-center space-x-2">
									<span class="inline-flex px-2 py-1 text-xs font-medium rounded-full bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-200">
										{getEntityType(pool)}
									</span>
									<a href={getEntityUrl(pool)} class="text-sm text-blue-600 dark:text-blue-400 hover:text-blue-500 dark:hover:text-blue-300">
										{getEntityName(pool)}
									</a>
								</div>
							</dd>
						</div>
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Created At</dt>
							<dd class="mt-1 text-sm text-gray-900 dark:text-white">{formatDate(pool.created_at || '')}</dd>
						</div>
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Updated At</dt>
							<dd class="mt-1 text-sm text-gray-900 dark:text-white">{formatDate(pool.updated_at || '')}</dd>
						</div>
					</dl>
				</div>
			</div>

			<!-- Configuration -->
			<div class="bg-white dark:bg-gray-800 shadow rounded-lg">
				<div class="px-4 py-5 sm:p-6">
					<h2 class="text-lg font-medium text-gray-900 dark:text-white mb-4">Configuration</h2>
					<dl class="space-y-4">
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Max Runners</dt>
							<dd class="mt-1 text-sm text-gray-900 dark:text-white">{pool.max_runners}</dd>
						</div>
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Min Idle Runners</dt>
							<dd class="mt-1 text-sm text-gray-900 dark:text-white">{pool.min_idle_runners}</dd>
						</div>
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Bootstrap Timeout</dt>
							<dd class="mt-1 text-sm text-gray-900 dark:text-white">{pool.runner_bootstrap_timeout} minutes</dd>
						</div>
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Priority</dt>
							<dd class="mt-1 text-sm text-gray-900 dark:text-white">{pool.priority}</dd>
						</div>
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Runner Prefix</dt>
							<dd class="mt-1 text-sm text-gray-900 dark:text-white">{pool.runner_prefix || 'garm'}</dd>
						</div>
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">OS Type / Architecture</dt>
							<dd class="mt-1 text-sm text-gray-900 dark:text-white">{pool.os_type} / {pool.os_arch}</dd>
						</div>
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Shell Access</dt>
							<dd class="mt-1">
								<span class="inline-flex px-2 py-1 text-xs font-medium rounded-full {pool.enable_shell ? 'bg-blue-100 dark:bg-blue-900 text-blue-800 dark:text-blue-200' : 'bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-200'}">
									{pool.enable_shell ? 'Enabled' : 'Disabled'}
								</span>
							</dd>
						</div>
						{#if (pool as any).template_name}
							<div>
								<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Runner Install Template</dt>
								<dd class="mt-1">
									<a href={resolve(`/templates/${(pool as any).template_id}`)} class="text-sm text-blue-600 dark:text-blue-400 hover:text-blue-500 dark:hover:text-blue-300">
										{(pool as any).template_name}
									</a>
								</dd>
							</div>
						{/if}
						{#if pool['github-runner-group']}
							<div>
								<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">GitHub Runner Group</dt>
								<dd class="mt-1 text-sm text-gray-900 dark:text-white">{pool['github-runner-group']}</dd>
							</div>
						{/if}
						{#if pool.tags && pool.tags.length > 0}
							<div>
								<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Tags</dt>
								<dd class="mt-1">
									<div class="flex flex-wrap gap-2">
										{#each pool.tags as tag}
											<span class="inline-flex items-center px-2 py-1 rounded-full text-xs font-medium bg-blue-100 dark:bg-blue-900 text-blue-800 dark:text-blue-200">
												{typeof tag === 'string' ? tag : tag.name}
											</span>
										{/each}
									</div>
								</dd>
							</div>
						{/if}
					</dl>
				</div>
			</div>
		</div>

		<!-- Extra Specs -->
		{#if pool.extra_specs}
			<div class="bg-white dark:bg-gray-800 shadow rounded-lg">
				<div class="px-4 py-5 sm:p-6">
					<h2 class="text-lg font-medium text-gray-900 dark:text-white mb-4">Extra Specifications</h2>
					<pre class="bg-gray-100 dark:bg-gray-700 p-4 rounded-md overflow-x-auto text-sm text-gray-900 dark:text-white font-mono">{formatExtraSpecs(pool.extra_specs)}</pre>
				</div>
			</div>
		{/if}

		<!-- Instances -->
		<InstancesSection instances={pool.instances || []} entityType="pool" onDeleteInstance={openDeleteInstanceModal} />

	{/if}
</div>

<!-- Modals -->
{#if showUpdateModal && pool}
	<UpdatePoolModal
		{pool}
		on:close={() => showUpdateModal = false}
		on:submit={(e) => handleUpdate(e.detail)}
	/>
{/if}

{#if showDeleteModal && pool}
	<DeleteModal
		title="Delete Pool"
		message="Are you sure you want to delete this pool? This action cannot be undone and will remove all associated runners."
		itemName={`Pool ${pool.id} (${getEntityName(pool)})`}
		on:close={() => showDeleteModal = false}
		on:confirm={handleDelete}
	/>
{/if}

{#if showDeleteInstanceModal && selectedInstance}
	<DeleteModal
		title="Delete Instance"
		message="Are you sure you want to delete this instance? This action cannot be undone."
		itemName={selectedInstance.name}
		on:close={() => { showDeleteInstanceModal = false; selectedInstance = null; }}
		on:confirm={handleDeleteInstance}
	/>
{/if}