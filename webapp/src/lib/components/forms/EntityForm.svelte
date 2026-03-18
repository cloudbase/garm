<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import type { ForgeCredentials } from '$lib/api/generated/api.js';
	import ForgeTypeSelector from '../ForgeTypeSelector.svelte';
	import { generateSecureWebhookSecret } from '$lib/utils/crypto';

	const dispatch = createEventDispatcher<{
		forgeTypeSelect: 'github' | 'gitea';
		credentialChange: void;
	}>();

	export let entityType: 'repository' | 'organization' | 'enterprise' | '' = '';
	export let name = '';
	export let owner = '';
	export let credentialsName = '';
	export let poolBalancerType = 'roundrobin';
	export let agentMode = false;
	export let installWebhook = true;
	export let webhookSecret = '';
	export let autoGenerateSecret = true;

	// Credentials data & display
	export let credentials: ForgeCredentials[] = [];
	export let showCredentialsSelector = true;
	export let fixedCredentialsName = '';

	// Forge type filtering
	export let forgeType: 'github' | 'gitea' | '' = '';
	export let showForgeTypeSelector = false;

	// Entity type selector
	export let showEntityTypeSelector = false;

	export let idPrefix = '';

	// Filter credentials by forge type
	$: filteredCredentials = credentials.filter(cred => {
		if (!forgeType) return true;
		return cred.forge_type === forgeType;
	});

	function handleForgeTypeSelect(event: CustomEvent<'github' | 'gitea'>) {
		forgeType = event.detail;
		credentialsName = '';
		dispatch('forgeTypeSelect', event.detail);
	}

	function handleCredentialChange() {
		if (credentialsName) {
			const credential = credentials.find(c => c.name === credentialsName);
			if (credential && credential.forge_type) {
				forgeType = credential.forge_type as 'github' | 'gitea';
			}
		}
		dispatch('credentialChange');
	}

	// Auto-generate webhook secret
	$: if (autoGenerateSecret) {
		webhookSecret = generateSecureWebhookSecret();
	}
</script>

