<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import CreatePoolModal from './CreatePoolModal.svelte';

	const dispatch = createEventDispatcher<{
		close: void;
		submit: any;
	}>();

	export let initialEntityType: 'repository' | 'organization' | 'enterprise' | '' = '';
	export let initialEntityId: string = '';
</script>

<CreatePoolModal
	poolType="scaleset"
	{initialEntityType}
	{initialEntityId}
	on:close={() => dispatch('close')}
	on:submit={(e) => dispatch('submit', e.detail)}
/>
