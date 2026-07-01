<script lang="ts">
	export let minIdleRunners: number | undefined = undefined;
	export let maxRunners: number | undefined = undefined;
	export let runnerBootstrapTimeout: number | undefined = undefined;
</script>

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
