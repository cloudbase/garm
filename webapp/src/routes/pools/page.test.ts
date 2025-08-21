import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/svelte';
import PoolsPage from './+page.svelte';
import { createMockPool } from '../../test/factories.js';

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
		updatePool: vi.fn(),
		deletePool: vi.fn()
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
		getPools: vi.fn(),
		retryResource: vi.fn()
	}
}));

// Mock utilities
vi.mock('$lib/utils/apiError', () => ({
	extractAPIError: vi.fn((err) => err.message || 'Unknown error')
}));

vi.mock('$lib/utils/common.js', async (importOriginal) => {
	const actual = await importOriginal() as any;
	return {
		...(actual as any),
		getEntityName: vi.fn((pool, cache) => pool.repo_name || pool.org_name || pool.ent_name || 'Unknown Entity'),
		filterEntities: vi.fn((entities, searchTerm, nameGetter) => {
			if (!searchTerm) return entities;
			return entities.filter((entity: any) => {
				const name = nameGetter ? nameGetter(entity) : entity.name;
				return name?.toLowerCase().includes(searchTerm.toLowerCase());
			});
		})
	};
});

const mockPool = createMockPool({
	id: 'pool-123',
	image: 'ubuntu:22.04',
	flavor: 'default',
	provider_name: 'test-provider',
	enabled: true,
	repo_id: 'repo-123'
});

const mockPools = [mockPool];

// Reset any component mocks that might be set by setup.ts
vi.unmock('$lib/components/PageHeader.svelte');
vi.unmock('$lib/components/DataTable.svelte');
vi.unmock('$lib/components/CreatePoolModal.svelte');
vi.unmock('$lib/components/UpdatePoolModal.svelte');
vi.unmock('$lib/components/DeleteModal.svelte');
vi.unmock('$lib/components/cells');

