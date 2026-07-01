<script lang="ts">
	import { createEventDispatcher, onMount, onDestroy } from 'svelte';
	import { slide } from 'svelte/transition';
	import { garmApi } from '$lib/api/client.js';
	import { resolve } from '$app/paths';
	import type {
		CreatePoolParams,
		CreateScaleSetParams,
		Repository,
		Organization,
		Enterprise,
		Provider,
		Template,
		UpdateEntityParams
	} from '$lib/api/generated/api.js';
	import WizardModal from './WizardModal.svelte';
	import ForgeTypeSelector from './ForgeTypeSelector.svelte';
	import RunnerConfigFields from './fields/RunnerConfigFields.svelte';
	import RunnerLimitsFields from './fields/RunnerLimitsFields.svelte';
	import RunnerAdvancedFields from './fields/RunnerAdvancedFields.svelte';
	import { extractAPIError } from '$lib/utils/apiError';
	import { getEntityForgeTypeById, getEntityAgentModeById } from '$lib/utils/entity';
	import { websocketStore, type WebSocketEvent } from '$lib/stores/websocket.js';
	import UpdateEntityModal from './UpdateEntityModal.svelte';

	const dispatch = createEventDispatcher<{
		close: void;
		submit: CreatePoolParams | CreateScaleSetParams;
	}>();

	// Export props for pre-populating the modal
	export let initialEntityType: 'repository' | 'organization' | 'enterprise' | 'forge_instance' | '' = '';
	export let initialEntityId: string = '';
	export let poolType: 'pool' | 'scaleset' = 'pool';

	let loading = false;
	let error = '';
	let currentStep = 0;
	let selectedForgeType: 'github' | 'gitea' | '' = '';
	let entityLevel: 'repository' | 'organization' | 'enterprise' | 'forge_instance' | '' = initialEntityType;
	let entities: (Repository | Organization | Enterprise)[] = [];
	let providers: Provider[] = [];
	let templates: Template[] = [];
	let loadingEntities = false;
	let loadingProviders = false;
	let loadingTemplates = false;
	let showEntityUpdateModal = false;
	let unsubscribeWebsocket: (() => void) | null = null;

	// Form fields
	let selectedEntityId = initialEntityId;
	let selectedProvider = '';
	let scaleSetName = '';
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
	let extraSpecs = '{}';
	let selectedTemplate: number | undefined = undefined;

	$: wizardTitle = poolType === 'pool' ? 'Create New Pool' : 'Create New Scale Set';
	$: wizardSubmitLabel = poolType === 'pool' ? 'Create Pool' : 'Create Scale Set';

	const steps = [
		{ title: 'Target', description: 'Where should the pool run?' },
		{ title: 'Runner', description: 'What should the runners look like?' },
		{ title: 'Settings', description: 'Additional configuration' }
	];

	// canAdvance logic per step
	$: canAdvance = currentStep === 0
		? !!(selectedEntityId && selectedProvider && (poolType !== 'scaleset' || scaleSetName.trim()))
		: currentStep === 1
			? !!(image && flavor)
			: true;

	// Reactive statement to check agent mode
	$: entityAgentMode = getEntityAgentModeById(selectedEntityId, entities);
	$: if (!entityAgentMode) {
		enableShell = false;
	}

	function getSelectedEntityForgeType(): string | null {
		return getEntityForgeTypeById(selectedEntityId, entities);
	}

	// Reload templates when entity selection or OS type changes
	$: if (selectedEntityId && osType) {
		loadTemplates();
	}

	async function loadProviders() {
		try {
			loadingProviders = true;
			const allProviders = await garmApi.listProviders();
			providers = allProviders.sort((a, b) => (a.name || '').localeCompare(b.name || ''));
		} catch (err) {
			error = extractAPIError(err);
		} finally {
			loadingProviders = false;
		}
	}

	async function loadTemplates() {
		try {
			loadingTemplates = true;

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
					selectedTemplate = templates[0].id;
				}
			}
		} catch (err) {
			error = extractAPIError(err);
		} finally {
			loadingTemplates = false;
		}
	}

	async function loadEntities() {
		if (!entityLevel) return;

		try {
			loadingEntities = true;
			entities = [];

			let allEntities: any[] = [];
			switch (entityLevel) {
				case 'repository':
					allEntities = await garmApi.listRepositories();
					break;
				case 'organization':
					allEntities = await garmApi.listOrganizations();
					break;
				case 'enterprise':
					allEntities = await garmApi.listEnterprises();
					break;
				case 'forge_instance':
					allEntities = await garmApi.listForgeInstances();
					break;
			}

			// Filter by selected forge type when applicable
			if (selectedForgeType && (entityLevel === 'repository' || entityLevel === 'organization')) {
				entities = allEntities.filter((e: any) => e.endpoint?.endpoint_type === selectedForgeType);
			} else {
				entities = allEntities;
			}
		} catch (err) {
			error = extractAPIError(err);
		} finally {
			loadingEntities = false;
		}
	}

	function selectForgeType(type: 'github' | 'gitea') {
		if (selectedForgeType === type) return;
		selectedForgeType = type;
		selectedEntityId = '';
		selectedTemplate = undefined;

		// Reset entity level if it's incompatible with the new forge type
		if (entityLevel === 'enterprise' && type !== 'github') {
			entityLevel = '';
			entities = [];
		} else if (entityLevel === 'forge_instance' && type !== 'gitea') {
			entityLevel = '';
			entities = [];
		} else if (entityLevel) {
			// Compatible level (repo/org) -- reload with new forge type filter
			loadEntities();
		} else {
			entities = [];
		}
	}

	function selectEntityLevel(level: 'repository' | 'organization' | 'enterprise' | 'forge_instance') {
		if (entityLevel === level) return;
		entityLevel = level;
		selectedEntityId = '';
		selectedTemplate = undefined;
		loadEntities();
	}

	function handleEntityWebSocketEvent(event: WebSocketEvent) {
		if (event.operation !== 'update') return;

		const updatedEntity = event.payload;

		// Update the local entities array so the reactive $: statement picks up the change
		if (updatedEntity.id === selectedEntityId) {
			entities = entities.map(e => e.id === updatedEntity.id ? { ...e, ...updatedEntity } : e);
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
				case 'forge_instance':
					await garmApi.updateForgeInstance(selectedEntityId, params);
					break;
			}

			await loadEntities();
			showEntityUpdateModal = false;
		} catch (err) {
			error = extractAPIError(err);
			showEntityUpdateModal = false;
		}
	}

	function getSelectedEntity(): Repository | Organization | Enterprise | null {
		if (!selectedEntityId || !entities) return null;
		return entities.find(e => e.id === selectedEntityId) || null;
	}

	async function handleSubmit() {
		if (!entityLevel || !selectedEntityId || !selectedProvider || !image || !flavor) {
			error = 'Please fill in all required fields';
			return;
		}

		if (poolType === 'scaleset' && !scaleSetName.trim()) {
			error = 'Please enter a scale set name';
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

			if (poolType === 'scaleset') {
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
					labels: tags,
					extra_specs: extraSpecs.trim() ? parsedExtraSpecs : undefined,
					template_id: selectedTemplate
				};

				if (initialEntityType && initialEntityId) {
					dispatch('submit', params);
				} else {
					switch (entityLevel) {
						case 'repository':
							await garmApi.createRepositoryScaleSet(selectedEntityId, params);
							break;
						case 'organization':
							await garmApi.createOrganizationScaleSet(selectedEntityId, params);
							break;
						case 'enterprise':
							await garmApi.createEnterpriseScaleSet(selectedEntityId, params);
							break;
						default:
							throw new Error('Invalid entity level for scale set');
					}
					dispatch('submit', params);
				}
			} else {
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
					extra_specs: extraSpecs.trim() ? parsedExtraSpecs : undefined,
					template_id: selectedTemplate
				};

				if (initialEntityType && initialEntityId) {
					dispatch('submit', params);
				} else {
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
						case 'forge_instance':
							await garmApi.createForgeInstancePool(selectedEntityId, params);
							break;
						default:
							throw new Error('Invalid entity level');
					}
					dispatch('submit', params);
				}
			}
		} catch (err) {
			error = extractAPIError(err);
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		loadProviders();
		if (initialEntityType) {
			loadEntities();
		}
	});

	onDestroy(() => {
		if (unsubscribeWebsocket) {
			unsubscribeWebsocket();
			unsubscribeWebsocket = null;
		}
	});

	// Re-subscribe when entity level changes
	$: if (entityLevel && (entityLevel === 'repository' || entityLevel === 'organization' || entityLevel === 'enterprise' || entityLevel === 'forge_instance')) {
		if (unsubscribeWebsocket) {
			unsubscribeWebsocket();
		}
		unsubscribeWebsocket = websocketStore.subscribeToEntity(
			entityLevel as 'repository' | 'organization' | 'enterprise' | 'forge_instance',
			['update'],
			handleEntityWebSocketEvent
		);
	}
