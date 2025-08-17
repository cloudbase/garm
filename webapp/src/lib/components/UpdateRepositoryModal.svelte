<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import { extractAPIError } from '$lib/utils/apiError';
	import type { Repository, UpdateEntityParams } from '$lib/api/generated/api.js';
	import Modal from './Modal.svelte';

	export let repository: Repository;

	const dispatch = createEventDispatcher<{
		close: void;
		submit: UpdateEntityParams;
	}>();

	let loading = false;
	let error = '';
	let webhookSecret = '';
	let changeWebhookSecret = false;

	async function handleSubmit() {
		try {
			loading = true;
			error = '';

			const params: UpdateEntityParams = {};

			// Only include webhook secret if user wants to change it and provided a value
			if (changeWebhookSecret) {
				if (!webhookSecret.trim()) {
					error = 'Please enter a webhook secret or uncheck the option to change it';
					return;
				}
				params.webhook_secret = webhookSecret;
			}

			// Only submit if there are actual changes
			if (Object.keys(params).length === 0) {
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
</script>

<Modal on:close={() => dispatch('close')}>
	<div class="max-w-2xl w-full">
		<div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
			<h2 class="text-xl font-semibold text-gray-900 dark:text-white">Update Repository</h2>
			<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{repository.owner}/{repository.name}</p>
		</div>

		<form on:submit|preventDefault={handleSubmit} class="p-6">
			{#if error}
				<div class="mb-4 rounded-md bg-red-50 dark:bg-red-900 p-4">
					<p class="text-sm font-medium text-red-800 dark:text-red-200">{error}</p>
				</div>
			{/if}

			<!-- Repository Info (Read-only) -->
			<div class="mb-6 bg-gray-50 dark:bg-gray-900 p-4 rounded-lg">
				<h3 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Repository Information</h3>
				<div class="space-y-2 text-sm">
					<div>
						<span class="text-gray-500 dark:text-gray-400">Owner:</span>
						<span class="ml-2 text-gray-900 dark:text-white">{repository.owner}</span>
					</div>
					<div>
						<span class="text-gray-500 dark:text-gray-400">Name:</span>
						<span class="ml-2 text-gray-900 dark:text-white">{repository.name}</span>
					</div>
					<div>
						<span class="text-gray-500 dark:text-gray-400">Endpoint:</span>
						<span class="ml-2 text-gray-900 dark:text-white">{repository.endpoint?.name}</span>
					</div>
					<div>
						<span class="text-gray-500 dark:text-gray-400">Credentials:</span>
						<span class="ml-2 text-gray-900 dark:text-white">{repository.credentials_name}</span>
					</div>
				</div>
			</div>

			<!-- Webhook Secret -->
			<div class="space-y-4">
				<div class="flex items-center">
					<input
						id="changeWebhookSecret"
						type="checkbox"
						bind:checked={changeWebhookSecret}
						class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 dark:border-gray-600 rounded"
					/>
					<label for="changeWebhookSecret" class="ml-2 block text-sm text-gray-700 dark:text-gray-300">
						I want to change the webhook secret
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

			<!-- Action Buttons -->
			<div class="flex justify-end space-x-3 pt-6 border-t border-gray-200 dark:border-gray-700 mt-6">
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
						Update Repository
					{/if}
				</button>
			</div>
		</form>
	</div>
</Modal>