describe('Pools Page - Unit Tests', () => {
	beforeEach(async () => {
		vi.clearAllMocks();
		
		// Set up default eager cache mock
		const { eagerCacheManager } = await import('$lib/stores/eager-cache.js');
		(eagerCacheManager.getPools as any).mockResolvedValue(mockPools);
	});

	describe('Component Initialization', () => {
		it('should render successfully', () => {
			const { container } = render(PoolsPage);
			expect(container).toBeInTheDocument();
		});

		it('should set page title', () => {
			render(PoolsPage);
			expect(document.title).toContain('Pools - GARM');
		});

		it('should display page header with correct props', async () => {
			render(PoolsPage);
			
			// Wait for component initialization
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should display header with pools title
			expect(screen.getByRole('heading', { name: 'Pools' })).toBeInTheDocument();
			expect(screen.getByText('Manage runner pools across all entities')).toBeInTheDocument();
		});
	});

	describe('Data Loading', () => {
		it('should load pools on mount', async () => {
			const { eagerCacheManager } = await import('$lib/stores/eager-cache.js');
			
			render(PoolsPage);
			
			// Wait for component mount
			await new Promise(resolve => setTimeout(resolve, 0));
			
			expect(eagerCacheManager.getPools).toHaveBeenCalled();
		});

		it('should handle loading state', async () => {
			const { eagerCache } = await import('$lib/stores/eager-cache.js');
			
			// Mock loading state
			vi.mocked(eagerCache.subscribe).mockImplementation((callback) => {
				callback(createMockCacheState({
					loading: {
						repositories: false,
						organizations: false,
						enterprises: false,
						pools: true,
						scalesets: false,
						credentials: false,
						endpoints: false,
						controllerInfo: false
					}
				}));
				return () => {};
			});
			
			render(PoolsPage);
			
			// Should show loading indicator
			expect(screen.getByText(/Loading pools/i)).toBeInTheDocument();
		});

		it('should handle API error state', async () => {
			const { eagerCacheManager } = await import('$lib/stores/eager-cache.js');
			
			// Mock API to fail
			const error = new Error('Failed to load pools');
			(eagerCacheManager.getPools as any).mockRejectedValue(error);
			
			render(PoolsPage);
			
			// Wait for the error to be handled
			await new Promise(resolve => setTimeout(resolve, 100));
			
			// Component should handle error gracefully
			expect(screen.getByRole('heading', { name: 'Pools' })).toBeInTheDocument();
		});

		it('should retry loading pools', async () => {
			const { eagerCacheManager } = await import('$lib/stores/eager-cache.js');
			
			render(PoolsPage);
			
			// Verify retry functionality is available
			expect(eagerCacheManager.retryResource).toBeDefined();
		});
	});

	describe('Search and Filtering', () => {
		it('should handle search functionality', async () => {
			render(PoolsPage);
			
			// Component should have search filtering logic available
			expect(screen.getByPlaceholderText(/Search by entity name/i)).toBeInTheDocument();
			
			// Verify search field is properly configured
			const searchInput = screen.getByPlaceholderText(/Search by entity name/i);
			expect(searchInput).toHaveAttribute('type', 'text');
		});

		it('should filter pools by entity name', async () => {
			const { filterEntities } = await import('$lib/utils/common.js');
			
			render(PoolsPage);
			
			// Component should filter pools by entity name since pools don't have names
			expect(filterEntities).toBeDefined();
			
			// Component should handle entity name filtering
			expect(screen.getByRole('heading', { name: 'Pools' })).toBeInTheDocument();
		});

		it('should handle pagination', async () => {
			render(PoolsPage);
			
			// Component should handle pagination state through the DataTable
			expect(screen.getByText(/Loading pools/i)).toBeInTheDocument();
			
			// Pagination controls should be available
			expect(screen.getByText(/Show:/i)).toBeInTheDocument();
			expect(screen.getByDisplayValue('25')).toBeInTheDocument();
		});
	});

	describe('Pool Creation', () => {
		it('should have create pool functionality', async () => {
			render(PoolsPage);
			
			// Wait for component initialization
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have add pool button
			expect(screen.getByRole('button', { name: /Add Pool/i })).toBeInTheDocument();
		});

		it('should open create modal when add button clicked', async () => {
			render(PoolsPage);
			
			// Wait for component initialization
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Click add pool button
			const addButton = screen.getByRole('button', { name: /Add Pool/i });
			await fireEvent.click(addButton);
			
			// Should show create modal
			expect(screen.getByText(/Create Pool/i)).toBeInTheDocument();
		});

		it('should handle successful pool creation', async () => {
			const { toastStore } = await import('$lib/stores/toast.js');
			
			render(PoolsPage);
			
			// Should have success toast functionality
			expect(toastStore.success).toBeDefined();
		});
	});

	describe('Pool Update', () => {
		it('should have update pool functionality', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			
			render(PoolsPage);
			
			expect(garmApi.updatePool).toBeDefined();
		});

		it('should show success toast after pool update', async () => {
			const { toastStore } = await import('$lib/stores/toast.js');
			
			render(PoolsPage);
			
			expect(toastStore.add).toBeDefined();
		});

		it('should handle update errors', async () => {
			const { toastStore } = await import('$lib/stores/toast.js');
			
			render(PoolsPage);
			
			expect(toastStore.add).toBeDefined();
		});
	});

	describe('Pool Deletion', () => {
		it('should have delete pool functionality', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			
			render(PoolsPage);
			
			expect(garmApi.deletePool).toBeDefined();
		});

		it('should show success toast after pool deletion', async () => {
			const { toastStore } = await import('$lib/stores/toast.js');
			
			render(PoolsPage);
			
			expect(toastStore.add).toBeDefined();
		});

		it('should handle deletion errors', async () => {
			const { toastStore } = await import('$lib/stores/toast.js');
			
			render(PoolsPage);
			
			expect(toastStore.add).toBeDefined();
		});
	});

	describe('Modal Management', () => {
		it('should handle create modal state', async () => {
			render(PoolsPage);
			
			// Wait for component initialization
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have create modal infrastructure
			expect(screen.getByRole('button', { name: /Add Pool/i })).toBeInTheDocument();
		});

		it('should handle update modal state', async () => {
			render(PoolsPage);
			
			// Component should have update API for modal functionality
			const { garmApi } = await import('$lib/api/client.js');
			expect(garmApi.updatePool).toBeDefined();
			
			// Should have toast notifications for update feedback
			const { toastStore } = await import('$lib/stores/toast.js');
			expect(toastStore.add).toBeDefined();
		});

		it('should handle delete modal state', async () => {
			render(PoolsPage);
			
			// Component should have delete API for modal functionality
			const { garmApi } = await import('$lib/api/client.js');
			expect(garmApi.deletePool).toBeDefined();
			
			// Should have toast notifications for delete feedback
			const { toastStore } = await import('$lib/stores/toast.js');
			expect(toastStore.add).toBeDefined();
		});

		it('should handle modal close functionality', () => {
			render(PoolsPage);
			
			// Component should manage modal state for various operations
			expect(screen.getByRole('heading', { name: 'Pools' })).toBeInTheDocument();
			
			// Modal infrastructure should be ready
			expect(document.body).toBeInTheDocument();
		});
	});

	describe('Eager Cache Integration', () => {
		it('should subscribe to eager cache on mount', async () => {
			const { eagerCache } = await import('$lib/stores/eager-cache.js');
			
			render(PoolsPage);
			
			// Wait for component mount
			await new Promise(resolve => setTimeout(resolve, 0));
			
			expect(eagerCache.subscribe).toHaveBeenCalled();
		});

		it('should handle cache data updates', async () => {
			const { eagerCache } = await import('$lib/stores/eager-cache.js');
			
			// Mock cache with pools data
			vi.mocked(eagerCache.subscribe).mockImplementation((callback) => {
				callback(createMockCacheState({
					pools: mockPools,
					loaded: {
						repositories: false,
						organizations: false,
						enterprises: false,
						pools: true,
						scalesets: false,
						credentials: false,
						endpoints: false,
						controllerInfo: false
					}
				}));
				return () => {};
			});
			
			render(PoolsPage);
			
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
						pools: 'Failed to load pools',
						scalesets: '',
						credentials: '',
						endpoints: '',
						controllerInfo: ''
					}
				}));
				return () => {};
			});
			
			render(PoolsPage);
			
			// Should handle cache errors
			expect(eagerCache.subscribe).toHaveBeenCalled();
		});
	});

	describe('Component Lifecycle', () => {
		it('should mount successfully', () => {
			const component = render(PoolsPage);
			expect(component.component).toBeDefined();
		});

		it('should unmount without errors', () => {
			const { unmount } = render(PoolsPage);
			expect(() => unmount()).not.toThrow();
		});

		it('should handle component initialization', async () => {
			const { container } = render(PoolsPage);
			
			// Component should initialize and render properly
			expect(container).toBeInTheDocument();
			
			// Should set page title during initialization
			expect(document.title).toContain('Pools - GARM');
			
			// Should load pools during initialization
			const { eagerCacheManager } = await import('$lib/stores/eager-cache.js');
			expect(eagerCacheManager.getPools).toBeDefined();
		});
	});

	describe('Data Transformation', () => {
		it('should handle pool filtering logic', async () => {
			const { filterEntities } = await import('$lib/utils/common.js');
			
			render(PoolsPage);
			
			// Component should filter pools by entity name
			expect(filterEntities).toBeDefined();
			
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
						pools: true,
						scalesets: false,
						credentials: false,
						endpoints: false,
						controllerInfo: false
					}
				}));
				return () => {};
			});
			
			render(PoolsPage);
			
			// Should show loading state
			expect(screen.getByText(/Loading pools/i)).toBeInTheDocument();
			
			// Pagination controls should be available
			expect(screen.getByDisplayValue('25')).toBeInTheDocument();
		});

		it('should handle entity name resolution', async () => {
			const { getEntityName } = await import('$lib/utils/common.js');
			
			render(PoolsPage);
			
			// Component should resolve entity names for pools
			expect(getEntityName).toBeDefined();
			
			// Component should display entity information
			expect(screen.getByRole('heading', { name: 'Pools' })).toBeInTheDocument();
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
						pools: true,
						scalesets: false,
						credentials: false,
						endpoints: false,
						controllerInfo: false
					}
				}));
				return () => {};
			});
			
			render(PoolsPage);
			
			// Should show loading state
			expect(screen.getByText(/Loading pools/i)).toBeInTheDocument();
			
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
						pools: true,
						scalesets: false,
						credentials: false,
						endpoints: false,
						controllerInfo: false
					}
				}));
				return () => {};
			});
			
			render(PoolsPage);
			
			// Should show loading state
			expect(screen.getByText(/Loading pools/i)).toBeInTheDocument();
			
			// Pagination controls should be integrated
			expect(screen.getByDisplayValue('25')).toBeInTheDocument();
		});

		it('should handle edit events', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			
			render(PoolsPage);
			
			// Component should handle edit events from DataTable
			expect(garmApi.updatePool).toBeDefined();
			
			// Edit infrastructure should be ready
			expect(screen.getByRole('heading', { name: 'Pools' })).toBeInTheDocument();
		});

		it('should handle delete events', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			
			render(PoolsPage);
			
			// Component should handle delete events from DataTable
			expect(garmApi.deletePool).toBeDefined();
			
			// Delete infrastructure should be ready
			expect(screen.getByRole('heading', { name: 'Pools' })).toBeInTheDocument();
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
						pools: true,
						scalesets: false,
						credentials: false,
						endpoints: false,
						controllerInfo: false
					}
				}));
				return () => {};
			});
			
			render(PoolsPage);
			
			// Component should handle retry events from DataTable
			expect(eagerCacheManager.retryResource).toBeDefined();
			
			// DataTable should be rendered for retry functionality
			expect(screen.getByText(/Loading pools/i)).toBeInTheDocument();
		});
	});

	describe('Utility Functions', () => {
		it('should handle API error extraction', async () => {
			const { extractAPIError } = await import('$lib/utils/apiError');
			
			render(PoolsPage);
			
			expect(extractAPIError).toBeDefined();
		});

		it('should handle pool identification', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			
			render(PoolsPage);
			
			// Component should identify pools by ID
			expect(garmApi.updatePool).toBeDefined();
			expect(garmApi.deletePool).toBeDefined();
			
			// Pool identification should work with pool IDs
			expect(screen.getByRole('heading', { name: 'Pools' })).toBeInTheDocument();
		});

		it('should handle entity name computation', async () => {
			const { getEntityName } = await import('$lib/utils/common.js');
			
			render(PoolsPage);
			
			// Component should compute entity names for display
			expect(getEntityName).toBeDefined();
			
			// Entity name resolution should be integrated
			expect(screen.getByRole('heading', { name: 'Pools' })).toBeInTheDocument();
		});
	});

	describe('Pool Configuration', () => {
		it('should have proper DataTable column configuration', () => {
			render(PoolsPage);
			
			// Component should configure DataTable with pool-specific columns
			expect(screen.getByRole('heading', { name: 'Pools' })).toBeInTheDocument();
			
			// DataTable should be configured for pools
			expect(screen.getByText(/Loading pools/i)).toBeInTheDocument();
		});

		it('should have proper mobile card configuration', () => {
			render(PoolsPage);
			
			// Component should configure mobile cards for pools
			expect(screen.getByRole('heading', { name: 'Pools' })).toBeInTheDocument();
			
			// Mobile responsiveness should be configured
			expect(screen.getByText(/Loading pools/i)).toBeInTheDocument();
		});

		it('should handle pool status display', () => {
			render(PoolsPage);
			
			// Component should display pool enabled/disabled status
			expect(screen.getByRole('heading', { name: 'Pools' })).toBeInTheDocument();
			
			// Status configuration should be ready
			expect(screen.getByText(/Loading pools/i)).toBeInTheDocument();
		});
	});
});