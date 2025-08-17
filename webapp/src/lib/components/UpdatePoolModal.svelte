<script lang="ts">
	import { createEventDispatcher, onMount } from 'svelte';
	import type { Pool, UpdatePoolParams } from '$lib/api/generated/api.js';
	import Modal from './Modal.svelte';
	import JsonEditor from './JsonEditor.svelte';
	import { extractAPIError } from '$lib/utils/apiError';
	import { eagerCache } from '$lib/stores/eager-cache.js';

	export let pool: Pool;

	const dispatch = createEventDispatcher<{
		close: void;
		submit: UpdatePoolParams;
	}>();

	let loading = false;
	let error = '';

	// Form fields - initialize with pool values
	let image = pool.image || '';
	let flavor = pool.flavor || '';
	let maxRunners = pool.max_runners;
	let minIdleRunners = pool.min_idle_runners;
	let runnerBootstrapTimeout = pool.runner_bootstrap_timeout;
	let priority = pool.priority;
	let runnerPrefix = pool.runner_prefix || '';
	let osType = pool.os_type || 'linux';
	let osArch = pool.os_arch || 'amd64';
	let githubRunnerGroup = pool['github-runner-group'] || '';
	let enabled = pool.enabled;
	let tags: string[] = (pool.tags || []).map(tag => tag.name || '').filter(Boolean);
	let newTag = '';
	let extraSpecs = '{}';

	function getEntityName(pool: Pool): string {
		// Look up friendly names from eager cache
		if (pool.repo_id) {
			const repo = $eagerCache.repositories.find(r => r.id === pool.repo_id);
			return repo ? `${repo.owner}/${repo.name}` : 'Unknown Entity';
		}
		if (pool.org_id) {
			const org = $eagerCache.organizations.find(o => o.id === pool.org_id);
			return (org && org.name) ? org.name : 'Unknown Entity';
		}
		if (pool.enterprise_id) {
			const enterprise = $eagerCache.enterprises.find(e => e.id === pool.enterprise_id);
			return (enterprise && enterprise.name) ? enterprise.name : 'Unknown Entity';
		}
		return 'Unknown Entity';
	}

	function getEntityType(pool: Pool): string {
		if (pool.repo_id) return 'Repository';
		if (pool.org_id) return 'Organization';
		if (pool.enterprise_id) return 'Enterprise';
		return 'Unknown';
	}

	// Initialize extra specs
	onMount(() => {
		if (pool.extra_specs) {
			try {
				// If pool.extra_specs is already an object, stringify it with formatting
				if (typeof pool.extra_specs === 'object') {
					extraSpecs = JSON.stringify(pool.extra_specs, null, 2);
				} else {
					// If it's a string, try to parse and reformat
					const parsed = JSON.parse(pool.extra_specs as string);
					extraSpecs = JSON.stringify(parsed, null, 2);
				}
			} catch (e) {
				// If parsing fails, use as-is or default to empty object
				extraSpecs = (pool.extra_specs as unknown as string) || '{}';
			}
		}
	});

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

			const params: UpdatePoolParams = {
				image: image !== pool.image ? image : undefined,
				flavor: flavor !== pool.flavor ? flavor : undefined,
				max_runners: maxRunners !== pool.max_runners ? maxRunners : undefined,
				min_idle_runners: minIdleRunners !== pool.min_idle_runners ? minIdleRunners : undefined,
				runner_bootstrap_timeout: runnerBootstrapTimeout !== pool.runner_bootstrap_timeout ? runnerBootstrapTimeout : undefined,
				priority: priority !== pool.priority ? priority : undefined,
				runner_prefix: runnerPrefix !== pool.runner_prefix ? runnerPrefix : undefined,
				os_type: osType !== pool.os_type ? osType : undefined,
				os_arch: osArch !== pool.os_arch ? osArch : undefined,
				'github-runner-group': githubRunnerGroup !== pool['github-runner-group'] ? githubRunnerGroup || undefined : undefined,
				enabled: enabled !== pool.enabled ? enabled : undefined,
				tags: JSON.stringify(tags) !== JSON.stringify((pool.tags || []).map(tag => tag.name || '').filter(Boolean)) ? tags : undefined,
				extra_specs: extraSpecs.trim() !== JSON.stringify(pool.extra_specs || {}, null, 2).trim() ? parsedExtraSpecs : undefined
			};

			// Remove undefined values
			Object.keys(params).forEach(key => {
				if (params[key as keyof UpdatePoolParams] === undefined) {
					delete params[key as keyof UpdatePoolParams];
				}
			});

			dispatch('submit', params);
		} catch (err) {
			error = extractAPIError(err);
		} finally {
			loading = false;
		}
	}
