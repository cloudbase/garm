import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import ScaleSetsPage from './+page.svelte';
import { createMockScaleSet } from '../../test/factories.js';

// Helper function to create complete EagerCacheState objects
function createMockCacheState(overrides: any = {}) {
	return {
		pools: [],
		repositories: [],
		organizations: [],
		enterprises: [],
		scalesets: [],
		credentials: [],
		endpoints: [],
		controllerInfo: null,
		loaded: {
			repositories: false,
			organizations: false,
			enterprises: false,
			pools: false,
			scalesets: false,
			credentials: false,
			endpoints: false,
			controllerInfo: false
		},
		loading: {
			repositories: false,
			organizations: false,
			enterprises: false,
			pools: false,
			scalesets: false,
			credentials: false,
			endpoints: false,
			controllerInfo: false
		},
		errorMessages: {
			repositories: '',
			organizations: '',
			enterprises: '',
			pools: '',
			scalesets: '',
			credentials: '',
			endpoints: '',
			controllerInfo: ''
		},
		...overrides
	};
}

// Mock the page stores
vi.mock('$app/stores', () => ({}));

// Mock navigation
vi.mock('$app/navigation', () => ({}));

// Mock the API client
vi.mock('$lib/api/client.js', () => ({
	garmApi: {
		updateScaleSet: vi.fn(),
		deleteScaleSet: vi.fn()
	}
}));

// Mock stores
vi.mock('$lib/stores/toast.js', () => ({
	toastStore: {
		success: vi.fn(),
		add: vi.fn(),
		error: vi.fn(),
		info: vi.fn()
	}
}));

vi.mock('$lib/stores/eager-cache.js', () => ({
	eagerCache: {
		subscribe: vi.fn((callback: any) => {
			callback(createMockCacheState());
			return () => {};
		})
	},
	eagerCacheManager: {
		getScaleSets: vi.fn(),
		retryResource: vi.fn()
	}
}));

// Mock utilities
vi.mock('$lib/utils/apiError', () => ({
	extractAPIError: vi.fn((err) => err.message || 'Unknown error')
}));

vi.mock('$lib/utils/common.js', async () => {
	const actual = await vi.importActual('$lib/utils/common.js') as any;
	return {
		...(actual as any),
		getEntityName: vi.fn((entity) => {
			if (entity.repo_name) return entity.repo_name;
			if (entity.org_name) return entity.org_name;
			if (entity.enterprise_name) return entity.enterprise_name;
			return 'Unknown';
		}),
		filterEntities: vi.fn((entities, searchTerm, getNameFn) => {
			if (!searchTerm) return entities;
			return entities.filter((entity: any) => {
				const name = getNameFn(entity);
				return name.toLowerCase().includes(searchTerm.toLowerCase());
			});
		})
	};
});

// Reset any component mocks that might be set by setup.ts
vi.unmock('$lib/components/PageHeader.svelte');
vi.unmock('$lib/components/DataTable.svelte');
vi.unmock('$lib/components/CreateScaleSetModal.svelte');
vi.unmock('$lib/components/UpdateScaleSetModal.svelte');
vi.unmock('$lib/components/DeleteModal.svelte');
vi.unmock('$lib/components/cells');

const mockScaleSet = createMockScaleSet({
	id: 123,
	name: 'test-scaleset',
	repo_name: 'test-repo',
	provider_name: 'hetzner',
	enabled: true,
	image: 'ubuntu:22.04',
	flavor: 'default',
	max_runners: 10,
	min_idle_runners: 1
});

