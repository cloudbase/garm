import { resolve } from '$app/paths';

/**
 * Common utility functions shared across components and pages
 */

/**
 * Formats a date string or Date object to a human-readable format
 */
export function formatDate(date: string | Date | null | undefined): string {
	if (!date) return 'N/A';
	try {
		const d = typeof date === 'string' ? new Date(date) : date;
		return d.toLocaleString();
	} catch {
		return 'Invalid Date';
	}
}

/**
 * Returns the appropriate forge icon SVG for the given endpoint type
 * @param endpointType - The type of endpoint ('github', 'gitea', etc.)
 * @param sizeClasses - Optional size classes (e.g., 'w-4 h-4', 'w-8 h-8'). Defaults to 'w-4 h-4'
 */
export function getForgeIcon(endpointType: string, sizeClasses: string = 'w-4 h-4'): string {
	if (endpointType === 'gitea') {
		return `<svg class="${sizeClasses}" xmlns="http://www.w3.org/2000/svg" xml:space="preserve" viewBox="0 0 640 640"><path d="m395.9 484.2-126.9-61c-12.5-6-17.9-21.2-11.8-33.8l61-126.9c6-12.5 21.2-17.9 33.8-11.8 17.2 8.3 27.1 13 27.1 13l-.1-109.2 16.7-.1.1 117.1s57.4 24.2 83.1 40.1c3.7 2.3 10.2 6.8 12.9 14.4 2.1 6.1 2 13.1-1 19.3l-61 126.9c-6.2 12.7-21.4 18.1-33.9 12" style="fill:#fff"/><path d="M622.7 149.8c-4.1-4.1-9.6-4-9.6-4s-117.2 6.6-177.9 8c-13.3.3-26.5.6-39.6.7v117.2c-5.5-2.6-11.1-5.3-16.6-7.9 0-36.4-.1-109.2-.1-109.2-29 .4-89.2-2.2-89.2-2.2s-141.4-7.1-156.8-8.5c-9.8-.6-22.5-2.1-39 1.5-8.7 1.8-33.5 7.4-53.8 26.9C-4.9 212.4 6.6 276.2 8 285.8c1.7 11.7 6.9 44.2 31.7 72.5 45.8 56.1 144.4 54.8 144.4 54.8s12.1 28.9 30.6 55.5c25 33.1 50.7 58.9 75.7 62 63 0 188.9-.1 188.9-.1s12 .1 28.3-10.3c14-8.5 26.5-23.4 26.5-23.4S547 483 565 451.5c5.5-9.7 10.1-19.1 14.1-28 0 0 55.2-117.1 55.2-231.1-1.1-34.5-9.6-40.6-11.6-42.6M125.6 353.9c-25.9-8.5-36.9-18.7-36.9-18.7S69.6 321.8 60 295.4c-16.5-44.2-1.4-71.2-1.4-71.2s8.4-22.5 38.5-30c13.8-3.7 31-3.1 31-3.1s7.1 59.4 15.7 94.2c7.2 29.2 24.8 77.7 24.8 77.7s-26.1-3.1-43-9.1m300.3 107.6s-6.1 14.5-19.6 15.4c-5.8.4-10.3-1.2-10.3-1.2s-.3-.1-5.3-2.1l-112.9-55s-10.9-5.7-12.8-15.6c-2.2-8.1 2.7-18.1 2.7-18.1L322 273s4.8-9.7 12.2-13c.6-.3 2.3-1 4.5-1.5 8.1-2.1 18 2.8 18 2.8L467.4 315s12.6 5.7 15.3 16.2c1.9 7.4-.5 14-1.8 17.2-6.3 15.4-55 113.1-55 113.1" style="fill:#609926"/><path d="M326.8 380.1c-8.2.1-15.4 5.8-17.3 13.8s2 16.3 9.1 20c7.7 4 17.5 1.8 22.7-5.4 5.1-7.1 4.3-16.9-1.8-23.1l24-49.1c1.5.1 3.7.2 6.2-.5 4.1-.9 7.1-3.6 7.1-3.6 4.2 1.8 8.6 3.8 13.2 6.1 4.8 2.4 9.3 4.9 13.4 7.3.9.5 1.8 1.1 2.8 1.9 1.6 1.3 3.4 3.1 4.7 5.5 1.9 5.5-1.9 14.9-1.9 14.9-2.3 7.6-18.4 40.6-18.4 40.6-8.1-.2-15.3 5-17.7 12.5-2.6 8.1 1.1 17.3 8.9 21.3s17.4 1.7 22.5-5.3c5-6.8 4.6-16.3-1.1-22.6 1.9-3.7 3.7-7.4 5.6-11.3 5-10.4 13.5-30.4 13.5-30.4.9-1.7 5.7-10.3 2.7-21.3-2.5-11.4-12.6-16.7-12.6-16.7-12.2-7.9-29.2-15.2-29.2-15.2s0-4.1-1.1-7.1c-1.1-3.1-2.8-5.1-3.9-6.3 4.7-9.7 9.4-19.3 14.1-29-4.1-2-8.1-4-12.2-6.1-4.8 9.8-9.7 19.7-14.5 29.5-6.7-.1-12.9 3.5-16.1 9.4-3.4 6.3-2.7 14.1 1.9 19.8z" style="fill:#609926"/></svg>`;
	} else if (endpointType === 'github') {
		// GitHub (also used for GHES)
		return `<div class="inline-flex ${sizeClasses}"><svg class="${sizeClasses} dark:hidden" width="98" height="96" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 98 96"><path fill-rule="evenodd" clip-rule="evenodd" d="M48.854 0C21.839 0 0 22 0 49.217c0 21.756 13.993 40.172 33.405 46.69 2.427.49 3.316-1.059 3.316-2.362 0-1.141-.08-5.052-.08-9.127-13.59 2.934-16.42-5.867-16.42-5.867-2.184-5.704-5.42-7.17-5.42-7.17-4.448-3.015.324-3.015.324-3.015 4.934.326 7.523 5.052 7.523 5.052 4.367 7.496 11.404 5.378 14.235 4.074.404-3.178 1.699-5.378 3.074-6.6-10.839-1.141-22.243-5.378-22.243-24.283 0-5.378 1.94-9.778 5.014-13.2-.485-1.222-2.184-6.275.486-13.038 0 0 4.125-1.304 13.426 5.052a46.97 46.97 0 0 1 12.214-1.63c4.125 0 8.33.571 12.213 1.63 9.302-6.356 13.427-5.052 13.427-5.052 2.67 6.763.97 11.816.485 13.038 3.155 3.422 5.015 7.822 5.015 13.2 0 18.905-11.404 23.06-22.324 24.283 1.78 1.548 3.316 4.481 3.316 9.126 0 6.6-.08 11.897-.08 13.526 0 1.304.89 2.853 3.316 2.364 19.412-6.52 33.405-24.935 33.405-46.691C97.707 22 75.788 0 48.854 0z" fill="#24292f"/></svg><svg class="${sizeClasses} hidden dark:block" width="98" height="96" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 98 96"><path fill-rule="evenodd" clip-rule="evenodd" d="M48.854 0C21.839 0 0 22 0 49.217c0 21.756 13.993 40.172 33.405 46.69 2.427.49 3.316-1.059 3.316-2.362 0-1.141-.08-5.052-.08-9.127-13.59 2.934-16.42-5.867-16.42-5.867-2.184-5.704-5.42-7.17-5.42-7.17-4.448-3.015.324-3.015.324-3.015 4.934.326 7.523 5.052 7.523 5.052 4.367 7.496 11.404 5.378 14.235 4.074.404-3.178 1.699-5.378 3.074-6.6-10.839-1.141-22.243-5.378-22.243-24.283 0-5.378 1.94-9.778 5.014-13.2-.485-1.222-2.184-6.275.486-13.038 0 0 4.125-1.304 13.426 5.052a46.97 46.97 0 0 1 12.214-1.63c4.125 0 8.33.571 12.213 1.63 9.302-6.356 13.427-5.052 13.427-5.052 2.67 6.763.97 11.816.485 13.038 3.155 3.422 5.015 7.822 5.015 13.2 0 18.905-11.404 23.06-22.324 24.283 1.78 1.548 3.316 4.481 3.316 9.126 0 6.6-.08 11.897-.08 13.526 0 1.304.89 2.853 3.316 2.364 19.412-6.52 33.405-24.935 33.405-46.691C97.707 22 75.788 0 48.854 0z" fill="#fff"/></svg></div>`;
	} else {
		// Return a generic placeholder icon if endpoint type is unknown
		return `<svg class="${sizeClasses} text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
			<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
			<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
		</svg>`;
	}
}

