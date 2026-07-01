<script lang="ts">
	export let tags: string[] = [];
	export let label: string = 'Tags';
	export let placeholder: string = 'Enter a tag';
	export let required: boolean = false;

	let newTag = '';

	function addTag() {
		if (newTag.trim() && !tags.includes(newTag.trim())) {
			tags = [...tags, newTag.trim()];
			newTag = '';
		}
	}

	function removeTag(index: number) {
		tags = tags.filter((_, i) => i !== index);
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Enter') {
			event.preventDefault();
			addTag();
		}
	}
</script>

<div>
	<label for="tag-input" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
		{label}{#if required}<span class="text-red-500"> *</span>{/if}
	</label>
	<div class="space-y-2">
		<div class="flex">
			<input
				id="tag-input"
				type="text"
				bind:value={newTag}
				on:keydown={handleKeydown}
				{placeholder}
				class="flex-1 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-l-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
			/>
			<button
				type="button"
				on:click={addTag}
				class="px-3 py-2 bg-blue-600 text-white rounded-r-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 cursor-pointer"
			>
				Add
			</button>
		</div>
		{#if tags.length > 0}
			<div class="flex flex-wrap gap-2">
				{#each tags as tag, index}
					<span class="inline-flex items-center px-2 py-1 rounded-full text-xs font-medium bg-blue-100 dark:bg-blue-900 text-blue-800 dark:text-blue-200">
						{tag}
						<button
							type="button"
							on:click={() => removeTag(index)}
							aria-label={`Remove tag ${tag}`}
							class="ml-1 h-4 w-4 rounded-full hover:bg-blue-200 dark:hover:bg-blue-800 flex items-center justify-center cursor-pointer"
						>
							<svg class="h-3 w-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
							</svg>
						</button>
					</span>
				{/each}
			</div>
		{/if}
	</div>
</div>
