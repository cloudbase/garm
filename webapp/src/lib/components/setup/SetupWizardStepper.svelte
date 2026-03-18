<script lang="ts">
	export let steps: Array<{ number: number; title: string; description: string }>;
	export let currentStep: number;
</script>

<nav aria-label="Setup progress">
	<ol class="flex items-center w-full">
		{#each steps as step, i}
			<li class="flex items-center {i < steps.length - 1 ? 'flex-1' : ''}">
				<div class="flex flex-col items-center">
					<!-- Circle -->
					{#if step.number < currentStep}
						<!-- Completed -->
						<div class="flex items-center justify-center w-10 h-10 rounded-full bg-blue-600 shrink-0">
							<svg class="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path>
							</svg>
						</div>
					{:else if step.number === currentStep}
						<!-- Current -->
						<div class="flex items-center justify-center w-10 h-10 rounded-full border-2 border-blue-600 bg-blue-50 dark:bg-blue-900/30 shrink-0">
							<span class="text-sm font-bold text-blue-600 dark:text-blue-400">{step.number}</span>
						</div>
					{:else}
						<!-- Pending -->
						<div class="flex items-center justify-center w-10 h-10 rounded-full border-2 border-gray-300 dark:border-gray-600 shrink-0">
							<span class="text-sm font-medium text-gray-500 dark:text-gray-400">{step.number}</span>
						</div>
					{/if}
					<!-- Label -->
					<div class="mt-2 text-center min-w-[80px]">
						<p class="text-sm font-medium {step.number <= currentStep ? 'text-blue-600 dark:text-blue-400' : 'text-gray-500 dark:text-gray-400'}">
							{step.title}
						</p>
						<p class="hidden md:block text-xs {step.number <= currentStep ? 'text-gray-600 dark:text-gray-300' : 'text-gray-400 dark:text-gray-500'} mt-0.5">
							{step.description}
						</p>
					</div>
				</div>
				<!-- Connector line -->
				{#if i < steps.length - 1}
					<div class="flex-1 h-0.5 mx-4 mt-[-2rem] {step.number < currentStep ? 'bg-blue-600' : 'bg-gray-300 dark:bg-gray-600'}"></div>
				{/if}
			</li>
		{/each}
	</ol>
</nav>
