<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import ForgeTypeSelector from '../ForgeTypeSelector.svelte';
	import Tooltip from '../Tooltip.svelte';
	import CaCertUpload from './CaCertUpload.svelte';

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

	function handleCertSelect(event: CustomEvent<{ base64: string; fileName: string }>) {
		formData.ca_cert_bundle = event.detail.base64;
		selectedCertFileName = event.detail.fileName;
	}

	function handleCertClear() {
		formData.ca_cert_bundle = '';
		selectedCertFileName = '';
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

<CaCertUpload
	fileName={selectedCertFileName}
	idPrefix="{idPrefix}"
	on:select={handleCertSelect}
	on:clear={handleCertClear}
	on:remove={handleCertClear}
/>
