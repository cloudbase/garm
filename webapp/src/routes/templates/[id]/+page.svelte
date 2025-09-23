<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { garmApi } from '$lib/api/client.js';
	import type { Template, UpdateTemplateParams } from '$lib/api/generated/api.js';
	import ActionButton from '$lib/components/ActionButton.svelte';
	import { toastStore } from '$lib/stores/toast.js';
	import { extractAPIError } from '$lib/utils/apiError';
	import Badge from '$lib/components/Badge.svelte';
	import DetailHeader from '$lib/components/DetailHeader.svelte';
	import CodeEditor from '$lib/components/CodeEditor.svelte';
	import ConfirmationModal from '$lib/components/ConfirmationModal.svelte';
	import DeleteModal from '$lib/components/DeleteModal.svelte';
	import { isCurrentUserAdmin } from '$lib/utils/jwt';
	import { getForgeIcon } from '$lib/utils/common.js';
	import { websocketStore } from '$lib/stores/websocket';

	let loading = true;
	let template: Template | null = null;
	let error = '';

	// Edit mode
	let editMode = false;
	let editForm = {
		name: '',
		description: '',
		data: new Uint8Array()
	};
	let editTemplateContent = '';
	let selectedLanguage: 'bash' | 'powershell' | 'python' | 'text' = 'text';
	let manualLanguageOverride = false;

	// Track original values for change detection
	let originalForm = {
		name: '',
		description: ''
	};
	let originalTemplateContent = '';

	// Delete modal
	let showDeleteModal = false;
	// Unsaved changes modal
	let showUnsavedChangesModal = false;

	$: templateId = parseInt(page.params.id || '0');

	async function loadTemplate() {
		try {
			loading = true;
			error = '';
			template = await garmApi.getTemplate(templateId);

			if (!template) {
				throw new Error('Template not found');
			}

			// Initialize edit form with current values
			editForm.name = template.name || '';
			editForm.description = template.description || '';

			// Store original values for change detection
			originalForm.name = template.name || '';
			originalForm.description = template.description || '';
			if (template.data) {
				// Convert number array to text content
				try {
					if (Array.isArray(template.data)) {
						// Convert number array to Uint8Array, then decode to text
						const uint8Array = new Uint8Array(template.data);
						editTemplateContent = new TextDecoder().decode(uint8Array);
						originalTemplateContent = editTemplateContent;
					} else {
						// If it's a string (base64), decode it
						editTemplateContent = atob(template.data as string);
						originalTemplateContent = editTemplateContent;
					}
				} catch (err) {
					console.error('Failed to decode template data:', err);
					editTemplateContent = '';
					originalTemplateContent = '';
				}
			} else {
				editTemplateContent = '';
				originalTemplateContent = '';
			}
		} catch (err) {
			error = extractAPIError(err);
			toastStore.add({
				type: 'error',
				title: 'Failed to load template',
				message: error
			});
		} finally {
			loading = false;
		}
	}

	async function handleUpdateTemplate() {
		if (!template?.id || !canSave) return;

		try {
			const params: UpdateTemplateParams = {
				name: editForm.name,
				description: editForm.description || undefined,
				data: Array.from(new TextEncoder().encode(editTemplateContent)) // Convert text to number array
			};

			const updatedTemplate = await garmApi.updateTemplate(template.id!, params);
			template = updatedTemplate;

			toastStore.add({
				type: 'success',
				title: 'Template updated',
				message: `Template "${editForm.name}" has been updated successfully.`
			});

			// Stay on the same page, but update the original values
			originalForm.name = editForm.name;
			originalForm.description = editForm.description;
			originalTemplateContent = editTemplateContent;

			editMode = false;
		} catch (err) {
			const errorMsg = extractAPIError(err);
			toastStore.add({
				type: 'error',
				title: 'Failed to update template',
				message: errorMsg
			});
		}
	}

	async function handleDeleteTemplate() {
		if (!template?.id) return;

		try {
			await garmApi.deleteTemplate(template.id!);

			toastStore.add({
				type: 'success',
				title: 'Template deleted',
				message: `Template "${template.name}" has been deleted successfully.`
			});

			goto(resolve('/templates'));
		} catch (err) {
			const errorMsg = extractAPIError(err);
			toastStore.add({
				type: 'error',
				title: 'Failed to delete template',
				message: errorMsg
			});
		}
	}

	function enableEditMode() {
		editMode = true;
		manualLanguageOverride = false; // Reset manual override when entering edit mode
	}

	function checkForUnsavedChanges(): boolean {
		return (
			editForm.name !== originalForm.name ||
			editForm.description !== originalForm.description ||
			editTemplateContent !== originalTemplateContent
		);
	}

	function cancelEdit() {
		// Check for unsaved changes
		if (checkForUnsavedChanges()) {
			showUnsavedChangesModal = true;
			return;
		}

		performCancel();
	}

	function performCancel() {
		editMode = false;
		manualLanguageOverride = false; // Reset manual override when cancelling edit

		// If we came from the list (edit=true parameter), navigate back to list
		if (page.url.searchParams.get('edit') === 'true') {
			goto(resolve('/templates'));
			return;
		}

		// Reset form to original values
		if (template) {
			editForm.name = template.name || '';
			editForm.description = template.description || '';
			if (template.data) {
				// Convert number array to text content
				try {
					if (Array.isArray(template.data)) {
						// Convert number array to Uint8Array, then decode to text
						const uint8Array = new Uint8Array(template.data);
						editTemplateContent = new TextDecoder().decode(uint8Array);
					} else {
						// If it's a string (base64), decode it
						editTemplateContent = atob(template.data as string);
					}
				} catch (err) {
					console.error('Failed to decode template data:', err);
					editTemplateContent = '';
				}
			} else {
				editTemplateContent = '';
			}
		}
	}

	function discardChanges() {
		showUnsavedChangesModal = false;
		performCancel();
	}

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

	function getTemplateContent(): string {
		if (!template?.data) return '';
		// Convert byte array back to original text content
		try {
			if (Array.isArray(template.data)) {
				// Convert number array (bytes) to Uint8Array, then decode as UTF-8 text
				const uint8Array = new Uint8Array(template.data);
				return new TextDecoder().decode(uint8Array);
			} else {
				// Fallback: if it's a string (base64), decode it
				return atob(template.data as string);
			}
		} catch (err) {
			console.error('Failed to decode template data:', err);
			return 'Error: Failed to decode template content';
		}
	}

	function detectLanguageFromContent(content: string): typeof selectedLanguage {
		const lines = content.split('\n');
		const firstLine = lines[0]?.trim() || '';

		// Check shebang patterns
		if (firstLine.startsWith('#!/bin/bash') || firstLine.startsWith('#!/bin/sh')) {
			return 'bash';
		}
		if (firstLine.startsWith('#!/usr/bin/env pwsh') || firstLine.includes('#ps1_sysnative')) {
			return 'powershell';
		}
		if (firstLine.startsWith('#!/usr/bin/env python') || firstLine.startsWith('#!/usr/bin/python')) {
			return 'python';
		}

		// Check for PowerShell patterns
		if (content.includes('param(') || content.includes('Write-Host') || content.includes('$_')) {
			return 'powershell';
		}

		// Check for Python patterns
		if (content.includes('def ') || content.includes('import ') || content.includes('print(')) {
			return 'python';
		}

		// Check for bash patterns
		if (content.includes('echo ') || content.includes('export ') || content.includes('if [')) {
			return 'bash';
		}

		return 'text';
	}

	// Update selected language when content changes, only if not manually overridden
	$: if (editTemplateContent && !manualLanguageOverride) {
		selectedLanguage = detectLanguageFromContent(editTemplateContent);
	}

	// Check if save button should be enabled - reactive to all form fields and editor content
	$: hasChanges = (
		editForm.name !== originalForm.name ||
		editForm.description !== originalForm.description ||
		editTemplateContent !== originalTemplateContent
	);
	$: canSave = hasChanges && editForm.name.trim().length > 0 && editTemplateContent.trim().length > 0;

	// Copy to clipboard functionality
	async function copyToClipboard(text: string) {
		try {
			// Check if Clipboard API is available
			if (navigator.clipboard && navigator.clipboard.writeText) {
				await navigator.clipboard.writeText(text);
				toastStore.add({
					type: 'success',
					title: 'Copied to clipboard',
					message: 'Template content has been copied to your clipboard.'
				});
			} else {
				// Fallback for older browsers or non-HTTPS contexts
				fallbackCopyToClipboard(text);
			}
		} catch (err) {
			console.error('Failed to copy to clipboard:', err);
			// Try fallback method if clipboard API fails
			fallbackCopyToClipboard(text);
		}
	}

	// Fallback copy method for browsers without Clipboard API support
	function fallbackCopyToClipboard(text: string) {
		try {
			// Create a temporary textarea element
			const textArea = document.createElement('textarea');
			textArea.value = text;
			textArea.style.position = 'fixed';
			textArea.style.left = '-999999px';
			textArea.style.top = '-999999px';
			document.body.appendChild(textArea);
			textArea.focus();
			textArea.select();

			// Execute copy command
			const successful = document.execCommand('copy');
			document.body.removeChild(textArea);

			if (successful) {
				toastStore.add({
					type: 'success',
					title: 'Copied to clipboard',
					message: 'Template content has been copied to your clipboard.'
				});
			} else {
				throw new Error('Copy command failed');
			}
		} catch (err) {
			console.error('Fallback copy failed:', err);
			toastStore.add({
				type: 'error',
				title: 'Copy failed',
				message: 'Unable to copy to clipboard. Please manually select and copy the content.'
			});
		}
	}

	onMount(() => {
		if (templateId) {
			loadTemplate().then(() => {
				// Check if edit mode should be enabled via URL parameters
				if (page.url.searchParams.get('edit') === 'true') {
					enableEditMode();
				}
			});

			// Subscribe to websocket events for this specific template
			const unsubscribe = websocketStore.subscribeToEntity('template', ['update', 'delete'], (event) => {
				// Only handle events for this specific template
				if (event.payload && event.payload.id === templateId) {
					if (event.operation === 'update') {
						// Only reload if not in edit mode to avoid losing user changes
						if (!editMode) {
							loadTemplate();
						} else {
							// Show a notification that the template was updated elsewhere
							toastStore.add({
								type: 'info',
								title: 'Template updated',
								message: 'This template has been updated externally. Your changes are preserved.'
							});
						}
					} else if (event.operation === 'delete') {
						// Redirect to templates list if this template is deleted
						toastStore.add({
							type: 'info',
							title: 'Template deleted',
							message: `Template "${template?.name || 'Unknown'}" has been deleted.`
						});
						goto(resolve('/templates'));
					}
				}
			});

			// Cleanup subscription on component destroy
			return unsubscribe;
		} else {
			error = 'Invalid template ID';
			loading = false;
		}
	});
