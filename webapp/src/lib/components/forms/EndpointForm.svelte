<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import ForgeTypeSelector from '../ForgeTypeSelector.svelte';
	import Tooltip from '../Tooltip.svelte';
	import { handleFileInputAsBase64 } from '$lib/utils/file';

	const dispatch = createEventDispatcher<{
		forgeTypeSelect: 'github' | 'gitea';
	}>();

	export let formData = {
		name: '',
		description: '',
		base_url: '',
		api_base_url: '',
		upload_base_url: '',
		ca_cert_bundle: '',
		tools_metadata_url: '',
		use_internal_tools_metadata: false
	};
	export let selectedForgeType: 'github' | 'gitea' | '' = 'github';
	export let selectedCertFileName = '';
	export let showForgeTypeSelector = true;
	export let idPrefix = '';

	function handleForgeTypeSelect(event: CustomEvent<'github' | 'gitea'>) {
		selectedForgeType = event.detail;
		formData.name = '';
		formData.description = '';
		formData.base_url = '';
		formData.api_base_url = '';
		formData.upload_base_url = '';
		formData.ca_cert_bundle = '';
		formData.tools_metadata_url = '';
		formData.use_internal_tools_metadata = false;
		selectedCertFileName = '';
		dispatch('forgeTypeSelect', event.detail);
	}

	function handleFileUpload(event: Event) {
		handleFileInputAsBase64(
			event,
			(base64, fileName) => {
				formData.ca_cert_bundle = base64;
				selectedCertFileName = fileName;
			},
			() => {
				formData.ca_cert_bundle = '';
				selectedCertFileName = '';
			}
		);
	}

	function clearCertificate() {
		formData.ca_cert_bundle = '';
		selectedCertFileName = '';
		const fileInput = document.getElementById(`${idPrefix}ca-cert-file`) as HTMLInputElement;
		if (fileInput) fileInput.value = '';
	}
</script>