{#if showForgeTypeSelector}
	<ForgeTypeSelector
		bind:selectedForgeType={forgeType}
		on:select={handleForgeTypeSelect}
	/>
{/if}

{#if showEntityTypeSelector}
	<!-- Entity type selector cards -->
	<div>
		<p class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">Entity Type</p>
		<div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
			<button
				type="button"
				on:click={() => { entityType = 'repository'; name = ''; owner = ''; }}
				class="p-4 rounded-lg border-2 text-center transition-colors cursor-pointer
					{entityType === 'repository'
						? 'border-blue-500 bg-blue-50 dark:bg-blue-900/30'
						: 'border-gray-200 dark:border-gray-600 hover:border-gray-300 dark:hover:border-gray-500'}"
			>
				<svg class="w-8 h-8 mx-auto mb-2 {entityType === 'repository' ? 'text-blue-600 dark:text-blue-400' : 'text-gray-400'}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/>
				</svg>
				<p class="text-sm font-medium {entityType === 'repository' ? 'text-blue-600 dark:text-blue-400' : 'text-gray-900 dark:text-white'}">Repository</p>
			</button>

			<button
				type="button"
				on:click={() => { entityType = 'organization'; name = ''; owner = ''; }}
				class="p-4 rounded-lg border-2 text-center transition-colors cursor-pointer
					{entityType === 'organization'
						? 'border-blue-500 bg-blue-50 dark:bg-blue-900/30'
						: 'border-gray-200 dark:border-gray-600 hover:border-gray-300 dark:hover:border-gray-500'}"
			>
				<svg class="w-8 h-8 mx-auto mb-2 {entityType === 'organization' ? 'text-blue-600 dark:text-blue-400' : 'text-gray-400'}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"/>
				</svg>
				<p class="text-sm font-medium {entityType === 'organization' ? 'text-blue-600 dark:text-blue-400' : 'text-gray-900 dark:text-white'}">Organization</p>
			</button>

			<button
				type="button"
				on:click={() => { if (forgeType !== 'gitea') { entityType = 'enterprise'; name = ''; owner = ''; } }}
				disabled={forgeType === 'gitea'}
				class="p-4 rounded-lg border-2 text-center transition-colors
					{forgeType === 'gitea' ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}
					{entityType === 'enterprise'
						? 'border-blue-500 bg-blue-50 dark:bg-blue-900/30'
						: 'border-gray-200 dark:border-gray-600 hover:border-gray-300 dark:hover:border-gray-500'}"
			>
				<svg class="w-8 h-8 mx-auto mb-2 {entityType === 'enterprise' ? 'text-blue-600 dark:text-blue-400' : 'text-gray-400'}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"/>
				</svg>
				<p class="text-sm font-medium {entityType === 'enterprise' ? 'text-blue-600 dark:text-blue-400' : 'text-gray-900 dark:text-white'}">Enterprise</p>
				{#if forgeType === 'gitea'}
					<p class="text-xs text-gray-400 mt-1">GitHub only</p>
				{/if}
			</button>
		</div>
	</div>
{/if}

<!-- Name / Owner fields -->
{#if entityType === 'repository'}
	<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
		<div>
			<label for="{idPrefix}owner" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
				Owner <span class="text-red-500">*</span>
			</label>
			<input
				type="text"
				id="{idPrefix}owner"
				bind:value={owner}
				required
				autocomplete="off"
				class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
				placeholder="e.g., my-org"
			/>
		</div>
		<div>
			<label for="{idPrefix}name" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
				Repository Name <span class="text-red-500">*</span>
			</label>
			<input
				type="text"
				id="{idPrefix}name"
				bind:value={name}
				required
				autocomplete="off"
				class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
				placeholder="e.g., my-repo"
			/>
		</div>
	</div>
{:else if entityType}
	<div>
		<label for="{idPrefix}name" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
			{entityType === 'organization' ? 'Organization' : 'Enterprise'} Name <span class="text-red-500">*</span>
		</label>
		<input
			type="text"
			id="{idPrefix}name"
			bind:value={name}
			required
			autocomplete="off"
			class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
			placeholder="Enter {entityType} name"
		/>
	</div>
{/if}

{#if entityType}
<!-- Credentials: selector or readonly -->
{#if showCredentialsSelector}
	<div>
		<label for="{idPrefix}credentials" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
			Credentials
		</label>
		<select
			id="{idPrefix}credentials"
			bind:value={credentialsName}
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
{:else if fixedCredentialsName}
	<div>
		<label for="{idPrefix}credentials" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
			Credentials
		</label>
		<input
			type="text"
			id="{idPrefix}credentials"
			value={fixedCredentialsName}
			disabled
			class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-gray-100 dark:bg-gray-600 text-gray-500 dark:text-gray-400 cursor-not-allowed"
		/>
	</div>
{/if}

<!-- Pool Balancer Type -->
<div>
	<div class="flex items-center mb-1">
		<label for="{idPrefix}balancer" class="block text-sm font-medium text-gray-700 dark:text-gray-300">
			Pool Balancer Type
		</label>
		<div class="ml-2 relative group">
			<svg class="w-4 h-4 text-gray-400 cursor-help" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
			</svg>
			<div class="absolute bottom-full left-1/2 transform -translate-x-1/2 mb-2 w-80 p-3 bg-gray-900 text-white text-xs rounded-lg shadow-lg opacity-0 invisible group-hover:opacity-100 group-hover:visible transition-all duration-200 z-50">
				<div class="mb-2"><strong>Round Robin:</strong> Cycles through pools in turn. Job 1 → Pool 1, Job 2 → Pool 2, etc.</div>
				<div><strong>Pack:</strong> Uses first available pool until full, then moves to next.</div>
				<div class="absolute top-full left-1/2 transform -translate-x-1/2 border-4 border-transparent border-t-gray-900"></div>
			</div>
		</div>
	</div>
	<select
		id="{idPrefix}balancer"
		bind:value={poolBalancerType}
		class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
	>
		<option value="roundrobin">Round Robin</option>
		<option value="pack">Pack</option>
	</select>
</div>

<!-- Agent Mode -->
<div class="flex items-center">
	<input
		id="{idPrefix}agent-mode"
		type="checkbox"
		bind:checked={agentMode}
		class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 dark:border-gray-600 rounded"
	/>
	<label for="{idPrefix}agent-mode" class="ml-2 text-sm font-medium text-gray-700 dark:text-gray-300">
		Agent Mode
	</label>
	<div class="ml-2 relative group">
		<svg class="w-4 h-4 text-gray-400 cursor-help" fill="none" stroke="currentColor" viewBox="0 0 24 24">
			<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
		</svg>
		<div class="absolute bottom-full left-1/2 transform -translate-x-1/2 mb-2 w-80 p-3 bg-gray-900 text-white text-xs rounded-lg shadow-lg opacity-0 invisible group-hover:opacity-100 group-hover:visible transition-all duration-200 z-50">
			When enabled, runners will be installed with the GARM agent via userdata install templates.
			<div class="absolute top-full left-1/2 transform -translate-x-1/2 border-4 border-transparent border-t-gray-900"></div>
		</div>
	</div>
</div>

<!-- Webhook Configuration -->
{#if entityType !== 'enterprise'}
	<div class="space-y-3 border-t border-gray-200 dark:border-gray-700 pt-4">
		<p class="text-sm font-medium text-gray-700 dark:text-gray-300">Webhook Configuration</p>
		<div class="flex items-center">
			<input
				id="{idPrefix}install-webhook"
				type="checkbox"
				bind:checked={installWebhook}
				class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 dark:border-gray-600 rounded"
			/>
			<label for="{idPrefix}install-webhook" class="ml-2 text-sm text-gray-700 dark:text-gray-300">
				Install Webhook
			</label>
		</div>

		<div class="flex items-center">
			<input
				id="{idPrefix}generate-secret"
				type="checkbox"
				bind:checked={autoGenerateSecret}
				class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 dark:border-gray-600 rounded"
			/>
			<label for="{idPrefix}generate-secret" class="ml-2 text-sm text-gray-700 dark:text-gray-300">
				Auto-generate webhook secret
			</label>
		</div>

		{#if !autoGenerateSecret}
			<input
				type="password"
				bind:value={webhookSecret}
				class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
				placeholder="Enter webhook secret"
			/>
		{:else}
			<p class="text-xs text-gray-500 dark:text-gray-400">Webhook secret will be automatically generated</p>
		{/if}
	</div>
{:else}
	<!-- Enterprise: manual webhook secret -->
	<div class="space-y-3 border-t border-gray-200 dark:border-gray-700 pt-4">
		<p class="text-sm font-medium text-gray-700 dark:text-gray-300">Webhook Secret</p>
		<p class="text-xs text-gray-500 dark:text-gray-400">
			Enterprise webhooks must be configured manually. Provide a webhook secret here that matches your enterprise webhook configuration.
		</p>
		<div class="flex items-center">
			<input
				id="{idPrefix}generate-secret-ent"
				type="checkbox"
				bind:checked={autoGenerateSecret}
				class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 dark:border-gray-600 rounded"
			/>
			<label for="{idPrefix}generate-secret-ent" class="ml-2 text-sm text-gray-700 dark:text-gray-300">
				Auto-generate webhook secret
			</label>
		</div>
		{#if !autoGenerateSecret}
			<input
				type="password"
				bind:value={webhookSecret}
				class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
				placeholder="Enter webhook secret"
			/>
		{/if}
	</div>
{/if}
{/if}
