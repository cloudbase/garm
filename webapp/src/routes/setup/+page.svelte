<script lang="ts">
	import { resolve } from '$app/paths';
	import SetupWizardStepper from '$lib/components/setup/SetupWizardStepper.svelte';
	import EndpointStep from '$lib/components/setup/EndpointStep.svelte';
	import CredentialsStep from '$lib/components/setup/CredentialsStep.svelte';
	import EntityStep from '$lib/components/setup/EntityStep.svelte';
	import RunnerStep from '$lib/components/setup/RunnerStep.svelte';

	let currentStep = 1;
	let completed = false;

	let wizardState = {
		endpointName: '',
		forgeType: '' as 'github' | 'gitea' | '',
		credentialsName: '',
		entityType: '' as 'repository' | 'organization' | 'enterprise' | '',
		entityId: '',
		entityName: '',
		runnerType: '' as 'pool' | 'scaleset' | '',
		runnerId: ''
	};

	const steps = [
		{ number: 1, title: 'Endpoint', description: 'Select forge endpoint' },
		{ number: 2, title: 'Credentials', description: 'Configure authentication' },
		{ number: 3, title: 'Entity', description: 'Add repo, org, or enterprise' },
		{ number: 4, title: 'Runner', description: 'Create pool or scale set' }
	];

	function handleEndpointComplete(event: CustomEvent<{ endpointName: string; forgeType: 'github' | 'gitea' }>) {
		wizardState.endpointName = event.detail.endpointName;
		wizardState.forgeType = event.detail.forgeType;
		currentStep = 2;
	}

	function handleCredentialsComplete(event: CustomEvent<{ credentialsName: string }>) {
		wizardState.credentialsName = event.detail.credentialsName;
		currentStep = 3;
	}

	function handleEntityComplete(event: CustomEvent<{ entityType: 'repository' | 'organization' | 'enterprise'; entityId: string; entityName: string }>) {
		wizardState.entityType = event.detail.entityType;
		wizardState.entityId = event.detail.entityId;
		wizardState.entityName = event.detail.entityName;
		currentStep = 4;
	}

	function handleRunnerComplete(event: CustomEvent<{ runnerType: 'pool' | 'scaleset'; runnerId: string }>) {
		wizardState.runnerType = event.detail.runnerType;
		wizardState.runnerId = event.detail.runnerId;
		completed = true;
	}

	function handleBack() {
		currentStep = Math.max(1, currentStep - 1);
	}

	function resetWizard() {
		currentStep = 1;
		completed = false;
		wizardState = {
			endpointName: '',
			forgeType: '',
			credentialsName: '',
			entityType: '',
			entityId: '',
			entityName: '',
			runnerType: '',
			runnerId: ''
		};
	}

	function getEntityDetailUrl(): string {
		if (wizardState.entityType === 'repository') {
			return resolve(`/repositories/${wizardState.entityId}`);
		} else if (wizardState.entityType === 'organization') {
			return resolve(`/organizations/${wizardState.entityId}`);
		} else {
			return resolve(`/enterprises/${wizardState.entityId}`);
		}
	}

	function getRunnerDetailUrl(): string {
		if (wizardState.runnerType === 'pool') {
			return resolve(`/pools/${wizardState.runnerId}`);
		} else {
			return resolve(`/scalesets/${wizardState.runnerId}`);
		}
	}
</script>

<svelte:head>
	<title>Setup - GARM</title>
</svelte:head>

