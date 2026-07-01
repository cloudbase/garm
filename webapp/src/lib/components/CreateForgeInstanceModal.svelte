<script lang="ts">
	import { createEventDispatcher, onMount } from 'svelte';
	import type { CreateForgeInstanceParams } from '$lib/api/generated/api.js';
	import Modal from './Modal.svelte';
	import EntityForm from './forms/EntityForm.svelte';
	import { extractAPIError } from '$lib/utils/apiError';
	import { eagerCache, eagerCacheManager } from '$lib/stores/eager-cache.js';

	const dispatch = createEventDispatcher<{
		close: void;
		submit: CreateForgeInstanceParams;
	}>();

	let loading = false;
	let error = '';

	// Get credentials from eager cache
	$: credentials = $eagerCache.credentials;
	$: credentialsLoading = $eagerCache.loading.credentials;

	// Get endpoints from eager cache
	$: endpoints = $eagerCache.endpoints;

	// Form data
	let endpointName = '';
	let credentialsName = '';
	let poolBalancerType = 'roundrobin';
	let agentMode = false;
	let autoGenerateSecret = true;
	let webhookSecret = '';

	// Dummy name field (EntityForm requires it but we don't use it)
	let name = '';

	async function loadDepsIfNeeded() {
		if (!$eagerCache.loaded.credentials && !$eagerCache.loading.credentials) {
			try {
				await eagerCacheManager.getCredentials();
			} catch (err) {
				error = extractAPIError(err);
			}
		}
		if (!$eagerCache.loaded.endpoints && !$eagerCache.loading.endpoints) {
			try {
				await eagerCacheManager.getEndpoints();
			} catch (err) {
				error = extractAPIError(err);
			}
		}
	}

	// Only show Gitea endpoints
	$: giteaEndpoints = endpoints.filter(ep => ep.endpoint_type === 'gitea');

	// Filter credentials by selected endpoint
	$: filteredCredentials = credentials.filter(cred =>
		cred.forge_type === 'gitea' &&
		(!endpointName || cred.endpoint?.name === endpointName)
	);

	// Clear credentials when endpoint changes (if current selection doesn't match)
	$: {
		if (endpointName && credentialsName) {
			const stillValid = filteredCredentials.find(c => c.name === credentialsName);
			if (!stillValid) {
				credentialsName = '';
			}
		}
	}

	$: isFormValid = credentialsName !== '' &&
					 endpointName !== '' &&
					 (webhookSecret && webhookSecret.trim() !== '');

	async function handleSubmit() {
		if (!credentialsName) {
			error = 'Please select credentials';
			return;
		}
		if (!endpointName) {
			error = 'Endpoint name is required';
			return;
		}

		try {
			loading = true;
			error = '';

			const submitData: CreateForgeInstanceParams = {
				endpoint_name: endpointName,
				credentials_name: credentialsName,
				webhook_secret: webhookSecret,
				forge_type: 'gitea',
				pool_balancer_type: poolBalancerType,
				agent_mode: agentMode
			};

			dispatch('submit', submitData);
		} catch (err) {
			error = extractAPIError(err);
			loading = false;
		}
	}

	onMount(() => {
		loadDepsIfNeeded();
	});
</script>

<Modal on:close={() => dispatch('close')}>
	<div class="max-w-2xl w-full p-6">
		<h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">Create Forge Instance</h3>
		<p class="text-sm text-gray-600 dark:text-gray-400 mb-4">
			Create an instance-level runner pool target for a Gitea server.
		</p>

		{#if error}
			<div class="mb-4 rounded-md bg-red-50 dark:bg-red-900 p-4">
				<p class="text-sm font-medium text-red-800 dark:text-red-200">{error}</p>
			</div>
		{/if}

		{#if loading}
			<div class="text-center py-4">
				<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600 mx-auto"></div>
				<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">Creating...</p>
			</div>
		{:else}
			<form on:submit|preventDefault={handleSubmit} class="space-y-4">
				<!-- Endpoint selector -->
				<div>
					<label for="fi-endpoint" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						Endpoint <span class="text-red-500">*</span>
					</label>
					{#if giteaEndpoints.length === 0}
						<p class="text-sm text-yellow-600 dark:text-yellow-400">No Gitea endpoints configured. Please create one first.</p>
					{:else}
						<select
							id="fi-endpoint"
							bind:value={endpointName}
							required
							class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
						>
							<option value="">Select an endpoint</option>
							{#each giteaEndpoints as ep}
								<option value={ep.name}>{ep.name}</option>
							{/each}
						</select>
					{/if}
				</div>

				<EntityForm
					entityType="forge_instance"
					bind:name
					bind:credentialsName
					bind:poolBalancerType
					bind:agentMode
					bind:webhookSecret
					bind:autoGenerateSecret
					forgeType="gitea"
					credentials={filteredCredentials}
					showCredentialsSelector={true}
					idPrefix="fi-"
				/>

				<!-- Actions -->
				<div class="flex justify-end space-x-3 pt-4">
					<button
						type="button"
						on:click={() => dispatch('close')}
						class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 rounded-md hover:bg-gray-50 dark:hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 dark:focus:ring-offset-gray-900 cursor-pointer"
					>
						Cancel
					</button>
					<button
						type="submit"
						disabled={loading || credentialsLoading || !isFormValid || filteredCredentials.length === 0}
						class="px-4 py-2 text-sm font-medium text-white bg-blue-600 border border-transparent rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 dark:focus:ring-offset-gray-900 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
					>
						{loading ? 'Creating...' : 'Create Forge Instance'}
					</button>
				</div>
			</form>
		{/if}
	</div>
</Modal>
