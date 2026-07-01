<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import { slide } from 'svelte/transition';
	import Modal from './Modal.svelte';

	const dispatch = createEventDispatcher<{
		close: void;
		submit: void;
		stepChange: number;
	}>();

	export let title: string;
	export let steps: Array<{ title: string; description: string }>;
	export let currentStep: number = 0;
	export let canAdvance: boolean = false;
	export let loading: boolean = false;
	export let submitLabel: string = 'Create';
	export let error: string = '';

	$: isLastStep = currentStep === steps.length - 1;

	function goBack() {
		if (currentStep > 0) {
			currentStep -= 1;
			dispatch('stepChange', currentStep);
		}
	}

	function goNext() {
		if (!isLastStep && canAdvance) {
			currentStep += 1;
			dispatch('stepChange', currentStep);
		}
	}

	function handleSubmit() {
		if (isLastStep && canAdvance && !loading) {
			dispatch('submit');
		}
	}
</script>

<Modal on:close={() => dispatch('close')}>
	<div class="min-w-[min(40rem,95vw)] max-w-4xl max-h-[90vh] overflow-y-auto transition-all duration-200 ease-in-out">
		<!-- Header -->
		<div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
			<h2 class="text-xl font-semibold text-gray-900 dark:text-white">{title}</h2>
		</div>

		<!-- Stepper -->
		<div class="px-6 pt-6 pb-2">
			<nav aria-label="Wizard progress">
				<ol class="flex items-center w-full">
					{#each steps as step, i}
						<li class="flex items-center {i < steps.length - 1 ? 'flex-1' : ''}">
							<div class="flex flex-col items-center">
								{#if i < currentStep}
									<!-- Completed -->
									<div class="flex items-center justify-center w-10 h-10 rounded-full bg-blue-600 shrink-0">
										<svg class="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
											<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path>
										</svg>
									</div>
								{:else if i === currentStep}
									<!-- Current -->
									<div class="flex items-center justify-center w-10 h-10 rounded-full border-2 border-blue-600 bg-blue-50 dark:bg-blue-900/30 shrink-0">
										<span class="text-sm font-bold text-blue-600 dark:text-blue-400">{i + 1}</span>
									</div>
								{:else}
									<!-- Pending -->
									<div class="flex items-center justify-center w-10 h-10 rounded-full border-2 border-gray-300 dark:border-gray-600 shrink-0">
										<span class="text-sm font-medium text-gray-500 dark:text-gray-400">{i + 1}</span>
									</div>
								{/if}
								<!-- Label -->
								<div class="mt-2 text-center min-w-[80px]">
									<p class="text-sm font-medium {i <= currentStep ? 'text-blue-600 dark:text-blue-400' : 'text-gray-500 dark:text-gray-400'}">
										{step.title}
									</p>
									<p class="hidden md:block text-xs {i <= currentStep ? 'text-gray-600 dark:text-gray-300' : 'text-gray-400 dark:text-gray-500'} mt-0.5">
										{step.description}
									</p>
								</div>
							</div>
							<!-- Connector line -->
							{#if i < steps.length - 1}
								<div class="flex-1 h-0.5 mx-4 mt-[-2rem] {i < currentStep ? 'bg-blue-600' : 'bg-gray-300 dark:bg-gray-600'}"></div>
							{/if}
						</li>
					{/each}
				</ol>
			</nav>
		</div>

		<!-- Error -->
		{#if error}
			<div class="mx-6 mt-4 rounded-md bg-red-50 dark:bg-red-900 p-4">
				<p class="text-sm font-medium text-red-800 dark:text-red-200">{error}</p>
			</div>
		{/if}

		<!-- Content area -->
		<div class="p-6">
			{#if currentStep === 0}
				<div transition:slide={{ duration: 200 }}>
					<slot name="step-0" />
				</div>
			{/if}
			{#if currentStep === 1}
				<div transition:slide={{ duration: 200 }}>
					<slot name="step-1" />
				</div>
			{/if}
			{#if currentStep === 2}
				<div transition:slide={{ duration: 200 }}>
					<slot name="step-2" />
				</div>
			{/if}
			{#if currentStep === 3}
				<div transition:slide={{ duration: 200 }}>
					<slot name="step-3" />
				</div>
			{/if}
			{#if currentStep === 4}
				<div transition:slide={{ duration: 200 }}>
					<slot name="step-4" />
				</div>
			{/if}
		</div>

		<!-- Footer -->
		<div class="px-6 py-4 border-t border-gray-200 dark:border-gray-700 flex justify-between">
			<button
				type="button"
				on:click={() => dispatch('close')}
				class="px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 cursor-pointer"
			>
				Cancel
			</button>
			<div class="flex space-x-3">
				{#if currentStep > 0}
					<button
						type="button"
						on:click={goBack}
						class="px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 cursor-pointer"
					>
						Back
					</button>
				{/if}
				{#if !isLastStep}
					<button
						type="button"
						on:click={goNext}
						disabled={!canAdvance}
						class="px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
					>
						Next
					</button>
				{:else}
					<button
						type="button"
						on:click={handleSubmit}
						disabled={!canAdvance || loading}
						class="px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
					>
						{#if loading}
							<div class="flex items-center">
								<div class="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2"></div>
								Creating...
							</div>
						{:else}
							{submitLabel}
						{/if}
					</button>
				{/if}
			</div>
		</div>
	</div>
</Modal>
