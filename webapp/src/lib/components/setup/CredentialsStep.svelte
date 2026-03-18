<script lang="ts">
	import { createEventDispatcher, onMount } from 'svelte';
	import { garmApi } from '$lib/api/client.js';
	import type { ForgeCredentials, CreateGithubCredentialsParams, CreateGiteaCredentialsParams } from '$lib/api/generated/api.js';
	import Badge from '../Badge.svelte';
	import CredentialsForm from '../forms/CredentialsForm.svelte';
	import { eagerCacheManager } from '$lib/stores/eager-cache.js';
	import { toastStore } from '$lib/stores/toast.js';
	import { extractAPIError } from '$lib/utils/apiError';
	import { getForgeIcon } from '$lib/utils/common.js';

	const AuthType = { PAT: 'pat', APP: 'app' } as const;

	export let endpointName: string;
	export let forgeType: 'github' | 'gitea' | '';
	export let credentialsName: string = '';

	const dispatch = createEventDispatcher<{
		complete: { credentialsName: string };
		back: void;
	}>();

	let mode: 'existing' | 'create' = 'existing';
	let allCredentials: ForgeCredentials[] = [];
	let loadingCredentials = true;
	let selectedCredential: ForgeCredentials | null = null;
	let creating = false;
	let error = '';

	// Filter credentials matching the selected endpoint
	$: matchingCredentials = allCredentials.filter(c => c.endpoint?.name === endpointName);

	// Create form state
	let selectedAuthType: typeof AuthType[keyof typeof AuthType] = AuthType.PAT;
	let formData = {
		name: '',
		description: '',
		endpoint: '',
		auth_type: AuthType.PAT as typeof AuthType[keyof typeof AuthType],
		oauth2_token: '',
		app_id: '',
		installation_id: '',
		private_key_bytes: ''
	};

	$: isFormValid = (() => {
		if (!formData.name) return false;
		if (selectedAuthType === AuthType.PAT) {
			return !!formData.oauth2_token;
		} else {
			return !!formData.app_id && !!formData.installation_id && !!formData.private_key_bytes;
		}
	})();

	onMount(async () => {
		try {
			allCredentials = await eagerCacheManager.getCredentials();
		} catch (err) {
			error = extractAPIError(err);
		} finally {
			loadingCredentials = false;
		}

		if (matchingCredentials.length === 0) {
			mode = 'create';
		} else {
			mode = 'existing';
			if (credentialsName) {
				selectedCredential = matchingCredentials.find(c => c.name === credentialsName) || null;
			}
		}
	});

	async function handleCreate() {
		creating = true;
		error = '';
		try {
			if (forgeType === 'github') {
				const githubParams: CreateGithubCredentialsParams = {
					name: formData.name.trim(),
					description: formData.description.trim(),
					endpoint: endpointName,
					auth_type: selectedAuthType
				};
				if (selectedAuthType === AuthType.PAT) {
					githubParams.pat = { oauth2_token: formData.oauth2_token.trim() };
					githubParams.app = {};
				} else {
					githubParams.app = {
						app_id: parseInt(formData.app_id.trim()),
						installation_id: parseInt(formData.installation_id.trim()),
						private_key_bytes: Array.from(atob(formData.private_key_bytes), (char: string) => char.charCodeAt(0))
					};
					githubParams.pat = {};
				}
				await garmApi.createGithubCredentials(githubParams);
			} else {
				const giteaParams: CreateGiteaCredentialsParams = {
					name: formData.name.trim(),
					description: formData.description.trim(),
					endpoint: endpointName,
					auth_type: AuthType.PAT,
					pat: { oauth2_token: formData.oauth2_token.trim() },
					app: {}
				};
				await garmApi.createGiteaCredentials(giteaParams);
			}

			toastStore.success('Credentials Created', `Credentials ${formData.name} have been created successfully.`);
			dispatch('complete', { credentialsName: formData.name.trim() });
		} catch (err) {
			error = extractAPIError(err);
		} finally {
			creating = false;
		}
	}

	function handleSelectExisting() {
		if (!selectedCredential) return;
		dispatch('complete', { credentialsName: selectedCredential.name! });
	}
