<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { garmApi } from '$lib/api/client.js';
	import type { ForgeCredentials, Repository, Organization, Enterprise } from '$lib/api/generated/api.js';
	import { resolve } from '$app/paths';
	import DeleteModal from '$lib/components/DeleteModal.svelte';
	import { getForgeIcon } from '$lib/utils/common.js';
	import { extractAPIError } from '$lib/utils/apiError';
	import { websocketStore, type WebSocketEvent } from '$lib/stores/websocket.js';
	import { toastStore } from '$lib/stores/toast.js';
	import DataTable from '$lib/components/DataTable.svelte';
	import { GenericCell, EntityCell } from '$lib/components/cells';

	let credential: ForgeCredentials | null = null;
	let loading = true;
	let error = '';
	let showDeleteModal = false;
	let unsubscribeWebsocket: (() => void) | null = null;

	$: credentialId = $page.params.id;
	$: isGithub = credential?.forge_type === 'github';
	$: isGitea = credential?.forge_type === 'gitea';

	async function loadCredential() {
		if (!credentialId) return;

		try {
			loading = true;
			error = '';

			const id = parseInt(credentialId, 10);
			if (isNaN(id)) {
				error = 'Invalid credential ID';
				loading = false;
				return;
			}

			// Try to fetch as GitHub credential first, then Gitea
			let githubError: any = null;
			try {
				credential = await garmApi.getGithubCredentials(id);
			} catch (err: any) {
				githubError = err;
				// If GitHub fails with 404, try Gitea
				if (err?.response?.status === 404 || err?.status === 404) {
					try {
						credential = await garmApi.getGiteaCredentials(id);
					} catch (giteaErr: any) {
						// If Gitea also fails with 404, throw the original GitHub error
						if (giteaErr?.response?.status === 404 || giteaErr?.status === 404) {
							throw new Error(`Credential with ID ${id} not found`);
						}
						// For other Gitea errors, throw them
						throw giteaErr;
					}
				} else {
					// For non-404 GitHub errors, throw them
					throw err;
				}
			}
		} catch (err) {
			error = extractAPIError(err);
		} finally {
			loading = false;
		}
	}

	async function handleDelete() {
		if (!credential) return;
		try {
			if (isGithub) {
				await garmApi.deleteGithubCredentials(credential.id!);
			} else {
				await garmApi.deleteGiteaCredentials(credential.id!);
			}
			goto(resolve('/credentials'));
		} catch (err) {
			const errorMessage = extractAPIError(err);
			toastStore.error(
				'Delete Failed',
				errorMessage
			);
		}
		showDeleteModal = false;
	}

	function handleCredentialEvent(event: WebSocketEvent) {
		if (event.operation === 'delete') {
			const deletedCredentialId = event.payload.id || event.payload;
			// If this credential was deleted, redirect to credentials list
			if (credential && credential.id === deletedCredentialId) {
				goto(resolve('/credentials'));
			}
		}
	}

	// Define column configurations as reactive statements to avoid evaluation at module load
	$: repoColumns = [
		{
			key: 'name',
			title: 'Repository',
			cellComponent: EntityCell,
			cellProps: { entityType: 'repository', showOwner: true }
		},
		{
			key: 'pool_balancing_type',
			title: 'Balancing',
			cellComponent: GenericCell,
			cellProps: { field: 'pool_balancing_type' }
		},
		{
			key: 'agent_mode',
			title: 'Agent Mode',
			cellComponent: GenericCell,
			cellProps: { getValue: (item: any) => item.agent_mode ? 'Yes' : 'No' }
		}
	];

	$: orgColumns = [
		{
			key: 'name',
			title: 'Organization',
			cellComponent: EntityCell,
			cellProps: { entityType: 'organization' }
		},
		{
			key: 'pool_balancing_type',
			title: 'Balancing',
			cellComponent: GenericCell,
			cellProps: { field: 'pool_balancing_type' }
		},
		{
			key: 'agent_mode',
			title: 'Agent Mode',
			cellComponent: GenericCell,
			cellProps: { getValue: (item: any) => item.agent_mode ? 'Yes' : 'No' }
		}
	];

	$: enterpriseColumns = [
		{
			key: 'name',
			title: 'Enterprise',
			cellComponent: EntityCell,
			cellProps: { entityType: 'enterprise' }
		},
		{
			key: 'pool_balancing_type',
			title: 'Balancing',
			cellComponent: GenericCell,
			cellProps: { field: 'pool_balancing_type' }
		},
		{
			key: 'agent_mode',
			title: 'Agent Mode',
			cellComponent: GenericCell,
			cellProps: { getValue: (item: any) => item.agent_mode ? 'Yes' : 'No' }
		}
	];

	$: repoMobileCardConfig = {
		entityType: 'repository' as const,
		primaryText: {
			field: 'name',
			format: (repo: Repository) => `${repo.owner}/${repo.name}`,
			isClickable: true,
			href: '/repositories/{id}'
		},
		customInfo: [
			{
				text: (repo: Repository) => `Balancing: ${repo.pool_balancing_type || 'N/A'}`
			},
			{
				text: (repo: Repository) => `Agent Mode: ${repo.agent_mode ? 'Yes' : 'No'}`
			}
		]
	};

	$: orgMobileCardConfig = {
		entityType: 'organization' as const,
		primaryText: {
			field: 'name',
			isClickable: true,
			href: '/organizations/{id}'
		},
		customInfo: [
			{
				text: (org: Organization) => `Balancing: ${org.pool_balancing_type || 'N/A'}`
			},
			{
				text: (org: Organization) => `Agent Mode: ${org.agent_mode ? 'Yes' : 'No'}`
			}
		]
	};

	$: enterpriseMobileCardConfig = {
		entityType: 'enterprise' as const,
		primaryText: {
			field: 'name',
			isClickable: true,
			href: '/enterprises/{id}'
		},
		customInfo: [
			{
				text: (ent: Enterprise) => `Balancing: ${ent.pool_balancing_type || 'N/A'}`
			},
			{
				text: (ent: Enterprise) => `Agent Mode: ${ent.agent_mode ? 'Yes' : 'No'}`
			}
		]
	};

	onMount(() => {
		loadCredential();

		// Subscribe to credential events (currently only delete is pushed via websocket)
		// Note: We subscribe to both github and gitea credential events
		const unsubscribeGithub = websocketStore.subscribeToEntity(
			'github_credentials',
			['delete'],
			handleCredentialEvent
		);
		const unsubscribeGitea = websocketStore.subscribeToEntity(
			'gitea_credentials',
			['delete'],
			handleCredentialEvent
		);

		unsubscribeWebsocket = () => {
			unsubscribeGithub();
			unsubscribeGitea();
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
	<title>{credential ? `${credential.name} - Credential Details` : 'Credential Details'} - GARM</title>
</svelte:head>

<div class="space-y-6">
	<!-- Breadcrumbs -->
	<nav class="flex" aria-label="Breadcrumb">
		<ol class="inline-flex items-center space-x-1 md:space-x-3">
			<li class="inline-flex items-center">
				<a href={resolve('/credentials')} class="inline-flex items-center text-sm font-medium text-gray-700 hover:text-blue-600 dark:text-gray-400 dark:hover:text-white">
					<svg class="w-3 h-3 mr-2.5" fill="currentColor" viewBox="0 0 20 20">
						<path d="M10.707 2.293a1 1 0 00-1.414 0l-7 7a1 1 0 001.414 1.414L4 10.414V17a1 1 0 001 1h2a1 1 0 001-1v-2a1 1 0 011-1h2a1 1 0 011 1v2a1 1 0 001 1h2a1 1 0 001-1v-6.586l.293.293a1 1 0 001.414-1.414l-7-7z"></path>
					</svg>
					Credentials
				</a>
			</li>
			<li aria-current="page">
				<div class="flex items-center">
					<svg class="w-6 h-6 text-gray-400" fill="currentColor" viewBox="0 0 20 20">
						<path fill-rule="evenodd" d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z" clip-rule="evenodd"></path>
					</svg>
					<span class="ml-1 text-sm font-medium text-gray-500 md:ml-2 dark:text-gray-400">
						{credential?.name || 'Loading...'}
					</span>
				</div>
			</li>
		</ol>
	</nav>

	{#if loading}
		<div class="flex items-center justify-center py-12">
			<div class="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600"></div>
		</div>
	{:else if error}
		<div class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-4">
			<div class="flex">
				<svg class="h-5 w-5 text-red-600 dark:text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
				</svg>
				<div class="ml-3">
					<h3 class="text-sm font-medium text-red-800 dark:text-red-200">Error loading credential</h3>
					<p class="mt-1 text-sm text-red-700 dark:text-red-300">{error}</p>
				</div>
			</div>
		</div>
	{:else if credential}
		<!-- Header with actions -->
		<div class="bg-white dark:bg-gray-800 shadow rounded-lg p-6">
			<div class="flex items-start justify-between">
				<div class="flex items-center space-x-4">
					<div class="flex-shrink-0">
						{@html getForgeIcon(credential.forge_type || '', 'w-12 h-12')}
					</div>
					<div>
						<h1 class="text-2xl font-bold text-gray-900 dark:text-white">{credential.name || 'Unnamed Credential'}</h1>
						<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
							<span class="capitalize">{credential.forge_type}</span> Credential
							{#if credential.description}
								<span class="text-gray-400 dark:text-gray-500 mx-2">•</span>
								<span>{credential.description}</span>
							{/if}
						</p>
					</div>
				</div>
				<div>
					<button
						on:click={() => showDeleteModal = true}
						class="px-4 py-2 bg-red-600 hover:bg-red-700 dark:bg-red-700 dark:hover:bg-red-800 text-white rounded-lg font-medium text-sm cursor-pointer"
					>
						Delete
					</button>
				</div>
			</div>
		</div>

		<!-- Credential Information -->
		<div class="bg-white dark:bg-gray-800 shadow rounded-lg p-6">
			<h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">Credential Information</h3>
			<dl class="grid grid-cols-1 md:grid-cols-2 gap-4">
				<div>
					<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">ID</dt>
					<dd class="mt-1 text-sm text-gray-900 dark:text-white">{credential.id}</dd>
				</div>
				<div>
					<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Name</dt>
					<dd class="mt-1 text-sm text-gray-900 dark:text-white">{credential.name}</dd>
				</div>
				<div>
					<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Description</dt>
					<dd class="mt-1 text-sm text-gray-900 dark:text-white">{credential.description || 'N/A'}</dd>
				</div>
				<div>
					<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Auth Type</dt>
					<dd class="mt-1 text-sm text-gray-900 dark:text-white uppercase">{credential['auth-type'] || 'N/A'}</dd>
				</div>
				<div>
					<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Forge Type</dt>
					<dd class="mt-1 text-sm text-gray-900 dark:text-white capitalize">{credential.forge_type || 'N/A'}</dd>
				</div>
				<div>
					<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Base URL</dt>
					<dd class="mt-1 text-sm text-gray-900 dark:text-white break-all">{credential.base_url || 'N/A'}</dd>
				</div>
				<div>
					<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">API Base URL</dt>
					<dd class="mt-1 text-sm text-gray-900 dark:text-white break-all">{credential.api_base_url || 'N/A'}</dd>
				</div>
				{#if isGithub && credential.upload_base_url}
					<div>
						<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Upload Base URL</dt>
						<dd class="mt-1 text-sm text-gray-900 dark:text-white break-all">{credential.upload_base_url}</dd>
					</div>
				{/if}
				<div>
					<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Created</dt>
					<dd class="mt-1 text-sm text-gray-900 dark:text-white">{new Date(credential.created_at || '').toLocaleString()}</dd>
				</div>
				<div>
					<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Updated</dt>
					<dd class="mt-1 text-sm text-gray-900 dark:text-white">{new Date(credential.updated_at || '').toLocaleString()}</dd>
				</div>
			</dl>
		</div>

		<!-- Endpoint Information -->
		{#if credential.endpoint}
			<div class="bg-white dark:bg-gray-800 shadow rounded-lg p-6">
				<h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">Endpoint Information</h3>
				<dl class="grid grid-cols-1 md:grid-cols-2 gap-4">
					<div>
						<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Name</dt>
						<dd class="mt-1 text-sm text-gray-900 dark:text-white">
							<a href={resolve(`/endpoints`)} class="text-blue-600 hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-300">
								{credential.endpoint.name}
							</a>
						</dd>
					</div>
					<div>
						<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Description</dt>
						<dd class="mt-1 text-sm text-gray-900 dark:text-white">{credential.endpoint.description || 'N/A'}</dd>
					</div>
					<div>
						<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Type</dt>
						<dd class="mt-1 text-sm text-gray-900 dark:text-white capitalize">{credential.endpoint.endpoint_type}</dd>
					</div>
					<div>
						<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Base URL</dt>
						<dd class="mt-1 text-sm text-gray-900 dark:text-white break-all">{credential.endpoint.base_url}</dd>
					</div>
					{#if isGitea && credential.endpoint.tools_metadata_url}
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Tools Metadata URL</dt>
							<dd class="mt-1 text-sm text-gray-900 dark:text-white break-all">{credential.endpoint.tools_metadata_url}</dd>
						</div>
					{/if}
					{#if isGitea}
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Use Internal Tools Metadata</dt>
							<dd class="mt-1 text-sm text-gray-900 dark:text-white">{credential.endpoint.use_internal_tools_metadata ? 'Yes' : 'No'}</dd>
						</div>
					{/if}
				</dl>
			</div>
		{/if}

		<!-- GitHub Rate Limit (GitHub only) -->
		{#if isGithub && credential.rate_limit}
			<div class="bg-white dark:bg-gray-800 shadow rounded-lg p-6">
				<h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">GitHub Rate Limit</h3>
				<dl class="grid grid-cols-1 md:grid-cols-2 gap-4">
					<div>
						<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Limit</dt>
						<dd class="mt-1 text-sm text-gray-900 dark:text-white">{credential.rate_limit.limit}</dd>
					</div>
					<div>
						<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Used</dt>
						<dd class="mt-1 text-sm text-gray-900 dark:text-white">{credential.rate_limit.used}</dd>
					</div>
					<div>
						<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Remaining</dt>
						<dd class="mt-1 text-sm text-gray-900 dark:text-white">{credential.rate_limit.remaining}</dd>
					</div>
					<div>
						<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Reset</dt>
						<dd class="mt-1 text-sm text-gray-900 dark:text-white">
							{new Date((credential.rate_limit.reset || 0) * 1000).toLocaleString()}
						</dd>
					</div>
				</dl>
			</div>
		{/if}

		<!-- Repositories -->
		{#if credential.repositories && credential.repositories.length > 0}
			<div class="bg-white dark:bg-gray-800 shadow rounded-lg">
				<div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
					<h3 class="text-lg font-medium text-gray-900 dark:text-white">Repositories</h3>
					<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
						Repositories using this credential
					</p>
				</div>
				<div class="p-6">
					<DataTable
						columns={repoColumns}
						data={credential.repositories}
						loading={false}
						searchPlaceholder="Search repositories..."
						itemName="repositories"
						emptyMessage="No repositories using this credential"
						mobileCardConfig={repoMobileCardConfig}
					/>
				</div>
			</div>
		{/if}

		<!-- Organizations -->
		{#if credential.organizations && credential.organizations.length > 0}
			<div class="bg-white dark:bg-gray-800 shadow rounded-lg">
				<div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
					<h3 class="text-lg font-medium text-gray-900 dark:text-white">Organizations</h3>
					<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
						Organizations using this credential
					</p>
				</div>
				<div class="p-6">
					<DataTable
						columns={orgColumns}
						data={credential.organizations}
						loading={false}
						searchPlaceholder="Search organizations..."
						itemName="organizations"
						emptyMessage="No organizations using this credential"
						mobileCardConfig={orgMobileCardConfig}
					/>
				</div>
			</div>
		{/if}

		<!-- Enterprises (GitHub only) -->
		{#if isGithub && credential.enterprises && credential.enterprises.length > 0}
			<div class="bg-white dark:bg-gray-800 shadow rounded-lg">
				<div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
					<h3 class="text-lg font-medium text-gray-900 dark:text-white">Enterprises</h3>
					<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
						Enterprises using this credential
					</p>
				</div>
				<div class="p-6">
					<DataTable
						columns={enterpriseColumns}
						data={credential.enterprises}
						loading={false}
						searchPlaceholder="Search enterprises..."
						itemName="enterprises"
						emptyMessage="No enterprises using this credential"
						mobileCardConfig={enterpriseMobileCardConfig}
					/>
				</div>
			</div>
		{/if}
	{/if}
</div>

<!-- Delete Modal -->
{#if showDeleteModal && credential}
	{@const repoCount = credential.repositories?.length || 0}
	{@const orgCount = credential.organizations?.length || 0}
	{@const entCount = (isGithub && credential.enterprises?.length) || 0}
	{@const warnings = [
		repoCount > 0 ? `Warning: This credential is currently used by ${repoCount} ${repoCount === 1 ? 'repository' : 'repositories'}.` : '',
		orgCount > 0 ? `Warning: This credential is currently used by ${orgCount} ${orgCount === 1 ? 'organization' : 'organizations'}.` : '',
		entCount > 0 ? `Warning: This credential is currently used by ${entCount} ${entCount === 1 ? 'enterprise' : 'enterprises'}.` : ''
	].filter(w => w).join(' ')}
	<DeleteModal
		title="Delete Credential"
		message={`Are you sure you want to delete the credential '${credential.name}'? This action cannot be undone. ${warnings}`}
		itemName={credential.name || 'credential'}
		on:confirm={handleDelete}
		on:cancel={() => showDeleteModal = false}
	/>
{/if}
