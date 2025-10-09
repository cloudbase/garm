import '@testing-library/jest-dom';

// Mock SvelteKit runtime modules
import { vi } from 'vitest';

// Mock SvelteKit stores
vi.mock('$app/stores', () => ({
	page: {
		subscribe: vi.fn(() => () => {})
	}
}));

// Mock SvelteKit paths
vi.mock('$app/paths', () => ({
	resolve: vi.fn((path: string) => path)
}));

// Mock SvelteKit environment - Set browser to true for client-side rendering
vi.mock('$app/environment', () => ({
	browser: true,
	dev: true,
	building: false,
	version: 'test'
}));

// Simple component mocks that render as basic divs
vi.mock('$lib/components/CreateRepositoryModal.svelte', () => ({
	default: function MockCreateRepositoryModal(anchor: any, props: any) {
		const target = anchor?.parentNode;
		if (target) {
			const div = document.createElement('div');
			div.setAttribute('data-testid', 'create-repository-modal');
			div.textContent = 'Create Repository Modal';
			target.appendChild(div);
		}
		return {
			$destroy: vi.fn(),
			$set: vi.fn(),
			$on: vi.fn()
		};
	}
}));

vi.mock('$lib/components/UpdateEntityModal.svelte', () => ({
	default: function MockUpdateEntityModal(anchor: any, props: any) {
		const target = anchor?.parentNode;
		if (target) {
			const div = document.createElement('div');
			div.setAttribute('data-testid', 'update-entity-modal');
			div.textContent = 'Update Entity Modal';
			target.appendChild(div);
		}
		return {
			$destroy: vi.fn(),
			$set: vi.fn(),
			$on: vi.fn()
		};
	}
}));

// DeleteModal is NOT mocked - use real component
// Modal content and buttons need to work correctly
/*
vi.mock('$lib/components/DeleteModal.svelte', () => ({
	default: function MockDeleteModal(anchor: any, props: any) {
		const target = anchor?.parentNode;
		if (target) {
			const div = document.createElement('div');
			div.setAttribute('data-testid', 'delete-modal');
			div.textContent = 'Delete Modal';
			target.appendChild(div);
		}
		return {
			$destroy: vi.fn(),
			$set: vi.fn(),
			$on: vi.fn()
		};
	}
}));
*/

// PageHeader is NOT mocked - use real component to support slots
// Slots don't work properly with mocked Svelte components
/*
vi.mock('$lib/components/PageHeader.svelte', () => ({
	default: function MockPageHeader(anchor: any, propsObj: any) {
		const target = anchor?.parentNode;
		if (target) {
			const div = document.createElement('div');
			div.setAttribute('data-testid', 'page-header');

			// Extract props
			const title = propsObj?.title || 'Runner Instances';
			const description = propsObj?.description || '';

			let html = `
				<div class="sm:flex sm:items-center sm:justify-between">
					<div>
						<h1 class="text-2xl font-bold text-gray-900 dark:text-white">${title}</h1>
						<p class="mt-2 text-sm text-gray-700 dark:text-gray-300">${description}</p>
					</div>
				</div>
			`;
			div.innerHTML = html;
			target.appendChild(div);
		}
		return {
			$destroy: vi.fn(),
			$set: vi.fn(),
			$on: vi.fn()
		};
	}
}));
*/

