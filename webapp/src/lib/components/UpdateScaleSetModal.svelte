<script lang="ts">
	import { createEventDispatcher, onMount } from 'svelte';
	import type { ScaleSet, CreateScaleSetParams } from '$lib/api/generated/api.js';
	import Modal from './Modal.svelte';
	import { extractAPIError } from '$lib/utils/apiError';
	import JsonEditor from './JsonEditor.svelte';

	export let scaleSet: ScaleSet;

	const dispatch = createEventDispatcher<{
		close: void;
		submit: Partial<CreateScaleSetParams>;
	}>();

	let loading = false;
	let error = '';

	// Form fields - initialize with scale set values
	let name = scaleSet.name || '';
	let image = scaleSet.image || '';
	let flavor = scaleSet.flavor || '';
	let maxRunners = scaleSet.max_runners;
	let minIdleRunners = scaleSet.min_idle_runners;
	let runnerBootstrapTimeout = scaleSet.runner_bootstrap_timeout;
	let runnerPrefix = scaleSet.runner_prefix || '';
	let osType = scaleSet.os_type || 'linux';
	let osArch = scaleSet.os_arch || 'amd64';
	let githubRunnerGroup = scaleSet['github-runner-group'] || '';
	let enabled = scaleSet.enabled;
	let extraSpecs = '{}';

	// Initialize extra specs
	onMount(() => {
		if (scaleSet.extra_specs) {
			try {
				// If scale set extra_specs is already an object, stringify it with formatting
				if (typeof scaleSet.extra_specs === 'object') {
					extraSpecs = JSON.stringify(scaleSet.extra_specs, null, 2);
				} else {
					// If it's a string, try to parse and reformat
					const parsed = JSON.parse(scaleSet.extra_specs as string);
					extraSpecs = JSON.stringify(parsed, null, 2);
				}
			} catch (e) {
				// If parsing fails, use as-is or default to empty object
				extraSpecs = (scaleSet.extra_specs as unknown as string) || '{}';
			}
		}
	});


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

			const params: Partial<CreateScaleSetParams> = {
				name: name !== scaleSet.name ? name : undefined,
				image: image !== scaleSet.image ? image : undefined,
				flavor: flavor !== scaleSet.flavor ? flavor : undefined,
				max_runners: maxRunners !== scaleSet.max_runners ? maxRunners : undefined,
				min_idle_runners: minIdleRunners !== scaleSet.min_idle_runners ? minIdleRunners : undefined,
				runner_bootstrap_timeout: runnerBootstrapTimeout !== scaleSet.runner_bootstrap_timeout ? runnerBootstrapTimeout : undefined,
				runner_prefix: runnerPrefix !== scaleSet.runner_prefix ? runnerPrefix : undefined,
				os_type: osType !== scaleSet.os_type ? osType as any : undefined,
				os_arch: osArch !== scaleSet.os_arch ? osArch as any : undefined,
				'github-runner-group': githubRunnerGroup !== scaleSet['github-runner-group'] ? githubRunnerGroup || undefined : undefined,
				enabled: enabled !== scaleSet.enabled ? enabled : undefined,
				extra_specs: extraSpecs.trim() !== JSON.stringify(scaleSet.extra_specs || {}, null, 2).trim() ? parsedExtraSpecs : undefined
			};

			// Remove undefined values
			Object.keys(params).forEach(key => {
				if (params[key as keyof Partial<CreateScaleSetParams>] === undefined) {
					delete params[key as keyof Partial<CreateScaleSetParams>];
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
				Update Scale Set {scaleSet.name}
			</h2>
		</div>

		<form on:submit|preventDefault={handleSubmit} class="p-6 space-y-6">
			{#if error}
				<div class="rounded-md bg-red-50 dark:bg-red-900 p-4">
					<p class="text-sm font-medium text-red-800 dark:text-red-200">{error}</p>
				</div>
			{/if}

			<!-- Scale Set Info (Read-only) -->
			<div class="bg-gray-50 dark:bg-gray-900 p-4 rounded-lg">
				<h3 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Scale Set Information</h3>
				<div class="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">
					<div class="flex">
						<span class="text-gray-500 dark:text-gray-400 w-20 flex-shrink-0">Provider:</span>
						<span class="text-gray-900 dark:text-white">{scaleSet.provider_name}</span>
					</div>
					<div class="flex">
						<span class="text-gray-500 dark:text-gray-400 w-16 flex-shrink-0">Entity:</span>
						<span class="text-gray-900 dark:text-white">
							{#if scaleSet.repo_name}Repository: {scaleSet.repo_name}
							{:else if scaleSet.org_name}Organization: {scaleSet.org_name}
							{:else if scaleSet.enterprise_name}Enterprise: {scaleSet.enterprise_name}
							{:else}Unknown Entity{/if}
						</span>
					</div>
				</div>
			</div>

			<!-- Scale Set Name -->
			<div>
				<label for="name" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
					Name
				</label>
				<input
					id="name"
					type="text"
					bind:value={name}
					placeholder="e.g., my-scale-set"
					class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
				/>
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
				<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
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
						Scale set enabled
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
						Update Scale Set
					{/if}
				</button>
			</div>
		</form>
	</div>
</Modal>