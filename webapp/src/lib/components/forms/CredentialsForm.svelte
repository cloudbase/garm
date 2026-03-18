<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import type { ForgeEndpoint } from '$lib/api/generated/api.js';
	import ForgeTypeSelector from '../ForgeTypeSelector.svelte';
	import { handleFileInputAsBase64 } from '$lib/utils/file';

	const AuthType = { PAT: 'pat', APP: 'app' } as const;

	const dispatch = createEventDispatcher<{
		forgeTypeSelect: 'github' | 'gitea';
	}>();

	export let formData = {
		name: '',
		description: '',
		endpoint: '',
		auth_type: AuthType.PAT as typeof AuthType[keyof typeof AuthType],
		oauth2_token: '',
		app_id: '',
		installation_id: '',
		private_key_bytes: ''
	};
	export let selectedAuthType: typeof AuthType[keyof typeof AuthType] = AuthType.PAT;
	export let forgeType: 'github' | 'gitea' | '' = '';
	export let endpoints: ForgeEndpoint[] = [];
	export let showEndpointSelector = false;
	export let showForgeTypeSelector = false;
	export let fixedEndpointName = '';
	export let idPrefix = '';

	// Filter endpoints based on forge type
	$: filteredEndpoints = forgeType ? endpoints.filter(e => e.endpoint_type === forgeType) : endpoints;

	function handleForgeTypeSelect(event: CustomEvent<'github' | 'gitea'>) {
		forgeType = event.detail;
		formData.name = '';
		formData.description = '';
		formData.endpoint = '';
		formData.auth_type = AuthType.PAT;
		formData.oauth2_token = '';
		formData.app_id = '';
		formData.installation_id = '';
		formData.private_key_bytes = '';
		selectedAuthType = AuthType.PAT;
		dispatch('forgeTypeSelect', event.detail);
	}

	function handleAuthTypeChange(authType: typeof AuthType[keyof typeof AuthType]) {
		selectedAuthType = authType;
		formData.auth_type = authType;
	}

	function handlePrivateKeyUpload(event: Event) {
		handleFileInputAsBase64(
			event,
			(base64) => { formData.private_key_bytes = base64; },
			() => { formData.private_key_bytes = ''; }
		);
	}
</script>

{#if showForgeTypeSelector}
	<ForgeTypeSelector
		bind:selectedForgeType={forgeType}
		on:select={handleForgeTypeSelect}
	/>
{/if}

<div>
	<label for="{idPrefix}name" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
		Credentials Name <span class="text-red-500">*</span>
	</label>
	<input
		type="text"
		id="{idPrefix}name"
		bind:value={formData.name}
		required
		autocomplete="off"
		class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
		placeholder="e.g., my-{forgeType || 'github'}-credentials"
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
		placeholder="Brief description of these credentials"
	></textarea>
</div>

<!-- Endpoint: selector or readonly -->
{#if showEndpointSelector}
	<div>
		<label for="{idPrefix}endpoint" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
			Endpoint <span class="text-red-500">*</span>
		</label>
		<select
			id="{idPrefix}endpoint"
			bind:value={formData.endpoint}
			required
			autocomplete="off"
			class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
		>
			<option value="">Select an endpoint</option>
			{#each filteredEndpoints as endpoint}
				<option value={endpoint.name}>
					{endpoint.name} ({endpoint.endpoint_type})
				</option>
			{/each}
		</select>
		{#if forgeType}
			<p class="text-xs text-gray-500 dark:text-gray-400 mt-1">
				Showing only {forgeType} endpoints
			</p>
		{/if}
	</div>
{:else if fixedEndpointName}
	<div>
		<label for="{idPrefix}endpoint" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
			Endpoint
		</label>
		<input
			type="text"
			id="{idPrefix}endpoint"
			value={fixedEndpointName}
			disabled
			class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-gray-100 dark:bg-gray-600 text-gray-500 dark:text-gray-400 cursor-not-allowed"
		/>
	</div>
{/if}

<!-- Authentication Type -->
<div role="group" aria-labelledby="{idPrefix}auth-type-heading">
	<div id="{idPrefix}auth-type-heading" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
		Authentication Type <span class="text-red-500">*</span>
	</div>
	<div class="flex space-x-4">
		<button
			type="button"
			on:click={() => handleAuthTypeChange(AuthType.PAT)}
			class="flex-1 py-2 px-4 text-sm font-medium rounded-md border focus:outline-none focus:ring-2 focus:ring-blue-500 cursor-pointer
				{selectedAuthType === AuthType.PAT
					? 'bg-blue-600 text-white border-blue-600'
					: 'bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-300 border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-600'}"
		>
			PAT
		</button>
		<button
			type="button"
			on:click={() => { if (forgeType !== 'gitea') handleAuthTypeChange(AuthType.APP); }}
			disabled={forgeType === 'gitea'}
			class="flex-1 py-2 px-4 text-sm font-medium rounded-md border focus:outline-none focus:ring-2 focus:ring-blue-500
				{selectedAuthType === AuthType.APP
					? 'bg-blue-600 text-white border-blue-600'
					: 'bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-300 border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-600'}
				{forgeType === 'gitea' ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}"
		>
			App
		</button>
	</div>
	{#if forgeType === 'gitea'}
		<p class="text-xs text-gray-500 dark:text-gray-400 mt-1">Gitea only supports PAT authentication</p>
	{/if}
</div>

<!-- PAT Fields -->
{#if selectedAuthType === AuthType.PAT}
	<div>
		<label for="{idPrefix}token" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
			Personal Access Token <span class="text-red-500">*</span>
		</label>
		<input
			type="password"
			id="{idPrefix}token"
			bind:value={formData.oauth2_token}
			required
			autocomplete="off"
			class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
			placeholder={forgeType === 'github' || forgeType === '' ? 'ghp_xxxxxxxxxxxxxxxxxxxx' : 'your-access-token'}
		/>
	</div>
{/if}

<!-- App Fields -->
{#if selectedAuthType === AuthType.APP}
	<div>
		<label for="{idPrefix}app-id" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
			App ID <span class="text-red-500">*</span>
		</label>
		<input
			type="text"
			id="{idPrefix}app-id"
			bind:value={formData.app_id}
			required
			autocomplete="off"
			class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
			placeholder="123456"
		/>
	</div>

	<div>
		<label for="{idPrefix}installation-id" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
			App Installation ID <span class="text-red-500">*</span>
		</label>
		<input
			type="text"
			id="{idPrefix}installation-id"
			bind:value={formData.installation_id}
			required
			autocomplete="off"
			class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
			placeholder="12345678"
		/>
	</div>

	<div>
		<label for="{idPrefix}private-key" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
			Private Key <span class="text-red-500">*</span>
		</label>
		<div class="border-2 border-dashed border-gray-300 dark:border-gray-600 rounded-lg p-4 text-center hover:border-blue-400 dark:hover:border-blue-400 transition-colors">
			<input
				type="file"
				id="{idPrefix}private-key"
				accept=".pem,.key"
				on:change={handlePrivateKeyUpload}
				class="hidden"
			/>
			<div class="space-y-2">
				<svg class="mx-auto h-8 w-8 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
				</svg>
				<p class="text-sm text-gray-600 dark:text-gray-400">
					<button type="button" on:click={() => document.getElementById(`${idPrefix}private-key`)?.click()} class="text-gray-900 dark:text-white hover:underline cursor-pointer">Choose a file</button>
					or drag and drop
				</p>
				<p class="text-xs text-gray-500 dark:text-gray-400">PEM, KEY files only</p>
			</div>
		</div>
	</div>
{/if}
