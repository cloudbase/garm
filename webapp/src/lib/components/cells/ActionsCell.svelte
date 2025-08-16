<script lang="ts">
	import ActionButton from '$lib/components/ActionButton.svelte';
	import { createEventDispatcher } from 'svelte';

	const dispatch = createEventDispatcher<{
		edit: { item: any };
		delete: { item: any };
		action: { type: string; item: any };
	}>();

	export let item: any;
	export let actions: Array<{
		type: 'edit' | 'delete' | 'custom';
		label?: string;
		title?: string;
		ariaLabel?: string;
		action?: 'edit' | 'delete' | 'view';
	}> = [
		{ type: 'edit', title: 'Edit', ariaLabel: 'Edit item', action: 'edit' },
		{ type: 'delete', title: 'Delete', ariaLabel: 'Delete item', action: 'delete' }
	];

	function handleAction(actionType: string) {
		// Safety check for undefined item
		if (!item) return;

		if (actionType === 'edit') {
			dispatch('edit', { item });
		} else if (actionType === 'delete') {
			dispatch('delete', { item });
		} else {
			dispatch('action', { type: actionType, item });
		}
	}
</script>

<div class="flex justify-end space-x-2">
	{#each actions as action}
		<ActionButton
			action={action.action || (action.type === 'edit' ? 'edit' : action.type === 'delete' ? 'delete' : 'view')}
			title={action.title || (action.type === 'edit' ? 'Edit' : action.type === 'delete' ? 'Delete' : action.label)}
			ariaLabel={action.ariaLabel || (action.type === 'edit' ? 'Edit item' : action.type === 'delete' ? 'Delete item' : action.label)}
			on:click={() => handleAction(action.type)}
		/>
	{/each}
</div>