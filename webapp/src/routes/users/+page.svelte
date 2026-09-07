<script lang="ts">
	import { onMount } from 'svelte';
	import { garmApi } from '$lib/api/client.js';
	import type { User } from '$lib/api/client.js';
	import PageHeader from '$lib/components/PageHeader.svelte';
	import DataTable from '$lib/components/DataTable.svelte';
	import Badge from '$lib/components/Badge.svelte';
	import { GenericCell } from '$lib/components/cells';
	import { extractAPIError } from '$lib/utils/apiError';

	let users: User[] = [];
	let loading = true;
	let error = '';
	let searchTerm = '';

	// Pagination
	let currentPage = 1;
	let perPage = 25;

	// Filter users by search term
	function filterBySearchTerm(users: User[], term: string): User[] {
		if (!term) return users;
		const lowerTerm = term.toLowerCase();
		return users.filter(user =>
			user.username?.toLowerCase().includes(lowerTerm) ||
			user.email?.toLowerCase().includes(lowerTerm) ||
			user.full_name?.toLowerCase().includes(lowerTerm)
		);
	}

	$: filteredUsers = filterBySearchTerm(users, searchTerm);
	$: totalPages = Math.ceil(filteredUsers.length / perPage);
	$: {
		if (currentPage > totalPages && totalPages > 0) {
			currentPage = totalPages;
		}
	}
	$: paginatedUsers = filteredUsers.slice(
		(currentPage - 1) * perPage,
		currentPage * perPage
	);

	async function loadUsers() {
		try {
			loading = true;
			error = '';
			users = await garmApi.listUsers();
		} catch (err) {
			error = extractAPIError(err);
			console.error('Failed to load users:', err);
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		loadUsers();
	});

	// DataTable configuration
	const columns = [
		{
			key: 'username',
			title: 'Username',
			cellComponent: GenericCell,
			cellProps: { field: 'username' }
		},
		{
			key: 'email',
			title: 'Email',
			cellComponent: GenericCell,
			cellProps: { field: 'email' }
		},
		{
			key: 'full_name',
			title: 'Full Name',
			cellComponent: GenericCell,
			cellProps: { field: 'full_name' }
		},
		{
			key: 'is_admin',
			title: 'Role',
			align: 'center' as const
		},
		{
			key: 'enabled',
			title: 'Status',
			align: 'center' as const
		}
	];

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
</script>

<svelte:head>
	<title>Users - GARM</title>
</svelte:head>

<div class="space-y-6">
	<!-- Header -->
	<PageHeader
		title="Users"
		description="View all users in the system"
	/>

	<DataTable
		{columns}
		data={paginatedUsers}
		{loading}
		{error}
		{searchTerm}
		searchPlaceholder="Search users..."
		{currentPage}
		{perPage}
		{totalPages}
		totalItems={filteredUsers.length}
		itemName="users"
		emptyIconType="users"
		showRetry={!!error}
		on:search={handleTableSearch}
		on:pageChange={handleTablePageChange}
		on:perPageChange={handleTablePerPageChange}
		on:retry={loadUsers}
	>
		<!-- Custom cell rendering for Role and Status -->
		<svelte:fragment slot="cell" let:column let:item>
			{#if column.key === 'is_admin'}
				<Badge
					variant={item.is_admin ? 'purple' : 'gray'}
					text={item.is_admin ? 'Admin' : 'User'}
				/>
			{:else if column.key === 'enabled'}
				<Badge
					variant={item.enabled ? 'green' : 'red'}
					text={item.enabled ? 'Enabled' : 'Disabled'}
				/>
			{/if}
		</svelte:fragment>

		<!-- Mobile card content -->
		<svelte:fragment slot="mobile-card" let:item={user}>
			<div class="flex items-center justify-between">
				<div class="flex-1 min-w-0">
					<p class="text-sm font-medium text-gray-900 dark:text-white truncate">
						{user.username}
					</p>
					<p class="text-xs text-gray-500 dark:text-gray-400 truncate">
						{user.email}
					</p>
					{#if user.full_name}
						<p class="text-xs text-gray-400 dark:text-gray-500 truncate">
							{user.full_name}
						</p>
					{/if}
				</div>
				<div class="flex items-center space-x-2 ml-4">
					<Badge
						variant={user.is_admin ? 'purple' : 'gray'}
						text={user.is_admin ? 'Admin' : 'User'}
					/>
					<Badge
						variant={user.enabled ? 'green' : 'red'}
						text={user.enabled ? 'Enabled' : 'Disabled'}
					/>
				</div>
			</div>
		</svelte:fragment>
	</DataTable>
</div>

