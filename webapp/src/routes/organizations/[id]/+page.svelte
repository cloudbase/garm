<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { garmApi } from '$lib/api/client.js';
	import type { Organization, Pool, Instance } from '$lib/api/generated/api.js';
	import { base } from '$app/paths';
	import UpdateEntityModal from '$lib/components/UpdateEntityModal.svelte';
	import DeleteModal from '$lib/components/DeleteModal.svelte';
	import EntityInformation from '$lib/components/EntityInformation.svelte';
	import DetailHeader from '$lib/components/DetailHeader.svelte';
	import { getForgeIcon } from '$lib/utils/common.js';
	import PoolsSection from '$lib/components/PoolsSection.svelte';
	import InstancesSection from '$lib/components/InstancesSection.svelte';
	import EventsSection from '$lib/components/EventsSection.svelte';
	import WebhookSection from '$lib/components/WebhookSection.svelte';
	import CreatePoolModal from '$lib/components/CreatePoolModal.svelte';
	import type { CreatePoolParams } from '$lib/api/generated/api.js';
	import { websocketStore, type WebSocketEvent } from '$lib/stores/websocket.js';
	import { toastStore } from '$lib/stores/toast.js';

	let organization: Organization | null = null;
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

	$: organizationId = $page.params.id;

	async function loadOrganization() {
		if (!organizationId) return;
		
		try {
			loading = true;
			error = '';
			
			const [org, orgPools, orgInstances] = await Promise.all([
				garmApi.getOrganization(organizationId),
				garmApi.listOrganizationPools(organizationId).catch(() => []),
				garmApi.listOrganizationInstances(organizationId).catch(() => [])
			]);
			
			organization = org;
			pools = orgPools;
			instances = orgInstances;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load organization';
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
		if (!organization) return;
		try {
			// Update organization
			await garmApi.updateOrganization(organization.id!, params);
			
			// Reload fresh data to ensure UI is up to date
			await loadOrganization();
			
			toastStore.success(
				'Organization Updated',
				`Organization ${organization.name} has been updated successfully.`
			);
			showUpdateModal = false;
		} catch (err) {
			throw err; // Let the modal handle the error
		}
	}

	async function handleDelete() {
		if (!organization) return;
		try {
			await garmApi.deleteOrganization(organization.id!);
			goto(`${base}/organizations`);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to delete organization';
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
			if (!organization) return;
			
			await garmApi.createOrganizationPool(organization.id!, event.detail);
			toastStore.success(
				'Pool Created',
				`Pool has been created successfully for organization ${organization.name}.`
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

	function handleOrganizationEvent(event: WebSocketEvent) {
		
		if (event.operation === 'update') {
			const updatedOrganization = event.payload as Organization;
			// Only update if this is the organization we're viewing
			if (organization && updatedOrganization.id === organization.id) {
				// Check if events have been updated
				const oldEventCount = organization.events?.length || 0;
				const newEventCount = updatedOrganization.events?.length || 0;
				
				// Update organization using selective field updates
				organization = updateEntityFields(organization, updatedOrganization);
				
				// Auto-scroll if new events were added
				if (newEventCount > oldEventCount) {
					// Use setTimeout to ensure the DOM has updated
					setTimeout(() => {
						scrollToBottomEvents();
					}, 100);
				}
			}
		} else if (event.operation === 'delete') {
			const deletedOrganizationId = event.payload.id || event.payload;
			// If this organization was deleted, redirect to organizations list
			if (organization && organization.id === deletedOrganizationId) {
				goto(`${base}/organizations`);
			}
		}
	}

	function handlePoolEvent(event: WebSocketEvent) {
		
		if (!organization) return;
		
		const pool = event.payload;
		// Only handle pools that belong to this organization
		if (pool.org_id !== organization.id) return;

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
		
		if (!organization || !pools) return;
		
		const instance = event.payload;
		// Check if instance belongs to any pool that belongs to this organization
		const belongsToOrganization = pools.some(pool => pool.id === instance.pool_id);
		if (!belongsToOrganization) return;

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
		loadOrganization().then(() => {
			// Scroll to bottom on initial load if there are events
			if (organization?.events?.length) {
				setTimeout(() => {
					scrollToBottomEvents();
				}, 100);
			}
		});
		
		// Subscribe to organization events
		const unsubscribeOrg = websocketStore.subscribeToEntity(
			'organization',
			['update', 'delete'],
			handleOrganizationEvent
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
			unsubscribeOrg();
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
	<title>{organization ? `${organization.name} - Organization Details` : 'Organization Details'} - GARM</title>
</svelte:head>

<div class="space-y-6">
	<!-- Breadcrumbs -->
	<nav class="flex" aria-label="Breadcrumb">
		<ol class="inline-flex items-center space-x-1 md:space-x-3">
			<li class="inline-flex items-center">
				<a href={`${base}/organizations`} class="inline-flex items-center text-sm font-medium text-gray-700 hover:text-blue-600 dark:text-gray-400 dark:hover:text-white">
					<svg class="w-3 h-3 mr-2.5" fill="currentColor" viewBox="0 0 20 20">
						<path d="M10.707 2.293a1 1 0 00-1.414 0l-7 7a1 1 0 001.414 1.414L4 10.414V17a1 1 0 001 1h2a1 1 0 001-1v-2a1 1 0 011-1h2a1 1 0 011 1v2a1 1 0 001 1h2a1 1 0 001-1v-6.586l.293.293a1 1 0 001.414-1.414l-7-7z"/>
					</svg>
					Organizations
				</a>
			</li>
			<li>
				<div class="flex items-center">
					<svg class="w-3 h-3 text-gray-400 mx-1" fill="currentColor" viewBox="0 0 20 20">
						<path fill-rule="evenodd" d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z" clip-rule="evenodd"/>
					</svg>
					<span class="ml-1 text-sm font-medium text-gray-500 md:ml-2 dark:text-gray-400">
						{organization ? organization.name : 'Loading...'}
					</span>
				</div>
			</li>
		</ol>
	</nav>

	{#if loading}
		<div class="p-6 text-center">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600 mx-auto"></div>
			<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">Loading organization...</p>
		</div>
	{:else if error}
		<div class="rounded-md bg-red-50 dark:bg-red-900 p-4">
			<p class="text-sm font-medium text-red-800 dark:text-red-200">{error}</p>
		</div>
	{:else if organization}
		<!-- Header -->
		<DetailHeader
			title={organization.name || 'Organization'}
			subtitle="Endpoint: {organization.endpoint?.name}"
			forgeIcon={getForgeIcon(organization.endpoint?.endpoint_type || 'unknown')}
			onEdit={() => showUpdateModal = true}
			onDelete={() => showDeleteModal = true}
		/>

		<!-- Organization Details -->
		<EntityInformation entity={organization} entityType="organization" />

		<!-- Webhook Status -->
		<WebhookSection 
			entityType="organization" 
			entityId={organization.id || ''} 
			entityName={organization.name || ''} 
		/>

		<!-- Pools -->
		<PoolsSection 
			{pools} 
			entityType="organization" 
			entityId={organization.id || ''}
			entityName={organization.name || ''}
			on:addPool={handleAddPool}
		/>

		<!-- Instances -->
		<InstancesSection {instances} entityType="organization" onDeleteInstance={openDeleteInstanceModal} />

		<!-- Events -->
		<EventsSection events={organization?.events} bind:eventsContainer />
	{/if}
</div>

<!-- Modals -->
{#if showUpdateModal && organization}
	<UpdateEntityModal
		entity={organization}
		entityType="organization"
		on:close={() => showUpdateModal = false}
		on:submit={(e) => handleUpdate(e.detail)}
	/>
{/if}

{#if showDeleteModal && organization}
	<DeleteModal
		title="Delete Organization"
		message="Are you sure you want to delete this organization? This action cannot be undone and will remove all associated pools and instances."
		itemName={organization.name}
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

{#if showCreatePoolModal && organization}
	<CreatePoolModal
		initialEntityType="organization"
		initialEntityId={organization.id || ''}
		on:close={() => showCreatePoolModal = false}
		on:submit={handleCreatePool}
	/>
{/if}