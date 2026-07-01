<script lang="ts">
	import type { Template } from '$lib/api/generated/api.js';

	export let image: string = '';
	export let flavor: string = '';
	export let osType: string = 'linux';
	export let osArch: string = 'amd64';
	export let selectedTemplate: number | undefined = undefined;
	export let templates: Template[] = [];
	export let loadingTemplates: boolean = false;
	export let idPrefix: string = '';

	function inputId(name: string): string {
		return idPrefix ? `${idPrefix}-${name}` : name;
	}
</script>

<div class="space-y-4">
	<h3 class="text-lg font-medium text-gray-900 dark:text-white border-b border-gray-200 dark:border-gray-700 pb-2">
		Image & OS Configuration
	</h3>
	<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
		<div>
			<label for={inputId('image')} class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
				Image <span class="text-red-500">*</span>
			</label>
			<input
				id={inputId('image')}
				type="text"
				bind:value={image}
				required
				placeholder="e.g., ubuntu:22.04"
				class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
			/>
		</div>
		<div>
			<label for={inputId('flavor')} class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
				Flavor <span class="text-red-500">*</span>
			</label>
			<input
				id={inputId('flavor')}
				type="text"
				bind:value={flavor}
				required
				placeholder="e.g., default"
				class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
			/>
		</div>
		<div>
			<label for={inputId('osType')} class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
				OS Type
			</label>
			<select
				id={inputId('osType')}
				bind:value={osType}
				class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
			>
				<option value="linux">Linux</option>
				<option value="windows">Windows</option>
			</select>
		</div>
		<div>
			<label for={inputId('osArch')} class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
				Architecture
			</label>
			<select
				id={inputId('osArch')}
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
		<label for={inputId('template')} class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
			Runner Install Template
		</label>
		{#if loadingTemplates}
			<div class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-gray-50 dark:bg-gray-700 flex items-center">
				<div class="animate-spin rounded-full h-4 w-4 border-b-2 border-blue-600 mr-2"></div>
				<span class="text-sm text-gray-600 dark:text-gray-400">Loading templates...</span>
			</div>
		{:else if templates.length > 0}
			<select
				id={inputId('template')}
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
			</p>
		{:else}
			<div class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-gray-50 dark:bg-gray-700 text-gray-500 dark:text-gray-400">
				<p class="text-sm">No templates available</p>
			</div>
		{/if}
	</div>
</div>
