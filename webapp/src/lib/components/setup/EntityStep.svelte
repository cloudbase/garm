<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import { garmApi } from '$lib/api/client.js';
	import type { CreateRepoParams, CreateOrgParams, CreateEnterpriseParams, CreateForgeInstanceParams } from '$lib/api/generated/api.js';
	import EntityForm from '../forms/EntityForm.svelte';
	import { toastStore } from '$lib/stores/toast.js';
	import { extractAPIError } from '$lib/utils/apiError';
	import { getForgeIcon } from '$lib/utils/common.js';
	import { eagerCache } from '$lib/stores/eager-cache.js';

	export let endpointName: string;
	export let forgeType: 'github' | 'gitea' | '';
	export let credentialsName: string;

	const dispatch = createEventDispatcher<{
		complete: { entityType: 'repository' | 'organization' | 'enterprise' | 'forge_instance'; entityId: string; entityName: string };
		back: void;
	}>();

	let entityType: 'repository' | 'organization' | 'enterprise' | 'forge_instance' | '' = '';
	let creating = false;
	let error = '';

	// Form data
	let name = '';
	let owner = '';
	let poolBalancerType = 'roundrobin';
	let agentMode = false;
	let installWebhook = true;
	let autoGenerateSecret = true;
	let webhookSecret = '';

	// Forge instance existence check
	let existingForgeInstance: any = null;
	let checkingForgeInstance = false;

	async function checkExistingForgeInstance() {
		checkingForgeInstance = true;
		existingForgeInstance = null;
		try {
			const instances = await garmApi.listForgeInstances(endpointName);
			if (instances.length > 0) {
				existingForgeInstance = instances[0];
			}
		} catch {
			// ignore — will create fresh
		} finally {
			checkingForgeInstance = false;
		}
	}

	$: isFormValid = (() => {
		if (!entityType) return false;
		if (entityType === 'forge_instance') return !!existingForgeInstance || (!autoGenerateSecret ? !!webhookSecret.trim() : true);
		if (!name.trim()) return false;
		if (entityType === 'repository' && !owner.trim()) return false;
		if (!autoGenerateSecret && !webhookSecret.trim()) return false;
		return true;
	})();

	// Get live status from eager cache when available
	$: cachedForgeInstance = existingForgeInstance
		? $eagerCache.forgeInstances.find((fi: any) => fi.id === existingForgeInstance.id) || existingForgeInstance
		: null;

	// Check for existing forge instance when entity type changes
	$: if (entityType === 'forge_instance') {
		checkExistingForgeInstance();
	} else {
		existingForgeInstance = null;
	}

	async function handleCreate() {
		creating = true;
		error = '';
		try {
			let entityId = '';
			let entityName = '';

			if (entityType === 'repository') {
				const params: CreateRepoParams = {
					name: name.trim(),
					owner: owner.trim(),
					credentials_name: credentialsName,
					webhook_secret: webhookSecret,
					agent_mode: agentMode
				};
				const created = await garmApi.createRepository(params);
				entityId = created.id!;
				entityName = `${created.owner}/${created.name}`;

				if (installWebhook && created.id) {
					try {
						await garmApi.installRepoWebhook(created.id);
					} catch (webhookError) {
						toastStore.warning('Webhook Installation Failed', 'Repository created but webhook could not be installed. You can install it manually later.');
					}
				}
				toastStore.success('Repository Created', `Repository ${entityName} has been created successfully.`);
			} else if (entityType === 'organization') {
				const params: CreateOrgParams = {
					name: name.trim(),
					credentials_name: credentialsName,
					webhook_secret: webhookSecret,
					pool_balancer_type: poolBalancerType,
					agent_mode: agentMode
				};
				const created = await garmApi.createOrganization(params);
				entityId = created.id!;
				entityName = created.name!;

				if (installWebhook && created.id) {
					try {
						await garmApi.installOrganizationWebhook(created.id);
					} catch (webhookError) {
						toastStore.warning('Webhook Installation Failed', 'Organization created but webhook could not be installed. You can install it manually later.');
					}
				}
				toastStore.success('Organization Created', `Organization ${entityName} has been created successfully.`);
			} else if (entityType === 'enterprise') {
				const params: CreateEnterpriseParams = {
					name: name.trim(),
					credentials_name: credentialsName,
					webhook_secret: webhookSecret,
					agent_mode: agentMode
				};
				const created = await garmApi.createEnterprise(params);
				entityId = created.id!;
				entityName = created.name!;
				toastStore.success('Enterprise Created', `Enterprise ${entityName} has been created successfully.`);
			} else if (entityType === 'forge_instance') {
				// Use existing forge instance if found
				if (existingForgeInstance) {
					entityId = existingForgeInstance.id!;
					entityName = existingForgeInstance.endpoint?.name || endpointName;
					dispatch('complete', { entityType: 'forge_instance', entityId, entityName });
					return;
				}

				const params: CreateForgeInstanceParams = {
					endpoint_name: endpointName,
					credentials_name: credentialsName,
					webhook_secret: webhookSecret,
					forge_type: 'gitea',
					pool_balancer_type: poolBalancerType,
					agent_mode: agentMode
				};
				const created = await garmApi.createForgeInstance(params);
				entityId = created.id!;
				entityName = created.endpoint?.name || endpointName;

				if (installWebhook && created.id) {
					try {
						await garmApi.installForgeInstanceWebhook(created.id);
					} catch (webhookError) {
						toastStore.warning('Webhook Installation Failed', 'Forge instance created but webhook could not be installed. You can install it manually later.');
					}
				}
				toastStore.success('Forge Instance Created', `Forge instance for ${entityName} has been created successfully.`);
			}

			dispatch('complete', { entityType: entityType as 'repository' | 'organization' | 'enterprise' | 'forge_instance', entityId, entityName });
		} catch (err) {
			error = extractAPIError(err);
		} finally {
			creating = false;
		}
	}
