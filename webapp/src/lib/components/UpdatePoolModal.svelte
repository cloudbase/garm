<script lang="ts">
	import { createEventDispatcher, onMount } from 'svelte';
	import type { Pool, ScaleSet, UpdatePoolParams, UpdateScaleSetParams, Template, Repository, Organization, Enterprise, ForgeInstance, UpdateEntityParams } from '$lib/api/generated/api.js';
	import Modal from './Modal.svelte';
	import RunnerConfigFields from './fields/RunnerConfigFields.svelte';
	import RunnerLimitsFields from './fields/RunnerLimitsFields.svelte';
	import RunnerAdvancedFields from './fields/RunnerAdvancedFields.svelte';
	import { extractAPIError } from '$lib/utils/apiError';
	import { eagerCache } from '$lib/stores/eager-cache.js';
	import { garmApi } from '$lib/api/client.js';
	import UpdateEntityModal from './UpdateEntityModal.svelte';

	export let pool: Pool | ScaleSet;
	export let poolType: 'pool' | 'scaleset' = 'pool';

	const dispatch = createEventDispatcher<{
		close: void;
		submit: any;
	}>();

	let loading = false;
	let error = '';
	let validationError = '';
	let templates: Template[] = [];
	let loadingTemplates = false;
	let showEntityUpdateModal = false;

	// Form fields - initialize with pool values
	let name = (pool as ScaleSet).name || '';
	let image = pool.image || '';
	let flavor = pool.flavor || '';
	let maxRunners = pool.max_runners;
	let minIdleRunners = pool.min_idle_runners;
	let runnerBootstrapTimeout = pool.runner_bootstrap_timeout;
	let priority = (pool as Pool).priority;
	let runnerPrefix = pool.runner_prefix || '';
	let osType = pool.os_type || 'linux';
	let osArch = pool.os_arch || 'amd64';
	let githubRunnerGroup = pool['github-runner-group'] || '';
	let enabled = pool.enabled;
	let enableShell = pool.enable_shell ?? false;
	let tags: string[] = poolType === 'pool'
		? ((pool as Pool).tags || []).map(tag => tag.name || '').filter(Boolean)
		: [];
	let extraSpecs = '{}';
	let selectedTemplate: number | undefined = (pool as any).template_id;

	function getEntityName(p: Pool | ScaleSet): string {
		if (poolType === 'scaleset') {
			const ss = p as ScaleSet;
			if (ss.repo_name) return ss.repo_name;
			if (ss.org_name) return ss.org_name;
			if (ss.enterprise_name) return ss.enterprise_name;
			return 'Unknown Entity';
		}
		const pp = p as Pool;
		// Look up friendly names from eager cache
		if (pp.repo_id) {
			const repo = $eagerCache.repositories.find(r => r.id === pp.repo_id);
			return repo ? `${repo.owner}/${repo.name}` : 'Unknown Entity';
		}
		if (pp.org_id) {
			const org = $eagerCache.organizations.find(o => o.id === pp.org_id);
			return (org && org.name) ? org.name : 'Unknown Entity';
		}
		if (pp.enterprise_id) {
			const enterprise = $eagerCache.enterprises.find(e => e.id === pp.enterprise_id);
			return (enterprise && enterprise.name) ? enterprise.name : 'Unknown Entity';
		}
		return 'Unknown Entity';
	}

	function getEntityType(p: Pool | ScaleSet): string {
		if (p.repo_id) return 'Repository';
		if (p.org_id) return 'Organization';
		if (p.enterprise_id) return 'Enterprise';
		if ((p as Pool).forge_instance_id) return 'Forge Instance';
		return 'Unknown';
	}

	function getEntityForgeType(): string | null {
		// First check if the pool itself has an endpoint
		if (pool.endpoint?.endpoint_type) {
			return pool.endpoint.endpoint_type;
		}

		// Fallback: Look up forge type from eager cache based on the pool's entity
		if (pool.repo_id) {
			const repo = $eagerCache.repositories.find(r => r.id === pool.repo_id);
			if (repo?.endpoint?.endpoint_type) {
				return repo.endpoint.endpoint_type;
			}
		}
		if (pool.org_id) {
			const org = $eagerCache.organizations.find(o => o.id === pool.org_id);
			if (org?.endpoint?.endpoint_type) {
				return org.endpoint.endpoint_type;
			}
		}
		if (pool.enterprise_id) {
			const enterprise = $eagerCache.enterprises.find(e => e.id === pool.enterprise_id);
			if (enterprise?.endpoint?.endpoint_type) {
				return enterprise.endpoint.endpoint_type;
			}
		}
		return null;
	}

	function getEntityAgentMode(cache: typeof $eagerCache): boolean {
		// Look up agent_mode from eager cache based on the pool's entity
		if (pool.repo_id) {
			const repo = cache.repositories.find(r => r.id === pool.repo_id);
			return repo?.agent_mode ?? false;
		}
		if (pool.org_id) {
			const org = cache.organizations.find(o => o.id === pool.org_id);
			return org?.agent_mode ?? false;
		}
		if (pool.enterprise_id) {
			const enterprise = cache.enterprises.find(e => e.id === pool.enterprise_id);
			return enterprise?.agent_mode ?? false;
		}
		if ((pool as Pool).forge_instance_id) {
			const fi = cache.forgeInstances.find(f => f.id === (pool as Pool).forge_instance_id);
			return fi?.agent_mode ?? false;
		}
		return false;
	}

	// Reactive statement to check agent mode
	$: entityAgentMode = getEntityAgentMode($eagerCache);
	$: if (!entityAgentMode) {
		// Disable shell if agent mode is not enabled on entity
		enableShell = false;
	}

	async function handleEntityUpdate(params: UpdateEntityParams) {
		try {
			if (pool.repo_id) {
				await garmApi.updateRepository(pool.repo_id, params);
			} else if (pool.org_id) {
				await garmApi.updateOrganization(pool.org_id, params);
			} else if (pool.enterprise_id) {
				await garmApi.updateEnterprise(pool.enterprise_id, params);
			} else if ((pool as Pool).forge_instance_id) {
				await garmApi.updateForgeInstance((pool as Pool).forge_instance_id!, params);
			}

			// Close the entity update modal
			// The eager cache store will be updated via WebSocket event
			showEntityUpdateModal = false;
		} catch (err) {
			error = extractAPIError(err);
			showEntityUpdateModal = false;
		}
	}

	function getEntity(): Repository | Organization | Enterprise | ForgeInstance | null {
		if (pool.repo_id) {
			return $eagerCache.repositories.find(r => r.id === pool.repo_id) || null;
		} else if (pool.org_id) {
			return $eagerCache.organizations.find(o => o.id === pool.org_id) || null;
		} else if (pool.enterprise_id) {
			return $eagerCache.enterprises.find(e => e.id === pool.enterprise_id) || null;
		} else if ((pool as Pool).forge_instance_id) {
			return $eagerCache.forgeInstances.find(f => f.id === (pool as Pool).forge_instance_id) || null;
		}
		return null;
	}

	function getEntityTypeForModal(): 'repository' | 'organization' | 'enterprise' | 'forge_instance' {
		if (pool.repo_id) return 'repository';
		if (pool.org_id) return 'organization';
		if (pool.enterprise_id) return 'enterprise';
		return 'forge_instance';
	}

	async function loadTemplates() {
		try {
			loadingTemplates = true;

			// Get forge type from the pool's entity
			const forgeType = getEntityForgeType();
			if (!forgeType) {
				templates = [];
				return;
			}

			templates = await garmApi.listTemplates(osType, undefined, forgeType);

			// Auto-select system template if no template is currently selected, or if templates change
			if (!selectedTemplate || !templates.find(t => t.id === selectedTemplate)) {
				const systemTemplate = templates.find(t => t.owner_id === 'system');
				if (systemTemplate) {
					selectedTemplate = systemTemplate.id;
				} else if (templates.length > 0) {
					// If no system template, select the first available template
					selectedTemplate = templates[0].id;
				}
			}
		} catch (err) {
			error = extractAPIError(err);
		} finally {
			loadingTemplates = false;
		}
	}

	// Initialize extra specs and load templates
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

		// Load templates for the current configuration
		loadTemplates();
	});

	// Reactive statements
	$: if (osType) {
		// Reload templates when OS type changes - selection will be auto-handled in loadTemplates
		loadTemplates();
	}

	// Validation reactive statement
	$: {
		if (minIdleRunners !== null && minIdleRunners !== undefined && maxRunners !== null && maxRunners !== undefined && minIdleRunners > maxRunners) {
			validationError = 'Min idle runners cannot be greater than max runners';
		} else {
			validationError = '';
		}
	}

	async function handleSubmit() {
		try {
			loading = true;
			error = '';

			// Client-side validation
			if (validationError) {
				throw new Error(validationError);
			}

			// Validate extra specs JSON
			let parsedExtraSpecs: any = {};
			if (extraSpecs.trim()) {
				try {
					parsedExtraSpecs = JSON.parse(extraSpecs);
				} catch (e) {
					throw new Error('Invalid JSON in extra specs');
				}
			}

			if (poolType === 'scaleset') {
				const ss = pool as ScaleSet;
				const params: UpdateScaleSetParams = {
					name: name !== ss.name ? name : undefined,
					image: image !== ss.image ? image : undefined,
					flavor: flavor !== ss.flavor ? flavor : undefined,
					max_runners: maxRunners !== ss.max_runners ? maxRunners : undefined,
					min_idle_runners: minIdleRunners !== ss.min_idle_runners ? minIdleRunners : undefined,
					runner_bootstrap_timeout: runnerBootstrapTimeout !== ss.runner_bootstrap_timeout ? runnerBootstrapTimeout : undefined,
					runner_prefix: runnerPrefix !== ss.runner_prefix ? runnerPrefix : undefined,
					os_type: osType !== ss.os_type ? osType : undefined,
					os_arch: osArch !== ss.os_arch ? osArch : undefined,
					runner_group: githubRunnerGroup !== ss['github-runner-group'] ? githubRunnerGroup || undefined : undefined,
					enabled: enabled !== ss.enabled ? enabled : undefined,
					enable_shell: enableShell !== ss.enable_shell ? enableShell : undefined,
					extra_specs: extraSpecs.trim() !== JSON.stringify(ss.extra_specs || {}, null, 2).trim() ? parsedExtraSpecs : undefined,
					template_id: selectedTemplate !== (ss as any).template_id ? selectedTemplate : undefined
				};

				// Remove undefined values
				Object.keys(params).forEach(key => {
					if (params[key as keyof UpdateScaleSetParams] === undefined) {
						delete params[key as keyof UpdateScaleSetParams];
					}
				});

				dispatch('submit', params);
			} else {
				const pp = pool as Pool;
				const params: UpdatePoolParams = {
					image: image !== pp.image ? image : undefined,
					flavor: flavor !== pp.flavor ? flavor : undefined,
					max_runners: maxRunners !== pp.max_runners ? maxRunners : undefined,
					min_idle_runners: minIdleRunners !== pp.min_idle_runners ? minIdleRunners : undefined,
					runner_bootstrap_timeout: runnerBootstrapTimeout !== pp.runner_bootstrap_timeout ? runnerBootstrapTimeout : undefined,
					priority: priority !== pp.priority ? priority : undefined,
					runner_prefix: runnerPrefix !== pp.runner_prefix ? runnerPrefix : undefined,
					os_type: osType !== pp.os_type ? osType : undefined,
					os_arch: osArch !== pp.os_arch ? osArch : undefined,
					'github-runner-group': githubRunnerGroup !== pp['github-runner-group'] ? githubRunnerGroup || undefined : undefined,
					enabled: enabled !== pp.enabled ? enabled : undefined,
					enable_shell: enableShell !== pp.enable_shell ? enableShell : undefined,
					tags: JSON.stringify(tags) !== JSON.stringify((pp.tags || []).map(tag => tag.name || '').filter(Boolean)) ? tags : undefined,
					extra_specs: extraSpecs.trim() !== JSON.stringify(pp.extra_specs || {}, null, 2).trim() ? parsedExtraSpecs : undefined,
					template_id: selectedTemplate !== (pp as any).template_id ? selectedTemplate : undefined
				};

				// Remove undefined values
				Object.keys(params).forEach(key => {
					if (params[key as keyof UpdatePoolParams] === undefined) {
						delete params[key as keyof UpdatePoolParams];
					}
				});

				dispatch('submit', params);
			}
		} catch (err) {
			error = extractAPIError(err);
		} finally {
			loading = false;
		}
	}

	$: isPool = poolType === 'pool';
	$: modalTitle = isPool ? `Update Pool ${pool.id}` : `Update Scale Set ${(pool as ScaleSet).name}`;
	$: infoTitle = isPool ? 'Pool Information (Read-only)' : 'Scale Set Information';
	$: enabledLabel = isPool ? 'Pool enabled' : 'Scale set enabled';
	$: submitLabel = isPool ? 'Update Pool' : 'Update Scale Set';