/**
 * Truncates an image name to a specified length and indicates if it was truncated
 */
export function truncateImageName(imageName: string, maxLength: number = 25): { truncated: string, isTruncated: boolean } {
	if (imageName.length <= maxLength) {
		return { truncated: imageName, isTruncated: false };
	}
	return { truncated: imageName.substring(0, maxLength) + '...', isTruncated: true };
}

/**
 * Gets the entity name for a Pool or ScaleSet object
 */
export function getEntityName(entity: any, eagerCacheStores?: any): string {
	// Both Pool and ScaleSet objects now include the name fields directly
	if (entity.repo_name) return entity.repo_name;
	if (entity.org_name) return entity.org_name;
	if (entity.enterprise_name) return entity.enterprise_name;
	
	// Fallback to eager cache lookup if name fields are not available (older API or cached data)
	if (entity.repo_id && !entity.repo_name && eagerCacheStores?.repositories) {
		const repo = eagerCacheStores.repositories.find((r: any) => r.id === entity.repo_id);
		return repo ? `${repo.owner}/${repo.name}` : 'Unknown Entity';
	}
	if (entity.org_id && !entity.org_name && eagerCacheStores?.organizations) {
		const org = eagerCacheStores.organizations.find((o: any) => o.id === entity.org_id);
		return (org && org.name) ? org.name : 'Unknown Entity';
	}
	if (entity.enterprise_id && !entity.enterprise_name && eagerCacheStores?.enterprises) {
		const enterprise = eagerCacheStores.enterprises.find((e: any) => e.id === entity.enterprise_id);
		return (enterprise && enterprise.name) ? enterprise.name : 'Unknown Entity';
	}
	
	return 'Unknown Entity';
}

