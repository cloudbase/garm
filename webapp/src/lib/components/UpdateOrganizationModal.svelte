<script lang="ts">
	import { createEventDispatcher, onMount } from 'svelte';
	import { garmApi } from '$lib/api/client.js';
	import { extractAPIError } from '$lib/utils/apiError';
	import type { Organization, UpdateEntityParams, ForgeCredentials } from '$lib/api/generated/api.js';
	import Modal from './Modal.svelte';

	export let organization: Organization;

	const dispatch = createEventDispatcher<{
		close: void;
		submit: UpdateEntityParams;
	}>();

	let loading = false;
	let error = '';
	let credentials: ForgeCredentials[] = [];

	// Form data
	let formData: UpdateEntityParams = {
		credentials_name: organization.credentials_name || '',
		webhook_secret: '',
		pool_balancer_type: organization.pool_balancing_type || 'roundrobin'
	};

	let changeWebhookSecret = false;
	let generateWebhookSecret = true;

	// Get the organization's endpoint type
	$: organizationEndpointType = organization.endpoint?.endpoint_type;

	// Filter credentials to match the organization's endpoint type
	$: filteredCredentials = credentials.filter(cred => {
		return cred.forge_type === organizationEndpointType;
	});

	async function loadCredentials() {
		try {
			loading = true;
			credentials = await garmApi.listAllCredentials();
		} catch (err) {
			error = extractAPIError(err);
		} finally {
			loading = false;
		}
	}

	async function handleSubmit() {
		if (!formData.credentials_name) {
			error = 'Please select credentials';
			return;
		}

		if (changeWebhookSecret && !generateWebhookSecret && !formData.webhook_secret?.trim()) {
			error = 'Please enter a webhook secret or uncheck the change webhook secret option';
			return;
		}

		try {
			loading = true;
			error = '';

			const submitData: UpdateEntityParams = {
				...formData
			};

			// Only include webhook_secret if user wants to change it
			if (!changeWebhookSecret) {
				delete submitData.webhook_secret;
			} else if (generateWebhookSecret) {
				submitData.webhook_secret = ''; // Empty string triggers auto-generation
			}

			dispatch('submit', submitData);
		} catch (err) {
			error = extractAPIError(err)
			loading = false;
		}
	}

	onMount(() => {
		loadCredentials();
	});
</script>

<Modal on:close={() => dispatch('close')}>
	<div class="max-w-2xl w-full">
		<div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
			<h2 class="text-xl font-semibold text-gray-900 dark:text-white">Update Organization</h2>
			<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{organization.name}</p>
		</div>

		<div class="p-6">

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
				<!-- Credentials -->
				<div>
					<label for="credentials" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						Credentials
					</label>
					<select
						id="credentials"
						bind:value={formData.credentials_name}
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
					<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
						Only showing credentials for {organizationEndpointType} endpoints
					</p>
				</div>

				<!-- Pool Balancer Type -->
				<div>
					<label for="pool_balancer_type" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						Pool Balancer Type
					</label>
					<select
						id="pool_balancer_type"
						bind:value={formData.pool_balancer_type}
						class="block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white sm:text-sm"
					>
						<option value="roundrobin">Round Robin</option>
						<option value="pack">Pack</option>
					</select>
				</div>

				<!-- Change Webhook Secret -->
				<div>
					<div class="flex items-center mb-2">
						<input
							id="change-webhook-secret"
							type="checkbox"
							bind:checked={changeWebhookSecret}
							class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 dark:border-gray-600 rounded"
						/>
						<label for="change-webhook-secret" class="ml-2 text-sm font-medium text-gray-700 dark:text-gray-300">
							I want to change the webhook secret
						</label>
					</div>

					{#if changeWebhookSecret}
						<div class="ml-6 space-y-2">
							<div class="flex items-center">
								<input
									id="generate-webhook-secret"
									type="checkbox"
									bind:checked={generateWebhookSecret}
									class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 dark:border-gray-600 rounded"
								/>
								<label for="generate-webhook-secret" class="ml-2 text-sm text-gray-700 dark:text-gray-300">
									Auto-generate new secret
								</label>
							</div>
							{#if !generateWebhookSecret}
								<input
									type="password"
									bind:value={formData.webhook_secret}
									required={changeWebhookSecret && !generateWebhookSecret}
									class="block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white sm:text-sm"
									placeholder="Enter new webhook secret"
								/>
							{:else}
								<p class="text-sm text-gray-500 dark:text-gray-400">
									A new webhook secret will be automatically generated
								</p>
							{/if}
						</div>
					{/if}
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
						disabled={loading || (changeWebhookSecret && !generateWebhookSecret && !formData.webhook_secret?.trim())}
						class="px-4 py-2 text-sm font-medium text-white bg-blue-600 border border-transparent rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 dark:focus:ring-offset-gray-900 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
					>
						{loading ? 'Updating...' : 'Update Organization'}
					</button>
				</div>
			</form>
		{/if}
		</div>
	</div>
</Modal>