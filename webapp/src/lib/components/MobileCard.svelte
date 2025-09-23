<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import { resolve } from '$app/paths';
	import ActionButton from './ActionButton.svelte';
	import Badge from './Badge.svelte';
	import { getForgeIcon } from '$lib/utils/common.js';

	const dispatch = createEventDispatcher<{
		edit: { item: any };
		delete: { item: any };
		clone: { item: any };
		action: { type: string; item: any };
	}>();

	export let item: any;
	export let config: {
		entityType: 'repository' | 'organization' | 'enterprise' | 'instance' | 'pool' | 'scaleset' | 'credential' | 'endpoint' | 'template';
		primaryText: {
			field: string;
			isClickable?: boolean;
			href?: string;
			useId?: boolean; // For pools - show truncated ID
			showOwner?: boolean; // For repositories - show owner/name
			isMonospace?: boolean; // For pools/instances
		};
		secondaryText?: {
			field: string;
			computedValue?: any | ((item: any) => string); // For computed values like entity names
		};
		badges?: Array<{
			type: 'status' | 'forge' | 'auth' | 'custom';
			field?: string;
			value?: any;
			icon?: boolean;
		}>;
		actions?: Array<{
			type: 'edit' | 'delete' | 'clone';
			handler: (item: any) => void;
		}>;
		customInfo?: Array<{
			icon?: string | ((item: any) => string);
			text: string | ((item: any) => string);
		}>;
	};

	function getPrimaryText(): string {
		if (!item) return 'Unknown';
		
		const { field, useId, showOwner } = config.primaryText;
		const value = item[field];
		
		if (useId && value) {
			// For pools - show truncated ID
			return `${value.slice(0, 8)}...`;
		}
		
		if (showOwner && item.owner && item.name) {
			// For repositories - show owner/name
			return `${item.owner}/${item.name}`;
		}
		
		return value || 'Unknown';
	}

	function getSecondaryText(): string {
		if (!config.secondaryText) return '';
		
		const { field, computedValue } = config.secondaryText;
		
		if (computedValue !== undefined) {
			// Handle function-based computed values
			if (typeof computedValue === 'function') {
				return computedValue(item);
			}
			return computedValue;
		}
		
		return item?.[field] || '';
	}

	function getEntityHref(): string {
		if (!config.primaryText.href || !item) return '#';
		
		let href = config.primaryText.href;
		
		// Replace placeholders
		href = href.replace('{id}', item.id || '');
		href = href.replace('{name}', encodeURIComponent(item.name || ''));
		
		return resolve(href as any);
	}

	function handleAction(actionType: string) {
		if (!item) return;
		
		const action = config.actions?.find(a => a.type === actionType);
		if (action) {
			action.handler(item);
		}
		
		// Also dispatch for backward compatibility
		if (actionType === 'edit') {
			dispatch('edit', { item });
		} else if (actionType === 'delete') {
			dispatch('delete', { item });
		} else if (actionType === 'clone') {
			dispatch('clone', { item });
		} else {
			dispatch('action', { type: actionType, item });
		}
	}

	function getBadgeProps(badge: any) {
		switch (badge.type) {
			case 'status':
				// Handle different status types
				if (config.entityType === 'instance') {
					const status = item?.[badge.field] || 'unknown';
					let variant = 'neutral';
					let text = status.charAt(0).toUpperCase() + status.slice(1);
					
					// Handle instance status values
					if (badge.field === 'status') {
						// Instance lifecycle status
						variant = status === 'running' ? 'success' : 
								  status === 'pending' || status === 'creating' ? 'info' :
								  status === 'failed' || status === 'error' ? 'error' : 'neutral';
					} else if (badge.field === 'runner_status') {
						// Runner status within GitHub
						variant = status === 'idle' ? 'info' :
								  status === 'active' || status === 'running' ? 'success' :
								  status === 'failed' || status === 'error' ? 'error' : 'neutral';
					}
					
					return { variant, text };
				}
				// Add more status handling for other entity types
				return { variant: 'neutral', text: item?.[badge.field] || 'Unknown' };
			
			case 'forge':
				return {
					variant: 'neutral',
					text: item?.[badge.field] || 'unknown'
				};
			
			case 'auth':
				const authType = item?.[badge.field] || 'pat';
				return {
					variant: authType === 'pat' ? 'success' : 'info',
					text: authType.toUpperCase()
				};
			
			case 'custom':
				// Handle custom badge function
				if (typeof badge.value === 'function') {
					const result = badge.value(item);
					return {
						variant: result?.variant || 'neutral',
						text: result?.text || ''
					};
				}
				return {
					variant: badge.value?.variant || 'neutral',
					text: badge.value?.text || ''
				};
			
			default:
				return { variant: 'neutral', text: '' };
		}
	}
