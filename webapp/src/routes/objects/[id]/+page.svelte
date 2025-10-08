<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { garmApi } from '$lib/api/client.js';
	import type { FileObject } from '$lib/api/generated/api.js';
	import { toastStore } from '$lib/stores/toast.js';
	import { extractAPIError } from '$lib/utils/apiError';
	import { formatFileSize, formatDateTime } from '$lib/utils/format';
	import DeleteModal from '$lib/components/DeleteModal.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import Button from '$lib/components/Button.svelte';
	import Badge from '$lib/components/Badge.svelte';

	let object: FileObject | null = null;
	let loading = true;
	let error = '';
	let showDeleteModal = false;
	let showUpdateModal = false;

	// Update form
	let updateForm = {
		name: '',
		tags: [] as string[],
		description: ''
	};
	let updateTagInput = '';

	$: objectId = $page.params.id || '';

	onMount(async () => {
		await loadObject();
	});

	async function loadObject() {
		if (!objectId) {
			error = 'Invalid object ID';
			loading = false;
			return;
		}

		try {
			loading = true;
			error = '';

			object = await garmApi.getFileObject(objectId);

		} catch (err) {
			error = extractAPIError(err);
			toastStore.add({
				type: 'error',
				title: 'Failed to load object',
				message: error
			});
		} finally {
			loading = false;
		}
	}

	async function handleDelete() {
		if (!object?.id) return;

		try {
			await garmApi.deleteFileObject(object.id.toString());

			toastStore.add({
				type: 'success',
				title: 'Object deleted',
				message: `Object "${object.name}" has been deleted successfully.`
			});

			goto(resolve('/objects'));
		} catch (err) {
			const errorMsg = extractAPIError(err);
			toastStore.add({
				type: 'error',
				title: 'Failed to delete object',
				message: errorMsg
			});
		}
	}

	async function handleDownload() {
		if (!object?.id) return;

		try {
			const baseURL = import.meta.env.VITE_API_BASE_URL || window.location.origin;
			const downloadURL = `${baseURL}/api/v1/objects/${object.id}/download`;

			// First, check if the file is accessible (fast HEAD request)
			const checkResponse = await fetch(downloadURL, {
				method: 'HEAD',
				credentials: 'include' // Ensures cookies (garm_token) are sent
			});

			if (!checkResponse.ok) {
				const errorText = await checkResponse.text();
				throw new Error(errorText || `Download failed with status ${checkResponse.status}`);
			}

			// If accessible, use direct link (browser handles download with progress)
			const link = document.createElement('a');
			link.href = downloadURL;
			link.download = object.name || 'download';
			document.body.appendChild(link);
			link.click();
			document.body.removeChild(link);
		} catch (err) {
			const errorMsg = extractAPIError(err);
			toastStore.add({
				type: 'error',
				title: 'Download failed',
				message: errorMsg
			});
		}
	}

	function openUpdateModal() {
		if (!object) return;

		updateForm = {
			name: object.name || '',
			tags: object.tags || [],
			description: object.description || ''
		};
		updateTagInput = '';
		showUpdateModal = true;
	}

	async function handleUpdate() {
		if (!object?.id) return;

		try {
			await garmApi.updateFileObject(object.id.toString(), {
				name: updateForm.name || undefined,
				tags: updateForm.tags,
				description: updateForm.description || undefined
			});

			toastStore.add({
				type: 'success',
				title: 'Object updated',
				message: `Object has been updated successfully.`
			});

			showUpdateModal = false;
			await loadObject(); // Reload to show updated data
		} catch (err) {
			const errorMsg = extractAPIError(err);
			toastStore.add({
				type: 'error',
				title: 'Failed to update object',
				message: errorMsg
			});
		}
	}

	function handleTagKeydown(event: KeyboardEvent) {
		const target = event.target as HTMLInputElement;
		const value = target.value.trim();

		if ((event.key === ' ' || event.key === 'Enter') && value) {
			event.preventDefault();
			addTag(value);
			updateTagInput = '';
		} else if (event.key === 'Backspace' && !value && updateForm.tags.length > 0) {
			event.preventDefault();
			removeTag(updateForm.tags.length - 1);
		}
	}

	function handleTagBlur() {
		const value = updateTagInput.trim();
		if (value) {
			addTag(value);
			updateTagInput = '';
		}
	}

	function addTag(tag: string) {
		const trimmed = tag.trim().toLowerCase();
		if (!trimmed) return;

		if (!updateForm.tags.includes(trimmed)) {
			updateForm.tags = [...updateForm.tags, trimmed];
		}
	}

	function removeTag(index: number) {
		updateForm.tags = updateForm.tags.filter((_, i) => i !== index);
	}
