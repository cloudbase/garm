<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import LoadingState from './LoadingState.svelte';
	import ErrorState from './ErrorState.svelte';
	import EmptyState from './EmptyState.svelte';
	import SearchFilterBar from './SearchFilterBar.svelte';
	import BackendSearchBar from './BackendSearchBar.svelte';
	import TablePagination from './TablePagination.svelte';
	import MobileCard from './MobileCard.svelte';
	import type { ComponentType } from 'svelte';
	
	// Table configuration
	export let columns: Array<{
		key: string;
		title: string;
		sortable?: boolean;
		width?: string;
		flexible?: boolean; // Column should expand to fill remaining space
		flexRatio?: number; // Custom flex ratio for flexible columns
		align?: 'left' | 'center' | 'right';
		class?: string;
		cellComponent?: any;
		cellProps?: Record<string, any> | ((item: any) => Record<string, any>);
	}> = [];
	
	// Data and state
	export let data: any[] = [];
	export let loading: boolean = false;
	export let error: string = '';
	export let totalItems: number = 0;
	export let itemName: string = 'results';
	
	// Search and pagination
	export let searchTerm: string = '';
	export let searchPlaceholder: string = 'Search...';
	export let showSearch: boolean = true;
	export let searchType: 'client' | 'backend' = 'client'; // Type of search
	export let searchHelpText: string = ''; // Help text for backend search
	export let currentPage: number = 1;
	export let perPage: number = 25;
	export let totalPages: number = 1;
	export let showPagination: boolean = true;
	export let showPerPageSelector: boolean = true;
	export let paginationComponent: ComponentType | null = null; // Custom pagination component
	export let paginationProps: Record<string, any> = {}; // Props to pass to custom pagination component
	
	// Empty state configuration
	export let emptyTitle: string = 'No items found';
	export let emptyMessage: string = '';
	export let emptyIconType: 'document' | 'building' | 'users' | 'cog' | 'key' | 'settings' = 'document';
	
	// Error state configuration
	export let errorTitle: string = 'Error loading data';
	export let showRetry: boolean = false;
	
	// Mobile responsive
	export let showMobileCards: boolean = true;
	export let mobileCardConfig: any = null;
	
	const dispatch = createEventDispatcher<{
		search: { term: string };
		pageChange: { page: number };
		perPageChange: { perPage: number };
		pageSizeChange: { pageSize: number };
		prefetch: { page: number };
		retry: void;
		rowClick: { item: any; index: number };
		cellClick: { item: any; column: any; value: any };
		edit: { item: any };
		delete: { item: any };
		clone: { item: any };
		shell: { item: any };
		action: { type: string; item: any };
	}>();

	function handleSearch(event: CustomEvent<{ term: string }> | CustomEvent<string>) {
		// Backend search sends string directly, client search sends object
		const term = typeof (event as any).detail === 'string' ? (event as any).detail : (event as any).detail.term;
		dispatch('search', { term });
	}
	
	function handlePageChange(event: CustomEvent<{ page: number }>) {
		dispatch('pageChange', event.detail);
	}
	
	function handlePerPageChange(event: CustomEvent<{ perPage: number }>) {
		dispatch('perPageChange', event.detail);
	}
	
	function handleRetry() {
		dispatch('retry');
	}
	
	function handleRowClick(item: any, index: number) {
		dispatch('rowClick', { item, index });
	}
	
	function handleCellClick(item: any, column: any, value: any) {
		dispatch('cellClick', { item, column, value });
	}

	function handleEdit(event: CustomEvent<{ item: any }>) {
		dispatch('edit', event.detail);
	}

	function handleDelete(event: CustomEvent<{ item: any }>) {
		dispatch('delete', event.detail);
	}

	function handleClone(event: CustomEvent<{ item: any }>) {
		dispatch('clone', event.detail);
	}

	function handleShell(event: CustomEvent<{ item: any }>) {
		dispatch('shell', event.detail);
	}

	function handleAction(event: CustomEvent<{ type: string; item: any }>) {
		dispatch('action', event.detail);
	}
	
	function getColumnClass(column: any): string {
		const baseClass = 'px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider';
		const alignClass = column.align === 'right' ? 'text-right' : column.align === 'center' ? 'text-center' : 'text-left';
		const customClass = column.class || '';
		return `${baseClass} ${alignClass} ${customClass}`.trim();
	}
	
	function getCellClass(column: any): string {
		const baseClass = 'px-6 py-4 text-sm';
		const alignClass = column.align === 'right' ? 'text-right' : column.align === 'center' ? 'text-center' : 'text-left';
		const colorClass = column.key === 'actions' ? 'font-medium' : 'text-gray-900 dark:text-white';
		// Add min-w-0 for flexible columns to ensure proper truncation
		const overflowClass = column.flexible ? 'min-w-0' : '';
		return `${baseClass} ${alignClass} ${colorClass} ${overflowClass}`.trim();
	}

	function getGridTemplate(): string {
		// Build grid-template-columns based on column configuration
		return columns.map(column => {
			if (column.flexible) {
				// Use custom flex ratio if specified, otherwise default to 1fr
				const ratio = column.flexRatio || 1;
				return `${ratio}fr`;
			} else {
				return 'auto'; // Size to content
			}
		}).join(' ');
	}
	
	$: computedEmptyMessage = emptyMessage || (searchTerm ? `No items found matching "${searchTerm}"` : `No ${itemName} found`);
