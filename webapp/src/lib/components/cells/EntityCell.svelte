<script lang="ts">
	import { resolve } from '$app/paths';

	export let item: any;
	export let entityType: 'repository' | 'organization' | 'enterprise' | 'pool' | 'scaleset' | 'instance' = 'repository';
	export let showOwner: boolean = false;
	export let showId: boolean = false;
	export let fontMono: boolean = false;

	$: entityName = getEntityName();
	$: entityUrl = getEntityUrl();

	function getEntityName(): string {
		// Safety check for undefined item
		if (!item) return 'Unknown';

		switch (entityType) {
			case 'repository':
				return showOwner ? `${item.owner || 'Unknown'}/${item.name || 'Unknown'}` : (item.name || 'Unknown');
			case 'organization':
			case 'enterprise':
				return item.name || 'Unknown';
			case 'pool':
				return showId ? (item.id || 'Unknown') : (item.name || 'Unknown');
			case 'scaleset':
				return item.name || 'Unknown';
			case 'instance':
				return item.name || 'Unknown';
			default:
				return item.name || item.id || 'Unknown';
		}
	}

	function getEntityUrl(): string {
		// Safety check for undefined item
		if (!item) return '#';

		let entityId;
		switch (entityType) {
			case 'instance':
				// For instances, always use name, not ID
				entityId = item.name;
				break;
			default:
				// For other entities, use ID first, then name as fallback
				entityId = item.id || item.name;
				break;
		}
		
		if (!entityId) return '#';

		switch (entityType) {
			case 'repository':
				return resolve(`/repositories/${entityId}`);
			case 'organization':
				return resolve(`/organizations/${entityId}`);
			case 'enterprise':
				return resolve(`/enterprises/${entityId}`);
			case 'pool':
				return resolve(`/pools/${entityId}`);
			case 'scaleset':
				return resolve(`/scalesets/${entityId}`);
			case 'instance':
				return resolve(`/instances/${encodeURIComponent(entityId)}`);
			default:
				return '#';
		}
	}
</script>

<div class="w-full min-w-0 text-sm font-medium">
	<a 
		href={entityUrl} 
		class="block w-full truncate text-blue-600 dark:text-blue-400 hover:text-blue-500 dark:hover:text-blue-300 {fontMono ? 'font-mono' : ''}"
		title={entityName}
	>{entityName}</a>{#if entityType === 'instance' && item?.provider_id}
		<div class="text-sm text-gray-500 dark:text-gray-400 truncate">
			{item.provider_id}
		</div>
	{/if}
</div>