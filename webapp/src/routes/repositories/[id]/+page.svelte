<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { garmApi } from '$lib/api/client.js';
	import type { Repository, Pool, Instance } from '$lib/api/generated/api.js';
	import { base } from '$app/paths';
	import UpdateEntityModal from '$lib/components/UpdateEntityModal.svelte';
	import DeleteModal from '$lib/components/DeleteModal.svelte';
	import EntityInformation from '$lib/components/EntityInformation.svelte';
	import DetailHeader from '$lib/components/DetailHeader.svelte';
	import PoolsSection from '$lib/components/PoolsSection.svelte';
	import { getForgeIcon } from '$lib/utils/common.js';
	import InstancesSection from '$lib/components/InstancesSection.svelte';
	import EventsSection from '$lib/components/EventsSection.svelte';
	import WebhookSection from '$lib/components/WebhookSection.svelte';
	import CreatePoolModal from '$lib/components/CreatePoolModal.svelte';
	import type { CreatePoolParams } from '$lib/api/generated/api.js';
	import { websocketStore, type WebSocketEvent } from '$lib/stores/websocket.js';
	import { toastStore } from '$lib/stores/toast.js';

	let repository: Repository | null = null;
	let pools: Pool[] = [];
	let instances: Instance[] = [];
	let loading = true;
	let error = '';
	let showUpdateModal = false;
	let showDeleteModal = false;
	let showDeleteInstanceModal = false;
	let showCreatePoolModal = false;
	let selectedInstance: Instance | null = null;
	let unsubscribeWebsocket: (() => void) | null = null;
	let eventsContainer: HTMLElement;

	$: repositoryId = $page.params.id;

	async function loadRepository() {
		if (!repositoryId) return;
		
		try {
			loading = true;
			error = '';
			
			const [repo, repoPools, repoInstances] = await Promise.all([
				garmApi.getRepository(repositoryId),
				garmApi.listRepositoryPools(repositoryId).catch(() => []),
				garmApi.listRepositoryInstances(repositoryId).catch(() => [])
			]);
			
			repository = repo;
			pools = repoPools;
			instances = repoInstances;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load repository';
		} finally {
			loading = false;
		}
	}

	function updateEntityFields(currentEntity: any, updatedFields: any): any {
		// Preserve only fields that are definitely not in the API response
		const { events: originalEvents } = currentEntity;
		
		// Use the API response as the primary source, add back preserved fields
		const result = {
			...updatedFields,
			events: originalEvents // Always preserve events since they're managed by websockets
		};
		
		return result;
	}

	async function handleUpdate(params: any) {
		if (!repository) return;
		try {
			// Update repository
			await garmApi.updateRepository(repository.id!, params);
			
			// Reload fresh data to ensure UI is up to date
			await loadRepository();
			
			toastStore.success(
				'Repository Updated',
				`Repository ${repository.owner}/${repository.name} has been updated successfully.`
			);
			showUpdateModal = false;
		} catch (err) {
			throw err; // Let the modal handle the error
		}
	}

	async function handleDelete() {
		if (!repository) return;
		try {
			await garmApi.deleteRepository(repository.id!);
			goto(`${base}/repositories`);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to delete repository';
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
			// No need to reload - websocket events will update the UI automatically
			showDeleteInstanceModal = false;
			selectedInstance = null;
		} catch (err) {
			const errorMessage = err instanceof Error ? err.message : 'Failed to delete instance';
			toastStore.error(
				'Delete Failed',
				errorMessage
			);
			showDeleteInstanceModal = false;
			selectedInstance = null;
		}
	}

	function openDeleteInstanceModal(instance: Instance) {
		selectedInstance = instance;
		showDeleteInstanceModal = true;
	}

	function handleAddPool() {
		showCreatePoolModal = true;
	}

	async function handleCreatePool(event: CustomEvent<CreatePoolParams>) {
		try {
			if (!repository) return;
			
			await garmApi.createRepositoryPool(repository.id!, event.detail);
			toastStore.success(
				'Pool Created',
				`Pool has been created successfully for repository ${repository.owner}/${repository.name}.`
			);
			showCreatePoolModal = false;
			// Pool will be updated via websocket, so no need to reload manually
		} catch (err) {
			throw err; // Let the modal handle the error
		}
	}

	function scrollToBottomEvents() {
		if (eventsContainer) {
			eventsContainer.scrollTop = eventsContainer.scrollHeight;
		}
	}

	function handleRepositoryEvent(event: WebSocketEvent) {
		
		if (event.operation === 'update') {
			const updatedRepository = event.payload as Repository;
			// Only update if this is the repository we're viewing
			if (repository && updatedRepository.id === repository.id) {
				// Check if events have been updated
				const oldEventCount = repository.events?.length || 0;
				const newEventCount = updatedRepository.events?.length || 0;
				
				// Update repository using selective field updates
				repository = updateEntityFields(repository, updatedRepository);
				
				// Auto-scroll if new events were added
				if (newEventCount > oldEventCount) {
					// Use setTimeout to ensure the DOM has updated
					setTimeout(() => {
						scrollToBottomEvents();
					}, 100);
				}
			}
		} else if (event.operation === 'delete') {
			const deletedRepositoryId = event.payload.id || event.payload;
			// If this repository was deleted, redirect to repositories list
			if (repository && repository.id === deletedRepositoryId) {
				goto(`${base}/repositories`);
			}
		}
	}

	function handlePoolEvent(event: WebSocketEvent) {
		
		if (!repository) return;
		
		const pool = event.payload;
		// Only handle pools that belong to this repository
		if (pool.repo_id !== repository.id) return;

		if (event.operation === 'create') {
			// Add new pool to the list
			pools = [...pools, pool];
		} else if (event.operation === 'update') {
			// Update existing pool
			pools = pools.map(p => 
				p.id === pool.id ? pool : p
			);
		} else if (event.operation === 'delete') {
			// Remove deleted pool
			const poolId = pool.id || pool;
			pools = pools.filter(p => p.id !== poolId);
		}
	}

	function handleInstanceEvent(event: WebSocketEvent) {
		
		if (!repository || !pools) return;
		
		const instance = event.payload;
		// Check if instance belongs to any pool that belongs to this repository
		const belongsToRepository = pools.some(pool => pool.id === instance.pool_id);
		if (!belongsToRepository) return;

		if (event.operation === 'create') {
			// Add new instance to the list
			instances = [...instances, instance];
		} else if (event.operation === 'update') {
			// Update existing instance
			instances = instances.map(inst => 
				inst.id === instance.id ? instance : inst
			);
		} else if (event.operation === 'delete') {
			// Remove deleted instance
			const instanceId = instance.id || instance;
			instances = instances.filter(inst => inst.id !== instanceId);
		}
	}

	onMount(() => {
		loadRepository().then(() => {
			// Scroll to bottom on initial load if there are events
			if (repository?.events?.length) {
				setTimeout(() => {
					scrollToBottomEvents();
				}, 100);
			}
		});
		
		// Subscribe to repository events
		const unsubscribeRepo = websocketStore.subscribeToEntity(
			'repository',
			['update', 'delete'],
			handleRepositoryEvent
		);

		// Subscribe to pool events
		const unsubscribePool = websocketStore.subscribeToEntity(
			'pool',
			['create', 'update', 'delete'],
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
			unsubscribeRepo();
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
	<title>{repository ? `${repository.name} - Repository Details` : 'Repository Details'} - GARM</title>
</svelte:head>

<div class="space-y-6">
	<!-- Breadcrumbs -->
	<nav class="flex" aria-label="Breadcrumb">
		<ol class="inline-flex items-center space-x-1 md:space-x-3">
			<li class="inline-flex items-center">
				<a href={`${base}/repositories`} class="inline-flex items-center text-sm font-medium text-gray-700 hover:text-blue-600 dark:text-gray-400 dark:hover:text-white">
					<svg class="w-3 h-3 mr-2.5" fill="currentColor" viewBox="0 0 20 20">
						<path d="M10.707 2.293a1 1 0 00-1.414 0l-7 7a1 1 0 001.414 1.414L4 10.414V17a1 1 0 001 1h2a1 1 0 001-1v-2a1 1 0 011-1h2a1 1 0 011 1v2a1 1 0 001 1h2a1 1 0 001-1v-6.586l.293.293a1 1 0 001.414-1.414l-7-7z"/>
					</svg>
					Repositories
				</a>
			</li>
			<li>
				<div class="flex items-center">
					<svg class="w-3 h-3 text-gray-400 mx-1" fill="currentColor" viewBox="0 0 20 20">
						<path fill-rule="evenodd" d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z" clip-rule="evenodd"/>
					</svg>
					<span class="ml-1 text-sm font-medium text-gray-500 md:ml-2 dark:text-gray-400">
						{repository ? repository.name : 'Loading...'}
					</span>
				</div>
			</li>
		</ol>
	</nav>

	{#if loading}
		<div class="p-6 text-center">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600 mx-auto"></div>
			<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">Loading repository...</p>
		</div>
	{:else if error}
		<div class="rounded-md bg-red-50 dark:bg-red-900 p-4">
			<p class="text-sm font-medium text-red-800 dark:text-red-200">{error}</p>
		</div>
	{:else if repository}
		<!-- Header -->
		<DetailHeader
			title={repository.name || 'Repository'}
			subtitle="Owner: {repository.owner} • Endpoint: {repository.endpoint?.name}"
			forgeIcon={getForgeIcon(repository.endpoint?.endpoint_type || 'unknown')}
			onEdit={() => showUpdateModal = true}
			onDelete={() => showDeleteModal = true}
		/>

		<!-- Repository Details -->
		<EntityInformation entity={repository} entityType="repository" />

		<!-- Webhook Status -->
		<WebhookSection 
			entityType="repository" 
			entityId={repository.id || ''} 
			entityName="{repository.owner}/{repository.name}" 
		/>

		<!-- Pools -->
		<PoolsSection 
			{pools} 
			entityType="repository" 
			entityId={repository.id || ''}
			entityName="{repository.owner}/{repository.name}"
			on:addPool={handleAddPool}
		/>

		<!-- Instances -->
		<InstancesSection {instances} entityType="repository" onDeleteInstance={openDeleteInstanceModal} />

		<!-- Events -->
		<EventsSection events={repository?.events} bind:eventsContainer />
	{/if}
</div>

<!-- Modals -->
{#if showUpdateModal && repository}
	<UpdateEntityModal
		entity={repository}
		entityType="repository"
		on:close={() => showUpdateModal = false}
		on:submit={(e) => handleUpdate(e.detail)}
	/>
{/if}

{#if showDeleteModal && repository}
	<DeleteModal
		title="Delete Repository"
		message="Are you sure you want to delete this repository? This action cannot be undone and will remove all associated pools and instances."
		itemName={`${repository.owner}/${repository.name}`}
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

{#if showCreatePoolModal && repository}
	<CreatePoolModal
		initialEntityType="repository"
		initialEntityId={repository.id || ''}
		on:close={() => showCreatePoolModal = false}
		on:submit={handleCreatePool}
	/>
{/if}