<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { garmApi } from '$lib/api/client.js';
	import type { ScaleSet, CreateScaleSetParams } from '$lib/api/generated/api.js';
	import { resolve } from '$app/paths';
	import UpdateScaleSetModal from '$lib/components/UpdateScaleSetModal.svelte';
	import DeleteModal from '$lib/components/DeleteModal.svelte';
	import InstancesSection from '$lib/components/InstancesSection.svelte';
	import DetailHeader from '$lib/components/DetailHeader.svelte';
	import { websocketStore, type WebSocketEvent } from '$lib/stores/websocket.js';
	import { toastStore } from '$lib/stores/toast.js';
	import type { Instance } from '$lib/api/generated/api.js';
	import { extractAPIError } from '$lib/utils/apiError';
	import { formatDate, getForgeIcon, getEntityName, getEntityType, getEntityUrl } from '$lib/utils/common.js';

	let scaleSet: ScaleSet | null = null;
	let loading = true;
	let error = '';
	let showUpdateModal = false;
	let showDeleteModal = false;
	let showDeleteInstanceModal = false;
	let selectedInstance: Instance | null = null;
	let unsubscribeWebsocket: (() => void) | null = null;

	$: scaleSetId = parseInt($page.params.id || '0');

	async function loadScaleSet() {
		if (!scaleSetId || isNaN(scaleSetId)) return;
		
		try {
			loading = true;
			error = '';
			scaleSet = await garmApi.getScaleSet(scaleSetId);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load scale set';
		} finally {
			loading = false;
		}
	}

	async function handleUpdate(params: Partial<CreateScaleSetParams>) {
		if (!scaleSet) return;
		try {
			await garmApi.updateScaleSet(scaleSet.id!, params);
			await loadScaleSet();
			toastStore.success(
				'Scale Set Updated',
				`Scale Set ${scaleSet.name} has been updated successfully.`
			);
			showUpdateModal = false;
		} catch (err) {
			throw err; // Let the modal handle the error
		}
	}

	async function handleDelete() {
		if (!scaleSet) return;
		try {
			await garmApi.deleteScaleSet(scaleSet.id!);
			goto(resolve('/scalesets'));
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

	function handleScaleSetEvent(event: WebSocketEvent) {
		
		if (event.operation === 'update') {
			const updatedScaleSet = event.payload as ScaleSet;
			// Only update if this is the scale set we're viewing
			if (scaleSet && updatedScaleSet.id === scaleSet.id) {
				scaleSet = updatedScaleSet;
			}
		} else if (event.operation === 'delete') {
			const deletedScaleSetId = event.payload.id || event.payload;
			// If this scale set was deleted, redirect to scale sets list
			if (scaleSet && scaleSet.id === deletedScaleSetId) {
				goto(resolve('/scalesets'));
			}
		}
	}

	function handleInstanceEvent(event: WebSocketEvent) {
		
		if (!scaleSet || !scaleSet.instances) return;
		
		const instance = event.payload as Instance;
		// Only handle instances that belong to this scale set
		if (instance.scale_set_id !== scaleSet.id) return;

		if (event.operation === 'create') {
			// Add new instance to the list
			scaleSet.instances = [...scaleSet.instances, instance];
		} else if (event.operation === 'update') {
			// Update existing instance
			scaleSet.instances = scaleSet.instances.map(inst => 
				inst.id === instance.id ? instance : inst
			);
		} else if (event.operation === 'delete') {
			// Remove deleted instance
			const instanceId = instance.id || instance;
			scaleSet.instances = scaleSet.instances.filter(inst => inst.id !== instanceId);
		}
		
		// Force reactivity
		scaleSet = scaleSet;
	}


	onMount(() => {
		loadScaleSet();

		// Subscribe to scaleSet events
		const unsubscribeScaleSet = websocketStore.subscribeToEntity(
			'scaleset',
			['update', 'delete'],
			handleScaleSetEvent
		);

		// Subscribe to instance events
		const unsubscribeInstance = websocketStore.subscribeToEntity(
			'instance',
			['create', 'update', 'delete'],
			handleInstanceEvent
		);

		// Combine unsubscribe functions
		unsubscribeWebsocket = () => {
			unsubscribeScaleSet();
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
	<title>{scaleSet ? `${scaleSet.name} - Scale Set Details` : 'Scale Set Details'} - GARM</title>
</svelte:head>

<div class="space-y-6">
	<!-- Breadcrumbs -->
	<nav class="flex" aria-label="Breadcrumb">
		<ol class="inline-flex items-center space-x-1 md:space-x-3">
			<li class="inline-flex items-center">
				<a href={resolve('/scalesets')} class="inline-flex items-center text-sm font-medium text-gray-700 hover:text-blue-600 dark:text-gray-400 dark:hover:text-white">
					<svg class="w-3 h-3 mr-2.5" fill="currentColor" viewBox="0 0 20 20">
						<path d="M10.707 2.293a1 1 0 00-1.414 0l-7 7a1 1 0 001.414 1.414L4 10.414V17a1 1 0 001 1h2a1 1 0 001-1v-2a1 1 0 011-1h2a1 1 0 011 1v2a1 1 0 001 1h2a1 1 0 001-1v-6.586l.293.293a1 1 0 001.414-1.414l-7-7z"/>
					</svg>
					Scale Sets
				</a>
			</li>
			<li>
				<div class="flex items-center">
					<svg class="w-3 h-3 text-gray-400 mx-1" fill="currentColor" viewBox="0 0 20 20">
						<path fill-rule="evenodd" d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z" clip-rule="evenodd"/>
					</svg>
					<span class="ml-1 text-sm font-medium text-gray-500 md:ml-2 dark:text-gray-400">
						{scaleSet ? scaleSet.name : 'Loading...'}
					</span>
				</div>
			</li>
		</ol>
	</nav>

	{#if loading}
		<div class="p-6 text-center">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600 mx-auto"></div>
			<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">Loading scale set...</p>
		</div>
	{:else if error}
		<div class="rounded-md bg-red-50 dark:bg-red-900 p-4">
			<p class="text-sm font-medium text-red-800 dark:text-red-200">{error}</p>
		</div>
	{:else if scaleSet}
		<!-- Header -->
		<DetailHeader
			title={scaleSet.name || 'Scale Set'}
			subtitle="Scale set for {getEntityName(scaleSet)} ({getEntityType(scaleSet)}) • GitHub Runner Scale Set"
			forgeIcon={getForgeIcon('github')}
			onEdit={() => showUpdateModal = true}
			onDelete={() => showDeleteModal = true}
		/>

		<!-- Scale Set Details -->
		<div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
			<!-- Basic Information -->
			<div class="bg-white dark:bg-gray-800 shadow rounded-lg">
				<div class="px-4 py-5 sm:p-6">
					<h2 class="text-lg font-medium text-gray-900 dark:text-white mb-4">Basic Information</h2>
					<dl class="space-y-4">
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Scale Set ID</dt>
							<dd class="mt-1 text-sm text-gray-900 dark:text-white">{scaleSet.id}</dd>
						</div>
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Name</dt>
							<dd class="mt-1 text-sm text-gray-900 dark:text-white font-medium">{scaleSet.name}</dd>
						</div>
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Provider</dt>
							<dd class="mt-1 text-sm text-gray-900 dark:text-white">{scaleSet.provider_name}</dd>
						</div>
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Image</dt>
							<dd class="mt-1 text-sm text-gray-900 dark:text-white">
								<code class="bg-gray-100 dark:bg-gray-700 px-2 py-1 rounded text-xs">{scaleSet.image}</code>
							</dd>
						</div>
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Flavor</dt>
							<dd class="mt-1 text-sm text-gray-900 dark:text-white">{scaleSet.flavor}</dd>
						</div>
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Status</dt>
							<dd class="mt-1">
								<span class="inline-flex px-2 py-1 text-xs font-medium rounded-full {scaleSet.enabled ? 'bg-green-100 dark:bg-green-900 text-green-800 dark:text-green-200' : 'bg-red-100 dark:bg-red-900 text-red-800 dark:text-red-200'}">
									{scaleSet.enabled ? 'Enabled' : 'Disabled'}
								</span>
							</dd>
						</div>
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Entity</dt>
							<dd class="mt-1">
								<div class="flex items-center space-x-2">
									<span class="inline-flex px-2 py-1 text-xs font-medium rounded-full bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-200">
										{getEntityType(scaleSet)}
									</span>
									<a href={getEntityUrl(scaleSet)} class="text-sm text-blue-600 dark:text-blue-400 hover:text-blue-500 dark:hover:text-blue-300">
										{getEntityName(scaleSet)}
									</a>
								</div>
							</dd>
						</div>
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Created At</dt>
							<dd class="mt-1 text-sm text-gray-900 dark:text-white">{formatDate(scaleSet.created_at || '')}</dd>
						</div>
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Updated At</dt>
							<dd class="mt-1 text-sm text-gray-900 dark:text-white">{formatDate(scaleSet.updated_at|| '')}</dd>
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
							<dd class="mt-1 text-sm text-gray-900 dark:text-white">{scaleSet.max_runners}</dd>
						</div>
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Min Idle Runners</dt>
							<dd class="mt-1 text-sm text-gray-900 dark:text-white">{scaleSet.min_idle_runners}</dd>
						</div>
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Bootstrap Timeout</dt>
							<dd class="mt-1 text-sm text-gray-900 dark:text-white">{scaleSet.runner_bootstrap_timeout} minutes</dd>
						</div>
						<!-- Priority is not available in ScaleSet API -->
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Runner Prefix</dt>
							<dd class="mt-1 text-sm text-gray-900 dark:text-white">{scaleSet.runner_prefix || 'garm'}</dd>
						</div>
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">OS Type / Architecture</dt>
							<dd class="mt-1 text-sm text-gray-900 dark:text-white">{scaleSet.os_type} / {scaleSet.os_arch}</dd>
						</div>
						{#if scaleSet['github-runner-group']}
							<div>
								<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">GitHub Runner Group</dt>
								<dd class="mt-1 text-sm text-gray-900 dark:text-white">{scaleSet['github-runner-group']}</dd>
							</div>
						{/if}
						<!-- Tags are not available in ScaleSet API -->
					</dl>
				</div>
			</div>
		</div>


		<!-- Extra Specs -->
		{#if scaleSet.extra_specs}
			<div class="bg-white dark:bg-gray-800 shadow rounded-lg">
				<div class="px-4 py-5 sm:p-6">
					<h2 class="text-lg font-medium text-gray-900 dark:text-white mb-4">Extra Specifications</h2>
					<pre class="bg-gray-100 dark:bg-gray-700 p-4 rounded-md overflow-x-auto text-sm text-gray-900 dark:text-white font-mono">{formatExtraSpecs(scaleSet.extra_specs)}</pre>
				</div>
			</div>
		{/if}

		<!-- Instances -->
		{#if scaleSet.instances}
			<InstancesSection instances={scaleSet.instances} entityType="scaleset" onDeleteInstance={openDeleteInstanceModal} />
		{/if}

	{/if}
</div>

<!-- Modals -->
{#if showUpdateModal && scaleSet}
	<UpdateScaleSetModal
		{scaleSet}
		on:close={() => showUpdateModal = false}
		on:submit={(e) => handleUpdate(e.detail)}
	/>
{/if}

{#if showDeleteModal && scaleSet}
	<DeleteModal
		title="Delete Scale Set"
		message="Are you sure you want to delete this scale set? This action cannot be undone and will remove all associated runners."
		itemName={`Scale Set ${scaleSet.name} (${getEntityName(scaleSet)})`}
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