</script>

<Modal on:close={() => dispatch('close')}>
	<div class="max-w-6xl w-full max-h-[90vh] overflow-y-auto">
		<div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
			<h2 class="text-xl font-semibold text-gray-900 dark:text-white">
				{modalTitle}
			</h2>
		</div>

		<form on:submit|preventDefault={handleSubmit} class="p-6 space-y-6">
			{#if error}
				<div class="rounded-md bg-red-50 dark:bg-red-900 p-4">
					<p class="text-sm font-medium text-red-800 dark:text-red-200">{error}</p>
				</div>
			{/if}

			{#if validationError}
				<div class="rounded-md bg-yellow-50 dark:bg-yellow-900 p-4">
					<p class="text-sm font-medium text-yellow-800 dark:text-yellow-200">{validationError}</p>
				</div>
			{/if}

			<!-- Pool/Scale Set Info (Read-only) -->
			<div class="bg-gray-50 dark:bg-gray-900 p-4 rounded-lg">
				<h3 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{infoTitle}</h3>
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

			<!-- Scale Set Name (editable, only for scale sets) -->
			{#if poolType === 'scaleset'}
				<div>
					<label for="name" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
						Scale Set Name
					</label>
					<input
						id="name"
						type="text"
						bind:value={name}
						placeholder="e.g., my-scale-set"
						class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
					/>
				</div>
			{/if}

			<!-- Image & OS Configuration -->
			<RunnerConfigFields
				bind:image
				bind:flavor
				bind:osType
				bind:osArch
				bind:selectedTemplate
				{templates}
				{loadingTemplates}
				idPrefix="update-{poolType}"
			/>

			<!-- Runner Limits & Timing -->
			<RunnerLimitsFields
				bind:minIdleRunners
				bind:maxRunners
				bind:runnerBootstrapTimeout
			/>

			<!-- Advanced Settings -->
			<RunnerAdvancedFields
				bind:runnerPrefix
				bind:priority
				bind:githubRunnerGroup
				bind:tags
				bind:extraSpecs
				bind:enabled
				bind:enableShell
				{entityAgentMode}
				{enabledLabel}
				showTags={isPool}
				showPriority={isPool}
				idPrefix="update-{poolType}"
				on:enableAgentMode={() => showEntityUpdateModal = true}
			/>

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
					disabled={loading || validationError !== ''}
					class="px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
				>
					{#if loading}
						<div class="flex items-center">
							<div class="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
							Updating...
						</div>
					{:else}
						{submitLabel}
					{/if}
				</button>
			</div>
		</form>
	</div>
</Modal>

<!-- Nested Entity Update Modals -->
{#if showEntityUpdateModal}
	{@const entity = getEntity()}
	{#if entity}
		<UpdateEntityModal
			{entity}
			entityType={getEntityTypeForModal()}
			on:close={() => showEntityUpdateModal = false}
			on:submit={(e) => handleEntityUpdate(e.detail)}
		/>
	{/if}
{/if}
