<script lang="ts">
	import ActionButton from '$lib/components/ActionButton.svelte';
	import { createEventDispatcher } from 'svelte';

	const dispatch = createEventDispatcher<{
		edit: { item: any };
		delete: { item: any };
		clone: { item: any };
		shell: { item: any };
		action: { type: string; item: any };
	}>();

	export let item: any;
	export let actions: Array<{
		type: 'edit' | 'delete' | 'copy' | 'custom' | 'shell';
		label?: string;
		title?: string;
		ariaLabel?: string;
		action?: 'edit' | 'delete' | 'view' | 'copy' | 'clone' | 'download' | 'shell';
		isDisabled?: (item: any) => boolean;
		disabledTitle?: string | ((item: any) => string);
	}> = [
		{ type: 'edit', title: 'Edit', ariaLabel: 'Edit item', action: 'edit' },
		{ type: 'delete', title: 'Delete', ariaLabel: 'Delete item', action: 'delete' }
	];

	function handleAction(actionType: string, actionValue?: string) {
		// Safety check for undefined item
		if (!item) return;

		// Use actionValue if provided, otherwise use actionType
		const effectiveAction = actionValue || actionType;

		if (effectiveAction === 'edit') {
			dispatch('edit', { item });
		} else if (effectiveAction === 'delete') {
			dispatch('delete', { item });
		} else if (effectiveAction === 'copy' || effectiveAction === 'clone') {
			dispatch('clone', { item });
		} else if (actionType === 'shell') {
			dispatch('shell', { item });
		} else {
			dispatch('action', { type: effectiveAction, item });
		}
	}
</script>

<div class="flex justify-end space-x-2">
	{#each actions as action}
		{@const isDisabled = action.isDisabled ? action.isDisabled(item) : false}
		{@const buttonAction = action.action === 'clone' ? 'copy' : (action.action || (action.type === 'edit' ? 'edit' : action.type === 'delete' ? 'delete' : action.type === 'copy' ? 'copy' : action.type === 'shell' ? 'shell' : 'view'))}
		{@const disabledTitleText = typeof action.disabledTitle === 'function' ? action.disabledTitle(item) : action.disabledTitle}
		{@const buttonTitle = isDisabled && disabledTitleText ? disabledTitleText : (action.title || (action.type === 'edit' ? 'Edit' : action.type === 'delete' ? 'Delete' : action.type === 'copy' ? 'Clone' : action.type === 'shell' ? 'Shell' : action.label))}
		<ActionButton
			action={buttonAction}
			title={buttonTitle}
			ariaLabel={action.ariaLabel || (action.type === 'edit' ? 'Edit item' : action.type === 'delete' ? 'Delete item' : action.type === 'copy' ? 'Clone item' : action.type === 'shell' ? 'Open shell' : action.label)}
			disabled={isDisabled}
			on:click={() => handleAction(action.type, action.action)}
		/>
	{/each}
</div>