</script>

<div class="space-y-6">
	<div>
		<h2 class="text-xl font-semibold text-gray-900 dark:text-white">Configure Credentials</h2>
		<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
			Set up authentication for your forge endpoint.
		</p>
	</div>

	<!-- Context banner -->
	<div class="flex items-center space-x-3 p-3 bg-gray-50 dark:bg-gray-700 rounded-lg">
		<div class="flex-shrink-0">{@html getForgeIcon(forgeType, 'w-5 h-5')}</div>
		<div class="text-sm">
			<span class="text-gray-500 dark:text-gray-400">Endpoint:</span>
			<span class="font-medium text-gray-900 dark:text-white ml-1">{endpointName}</span>
		</div>
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
		{#if loadingCredentials}
			<div class="text-center py-8">
				<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600 mx-auto"></div>
				<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">Loading credentials...</p>
			</div>
		{:else if matchingCredentials.length === 0}
			<div class="text-center py-8">
				<p class="text-sm text-gray-500 dark:text-gray-400">No credentials found for endpoint <strong>{endpointName}</strong>. Create new credentials to continue.</p>
			</div>
		{:else}
			<div class="space-y-3">
				{#each matchingCredentials as credential}
					<button
						type="button"
						on:click={() => selectedCredential = credential}
						class="w-full text-left p-4 rounded-lg border-2 transition-colors cursor-pointer
							{selectedCredential?.id === credential.id
								? 'border-blue-500 bg-blue-50 dark:bg-blue-900/30'
								: 'border-gray-200 dark:border-gray-600 hover:border-gray-300 dark:hover:border-gray-500'}"
					>
						<div class="flex items-center justify-between">
							<div>
								<p class="text-sm font-medium text-gray-900 dark:text-white">{credential.name}</p>
								{#if credential.description}
									<p class="text-xs text-gray-500 dark:text-gray-400 mt-0.5">{credential.description}</p>
								{/if}
							</div>
							<div class="flex items-center space-x-3">
								{#if (credential['auth-type'] || 'pat') === 'pat'}
									<Badge variant="success" text="PAT" />
								{:else}
									<Badge variant="info" text="App" />
								{/if}
								{#if selectedCredential?.id === credential.id}
									<svg class="w-5 h-5 text-blue-600" fill="currentColor" viewBox="0 0 20 20">
										<path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/>
									</svg>
								{/if}
							</div>
						</div>
					</button>
				{/each}
			</div>

			<div class="flex justify-between pt-4">
				<button type="button" on:click={() => dispatch('back')}
					class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-600 hover:bg-gray-200 dark:hover:bg-gray-500 rounded-md cursor-pointer">
					Back
				</button>
				<button
					type="button"
					on:click={handleSelectExisting}
					disabled={!selectedCredential}
					class="px-6 py-2 text-sm font-medium text-white rounded-md focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 dark:focus:ring-offset-gray-900 transition-colors
						{selectedCredential ? 'bg-blue-600 hover:bg-blue-700 cursor-pointer' : 'bg-gray-400 cursor-not-allowed'}"
				>
					Continue
				</button>
			</div>
		{/if}
	{:else}
		<!-- Create new credentials form -->
		<form on:submit|preventDefault={handleCreate} class="space-y-4">
			<CredentialsForm
				bind:formData
				bind:selectedAuthType
				{forgeType}
				fixedEndpointName={endpointName}
				idPrefix="cred-"
			/>

			<div class="flex justify-between pt-4">
				<button type="button" on:click={() => dispatch('back')}
					class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-600 hover:bg-gray-200 dark:hover:bg-gray-500 rounded-md cursor-pointer">
					Back
				</button>
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
