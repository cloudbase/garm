<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { garmApi } from '$lib/api/client.js';
	import type { ForgeInstance, Pool, Instance } from '$lib/api/generated/api.js';
	import { resolve } from '$app/paths';
	import UpdateEntityModal from '$lib/components/UpdateEntityModal.svelte';
	import DeleteModal from '$lib/components/DeleteModal.svelte';
	import EntityInformation from '$lib/components/EntityInformation.svelte';
	import DetailHeader from '$lib/components/DetailHeader.svelte';
	import PoolsSection from '$lib/components/PoolsSection.svelte';
	import InstancesSection from '$lib/components/InstancesSection.svelte';
	import EventsSection from '$lib/components/EventsSection.svelte';
	import WebhookSection from '$lib/components/WebhookSection.svelte';
	import { getForgeIcon, updateEntityFields } from '$lib/utils/common.js';
	import { websocketStore, type WebSocketEvent } from '$lib/stores/websocket.js';
	import { toastStore } from '$lib/stores/toast.js';
	import CreatePoolModal from '$lib/components/CreatePoolModal.svelte';
	import { extractAPIError } from '$lib/utils/apiError';
	import type { CreatePoolParams } from '$lib/api/generated/api.js';

	let forgeInstance: ForgeInstance | null = null;
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

	$: forgeInstanceId = $page.params.id;

	async function loadForgeInstance() {
		if (!forgeInstanceId) return;

		try {
			loading = true;
			error = '';

			const [fi, fiPools, fiInstances] = await Promise.all([
				garmApi.getForgeInstance(forgeInstanceId),
				garmApi.listForgeInstancePools(forgeInstanceId).catch(() => []),
				garmApi.listForgeInstanceInstances(forgeInstanceId).catch(() => [])
			]);

			forgeInstance = fi;
			pools = fiPools;
			instances = fiInstances;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load forge instance';
		} finally {
			loading = false;
		}
	}

	async function handleUpdate(params: any) {
		if (!forgeInstance) return;
		try {
			await garmApi.updateForgeInstance(forgeInstance.id!, params);
			await loadForgeInstance();

			toastStore.success(
				'Forge Instance Updated',
				`Forge instance has been updated successfully.`
			);
			showUpdateModal = false;
		} catch (err) {
			toastStore.error('Update Failed', extractAPIError(err));
		}
	}

	async function handleDelete() {
		if (!forgeInstance) return;
		try {
			await garmApi.deleteForgeInstance(forgeInstance.id!);
			goto(resolve('/forge-instances'));
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
			showDeleteInstanceModal = false;
			selectedInstance = null;
		} catch (err) {
			const errorMessage = extractAPIError(err);
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
			if (!forgeInstance) return;

			await garmApi.createForgeInstancePool(forgeInstance.id!, event.detail);
			toastStore.success(
				'Pool Created',
				`Pool has been created successfully for forge instance.`
			);
			showCreatePoolModal = false;
		} catch (err) {
			const errorMessage = extractAPIError(err);
			toastStore.error(
				'Pool Creation Failed',
				errorMessage
			);
		}
	}

	function scrollToBottomEvents() {
		if (eventsContainer) {
			eventsContainer.scrollTop = eventsContainer.scrollHeight;
		}
	}

	function handleForgeInstanceEvent(event: WebSocketEvent) {
		if (event.operation === 'update') {
			const updated = event.payload as ForgeInstance;
			if (forgeInstance && updated.id === forgeInstance.id) {
				const oldEventCount = forgeInstance.events?.length || 0;
				const newEventCount = updated.events?.length || 0;

				forgeInstance = updateEntityFields(forgeInstance, updated);

				if (newEventCount > oldEventCount) {
					setTimeout(() => {
						scrollToBottomEvents();
					}, 100);
				}
			}
		} else if (event.operation === 'delete') {
			const deletedId = event.payload.id || event.payload;
			if (forgeInstance && forgeInstance.id === deletedId) {
				goto(resolve('/forge-instances'));
			}
		}
	}

	function handlePoolEvent(event: WebSocketEvent) {
		if (!forgeInstance) return;

		const pool = event.payload;
		if (pool.forge_instance_id !== forgeInstance.id) return;

		if (event.operation === 'create') {
			pools = [...pools, pool];
		} else if (event.operation === 'update') {
			pools = pools.map(p =>
				p.id === pool.id ? pool : p
			);
		} else if (event.operation === 'delete') {
			const poolId = pool.id || pool;
			pools = pools.filter(p => p.id !== poolId);
		}
	}

	function handleInstanceEvent(event: WebSocketEvent) {
		if (!forgeInstance || !pools) return;

		const instance = event.payload;
		const belongsToForgeInstance = pools.some(pool => pool.id === instance.pool_id);
		if (!belongsToForgeInstance) return;

		if (event.operation === 'create') {
			instances = [...instances, instance];
		} else if (event.operation === 'update') {
			instances = instances.map(inst =>
				inst.id === instance.id ? instance : inst
			);
		} else if (event.operation === 'delete') {
			const instanceId = instance.id || instance;
			instances = instances.filter(inst => inst.id !== instanceId);
		}
	}

	onMount(() => {
		loadForgeInstance().then(() => {
			if (forgeInstance?.events?.length) {
				setTimeout(() => {
					scrollToBottomEvents();
				}, 100);
			}
		});

		const unsubscribeFI = websocketStore.subscribeToEntity(
			'forge_instance',
			['update', 'delete'],
			handleForgeInstanceEvent
		);
		const unsubscribePool = websocketStore.subscribeToEntity(
			'pool',
			['create', 'update', 'delete'],
			handlePoolEvent
		);
		const unsubscribeInstance = websocketStore.subscribeToEntity(
			'instance',
			['create', 'update', 'delete'],
			handleInstanceEvent
		);
		unsubscribeWebsocket = () => {
			unsubscribeFI();
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
	<title>{forgeInstance ? `${forgeInstance.endpoint?.name} - Forge Instance Details` : 'Forge Instance Details'} - GARM</title>
</svelte:head>

<div class="space-y-6">
	<!-- Breadcrumbs -->
	<nav class="flex" aria-label="Breadcrumb">
		<ol class="inline-flex items-center space-x-1 md:space-x-3">
			<li class="inline-flex items-center">
				<a href={resolve('/forge-instances')} class="inline-flex items-center text-sm font-medium text-gray-700 hover:text-blue-600 dark:text-gray-400 dark:hover:text-white">
					<svg class="w-3 h-3 mr-2.5" fill="currentColor" viewBox="0 0 20 20">
						<path d="M10.707 2.293a1 1 0 00-1.414 0l-7 7a1 1 0 001.414 1.414L4 10.414V17a1 1 0 001 1h2a1 1 0 001-1v-2a1 1 0 011-1h2a1 1 0 011 1v2a1 1 0 001 1h2a1 1 0 001-1v-6.586l.293.293a1 1 0 001.414-1.414l-7-7z"/>
					</svg>
					Forge Instances
				</a>
			</li>
			<li>
				<div class="flex items-center">
					<svg class="w-3 h-3 text-gray-400 mx-1" fill="currentColor" viewBox="0 0 20 20">
						<path fill-rule="evenodd" d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z" clip-rule="evenodd"/>
					</svg>
					<span class="ml-1 text-sm font-medium text-gray-500 md:ml-2 dark:text-gray-400">
						{forgeInstance ? (forgeInstance.endpoint?.name || 'Forge Instance') : 'Loading...'}
					</span>
				</div>
			</li>
		</ol>
	</nav>

	{#if loading}
		<div class="p-6 text-center">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600 mx-auto"></div>
			<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">Loading forge instance...</p>
		</div>
	{:else if error}
		<div class="rounded-md bg-red-50 dark:bg-red-900 p-4">
			<p class="text-sm font-medium text-red-800 dark:text-red-200">{error}</p>
		</div>
	{:else if forgeInstance}
		<!-- Header -->
		<DetailHeader
			title={forgeInstance.endpoint?.name || 'Forge Instance'}
			subtitle="Endpoint: {forgeInstance.endpoint?.name} • Gitea Instance"
			forgeIcon={getForgeIcon("gitea")}
			onEdit={() => showUpdateModal = true}
			onDelete={() => showDeleteModal = true}
		/>

		<!-- Forge Instance Details -->
		<EntityInformation entity={forgeInstance} entityType="forge_instance" />

		<!-- Webhook -->
		<WebhookSection
			entityType="forge_instance"
			entityId={forgeInstance.id || ''}
			entityName={forgeInstance.endpoint?.name || ''}
		/>

		<!-- Pools -->
		<PoolsSection
			{pools}
			entityType="forge_instance"
			entityId={forgeInstance.id || ''}
			entityName={forgeInstance.endpoint?.name || ''}
			on:addPool={handleAddPool}
		/>

		<!-- Instances -->
		<InstancesSection {instances} entityType="forge_instance" onDeleteInstance={openDeleteInstanceModal} />

		<!-- Events -->
		<EventsSection events={forgeInstance?.events} bind:eventsContainer />
	{/if}
</div>

<!-- Modals -->
{#if showUpdateModal && forgeInstance}
	<UpdateEntityModal
		entity={forgeInstance}
		entityType="forge_instance"
		on:close={() => showUpdateModal = false}
		on:submit={(e) => handleUpdate(e.detail)}
	/>
{/if}

{#if showDeleteModal && forgeInstance}
	<DeleteModal
		title="Delete Forge Instance"
		message="Are you sure you want to delete this forge instance? This action cannot be undone and will remove all associated pools and instances."
		itemName={forgeInstance.endpoint?.name}
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

{#if showCreatePoolModal && forgeInstance}
	<CreatePoolModal
		initialEntityType="forge_instance"
		initialEntityId={forgeInstance.id || ''}
		on:close={() => showCreatePoolModal = false}
		on:submit={handleCreatePool}
	/>
{/if}
