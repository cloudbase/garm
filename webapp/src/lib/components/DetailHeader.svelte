<script lang="ts">
	import Button from './Button.svelte';

	export let title: string;
	export let subtitle: string;
	export let forgeIcon: string = '';
	export let onEdit: (() => void) | null = null;
	export let onDelete: (() => void) | null = null;
	export let editLabel: string = 'Edit';
	export let deleteLabel: string = 'Delete';
	export let titleClass: string = '';
</script>

<div class="bg-white dark:bg-gray-800 shadow rounded-lg">
	<div class="px-4 py-5 sm:p-6">
		<div class="sm:flex sm:items-center sm:justify-between">
			<div class="flex items-center space-x-3">
				{#if forgeIcon}
					<div class="flex-shrink-0">
						{@html forgeIcon}
					</div>
				{/if}
				<div>
					<h1 class="text-2xl font-bold text-gray-900 dark:text-white {titleClass}">{title}</h1>
					<p class="text-sm text-gray-500 dark:text-gray-400">
						{subtitle}
					</p>
				</div>
			</div>
			{#if onEdit || onDelete}
				<div class="mt-4 sm:mt-0 flex space-x-3">
					{#if onEdit}
						<Button
							variant="secondary"
							size="md"
							on:click={onEdit}
							icon="<path stroke-linecap='round' stroke-linejoin='round' stroke-width='2' d='M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z'/>"
						>
							{editLabel}
						</Button>
					{/if}
					{#if onDelete}
						<Button
							variant="danger"
							size="md"
							on:click={onDelete}
							icon="<path stroke-linecap='round' stroke-linejoin='round' stroke-width='2' d='M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16'/>"
						>
							{deleteLabel}
						</Button>
					{/if}
				</div>
			{/if}
		</div>
	</div>
</div>