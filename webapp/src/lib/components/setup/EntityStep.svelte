<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import { garmApi } from '$lib/api/client.js';
	import type { CreateRepoParams, CreateOrgParams, CreateEnterpriseParams } from '$lib/api/generated/api.js';
	import EntityForm from '../forms/EntityForm.svelte';
	import { toastStore } from '$lib/stores/toast.js';
	import { extractAPIError } from '$lib/utils/apiError';
	import { getForgeIcon } from '$lib/utils/common.js';

	export let endpointName: string;
	export let forgeType: 'github' | 'gitea' | '';
	export let credentialsName: string;

	const dispatch = createEventDispatcher<{
		complete: { entityType: 'repository' | 'organization' | 'enterprise'; entityId: string; entityName: string };
		back: void;
	}>();

	let entityType: 'repository' | 'organization' | 'enterprise' | '' = '';
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

	$: isFormValid = (() => {
		if (!entityType) return false;
		if (!name.trim()) return false;
		if (entityType === 'repository' && !owner.trim()) return false;
		if (!autoGenerateSecret && !webhookSecret.trim()) return false;
		return true;
	})();

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
			}

			dispatch('complete', { entityType: entityType as 'repository' | 'organization' | 'enterprise', entityId, entityName });
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
			Create a repository, organization, or enterprise to manage runners for.
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
			idPrefix="entity-"
		/>

		<div class="flex justify-between pt-4">
			<button type="button" on:click={() => dispatch('back')}
				class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-600 hover:bg-gray-200 dark:hover:bg-gray-500 rounded-md cursor-pointer">
				Back
			</button>
			{#if entityType}
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
			{/if}
		</div>
	</form>
</div>
