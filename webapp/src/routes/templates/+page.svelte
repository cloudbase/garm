<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { garmApi } from '$lib/api/client.js';
	import type { Template } from '$lib/api/generated/api.js';
	import PageHeader from '$lib/components/PageHeader.svelte';
	import ActionButton from '$lib/components/ActionButton.svelte';
	import Button from '$lib/components/Button.svelte';
	import DataTable from '$lib/components/DataTable.svelte';
	import { toastStore } from '$lib/stores/toast.js';
	import { extractAPIError } from '$lib/utils/apiError';
	import Badge from '$lib/components/Badge.svelte';
	import DeleteModal from '$lib/components/DeleteModal.svelte';
	import { EntityCell, GenericCell, ActionsCell } from '$lib/components/cells';
	import { isCurrentUserAdmin } from '$lib/utils/jwt';
	import { eagerCache, eagerCacheManager } from '$lib/stores/eager-cache';

	let templates: Template[] = [];
	let error = '';
	let searchTerm = '';

	// Subscribe to eager cache for templates
	$: {
		// Only use cache data if we're not in direct API mode
		if (!templates.length || $eagerCache.loaded.templates) {
			templates = $eagerCache.templates;
		}
	}
	$: loading = $eagerCache.loading.templates;
	$: cacheError = $eagerCache.errorMessages.templates;

	// Pagination
	let currentPage = 1;
	let perPage = 25;
	let totalPages = 1;

	// Modals
	let showDeleteModal = false;
	let showRestoreModal = false;
	let selectedTemplate: Template | null = null;

	$: filteredTemplates = searchTerm
		? templates.filter(template =>
			template.name?.toLowerCase().includes(searchTerm.toLowerCase()) ||
			template.description?.toLowerCase().includes(searchTerm.toLowerCase()) ||
			template.forge_type?.toLowerCase().includes(searchTerm.toLowerCase()) ||
			template.os_type?.toLowerCase().includes(searchTerm.toLowerCase())
		)
		: templates;

	$: {
		totalPages = Math.ceil(filteredTemplates.length / perPage);
		if (currentPage > totalPages && totalPages > 0) {
			currentPage = totalPages;
		}
	}

	$: paginatedTemplates = filteredTemplates.slice(
		(currentPage - 1) * perPage,
		currentPage * perPage
	);

	async function retryLoadTemplates() {
		try {
			await eagerCacheManager.retryResource('templates');
		} catch (err) {
			console.error('Retry failed:', err);
		}
	}



	async function handleDeleteTemplate() {
		if (!selectedTemplate?.id) return;

		try {
			await garmApi.deleteTemplate(selectedTemplate.id);

			toastStore.add({
				type: 'success',
				title: 'Template deleted',
				message: `Template "${selectedTemplate.name}" has been deleted successfully.`
			});

			showDeleteModal = false;
			selectedTemplate = null;
		} catch (err) {
			const errorMsg = extractAPIError(err);
			toastStore.add({
				type: 'error',
				title: 'Failed to delete template',
				message: errorMsg
			});
		}
	}

	function openCreateModal() {
		goto(resolve('/templates/create'));
	}

	async function openCloneModal(template: Template) {
		if (!template.id) {
			toastStore.add({
				type: 'error',
				title: 'Error',
				message: 'Template ID is missing'
			});
			return;
		}

		goto(resolve(`/templates/create?clone=${template.id}`));
	}



	function openDeleteModal(template: Template) {
		selectedTemplate = template;
		showDeleteModal = true;
	}

	function openRestoreModal() {
		showRestoreModal = true;
	}

	async function handleRestoreTemplates() {
		try {
			await garmApi.restoreTemplates({ restore_all: true });

			toastStore.add({
				type: 'success',
				title: 'Templates restored',
				message: 'System templates have been restored successfully.'
			});

			showRestoreModal = false;

			// Reload templates to show the restored ones
			await eagerCacheManager.retryResource('templates');
		} catch (err) {
			const errorMsg = extractAPIError(err);
			toastStore.add({
				type: 'error',
				title: 'Failed to restore templates',
				message: errorMsg
			});
		}
	}


	// Remove handleTableAction since we're using direct edit/delete handlers now

	function getForgeTypeBadge(forgeType?: string): { color: 'success' | 'error' | 'warning' | 'info' | 'gray' | 'blue' | 'green' | 'red' | 'yellow' | 'secondary', text: string } {
		switch (forgeType) {
			case 'github':
				return { color: 'blue', text: 'GitHub' };
			case 'gitea':
				return { color: 'green', text: 'Gitea' };
			default:
				return { color: 'gray', text: forgeType || 'Unknown' };
		}
	}

	function getOSTypeBadge(osType?: string): { color: 'success' | 'error' | 'warning' | 'info' | 'gray' | 'blue' | 'green' | 'red' | 'yellow' | 'secondary', text: string } {
		switch (osType) {
			case 'linux':
				return { color: 'blue', text: 'Linux' };
			case 'windows':
				return { color: 'info', text: 'Windows' };
			default:
				return { color: 'gray', text: osType || 'Unknown' };
		}
	}

	const columns = [
		{
			key: 'name',
			title: 'Name',
			cellComponent: EntityCell,
			cellProps: { entityType: 'template' }
		},
		{
			key: 'description',
			title: 'Description',
			cellComponent: GenericCell,
			cellProps: { field: 'description', type: 'description' }
		},
		{
			key: 'forge_type',
			title: 'Forge Type',
			cellComponent: GenericCell,
			cellProps: { field: 'forge_type' }
		},
		{
			key: 'os_type',
			title: 'OS Type',
			cellComponent: GenericCell,
			cellProps: { field: 'os_type' }
		},
		{
			key: 'owner_id',
			title: 'Owner',
			cellComponent: GenericCell,
			cellProps: { field: 'owner_id' }
		},
		{
			key: 'actions',
			title: 'Actions',
			align: 'right' as const,
			cellComponent: ActionsCell,
			cellProps: (item: any) => {
				const isAdmin = isCurrentUserAdmin();
				const isSystemTemplate = item.owner_id === 'system';

				const actions = [];

				// Always show clone button
				actions.push({ type: 'copy' as const, title: 'Clone', ariaLabel: 'Clone template', action: 'clone' as const });

				// Show edit button if: user is admin OR it's a user template (non-system)
				if (isAdmin || !isSystemTemplate) {
					actions.push({ type: 'edit' as const, title: 'Edit', ariaLabel: 'Edit template', action: 'edit' as const });
				}

				// Show delete button if: user is admin OR it's a user template (non-system)
				if (isAdmin || !isSystemTemplate) {
					actions.push({ type: 'delete' as const, title: 'Delete', ariaLabel: 'Delete template', action: 'delete' as const });
				}

				return { actions };
			}
		}
	];

	// Mobile card configuration for templates
	const mobileCardConfig = {
		entityType: 'template',
		primaryText: {
			field: 'name',
			isClickable: true,
			href: '/templates/{id}'
		},
		secondaryText: {
			field: 'description'
		},
		badges: [
			{
				type: 'custom',
				value: (item: any) => {
					const badge = getForgeTypeBadge(item.forge_type);
					return { variant: badge.color, text: badge.text };
				}
			},
			{
				type: 'custom',
				value: (item: any) => {
					const badge = getOSTypeBadge(item.os_type);
					return { variant: badge.color, text: badge.text };
				}
			}
		],
		actions: [
			{
				type: 'clone',
				handler: (item: any) => openCloneModal(item)
			},
			{
				type: 'edit',
				handler: (item: any) => goto(resolve(`/templates/${item.id}?edit=true`))
			},
			{
				type: 'delete',
				handler: (item: any) => openDeleteModal(item)
			}
		]
	};

	onMount(async () => {
		// Load templates through eager cache (priority load + background load others)
		try {
			const templateData = await eagerCacheManager.getTemplates();
			// If WebSocket is disconnected, getTemplates returns direct API data
			// Update our local templates array with this data
			if (templateData && Array.isArray(templateData)) {
				templates = templateData;
			}
		} catch (err) {
			// Cache error is already handled by the eager cache system
			console.error('Failed to load templates:', err);
			error = err instanceof Error ? err.message : 'Failed to load templates';
		}
	});
