<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { garmApi } from '$lib/api/client.js';
	import type { FileObject } from '$lib/api/generated/api.js';
	import PageHeader from '$lib/components/PageHeader.svelte';
	import ActionButton from '$lib/components/ActionButton.svelte';
	import Button from '$lib/components/Button.svelte';
	import DataTable from '$lib/components/DataTable.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import BackendPagination from '$lib/components/BackendPagination.svelte';
	import { toastStore } from '$lib/stores/toast.js';
	import { extractAPIError } from '$lib/utils/apiError';
	import DeleteModal from '$lib/components/DeleteModal.svelte';
	import { EntityCell, GenericCell, ActionsCell, TagsCell } from '$lib/components/cells';
	import { formatFileSize, formatDateTime } from '$lib/utils/format';
	import { websocketStore, type WebSocketEvent } from '$lib/stores/websocket.js';

	let objects: FileObject[] = [];
	let loading = true;
	let error = '';
	let searchTerm = '';

	// Pagination - backend pagination
	let currentPage = 1;
	let pageSize = 25; // Will be loaded from localStorage in onMount
	let totalPages = 1;
	let totalObjects = 0;

	// Track last page before search to restore when clearing search
	let pageBeforeSearch = 1;

	// Load page size from localStorage
	const PAGE_SIZE_STORAGE_KEY = 'garm_objects_page_size';

	// Prefetch cache for smooth navigation
	let pageCache = new Map<string, { results: FileObject[]; timestamp: number }>();
	const CACHE_TTL = 5 * 60 * 1000; // 5 minutes

	// Search debouncing
	let searchDebounceTimer: ReturnType<typeof setTimeout> | null = null;
	const SEARCH_DEBOUNCE_MS = 500;

	// Modals
	let showDeleteModal = false;
	let showUploadModal = false;
	let showUpdateModal = false;
	let selectedObject: FileObject | null = null;
	let showDescription = false;

	// Websocket subscription
	let unsubscribeWebsocket: (() => void) | null = null;

	// Upload form
	let uploadForm = {
		name: '',
		file: null as File | null,
		tags: [] as string[],
		description: ''
	};
	let uploadProgress = 0;
	let uploading = false;

	// Update form
	let updateForm = {
		name: '',
		tags: [] as string[],
		description: ''
	};

	function handleFileObjectEvent(event: WebSocketEvent) {
		// Handle real-time updates for file objects
		if (event.operation === 'create') {
			// For create, reload the list to get fresh data with pagination
			loadObjects();
		} else if (event.operation === 'update') {
			// Update existing object in the list
			const updatedObject = event.payload as FileObject;
			objects = objects.map(obj =>
				obj.id === updatedObject.id ? updatedObject : obj
			);
		} else if (event.operation === 'delete') {
			// Remove object from list
			const objectId = event.payload.id || event.payload;
			objects = objects.filter(obj => obj.id !== objectId);
			// Update total count
			totalObjects = Math.max(0, totalObjects - 1);
		}
	}

	onMount(async () => {
		// Load page size preference from localStorage
		const savedPageSize = localStorage.getItem(PAGE_SIZE_STORAGE_KEY);
		if (savedPageSize) {
			const parsed = parseInt(savedPageSize, 10);
			if (!isNaN(parsed) && parsed > 0) {
				pageSize = parsed;
			}
		}

		// Initial load
		await loadObjects();

		// Subscribe to real-time file object events
		unsubscribeWebsocket = websocketStore.subscribeToEntity(
			'file_object',
			['create', 'update', 'delete'],
			handleFileObjectEvent
		);
	});

	onDestroy(() => {
		// Clean up websocket subscription
		if (unsubscribeWebsocket) {
			unsubscribeWebsocket();
			unsubscribeWebsocket = null;
		}
	});

	function getCacheKey(page: number, tags?: string, size: number = pageSize): string {
		return `${page}-${tags || 'all'}-${size}`;
	}

	function cleanExpiredCache() {
		const now = Date.now();
		for (const [key, value] of pageCache.entries()) {
			if (now - value.timestamp > CACHE_TTL) {
				pageCache.delete(key);
			}
		}
	}

	async function loadObjects(useCache: boolean = false) {
		try {
			loading = !useCache; // Don't show loading if using cache
			error = '';

			// Convert search term to comma-separated tags
			const tags = searchTerm.trim() ? searchTerm.trim().replace(/\s+/g, ',') : undefined;
			const cacheKey = getCacheKey(currentPage, tags);

			// Check cache first
			if (useCache) {
				const cached = pageCache.get(cacheKey);
				if (cached && Date.now() - cached.timestamp < CACHE_TTL) {
					objects = cached.results;
					loading = false;
					return;
				}
			}

			const response = await garmApi.listFileObjects(tags, currentPage, pageSize);

			objects = response.results || [];
			totalPages = response.pages || 1;
			totalObjects = response.total_count || 0;

			// Store in cache
			pageCache.set(cacheKey, {
				results: objects,
				timestamp: Date.now()
			});

			// Clean expired cache entries
			cleanExpiredCache();

		} catch (err) {
			error = extractAPIError(err);
			toastStore.add({
				type: 'error',
				title: 'Failed to load objects',
				message: error
			});
		} finally {
			loading = false;
		}
	}

	async function prefetchPage(page: number) {
		if (page < 1 || page > totalPages) return;

		const tags = searchTerm.trim() ? searchTerm.trim().replace(/\s+/g, ',') : undefined;
		const cacheKey = getCacheKey(page, tags);

		// Skip if already cached and fresh
		const cached = pageCache.get(cacheKey);
		if (cached && Date.now() - cached.timestamp < CACHE_TTL) {
			return;
		}

		try {
			const response = await garmApi.listFileObjects(tags, page, pageSize);
			pageCache.set(cacheKey, {
				results: response.results || [],
				timestamp: Date.now()
			});
		} catch (err) {
			// Silently fail prefetch
			console.debug('Prefetch failed for page', page, err);
		}
	}

	async function handleDeleteObject() {
		if (!selectedObject?.id) return;

		try {
			await garmApi.deleteFileObject(selectedObject.id.toString());

			toastStore.add({
				type: 'success',
				title: 'Object deleted',
				message: `Object "${selectedObject.name}" has been deleted successfully.`
			});

			showDeleteModal = false;
			selectedObject = null;
			await loadObjects();
		} catch (err) {
			const errorMsg = extractAPIError(err);
			toastStore.add({
				type: 'error',
				title: 'Failed to delete object',
				message: errorMsg
			});
		}
	}

	function openUploadModal() {
		uploadForm = {
			name: '',
			file: null,
			tags: [],
			description: ''
		};
		uploadProgress = 0;
		showUploadModal = true;
	}

	function openUpdateModal(obj: FileObject) {
		selectedObject = obj;
		updateForm = {
			name: obj.name || '',
			tags: obj.tags || [],
			description: obj.description || ''
		};
		showUpdateModal = true;
	}

	function openDeleteModal(obj: FileObject) {
		selectedObject = obj;
		showDeleteModal = true;
	}

	async function handleUpload() {
		if (!uploadForm.file || !uploadForm.name.trim()) {
			toastStore.add({
				type: 'error',
				title: 'Validation error',
				message: 'Please provide a file name and select a file to upload.'
			});
			return;
		}

		try {
			uploading = true;
			uploadProgress = 0;

			const formData = new FormData();
			formData.append('file', uploadForm.file);

			// Create XMLHttpRequest for progress tracking
			const xhr = new XMLHttpRequest();

			// Track upload progress
			xhr.upload.addEventListener('progress', (e) => {
				if (e.lengthComputable) {
					uploadProgress = Math.round((e.loaded / e.total) * 100);
				}
			});

			// Handle completion
			await new Promise<void>((resolvePromise, reject) => {
				xhr.addEventListener('load', async () => {
					if (xhr.status >= 200 && xhr.status < 300) {
						resolvePromise();
					} else {
						reject(new Error(`Upload failed with status ${xhr.status}`));
					}
				});

				xhr.addEventListener('error', () => {
					reject(new Error('Upload failed'));
				});

				// Get the auth token
				const token = localStorage.getItem('token');

				// Open connection
				const baseURL = import.meta.env.VITE_API_BASE_URL || window.location.origin;
				xhr.open('POST', `${baseURL}/api/v1/objects/`);

				// Set headers
				if (token) {
					xhr.setRequestHeader('Authorization', `Bearer ${token}`);
				}
				xhr.setRequestHeader('X-File-Name', uploadForm.name);
				if (uploadForm.tags.length > 0) {
					xhr.setRequestHeader('X-Tags', uploadForm.tags.join(','));
				}
				if (uploadForm.description.trim()) {
					xhr.setRequestHeader('X-File-Description', uploadForm.description.trim());
				}

				// Send the file
				xhr.send(uploadForm.file);
			});

			toastStore.add({
				type: 'success',
				title: 'Upload successful',
				message: `File "${uploadForm.name}" has been uploaded successfully.`
			});

			showUploadModal = false;
			await loadObjects();

		} catch (err) {
			const errorMsg = extractAPIError(err);
			toastStore.add({
				type: 'error',
				title: 'Upload failed',
				message: errorMsg
			});
		} finally {
			uploading = false;
			uploadProgress = 0;
		}
	}

	async function handleUpdate() {
		if (!selectedObject?.id) return;

		try {
			await garmApi.updateFileObject(selectedObject.id.toString(), {
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
			selectedObject = null;
			await loadObjects();
		} catch (err) {
			const errorMsg = extractAPIError(err);
			toastStore.add({
				type: 'error',
				title: 'Failed to update object',
				message: errorMsg
			});
		}
	}

	async function handleDownload(obj: FileObject) {
		try {
			const baseURL = import.meta.env.VITE_API_BASE_URL || window.location.origin;
			const downloadURL = `${baseURL}/api/v1/objects/${obj.id}/download`;

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
			link.download = obj.name || 'download';
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

	function handleSearch(event: CustomEvent<{ term: string }>) {
		const newSearchTerm = event.detail.term;
		const wasSearching = searchTerm.length > 0;
		const isSearching = newSearchTerm.length > 0;

		searchTerm = newSearchTerm;

		// Clear any existing debounce timer
		if (searchDebounceTimer) {
			clearTimeout(searchDebounceTimer);
		}

		// Debounce the search - wait 500ms after user stops typing
		searchDebounceTimer = setTimeout(() => {
			// If starting a search, save current page
			if (!wasSearching && isSearching) {
				pageBeforeSearch = currentPage;
				currentPage = 1; // Reset to first page when starting search
			}
			// If clearing search, restore previous page
			else if (wasSearching && !isSearching) {
				currentPage = pageBeforeSearch;
			}
			// If still searching (changing search term), stay on page 1
			else if (isSearching) {
				currentPage = 1;
			}

			pageCache.clear(); // Clear cache on search change
			loadObjects();
			searchDebounceTimer = null;
		}, SEARCH_DEBOUNCE_MS);
	}

	function handlePageChange(event: CustomEvent<{ page: number }>) {
		currentPage = event.detail.page;
		loadObjects(true); // Try to use cache
	}

	function handlePageSizeChange(event: CustomEvent<{ pageSize: number }>) {
		pageSize = event.detail.pageSize;
		// Save to localStorage
		localStorage.setItem(PAGE_SIZE_STORAGE_KEY, pageSize.toString());
		currentPage = 1;
		pageCache.clear(); // Clear cache on page size change
		loadObjects();
	}

	function handlePrefetch(event: CustomEvent<{ page: number }>) {
		prefetchPage(event.detail.page);
	}

	function handleFileInput(event: Event) {
		const target = event.target as HTMLInputElement;
		if (target.files && target.files[0]) {
			uploadForm.file = target.files[0];
			// Always update name to match selected file
			uploadForm.name = target.files[0].name;
		}
	}

	let uploadTagInput = '';
	let updateTagInput = '';

	function handleTagKeydown(event: KeyboardEvent, form: 'upload' | 'update') {
		const target = event.target as HTMLInputElement;
		const value = target.value.trim();

		// Space or Enter key creates a tag
		if ((event.key === ' ' || event.key === 'Enter') && value) {
			event.preventDefault();
			addTag(value, form);
			if (form === 'upload') {
				uploadTagInput = '';
			} else {
				updateTagInput = '';
			}
		}
		// Backspace on empty input removes last tag
		else if (event.key === 'Backspace' && !value) {
			event.preventDefault();
			const tags = form === 'upload' ? uploadForm.tags : updateForm.tags;
			if (tags.length > 0) {
				removeTag(tags.length - 1, form);
			}
		}
	}

	function handleTagBlur(form: 'upload' | 'update') {
		// Add any remaining text as a tag when field loses focus
		const value = form === 'upload' ? uploadTagInput.trim() : updateTagInput.trim();
		if (value) {
			addTag(value, form);
			if (form === 'upload') {
				uploadTagInput = '';
			} else {
				updateTagInput = '';
			}
		}
	}

	function addTag(tag: string, form: 'upload' | 'update') {
		const trimmed = tag.trim().toLowerCase();
		if (!trimmed) return;

		if (form === 'upload') {
			if (!uploadForm.tags.includes(trimmed)) {
				uploadForm.tags = [...uploadForm.tags, trimmed];
			}
		} else {
			if (!updateForm.tags.includes(trimmed)) {
				updateForm.tags = [...updateForm.tags, trimmed];
			}
		}
	}

	function removeTag(index: number, form: 'upload' | 'update') {
		if (form === 'upload') {
			uploadForm.tags = uploadForm.tags.filter((_, i) => i !== index);
		} else {
			updateForm.tags = updateForm.tags.filter((_, i) => i !== index);
		}
	}

	const columns = [
		{
			key: 'name',
			title: 'Name',
			cellComponent: EntityCell,
			cellProps: { entityType: 'object' }
		},
		{
			key: 'size',
			title: 'Size',
			cellComponent: GenericCell,
			cellProps: {
				getValue: (obj: FileObject) => formatFileSize(obj.size || 0)
			}
		},
		{
			key: 'tags',
			title: 'Tags',
			cellComponent: TagsCell,
			hideOnMobile: true,
			cellProps: {
				tags: [] // Will be passed per-row by DataTable
			}
		},
		{
			key: 'updated_at',
			title: 'Last Modified',
			hideOnMobile: true,
			cellComponent: GenericCell,
			cellProps: {
				getValue: (obj: FileObject) => formatDateTime(obj.updated_at)
			}
		},
		{
			key: 'actions',
			title: 'Actions',
			align: 'right' as const,
			width: 'min',
			cellComponent: ActionsCell,
			cellProps: {
				actions: [
					{
						type: 'custom' as const,
						label: 'Download',
						action: 'download',
						title: 'Download',
						ariaLabel: 'Download object'
					},
					{
						type: 'edit' as const,
						action: 'edit',
						title: 'Update',
						ariaLabel: 'Update object'
					},
					{
						type: 'delete' as const,
						action: 'delete',
						title: 'Delete',
						ariaLabel: 'Delete object'
					}
				]
			}
		}
	];

	const objectStorageDescription = `
This feature allows you to use GARM as a simple, private internal-use object storage system. The primary goal of this is to allow users to store provider binaries, agent binaries, runner tools and any other type of files needed for a functional GARM deployment.

Files are stored in the database as blobs. You do not need to configure additional storage.

It is not meant to be used to serve files outside of the needs of GARM and it does not implement S3, nor will it ever.
	`.trim();

	// Mobile card configuration
	const mobileCardConfig = {
		entityType: 'repository' as const, // Reuse existing type since 'object' isn't defined
		primaryText: {
			field: 'name',
			isClickable: true,
			href: '/objects/{id}'
		},
		secondaryText: {
			field: 'description',
			computedValue: (obj: FileObject) => {
				const desc = obj.description || 'No description';
				// Truncate long descriptions for mobile card (max 100 chars)
				return desc.length > 100 ? desc.substring(0, 100) + '...' : desc;
			}
		},
		customInfo: [
			{
				icon: '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z" />',
				text: (obj: FileObject) => formatFileSize(obj.size || 0)
			},
			{
				icon: '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />',
				text: (obj: FileObject) => formatDateTime(obj.updated_at)
			}
		],
		actions: [
			{
				type: 'edit' as const,
				handler: openUpdateModal
			},
			{
				type: 'delete' as const,
				handler: openDeleteModal
			}
		]
	};
</script>

<PageHeader
	title="Object Storage"
	description="Manage files stored in GARM's internal object storage"
>
	<svelte:fragment slot="actions">
		<Button
			variant="primary"
			on:click={openUploadModal}
			icon='<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6" />'
		>
			Upload New Object
		</Button>
	</svelte:fragment>
</PageHeader>

<!-- What's this? description -->
<div class="mb-6">
	<button
		on:click={() => showDescription = !showDescription}
		class="text-sm font-medium text-indigo-600 dark:text-indigo-400 hover:text-indigo-500 dark:hover:text-indigo-300 flex items-center cursor-pointer"
	>
		<svg class="w-4 h-4 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
			<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
		</svg>
		What's this?
		<svg class="w-4 h-4 ml-1 transform {showDescription ? 'rotate-180' : ''}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
			<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
		</svg>
	</button>

	{#if showDescription}
		<div class="mt-3 p-4 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg text-sm text-gray-700 dark:text-gray-300 whitespace-pre-wrap">
			{objectStorageDescription}
		</div>
	{/if}
</div>

<!-- Table with integrated backend search -->
<DataTable
	{columns}
	data={objects}
	{loading}
	{error}
	itemName="objects"
	emptyIconType="document"
	showSearch={true}
	searchType="backend"
	bind:searchTerm
	searchPlaceholder="Search by name or tags..."
	searchHelpText=""
	showPagination={true}
	paginationComponent={BackendPagination}
	currentPage={currentPage}
	totalPages={totalPages}
	totalItems={totalObjects}
	perPage={pageSize}
	{mobileCardConfig}
	on:search={handleSearch}
	on:pageChange={handlePageChange}
	on:pageSizeChange={handlePageSizeChange}
	on:prefetch={handlePrefetch}
	on:edit={(e) => openUpdateModal(e.detail.item)}
	on:delete={(e) => openDeleteModal(e.detail.item)}
	on:action={(e) => e.detail.type === 'download' && handleDownload(e.detail.item)}
/>

<!-- Upload Modal -->
{#if showUploadModal}
	<Modal on:close={() => !uploading && (showUploadModal = false)}>
		<div class="max-w-2xl w-full p-6">
			<h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">Upload New Object</h3>

			<form on:submit|preventDefault={handleUpload} class="space-y-4">
				<div>
					<label for="upload-file-name" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						File Name
					</label>
					<input
						id="upload-file-name"
						type="text"
						bind:value={uploadForm.name}
						disabled={uploading}
						class="block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white sm:text-sm"
						placeholder="my-file.bin"
					/>
				</div>

				<div>
					<label for="upload-file-input" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						File
					</label>
					<div class="flex items-center gap-3">
						<input
							id="upload-file-input"
							type="file"
							on:change={handleFileInput}
							disabled={uploading}
							class="hidden"
						/>
						<Button
							type="button"
							variant="secondary"
							disabled={uploading}
							on:click={() => document.getElementById('upload-file-input')?.click()}
						>
							Choose File
						</Button>
						<span class="text-sm text-gray-600 dark:text-gray-400">
							{uploadForm.file ? uploadForm.file.name : 'No file chosen'}
						</span>
					</div>
				</div>

				<div>
					<label for="upload-file-tags" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						Tags
					</label>
					<div class="block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm bg-white dark:bg-gray-700 min-h-[42px] flex flex-wrap gap-2 items-center">
						{#each uploadForm.tags as tag, index}
							<span class="inline-flex items-center gap-1 px-2 py-1 text-sm font-medium text-blue-700 dark:text-blue-300 bg-blue-100 dark:bg-blue-900 rounded-md">
								{tag}
								<button
									type="button"
									on:click={() => removeTag(index, 'upload')}
									disabled={uploading}
									class="inline-flex items-center justify-center w-4 h-4 text-blue-700 dark:text-blue-300 hover:text-blue-900 dark:hover:text-blue-100 rounded-full hover:bg-blue-200 dark:hover:bg-blue-800 disabled:opacity-50"
									aria-label="Remove tag {tag}"
								>
									<svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
									</svg>
								</button>
							</span>
						{/each}
						<input
							id="upload-file-tags"
							type="text"
							bind:value={uploadTagInput}
							on:keydown={(e) => handleTagKeydown(e, 'upload')}
							on:blur={() => handleTagBlur('upload')}
							disabled={uploading}
							class="flex-1 min-w-[120px] outline-none bg-transparent text-gray-900 dark:text-white text-sm"
							placeholder={uploadForm.tags.length === 0 ? "Type and press space to add tags..." : ""}
						/>
					</div>
					<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
						Press Space or Enter to add a tag. Press Backspace to remove the last tag.
					</p>
				</div>

				<div>
					<label for="upload-file-description" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						Description (optional)
					</label>
					<textarea
						id="upload-file-description"
						bind:value={uploadForm.description}
						disabled={uploading}
						maxlength="8192"
						rows="3"
						class="block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white sm:text-sm resize-y"
						placeholder="Add a description for this file..."
					></textarea>
					<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
						{uploadForm.description.length} / 8192 characters
					</p>
				</div>

				{#if uploading}
					<div>
						<div class="flex justify-between mb-2">
							<span class="text-sm font-medium text-gray-700 dark:text-gray-300">Uploading...</span>
							<span class="text-sm font-medium text-gray-700 dark:text-gray-300">{uploadProgress}%</span>
						</div>
						<div class="w-full bg-gray-200 rounded-full h-2.5 dark:bg-gray-700">
							<div class="bg-blue-600 h-2.5 rounded-full transition-all duration-300" style="width: {uploadProgress}%"></div>
						</div>
					</div>
				{/if}

				<div class="flex justify-end space-x-3 pt-4">
					<Button
						type="button"
						variant="secondary"
						disabled={uploading}
						on:click={() => showUploadModal = false}
					>
						Cancel
					</Button>
					<Button
						type="submit"
						variant="primary"
						disabled={uploading}
						loading={uploading}
					>
						{uploading ? 'Uploading...' : 'Upload'}
					</Button>
				</div>
			</form>
		</div>
	</Modal>
{/if}

<!-- Update Modal -->
{#if showUpdateModal && selectedObject}
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
									on:click={() => removeTag(index, 'update')}
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
							on:keydown={(e) => handleTagKeydown(e, 'update')}
							on:blur={() => handleTagBlur('update')}
							class="flex-1 min-w-[120px] outline-none bg-transparent text-gray-900 dark:text-white text-sm"
							placeholder={updateForm.tags.length === 0 ? "Type and press space to add tags..." : ""}
						/>
					</div>
					<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
						Press Space or Enter to add a tag. Press Backspace to remove the last tag.
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

				<div class="flex justify-end space-x-3 pt-4">
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
						Update
					</Button>
				</div>
			</form>
		</div>
	</Modal>
{/if}

<!-- Delete Modal -->
{#if showDeleteModal && selectedObject}
	<DeleteModal
		title="Delete Object"
		message="Are you sure you want to delete the object '{selectedObject.name}'? This action cannot be undone."
		on:confirm={handleDeleteObject}
		on:close={() => {
			showDeleteModal = false;
			selectedObject = null;
		}}
	/>
{/if}
