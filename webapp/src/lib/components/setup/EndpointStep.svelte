<script lang="ts">
	import { createEventDispatcher, onMount } from 'svelte';
	import { garmApi } from '$lib/api/client.js';
	import type { ForgeEndpoint } from '$lib/api/generated/api.js';
	import EndpointForm from '../forms/EndpointForm.svelte';
	import { eagerCacheManager } from '$lib/stores/eager-cache.js';
	import { toastStore } from '$lib/stores/toast.js';
	import { extractAPIError } from '$lib/utils/apiError';
	import { getForgeIcon } from '$lib/utils/common.js';

	export let endpointName: string = '';

	const dispatch = createEventDispatcher<{
		complete: { endpointName: string; forgeType: 'github' | 'gitea' };
	}>();

	let mode: 'existing' | 'create' = 'existing';
	let endpoints: ForgeEndpoint[] = [];
	let loadingEndpoints = true;
	let selectedEndpoint: ForgeEndpoint | null = null;
	let creating = false;
	let error = '';

	// Create form state
	let selectedForgeType: 'github' | 'gitea' | '' = 'github';
	let formData = {
		name: '',
		description: '',
		base_url: '',
		api_base_url: '',
		upload_base_url: '',
		ca_cert_bundle: '',
		tools_metadata_url: '',
		use_internal_tools_metadata: false
	};
	let selectedCertFileName = '';

	$: isFormValid = (() => {
		if (!formData.name || !formData.base_url) return false;
		if (selectedForgeType === 'github' && !formData.api_base_url) return false;
		return true;
	})();

	onMount(async () => {
		try {
			endpoints = await eagerCacheManager.getEndpoints();
		} catch (err) {
			error = extractAPIError(err);
		} finally {
			loadingEndpoints = false;
		}

		if (endpoints.length === 0) {
			mode = 'create';
		} else {
			mode = 'existing';
			if (endpointName) {
				selectedEndpoint = endpoints.find(e => e.name === endpointName) || null;
			}
			if (!selectedEndpoint) {
				const ghEndpoint = endpoints.find(e =>
					e.base_url?.includes('github.com') && e.endpoint_type === 'github'
				);
				if (ghEndpoint) {
					selectedEndpoint = ghEndpoint;
				}
			}
		}
	});

	async function handleCreate() {
		creating = true;
		error = '';
		try {
			const endpointParams: any = {
				name: formData.name,
				description: formData.description,
				base_url: formData.base_url,
				api_base_url: formData.api_base_url,
				upload_base_url: formData.upload_base_url
			};

			if (selectedForgeType === 'gitea') {
				if (formData.tools_metadata_url.trim() !== '') {
					endpointParams.tools_metadata_url = formData.tools_metadata_url.trim();
				}
				endpointParams.use_internal_tools_metadata = formData.use_internal_tools_metadata;
			}

			if (formData.ca_cert_bundle && formData.ca_cert_bundle.trim() !== '') {
				try {
					const bytes = atob(formData.ca_cert_bundle);
					endpointParams.ca_cert_bundle = Array.from(bytes, (char: string) => char.charCodeAt(0));
				} catch (e) {
					// Skip invalid base64
				}
			}

			if (selectedForgeType === 'github') {
				await garmApi.createGithubEndpoint(endpointParams);
			} else {
				await garmApi.createGiteaEndpoint(endpointParams);
			}

			toastStore.success('Endpoint Created', `Endpoint ${formData.name} has been created successfully.`);
			dispatch('complete', { endpointName: formData.name, forgeType: selectedForgeType as 'github' | 'gitea' });
		} catch (err) {
			error = extractAPIError(err);
		} finally {
			creating = false;
		}
	}

	function handleSelectExisting() {
		if (!selectedEndpoint) return;
		dispatch('complete', {
			endpointName: selectedEndpoint.name!,
			forgeType: selectedEndpoint.endpoint_type as 'github' | 'gitea'
		});
	}
</script>