<div class="space-y-8 max-w-4xl mx-auto">
	<!-- Breadcrumb -->
	<nav class="flex" aria-label="Breadcrumb">
		<ol class="inline-flex items-center space-x-1 md:space-x-3">
			<li class="inline-flex items-center">
				<a href={resolve('/')} class="inline-flex items-center text-sm font-medium text-gray-700 hover:text-blue-600 dark:text-gray-400 dark:hover:text-white">
					<svg class="w-3 h-3 mr-2.5" fill="currentColor" viewBox="0 0 20 20">
						<path d="M10.707 2.293a1 1 0 00-1.414 0l-7 7a1 1 0 001.414 1.414L4 10.414V17a1 1 0 001 1h2a1 1 0 001-1v-2a1 1 0 011-1h2a1 1 0 011 1v2a1 1 0 001 1h2a1 1 0 001-1v-6.586l.293.293a1 1 0 001.414-1.414l-7-7z"/>
					</svg>
					Dashboard
				</a>
			</li>
			<li>
				<div class="flex items-center">
					<svg class="w-3 h-3 text-gray-400 mx-1" fill="currentColor" viewBox="0 0 20 20">
						<path fill-rule="evenodd" d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z" clip-rule="evenodd"/>
					</svg>
					<span class="ml-1 text-sm font-medium text-gray-500 md:ml-2 dark:text-gray-400">
						Setup
					</span>
				</div>
			</li>
		</ol>
	</nav>

	{#if !completed}
		<!-- Header -->
		<div>
			<h1 class="text-2xl font-bold text-gray-900 dark:text-white">Set Up Runner Infrastructure</h1>
			<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
				Follow the steps below to configure your GitHub Actions runner infrastructure.
			</p>
		</div>

		<!-- Stepper -->
		<SetupWizardStepper {steps} {currentStep} />

		<!-- Step content -->
		<div class="bg-white dark:bg-gray-800 shadow rounded-lg p-6">
			{#if currentStep === 1}
				<EndpointStep
					endpointName={wizardState.endpointName}
					on:complete={handleEndpointComplete}
				/>
			{:else if currentStep === 2}
				<CredentialsStep
					endpointName={wizardState.endpointName}
					forgeType={wizardState.forgeType}
					credentialsName={wizardState.credentialsName}
					on:complete={handleCredentialsComplete}
					on:back={handleBack}
				/>
			{:else if currentStep === 3}
				<EntityStep
					endpointName={wizardState.endpointName}
					forgeType={wizardState.forgeType}
					credentialsName={wizardState.credentialsName}
					on:complete={handleEntityComplete}
					on:back={handleBack}
				/>
			{:else if currentStep === 4}
				<RunnerStep
					endpointName={wizardState.endpointName}
					forgeType={wizardState.forgeType}
					credentialsName={wizardState.credentialsName}
					entityType={wizardState.entityType}
					entityId={wizardState.entityId}
					entityName={wizardState.entityName}
					on:complete={handleRunnerComplete}
					on:back={handleBack}
				/>
			{/if}
		</div>
	{:else}
		<!-- Completion Screen -->
		<div class="bg-white dark:bg-gray-800 shadow rounded-lg p-8 text-center">
			<div class="mx-auto flex items-center justify-center w-16 h-16 rounded-full bg-green-100 dark:bg-green-900 mb-6">
				<svg class="w-8 h-8 text-green-600 dark:text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path>
				</svg>
			</div>

			<h2 class="text-2xl font-bold text-gray-900 dark:text-white mb-2">Setup Complete!</h2>
			<p class="text-gray-500 dark:text-gray-400 mb-8">
				Your runner infrastructure has been configured successfully.
			</p>

			<!-- Summary -->
			<div class="max-w-md mx-auto bg-gray-50 dark:bg-gray-700 rounded-lg p-6 text-left mb-8">
				<h3 class="text-sm font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider mb-4">Summary</h3>
				<dl class="space-y-3">
					<div class="flex justify-between">
						<dt class="text-sm text-gray-500 dark:text-gray-400">Endpoint</dt>
						<dd class="text-sm font-medium text-gray-900 dark:text-white">{wizardState.endpointName}</dd>
					</div>
					<div class="flex justify-between">
						<dt class="text-sm text-gray-500 dark:text-gray-400">Credentials</dt>
						<dd class="text-sm font-medium text-gray-900 dark:text-white">{wizardState.credentialsName}</dd>
					</div>
					<div class="flex justify-between">
						<dt class="text-sm text-gray-500 dark:text-gray-400 capitalize">{wizardState.entityType}</dt>
						<dd class="text-sm font-medium text-gray-900 dark:text-white">{wizardState.entityName}</dd>
					</div>
					<div class="flex justify-between">
						<dt class="text-sm text-gray-500 dark:text-gray-400">{wizardState.runnerType === 'scaleset' ? 'Scale Set' : 'Pool'}</dt>
						<dd class="text-sm font-medium text-gray-900 dark:text-white">{wizardState.runnerId}</dd>
					</div>
				</dl>
			</div>

			<!-- Actions -->
			<div class="flex flex-col sm:flex-row justify-center gap-3">
				<a
					href={getEntityDetailUrl()}
					class="inline-flex items-center justify-center px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-md text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600"
				>
					View {wizardState.entityType === 'repository' ? 'Repository' : wizardState.entityType === 'organization' ? 'Organization' : 'Enterprise'}
				</a>
				<a
					href={getRunnerDetailUrl()}
					class="inline-flex items-center justify-center px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-md text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600"
				>
					View {wizardState.runnerType === 'scaleset' ? 'Scale Set' : 'Pool'}
				</a>
				<button
					on:click={resetWizard}
					class="inline-flex items-center justify-center px-4 py-2 border border-transparent rounded-md text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 cursor-pointer"
				>
					Set Up Another
				</button>
				<a
					href={resolve('/')}
					class="inline-flex items-center justify-center px-4 py-2 border border-transparent rounded-md text-sm font-medium text-blue-600 dark:text-blue-400 hover:text-blue-500 dark:hover:text-blue-300"
				>
					Back to Dashboard
				</a>
			</div>
		</div>
	{/if}
</div>
