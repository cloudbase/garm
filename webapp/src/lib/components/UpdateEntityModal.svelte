<script lang="ts">
	import { createEventDispatcher, onMount } from 'svelte';
	import type { Repository, Organization, Enterprise, ForgeCredentials, UpdateEntityParams } from '$lib/api/generated/api.js';
	import { garmApi } from '$lib/api/client.js';
	import { extractAPIError } from '$lib/utils/apiError';
	import Modal from './Modal.svelte';

	type Entity = Repository | Organization | Enterprise;
	
	export let entity: Entity;
	export let entityType: 'repository' | 'organization' | 'enterprise';

	const dispatch = createEventDispatcher<{
		close: void;
		submit: UpdateEntityParams;
	}>();

	let loading = false;
	let error = '';
	let credentials: ForgeCredentials[] = [];
	let loadingCredentials = false;
	
	// Form fields
	let selectedCredentials = '';
	let poolBalancerType = '';
	let webhookSecret = '';
	let changeWebhookSecret = false;

	function getEntityDisplayName(): string {
		if (entityType === 'repository') {
			const repo = entity as Repository;
			return `${repo.owner}/${repo.name}`;
		}
		return entity.name || '';
	}

	function getEntityTitle(): string {
		return entityType.charAt(0).toUpperCase() + entityType.slice(1);
	}

	function getOwner(): string {
		if (entityType === 'repository') {
			return (entity as Repository).owner || '';
		}
		return '';
	}

	async function loadCredentials() {
		try {
			loadingCredentials = true;
			credentials = await garmApi.listCredentials();
		} catch (err) {
			error = extractAPIError(err);
		} finally {
			loadingCredentials = false;
		}
	}

	function initializeForm() {
		// Initialize form with current entity values
		selectedCredentials = entity.credentials_name || '';
		poolBalancerType = entity.pool_balancing_type || 'roundrobin';
		webhookSecret = '';
		changeWebhookSecret = false;
	}

	async function handleSubmit() {
		try {
			loading = true;
			error = '';

			const params: UpdateEntityParams = {};
			let hasChanges = false;

			// Check if credentials changed
			if (selectedCredentials && selectedCredentials !== entity.credentials_name) {
				params.credentials_name = selectedCredentials;
				hasChanges = true;
			}

			// Check if pool balancer type changed
			if (poolBalancerType && poolBalancerType !== entity.pool_balancing_type) {
				params.pool_balancer_type = poolBalancerType as any;
				hasChanges = true;
			}

			// Only include webhook secret if user wants to change it
			if (changeWebhookSecret) {
				if (!webhookSecret.trim()) {
					error = 'Please enter a webhook secret or uncheck the option to change it';
					return;
				}
				params.webhook_secret = webhookSecret;
				hasChanges = true;
			}

			// Only submit if there are actual changes
			if (!hasChanges) {
				dispatch('close');
				return;
			}

			dispatch('submit', params);
		} catch (err) {
			error = extractAPIError(err);
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		loadCredentials();
		initializeForm();
	});
</script>

<Modal on:close={() => dispatch('close')}>
	<div class="max-w-2xl w-full">
		<div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
			<h2 class="text-xl font-semibold text-gray-900 dark:text-white">Update {getEntityTitle()}</h2>
			<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{getEntityDisplayName()}</p>
		</div>

		<form on:submit|preventDefault={handleSubmit} class="p-6 space-y-6">
			{#if error}
				<div class="rounded-md bg-red-50 dark:bg-red-900 p-4">
					<p class="text-sm font-medium text-red-800 dark:text-red-200">{error}</p>
				</div>
			{/if}

			<!-- Entity Info (Read-only) -->
			<div class="bg-gray-50 dark:bg-gray-900 p-4 rounded-lg">
				<h3 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">{getEntityTitle()} Information</h3>
				<div class="space-y-2 text-sm">
					{#if entityType === 'repository'}
						<div>
							<span class="text-gray-500 dark:text-gray-400">Owner:</span>
							<span class="ml-2 text-gray-900 dark:text-white">{getOwner()}</span>
						</div>
					{/if}
					<div>
						<span class="text-gray-500 dark:text-gray-400">Name:</span>
						<span class="ml-2 text-gray-900 dark:text-white">{entity.name}</span>
					</div>
					<div>
						<span class="text-gray-500 dark:text-gray-400">Endpoint:</span>
						<span class="ml-2 text-gray-900 dark:text-white">{entity.endpoint?.name}</span>
					</div>
					<div>
						<span class="text-gray-500 dark:text-gray-400">Current Credentials:</span>
						<span class="ml-2 text-gray-900 dark:text-white">{entity.credentials_name}</span>
					</div>
					<div>
						<span class="text-gray-500 dark:text-gray-400">Current Pool Balancer:</span>
						<span class="ml-2 text-gray-900 dark:text-white">{entity.pool_balancing_type || 'roundrobin'}</span>
					</div>
				</div>
			</div>

			<div class="space-y-4">
				<!-- Credentials -->
				<div>
					<label for="credentials" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
						Credentials
					</label>
					{#if loadingCredentials}
						<div class="animate-pulse bg-gray-200 dark:bg-gray-700 h-10 rounded"></div>
					{:else}
						<select 
							id="credentials"
							bind:value={selectedCredentials}
							class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
						>
							<option value="">Keep current credentials</option>
							{#each credentials as credential}
								<option value={credential.name}>
									{credential.name} ({credential.endpoint?.name || 'Unknown'})
								</option>
							{/each}
						</select>
					{/if}
					<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
						Leave unchanged to keep current credentials
					</p>
				</div>

				<!-- Pool Balancer Type -->
				<div>
					<label for="poolBalancer" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
						Pool Balancer Type
					</label>
					<select 
						id="poolBalancer"
						bind:value={poolBalancerType}
						class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
					>
						<option value="roundrobin">Round Robin</option>
						<option value="pack">Pack</option>
					</select>
					<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
						Round Robin distributes jobs evenly across pools, Pack fills pools in order
					</p>
				</div>

				<!-- Webhook Secret -->
				<div class="space-y-3">
					<div class="flex items-center">
						<input
							id="changeWebhookSecret"
							type="checkbox"
							bind:checked={changeWebhookSecret}
							class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 dark:border-gray-600 rounded"
						/>
						<label for="changeWebhookSecret" class="ml-2 block text-sm text-gray-700 dark:text-gray-300">
							Change webhook secret
						</label>
					</div>

					{#if changeWebhookSecret}
						<div>
							<label for="webhookSecret" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
								New Webhook Secret <span class="text-red-500">*</span>
							</label>
							<input
								id="webhookSecret"
								type="password"
								bind:value={webhookSecret}
								required={changeWebhookSecret}
								placeholder="Enter new webhook secret"
								class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
							/>
							<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
								Leave empty to auto-generate a new secret
							</p>
						</div>
					{/if}
				</div>
			</div>

			<!-- Action Buttons -->
			<div class="flex justify-end space-x-3 pt-6 border-t border-gray-200 dark:border-gray-700">
				<button
					type="button"
					on:click={() => dispatch('close')}
					class="px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 cursor-pointer"
				>
					Cancel
				</button>
				<button
					type="submit"
					disabled={loading || (changeWebhookSecret && !webhookSecret.trim())}
					class="px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
				>
					{#if loading}
						<div class="flex items-center">
							<div class="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
							Updating...
						</div>
					{:else}
						Update {getEntityTitle()}
					{/if}
				</button>
			</div>
		</form>
	</div>
</Modal>