/**
 * Gets the entity type for a Pool or ScaleSet object
 */
export function getEntityType(entity: any): string {
	if (entity.repo_id) return 'repository';
	if (entity.org_id) return 'organization';
	if (entity.enterprise_id) return 'enterprise';
	return 'unknown';
}

/**
 * Gets the URL for an entity detail page
 */
export function getEntityUrl(entity: any): string {
	if (entity.repo_id) return resolve(`/repositories/${entity.repo_id}`);
	if (entity.org_id) return resolve(`/organizations/${entity.org_id}`);
	if (entity.enterprise_id) return resolve(`/enterprises/${entity.enterprise_id}`);
	return '#';
}

/**
 * Updates entity fields, preserving events and other non-API fields
 */
export function updateEntityFields(currentEntity: any, updatedFields: any): any {
	// Preserve only fields that are definitely not in the API response
	const { events: originalEvents } = currentEntity;
	
	// Use the API response as the primary source, add back preserved fields
	const result = {
		...updatedFields,
		events: originalEvents // Always preserve events since they're managed by websockets
	};
	
	return result;
}

/**
 * Scrolls to bottom of events container
 */
export function scrollToBottomEvents(eventsContainer: HTMLElement | null): void {
	if (eventsContainer) {
		eventsContainer.scrollTop = eventsContainer.scrollHeight;
	}
}

/**
 * Changes pagination page
 */
export function changePage(currentPage: number, targetPage: number, totalPages: number): number {
	if (targetPage >= 1 && targetPage <= totalPages) {
		return targetPage;
	}
	return currentPage;
}

/**
 * Changes items per page and resets to page 1
 */
export function changePerPage(newPerPage: number): { newPerPage: number, newCurrentPage: number } {
	return { newPerPage, newCurrentPage: 1 };
}

/**
 * Gets entity status badge information based on pool_manager_status
 */
export function getEntityStatusBadge(entity: any): { text: string, variant: 'success' | 'error' } {
	if (entity.pool_manager_status?.running) {
		return {
			text: 'Running',
			variant: 'success'
		};
	} else {
		return {
			text: 'Stopped',
			variant: 'error'
		};
	}
}

/**
 * Gets badge variant for enabled/disabled status
 */
export function getEnabledStatusBadge(enabled: boolean): { text: string, variant: 'success' | 'error' } {
	return {
		text: enabled ? 'Enabled' : 'Disabled',
		variant: enabled ? 'success' : 'error'
	};
}