</script>

<!-- Breadcrumb -->
<nav class="flex mb-6" aria-label="Breadcrumb">
	<ol class="inline-flex items-center space-x-1 md:space-x-3">
		<li class="inline-flex items-center">
			<a href={resolve('/objects')} class="inline-flex items-center text-sm font-medium text-gray-700 dark:text-gray-300 hover:text-blue-600 dark:hover:text-blue-400">
				Object Storage
			</a>
		</li>
		<li>
			<div class="flex items-center">
				<svg class="w-6 h-6 text-gray-400" fill="currentColor" viewBox="0 0 20 20">
					<path fill-rule="evenodd" d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z" clip-rule="evenodd"/>
				</svg>
				<span class="text-sm font-medium text-gray-500 dark:text-gray-400 ml-1 md:ml-2">
					{object ? object.name : 'Object Details'}
				</span>
			</div>
		</li>
	</ol>
</nav>

{#if error}
	<div class="bg-red-50 dark:bg-red-900/50 border border-red-200 dark:border-red-800 rounded-md p-4 mb-6">
		<div class="flex">
			<div class="ml-3">
				<h3 class="text-sm font-medium text-red-800 dark:text-red-200">Error</h3>
				<div class="mt-2 text-sm text-red-700 dark:text-red-300">{error}</div>
			</div>
		</div>
	</div>
{/if}

{#if loading}
	<div class="bg-white dark:bg-gray-800 shadow rounded-lg">
		<div class="px-6 py-4 text-center">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600 mx-auto"></div>
			<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">Loading object details...</p>
		</div>
	</div>
{:else if object}
	<!-- Object Information Cards -->
	<div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
		<!-- File Information -->
		<div class="bg-white dark:bg-gray-800 shadow rounded-lg p-6">
			<div class="flex items-center justify-between mb-4">
				<h3 class="text-lg font-medium text-gray-900 dark:text-white">File Information</h3>
				<div class="flex items-center space-x-3">
					<button
						on:click={openUpdateModal}
						class="px-4 py-2 bg-gray-600 hover:bg-gray-700 dark:bg-gray-700 dark:hover:bg-gray-800 text-white rounded-lg font-medium text-sm cursor-pointer"
					>
						Edit
					</button>
					<button
						on:click={handleDownload}
						class="px-4 py-2 bg-blue-600 hover:bg-blue-700 dark:bg-blue-700 dark:hover:bg-blue-800 text-white rounded-lg font-medium text-sm cursor-pointer"
					>
						Download
					</button>
					<button
						on:click={() => showDeleteModal = true}
						class="px-4 py-2 bg-red-600 hover:bg-red-700 dark:bg-red-700 dark:hover:bg-red-800 text-white rounded-lg font-medium text-sm cursor-pointer"
					>
						Delete
					</button>
				</div>
			</div>
			<dl class="space-y-3">
				<div class="flex justify-between">
					<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">ID:</dt>
					<dd class="text-sm font-mono text-gray-900 dark:text-white break-all">{object.id}</dd>
				</div>
				<div class="flex justify-between">
					<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Name:</dt>
					<dd class="text-sm text-gray-900 dark:text-white">{object.name}</dd>
				</div>
				<div class="flex justify-between">
					<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Size:</dt>
					<dd class="text-sm text-gray-900 dark:text-white">{formatFileSize(object.size || 0)}</dd>
				</div>
				<div class="flex justify-between">
					<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">File Type:</dt>
					<dd class="text-sm text-gray-900 dark:text-white">{object.file_type || 'N/A'}</dd>
				</div>
				<div>
					<dt class="text-sm font-medium text-gray-500 dark:text-gray-400 mb-2">SHA256:</dt>
					<dd class="text-xs font-mono text-gray-900 dark:text-white bg-gray-50 dark:bg-gray-700 p-2 rounded break-all">
						{object.sha256 || 'N/A'}
					</dd>
				</div>
			</dl>
		</div>

		<!-- Metadata & Timestamps -->
		<div class="bg-white dark:bg-gray-800 shadow rounded-lg p-6">
			<h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">Metadata & Timestamps</h3>
			<dl class="space-y-3">
				<div class="flex justify-between">
					<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Created At:</dt>
					<dd class="text-sm text-gray-900 dark:text-white">{formatDateTime(object.created_at)}</dd>
				</div>
				<div class="flex justify-between">
					<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Updated At:</dt>
					<dd class="text-sm text-gray-900 dark:text-white">{formatDateTime(object.updated_at)}</dd>
				</div>
				<div>
					<dt class="text-sm font-medium text-gray-500 dark:text-gray-400 mb-2">Tags:</dt>
					<dd class="text-sm">
						{#if object.tags && object.tags.length > 0}
							<div class="flex flex-wrap gap-2">
								{#each object.tags as tag}
									<Badge variant="blue" text={tag} />
								{/each}
							</div>
						{:else}
							<span class="text-gray-500 dark:text-gray-400 italic">No tags</span>
						{/if}
					</dd>
				</div>
			</dl>
		</div>
	</div>

	<!-- Description Card (Full Width) -->
	{#if object.description}
		<div class="mt-6 bg-white dark:bg-gray-800 shadow rounded-lg p-6">
			<h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">Description</h3>
			<div class="text-sm text-gray-900 dark:text-white whitespace-pre-wrap break-words">
				{object.description}
			</div>
		</div>
	{/if}
{/if}

<!-- Update Modal -->
{#if showUpdateModal && object}
	<Modal on:close={() => showUpdateModal = false}>
		<div class="max-w-2xl w-full p-6">
			<h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">Update Object</h3>

			<form on:submit|preventDefault={handleUpdate} class="space-y-4">
				<div>
					<label for="update-file-name" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						File Name
					</label>
					<input
						id="update-file-name"
						type="text"
						bind:value={updateForm.name}
						class="block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white sm:text-sm"
						placeholder="my-file.bin"
					/>
				</div>

				<div>
					<label for="update-file-tags" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						Tags
					</label>
					<div class="block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm bg-white dark:bg-gray-700 min-h-[42px] flex flex-wrap gap-2 items-center">
						{#each updateForm.tags as tag, index}
							<span class="inline-flex items-center gap-1 px-2 py-1 text-sm font-medium text-blue-700 dark:text-blue-300 bg-blue-100 dark:bg-blue-900 rounded-md">
								{tag}
								<button
									type="button"
									on:click={() => removeTag(index)}
									class="inline-flex items-center justify-center w-4 h-4 text-blue-700 dark:text-blue-300 hover:text-blue-900 dark:hover:text-blue-100 rounded-full hover:bg-blue-200 dark:hover:bg-blue-800"
									aria-label="Remove tag {tag}"
								>
									<svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
									</svg>
								</button>
							</span>
						{/each}
						<input
							id="update-file-tags"
							type="text"
							bind:value={updateTagInput}
							on:keydown={handleTagKeydown}
							on:blur={handleTagBlur}
							class="flex-1 min-w-[120px] bg-transparent border-none outline-none text-gray-900 dark:text-white text-sm"
							placeholder={updateForm.tags.length === 0 ? 'Type and press Space or Enter' : ''}
						/>
					</div>
					<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
						Press Space or Enter to add a tag. Press Backspace on empty field to remove last tag.
					</p>
				</div>

				<div>
					<label for="update-file-description" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						Description (optional)
					</label>
					<textarea
						id="update-file-description"
						bind:value={updateForm.description}
						maxlength="8192"
						rows="3"
						class="block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white sm:text-sm resize-y"
						placeholder="Add a description for this file..."
					></textarea>
					<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
						{updateForm.description.length} / 8192 characters
					</p>
				</div>

				<div class="flex justify-end gap-3 pt-4">
					<Button
						type="button"
						variant="secondary"
						on:click={() => showUpdateModal = false}
					>
						Cancel
					</Button>
					<Button
						type="submit"
						variant="primary"
					>
						Update Object
					</Button>
				</div>
			</form>
		</div>
	</Modal>
{/if}

<!-- Delete Modal -->
{#if showDeleteModal && object}
	<DeleteModal
		title="Delete Object"
		message="Are you sure you want to delete the object '{object.name}'? This action cannot be undone."
		on:confirm={handleDelete}
		on:close={() => showDeleteModal = false}
	/>
{/if}