// NOTE: DataTable is commented out to allow real component in tests
// This is necessary because reactive props don't work well with mocks
/*
vi.mock('$lib/components/DataTable.svelte', () => ({
	default: function MockDataTable(anchor: any, propsArg: any) {
		const target = anchor?.parentNode;
		const props = propsArg || {};
		let searchInput: HTMLInputElement | null = null;
		const eventListeners: Record<string, Array<(event: any) => void>> = {};

		// Cache resolved prop values to detect changes
		let cachedData: any[] = [];
		let cachedLoading = false;
		let cachedError = '';

		const resolveProps = () => {
			// Access getters to get current values
			const data = props.data || [];
			const loading = props.loading || false;
			const error = props.error || '';

			// Check if props changed
			if (data !== cachedData || data.length !== cachedData.length ||
				loading !== cachedLoading || error !== cachedError) {
				cachedData = data;
				cachedLoading = loading;
				cachedError = error;
				return { changed: true, data, loading, error };
			}
			return { changed: false, data, loading, error };
		};

		const renderContent = () => {
			if (!target) return;

			const div = target.querySelector('[data-testid="data-table"]') || document.createElement('div');
			div.setAttribute('data-testid', 'data-table');

			// Access getters to get current values
			const { data, loading, error } = resolveProps();
			const searchPlaceholder = props.searchPlaceholder || 'Search...';
			const columns = props.columns || [];
			const showSearch = props.showSearch !== false;
			const searchType = props.searchType || 'client';

			// Create table structure
			let html = '<div>';

			// Search bar
			if (showSearch) {
				html += `<div data-testid="${searchType}-search-bar">`;
				html += `<input type="search" placeholder="${searchPlaceholder}" data-testid="search-input" class="search-input" />`;
				if (searchType === 'backend') {
					html += `<button class="search-button">Search</button>`;
				}
				html += `</div>`;
			}

			// Loading state
			if (loading) {
				html += '<div data-testid="loading-state">Loading...</div>';
			}
			// Error state
			else if (error) {
				html += `<div data-testid="error-state">${error}</div>`;
			}
			// Table with data
			else if (data.length > 0 && columns.length > 0) {
				html += '<table data-testid="data-table-table"><thead><tr>';
				columns.forEach((col: any) => {
					html += `<th>${col.title || col.key}</th>`;
				});
				html += '</tr></thead><tbody>';
				data.forEach((item: any, index: number) => {
					html += `<tr data-testid="table-row-${index}">`;
					columns.forEach((col: any) => {
						const value = item[col.key] || '';
						html += `<td>${typeof value === 'object' ? JSON.stringify(value) : value}</td>`;
					});
					// Add action buttons if column has actions
					if (columns.some((c: any) => c.key === 'actions')) {
						html += `<td><button data-action="delete" data-index="${index}">Delete</button></td>`;
					}
					html += '</tr>';
				});
				html += '</tbody></table>';
			}
			// Empty state
			else if (!loading && !error) {
				html += '<div data-testid="empty-state">No objects found</div>';
			}

			html += '</div>';
			div.innerHTML = html;

			if (!div.parentNode) {
				target.appendChild(div);
			}

			// Attach event listeners to search input
			searchInput = div.querySelector('.search-input') as HTMLInputElement;
			if (searchInput && eventListeners.search) {
				searchInput.addEventListener('input', (e) => {
					const term = (e.target as HTMLInputElement).value;
					eventListeners.search?.forEach(cb => cb({ detail: { term } }));
				});
			}

			// Attach event listeners to delete buttons
			const deleteButtons = div.querySelectorAll('[data-action="delete"]');
			deleteButtons.forEach((btn) => {
				btn.addEventListener('click', (e) => {
					const index = parseInt((e.target as HTMLElement).getAttribute('data-index') || '0');
					const item = data[index];
					eventListeners.delete?.forEach(cb => cb({ detail: { item } }));
				});
			});
		};

		// Initial render
		renderContent();

		return {
			$destroy: vi.fn(() => {
				if (target) {
					const div = target.querySelector('[data-testid="data-table"]');
					if (div) div.remove();
				}
			}),
			$set: vi.fn((newProps: any) => {
				console.log('[DataTable $set] newProps:', newProps);
				console.log('[DataTable $set] newProps.data:', newProps?.data);
				console.log('[DataTable $set] newProps.loading:', newProps?.loading);
				Object.assign(props, newProps);
				renderContent();
			}),
			$on: vi.fn((event: string, callback: (e: any) => void) => {
				if (!eventListeners[event]) {
					eventListeners[event] = [];
				}
				eventListeners[event].push(callback);
				return () => {
					const index = eventListeners[event]?.indexOf(callback);
					if (index !== undefined && index > -1) {
						eventListeners[event].splice(index, 1);
					}
				};
			})
		};
	}
}));
*/

// Cell components are NOT mocked - use real components
// Mocked cells don't render actual data values
/*
vi.mock('$lib/components/cells', () => ({
	EntityCell: function MockEntityCell(anchor: any, props: any) {
		const target = anchor?.parentNode;
		if (target) {
			const div = document.createElement('div');
			div.setAttribute('data-testid', 'entity-cell');
			div.textContent = props?.value || 'Entity Cell';
			target.appendChild(div);
		}
		return { $destroy: vi.fn(), $set: vi.fn(), $on: vi.fn() };
	},
	EndpointCell: function MockEndpointCell(anchor: any, props: any) {
		const target = anchor?.parentNode;
		if (target) {
			const div = document.createElement('div');
			div.setAttribute('data-testid', 'endpoint-cell');
			div.textContent = props?.value || 'Endpoint Cell';
			target.appendChild(div);
		}
		return { $destroy: vi.fn(), $set: vi.fn(), $on: vi.fn() };
	},
	StatusCell: function MockStatusCell(anchor: any, props: any) {
		const target = anchor?.parentNode;
		if (target) {
			const div = document.createElement('div');
			div.setAttribute('data-testid', 'status-cell');
			div.textContent = props?.value || 'Status Cell';
			target.appendChild(div);
		}
		return { $destroy: vi.fn(), $set: vi.fn(), $on: vi.fn() };
	},
	ActionsCell: function MockActionsCell(anchor: any, props: any) {
		const target = anchor?.parentNode;
		if (target) {
			const div = document.createElement('div');
			div.setAttribute('data-testid', 'actions-cell');
			div.textContent = 'Actions Cell';
			target.appendChild(div);
		}
		return { $destroy: vi.fn(), $set: vi.fn(), $on: vi.fn() };
	},
	GenericCell: function MockGenericCell(anchor: any, props: any) {
		const target = anchor?.parentNode;
		if (target) {
			const div = document.createElement('div');
			div.setAttribute('data-testid', 'generic-cell');
			div.textContent = props?.value || 'Generic Cell';
			target.appendChild(div);
		}
		return { $destroy: vi.fn(), $set: vi.fn(), $on: vi.fn() };
	},
	InstancePoolCell: function MockInstancePoolCell(anchor: any, props: any) {
		const target = anchor?.parentNode;
		if (target) {
			const div = document.createElement('div');
			div.setAttribute('data-testid', 'instance-pool-cell');
			div.textContent = props?.value || 'Instance Pool Cell';
			target.appendChild(div);
		}
		return { $destroy: vi.fn(), $set: vi.fn(), $on: vi.fn() };
	},
	TagsCell: function MockTagsCell(anchor: any, props: any) {
		const target = anchor?.parentNode;
		if (target) {
			const div = document.createElement('div');
			div.setAttribute('data-testid', 'tags-cell');
			div.textContent = props?.tags?.join(', ') || 'Tags Cell';
			target.appendChild(div);
		}
		return { $destroy: vi.fn(), $set: vi.fn(), $on: vi.fn() };
	}
}));
*/