{#if showForgeTypeSelector}
	<ForgeTypeSelector
		bind:selectedForgeType
		on:select={handleForgeTypeSelect}
	/>
{/if}

<div>
	<label for="{idPrefix}name" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
		Endpoint Name <span class="text-red-500">*</span>
	</label>
	<input
		type="text"
		id="{idPrefix}name"
		bind:value={formData.name}
		required
		autocomplete="off"
		class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
		placeholder={selectedForgeType === 'github' ? 'e.g., github-enterprise or github-com' : 'e.g., gitea-main or my-gitea'}
	/>
</div>

<div>
	<label for="{idPrefix}description" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
		Description
	</label>
	<textarea
		id="{idPrefix}description"
		bind:value={formData.description}
		rows="2"
		autocomplete="off"
		class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
		placeholder="Brief description of this endpoint"
	></textarea>
</div>

<div>
	<label for="{idPrefix}base-url" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
		Base URL <span class="text-red-500">*</span>
	</label>
	<input
		type="url"
		id="{idPrefix}base-url"
		bind:value={formData.base_url}
		required
		autocomplete="off"
		class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
		placeholder={selectedForgeType === 'github' ? 'https://github.com or https://github.example.com' : 'https://gitea.example.com'}
	/>
</div>

{#if selectedForgeType === 'github'}
	<div>
		<label for="{idPrefix}api-base-url" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
			API Base URL <span class="text-red-500">*</span>
		</label>
		<input
			type="url"
			id="{idPrefix}api-base-url"
			bind:value={formData.api_base_url}
			required
			autocomplete="off"
			class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
			placeholder="https://api.github.com or https://github.example.com/api/v3"
		/>
	</div>

	<div>
		<label for="{idPrefix}upload-base-url" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
			Upload Base URL
		</label>
		<input
			type="url"
			id="{idPrefix}upload-base-url"
			bind:value={formData.upload_base_url}
			autocomplete="off"
			class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
			placeholder="https://uploads.github.com"
		/>
	</div>
{:else}
	<div>
		<label for="{idPrefix}api-base-url" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
			API Base URL <span class="text-xs text-gray-500">(optional)</span>
		</label>
		<input
			type="url"
			id="{idPrefix}api-base-url"
			bind:value={formData.api_base_url}
			autocomplete="off"
			class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
			placeholder="https://gitea.example.com/api/v1 (leave empty to use Base URL)"
		/>
		<p class="text-xs text-gray-500 dark:text-gray-400 mt-1">If empty, Base URL will be used as API Base URL</p>
	</div>

	<!-- Gitea-specific tools metadata fields -->
	<div>
		<div class="flex items-center mb-1">
			<label for="{idPrefix}tools-metadata-url" class="block text-sm font-medium {formData.use_internal_tools_metadata ? 'text-gray-400 dark:text-gray-500' : 'text-gray-700 dark:text-gray-300'}">
				Tools Metadata URL <span class="text-xs text-gray-500">(optional)</span>
			</label>
			<div class="ml-2">
				<Tooltip
					title="Tools Metadata URL"
					content="URL where GARM checks for act_runner binary downloads and release information. Defaults to https://gitea.com/api/v1/repos/gitea/act_runner/releases if not specified. Use a custom URL to point to your own tools repository or mirror."
					position="top"
					width="w-80"
				/>
			</div>
		</div>
		<input
			type="url"
			id="{idPrefix}tools-metadata-url"
			bind:value={formData.tools_metadata_url}
			disabled={formData.use_internal_tools_metadata}
			autocomplete="off"
			class="w-full px-3 py-2 border rounded-md focus:outline-none transition-colors {formData.use_internal_tools_metadata ? 'bg-gray-100 dark:bg-gray-800 border-gray-300 dark:border-gray-700 text-gray-400 dark:text-gray-500 cursor-not-allowed' : 'border-gray-300 dark:border-gray-600 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white'}"
			placeholder="https://gitea.com/api/v1/repos/gitea/act_runner/releases"
		/>
		<p class="text-xs {formData.use_internal_tools_metadata ? 'text-gray-400 dark:text-gray-500' : 'text-gray-500 dark:text-gray-400'} mt-1">{formData.use_internal_tools_metadata ? 'Disabled when using internal tools metadata' : 'Leave empty to use default Gitea releases URL'}</p>
	</div>

	<div class="flex items-center">
		<input
			id="{idPrefix}use-internal-tools"
			type="checkbox"
			bind:checked={formData.use_internal_tools_metadata}
			class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 dark:border-gray-600 rounded"
		/>
		<label for="{idPrefix}use-internal-tools" class="ml-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
			Use Internal Tools Metadata
		</label>
		<div class="ml-2">
			<Tooltip
				title="Internal Tools Metadata"
				content="When enabled, GARM uses built-in URLs for nightly act_runner binaries instead of calling the external tools metadata URL. This is useful in air-gapped environments where runner images already include the binaries and don't need to download them."
				position="top"
				width="w-80"
			/>
		</div>
	</div>
{/if}

<!-- CA Certificate Upload -->
<div class="space-y-3 border-t border-gray-200 dark:border-gray-700 pt-4">
	<label for="{idPrefix}ca-cert-file" class="block text-sm font-medium text-gray-700 dark:text-gray-300">CA Certificate Bundle (Optional)</label>
	<div class="border-2 border-dashed rounded-lg p-4 text-center transition-colors {selectedCertFileName ? 'border-green-500 dark:border-green-400 bg-green-50 dark:bg-green-900/20' : 'border-gray-300 dark:border-gray-600 hover:border-blue-400 dark:hover:border-blue-400'}">
		<input
			type="file"
			id="{idPrefix}ca-cert-file"
			accept=".pem,.crt,.cer,.cert"
			on:change={handleFileUpload}
			class="hidden"
		/>
		{#if selectedCertFileName}
			<div class="space-y-2">
				<svg class="mx-auto h-8 w-8 text-green-600 dark:text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path>
				</svg>
				<p class="text-sm font-medium text-green-700 dark:text-green-300">{selectedCertFileName}</p>
				<div class="flex justify-center space-x-2">
					<button type="button" on:click={() => document.getElementById(`${idPrefix}ca-cert-file`)?.click()} class="text-xs text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 hover:underline cursor-pointer">
						Replace
					</button>
					<span class="text-xs text-gray-400">&bull;</span>
					<button type="button" on:click={clearCertificate} class="text-xs text-red-600 dark:text-red-400 hover:text-red-800 dark:hover:text-red-300 hover:underline cursor-pointer">
						Remove
					</button>
				</div>
			</div>
		{:else}
			<div class="space-y-2">
				<svg class="mx-auto h-8 w-8 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
				</svg>
				<p class="text-sm text-gray-600 dark:text-gray-400">
					<button type="button" on:click={() => document.getElementById(`${idPrefix}ca-cert-file`)?.click()} class="text-gray-900 dark:text-white hover:text-gray-700 dark:hover:text-gray-300 hover:underline cursor-pointer">
						Choose a file
					</button>
					or drag and drop
				</p>
				<p class="text-xs text-gray-500 dark:text-gray-400">PEM, CRT, CER, CERT files only</p>
			</div>
		{/if}
	</div>
</div>
