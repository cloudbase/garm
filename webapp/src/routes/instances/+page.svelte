<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { garmApi } from '$lib/api/client.js';
	import type { Instance } from '$lib/api/generated/api.js';
	import DeleteModal from '$lib/components/DeleteModal.svelte';
	import PageHeader from '$lib/components/PageHeader.svelte';
	import { websocketStore, type WebSocketEvent } from '$lib/stores/websocket.js';
	import { toastStore } from '$lib/stores/toast.js';
	import DataTable from '$lib/components/DataTable.svelte';
	import { EntityCell, EndpointCell, StatusCell, ActionsCell, GenericCell, InstancePoolCell } from '$lib/components/cells';

	let instances: Instance[] = [];
	let loading = true;
	let error = '';
	let statusFilter = '';
	let unsubscribeWebsocket: (() => void) | null = null;


	// Pagination
	let currentPage = 1;
	let perPage = 25;
	let searchTerm = '';

	// Modal state
	let showDeleteModal = false;
	let instanceToDelete: Instance | null = null;

	$: filteredInstances = instances.filter(instance => {
		const matchesSearch = searchTerm === '' || 
			instance.name?.toLowerCase().includes(searchTerm.toLowerCase()) ||
			instance.provider_id?.toLowerCase().includes(searchTerm.toLowerCase());
		const matchesStatus = statusFilter === '' || instance.status === statusFilter || instance.runner_status === statusFilter;
		return matchesSearch && matchesStatus;
	});

	$: totalPages = Math.ceil(filteredInstances.length / perPage);
	$: {
		if (currentPage > totalPages && totalPages > 0) {
			currentPage = totalPages;
		}
	}
	$: paginatedInstances = filteredInstances.slice(
		(currentPage - 1) * perPage,
		currentPage * perPage
	);

	async function loadInstances() {
		try {
			loading = true;
			error = '';
			instances = await garmApi.listInstances();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load instances';
		} finally {
			loading = false;
		}
	}

	function handleDelete(instance: Instance) {
		instanceToDelete = instance;
		showDeleteModal = true;
	}

	async function confirmDelete() {
		if (!instanceToDelete) return;
		
		try {
			await garmApi.deleteInstance(instanceToDelete.name!);
			// No need to reload - websocket will handle the update
			toastStore.success(
				'Instance Deleted',
				`Instance ${instanceToDelete.name} has been deleted successfully.`
			);
			showDeleteModal = false;
			instanceToDelete = null;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to delete instance';
		}
	}

	function handleStatusFilterChange(event: Event) {
		statusFilter = (event.target as HTMLSelectElement).value;
		currentPage = 1;
	}

	// DataTable configuration
	const columns = [
		{ 
			key: 'name', 
			title: 'Name',
			cellComponent: EntityCell,
			cellProps: { entityType: 'instance', showId: true }
		},
		{ 
			key: 'pool_scale_set', 
			title: 'Pool/Scale Set',
			flexible: true,
			cellComponent: InstancePoolCell
		},
		{ 
			key: 'created', 
			title: 'Created',
			cellComponent: GenericCell,
			cellProps: { field: 'created_at', type: 'date' }
		},
		{ 
			key: 'status', 
			title: 'Status',
			cellComponent: StatusCell,
			cellProps: { statusType: 'instance', statusField: 'status' }
		},
		{ 
			key: 'runner_status', 
			title: 'Runner Status',
			cellComponent: StatusCell,
			cellProps: { statusType: 'instance', statusField: 'runner_status' }
		},
		{ 
			key: 'actions', 
			title: 'Actions', 
			align: 'right' as const,
			cellComponent: ActionsCell,
			cellProps: { 
				actions: [
					{ type: 'delete', title: 'Delete', ariaLabel: 'Delete instance', action: 'delete' }
				]
			}
		}
	];

	// Mobile card configuration  
	const mobileCardConfig = {
		entityType: 'instance' as const,
		primaryText: {
			field: 'name',
			isClickable: true,
			href: '/instances/{name}'
		},
		secondaryText: {
			field: 'provider_id'
		},
		badges: [
			{ type: 'status' as const, field: 'status' },
			{ type: 'status' as const, field: 'runner_status' }
		],
		actions: [
			{ 
				type: 'delete' as const, 
				handler: (item: any) => handleDelete(item) 
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

	async function retryLoadInstances() {
		try {
			await loadInstances();
		} catch (err) {
			console.error('Retry failed:', err);
		}
	}

	function handleEdit(event: CustomEvent<{ item: any }>) {
		// Instances don't have edit functionality
	}

	function handleDeleteInstance(event: CustomEvent<{ item: any }>) {
		handleDelete(event.detail.item);
	}



	function handleInstanceEvent(event: WebSocketEvent) {
		if (event.operation === 'create') {
			// Add new instance
			const newInstance = event.payload as Instance;
			instances = [...instances, newInstance];
		} else if (event.operation === 'update') {
			// Update existing instance - instances use name as identifier, not id
			const updatedInstance = event.payload as Instance;
			instances = instances.map(instance => 
				instance.name === updatedInstance.name ? updatedInstance : instance
			);
		} else if (event.operation === 'delete') {
			// Remove instance - payload might only contain name
			const instanceName = event.payload.name || event.payload;
			instances = instances.filter(instance => instance.name !== instanceName);
		}
	}

	onMount(() => {
		// Initial load
		loadInstances();
		
		// Subscribe to real-time instance events - correct entity type is 'instance'
		unsubscribeWebsocket = websocketStore.subscribeToEntity(
			'instance',
			['create', 'update', 'delete'],
			handleInstanceEvent
		);
	});

	onDestroy(() => {
		// Clean up websocket subscription
		if (unsubscribeWebsocket) {
			unsubscribeWebsocket();
			unsubscribeWebsocket = null;
		}
	});
</script>

<svelte:head>
	<title>Instances - GARM</title>
</svelte:head>

<div class="space-y-6">
	<PageHeader
		title="Runner Instances"
		description="Monitor your running instances"
		showAction={false}
	/>

	{#if error}
		<div class="bg-red-50 dark:bg-red-900/50 border border-red-200 dark:border-red-800 rounded-md p-4">
			<div class="flex">
				<div class="ml-3">
					<h3 class="text-sm font-medium text-red-800 dark:text-red-200">Error</h3>
					<div class="mt-2 text-sm text-red-700 dark:text-red-300">{error}</div>
				</div>
			</div>
		</div>
	{/if}


	<DataTable
		{columns}
		data={paginatedInstances}
		{loading}
		{error}
		{searchTerm}
		searchPlaceholder="Search instances..."
		{currentPage}
		{perPage}
		{totalPages}
		totalItems={filteredInstances.length}
		itemName="instances"
		emptyIconType="cog"
		showRetry={!!error}
		{mobileCardConfig}
		on:search={handleTableSearch}
		on:pageChange={handleTablePageChange}
		on:perPageChange={handleTablePerPageChange}
		on:retry={retryLoadInstances}
		on:edit={handleEdit}
		on:delete={handleDeleteInstance}
	/>
</div>

<!-- Delete Modal -->
{#if showDeleteModal && instanceToDelete}
	<DeleteModal
		title="Delete Instance"
		message="Are you sure you want to delete this instance? This action cannot be undone."
		itemName={instanceToDelete.name}
		on:close={() => {
			showDeleteModal = false;
			instanceToDelete = null;
		}}
		on:confirm={confirmDelete}
	/>
{/if}