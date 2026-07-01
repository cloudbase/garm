<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import TagsField from './TagsField.svelte';
	import JsonEditor from '../JsonEditor.svelte';

	const dispatch = createEventDispatcher<{
		enableAgentMode: void;
	}>();

	export let runnerPrefix: string = 'garm';
	export let priority: number = 100;
	export let githubRunnerGroup: string = '';
	export let tags: string[] = [];
	export let extraSpecs: string = '{}';
	export let enabled: boolean = true;
	export let enableShell: boolean = false;
	export let entityAgentMode: boolean = false;
	export let idPrefix: string = '';
	export let enabledLabel: string = 'Enable pool immediately';
	export let showTags: boolean = true;
	export let showPriority: boolean = true;

	$: if (!entityAgentMode) {
		enableShell = false;
	}

	function inputId(name: string): string {
		return idPrefix ? `${idPrefix}-${name}` : name;
	}
</script>

<div class="space-y-4">
	<h3 class="text-lg font-medium text-gray-900 dark:text-white border-b border-gray-200 dark:border-gray-700 pb-2">
		Advanced Settings
	</h3>
	<div class="grid grid-cols-1 md:grid-cols-3 gap-4">
		<div>
			<label for={inputId('runnerPrefix')} class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
				Runner Prefix
			</label>
			<input
				id={inputId('runnerPrefix')}
				type="text"
				bind:value={runnerPrefix}
				placeholder="garm"
				class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
			/>
		</div>
		{#if showPriority}
		<div>
			<label for={inputId('priority')} class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
				Priority
			</label>
			<input
				id={inputId('priority')}
				type="number"
				bind:value={priority}
				min="1"
				placeholder="100"
				class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
			/>
		</div>
		{/if}
		<div>
			<label for={inputId('githubRunnerGroup')} class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
				GitHub Runner Group (optional)
			</label>
			<input
				id={inputId('githubRunnerGroup')}
				type="text"
				bind:value={githubRunnerGroup}
				placeholder="Default group"
				class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
			/>
		</div>
	</div>

	<!-- Tags -->
	{#if showTags}
	<TagsField bind:tags />
	{/if}

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
			id={inputId('enabled')}
			type="checkbox"
			bind:checked={enabled}
			class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 dark:border-gray-600 rounded"
		/>
		<label for={inputId('enabled')} class="ml-2 block text-sm text-gray-700 dark:text-gray-300">
			{enabledLabel}
		</label>
	</div>

	<!-- Enable Shell Checkbox -->
	<div class="space-y-2">
		<div class="flex items-center">
			<input
				id={inputId('enableShell')}
				type="checkbox"
				bind:checked={enableShell}
				disabled={!entityAgentMode}
				class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 dark:border-gray-600 rounded disabled:opacity-50 disabled:cursor-not-allowed"
			/>
			<label for={inputId('enableShell')} class="ml-2 block text-sm font-medium text-gray-700 dark:text-gray-300 {!entityAgentMode ? 'opacity-50' : ''}">
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
					Shell access requires agent mode to be enabled on the entity.
					<button
						type="button"
						on:click={() => dispatch('enableAgentMode')}
						class="underline hover:text-yellow-800 dark:hover:text-yellow-300 cursor-pointer"
					>
						Enable agent mode
					</button>
				</span>
			</div>
		{/if}
	</div>
</div>
