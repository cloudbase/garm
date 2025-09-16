<script lang="ts">
	import type { Repository, Organization, Enterprise } from '$lib/api/generated/api.js';
	import { formatDate } from '$lib/utils/common.js';
	import Badge from './Badge.svelte';

	type Entity = Repository | Organization | Enterprise;
	
	export let entity: Entity;
	export let entityType: 'repository' | 'organization' | 'enterprise';

	function getEntityTitle(): string {
		return `${entityType.charAt(0).toUpperCase() + entityType.slice(1)} Information`;
	}

	function getEntityUrl(): string {
		if (!entity.endpoint?.base_url) return '#';
		
		// Remove trailing slash from base URL to avoid double slashes
		const baseUrl = entity.endpoint.base_url.replace(/\/$/, '');
		
		switch (entityType) {
			case 'repository':
				const repo = entity as Repository;
				return `${baseUrl}/${repo.owner}/${entity.name}`;
			case 'organization':
				return `${baseUrl}/${entity.name}`;
			case 'enterprise':
				return `${baseUrl}/enterprises/${entity.name}`;
			default:
				return '#';
		}
	}

	function getUrlLabel(): string {
		return `${entityType.charAt(0).toUpperCase() + entityType.slice(1)} URL`;
	}

	function getPoolBalancerDisplay(): string {
		const balancerType = entity.pool_balancing_type;
		if (!balancerType || balancerType === '' || balancerType === 'none') {
			return 'Round Robin (default)';
		}
		
		switch (balancerType) {
			case 'roundrobin':
				return 'Round Robin';
			case 'pack':
				return 'Pack';
			default:
				return balancerType;
		}
	}


	function getOwner(): string {
		if (entityType === 'repository') {
			return (entity as Repository).owner || '';
		}
		return '';
	}
</script>

<div class="bg-white dark:bg-gray-800 shadow rounded-lg">
	<div class="px-4 py-5 sm:p-6">
		<h2 class="text-lg font-medium text-gray-900 dark:text-white mb-4">{getEntityTitle()}</h2>
		<dl class="grid grid-cols-1 gap-x-4 gap-y-6 sm:grid-cols-2">
			<div>
				<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">ID</dt>
				<dd class="mt-1 text-sm text-gray-900 dark:text-white font-mono">{entity.id}</dd>
			</div>
			<div>
				<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Created At</dt>
				<dd class="mt-1 text-sm text-gray-900 dark:text-white">{formatDate(entity.created_at)}</dd>
			</div>
			<div>
				<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Updated At</dt>
				<dd class="mt-1 text-sm text-gray-900 dark:text-white">{formatDate(entity.updated_at)}</dd>
			</div>
			<div>
				<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Status</dt>
				<dd class="mt-1">
					{#if entity.pool_manager_status?.running}
						<Badge variant="success" text="Running" />
					{:else}
						<Badge variant="error" text="Stopped" />
					{/if}
				</dd>
			</div>
			<div>
				<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Pool Balancer Type</dt>
				<dd class="mt-1 text-sm text-gray-900 dark:text-white">{getPoolBalancerDisplay()}</dd>
			</div>
			<div>
				<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">{getUrlLabel()}</dt>
				<dd class="mt-1 text-sm">
					<a href={getEntityUrl()} target="_blank" rel="noopener noreferrer" class="text-blue-600 dark:text-blue-400 hover:text-blue-500 dark:hover:text-blue-300">
						{getEntityUrl()}
						<svg class="inline w-3 h-3 ml-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"/>
						</svg>
					</a>
				</dd>
			</div>
		</dl>
	</div>
</div>