/**
 * Gets badge variant for authentication type
 */
export function getAuthTypeBadge(authType: string): { text: string, variant: 'success' | 'info' } {
	return {
		text: authType === 'pat' ? 'PAT' : 'App',
		variant: authType === 'pat' ? 'success' : 'info'
	};
}

/**
 * Gets badge variant for event level
 */
export function getEventLevelBadge(level: string): { text: string, variant: 'success' | 'error' | 'warning' | 'info' } {
	const normalizedLevel = level.toLowerCase();
	switch (normalizedLevel) {
		case 'error':
			return { text: 'Error', variant: 'error' };
		case 'warning':
			return { text: 'Warning', variant: 'warning' };
		case 'info':
			return { text: 'Info', variant: 'info' };
		default:
			return { text: level, variant: 'info' };
	}
}

/**
 * Filters entities by search term, supporting different search field configurations
 */
export function filterEntities<T extends Record<string, any>>(
	entities: T[], 
	searchTerm: string, 
	searchFields: string[] | ((entity: T, eagerCache?: any) => string)
): T[] {
	if (!searchTerm.trim()) return entities;

	const lowercaseSearch = searchTerm.toLowerCase();

	return entities.filter(entity => {
		if (typeof searchFields === 'function') {
			// Custom search function (e.g., for pools/scalesets using getEntityName)
			const searchText = searchFields(entity);
			return searchText.toLowerCase().includes(lowercaseSearch);
		} else {
			// Field-based search
			return searchFields.some(field => {
				const value = entity[field];
				return value?.toString().toLowerCase().includes(lowercaseSearch);
			});
		}
	});
}

/**
 * Convenience function for filtering repositories (searches name and owner)
 */
export function filterRepositories<T extends { name?: string, owner?: string }>(repositories: T[], searchTerm: string): T[] {
	return filterEntities(repositories, searchTerm, ['name', 'owner']);
}

/**
 * Convenience function for filtering organizations/enterprises (searches name only)
 */
export function filterByName<T extends { name?: string }>(entities: T[], searchTerm: string): T[] {
	return filterEntities(entities, searchTerm, ['name']);
}

/**
 * Convenience function for filtering credentials (searches name, description, and endpoint name)
 */
export function filterCredentials<T extends { name?: string, description?: string, endpoint?: { name?: string } }>(credentials: T[], searchTerm: string): T[] {
	return filterEntities(credentials, searchTerm, (credential) => {
		const searchableText = [
			credential.name || '',
			credential.description || '',
			credential.endpoint?.name || ''
		].join(' ');
		return searchableText;
	});
}

/**
 * Convenience function for filtering endpoints (searches name, description, base_url, and api_base_url)
 */
export function filterEndpoints<T extends { name?: string, description?: string, base_url?: string, api_base_url?: string }>(endpoints: T[], searchTerm: string): T[] {
	return filterEntities(endpoints, searchTerm, ['name', 'description', 'base_url', 'api_base_url']);
}

/**
 * Pagination utility functions
 */
export interface PaginationState {
	currentPage: number;
	perPage: number;
	totalPages: number;
}

/**
 * Creates paginated slice of items
 */
export function paginateItems<T>(items: T[], currentPage: number, perPage: number): T[] {
	return items.slice(
		(currentPage - 1) * perPage,
		currentPage * perPage
	);
}

/**
 * Calculates total pages and adjusts current page if needed
 */
export function calculatePagination(totalItems: number, perPage: number, currentPage: number): PaginationState {
	const totalPages = Math.ceil(totalItems / perPage);
	const adjustedCurrentPage = (currentPage > totalPages && totalPages > 0) ? totalPages : currentPage;
	
	return {
		currentPage: adjustedCurrentPage,
		perPage,
		totalPages
	};
}

/**
 * Creates pagination info text (e.g., "Showing 1 to 25 of 100 results")
 */
export function getPaginationInfo(currentPage: number, perPage: number, totalItems: number): string {
	if (totalItems === 0) return 'No results';
	
	const start = (currentPage - 1) * perPage + 1;
	const end = Math.min(currentPage * perPage, totalItems);
	
	return `Showing ${start} to ${end} of ${totalItems} results`;
}