</script>

<WizardModal
	title={wizardTitle}
	{steps}
	bind:currentStep
	{canAdvance}
	{loading}
	submitLabel={wizardSubmitLabel}
	{error}
	on:close={() => dispatch('close')}
	on:submit={handleSubmit}
>
	<!-- Step 0: Target -->
	<div slot="step-0">
		{#if !initialEntityType}
			<!-- Forge Type Selection -->
			<ForgeTypeSelector
				{selectedForgeType}
				label="Forge Type *"
				on:select={(e) => selectForgeType(e.detail)}
			/>

			<!-- Entity Level Selection (filtered by forge type) -->
			{#if selectedForgeType}
				<fieldset class="mt-6" transition:slide={{ duration: 200 }}>
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
						{#if selectedForgeType === 'github'}
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
						{:else}
							<button
								type="button"
								on:click={() => selectEntityLevel('forge_instance')}
								class="flex flex-col items-center justify-center p-4 border-2 rounded-lg transition-colors cursor-pointer {entityLevel === 'forge_instance' ? 'border-blue-500 bg-blue-50 dark:bg-blue-900' : 'border-gray-300 dark:border-gray-600 hover:border-gray-400 dark:hover:border-gray-500'}"
							>
								<svg class="w-8 h-8 mb-2 text-gray-600 dark:text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01"/>
								</svg>
								<span class="text-sm font-medium text-gray-900 dark:text-white">Forge Instance</span>
							</button>
						{/if}
					</div>
				</fieldset>
			{/if}
		{/if}

		{#if entityLevel}
			<div class="space-y-4" class:mt-6={!initialEntityType} transition:slide={{ duration: 150 }}>
				<h3 class="text-lg font-medium text-gray-900 dark:text-white border-b border-gray-200 dark:border-gray-700 pb-2">
					Entity & Provider Configuration
				</h3>
				<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
					<div>
						<label for="entity" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
							{entityLevel === 'forge_instance' ? 'Forge Instance' : entityLevel.charAt(0).toUpperCase() + entityLevel.slice(1)} <span class="text-red-500">*</span>
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
								<option value="">Select a {entityLevel === 'forge_instance' ? 'forge instance' : entityLevel}</option>
								{#each entities as entity}
									<option value={entity.id}>
										{#if entityLevel === 'repository'}
											{(entity as any).owner}/{entity.name} ({entity.endpoint?.name})
										{:else if entityLevel === 'forge_instance'}
											{entity.endpoint?.name} ({entity.credentials?.name})
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

				{#if poolType === 'scaleset'}
					<div transition:slide={{ duration: 150 }}>
						<label for="scaleSetName" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
							Scale Set Name <span class="text-red-500">*</span>
						</label>
						<input
							id="scaleSetName"
							type="text"
							bind:value={scaleSetName}
							required
							placeholder="e.g., my-scale-set"
							class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
						/>
					</div>
				{/if}
			</div>
		{/if}
	</div>

	<!-- Step 1: Runner -->
	<div slot="step-1">
		<RunnerConfigFields
			bind:image
			bind:flavor
			bind:osType
			bind:osArch
			bind:selectedTemplate
			{templates}
			{loadingTemplates}
			idPrefix="create-pool"
		/>
		<div class="mt-6">
			<RunnerLimitsFields
				bind:minIdleRunners
				bind:maxRunners
				bind:runnerBootstrapTimeout
			/>
		</div>
	</div>

	<!-- Step 2: Settings -->
	<div slot="step-2">
		<RunnerAdvancedFields
			bind:runnerPrefix
			bind:priority
			bind:githubRunnerGroup
			bind:tags
			bind:extraSpecs
			bind:enabled
			bind:enableShell
			{entityAgentMode}
			idPrefix="create-pool"
			on:enableAgentMode={() => { if (selectedEntityId) showEntityUpdateModal = true; }}
		/>
	</div>
</WizardModal>

<!-- Nested Entity Update Modal -->
{#if showEntityUpdateModal && selectedEntityId}
	{@const selectedEntity = getSelectedEntity()}
	{#if selectedEntity && (entityLevel === 'repository' || entityLevel === 'organization' || entityLevel === 'enterprise')}
		<UpdateEntityModal
			entity={selectedEntity}
			entityType={entityLevel}
			on:close={() => showEntityUpdateModal = false}
			on:submit={(e) => handleEntityUpdate(e.detail)}
		/>
	{/if}
{/if}