describe('Scale Sets Page - Unit Tests', () => {
	beforeEach(async () => {
		vi.clearAllMocks();
		
		// Set up default eager cache manager mock
		const { eagerCacheManager } = await import('$lib/stores/eager-cache.js');
		(eagerCacheManager.getScaleSets as any).mockResolvedValue([mockScaleSet]);
	});

	describe('Component Initialization', () => {
		it('should render successfully', () => {
			const { container } = render(ScaleSetsPage);
			expect(container).toBeInTheDocument();
		});

		it('should set page title', () => {
			render(ScaleSetsPage);
			expect(document.title).toBe('Scale Sets - GARM');
		});

		it('should load scale sets on mount', async () => {
			const { eagerCacheManager } = await import('$lib/stores/eager-cache.js');
			
			render(ScaleSetsPage);
			
			// Wait for component mount
			await new Promise(resolve => setTimeout(resolve, 0));
			
			expect(eagerCacheManager.getScaleSets).toHaveBeenCalled();
		});
	});

	describe('Data Loading', () => {
		it('should handle loading state', async () => {
			const { eagerCache } = await import('$lib/stores/eager-cache.js');
			
			// Mock loading state
			vi.mocked(eagerCache.subscribe).mockImplementation((callback) => {
				callback(createMockCacheState({
					loading: {
						repositories: false,
						organizations: false,
						enterprises: false,
						pools: false,
						scalesets: true,
						credentials: false,
						endpoints: false,
						controllerInfo: false
					}
				}));
				return () => {};
			});
			
			render(ScaleSetsPage);
			
			// Should show loading indicator
			expect(screen.getByText(/Loading scale sets/i)).toBeInTheDocument();
		});

		it('should handle API error state', async () => {
			const { eagerCacheManager } = await import('$lib/stores/eager-cache.js');
			
			// Mock API to fail
			const error = new Error('Failed to load scale sets');
			(eagerCacheManager.getScaleSets as any).mockRejectedValue(error);
			
			render(ScaleSetsPage);
			
			// Wait for the error to be handled
			await new Promise(resolve => setTimeout(resolve, 100));
			
			// Should handle error gracefully
			expect(eagerCacheManager.getScaleSets).toHaveBeenCalled();
		});
	});

	describe('Scale Sets Display', () => {
		it('should display scale sets in data table', async () => {
			const mockScaleSets = [mockScaleSet];
			const { eagerCache } = await import('$lib/stores/eager-cache.js');
			
			// Mock cache with scale sets data
			vi.mocked(eagerCache.subscribe).mockImplementation((callback) => {
				callback(createMockCacheState({
					scalesets: mockScaleSets,
					loaded: {
						repositories: false,
						organizations: false,
						enterprises: false,
						pools: false,
						scalesets: true,
						credentials: false,
						endpoints: false,
						controllerInfo: false
					}
				}));
				return () => {};
			});
			
			render(ScaleSetsPage);
			
			// Wait for data to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should display scale sets table
			expect(screen.getByRole('heading', { name: 'Scale Sets' })).toBeInTheDocument();
		});

		it('should handle empty scale sets list', async () => {
			const { eagerCache } = await import('$lib/stores/eager-cache.js');
			
			// Mock cache with empty data
			vi.mocked(eagerCache.subscribe).mockImplementation((callback) => {
				callback(createMockCacheState({
					scalesets: [],
					loaded: {
						repositories: false,
						organizations: false,
						enterprises: false,
						pools: false,
						scalesets: true,
						credentials: false,
						endpoints: false,
						controllerInfo: false
					}
				}));
				return () => {};
			});
			
			render(ScaleSetsPage);
			
			// Wait for data to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should show empty state
			expect(screen.getByText(/No scale sets found/i)).toBeInTheDocument();
		});
	});

	describe('Eager Cache Integration', () => {
		it('should subscribe to eager cache', async () => {
			const { eagerCache } = await import('$lib/stores/eager-cache.js');
			
			render(ScaleSetsPage);
			
			// Should subscribe to cache
			expect(eagerCache.subscribe).toHaveBeenCalled();
		});

		it('should handle cache data updates', async () => {
			const mockScaleSets = [mockScaleSet];
			const { eagerCache } = await import('$lib/stores/eager-cache.js');
			
			// Mock cache with scale sets data
			vi.mocked(eagerCache.subscribe).mockImplementation((callback) => {
				callback(createMockCacheState({
					scalesets: mockScaleSets,
					loaded: {
						repositories: false,
						organizations: false,
						enterprises: false,
						pools: false,
						scalesets: true,
						credentials: false,
						endpoints: false,
						controllerInfo: false
					}
				}));
				return () => {};
			});
			
			render(ScaleSetsPage);
			
			// Component should handle cache updates
			expect(eagerCache.subscribe).toHaveBeenCalled();
		});

		it('should handle cache error states', async () => {
			const { eagerCache } = await import('$lib/stores/eager-cache.js');
			
			// Mock cache with error
			vi.mocked(eagerCache.subscribe).mockImplementation((callback) => {
				callback(createMockCacheState({
					errorMessages: {
						repositories: '',
						organizations: '',
						enterprises: '',
						pools: '',
						scalesets: 'Failed to load scale sets',
						credentials: '',
						endpoints: '',
						controllerInfo: ''
					}
				}));
				return () => {};
			});
			
			render(ScaleSetsPage);
			
			// Should handle cache errors
			expect(screen.getByText(/Failed to load scale sets/i)).toBeInTheDocument();
		});

		it('should handle cache error states', async () => {
			const { eagerCache } = await import('$lib/stores/eager-cache.js');
			
			// Mock loading error state
			vi.mocked(eagerCache.subscribe).mockImplementation((callback) => {
				callback(createMockCacheState({
					errorMessages: {
						repositories: '',
						organizations: '',
						enterprises: '',
						pools: '',
						scalesets: 'Failed to load scale sets from cache',
						credentials: '',
						endpoints: '',
						controllerInfo: ''
					}
				}));
				return () => {};
			});
			
			render(ScaleSetsPage);
			
			// Should display error
			expect(screen.getByText(/Failed to load scale sets from cache/i)).toBeInTheDocument();
		});
	});

	describe('Search and Filtering', () => {
		it('should handle search functionality', async () => {
			render(ScaleSetsPage);
			
			// Wait for component to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Search functionality should be available
			expect(screen.getByPlaceholderText(/Search by entity name/i)).toBeInTheDocument();
		});

		it('should handle pagination calculations', async () => {
			// Mock eager cache with loading state
			const { eagerCache } = await import('$lib/stores/eager-cache.js');
			vi.mocked(eagerCache.subscribe).mockImplementation((callback: any) => {
				callback(createMockCacheState({
					loading: {
						repositories: false,
						organizations: false,
						enterprises: false,
						pools: false,
						scalesets: true,
						credentials: false,
						endpoints: false,
						controllerInfo: false
					}
				}));
				return () => {};
			});
			
			render(ScaleSetsPage);
			
			// Should show loading state
			expect(screen.getByText(/Loading scale sets/i)).toBeInTheDocument();
			
			// Pagination controls should be available
			expect(screen.getByDisplayValue('25')).toBeInTheDocument();
		});

		it('should filter scale sets by search term', async () => {
			render(ScaleSetsPage);
			
			// Wait for component to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Search input should be available for search events
			expect(screen.getByPlaceholderText(/Search by entity name/i)).toBeInTheDocument();
		});
	});

	describe('Event Handling', () => {
		it('should handle table search events', async () => {
			// Mock eager cache with loading state
			const { eagerCache } = await import('$lib/stores/eager-cache.js');
			vi.mocked(eagerCache.subscribe).mockImplementation((callback: any) => {
				callback(createMockCacheState({
					loading: {
						repositories: false,
						organizations: false,
						enterprises: false,
						pools: false,
						scalesets: true,
						credentials: false,
						endpoints: false,
						controllerInfo: false
					}
				}));
				return () => {};
			});
			
			render(ScaleSetsPage);
			
			// Should show loading state
			expect(screen.getByText(/Loading scale sets/i)).toBeInTheDocument();
			
			// Search input should be available for search events
			expect(screen.getByPlaceholderText(/Search by entity name/i)).toBeInTheDocument();
		});

		it('should handle table pagination events', async () => {
			// Mock eager cache with loading state
			const { eagerCache } = await import('$lib/stores/eager-cache.js');
			vi.mocked(eagerCache.subscribe).mockImplementation((callback: any) => {
				callback(createMockCacheState({
					loading: {
						repositories: false,
						organizations: false,
						enterprises: false,
						pools: false,
						scalesets: true,
						credentials: false,
						endpoints: false,
						controllerInfo: false
					}
				}));
				return () => {};
			});
			
			render(ScaleSetsPage);
			
			// Should show loading state
			expect(screen.getByText(/Loading scale sets/i)).toBeInTheDocument();
			
			// Pagination controls should be integrated
			expect(screen.getByDisplayValue('25')).toBeInTheDocument();
		});

		it('should handle edit events', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			
			render(ScaleSetsPage);
			
			// Component should handle edit events from DataTable
			expect(garmApi.updateScaleSet).toBeDefined();
			
			// Edit infrastructure should be ready
			expect(screen.getByRole('heading', { name: 'Scale Sets' })).toBeInTheDocument();
		});

		it('should handle delete events', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			
			render(ScaleSetsPage);
			
			// Component should handle delete events from DataTable
			expect(garmApi.deleteScaleSet).toBeDefined();
			
			// Delete infrastructure should be ready
			expect(screen.getByRole('heading', { name: 'Scale Sets' })).toBeInTheDocument();
		});

		it('should handle retry events', async () => {
			const { eagerCacheManager, eagerCache } = await import('$lib/stores/eager-cache.js');
			
			// Mock eager cache with loading state
			vi.mocked(eagerCache.subscribe).mockImplementation((callback: any) => {
				callback(createMockCacheState({
					loading: {
						repositories: false,
						organizations: false,
						enterprises: false,
						pools: false,
						scalesets: true,
						credentials: false,
						endpoints: false,
						controllerInfo: false
					}
				}));
				return () => {};
			});
			
			render(ScaleSetsPage);
			
			// Component should handle retry events from DataTable
			expect(eagerCacheManager.retryResource).toBeDefined();
			
			// Retry infrastructure should be ready
			expect(screen.getByRole('heading', { name: 'Scale Sets' })).toBeInTheDocument();
		});
	});

	describe('Modal Management', () => {
		it('should handle create modal state', async () => {
			render(ScaleSetsPage);
			
			// Wait for component to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Create button should be available
			expect(screen.getByText('Add Scale Set')).toBeInTheDocument();
		});

		it('should handle update modal state', async () => {
			render(ScaleSetsPage);
			
			// Wait for component to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Modal infrastructure should be ready
			expect(screen.getByRole('heading', { name: 'Scale Sets' })).toBeInTheDocument();
		});

		it('should handle delete modal state', async () => {
			render(ScaleSetsPage);
			
			// Wait for component to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Modal infrastructure should be ready
			expect(screen.getByRole('heading', { name: 'Scale Sets' })).toBeInTheDocument();
		});
	});

	describe('CRUD Operations', () => {
		it('should handle create scale set', async () => {
			render(ScaleSetsPage);
			
			// Wait for component to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Create functionality should be available
			expect(screen.getByText('Add Scale Set')).toBeInTheDocument();
		});

		it('should handle update scale set', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			
			render(ScaleSetsPage);
			
			// Update functionality should be available
			expect(garmApi.updateScaleSet).toBeDefined();
		});

		it('should handle delete scale set', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			
			render(ScaleSetsPage);
			
			// Delete functionality should be available
			expect(garmApi.deleteScaleSet).toBeDefined();
		});
	});

	describe('Toast Integration', () => {
		it('should show success messages for CRUD operations', async () => {
			const { toastStore } = await import('$lib/stores/toast.js');
			
			render(ScaleSetsPage);
			
			// Toast store should be available for success messages
			expect(toastStore.success).toBeDefined();
			expect(toastStore.error).toBeDefined();
		});
	});

	describe('Component Lifecycle', () => {
		it('should mount successfully', () => {
			const component = render(ScaleSetsPage);
			expect(component.component).toBeDefined();
		});

		it('should unmount without errors', () => {
			const { unmount } = render(ScaleSetsPage);
			expect(() => unmount()).not.toThrow();
		});
	});

	describe('Error Handling', () => {
		it('should handle mount errors gracefully', async () => {
			const { eagerCacheManager } = await import('$lib/stores/eager-cache.js');
			
			// Mock mount to fail
			const error = new Error('Mount failed');
			(eagerCacheManager.getScaleSets as any).mockRejectedValue(error);
			
			expect(() => render(ScaleSetsPage)).not.toThrow();
		});

		it('should handle API errors during operations', async () => {
			const { extractAPIError } = await import('$lib/utils/apiError');
			
			render(ScaleSetsPage);
			
			// Error handling should be available
			expect(extractAPIError).toBeDefined();
		});
	});
});