</script>

<div class="space-y-6">
	{#if showSearch}
		{#if searchType === 'backend'}
			<BackendSearchBar
				bind:value={searchTerm}
				placeholder={searchPlaceholder}
				helpText={searchHelpText}
				showButton={false}
				on:search={handleSearch}
			/>
		{:else}
			<SearchFilterBar
				bind:searchTerm
				bind:perPage
				placeholder={searchPlaceholder}
				{showPerPageSelector}
				on:search={handleSearch}
				on:perPageChange={handlePerPageChange}
			/>
		{/if}
	{/if}
	
	<div class="bg-white dark:bg-gray-800 shadow rounded-lg overflow-hidden">
		{#if loading}
			<LoadingState message="Loading {itemName}..." />
		{:else if error}
			<ErrorState 
				title={errorTitle}
				message={error}
				{showRetry}
				onRetry={showRetry ? handleRetry : undefined}
			/>
		{:else if data.length === 0}
			<EmptyState 
				title={emptyTitle}
				message={computedEmptyMessage}
				iconType={emptyIconType}
			/>
		{:else}
			{#if showMobileCards}
				<!-- Mobile view - Card layout -->
				<div class="block sm:hidden divide-y divide-gray-200 dark:divide-gray-700">
					{#each data as item, index (item.id || item.name || index)}
						<div class="px-4 py-4 hover:bg-gray-50 dark:hover:bg-gray-700 relative">
							{#if mobileCardConfig}
								<!-- Use MobileCard component -->
								{#key `${item.id || item.name}-${item.updated_at}-mobile`}
									<MobileCard 
										{item} 
										config={mobileCardConfig} 
										on:edit
										on:delete
										on:clone
										on:action
									/>
								{/key}
							{:else}
								<!-- Fallback to slot for backward compatibility -->
								<slot name="mobile-card" {item} {index} />
							{/if}
						</div>
					{/each}
				</div>
			{/if}

			<!-- Desktop view - Grid layout -->
			<div class="hidden sm:block overflow-x-auto bg-white dark:bg-gray-800 rounded-lg shadow">
				<div 
					class="grid gap-0"
					style="grid-template-columns: {getGridTemplate()}"
				>
					<!-- Header row -->
					{#each columns as column}
						<div class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider bg-gray-50 dark:bg-gray-700 border-b border-gray-200 dark:border-gray-600 {column.align === 'right' ? 'text-right' : column.align === 'center' ? 'text-center' : 'text-left'}">
							{column.title}
						</div>
					{/each}
					
					<!-- Data rows -->
					{#each data as item, index (item.id || item.name || index)}
						{#each columns as column}
							<div class="{getCellClass(column)} border-b border-gray-200 dark:border-gray-700">
								{#if column.cellComponent}
									{#key `${item.id || item.name}-${item.updated_at}-${column.key}`}
										<svelte:component 
											this={column.cellComponent} 
											{item} 
											{...typeof column.cellProps === 'function' ? column.cellProps(item) : column.cellProps}
											on:edit={handleEdit}
											on:delete={handleDelete}
											on:clone={handleClone}
											on:shell={handleShell}
											on:action={handleAction}
										/>
									{/key}
								{:else}
									<slot name="cell" {item} {column} {index} value={item[column.key]} />
								{/if}
							</div>
						{/each}
					{/each}
				</div>
			</div>
		{/if}
		
		{#if showPagination && !loading && !error && data.length > 0}
			{#if paginationComponent}
				<svelte:component
					this={paginationComponent}
					{currentPage}
					{totalPages}
					{totalItems}
					pageSize={perPage}
					{loading}
					{itemName}
					{...paginationProps}
					on:pageChange={handlePageChange}
					on:pageSizeChange={handlePerPageChange}
					on:prefetch
				/>
			{:else}
				<TablePagination
					{currentPage}
					{totalPages}
					{perPage}
					{totalItems}
					{itemName}
					on:pageChange={handlePageChange}
				/>
			{/if}
		{/if}
	</div>
</div>