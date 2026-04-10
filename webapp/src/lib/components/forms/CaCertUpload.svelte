<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import { handleFileInputAsBase64 } from '$lib/utils/file';

	const dispatch = createEventDispatcher<{
		select: { base64: string; fileName: string };
		clear: void;
		remove: void;
		view: void;
	}>();

	/** Display name of the currently selected or existing certificate file */
	export let fileName = '';
	/** Whether a certificate is already configured on the server */
	export let hasExisting = false;
	/** Whether removal is pending (not yet saved) */
	export let pendingRemoval = false;
	/** Whether to show a "View" action for the existing certificate */
	export let showView = false;
	/** Prefix for unique element IDs (to avoid collisions when multiple instances exist) */
	export let idPrefix = '';

	const inputId = `${idPrefix}ca-cert-file`;

	function handleFileUpload(event: Event) {
		handleFileInputAsBase64(
			event,
			(base64, name) => {
				dispatch('select', { base64, fileName: name });
			},
			() => {
				dispatch('clear');
			}
		);
	}

	function clearFile() {
		const fileInput = document.getElementById(inputId) as HTMLInputElement;
		if (fileInput) fileInput.value = '';
		dispatch('clear');
	}

	function triggerFileSelect() {
		document.getElementById(inputId)?.click();
	}
</script>

<div class="space-y-3 border-t border-gray-200 dark:border-gray-700 pt-4">
	<label for={inputId} class="block text-sm font-medium text-gray-700 dark:text-gray-300">
		CA Certificate Bundle (Optional)
	</label>
	{#if pendingRemoval}
		<div class="border-2 border-dashed rounded-lg p-4 text-center border-yellow-400 dark:border-yellow-500 bg-yellow-50 dark:bg-yellow-900/20">
			<div class="space-y-2">
				<svg class="mx-auto h-8 w-8 text-yellow-500 dark:text-yellow-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z"></path>
				</svg>
				<p class="text-sm font-medium text-yellow-700 dark:text-yellow-300">Will be removed on save</p>
				<button type="button" on:click={() => dispatch('clear')} class="text-xs text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 hover:underline cursor-pointer">
					Undo
				</button>
			</div>
		</div>
	{:else}
		<div class="border-2 border-dashed rounded-lg p-4 text-center transition-colors {fileName || hasExisting ? 'border-green-500 dark:border-green-400 bg-green-50 dark:bg-green-900/20' : 'border-gray-300 dark:border-gray-600 hover:border-blue-400 dark:hover:border-blue-400'}">
			<input
				type="file"
				id={inputId}
				accept=".pem,.crt,.cer,.cert"
				on:change={handleFileUpload}
				class="hidden"
			/>
			{#if fileName}
				<div class="space-y-2">
					<svg class="mx-auto h-8 w-8 text-green-600 dark:text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path>
					</svg>
					<p class="text-sm font-medium text-green-700 dark:text-green-300">{fileName}</p>
					<div class="flex justify-center space-x-2">
						<button type="button" on:click={triggerFileSelect} class="text-xs text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 hover:underline cursor-pointer">
							Replace
						</button>
						<span class="text-xs text-gray-400">&bull;</span>
						<button type="button" on:click={clearFile} class="text-xs text-red-600 dark:text-red-400 hover:text-red-800 dark:hover:text-red-300 hover:underline cursor-pointer">
							Remove
						</button>
					</div>
				</div>
			{:else if hasExisting}
				<div class="space-y-2">
					<svg class="mx-auto h-8 w-8 text-green-600 dark:text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path>
					</svg>
					<p class="text-sm font-medium text-green-700 dark:text-green-300">Certificate configured</p>
					<div class="flex justify-center space-x-2">
						<button type="button" on:click={triggerFileSelect} class="text-xs text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 hover:underline cursor-pointer">
							Replace
						</button>
						{#if showView}
							<span class="text-xs text-gray-400">&bull;</span>
							<button type="button" on:click={() => dispatch('view')} class="text-xs text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 hover:underline cursor-pointer">
								View
							</button>
						{/if}
						<span class="text-xs text-gray-400">&bull;</span>
						<button type="button" on:click={() => dispatch('remove')} class="text-xs text-red-600 dark:text-red-400 hover:text-red-800 dark:hover:text-red-300 hover:underline cursor-pointer">
							Remove
						</button>
					</div>
				</div>
			{:else}
				<div class="space-y-2">
					<svg class="mx-auto h-8 w-8 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
					</svg>
					<p class="text-sm text-gray-600 dark:text-gray-400">
						<button type="button" on:click={triggerFileSelect} class="text-gray-900 dark:text-white hover:text-gray-700 dark:hover:text-gray-300 hover:underline cursor-pointer">
							Choose a file
						</button>
						or drag and drop
					</p>
					<p class="text-xs text-gray-500 dark:text-gray-400">PEM, CRT, CER, CERT files only</p>
				</div>
			{/if}
		</div>
	{/if}
</div>