</script>

<div class="flex items-center justify-between">
	<div class="flex-1 min-w-0">
		{#if config.primaryText.isClickable}
			<a href={getEntityHref()} class="block">
				<p class="text-sm font-medium text-blue-600 dark:text-blue-400 hover:text-blue-500 dark:hover:text-blue-300 truncate{config.primaryText.isMonospace ? ' font-mono' : ''}">{getPrimaryText()}</p>
				{#if config.secondaryText}
					<p class="text-sm text-gray-500 dark:text-gray-400 truncate mt-1">
						{getSecondaryText()}
					</p>
				{/if}
			</a>
		{:else}
			<div class="block">
				<p class="text-sm font-medium text-gray-900 dark:text-white truncate">{getPrimaryText()}</p>
				{#if config.secondaryText}
					<p class="text-sm text-gray-500 dark:text-gray-400 truncate mt-1">
						{getSecondaryText()}
					</p>
				{/if}
			</div>
		{/if}
		
		{#if config.customInfo || config.badges?.some(b => b.type === 'forge')}
			<div class="flex items-center mt-2 space-x-3">
				{#if config.customInfo}
					{#each config.customInfo as info}
						{@const iconHtml = typeof info.icon === 'function' ? info.icon(item) : info.icon}
						{@const text = typeof info.text === 'function' ? info.text(item) : info.text}
						<div class="flex items-center text-xs text-gray-500 dark:text-gray-400">
							{#if iconHtml}
								{@html iconHtml}
							{/if}
							<span class="ml-1">{text}</span>
						</div>
					{/each}
				{/if}
				
				{#if config.badges}
					{#each config.badges.filter(b => b.type === 'forge') as badge}
						<div class="flex items-center">
							{@html getForgeIcon(badge.field ? (item?.[badge.field] || 'unknown') : (item?.endpoint?.endpoint_type || 'unknown'))}
							<span class="ml-1 text-xs text-gray-500 dark:text-gray-400">
								{item?.endpoint?.name || 'Unknown'}
							</span>
						</div>
					{/each}
				{/if}
			</div>
		{/if}
	</div>
	
	<div class="flex items-center space-x-3 ml-4">
		{#if config.badges}
			{#each config.badges.filter(b => b.type !== 'forge') as badge}
				{#if badge.type === 'status'}
					{@const badgeProps = getBadgeProps(badge)}
					<span class="inline-flex items-center rounded-full px-2 py-1 text-xs font-medium ring-1 ring-inset {badgeProps.variant === 'success' ? 'bg-green-50 text-green-700 ring-green-600/20 dark:bg-green-900/50 dark:text-green-300 dark:ring-green-400/20' : badgeProps.variant === 'info' ? 'bg-blue-50 text-blue-700 ring-blue-600/20 dark:bg-blue-900/50 dark:text-blue-300 dark:ring-blue-400/20' : badgeProps.variant === 'error' ? 'bg-red-50 text-red-700 ring-red-600/20 dark:bg-red-900/50 dark:text-red-300 dark:ring-red-400/20' : 'bg-gray-50 text-gray-700 ring-gray-600/20 dark:bg-gray-900/50 dark:text-gray-300 dark:ring-gray-400/20'}">
						{badgeProps.text}
					</span>
				{:else}
					{@const badgeProps = getBadgeProps(badge)}
					<Badge variant={badgeProps.variant} text={badgeProps.text} />
				{/if}
			{/each}
		{/if}
		
		{#if config.actions}
			<div class="flex space-x-2">
				{#each config.actions as action}
					<ActionButton
						action={action.type === 'clone' ? 'copy' : action.type}
						size="sm"
						title={action.type === 'edit' ? `Edit ${config.entityType}` : action.type === 'delete' ? `Delete ${config.entityType}` : action.type === 'clone' ? `Clone ${config.entityType}` : action.type}
						ariaLabel={action.type === 'edit' ? `Edit ${config.entityType}` : action.type === 'delete' ? `Delete ${config.entityType}` : action.type === 'clone' ? `Clone ${config.entityType}` : action.type}
						on:click={() => handleAction(action.type)}
					/>
				{/each}
			</div>
		{/if}
	</div>
</div>