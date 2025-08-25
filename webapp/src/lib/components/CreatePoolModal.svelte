<script lang="ts">
	import { createEventDispatcher, onMount } from 'svelte';
	import { garmApi } from '$lib/api/client.js';
	import type { 
		CreatePoolParams,
		Repository,
		Organization,
		Enterprise,
		Provider
	} from '$lib/api/generated/api.js';
	import Modal from './Modal.svelte';
	import { extractAPIError } from '$lib/utils/apiError';
	import JsonEditor from './JsonEditor.svelte';

	const dispatch = createEventDispatcher<{
		close: void;
		submit: CreatePoolParams;
	}>();

	// Export props for pre-populating the modal
	export let initialEntityType: 'repository' | 'organization' | 'enterprise' | '' = '';
	export let initialEntityId: string = '';

	let loading = false;
	let error = '';
	let entityLevel = initialEntityType;
	let entities: (Repository | Organization | Enterprise)[] = [];
	let providers: Provider[] = [];
	let loadingEntities = false;
	let loadingProviders = false;

	// Form fields
	let selectedEntityId = initialEntityId;
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
	let tags: string[] = [];
	let newTag = '';
	let extraSpecs = '{}';

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

	async function loadEntities() {
		if (!entityLevel) return;
		
		try {
			loadingEntities = true;
			entities = [];
			
			switch (entityLevel) {
				case 'repository':
					entities = await garmApi.listRepositories();
					break;
				case 'organization':
					entities = await garmApi.listOrganizations();
					break;
				case 'enterprise':
					entities = await garmApi.listEnterprises();
					break;
			}
		} catch (err) {
			error = extractAPIError(err);
		} finally {
			loadingEntities = false;
		}
	}

	function selectEntityLevel(level: 'repository' | 'organization' | 'enterprise') {
		if (entityLevel === level) return;
		entityLevel = level;
		selectedEntityId = '';
		loadEntities();
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

	async function handleSubmit() {
		if (!entityLevel || !selectedEntityId || !selectedProvider || !image || !flavor) {
			error = 'Please fill in all required fields';
			return;
		}

		try {
			loading = true;
			error = '';

			// Validate extra specs JSON
			let parsedExtraSpecs: any = {};
			if (extraSpecs.trim()) {
				try {
					parsedExtraSpecs = JSON.parse(extraSpecs);
				} catch (e) {
					throw new Error('Invalid JSON in extra specs');
				}
			}

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
				tags,
				extra_specs: extraSpecs.trim() ? parsedExtraSpecs : undefined
			};

			// If we have an initial entity, let the parent handle the API call to avoid duplicates
			// If no initial entity, we handle it ourselves (global pools page scenario)
			if (initialEntityType && initialEntityId) {
				// Entity pages: parent handles the API call
				dispatch('submit', params);
			} else {
				// Global pools page: modal handles the API call
				switch (entityLevel) {
					case 'repository':
						await garmApi.createRepositoryPool(selectedEntityId, params);
						break;
					case 'organization':
						await garmApi.createOrganizationPool(selectedEntityId, params);
						break;
					case 'enterprise':
						await garmApi.createEnterprisePool(selectedEntityId, params);
						break;
					default:
						throw new Error('Invalid entity level');
				}
				dispatch('submit', params);
			}
		} catch (err) {
			error = extractAPIError(err);
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		loadProviders();
		// If we have an initial entity type, load the entities for that type
		if (initialEntityType) {
			loadEntities();
		}
	});
</script>