<div class="space-y-6">
	<div>
		<h2 class="text-xl font-semibold text-gray-900 dark:text-white">Select Forge Endpoint</h2>
		<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
			Choose an existing endpoint or create a new one to connect to GitHub or Gitea.
		</p>
	</div>

	{#if error}
		<div class="rounded-md bg-red-50 dark:bg-red-900 p-4">
			<p class="text-sm font-medium text-red-800 dark:text-red-200">{error}</p>
		</div>
	{/if}

	<!-- Mode toggle -->
	<div class="flex space-x-1 bg-gray-100 dark:bg-gray-700 rounded-lg p-1">
		<button
			type="button"
			on:click={() => mode = 'existing'}
			class="flex-1 py-2 px-4 text-sm font-medium rounded-md transition-colors cursor-pointer
				{mode === 'existing'
					? 'bg-white dark:bg-gray-600 text-gray-900 dark:text-white shadow-sm'
					: 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'}"
		>
			Use Existing
		</button>
		<button
			type="button"
			on:click={() => mode = 'create'}
			class="flex-1 py-2 px-4 text-sm font-medium rounded-md transition-colors cursor-pointer
				{mode === 'create'
					? 'bg-white dark:bg-gray-600 text-gray-900 dark:text-white shadow-sm'
					: 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'}"
		>
			Create New
		</button>
	</div>

	{#if mode === 'existing'}
		<!-- Existing endpoints list -->
		{#if loadingEndpoints}
			<div class="text-center py-8">
				<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600 mx-auto"></div>
				<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">Loading endpoints...</p>
			</div>
		{:else if endpoints.length === 0}
			<div class="text-center py-8">
				<p class="text-sm text-gray-500 dark:text-gray-400">No endpoints found. Create a new one to get started.</p>
			</div>
		{:else}
			<div class="space-y-3">
				{#each endpoints as endpoint}
					<button
						type="button"
						on:click={() => selectedEndpoint = endpoint}
						class="w-full text-left p-4 rounded-lg border-2 transition-colors cursor-pointer
							{selectedEndpoint?.name === endpoint.name
								? 'border-blue-500 bg-blue-50 dark:bg-blue-900/30'
								: 'border-gray-200 dark:border-gray-600 hover:border-gray-300 dark:hover:border-gray-500'}"
					>
						<div class="flex items-center justify-between">
							<div class="flex items-center space-x-3">
								<div class="flex-shrink-0">
									{@html getForgeIcon(endpoint.endpoint_type || '', 'w-8 h-8')}
								</div>
								<div>
									<p class="text-sm font-medium text-gray-900 dark:text-white">{endpoint.name}</p>
									{#if endpoint.description}
										<p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">{endpoint.description}</p>
									{/if}
									<p class="text-xs text-gray-400 dark:text-gray-500 mt-0.5">{endpoint.api_base_url || endpoint.base_url}</p>
								</div>
							</div>
							{#if selectedEndpoint?.name === endpoint.name}
								<svg class="w-5 h-5 text-blue-600" fill="currentColor" viewBox="0 0 20 20">
									<path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/>
								</svg>
							{/if}
						</div>
					</button>
				{/each}
			</div>

			<div class="flex justify-end pt-4">
				<button
					type="button"
					on:click={handleSelectExisting}
					disabled={!selectedEndpoint}
					class="px-6 py-2 text-sm font-medium text-white rounded-md focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 dark:focus:ring-offset-gray-900 transition-colors
						{selectedEndpoint ? 'bg-blue-600 hover:bg-blue-700 cursor-pointer' : 'bg-gray-400 cursor-not-allowed'}"
				>
					Continue
				</button>
			</div>
		{/if}
	{:else}
		<!-- Create new endpoint form -->
		<form on:submit|preventDefault={handleCreate} class="space-y-4">
			<EndpointForm
				bind:formData
				bind:selectedForgeType
				bind:selectedCertFileName
				idPrefix="endpoint-"
			/>

			<div class="flex justify-end pt-4">
				<button
					type="submit"
					disabled={!isFormValid || creating}
					class="px-6 py-2 text-sm font-medium text-white rounded-md focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 dark:focus:ring-offset-gray-900 transition-colors
						{isFormValid && !creating ? 'bg-blue-600 hover:bg-blue-700 cursor-pointer' : 'bg-gray-400 cursor-not-allowed'}"
				>
					{#if creating}
						<div class="flex items-center">
							<div class="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
							Creating...
						</div>
					{:else}
						Create & Continue
					{/if}
				</button>
			</div>
		</form>
	{/if}
</div>
