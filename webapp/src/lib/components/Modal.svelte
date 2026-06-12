<script lang="ts" module>
	// Stack of close callbacks for currently-open modals. Escape only closes
	// the topmost modal so nested modals unwind one at a time.
	const modalStack: Array<() => void> = [];
</script>

<script lang="ts">
	import { createEventDispatcher, onMount } from 'svelte';

	const dispatch = createEventDispatcher<{
		close: void;
	}>();

	function close() {
		dispatch('close');
	}

	function handleBackdropClick() {
		close();
	}

	function handleContentClick(event: Event) {
		event.stopPropagation();
	}

	function handleWindowKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape' && modalStack[modalStack.length - 1] === close) {
			event.stopPropagation();
			close();
		}
	}

	onMount(() => {
		modalStack.push(close);
		return () => {
			const index = modalStack.indexOf(close);
			if (index > -1) {
				modalStack.splice(index, 1);
			}
		};
	});
</script>

<svelte:window on:keydown={handleWindowKeydown} />

<!-- svelte-ignore a11y_click_events_have_key_events -->
<div
	class="fixed inset-0 bg-black/30 dark:bg-black/50 overflow-y-auto h-full w-full z-50 flex items-center justify-center p-4"
	on:click={handleBackdropClick}
	role="dialog"
	aria-modal="true"
	tabindex="-1"
>
	<!-- svelte-ignore a11y-click-events-have-key-events -->
	<!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
	<div
		class="relative mx-auto bg-white dark:bg-gray-800 rounded-lg shadow-lg"
		on:click={handleContentClick}
		role="document"
	>
		<slot />
	</div>
</div>