</script>

<Modal on:close={() => dispatch('close')}>
	<div class="max-w-6xl w-full max-h-[90vh] overflow-y-auto">
		<div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
			<h2 class="text-xl font-semibold text-gray-900 dark:text-white">
				Update Pool {pool.id}
			</h2>
		</div>

		<form on:submit|preventDefault={handleSubmit} class="p-6 space-y-6">
			{#if error}
				<div class="rounded-md bg-red-50 dark:bg-red-900 p-4">
					<p class="text-sm font-medium text-red-800 dark:text-red-200">{error}</p>
				</div>
			{/if}

			<!-- Pool Info (Read-only) -->
			<div class="bg-gray-50 dark:bg-gray-900 p-4 rounded-lg">
				<h3 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Pool Information (Read-only)</h3>
				<div class="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">
					<div class="flex">
						<span class="text-gray-500 dark:text-gray-400 w-20 flex-shrink-0">Provider:</span>
						<span class="text-gray-900 dark:text-white">{pool.provider_name}</span>
					</div>
					<div class="flex">
						<span class="text-gray-500 dark:text-gray-400 w-16 flex-shrink-0">Entity:</span>
						<span class="text-gray-900 dark:text-white">
							{getEntityType(pool)}: {getEntityName(pool)}
						</span>
					</div>
				</div>
			</div>

			<!-- Group 1: Image & OS Configuration -->
			<div class="space-y-4">
				<h3 class="text-lg font-medium text-gray-900 dark:text-white border-b border-gray-200 dark:border-gray-700 pb-2">
					Image & OS Configuration
				</h3>
				<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
					<div>
						<label for="image" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
							Image
						</label>
						<input
							id="image"
							type="text"
							bind:value={image}
							placeholder="e.g., ubuntu:22.04"
							class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
						/>
					</div>
					<div>
						<label for="flavor" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
							Flavor
						</label>
						<input
							id="flavor"
							type="text"
							bind:value={flavor}
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

			<!-- Group 2: Runner Limits & Timing -->
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

			<!-- Group 3: Advanced Settings -->
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
					<fieldset>
						<legend class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
							Tags
						</legend>
					<div class="space-y-2">
						<div class="flex">
							<input
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
											class="ml-1 h-4 w-4 rounded-full hover:bg-blue-200 dark:hover:bg-blue-800 flex items-center justify-center cursor-pointer"
											aria-label="Remove tag {tag}"
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
					</fieldset>
				</div>

				<!-- Extra Specs -->
				<div>
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
				</div>

				<!-- Enabled Checkbox -->
				<div class="flex items-center">
					<input
						id="enabled"
						type="checkbox"
						bind:checked={enabled}
						class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 dark:border-gray-600 rounded"
					/>
					<label for="enabled" class="ml-2 block text-sm text-gray-700 dark:text-gray-300">
						Pool enabled
					</label>
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
					disabled={loading}
					class="px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
				>
					{#if loading}
						<div class="flex items-center">
							<div class="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
							Updating...
						</div>
					{:else}
						Update Pool
					{/if}
				</button>
			</div>
		</form>
	</div>
</Modal>