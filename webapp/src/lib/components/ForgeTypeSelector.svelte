<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import { getForgeIcon } from '$lib/utils/common.js';

	const dispatch = createEventDispatcher<{
		select: 'github' | 'gitea';
	}>();

	export let selectedForgeType: 'github' | 'gitea' | '' = '';
	export let label: string = 'Select Forge Type';

	function selectForgeType(forgeType: 'github' | 'gitea') {
		selectedForgeType = forgeType;
		dispatch('select', forgeType);
	}
</script>

<fieldset>
	<legend class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
		{label}
	</legend>
	<div class="grid grid-cols-2 gap-4">
		<button
			type="button"
			on:click={() => selectForgeType('github')}
			class="flex flex-col items-center justify-center p-6 border-2 rounded-lg transition-colors cursor-pointer {selectedForgeType === 'github' ? 'border-blue-500 bg-blue-50 dark:bg-blue-900' : 'border-gray-300 dark:border-gray-600 hover:border-gray-400 dark:hover:border-gray-500'}"
		>
			{@html getForgeIcon('github', 'w-8 h-8')}
			<span class="mt-2 text-sm font-medium text-gray-900 dark:text-white">GitHub</span>
		</button>
		<button
			type="button"
			on:click={() => selectForgeType('gitea')}
			class="flex flex-col items-center justify-center p-6 border-2 rounded-lg transition-colors cursor-pointer {selectedForgeType === 'gitea' ? 'border-blue-500 bg-blue-50 dark:bg-blue-900' : 'border-gray-300 dark:border-gray-600 hover:border-gray-400 dark:hover:border-gray-500'}"
		>
			{@html getForgeIcon('gitea', 'w-8 h-8')}
			<span class="mt-2 text-sm font-medium text-gray-900 dark:text-white">Gitea</span>
		</button>
	</div>
</fieldset>