<script lang="ts">
	import { createEventDispatcher, onMount } from 'svelte';
	import { garmApi } from '$lib/api/client.js';
	import type { CreateEnterpriseParams, ForgeCredentials } from '$lib/api/generated/api.js';
	import Modal from './Modal.svelte';
	import { eagerCache, eagerCacheManager } from '$lib/stores/eager-cache.js';

	const dispatch = createEventDispatcher<{
		close: void;
		submit: CreateEnterpriseParams;
	}>();

	let loading = false;
	let error = '';
	
	// Get credentials from eager cache
	$: credentials = $eagerCache.credentials;
	$: credentialsLoading = $eagerCache.loading.credentials;

	// Form data
	let formData: CreateEnterpriseParams = {
		name: '',
		credentials_name: '',
		webhook_secret: '',
		pool_balancer_type: 'roundrobin'
	};

	// Enterprises can't auto-generate webhook secrets since they can't install webhooks programmatically

	// Only show GitHub credentials (enterprises are GitHub-only)
	$: filteredCredentials = credentials.filter(cred => {
		return cred.forge_type === 'github';
	});

	async function loadCredentialsIfNeeded() {
		if (!$eagerCache.loaded.credentials && !$eagerCache.loading.credentials) {
			try {
				await eagerCacheManager.getCredentials();
			} catch (err) {
				error = err instanceof Error ? err.message : 'Failed to load credentials';
			}
		}
	}

	// Check if all mandatory fields are filled
	$: isFormValid = formData.name && formData.name.trim() !== '' && 
					 formData.credentials_name !== '' &&
					 (formData.webhook_secret && formData.webhook_secret.trim() !== '');

	async function handleSubmit() {
		if (!formData.name || !formData.name.trim()) {
			error = 'Enterprise name is required';
			return;
		}

		if (!formData.credentials_name) {
			error = 'Please select credentials';
			return;
		}

		try {
			loading = true;
			error = '';

			const submitData: CreateEnterpriseParams = {
				...formData
			};

			dispatch('submit', submitData);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create enterprise';
			loading = false;
		}
	}

	onMount(() => {
		loadCredentialsIfNeeded();
	});
</script>

<Modal on:close={() => dispatch('close')}>
	<div class="max-w-2xl w-full p-6">
		<h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">Create Enterprise</h3>
		<p class="text-sm text-gray-600 dark:text-gray-400 mb-4">
			Enterprises are only available for GitHub endpoints.
		</p>

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
				<!-- Enterprise Name -->
				<div>
					<label for="name" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						Enterprise Name
					</label>
					<input
						id="name"
						type="text"
						bind:value={formData.name}
						required
						class="block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white sm:text-sm"
						placeholder="Enter enterprise name"
					/>
				</div>

				<!-- Credentials -->
				<div>
					<label for="credentials" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						GitHub Credentials
					</label>
					<select
						id="credentials"
						bind:value={formData.credentials_name}
						required
						class="block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white sm:text-sm"
					>
						<option value="">Select GitHub credentials...</option>
						{#each filteredCredentials as credential}
							<option value={credential.name}>
								{credential.name} ({credential.endpoint?.name || 'Unknown endpoint'})
							</option>
						{/each}
					</select>
					{#if credentialsLoading}
						<p class="mt-1 text-xs text-gray-600 dark:text-gray-400">
							Loading credentials...
						</p>
					{:else if filteredCredentials.length === 0}
						<p class="mt-1 text-xs text-red-600 dark:text-red-400">
							No GitHub credentials found. Please create GitHub credentials first.
						</p>
					{/if}
				</div>

				<!-- Pool Balancer Type -->
				<div>
					<div class="flex items-center mb-1">
						<label for="pool_balancer_type" class="block text-sm font-medium text-gray-700 dark:text-gray-300">
							Pool Balancer Type
						</label>
						<div class="ml-2 relative group">
							<svg class="w-4 h-4 text-gray-400 cursor-help" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
							</svg>
							<div class="absolute bottom-full left-1/2 transform -translate-x-1/2 mb-2 w-64 p-3 bg-gray-900 text-white text-xs rounded-lg shadow-lg opacity-0 invisible group-hover:opacity-100 group-hover:visible transition-all duration-200 z-50">
								<div class="mb-2">
									<strong>Round Robin:</strong> Cycles through pools in turn. Job 1 → Pool 1, Job 2 → Pool 2, etc.
								</div>
								<div>
									<strong>Pack:</strong> Uses first available pool until full, then moves to next pool.
								</div>
								<div class="absolute top-full left-1/2 transform -translate-x-1/2 border-4 border-transparent border-t-gray-900"></div>
							</div>
						</div>
					</div>
					<select
						id="pool_balancer_type"
						bind:value={formData.pool_balancer_type}
						class="block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white sm:text-sm"
					>
						<option value="roundrobin">Round Robin</option>
						<option value="pack">Pack</option>
					</select>
				</div>

				<!-- Webhook Secret -->
				<div>
					<label for="webhook_secret" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						Webhook Secret
					</label>
					<input
						id="webhook_secret"
						type="password"
						bind:value={formData.webhook_secret}
						class="block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white sm:text-sm"
						placeholder="Enter webhook secret"
					/>
					<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
						You'll need to manually configure this secret in GitHub's enterprise webhook settings.
					</p>
				</div>

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
						{loading ? 'Creating...' : 'Create Enterprise'}
					</button>
				</div>
			</form>
		{/if}
	</div>
</Modal>