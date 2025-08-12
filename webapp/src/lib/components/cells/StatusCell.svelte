<script lang="ts">
	import Badge from '$lib/components/Badge.svelte';
	import { getEntityStatusBadge } from '$lib/utils/common.js';
	import { formatStatusText, getStatusBadgeClass } from '$lib/utils/status.js';

	export let item: any;
	export let statusType: 'entity' | 'instance' | 'enabled' | 'custom' = 'entity';
	export let statusField: string = 'status';

	// Make status directly reactive to item properties instead of using getStatus()
	$: statusValue = item?.[statusField] || 'unknown';
	$: status = (() => {
		if (!item) {
			return {
				variant: 'error' as const,
				text: 'Unknown'
			};
		}

		switch (statusType) {
			case 'entity':
				return getEntityStatusBadge(item);
			case 'instance':
				// Map status values to badge variants based on official enums
				let variant: any = 'secondary';
				switch (statusValue.toLowerCase()) {
					// InstanceStatus - Running states (green)
					case 'running':
						variant = 'success';
						break;
					
					// InstanceStatus - Stopped/idle states (blue)  
					case 'stopped':
						variant = 'info';
						break;
					
					// InstanceStatus - Creating/pending states (yellow)
					case 'creating':
					case 'pending_create':
						variant = 'warning';
						break;
					
					// InstanceStatus - Deleting states (orange/yellow)
					case 'deleting':
					case 'pending_delete':
					case 'pending_force_delete':
						variant = 'warning';
						break;
					
					// InstanceStatus - Error/deleted states (red)
					case 'error':
					case 'deleted':
						variant = 'error';
						break;
					
					// RunnerStatus - Active/working states (green)
					case 'active':
					case 'online':
						variant = 'success';
						break;
					
					// RunnerStatus - Idle states (blue)
					case 'idle':
						variant = 'info';
						break;
					
					// RunnerStatus - Pending/installing states (yellow)
					case 'pending':
					case 'installing':
						variant = 'warning';
						break;
					
					// RunnerStatus - Failed/terminated/offline states (red)
					case 'failed':
					case 'terminated':
					case 'offline':
						variant = 'error';
						break;
					
					// Unknown states (gray)
					case 'unknown':
					default:
						variant = 'secondary';
						break;
				}
				return {
					variant,
					text: formatStatusText(statusValue)
				};
			case 'enabled':
				return {
					variant: item.enabled ? 'success' as const : 'error' as const,
					text: item.enabled ? 'Enabled' : 'Disabled'
				};
			case 'custom':
				const value = item[statusField] || 'Unknown';
				// Special handling for auth types
				if (statusField === 'auth-type') {
					const authType = value === 'pat' || !value ? 'pat' : 'app';
					return {
						variant: authType === 'pat' ? 'success' as const : 'info' as const,
						text: authType === 'pat' ? 'PAT' : 'App'
					};
				}
				return {
					variant: 'info' as const,
					text: value
				};
			default:
				return getEntityStatusBadge(item);
		}
	})();

</script>

{#key `${item?.name || 'item'}-${item?.[statusField] || 'status'}-${item?.updated_at || 'time'}`}
	<Badge variant={status.variant} text={status.text} />
{/key}