</script>

<div class="space-y-6">
	<div>
		<h2 class="text-xl font-semibold text-gray-900 dark:text-white">Add Entity</h2>
		<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
			Create a repository, organization, enterprise, or forge instance to manage runners for.
		</p>
	</div>

	<!-- Context banner -->
	<div class="flex items-center space-x-4 p-3 bg-gray-50 dark:bg-gray-700 rounded-lg text-sm">
		<div class="flex items-center space-x-2">
			<span class="flex-shrink-0">{@html getForgeIcon(forgeType, 'w-4 h-4')}</span>
			<span class="text-gray-500 dark:text-gray-400">Endpoint:</span>
			<span class="font-medium text-gray-900 dark:text-white">{endpointName}</span>
		</div>
		<span class="text-gray-300 dark:text-gray-500">|</span>
		<div>
			<span class="text-gray-500 dark:text-gray-400">Credentials:</span>
			<span class="font-medium text-gray-900 dark:text-white">{credentialsName}</span>
		</div>
	</div>

	{#if error}
		<div class="rounded-md bg-red-50 dark:bg-red-900 p-4">
			<p class="text-sm font-medium text-red-800 dark:text-red-200">{error}</p>
		</div>
	{/if}

	<form on:submit|preventDefault={handleCreate} class="space-y-4">
		<EntityForm
			bind:entityType
			bind:name
			bind:owner
			bind:poolBalancerType
			bind:agentMode
			bind:installWebhook
			bind:webhookSecret
			bind:autoGenerateSecret
			{forgeType}
			fixedCredentialsName={credentialsName}
			showCredentialsSelector={false}
			showEntityTypeSelector={true}
			showEntityTypeSelectorOnly={entityType === 'forge_instance' && (checkingForgeInstance || !!existingForgeInstance)}
			idPrefix="entity-"
		/>

		<!-- When forge instance exists, show status card (form fields are hidden via EntityForm's forge_instance logic) -->
		{#if entityType === 'forge_instance' && checkingForgeInstance}
			<div class="flex items-center p-4 bg-gray-50 dark:bg-gray-700 rounded-lg">
				<div class="animate-spin rounded-full h-5 w-5 border-b-2 border-blue-600 mr-3"></div>
				<span class="text-sm text-gray-600 dark:text-gray-300">Checking for existing forge instance...</span>
			</div>
		{:else if entityType === 'forge_instance' && cachedForgeInstance}
			<div class="p-4 bg-green-50 dark:bg-green-900/30 border border-green-200 dark:border-green-800 rounded-lg">
				<div class="flex items-center">
					<svg class="w-5 h-5 text-green-500 mr-3 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
						<path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/>
					</svg>
					<div>
						<p class="text-sm font-medium text-green-800 dark:text-green-200">
							Forge instance already exists
						</p>
						<p class="text-xs text-green-600 dark:text-green-400 mt-1">
							Endpoint: {cachedForgeInstance.endpoint?.name || endpointName}
							&middot; Credentials: {cachedForgeInstance.credentials_name || cachedForgeInstance.credentials?.name || 'N/A'}
						</p>
					</div>
				</div>
			</div>
		{/if}

		<div class="flex justify-between pt-4">
			<button type="button" on:click={() => dispatch('back')}
				class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-600 hover:bg-gray-200 dark:hover:bg-gray-500 rounded-md cursor-pointer">
				Back
			</button>
			{#if entityType}
				<button
					type="submit"
					disabled={!isFormValid || creating || checkingForgeInstance}
					class="px-6 py-2 text-sm font-medium text-white rounded-md focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 dark:focus:ring-offset-gray-900 transition-colors
						{isFormValid && !creating && !checkingForgeInstance ? 'bg-blue-600 hover:bg-blue-700 cursor-pointer' : 'bg-gray-400 cursor-not-allowed'}"
				>
					{#if creating}
						<div class="flex items-center">
							<div class="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
							Creating...
						</div>
					{:else if entityType === 'forge_instance' && existingForgeInstance}
						Continue
					{:else}
						Create & Continue
					{/if}
				</button>
			{/if}
		</div>
	</form>
</div>
