<script lang="ts">
	import { createEventDispatcher, onMount } from 'svelte';
	import type { CreateRepoParams } from '$lib/api/generated/api.js';
	import Modal from './Modal.svelte';
	import EntityForm from './forms/EntityForm.svelte';
	import { extractAPIError } from '$lib/utils/apiError';
	import { eagerCache, eagerCacheManager } from '$lib/stores/eager-cache.js';

	const dispatch = createEventDispatcher<{
		close: void;
		submit: CreateRepoParams & { install_webhook?: boolean; auto_generate_secret?: boolean };
	}>();

	let loading = false;
	let error = '';
	let selectedForgeType: 'github' | 'gitea' | '' = 'github';

	// Get credentials from eager cache
	$: credentials = $eagerCache.credentials;
	$: credentialsLoading = $eagerCache.loading.credentials;

	// Form data as individual fields
	let name = '';
	let owner = '';
	let credentialsName = '';
	let poolBalancerType = 'roundrobin';
	let agentMode = false;
	let installWebhook = true;
	let autoGenerateSecret = true;
	let webhookSecret = '';

	async function loadCredentialsIfNeeded() {
		if (!$eagerCache.loaded.credentials && !$eagerCache.loading.credentials) {
			try {
				await eagerCacheManager.getCredentials();
			} catch (err) {
				error = extractAPIError(err);
			}
		}
	}

	// Check if all mandatory fields are filled
	$: isFormValid = name?.trim() !== '' &&
					 owner?.trim() !== '' &&
					 credentialsName !== '' &&
					 (autoGenerateSecret || (webhookSecret?.trim() !== ''));

	async function handleSubmit() {
		if (!name?.trim()) {
			error = 'Repository name is required';
			return;
		}
		if (!owner?.trim()) {
			error = 'Repository owner is required';
			return;
		}
		if (!credentialsName) {
			error = 'Please select credentials';
			return;
		}

		try {
			loading = true;
			error = '';

			const submitData: CreateRepoParams & { install_webhook?: boolean; auto_generate_secret?: boolean } = {
				name,
				owner,
				credentials_name: credentialsName,
				webhook_secret: webhookSecret,
				pool_balancer_type: poolBalancerType,
				agent_mode: agentMode,
				install_webhook: installWebhook,
				auto_generate_secret: autoGenerateSecret
			};

			dispatch('submit', submitData);
		} catch (err) {
			error = extractAPIError(err);
			loading = false;
		}
	}

	onMount(() => {
		loadCredentialsIfNeeded();
	});
</script>

<Modal on:close={() => dispatch('close')}>
	<div class="max-w-2xl w-full p-6">
		<h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">Create Repository</h3>

		{#if error}
			<div class="mb-4 rounded-md bg-red-50 dark:bg-red-900 p-4">
				<p class="text-sm font-medium text-red-800 dark:text-red-200">{error}</p>
			</div>
		{/if}

		{#if loading}
			<div class="text-center py-4">
				<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600 mx-auto"></div>
				<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">Loading...</p>
			</div>
		{:else}
			<form on:submit|preventDefault={handleSubmit} class="space-y-4">
				<EntityForm
					entityType="repository"
					bind:name
					bind:owner
					bind:credentialsName
					bind:poolBalancerType
					bind:agentMode
					bind:installWebhook
					bind:webhookSecret
					bind:autoGenerateSecret
					bind:forgeType={selectedForgeType}
					{credentials}
					showCredentialsSelector={true}
					showForgeTypeSelector={true}
					idPrefix="repo-"
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
						disabled={loading || credentialsLoading || !isFormValid}
						class="px-4 py-2 text-sm font-medium text-white bg-blue-600 border border-transparent rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 dark:focus:ring-offset-gray-900 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
					>
						{loading ? 'Creating...' : 'Create Repository'}
					</button>
				</div>
			</form>
		{/if}
	</div>
</Modal>
