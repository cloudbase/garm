<script lang="ts">
	export let message: string = 'Loading...';
</script>

<div class="p-6 text-center">
	<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600 mx-auto"></div>
	<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">{message}</p>
</div>