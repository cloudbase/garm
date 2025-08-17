<script lang="ts">
	import { garmApi } from '$lib/api/client.js';
	import type { HookInfo } from '$lib/api/generated/api.js';
	import { toastStore } from '$lib/stores/toast.js';
	import Button from './Button.svelte';
	import { createEventDispatcher } from 'svelte';
	import { extractAPIError } from '$lib/utils/apiError';

	export let entityType: 'repository' | 'organization';
	export let entityId: string;
	export let entityName: string;

	let webhookInfo: HookInfo | null = null;
	let loading = false;
	let checking = true;

	const dispatch = createEventDispatcher<{
		webhookStatusChanged: { installed: boolean };
	}>();

	async function checkWebhookStatus() {
		if (!entityId) return;
		
		try {
			checking = true;
			if (entityType === 'repository') {
				webhookInfo = await garmApi.getRepositoryWebhookInfo(entityId);
			} else {
				webhookInfo = await garmApi.getOrganizationWebhookInfo(entityId);
			}
		} catch (err) {
			// If we get a 404, it means no webhook is installed
			if (err && typeof err === 'object' && 'response' in err && (err as any).response?.status === 404) {
				webhookInfo = null;
			} else {
				console.warn('Failed to check webhook status:', err);
				webhookInfo = null;
			}
		} finally {
			checking = false;
		}
	}

	async function installWebhook() {
		if (!entityId) return;
		
		try {
			loading = true;
			
			if (entityType === 'repository') {
				await garmApi.installRepositoryWebhook(entityId);
			} else {
				await garmApi.installOrganizationWebhook(entityId);
			}
			
			toastStore.success(
				'Webhook Installed',
				`Webhook for ${entityType} ${entityName} has been installed successfully.`
			);
			
			// Refresh webhook status
			await checkWebhookStatus();
			dispatch('webhookStatusChanged', { installed: true });
		} catch (err) {
			toastStore.error(
				'Webhook Installation Failed',
				err instanceof Error ? err.message : 'Failed to install webhook.'
			);
		} finally {
			loading = false;
		}
	}

	async function uninstallWebhook() {
		if (!entityId) return;
		
		try {
			loading = true;
			
			if (entityType === 'repository') {
				await garmApi.uninstallRepositoryWebhook(entityId);
			} else {
				await garmApi.uninstallOrganizationWebhook(entityId);
			}
			
			toastStore.success(
				'Webhook Uninstalled',
				`Webhook for ${entityType} ${entityName} has been uninstalled successfully.`
			);
			
			// Refresh webhook status
			await checkWebhookStatus();
			dispatch('webhookStatusChanged', { installed: false });
		} catch (err) {
			toastStore.error(
				'Webhook Uninstall Failed',
				extractAPIError(err)
			);
		} finally {
			loading = false;
		}
	}

	// Check webhook status when component mounts or entityId changes
	$: if (entityId) {
		checkWebhookStatus();
	}

	$: isInstalled = webhookInfo && webhookInfo.active;
</script>

<div class="bg-white dark:bg-gray-800 shadow rounded-lg">
	<div class="px-4 py-5 sm:p-6">
		<div class="flex items-center justify-between">
			<div>
				<h3 class="text-lg font-medium text-gray-900 dark:text-white">
					Webhook Status
				</h3>
				<div class="mt-1 flex items-center">
					{#if checking}
						<div class="flex items-center">
							<div class="animate-spin rounded-full h-4 w-4 border-b-2 border-blue-600 mr-2"></div>
							<span class="text-sm text-gray-500 dark:text-gray-400">Checking...</span>
						</div>
					{:else if isInstalled}
						<div class="flex items-center">
							<svg class="w-4 h-4 text-green-500 mr-2" fill="currentColor" viewBox="0 0 20 20">
								<path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/>
							</svg>
							<span class="text-sm text-green-700 dark:text-green-300">Webhook installed</span>
						</div>
						{#if webhookInfo}
							<div class="ml-4 text-xs text-gray-500 dark:text-gray-400">
								URL: {webhookInfo.url || 'N/A'}
							</div>
						{/if}
					{:else}
						<div class="flex items-center">
							<svg class="w-4 h-4 text-gray-400 mr-2" fill="currentColor" viewBox="0 0 20 20">
								<path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm0-2a6 6 0 100-12 6 6 0 000 12zm0-10a1 1 0 011 1v3a1 1 0 01-2 0V7a1 1 0 011-1z" clip-rule="evenodd"/>
							</svg>
							<span class="text-sm text-gray-500 dark:text-gray-400">No webhook installed</span>
						</div>
					{/if}
				</div>
			</div>
			
			<div class="flex space-x-2">
				{#if !checking}
					{#if isInstalled}
						<Button
							variant="danger"
							size="sm"
							disabled={loading}
							on:click={uninstallWebhook}
						>
							{loading ? 'Uninstalling...' : 'Uninstall'}
						</Button>
					{:else}
						<Button
							variant="primary"
							size="sm"
							disabled={loading}
							on:click={installWebhook}
						>
							{loading ? 'Installing...' : 'Install Webhook'}
						</Button>
					{/if}
				{/if}
			</div>
		</div>
	</div>
</div>