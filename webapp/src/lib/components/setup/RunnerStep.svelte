<script lang="ts">
	import { createEventDispatcher, onMount } from 'svelte';
	import { garmApi } from '$lib/api/client.js';
	import type { CreatePoolParams, CreateScaleSetParams, Provider, Template } from '$lib/api/generated/api.js';
	import JsonEditor from '../JsonEditor.svelte';
	import { toastStore } from '$lib/stores/toast.js';
	import { extractAPIError } from '$lib/utils/apiError';
	import { getForgeIcon } from '$lib/utils/common.js';

	export let endpointName: string;
	export let forgeType: 'github' | 'gitea' | '';
	export let credentialsName: string;
	export let entityType: 'repository' | 'organization' | 'enterprise' | '';
	export let entityId: string;
	export let entityName: string;

	const dispatch = createEventDispatcher<{
		complete: { runnerType: 'pool' | 'scaleset'; runnerId: string };
		back: void;
	}>();

	let runnerType: 'pool' | 'scaleset' | '' = '';
	let creating = false;
	let error = '';

	// Loaded data
	let providers: Provider[] = [];
	let templates: Template[] = [];
	let loadingProviders = false;
	let loadingTemplates = false;

	// Form fields
	let scaleSetName = '';
	let selectedProvider = '';
	let image = '';
	let flavor = '';
	let maxRunners: number | undefined = undefined;
	let minIdleRunners: number | undefined = undefined;
	let runnerBootstrapTimeout: number | undefined = undefined;
	let priority: number = 100;
	let runnerPrefix = 'garm';
	let osType = 'linux';
	let osArch = 'amd64';
	let githubRunnerGroup = '';
	let enabled = true;
	let enableShell = false;
	let tags: string[] = [];
	let newTag = '';
	let extraSpecs = '{}';
	let selectedTemplate: number | undefined = undefined;

	$: isFormValid = (() => {
		if (!runnerType) return false;
		if (!selectedProvider || !image || !flavor) return false;
		if (runnerType === 'scaleset' && !scaleSetName.trim()) return false;
		return true;
	})();

	onMount(async () => {
		await loadProviders();
		await loadTemplates();
	});

	async function loadProviders() {
		try {
			loadingProviders = true;
			providers = await garmApi.listProviders();
		} catch (err) {
			error = extractAPIError(err);
		} finally {
			loadingProviders = false;
		}
	}

	async function loadTemplates() {
		try {
			loadingTemplates = true;
			templates = await garmApi.listTemplates(osType, undefined, forgeType);

			// Auto-select system template
			if (!selectedTemplate || !templates.find(t => t.id === selectedTemplate)) {
				const systemTemplate = templates.find(t => t.owner_id === 'system');
				if (systemTemplate) {
					selectedTemplate = systemTemplate.id;
				} else if (templates.length > 0) {
					selectedTemplate = templates[0].id;
				}
			}
		} catch (err) {
			// Templates are optional, don't show error
		} finally {
			loadingTemplates = false;
		}
	}

	// Reload templates when OS type changes
	$: if (osType) {
		loadTemplates();
	}

	function addTag() {
		if (newTag.trim() && !tags.includes(newTag.trim())) {
			tags = [...tags, newTag.trim()];
			newTag = '';
		}
	}

	function removeTag(index: number) {
		tags = tags.filter((_, i) => i !== index);
	}

	function handleTagKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter') {
			event.preventDefault();
			addTag();
		}
	}

	async function handleCreate() {
		if (!runnerType || !selectedProvider || !image || !flavor) {
			error = 'Please fill in all required fields';
			return;
		}

		creating = true;
		error = '';

		try {
			let parsedExtraSpecs: any = {};
			if (extraSpecs.trim() && extraSpecs.trim() !== '{}') {
				try {
					parsedExtraSpecs = JSON.parse(extraSpecs);
				} catch (e) {
					throw new Error('Invalid JSON in extra specs');
				}
			}

			let runnerId = '';

			if (runnerType === 'pool') {
				const params: CreatePoolParams = {
					provider_name: selectedProvider,
					image,
					flavor,
					max_runners: maxRunners || 10,
					min_idle_runners: minIdleRunners || 0,
					runner_bootstrap_timeout: runnerBootstrapTimeout || 20,
					priority,
					runner_prefix: runnerPrefix,
					os_type: osType as any,
					os_arch: osArch as any,
					'github-runner-group': githubRunnerGroup || undefined,
					enabled,
					enable_shell: enableShell,
					tags,
					extra_specs: extraSpecs.trim() && extraSpecs.trim() !== '{}' ? parsedExtraSpecs : undefined,
					template_id: selectedTemplate
				};

				let created;
				switch (entityType) {
					case 'repository':
						created = await garmApi.createRepositoryPool(entityId, params);
						break;
					case 'organization':
						created = await garmApi.createOrganizationPool(entityId, params);
						break;
					case 'enterprise':
						created = await garmApi.createEnterprisePool(entityId, params);
						break;
				}
				runnerId = created?.id || '';
				toastStore.success('Pool Created', `Pool has been created successfully for ${entityName}.`);
			} else {
				const params: CreateScaleSetParams = {
					name: scaleSetName.trim(),
					provider_name: selectedProvider,
					image,
					flavor,
					max_runners: maxRunners || 10,
					min_idle_runners: minIdleRunners || 0,
					runner_bootstrap_timeout: runnerBootstrapTimeout || 20,
					runner_prefix: runnerPrefix,
					os_type: osType as any,
					os_arch: osArch as any,
					'github-runner-group': githubRunnerGroup || undefined,
					enabled,
					enable_shell: enableShell,
					extra_specs: extraSpecs.trim() && extraSpecs.trim() !== '{}' ? parsedExtraSpecs : undefined,
					template_id: selectedTemplate
				};

				let created;
				switch (entityType) {
					case 'repository':
						created = await garmApi.createRepositoryScaleSet(entityId, params);
						break;
					case 'organization':
						created = await garmApi.createOrganizationScaleSet(entityId, params);
						break;
					case 'enterprise':
						created = await garmApi.createEnterpriseScaleSet(entityId, params);
						break;
				}
				runnerId = String(created?.id || '');
				toastStore.success('Scale Set Created', `Scale set "${scaleSetName}" has been created successfully for ${entityName}.`);
			}

			dispatch('complete', { runnerType, runnerId });
		} catch (err) {
			error = extractAPIError(err);
		} finally {
			creating = false;
		}
	}
