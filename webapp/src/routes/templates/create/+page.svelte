<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { garmApi } from '$lib/api/client.js';
	import type { Template, CreateTemplateParams } from '$lib/api/generated/api.js';
	import ActionButton from '$lib/components/ActionButton.svelte';
	import { toastStore } from '$lib/stores/toast.js';
	import { extractAPIError } from '$lib/utils/apiError';
	import Badge from '$lib/components/Badge.svelte';
	import DetailHeader from '$lib/components/DetailHeader.svelte';
	import CodeEditor from '$lib/components/CodeEditor.svelte';
	import ConfirmationModal from '$lib/components/ConfirmationModal.svelte';

	let loading = false;
	let cloneTemplate: Template | null = null;

	// Create form
	let createForm = {
		name: '',
		description: '',
		forge_type: 'github',
		os_type: 'linux',
		data: new Uint8Array()
	};

	// Template content (convert data to string for editing)
	let createTemplateContent = '';
	let selectedLanguage: 'bash' | 'powershell' | 'python' | 'text' = 'text';
	let manualLanguageOverride = false;

	// File upload
	let fileInput: HTMLInputElement;
	let uploading = false;

	// Track original values for change detection
	let originalForm = {
		name: '',
		description: '',
		forge_type: 'github',
		os_type: 'linux'
	};
	let originalTemplateContent = '';

	// Unsaved changes modal
	let showUnsavedChangesModal = false;

	// Check for unsaved changes
	$: hasChanges = (
		createForm.name !== originalForm.name ||
		createForm.description !== originalForm.description ||
		createForm.forge_type !== originalForm.forge_type ||
		createForm.os_type !== originalForm.os_type ||
		createTemplateContent !== originalTemplateContent
	);

	$: canSave = createForm.name.trim().length > 0 && createTemplateContent.trim().length > 0;

	async function handleCreateTemplate() {
		if (!canSave) return;

		try {
			loading = true;
			const params: CreateTemplateParams = {
				name: createForm.name,
				description: createForm.description || undefined,
				forge_type: createForm.forge_type,
				os_type: createForm.os_type,
				data: Array.from(new TextEncoder().encode(createTemplateContent)) // Convert text to number array
			};

			await garmApi.createTemplate(params);

			toastStore.add({
				type: 'success',
				title: 'Template created',
				message: `Template "${createForm.name}" has been created successfully.`
			});

			goto(resolve('/templates'));
		} catch (err) {
			const errorMsg = extractAPIError(err);
			toastStore.add({
				type: 'error',
				title: 'Failed to create template',
				message: errorMsg
			});
		} finally {
			loading = false;
		}
	}

	function checkForUnsavedChanges(): boolean {
		return hasChanges;
	}

	function cancelCreate() {
		// Check for unsaved changes
		if (checkForUnsavedChanges()) {
			showUnsavedChangesModal = true;
			return;
		}

		performCancel();
	}

	function performCancel() {
		goto(resolve('/templates'));
	}

	function discardChanges() {
		showUnsavedChangesModal = false;
		performCancel();
	}

	function handleFileUpload() {
		fileInput.click();
	}

	async function onFileSelected(event: Event) {
		const input = event.target as HTMLInputElement;
		const file = input.files?.[0];
		
		if (!file) return;

		// Check file size (limit to 1MB)
		if (file.size > 1024 * 1024) {
			toastStore.add({
				type: 'error',
				title: 'File too large',
				message: 'Please select a file smaller than 1MB.'
			});
			return;
		}

		try {
			uploading = true;
			const text = await file.text();
			createTemplateContent = text;

			// Auto-detect language from filename or content
			const fileName = file.name.toLowerCase();
			let detectedLanguage: typeof selectedLanguage = 'text';

			if (fileName.endsWith('.sh') || fileName.endsWith('.bash')) {
				detectedLanguage = 'bash';
			} else if (fileName.endsWith('.ps1') || fileName.endsWith('.psm1')) {
				detectedLanguage = 'powershell';
			} else if (fileName.endsWith('.py')) {
				detectedLanguage = 'python';
			} else {
				// Try to detect from content
				detectedLanguage = detectLanguageFromContent(text);
			}

			selectedLanguage = detectedLanguage;
			manualLanguageOverride = false; // Allow auto-detection to work again

			toastStore.add({
				type: 'success',
				title: 'File uploaded',
				message: `Successfully loaded content from "${file.name}".`
			});
		} catch (err) {
			toastStore.add({
				type: 'error',
				title: 'Failed to read file',
				message: 'Unable to read the selected file. Please try again.'
			});
		} finally {
			uploading = false;
			// Clear the input so the same file can be selected again
			input.value = '';
		}
	}

	function getForgeTypeBadge(forgeType: string): { color: 'success' | 'error' | 'warning' | 'info' | 'gray' | 'blue' | 'green' | 'red' | 'yellow' | 'secondary', text: string } {
		switch (forgeType) {
			case 'github':
				return { color: 'blue', text: 'GitHub' };
			case 'gitea':
				return { color: 'green', text: 'Gitea' };
			default:
				return { color: 'gray', text: forgeType || 'Unknown' };
		}
	}

	function getOSTypeBadge(osType: string): { color: 'success' | 'error' | 'warning' | 'info' | 'gray' | 'blue' | 'green' | 'red' | 'yellow' | 'secondary', text: string } {
		switch (osType) {
			case 'linux':
				return { color: 'blue', text: 'Linux' };
			case 'windows':
				return { color: 'info', text: 'Windows' };
			default:
				return { color: 'gray', text: osType || 'Unknown' };
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
	$: if (createTemplateContent && !manualLanguageOverride) {
		selectedLanguage = detectLanguageFromContent(createTemplateContent);
	}

	async function loadCloneTemplate() {
		const cloneId = page.url.searchParams.get('clone');
		if (!cloneId) return;

		try {
			loading = true;
			cloneTemplate = await garmApi.getTemplate(parseInt(cloneId));

			if (!cloneTemplate) {
				throw new Error('Template not found');
			}

			// Pre-fill form with template data
			createForm.name = `${cloneTemplate.name} (Copy)`;
			createForm.description = cloneTemplate.description || '';
			createForm.forge_type = cloneTemplate.forge_type || 'github';
			createForm.os_type = cloneTemplate.os_type || 'linux';

			// Decode template content
			if (cloneTemplate.data) {
				try {
					if (Array.isArray(cloneTemplate.data)) {
						const uint8Array = new Uint8Array(cloneTemplate.data);
						createTemplateContent = new TextDecoder().decode(uint8Array);
					} else {
						createTemplateContent = atob(cloneTemplate.data as string);
					}

					// Auto-detect language from content
					selectedLanguage = detectLanguageFromContent(createTemplateContent);
				} catch (err) {
					console.error('Failed to decode template data:', err);
					createTemplateContent = '';
				}
			}
		} catch (err) {
			const errorMsg = extractAPIError(err);
			toastStore.add({
				type: 'error',
				title: 'Failed to load template',
				message: errorMsg
			});
			goto(resolve('/templates'));
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		loadCloneTemplate();
	});
</script>

<svelte:head>
	<title>{cloneTemplate ? `Clone ${cloneTemplate.name}` : 'Create Template'} - GARM</title>
</svelte:head>

{#if loading}
	<div class="flex items-center justify-center py-12">
		<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
		<span class="ml-3 text-gray-600 dark:text-gray-400">Loading...</span>
	</div>
{:else}
	<!-- Header -->
	<DetailHeader
		title={cloneTemplate ? `Clone Template: ${cloneTemplate.name}` : 'Create New Template'}
		subtitle="Create a new runner install template"
		onEdit={cancelCreate}
		onDelete={handleCreateTemplate}
		editLabel="Cancel"
		deleteLabel="Create Template"
		editVariant="secondary"
		deleteVariant="primary"
		deleteDisabled={!canSave}
		editIcon="<path stroke-linecap='round' stroke-linejoin='round' stroke-width='2' d='M6 18L18 6M6 6l12 12'/>"
		deleteIcon="<path stroke-linecap='round' stroke-linejoin='round' stroke-width='2' d='M8 7H5a2 2 0 00-2 2v9a2 2 0 002 2h8a2 2 0 002-2V9a2 2 0 00-2-2h-3m0 0V5a2 2 0 00-2-2H9a2 2 0 00-2 2v2m0 0h4'/>"
	/>

	<div class="flex flex-col h-full space-y-6">
		<!-- Template Information -->
		<div class="bg-white dark:bg-gray-800 shadow rounded-lg flex-shrink-0">
			<div class="px-4 py-5 sm:p-6">
				<h3 class="text-lg leading-6 font-medium text-gray-900 dark:text-white mb-4">
					Template Information
				</h3>

				<form on:submit|preventDefault={handleCreateTemplate} class="space-y-4">
					<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
						<div>
							<label for="name" class="block text-sm font-medium text-gray-700 dark:text-gray-300">
								Name *
							</label>
							<input
								type="text"
								id="name"
								bind:value={createForm.name}
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
								bind:value={createForm.description}
								class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white sm:text-sm"
							/>
						</div>
					</div>

					<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
						<div>
							<label for="forge_type" class="block text-sm font-medium text-gray-700 dark:text-gray-300">
								Forge Type *
							</label>
							<select
								id="forge_type"
								bind:value={createForm.forge_type}
								required
								class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white sm:text-sm"
							>
								<option value="github">GitHub</option>
								<option value="gitea">Gitea</option>
							</select>
						</div>

						<div>
							<label for="os_type" class="block text-sm font-medium text-gray-700 dark:text-gray-300">
								OS Type *
							</label>
							<select
								id="os_type"
								bind:value={createForm.os_type}
								required
								class="mt-1 block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white sm:text-sm"
							>
								<option value="linux">Linux</option>
								<option value="windows">Windows</option>
							</select>
						</div>
					</div>
				</form>
			</div>
		</div>

		<!-- Template Content -->
		<div class="bg-white dark:bg-gray-800 shadow rounded-lg flex-1 flex flex-col min-h-0">
			<div class="px-4 py-5 sm:p-6 flex-1 flex flex-col">
				<div class="flex items-center justify-between mb-4">
					<h3 class="text-lg leading-6 font-medium text-gray-900 dark:text-white">
						Template Content
					</h3>
					<div class="flex items-center space-x-3">
						<button
							type="button"
							on:click={handleFileUpload}
							disabled={uploading}
							class="inline-flex items-center px-3 py-1.5 text-sm font-medium rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed"
						>
							{#if uploading}
								<div class="animate-spin rounded-full h-4 w-4 border-b-2 border-gray-600 mr-2"></div>
								Uploading...
							{:else}
								<svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"/>
								</svg>
								Upload File
							{/if}
						</button>
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
				</div>

				<!-- Hidden file input -->
				<input
					type="file"
					bind:this={fileInput}
					on:change={onFileSelected}
					accept=".sh,.bash,.ps1,.psm1,.py,.txt,.md"
					style="display: none"
				/>

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

				<div class="flex-1 min-h-0">
					<CodeEditor
						bind:value={createTemplateContent}
						language={selectedLanguage}
						autoDetect={!manualLanguageOverride}
						enableTemplateCompletion={true}
						minHeight="400px"
						placeholder="Enter your template script content here..."
						on:change={(e) => createTemplateContent = e.detail.value}
					/>
					<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
						Template content should be a {createForm.os_type === 'windows' ? 'PowerShell' : 'bash'} script for runner installation and configuration on {createForm.os_type}.
					</p>
				</div>
			</div>
		</div>
	</div>
{/if}

<!-- Unsaved Changes Modal -->
{#if showUnsavedChangesModal}
	<ConfirmationModal
		title="Unsaved Changes"
		message="You have unsaved changes. Are you sure you want to discard them?"
		confirmText="Discard Changes"
		cancelText="Stay on Page"
		variant="danger"
		on:close={() => showUnsavedChangesModal = false}
		on:confirm={discardChanges}
	/>
{/if}