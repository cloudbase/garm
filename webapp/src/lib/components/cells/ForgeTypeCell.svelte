<script lang="ts">
	import Badge from '$lib/components/Badge.svelte';

	export let item: any;
	export let showUrl: boolean = true;

	$: forgeType = item?.endpoint?.endpoint_type || 'Unknown';
	$: baseUrl = item?.endpoint?.base_url;
	$: variant = getForgeVariant(forgeType);

	function getForgeVariant(type: string): 'gray' | 'green' | 'secondary' {
		switch (type?.toLowerCase()) {
			case 'github':
				return 'gray';
			case 'gitea':
				return 'green';
			default:
				return 'secondary';
		}
	}

	function getDisplayName(type: string) {
		switch (type?.toLowerCase()) {
			case 'github':
				return 'GitHub';
			case 'gitea':
				return 'Gitea';
			default:
				return type || 'Unknown';
		}
	}
</script>

<div class="flex items-center space-x-2">
	<Badge variant={variant} text={getDisplayName(forgeType)} />
	{#if showUrl && baseUrl}
		<a href={baseUrl} target="_blank" rel="noopener noreferrer" class="text-sm text-blue-600 dark:text-blue-400 hover:text-blue-500 dark:hover:text-blue-300">
			{baseUrl}
		</a>
	{/if}
</div>