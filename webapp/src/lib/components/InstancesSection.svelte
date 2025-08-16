<script lang="ts">
	import type { Instance } from '$lib/api/generated/api.js';
	import { base } from '$app/paths';
	import DataTable from './DataTable.svelte';
	import { EntityCell, StatusCell, GenericCell, ActionsCell } from './cells';

	export let instances: Instance[];
	export let entityType: 'repository' | 'organization' | 'enterprise' | 'scaleset';
	export let onDeleteInstance: (instance: Instance) => void;

	// DataTable configuration for instances section
	const columns = [
		{ 
			key: 'name', 
			title: 'Name',
			cellComponent: EntityCell,
			cellProps: { entityType: 'instance', nameField: 'name' }
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
			key: 'created', 
			title: 'Created',
			cellComponent: GenericCell,
			cellProps: { field: 'created_at', type: 'date' }
		},
		{ 
			key: 'actions', 
			title: 'Actions',
			align: 'right' as const,
			cellComponent: ActionsCell,
			cellProps: { 
				actions: [{
					type: 'delete' as const,
					label: 'Delete',
					title: 'Delete instance',
					ariaLabel: 'Delete instance',
					action: 'delete' as const
				}]
			}
		}
	];

	// Mobile card configuration for instances section
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
			{ 
				type: 'status' as const, 
				field: 'status' 
			}
		],
		actions: [
			{ 
				type: 'delete' as const, 
				handler: (item: any) => handleDelete(item) 
			}
		]
	};

	function handleDelete(instance: Instance) {
		onDeleteInstance(instance);
	}

	function handleDeleteInstance(event: CustomEvent<{ item: any }>) {
		handleDelete(event.detail.item);
	}
</script>

<div class="bg-white dark:bg-gray-800 shadow rounded-lg">
	<div class="px-4 py-5 sm:p-6">
		<div class="flex items-center justify-between mb-4">
			<h2 class="text-lg font-medium text-gray-900 dark:text-white">Instances ({instances.length})</h2>
			<a href={`${base}/instances`} class="text-sm text-blue-600 dark:text-blue-400 hover:text-blue-500 dark:hover:text-blue-300">View all instances</a>
		</div>
		<DataTable
			{columns}
			data={instances}
			loading={false}
			error=""
			searchTerm=""
			showSearch={false}
			showPagination={false}
			currentPage={1}
			perPage={instances.length}
			totalPages={1}
			totalItems={instances.length}
			itemName="instances"
			emptyTitle="No instances running"
			emptyMessage="No instances running for this {entityType}."
			emptyIconType="cog"
			{mobileCardConfig}
			on:delete={handleDeleteInstance}
		/>
	</div>
</div>