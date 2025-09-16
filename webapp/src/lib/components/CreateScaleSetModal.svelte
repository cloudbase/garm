<script lang="ts">
	import { createEventDispatcher, onMount, onDestroy } from 'svelte';
	import { garmApi } from '$lib/api/client.js';
	import { resolve } from '$app/paths';
	import type {
		CreateScaleSetParams,
		Repository,
		Organization,
		Enterprise,
		Provider,
		Template,
		UpdateEntityParams
	} from '$lib/api/generated/api.js';
	import Modal from './Modal.svelte';
	import JsonEditor from './JsonEditor.svelte';
	import { extractAPIError } from '$lib/utils/apiError';
	import { websocketStore, type WebSocketEvent } from '$lib/stores/websocket.js';
	import { eagerCache } from '$lib/stores/eager-cache.js';
	import UpdateRepositoryModal from './UpdateRepositoryModal.svelte';
	import UpdateOrganizationModal from './UpdateOrganizationModal.svelte';
	import UpdateEnterpriseModal from './UpdateEnterpriseModal.svelte';

	const dispatch = createEventDispatcher<{
		close: void;
		submit: any;
	}>();

	let loading = false;
	let error = '';
	let entityLevel = '';
	let entities: (Repository | Organization | Enterprise)[] = [];
	let providers: Provider[] = [];
	let templates: Template[] = [];
	let loadingEntities = false;
	let loadingProviders = false;
	let loadingTemplates = false;
	let showEntityUpdateModal = false;
	let unsubscribeWebsocket: (() => void) | null = null;

	// Form fields
	let name = '';
	let selectedEntityId = '';
	let selectedProvider = '';
	let image = '';
	let flavor = '';
	let maxRunners: number | undefined = undefined;
	let minIdleRunners: number | undefined = undefined;
	let runnerBootstrapTimeout: number | undefined = undefined;
	let runnerPrefix = 'garm';
	let osType = 'linux';
	let osArch = 'amd64';
	let githubRunnerGroup = '';
	let enabled = true;
	let enableShell = false;
	let extraSpecs = '{}';
	let selectedTemplate: number | undefined = undefined;

	// Reactive validation
	$: isFormValid = !loading && 
		name.trim() !== '' && 
		entityLevel !== '' && 
		selectedEntityId !== '' && 
		selectedProvider !== '' && 
		image.trim() !== '' && 
		flavor.trim() !== '';

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
			
			// Get forge type from selected entity
			const forgeType = getSelectedEntityForgeType();
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

	function getSelectedEntityForgeType(): string | null {
		if (!selectedEntityId || !entities) return null;

		const selectedEntity = entities.find(e => e.id === selectedEntityId);
		if (!selectedEntity) return null;

		// All entities should have a forge_type or endpoint property that indicates the forge
		// Check the entity structure to determine forge type
		if ('forge_type' in selectedEntity) {
			return selectedEntity.forge_type as string;
		}
		if ('endpoint' in selectedEntity) {
			// Try to determine from endpoint
			const endpoint = selectedEntity.endpoint;
			if (endpoint && 'endpoint_type' in endpoint) {
				return (endpoint.endpoint_type as string) || null;
			}
		}

		// Default to github for now
		return 'github';
	}

	function getSelectedEntityAgentMode(): boolean {
		if (!selectedEntityId || !entities) return false;

		const selectedEntity = entities.find(e => e.id === selectedEntityId);
		if (!selectedEntity) return false;

		// Check if entity has agent_mode property
		if ('agent_mode' in selectedEntity) {
			return selectedEntity.agent_mode as boolean ?? false;
		}

		return false;
	}

	// Reactive statement to check agent mode
	$: entityAgentMode = getSelectedEntityAgentMode();
	$: if (!entityAgentMode) {
		// Disable shell if agent mode is not enabled on entity
		enableShell = false;
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

	function selectEntityLevel(level: string) {
		if (entityLevel === level) return;
		entityLevel = level;
		selectedEntityId = '';
		selectedTemplate = undefined;
		loadEntities();
	}

	function handleEntityWebSocketEvent(event: WebSocketEvent) {
		if (event.operation !== 'update') return;

		const updatedEntity = event.payload;

		// Update the eager cache for the entity that was updated
		if (entityLevel === 'repository' && updatedEntity.id === selectedEntityId) {
			const repo = $eagerCache.repositories.find(r => r.id === selectedEntityId);
			if (repo) {
				Object.assign(repo, updatedEntity);
				// Force reactive update by reassigning entityAgentMode
				entityAgentMode = getSelectedEntityAgentMode();
			}
		} else if (entityLevel === 'organization' && updatedEntity.id === selectedEntityId) {
			const org = $eagerCache.organizations.find(o => o.id === selectedEntityId);
			if (org) {
				Object.assign(org, updatedEntity);
				// Force reactive update by reassigning entityAgentMode
				entityAgentMode = getSelectedEntityAgentMode();
			}
		} else if (entityLevel === 'enterprise' && updatedEntity.id === selectedEntityId) {
			const enterprise = $eagerCache.enterprises.find(e => e.id === selectedEntityId);
			if (enterprise) {
				Object.assign(enterprise, updatedEntity);
				// Force reactive update by reassigning entityAgentMode
				entityAgentMode = getSelectedEntityAgentMode();
			}
		}
	}

	async function handleEntityUpdate(params: UpdateEntityParams) {
		if (!selectedEntityId || !entityLevel) return;

		try {
			switch (entityLevel) {
				case 'repository':
					await garmApi.updateRepository(selectedEntityId, params);
					break;
				case 'organization':
					await garmApi.updateOrganization(selectedEntityId, params);
					break;
				case 'enterprise':
					await garmApi.updateEnterprise(selectedEntityId, params);
					break;
			}

			await loadEntities();
			showEntityUpdateModal = false;
		} catch (err) {
			throw err;
		}
	}

	function getSelectedEntity(): Repository | Organization | Enterprise | null {
		if (!selectedEntityId || !entities) return null;
		return entities.find(e => e.id === selectedEntityId) || null;
	}

	// Reactive statements
	$: if (selectedEntityId && osType) {
		loadTemplates();
	}

	$: if (osType) {
		// Reload templates when OS type changes - selection will be auto-handled in loadTemplates
		if (selectedEntityId) {
			loadTemplates();
		}
	}


	async function handleSubmit() {
		if (!isFormValid) {
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

			const params: CreateScaleSetParams = {
				name,
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
				extra_specs: extraSpecs.trim() ? parsedExtraSpecs : undefined,
				template_id: selectedTemplate
			};

			// Create the scale set using entity-specific method
			let createdScaleSet;
			switch (entityLevel) {
				case 'repository':
					createdScaleSet = await garmApi.createRepositoryScaleSet(selectedEntityId, params);
					break;
				case 'organization':
					createdScaleSet = await garmApi.createOrganizationScaleSet(selectedEntityId, params);
					break;
				case 'enterprise':
					createdScaleSet = await garmApi.createEnterpriseScaleSet(selectedEntityId, params);
					break;
				default:
					throw new Error('Invalid entity level selected');
			}
			dispatch('submit', createdScaleSet);
		} catch (err) {
			error = extractAPIError(err);
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		loadProviders();

		// Subscribe to entity updates via websocket
		if (entityLevel && (entityLevel === 'repository' || entityLevel === 'organization' || entityLevel === 'enterprise')) {
			unsubscribeWebsocket = websocketStore.subscribeToEntity(
				entityLevel as 'repository' | 'organization' | 'enterprise',
				['update'],
				handleEntityWebSocketEvent
			);
		}
	});

	onDestroy(() => {
		if (unsubscribeWebsocket) {
			unsubscribeWebsocket();
			unsubscribeWebsocket = null;
		}
	});

	// Re-subscribe when entity level changes
	$: if (entityLevel && (entityLevel === 'repository' || entityLevel === 'organization' || entityLevel === 'enterprise')) {
		if (unsubscribeWebsocket) {
			unsubscribeWebsocket();
		}
		unsubscribeWebsocket = websocketStore.subscribeToEntity(
			entityLevel as 'repository' | 'organization' | 'enterprise',
			['update'],
			handleEntityWebSocketEvent
		);
	}
</script>

<Modal on:close={() => dispatch('close')}>
	<div class="max-w-6xl w-full max-h-[90vh] overflow-y-auto">
		<div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
			<h2 class="text-xl font-semibold text-gray-900 dark:text-white">Create New Scale Set</h2>
			<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">Scale sets are only available for GitHub endpoints</p>
		</div>

		<form on:submit|preventDefault={handleSubmit} class="p-6 space-y-6">
			{#if error}
				<div class="rounded-md bg-red-50 dark:bg-red-900 p-4">
					<p class="text-sm font-medium text-red-800 dark:text-red-200">{error}</p>
				</div>
			{/if}

			<!-- Scale Set Name -->
			<div>
				<label for="name" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
					Name <span class="text-red-500">*</span>
				</label>
				<input
					id="name"
					type="text"
					bind:value={name}
					required
					placeholder="e.g., my-scale-set"
					class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
				/>
			</div>

			<!-- Entity Level Selection -->
			<div>
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
			</div>

			{#if entityLevel}
				<!-- Group 1: Entity & Provider Configuration -->
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
												{(entity as any).owner}/{entity.name} ({entity.endpoint?.name || 'Unknown endpoint'})
											{:else}
												{entity.name} ({entity.endpoint?.name || 'Unknown endpoint'})
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
					
					<!-- Template Selection -->
					<div>
						<label for="template" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
							Runner Install Template
						</label>
						{#if loadingTemplates}
							<div class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-gray-50 dark:bg-gray-700 flex items-center">
								<div class="animate-spin rounded-full h-4 w-4 border-b-2 border-blue-600 mr-2"></div>
								<span class="text-sm text-gray-600 dark:text-gray-400">Loading templates...</span>
							</div>
						{:else if templates.length > 0}
							<select
								id="template"
								bind:value={selectedTemplate}
								class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
							>
								{#each templates as template}
									<option value={template.id}>
										{template.name} {template.owner_id === 'system' ? '(System)' : ''}
										{#if template.description} - {template.description}{/if}
									</option>
								{/each}
							</select>
							<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
								Templates define how the runner software is installed and configured.
								{#if selectedEntityId}
									Showing templates for {getSelectedEntityForgeType()} {osType}.
								{/if}
							</p>
						{:else if selectedEntityId}
							<div class="w-full px-3 py-2 border border-yellow-300 dark:border-yellow-600 rounded-md bg-yellow-50 dark:bg-yellow-900/20 text-yellow-800 dark:text-yellow-200">
								<p class="text-sm">No templates found for {getSelectedEntityForgeType()} {osType}.</p>
								<p class="text-xs mt-1">
									<a href={resolve('/templates')} class="text-yellow-700 dark:text-yellow-300 hover:underline">
										Create a template first
									</a> or proceed without a template to use default behavior.
								</p>
							</div>
						{:else}
							<div class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-gray-50 dark:bg-gray-700 text-gray-500 dark:text-gray-400">
								<p class="text-sm">Select an entity first to see available templates</p>
							</div>
						{/if}
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
						<div class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
							Extra Specs (JSON)
						</div>
						<JsonEditor
							bind:value={extraSpecs}
							rows={4}
							placeholder="{'{}'}"
						/>
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
							Enable scale set immediately
						</label>
					</div>

					<!-- Enable Shell Checkbox -->
					<div class="space-y-2">
						<div class="flex items-center">
							<input
								id="enableShell"
								type="checkbox"
								bind:checked={enableShell}
								disabled={!entityAgentMode}
								class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 dark:border-gray-600 rounded disabled:opacity-50 disabled:cursor-not-allowed"
							/>
							<label for="enableShell" class="ml-2 block text-sm font-medium text-gray-700 dark:text-gray-300 {!entityAgentMode ? 'opacity-50' : ''}">
								Enable Shell
							</label>
							<div class="ml-2 relative group">
								<svg class="w-4 h-4 text-gray-400 cursor-help" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
								</svg>
								<div class="absolute left-full top-1/2 transform -translate-y-1/2 ml-2 w-80 p-3 bg-gray-900 text-white text-xs rounded-lg shadow-lg opacity-0 invisible group-hover:opacity-100 group-hover:visible transition-all duration-200 z-50">
									This enables remote shell in the GARM agent, allowing users to connect via garm-cli or web UI using websockets.
									<div class="absolute right-full top-1/2 transform -translate-y-1/2 border-4 border-transparent border-r-gray-900"></div>
								</div>
							</div>
						</div>
						{#if !entityAgentMode}
							<div class="ml-6 flex items-start space-x-2 text-xs text-yellow-700 dark:text-yellow-400">
								<svg class="w-4 h-4 mt-0.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
								</svg>
								<span>
									Shell access requires agent mode to be enabled on the {entityLevel}.
									{#if selectedEntityId}
										<button
											type="button"
											on:click={() => showEntityUpdateModal = true}
											class="underline hover:text-yellow-800 dark:hover:text-yellow-300 cursor-pointer"
										>
											Enable agent mode
										</button>
									{/if}
								</span>
							</div>
						{/if}
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
					disabled={!isFormValid}
					class="px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
				>
					{#if loading}
						<div class="flex items-center">
							<div class="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
							Creating...
						</div>
					{:else}
						Create Scale Set
					{/if}
				</button>
			</div>
		</form>
	</div>
</Modal>

<!-- Nested Entity Update Modals -->
{#if showEntityUpdateModal && selectedEntityId}
	{@const selectedEntity = getSelectedEntity()}
	{#if selectedEntity}
		{#if entityLevel === 'repository'}
			<UpdateRepositoryModal
				repository={selectedEntity}
				on:close={() => showEntityUpdateModal = false}
				on:submit={(e) => handleEntityUpdate(e.detail)}
			/>
		{:else if entityLevel === 'organization'}
			<UpdateOrganizationModal
				organization={selectedEntity}
				on:close={() => showEntityUpdateModal = false}
				on:submit={(e) => handleEntityUpdate(e.detail)}
			/>
		{:else if entityLevel === 'enterprise'}
			<UpdateEnterpriseModal
				enterprise={selectedEntity}
				on:close={() => showEntityUpdateModal = false}
				on:submit={(e) => handleEntityUpdate(e.detail)}
			/>
		{/if}
	{/if}
{/if}