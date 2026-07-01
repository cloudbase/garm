<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import type { ScaleSet } from '$lib/api/generated/api.js';
	import UpdatePoolModal from './UpdatePoolModal.svelte';

	const dispatch = createEventDispatcher<{
		close: void;
		submit: any;
	}>();

	export let pool: ScaleSet;
</script>

<UpdatePoolModal
	{pool}
	poolType="scaleset"
	on:close={() => dispatch('close')}
	on:submit={(e) => dispatch('submit', e.detail)}
/>