</script>

<svelte:head>
	<title>Runner Install Templates - GARM</title>
</svelte:head>

<PageHeader
	title="Runner Install Templates"
	description="Manage templates for configuring runner software installation. These templates can be set on pools or scale sets."
	actionLabel="Create Template"
	showAction={true}
	on:action={openCreateModal}
>
	<svelte:fragment slot="secondary-actions">
		{#if isCurrentUserAdmin()}
			<Button
				variant="secondary"
				icon='<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />'
				on:click={openRestoreModal}
			>
				Restore System Templates
			</Button>
		{/if}
	</svelte:fragment>
</PageHeader>

{#if (error || cacheError) && !loading}
	<div class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md p-4 mb-6">
		<div class="flex">
			<div class="flex-shrink-0">
				<svg class="h-5 w-5 text-red-400" viewBox="0 0 20 20" fill="currentColor">
					<path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
				</svg>
			</div>
			<div class="ml-3">
				<h3 class="text-sm font-medium text-red-800 dark:text-red-200">
					Error loading templates
				</h3>
				<div class="mt-2 text-sm text-red-700 dark:text-red-300">
					{error || cacheError}
				</div>
				<div class="mt-4">
					<ActionButton variant="secondary" size="sm" on:click={retryLoadTemplates}>
						Try Again
					</ActionButton>
				</div>
			</div>
		</div>
	</div>
{/if}

<DataTable
	{columns}
	data={paginatedTemplates}
	{loading}
	{error}
	{searchTerm}
	searchPlaceholder="Search templates by name, description, type..."
	{currentPage}
	{perPage}
	{totalPages}
	totalItems={filteredTemplates.length}
	{mobileCardConfig}
	on:search={(e) => { searchTerm = e.detail.term; currentPage = 1; }}
	on:pageChange={(e) => currentPage = e.detail.page}
	on:perPageChange={(e) => { perPage = e.detail.perPage; currentPage = 1; }}
	on:clone={(e) => openCloneModal(e.detail.item)}
	on:edit={(e) => goto(resolve(`/templates/${e.detail.item.id}?edit=true`))}
	on:delete={(e) => openDeleteModal(e.detail.item)}
	emptyMessage="No templates found"
>
	<svelte:fragment slot="cell" let:item let:column>
		{#if column.key === 'forge_type'}
			{@const badgeInfo = getForgeTypeBadge(item.forge_type)}
			<Badge variant={badgeInfo.color} text={badgeInfo.text} />
		{:else if column.key === 'os_type'}
			{@const badgeInfo = getOSTypeBadge(item.os_type)}
			<Badge variant={badgeInfo.color} text={badgeInfo.text} />
		{:else if column.key === 'owner_id'}
			{item.owner_id === 'system' ? 'System' : (item.owner_id || 'Unknown')}
		{/if}
	</svelte:fragment>
</DataTable>


<!-- Delete Template Modal -->
{#if showDeleteModal && selectedTemplate}
	<DeleteModal
		title="Delete Template"
		message="Are you sure you want to delete this template? This action cannot be undone."
		itemName={selectedTemplate.name}
		on:close={() => { showDeleteModal = false; selectedTemplate = null; }}
		on:confirm={handleDeleteTemplate}
	/>
{/if}

<!-- Restore Templates Modal -->
{#if showRestoreModal}
	<DeleteModal
		title="Restore System Templates"
		message="This will restore all system templates from the default configuration. Any missing system templates will be created, and any changes made to existing system templates will be overwritten with the default content."
		itemName=""
		confirmLabel="Restore Templates"
		danger={false}
		on:close={() => showRestoreModal = false}
		on:confirm={handleRestoreTemplates}
	/>
{/if}