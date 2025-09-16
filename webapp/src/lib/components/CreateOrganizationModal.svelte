<script lang="ts">
	import { createEventDispatcher, onMount } from 'svelte';
	import type { CreateOrgParams } from '$lib/api/generated/api.js';
	import Modal from './Modal.svelte';
	import ForgeTypeSelector from './ForgeTypeSelector.svelte';
	import { extractAPIError } from '$lib/utils/apiError';
	import { eagerCache, eagerCacheManager } from '$lib/stores/eager-cache.js';

	const dispatch = createEventDispatcher<{
		close: void;
		submit: CreateOrgParams & { install_webhook?: boolean; auto_generate_secret?: boolean };
	}>();

	let loading = false;
	let error = '';
	let selectedForgeType: 'github' | 'gitea' | '' = 'github';
	
	// Get credentials from eager cache
	$: credentials = $eagerCache.credentials;
	$: credentialsLoading = $eagerCache.loading.credentials;

	// Form data
	let formData: CreateOrgParams = {
		name: '',
		credentials_name: '',
		webhook_secret: '',
		pool_balancer_type: 'roundrobin',
		agent_mode: false
	};

	let installWebhook = true;
	let generateWebhookSecret = true;

	// Filtered credentials based on selected forge type
	$: filteredCredentials = credentials.filter(cred => {
		if (!selectedForgeType) return true;
		return cred.forge_type === selectedForgeType;
	});

	async function loadCredentialsIfNeeded() {
		if (!$eagerCache.loaded.credentials && !$eagerCache.loading.credentials) {
			try {
				await eagerCacheManager.getCredentials();
			} catch (err) {
				error = extractAPIError(err);
			}
		}
	}

	function handleForgeTypeSelect(event: CustomEvent<'github' | 'gitea'>) {
		selectedForgeType = event.detail;
		// Reset credential selection when forge type changes
		formData.credentials_name = '';
	}

	function handleCredentialChange() {
		// Auto-detect forge type when credential is selected
		if (formData.credentials_name) {
			const credential = credentials.find(c => c.name === formData.credentials_name);
			if (credential && credential.forge_type) {
				selectedForgeType = credential.forge_type as 'github' | 'gitea';
			}
		}
	}

	// Generate secure random webhook secret
	function generateSecureWebhookSecret(): string {
		const array = new Uint8Array(32);
		crypto.getRandomValues(array);
		return Array.from(array, byte => byte.toString(16).padStart(2, '0')).join('');
	}

	// Auto-generate webhook secret when checkbox is checked
	$: if (generateWebhookSecret) {
		formData.webhook_secret = generateSecureWebhookSecret();
	} else if (!generateWebhookSecret) {
		// Clear the secret if user unchecks auto-generate
		formData.webhook_secret = '';
	}

	// Check if all mandatory fields are filled
	$: isFormValid = formData.name?.trim() !== '' && 
					 formData.credentials_name !== '' &&
					 (generateWebhookSecret || (formData.webhook_secret && formData.webhook_secret.trim() !== ''));

	async function handleSubmit() {
		if (!formData.name?.trim()) {
			error = 'Organization name is required';
			return;
		}

		if (!formData.credentials_name) {
			error = 'Please select credentials';
			return;
		}

		try {
			loading = true;
			error = '';

			const submitData = {
				...formData,
				install_webhook: installWebhook,
				auto_generate_secret: generateWebhookSecret
			};

			dispatch('submit', submitData);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create organization';
			loading = false;
		}
	}

	onMount(() => {
		loadCredentialsIfNeeded();
	});
</script>

<Modal on:close={() => dispatch('close')}>
	<div class="max-w-2xl w-full p-6">
		<h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">Create Organization</h3>

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
				<!-- Forge Type Selection -->
				<ForgeTypeSelector 
					bind:selectedForgeType 
					on:select={handleForgeTypeSelect}
				/>

				<!-- Organization Name -->
				<div>
					<label for="name" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						Organization Name
					</label>
					<input
						id="name"
						type="text"
						bind:value={formData.name}
						required
						class="block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white sm:text-sm"
						placeholder="Enter organization name"
					/>
				</div>

				<!-- Credentials -->
				<div>
					<label for="credentials" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						Credentials
					</label>
					<select
						id="credentials"
						bind:value={formData.credentials_name}
						on:change={handleCredentialChange}
						required
						class="block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white sm:text-sm"
					>
						<option value="">Select credentials...</option>
						{#each filteredCredentials as credential}
							<option value={credential.name}>
								{credential.name} ({credential.endpoint?.name || 'Unknown endpoint'})
							</option>
						{/each}
					</select>
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
							<div class="absolute bottom-full left-1/2 transform -translate-x-1/2 mb-2 w-80 p-3 bg-gray-900 text-white text-xs rounded-lg shadow-lg opacity-0 invisible group-hover:opacity-100 group-hover:visible transition-all duration-200 z-50">
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

				<!-- Agent Mode -->
				<div>
					<div class="flex items-center mb-3">
						<input
							id="agent-mode"
							type="checkbox"
							bind:checked={formData.agent_mode}
							class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 dark:border-gray-600 rounded"
						/>
						<label for="agent-mode" class="ml-2 text-sm font-medium text-gray-700 dark:text-gray-300">
							Agent Mode
						</label>
						<div class="ml-2 relative group">
							<svg class="w-4 h-4 text-gray-400 cursor-help" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
							</svg>
							<div class="absolute bottom-full left-1/2 transform -translate-x-1/2 mb-2 w-80 p-3 bg-gray-900 text-white text-xs rounded-lg shadow-lg opacity-0 invisible group-hover:opacity-100 group-hover:visible transition-all duration-200 z-50">
								When enabled, runners will be installed with the GARM agent via userdata install templates. This allows for enhanced runner management and control.
								<div class="absolute top-full left-1/2 transform -translate-x-1/2 border-4 border-transparent border-t-gray-900"></div>
							</div>
						</div>
					</div>
				</div>

				<!-- Webhook Configuration -->
				<div>
					<div class="flex items-center mb-3">
						<input
							id="install-webhook"
							type="checkbox"
							bind:checked={installWebhook}
							class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 dark:border-gray-600 rounded"
						/>
						<label for="install-webhook" class="ml-2 text-sm font-medium text-gray-700 dark:text-gray-300">
							Install Webhook
						</label>
					</div>
					
					<div class="space-y-3">
						<div class="flex items-center">
							<input
								id="generate-webhook-secret"
								type="checkbox"
								bind:checked={generateWebhookSecret}
								class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 dark:border-gray-600 rounded"
							/>
							<label for="generate-webhook-secret" class="ml-2 text-sm text-gray-700 dark:text-gray-300">
								Auto-generate webhook secret
							</label>
						</div>
						
						{#if !generateWebhookSecret}
							<input
								type="password"
								bind:value={formData.webhook_secret}
								class="block w-full px-3 py-2 mt-3 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white sm:text-sm"
								placeholder="Enter webhook secret"
							/>
						{:else}
							<p class="text-sm text-gray-500 dark:text-gray-400">
								Webhook secret will be automatically generated
							</p>
						{/if}
					</div>
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
						disabled={loading || credentialsLoading || !isFormValid}
						class="px-4 py-2 text-sm font-medium text-white bg-blue-600 border border-transparent rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 dark:focus:ring-offset-gray-900 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
					>
						{loading ? 'Creating...' : 'Create Organization'}
					</button>
				</div>
			</form>
		{/if}
	</div>
</Modal>