</script>

<svelte:head>
	<title>{template?.name || 'Template'} - GARM</title>
</svelte:head>

{#if loading}
	<div class="flex items-center justify-center py-12">
		<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
		<span class="ml-3 text-gray-600 dark:text-gray-400">Loading template...</span>
	</div>
{:else if error && !template}
	<div class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md p-4">
		<div class="flex">
			<div class="flex-shrink-0">
				<svg class="h-5 w-5 text-red-400" viewBox="0 0 20 20" fill="currentColor">
					<path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
				</svg>
			</div>
			<div class="ml-3">
				<h3 class="text-sm font-medium text-red-800 dark:text-red-200">
					Error loading template
				</h3>
				<div class="mt-2 text-sm text-red-700 dark:text-red-300">
					{error}
				</div>
				<div class="mt-4 space-x-3">
					<ActionButton variant="secondary" size="sm" on:click={loadTemplate}>
						Try Again
					</ActionButton>
					<ActionButton variant="secondary" size="sm" href={resolve('/templates')}>
						Back to Templates
					</ActionButton>
				</div>
			</div>
		</div>
	</div>
{:else if template}
	{@const isAdmin = isCurrentUserAdmin()}
	{@const isSystemTemplate = template.owner_id === 'system'}
	{@const canEdit = isAdmin || !isSystemTemplate}
	{@const canDelete = isAdmin || !isSystemTemplate}

	<!-- Header -->
	<DetailHeader
		title={template.name || 'Unnamed Template'}
		subtitle="View and manage template details"
		forgeIcon={getForgeIcon(template.forge_type || 'unknown')}
		onEdit={editMode ? cancelEdit : (canEdit ? enableEditMode : null)}
		onDelete={editMode ? handleUpdateTemplate : (canDelete ? () => showDeleteModal = true : null)}
		editLabel={editMode ? 'Close' : 'Edit'}
		deleteLabel={editMode ? 'Save Changes' : 'Delete'}
		editVariant={editMode ? 'secondary' : 'secondary'}
		deleteVariant={editMode ? 'primary' : 'danger'}
		deleteDisabled={editMode ? !canSave : false}
		editIcon={editMode ? "<path stroke-linecap='round' stroke-linejoin='round' stroke-width='2' d='M6 18L18 6M6 6l12 12'/>" : "<path stroke-linecap='round' stroke-linejoin='round' stroke-width='2' d='M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z'/>"}
		deleteIcon={editMode ? "<path stroke-linecap='round' stroke-linejoin='round' stroke-width='2' d='M8 7H5a2 2 0 00-2 2v9a2 2 0 002 2h8a2 2 0 002-2V9a2 2 0 00-2-2h-3m0 0V5a2 2 0 00-2-2H9a2 2 0 00-2 2v2m0 0h4'/>" : "<path stroke-linecap='round' stroke-linejoin='round' stroke-width='2' d='M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16'/>"}
	/>


	<div class="flex flex-col h-full space-y-6">
		<!-- Template Information -->
		<div class="bg-white dark:bg-gray-800 shadow rounded-lg flex-shrink-0">
			<div class="px-4 py-5 sm:p-6">
				<h3 class="text-lg leading-6 font-medium text-gray-900 dark:text-white mb-4">
					Template Information
				</h3>

				{#if editMode}
					<form on:submit|preventDefault={handleUpdateTemplate} class="space-y-4">
						<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
							<div>
								<label for="name" class="block text-sm font-medium text-gray-700 dark:text-gray-300">
									Name *
								</label>
								<input
									type="text"
									id="name"
									bind:value={editForm.name}
									required
									class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white sm:text-sm"
								/>
							</div>

							<div>
								<label for="description" class="block text-sm font-medium text-gray-700 dark:text-gray-300">
									Description
								</label>
								<input
									type="text"
									id="description"
									bind:value={editForm.description}
									class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white sm:text-sm"
								/>
							</div>
						</div>

						<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
							<div>
								<span class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
									Forge Type
								</span>
								{#if template.forge_type}
									{@const badgeInfo = getForgeTypeBadge(template.forge_type)}
									<Badge variant={badgeInfo.color} text={badgeInfo.text} />
								{/if}
								<p class="text-xs text-gray-500 dark:text-gray-400 mt-1">Cannot be changed</p>
							</div>

							<div>
								<span class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
									OS Type
								</span>
								{#if template.os_type}
									{@const badgeInfo = getOSTypeBadge(template.os_type)}
									<Badge variant={badgeInfo.color} text={badgeInfo.text} />
								{/if}
								<p class="text-xs text-gray-500 dark:text-gray-400 mt-1">Cannot be changed</p>
							</div>
						</div>
					</form>
				{:else}
					<dl class="grid grid-cols-1 gap-x-4 gap-y-6 sm:grid-cols-2">
						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Name</dt>
							<dd class="mt-1 text-sm text-gray-900 dark:text-white">{template.name || 'Unnamed Template'}</dd>
						</div>

						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Description</dt>
							<dd class="mt-1 text-sm text-gray-900 dark:text-white">{template.description || 'No description'}</dd>
						</div>

						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Forge Type</dt>
							<dd class="mt-1">
								{#if template.forge_type}
									{@const badgeInfo = getForgeTypeBadge(template.forge_type)}
									<Badge variant={badgeInfo.color} text={badgeInfo.text} />
								{/if}
							</dd>
						</div>

						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">OS Type</dt>
							<dd class="mt-1">
								{#if template.os_type}
									{@const badgeInfo = getOSTypeBadge(template.os_type)}
									<Badge variant={badgeInfo.color} text={badgeInfo.text} />
								{/if}
							</dd>
						</div>

						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Owner</dt>
							<dd class="mt-1 text-sm text-gray-900 dark:text-white">
								{template.owner_id === 'system' ? 'System' : (template.owner_id || 'Unknown')}
							</dd>
						</div>

						<div>
							<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Template ID</dt>
							<dd class="mt-1 text-sm text-gray-900 dark:text-white">{template.id}</dd>
						</div>

						{#if template.created_at}
							<div>
								<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Created</dt>
								<dd class="mt-1 text-sm text-gray-900 dark:text-white">
									{new Date(template.created_at).toLocaleDateString()}
								</dd>
							</div>
						{/if}

						{#if template.updated_at}
							<div>
								<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Last Updated</dt>
								<dd class="mt-1 text-sm text-gray-900 dark:text-white">
									{new Date(template.updated_at).toLocaleDateString()}
								</dd>
							</div>
						{/if}
					</dl>
				{/if}
			</div>
		</div>

		<!-- Template Content -->
		<div class="bg-white dark:bg-gray-800 shadow rounded-lg flex-1 flex flex-col min-h-0">
			<div class="px-4 py-5 sm:p-6 flex-1 flex flex-col">
				<div class="flex items-center justify-between mb-4">
					<h3 class="text-lg leading-6 font-medium text-gray-900 dark:text-white">
						Template Content
					</h3>
					{#if editMode}
						<div class="flex items-center space-x-3">
							<label for="language-select" class="text-sm font-medium text-gray-700 dark:text-gray-300">
								Language:
							</label>
							<select
								id="language-select"
								bind:value={selectedLanguage}
								on:change={() => manualLanguageOverride = true}
								class="px-3 py-1 text-sm border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
							>
								<option value="bash">Bash</option>
								<option value="powershell">PowerShell</option>
								<option value="python">Python</option>
								<option value="text">Text</option>
							</select>
						</div>
					{/if}
				</div>

				{#if editMode}
					<!-- Template Context Help -->
					<div class="mb-4 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-4">
						<h4 class="text-sm font-medium text-blue-900 dark:text-blue-100 mb-2">
							Available Template Variables
						</h4>
						<p class="text-xs text-blue-800 dark:text-blue-200 mb-3">
							Your template can use the following variables using Go template syntax (e.g., <code class="bg-blue-100 dark:bg-blue-800 px-1 rounded">{'{{ .RunnerName }}'}</code>):
						</p>
						<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 text-xs">
							<div>
								<h5 class="font-medium text-blue-900 dark:text-blue-100 mb-1">Runner Info</h5>
								<ul class="space-y-1 text-blue-800 dark:text-blue-200">
									<li><code class="bg-blue-100 dark:bg-blue-800 px-1 rounded">.RunnerName</code> - Runner name</li>
									<li><code class="bg-blue-100 dark:bg-blue-800 px-1 rounded">.RunnerLabels</code> - Comma separated labels</li>
									<li><code class="bg-blue-100 dark:bg-blue-800 px-1 rounded">.RunnerUsername</code> - Runner service username</li>
									<li><code class="bg-blue-100 dark:bg-blue-800 px-1 rounded">.RunnerGroup</code> - Runner service group</li>
									<li><code class="bg-blue-100 dark:bg-blue-800 px-1 rounded">.GitHubRunnerGroup</code> - GitHub runner group</li>
								</ul>
							</div>
							<div>
								<h5 class="font-medium text-blue-900 dark:text-blue-100 mb-1">Download & Install</h5>
								<ul class="space-y-1 text-blue-800 dark:text-blue-200">
									<li><code class="bg-blue-100 dark:bg-blue-800 px-1 rounded">.FileName</code> - Download file name</li>
									<li><code class="bg-blue-100 dark:bg-blue-800 px-1 rounded">.DownloadURL</code> - Runner download URL</li>
									<li><code class="bg-blue-100 dark:bg-blue-800 px-1 rounded">.TempDownloadToken</code> - Download token</li>
									<li><code class="bg-blue-100 dark:bg-blue-800 px-1 rounded">.RepoURL</code> - Repository URL</li>
								</ul>
							</div>
							<div>
								<h5 class="font-medium text-blue-900 dark:text-blue-100 mb-1">Configuration</h5>
								<ul class="space-y-1 text-blue-800 dark:text-blue-200">
									<li><code class="bg-blue-100 dark:bg-blue-800 px-1 rounded">.MetadataURL</code> - Instance metadata URL</li>
									<li><code class="bg-blue-100 dark:bg-blue-800 px-1 rounded">.CallbackURL</code> - Status callback URL</li>
									<li><code class="bg-blue-100 dark:bg-blue-800 px-1 rounded">.CallbackToken</code> - Callback token</li>
									<li><code class="bg-blue-100 dark:bg-blue-800 px-1 rounded">.CABundle</code> - CA certificate bundle</li>
									<li><code class="bg-blue-100 dark:bg-blue-800 px-1 rounded">.EnableBootDebug</code> - Enable debug mode</li>
									<li><code class="bg-blue-100 dark:bg-blue-800 px-1 rounded">.UseJITConfig</code> - Use JIT configuration</li>
									<li><code class="bg-blue-100 dark:bg-blue-800 px-1 rounded">.ExtraContext</code> - Additional context map</li>
								</ul>
							</div>
						</div>
						<p class="text-xs text-blue-700 dark:text-blue-300 mt-3">
							💡 <strong>Tip:</strong> Use <code class="bg-blue-100 dark:bg-blue-800 px-1 rounded">{'{{if .CABundle}}...{{end}}'}</code> for conditional content, or <code class="bg-blue-100 dark:bg-blue-800 px-1 rounded">{'{{range $key, $value := .ExtraContext}}{{$key}}: {{$value}}{{end}}'}</code> to iterate over extra context.
						</p>
					</div>
				{/if}

				{#if editMode}
					<div class="flex-1 min-h-0">
						<CodeEditor
							bind:value={editTemplateContent}
							language={selectedLanguage}
							autoDetect={!manualLanguageOverride}
							enableTemplateCompletion={true}
							minHeight="400px"
							placeholder="Enter your template script content here..."
							on:change={(e) => editTemplateContent = e.detail.value}
						/>
						<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
							Template content should be a {template.os_type === 'windows' ? 'PowerShell' : 'bash'} script for runner installation and configuration on {template.os_type}.
						</p>
					</div>
				{:else}
					<div class="flex items-center justify-between mb-4">
						<div></div> <!-- Empty spacer -->
						<button
							type="button"
							on:click={() => copyToClipboard(getTemplateContent())}
							class="inline-flex items-center px-3 py-2 border border-gray-300 dark:border-gray-600 shadow-sm text-sm leading-4 font-medium rounded-md text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 transition-colors"
							title="Copy template content to clipboard"
						>
							<svg class="h-4 w-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
							</svg>
							Copy Code
						</button>
					</div>
					<div class="flex-1 min-h-0">
						<CodeEditor
							value={getTemplateContent()}
							language={detectLanguageFromContent(getTemplateContent())}
							readonly={true}
							minHeight="400px"
						/>
					</div>
				{/if}
			</div>
		</div>
	</div>
{/if}

<!-- Delete Template Modal -->
{#if showDeleteModal && template}
	<DeleteModal
		title="Delete Template"
		message="Are you sure you want to delete this template? This action cannot be undone."
		itemName={template.name}
		on:close={() => showDeleteModal = false}
		on:confirm={handleDeleteTemplate}
	/>
{/if}

<!-- Unsaved Changes Modal -->
{#if showUnsavedChangesModal}
	<ConfirmationModal
		title="Unsaved Changes"
		message="You have unsaved changes. Are you sure you want to discard them?"
		confirmText="Discard Changes"
		cancelText="Stay on Page"
		variant="warning"
		on:close={() => showUnsavedChangesModal = false}
		on:confirm={discardChanges}
	/>
{/if}