</script>

<div class="space-y-6">
	<div>
		<h2 class="text-xl font-semibold text-gray-900 dark:text-white">Create Runner</h2>
		<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
			Configure a pool or scale set to manage your runners.
		</p>
	</div>

	<!-- Context banner -->
	<div class="flex flex-wrap items-center gap-x-4 gap-y-1 p-3 bg-gray-50 dark:bg-gray-700 rounded-lg text-sm">
		<div class="flex items-center space-x-2">
			<span class="flex-shrink-0">{@html getForgeIcon(forgeType, 'w-4 h-4')}</span>
			<span class="font-medium text-gray-900 dark:text-white">{endpointName}</span>
		</div>
		<span class="text-gray-300 dark:text-gray-500">|</span>
		<span class="text-gray-900 dark:text-white">{credentialsName}</span>
		<span class="text-gray-300 dark:text-gray-500">|</span>
		<span class="text-gray-900 dark:text-white capitalize">{entityType}: {entityName}</span>
	</div>

	{#if error}
		<div class="rounded-md bg-red-50 dark:bg-red-900 p-4">
			<p class="text-sm font-medium text-red-800 dark:text-red-200">{error}</p>
		</div>
	{/if}

	<!-- Runner type selector -->
	<div>
		<p class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">Runner Type</p>
		<div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
			<button
				type="button"
				on:click={() => runnerType = 'pool'}
				class="p-4 rounded-lg border-2 text-center transition-colors cursor-pointer
					{runnerType === 'pool'
						? 'border-blue-500 bg-blue-50 dark:bg-blue-900/30'
						: 'border-gray-200 dark:border-gray-600 hover:border-gray-300 dark:hover:border-gray-500'}"
			>
				<svg class="w-8 h-8 mx-auto mb-2 {runnerType === 'pool' ? 'text-blue-600 dark:text-blue-400' : 'text-gray-400'}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"/>
				</svg>
				<p class="text-sm font-medium {runnerType === 'pool' ? 'text-blue-600 dark:text-blue-400' : 'text-gray-900 dark:text-white'}">Pool</p>
				<p class="text-xs text-gray-500 dark:text-gray-400 mt-1">Standard runner pool</p>
			</button>

			<button
				type="button"
				on:click={() => { if (forgeType !== 'gitea') runnerType = 'scaleset'; }}
				disabled={forgeType === 'gitea'}
				class="p-4 rounded-lg border-2 text-center transition-colors
					{forgeType === 'gitea' ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}
					{runnerType === 'scaleset'
						? 'border-blue-500 bg-blue-50 dark:bg-blue-900/30'
						: 'border-gray-200 dark:border-gray-600 hover:border-gray-300 dark:hover:border-gray-500'}"
			>
				<svg class="w-8 h-8 mx-auto mb-2 {runnerType === 'scaleset' ? 'text-blue-600 dark:text-blue-400' : 'text-gray-400'}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4"/>
				</svg>
				<p class="text-sm font-medium {runnerType === 'scaleset' ? 'text-blue-600 dark:text-blue-400' : 'text-gray-900 dark:text-white'}">Scale Set</p>
				<p class="text-xs text-gray-500 dark:text-gray-400 mt-1">{forgeType === 'gitea' ? 'GitHub only' : 'GitHub Actions Runner Scale Sets'}</p>
			</button>
		</div>
	</div>

	{#if runnerType}
		<form on:submit|preventDefault={handleCreate} class="space-y-6">
			<!-- Scale Set Name (scale set only) -->
			{#if runnerType === 'scaleset'}
				<div>
					<label for="runner-name" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						Scale Set Name <span class="text-red-500">*</span>
					</label>
					<input
						type="text"
						id="runner-name"
						bind:value={scaleSetName}
						required
						autocomplete="off"
						class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
						placeholder="e.g., my-scale-set"
					/>
				</div>
			{/if}

			<!-- Provider & Image Configuration -->
			<div class="space-y-4">
				<h3 class="text-lg font-medium text-gray-900 dark:text-white border-b border-gray-200 dark:border-gray-700 pb-2">
					Provider & Image
				</h3>

				<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
					<div>
						<label for="runner-provider" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
							Provider <span class="text-red-500">*</span>
						</label>
						{#if loadingProviders}
							<div class="animate-pulse bg-gray-200 dark:bg-gray-700 h-10 rounded"></div>
						{:else}
							<select
								id="runner-provider"
								bind:value={selectedProvider}
								required
								class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
							>
								<option value="">Select a provider</option>
								{#each providers as provider}
									<option value={provider.name}>{provider.name} ({provider.description || provider.type})</option>
								{/each}
							</select>
						{/if}
					</div>

					<div>
						<label for="runner-template" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
							Runner Install Template
						</label>
						{#if loadingTemplates}
							<div class="animate-pulse bg-gray-200 dark:bg-gray-700 h-10 rounded"></div>
						{:else}
							<select
								id="runner-template"
								bind:value={selectedTemplate}
								class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
							>
								<option value={undefined}>No template</option>
								{#each templates as template}
									<option value={template.id}>{template.name}{template.owner_id === 'system' ? ' (System)' : ''}</option>
								{/each}
							</select>
						{/if}
					</div>
				</div>

				<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
					<div>
						<label for="runner-image" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
							Image <span class="text-red-500">*</span>
						</label>
						<input
							type="text"
							id="runner-image"
							bind:value={image}
							required
							autocomplete="off"
							class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
							placeholder="e.g., ubuntu:22.04"
						/>
					</div>

					<div>
						<label for="runner-flavor" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
							Flavor <span class="text-red-500">*</span>
						</label>
						<input
							type="text"
							id="runner-flavor"
							bind:value={flavor}
							required
							autocomplete="off"
							class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
							placeholder="e.g., standard or m1.large"
						/>
					</div>
				</div>

				<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
					<div>
						<label for="runner-os-type" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
							OS Type
						</label>
						<select
							id="runner-os-type"
							bind:value={osType}
							class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
						>
							<option value="linux">Linux</option>
							<option value="windows">Windows</option>
						</select>
					</div>

					<div>
						<label for="runner-os-arch" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
							Architecture
						</label>
						<select
							id="runner-os-arch"
							bind:value={osArch}
							class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
						>
							<option value="amd64">amd64</option>
							<option value="arm64">arm64</option>
							<option value="arm">arm</option>
						</select>
					</div>
				</div>
			</div>

			<!-- Runner Scaling -->
			<div class="space-y-4">
				<h3 class="text-lg font-medium text-gray-900 dark:text-white border-b border-gray-200 dark:border-gray-700 pb-2">
					Scaling Configuration
				</h3>

				<div class="grid grid-cols-1 md:grid-cols-3 gap-4">
					<div>
						<label for="runner-min-idle" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
							Min Idle Runners
						</label>
						<input
							type="number"
							id="runner-min-idle"
							bind:value={minIdleRunners}
							min="0"
							class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
							placeholder="0"
						/>
					</div>

					<div>
						<label for="runner-max" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
							Max Runners
						</label>
						<input
							type="number"
							id="runner-max"
							bind:value={maxRunners}
							min="1"
							class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
							placeholder="10"
						/>
					</div>

					<div>
						<label for="runner-timeout" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
							Bootstrap Timeout (min)
						</label>
						<input
							type="number"
							id="runner-timeout"
							bind:value={runnerBootstrapTimeout}
							min="1"
							class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
							placeholder="20"
						/>
					</div>
				</div>
			</div>

			<!-- Advanced Settings -->
			<div class="space-y-4">
				<h3 class="text-lg font-medium text-gray-900 dark:text-white border-b border-gray-200 dark:border-gray-700 pb-2">
					Advanced Settings
				</h3>

				<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
					<div>
						<label for="runner-prefix" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
							Runner Prefix
						</label>
						<input
							type="text"
							id="runner-prefix"
							bind:value={runnerPrefix}
							autocomplete="off"
							class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
							placeholder="garm"
						/>
					</div>

					{#if runnerType === 'pool'}
						<div>
							<label for="runner-priority" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
								Priority
							</label>
							<input
								type="number"
								id="runner-priority"
								bind:value={priority}
								min="0"
								class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
								placeholder="100"
							/>
						</div>
					{/if}

					<div>
						<label for="runner-group" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
							GitHub Runner Group
						</label>
						<input
							type="text"
							id="runner-group"
							bind:value={githubRunnerGroup}
							autocomplete="off"
							class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
							placeholder="Optional"
						/>
					</div>
				</div>

				<!-- Checkboxes -->
				<div class="flex flex-wrap gap-6">
					<div class="flex items-center">
						<input
							id="runner-enabled"
							type="checkbox"
							bind:checked={enabled}
							class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 dark:border-gray-600 rounded"
						/>
						<label for="runner-enabled" class="ml-2 text-sm font-medium text-gray-700 dark:text-gray-300">
							Enabled
						</label>
					</div>

					<div class="flex items-center">
						<input
							id="runner-shell"
							type="checkbox"
							bind:checked={enableShell}
							class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 dark:border-gray-600 rounded"
						/>
						<label for="runner-shell" class="ml-2 text-sm font-medium text-gray-700 dark:text-gray-300">
							Enable Shell
						</label>
					</div>
				</div>

				<!-- Tags (pool only) -->
				{#if runnerType === 'pool'}
					<div>
						<label for="runner-new-tag" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
							Tags
						</label>
						<div class="flex">
							<input
								type="text"
								id="runner-new-tag"
								bind:value={newTag}
								on:keydown={handleTagKeydown}
								autocomplete="off"
								class="flex-1 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-l-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white"
								placeholder="Add a tag"
							/>
							<button
								type="button"
								on:click={addTag}
								class="px-3 py-2 bg-blue-600 text-white rounded-r-md hover:bg-blue-700 cursor-pointer text-sm"
							>
								Add
							</button>
						</div>
						{#if tags.length > 0}
							<div class="flex flex-wrap gap-2 mt-2">
								{#each tags as tag, index}
									<span class="inline-flex items-center px-2 py-1 rounded-full text-xs font-medium bg-blue-100 dark:bg-blue-900 text-blue-800 dark:text-blue-200">
										{tag}
										<button type="button" on:click={() => removeTag(index)} class="ml-1 text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-200 cursor-pointer" aria-label="Remove tag {tag}">
											<svg class="w-3 h-3" fill="currentColor" viewBox="0 0 20 20">
												<path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd"/>
											</svg>
										</button>
									</span>
								{/each}
							</div>
						{/if}
					</div>
				{/if}

				<!-- Extra Specs -->
				<div>
					<label for="runner-extra-specs" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						Extra Specs (JSON)
					</label>
					<JsonEditor bind:value={extraSpecs} />
				</div>
			</div>

			<div class="flex justify-between pt-4">
				<button type="button" on:click={() => dispatch('back')}
					class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-600 hover:bg-gray-200 dark:hover:bg-gray-500 rounded-md cursor-pointer">
					Back
				</button>
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
						Create {runnerType === 'scaleset' ? 'Scale Set' : 'Pool'} & Finish
					{/if}
				</button>
			</div>
		</form>
	{:else}
		<div class="flex justify-between pt-4">
			<button type="button" on:click={() => dispatch('back')}
				class="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-600 hover:bg-gray-200 dark:hover:bg-gray-500 rounded-md cursor-pointer">
				Back
			</button>
		</div>
	{/if}
</div>