<Modal on:close={() => dispatch('close')}>
	<div class="max-w-6xl w-full max-h-[90vh] overflow-y-auto">
		<div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
			<h2 class="text-xl font-semibold text-gray-900 dark:text-white">Create New Pool</h2>
		</div>

		<form on:submit|preventDefault={handleSubmit} class="p-6 space-y-6">
			{#if error}
				<div class="rounded-md bg-red-50 dark:bg-red-900 p-4">
					<p class="text-sm font-medium text-red-800 dark:text-red-200">{error}</p>
				</div>
			{/if}

			<!-- Entity Level Selection -->
			<fieldset>
				<legend class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">
					Entity Level <span class="text-red-500">*</span>
				</legend>
				<div class="grid grid-cols-3 gap-4">
					<button
						type="button"
						on:click={() => selectEntityLevel('repository')}
						class="flex flex-col items-center justify-center p-4 border-2 rounded-lg transition-colors cursor-pointer {entityLevel === 'repository' ? 'border-blue-500 bg-blue-50 dark:bg-blue-900' : 'border-gray-300 dark:border-gray-600 hover:border-gray-400 dark:hover:border-gray-500'}"
					>
						<svg class="w-8 h-8 mb-2 text-gray-600 dark:text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2H5a2 2 0 00-2-2z"/>
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 5a2 2 0 012-2h4a2 2 0 012 2v2H8V5z"/>
						</svg>
						<span class="text-sm font-medium text-gray-900 dark:text-white">Repository</span>
					</button>
					<button
						type="button"
						on:click={() => selectEntityLevel('organization')}
						class="flex flex-col items-center justify-center p-4 border-2 rounded-lg transition-colors cursor-pointer {entityLevel === 'organization' ? 'border-blue-500 bg-blue-50 dark:bg-blue-900' : 'border-gray-300 dark:border-gray-600 hover:border-gray-400 dark:hover:border-gray-500'}"
					>
						<svg class="w-8 h-8 mb-2 text-gray-600 dark:text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"/>
						</svg>
						<span class="text-sm font-medium text-gray-900 dark:text-white">Organization</span>
					</button>
					<button
						type="button"
						on:click={() => selectEntityLevel('enterprise')}
						class="flex flex-col items-center justify-center p-4 border-2 rounded-lg transition-colors cursor-pointer {entityLevel === 'enterprise' ? 'border-blue-500 bg-blue-50 dark:bg-blue-900' : 'border-gray-300 dark:border-gray-600 hover:border-gray-400 dark:hover:border-gray-500'}"
					>
						<svg class="w-8 h-8 mb-2 text-gray-600 dark:text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"/>
						</svg>
						<span class="text-sm font-medium text-gray-900 dark:text-white">Enterprise</span>
					</button>
				</div>
			</fieldset>

			{#if entityLevel}
				<!-- Group 1: Entity & Provider Selection -->
				<div class="space-y-4">
					<h3 class="text-lg font-medium text-gray-900 dark:text-white border-b border-gray-200 dark:border-gray-700 pb-2">
						Entity & Provider Configuration
					</h3>
					<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
						<div>
							<label for="entity" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
								{entityLevel.charAt(0).toUpperCase() + entityLevel.slice(1)} <span class="text-red-500">*</span>
							</label>
							{#if loadingEntities}
								<div class="animate-pulse bg-gray-200 dark:bg-gray-700 h-10 rounded"></div>
							{:else}
								<select 
									id="entity"
									bind:value={selectedEntityId}
									required
									class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
								>
									<option value="">Select a {entityLevel}</option>
									{#each entities as entity}
										<option value={entity.id}>
											{#if entityLevel === 'repository'}
												{(entity as any).owner}/{entity.name} ({entity.endpoint?.name})
											{:else}
												{entity.name} ({entity.endpoint?.name})
											{/if}
										</option>
									{/each}
								</select>
							{/if}
						</div>
						<div>
							<label for="provider" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
								Provider <span class="text-red-500">*</span>
							</label>
							{#if loadingProviders}
								<div class="animate-pulse bg-gray-200 dark:bg-gray-700 h-10 rounded"></div>
							{:else}
								<select 
									id="provider"
									bind:value={selectedProvider}
									required
									class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
								>
									<option value="">Select a provider</option>
									{#each providers as provider}
										<option value={provider.name}>{provider.name}</option>
									{/each}
								</select>
							{/if}
						</div>
					</div>
				</div>

				<!-- Group 2: Image & OS Configuration -->
				<div class="space-y-4">
					<h3 class="text-lg font-medium text-gray-900 dark:text-white border-b border-gray-200 dark:border-gray-700 pb-2">
						Image & OS Configuration
					</h3>
					<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
						<div>
							<label for="image" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
								Image <span class="text-red-500">*</span>
							</label>
							<input
								id="image"
								type="text"
								bind:value={image}
								required
								placeholder="e.g., ubuntu:22.04"
								class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
							/>
						</div>
						<div>
							<label for="flavor" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
								Flavor <span class="text-red-500">*</span>
							</label>
							<input
								id="flavor"
								type="text"
								bind:value={flavor}
								required
								placeholder="e.g., default"
								class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
							/>
						</div>
						<div>
							<label for="osType" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
								OS Type
							</label>
							<select
								id="osType"
								bind:value={osType}
								class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
							>
								<option value="linux">Linux</option>
								<option value="windows">Windows</option>
							</select>
						</div>
						<div>
							<label for="osArch" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
								Architecture
							</label>
							<select
								id="osArch"
								bind:value={osArch}
								class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
							>
								<option value="amd64">AMD64</option>
								<option value="arm64">ARM64</option>
							</select>
						</div>
					</div>
				</div>

				<!-- Group 3: Runner Limits & Timing -->
				<div class="space-y-4">
					<h3 class="text-lg font-medium text-gray-900 dark:text-white border-b border-gray-200 dark:border-gray-700 pb-2">
						Runner Limits & Timing
					</h3>
					<div class="grid grid-cols-1 md:grid-cols-3 gap-4">
						<div>
							<label for="minIdleRunners" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
								Min Idle Runners
							</label>
							<input
								id="minIdleRunners"
								type="number"
								bind:value={minIdleRunners}
								min="0"
								placeholder="0"
								class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
							/>
						</div>
						<div>
							<label for="maxRunners" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
								Max Runners
							</label>
							<input
								id="maxRunners"
								type="number"
								bind:value={maxRunners}
								min="1"
								placeholder="10"
								class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
							/>
						</div>
						<div>
							<label for="bootstrapTimeout" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
								Bootstrap Timeout (min)
							</label>
							<input
								id="bootstrapTimeout"
								type="number"
								bind:value={runnerBootstrapTimeout}
								min="1"
								placeholder="20"
								class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
							/>
						</div>
					</div>
				</div>

				<!-- Group 4: Advanced Settings -->
				<div class="space-y-4">
					<h3 class="text-lg font-medium text-gray-900 dark:text-white border-b border-gray-200 dark:border-gray-700 pb-2">
						Advanced Settings
					</h3>
					<div class="grid grid-cols-1 md:grid-cols-3 gap-4">
						<div>
							<label for="runnerPrefix" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
								Runner Prefix
							</label>
							<input
								id="runnerPrefix"
								type="text"
								bind:value={runnerPrefix}
								placeholder="garm"
								class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
							/>
						</div>
						<div>
							<label for="priority" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
								Priority
							</label>
							<input
								id="priority"
								type="number"
								bind:value={priority}
								min="1"
								placeholder="100"
								class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
							/>
						</div>
						<div>
							<label for="githubRunnerGroup" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
								GitHub Runner Group (optional)
							</label>
							<input
								id="githubRunnerGroup"
								type="text"
								bind:value={githubRunnerGroup}
								placeholder="Default group"
								class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
							/>
						</div>
					</div>

					<!-- Tags -->
					<div>
						<label for="tag-input" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
							Tags
						</label>
						<div class="space-y-2">
							<div class="flex">
								<input
									id="tag-input"
									type="text"
									bind:value={newTag}
									on:keydown={handleTagKeydown}
									placeholder="Enter a tag"
									class="flex-1 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-l-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
								/>
								<button
									type="button"
									on:click={addTag}
									class="px-3 py-2 bg-blue-600 text-white rounded-r-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 cursor-pointer"
								>
									Add
								</button>
							</div>
							{#if tags.length > 0}
								<div class="flex flex-wrap gap-2">
									{#each tags as tag, index}
										<span class="inline-flex items-center px-2 py-1 rounded-full text-xs font-medium bg-blue-100 dark:bg-blue-900 text-blue-800 dark:text-blue-200">
											{tag}
											<button
												type="button"
												on:click={() => removeTag(index)}
												aria-label={`Remove tag ${tag}`}
												class="ml-1 h-4 w-4 rounded-full hover:bg-blue-200 dark:hover:bg-blue-800 flex items-center justify-center cursor-pointer"
											>
												<svg class="h-3 w-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
													<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
												</svg>
											</button>
										</span>
									{/each}
								</div>
							{/if}
						</div>
					</div>

					<!-- Extra Specs -->
					<fieldset>
						<legend class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
							Extra Specs (JSON)
						</legend>
						<JsonEditor
							bind:value={extraSpecs}
							rows={4}
							placeholder="{'{}'}"
						/>
					</fieldset>

					<!-- Enabled Checkbox -->
					<div class="flex items-center">
						<input
							id="enabled"
							type="checkbox"
							bind:checked={enabled}
							class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 dark:border-gray-600 rounded"
						/>
						<label for="enabled" class="ml-2 block text-sm text-gray-700 dark:text-gray-300">
							Enable pool immediately
						</label>
					</div>
				</div>
			{/if}

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
					disabled={loading || !entityLevel || !selectedEntityId || !selectedProvider || !image || !flavor}
					class="px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
				>
					{#if loading}
						<div class="flex items-center">
							<div class="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
							Creating...
						</div>
					{:else}
						Create Pool
					{/if}
				</button>
			</div>
		</form>
	</div>
</Modal>