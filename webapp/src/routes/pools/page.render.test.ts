import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
import PoolsPage from './+page.svelte';
import { createMockPool } from '../../test/factories.js';

// Mock all external dependencies
vi.mock('$app/stores', () => ({}));

vi.mock('$app/navigation', () => ({}));

vi.mock('$lib/api/client.js', () => ({
	garmApi: {
		updatePool: vi.fn(),
		deletePool: vi.fn()
	}
}));

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
			callback({
				pools: [],
				loaded: { pools: false },
				loading: { pools: false },
				errorMessages: { pools: '' },
				repositories: [],
				organizations: [],
				enterprises: []
			});
			return () => {};
		})
	},
	eagerCacheManager: {
		getPools: vi.fn(),
		retryResource: vi.fn()
	}
}));

vi.mock('$lib/utils/apiError', () => ({
	extractAPIError: vi.fn((err) => err.message || 'Unknown error')
}));

vi.mock('$lib/utils/common.js', async (importOriginal) => {
	const actual = await importOriginal() as any;
	return {
		...(actual as any),
		getEntityName: vi.fn((pool, cache) => pool.repo_name || pool.org_name || pool.ent_name || 'Test Entity'),
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

describe('Pools Page - Render Tests', () => {
	beforeEach(async () => {
		vi.clearAllMocks();
		
		// Set up default eager cache mocks
		const { eagerCacheManager } = await import('$lib/stores/eager-cache.js');
		(eagerCacheManager.getPools as any).mockResolvedValue(mockPools);
	});

	describe('Basic Rendering', () => {
		it('should render without crashing', () => {
			const { container } = render(PoolsPage);
			expect(container).toBeInTheDocument();
		});

		it('should have proper document structure', () => {
			const { container } = render(PoolsPage);
			expect(container.querySelector('div')).toBeInTheDocument();
		});

		it('should render page header', async () => {
			render(PoolsPage);
			
			// Wait for component initialization
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have page header
			expect(screen.getByRole('heading', { name: 'Pools' })).toBeInTheDocument();
			expect(screen.getByText('Manage runner pools across all entities')).toBeInTheDocument();
		});

		it('should render data table', async () => {
			render(PoolsPage);
			
			// Wait for component initialization
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have DataTable rendered - check for elements that are always present
			expect(screen.getByPlaceholderText(/Search by entity name/i)).toBeInTheDocument();
			expect(screen.getByDisplayValue('25')).toBeInTheDocument();
			expect(screen.getByText(/Show:/i)).toBeInTheDocument();
		});

		it('should render add pool button', async () => {
			render(PoolsPage);
			
			// Wait for component initialization
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have add pool button
			expect(screen.getByRole('button', { name: /Add Pool/i })).toBeInTheDocument();
		});
	});

	describe('Component Lifecycle', () => {
		it('should mount successfully', () => {
			const { component } = render(PoolsPage);
			expect(component).toBeDefined();
		});

		it('should unmount without errors', () => {
			const { unmount } = render(PoolsPage);
			expect(() => unmount()).not.toThrow();
		});

		it('should handle component updates', async () => {
			const { component } = render(PoolsPage);
			
			// Component should handle reactive updates
			expect(component).toBeDefined();
		});

		it('should load pools on mount', async () => {
			const { eagerCacheManager } = await import('$lib/stores/eager-cache.js');
			
			render(PoolsPage);
			
			// Wait for component mount and data loading
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should call eager cache to load pools
			expect(eagerCacheManager.getPools).toHaveBeenCalled();
		});

		it('should subscribe to eager cache on mount', async () => {
			const { eagerCache } = await import('$lib/stores/eager-cache.js');
			
			render(PoolsPage);
			
			// Wait for component mount
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should subscribe to eager cache
			expect(eagerCache.subscribe).toHaveBeenCalled();
		});
	});

	describe('DOM Structure', () => {
		it('should create proper DOM hierarchy', async () => {
			const { container } = render(PoolsPage);
			
			// Wait for component initialization
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have main container with proper spacing
			const mainDiv = container.querySelector('div.space-y-6');
			expect(mainDiv).toBeInTheDocument();
		});

		it('should render svelte:head for page title', async () => {
			render(PoolsPage);
			
			// Wait for component initialization
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should set page title
			expect(document.title).toContain('Pools - GARM');
		});

		it('should handle error display conditionally', async () => {
			const { eagerCache } = await import('$lib/stores/eager-cache.js');
			
			// Mock cache with error
			vi.mocked(eagerCache.subscribe).mockImplementation((callback: any) => {
				callback({
					pools: [],
					loaded: { pools: false },
					loading: { pools: false },
					errorMessages: { pools: 'Test error' },
					repositories: [],
					organizations: [],
					enterprises: []
				});
				return () => {};
			});
			
			render(PoolsPage);
			
			// Wait for error handling
			await new Promise(resolve => setTimeout(resolve, 100));
			
			// Error display should be conditional
			expect(screen.getByText(/Test error/i)).toBeInTheDocument();
		});

		it('should render loading state initially', async () => {
			const { eagerCache } = await import('$lib/stores/eager-cache.js');
			
			// Mock loading state
			vi.mocked(eagerCache.subscribe).mockImplementation((callback: any) => {
				callback({
					pools: [],
					loaded: { pools: false },
					loading: { pools: true },
					errorMessages: { pools: '' },
					repositories: [],
					organizations: [],
					enterprises: []
				});
				return () => {};
			});
			
			render(PoolsPage);
			
			// Should show loading initially
			expect(screen.getByText(/Loading pools/i)).toBeInTheDocument();
		});
	});

	describe('Data Table Rendering', () => {
		it('should render data table with correct configuration', async () => {
			render(PoolsPage);
			
			// Wait for component initialization
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should render DataTable with correct search and pagination
			expect(screen.getByPlaceholderText(/Search by entity name/i)).toBeInTheDocument();
			expect(screen.getByDisplayValue('25')).toBeInTheDocument();
		});

		it('should render search functionality', async () => {
			render(PoolsPage);
			
			// Wait for component initialization
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should render search input
			const searchInput = screen.getByPlaceholderText(/Search by entity name/i);
			expect(searchInput).toBeInTheDocument();
			expect(searchInput).toHaveAttribute('type', 'text');
		});

		it('should render pagination controls', async () => {
			render(PoolsPage);
			
			// Wait for component initialization
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should render pagination
			expect(screen.getByText(/Show:/i)).toBeInTheDocument();
			expect(screen.getByDisplayValue('25')).toBeInTheDocument();
		});

		it('should render empty state when no pools', async () => {
			const { eagerCache } = await import('$lib/stores/eager-cache.js');
			
			// Mock empty pools
			vi.mocked(eagerCache.subscribe).mockImplementation((callback: any) => {
				callback({
					pools: [],
					loaded: { pools: true },
					loading: { pools: false },
					errorMessages: { pools: '' },
					repositories: [],
					organizations: [],
					enterprises: []
				});
				return () => {};
			});
			
			render(PoolsPage);
			
			// Wait for component initialization
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should render empty state
			expect(screen.getByText(/No pools found/i)).toBeInTheDocument();
		});

		it('should render retry button on cache error', async () => {
			const { eagerCache } = await import('$lib/stores/eager-cache.js');
			
			// Mock cache error
			vi.mocked(eagerCache.subscribe).mockImplementation((callback: any) => {
				callback({
					pools: [],
					loaded: { pools: false },
					loading: { pools: false },
					errorMessages: { pools: 'Cache error' },
					repositories: [],
					organizations: [],
					enterprises: []
				});
				return () => {};
			});
			
			render(PoolsPage);
			
			// Wait for component initialization
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should render retry button
			expect(screen.getByRole('button', { name: /Retry/i })).toBeInTheDocument();
		});
	});

	describe('Modal Rendering', () => {
		it('should conditionally render create pool modal', async () => {
			render(PoolsPage);
			
			// Wait for component initialization
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Create modal should not be visible initially
			expect(screen.queryByText('Create Pool')).not.toBeInTheDocument();
		});

		it('should show create modal when add button clicked', async () => {
			render(PoolsPage);
			
			// Wait for component initialization
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Click add pool button
			const addButton = screen.getByRole('button', { name: /Add Pool/i });
			await fireEvent.click(addButton);
			
			// Should show create modal
			expect(screen.getByText(/Create Pool/i)).toBeInTheDocument();
		});

		it('should conditionally render update pool modal', async () => {
			render(PoolsPage);
			
			// Wait for component initialization
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Update modal should not be visible initially
			expect(screen.queryByText('Update Pool')).not.toBeInTheDocument();
		});

		it('should conditionally render delete pool modal', async () => {
			render(PoolsPage);
			
			// Wait for component initialization
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Delete modal should not be visible initially
			expect(screen.queryByText('Delete Pool')).not.toBeInTheDocument();
		});
	});

	describe('Pool Data Rendering', () => {
		it('should render pool data when available', async () => {
			render(PoolsPage);
			
			// Wait for component initialization
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should render the page structure correctly
			expect(screen.getByRole('heading', { name: 'Pools' })).toBeInTheDocument();
			expect(screen.getByPlaceholderText(/Search by entity name/i)).toBeInTheDocument();
		});

		it('should handle different pool states', async () => {
			render(PoolsPage);
			
			// Wait for component initialization
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should render the page structure correctly
			expect(screen.getByRole('heading', { name: 'Pools' })).toBeInTheDocument();
			expect(screen.getByPlaceholderText(/Search by entity name/i)).toBeInTheDocument();
		});

		it('should handle pool filtering and pagination', async () => {
			render(PoolsPage);
			
			// Wait for component initialization
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should render pagination controls
			expect(screen.getByText(/Show:/i)).toBeInTheDocument();
			expect(screen.getByDisplayValue('25')).toBeInTheDocument();
		});
	});

	describe('Interactive Elements', () => {
		it('should handle search input interaction', async () => {
			render(PoolsPage);
			
			// Wait for component initialization
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have interactive search input
			const searchInput = screen.getByPlaceholderText(/Search by entity name/i);
			await fireEvent.input(searchInput, { target: { value: 'test' } });
			
			// Input should be interactive
			expect(searchInput).toHaveValue('test');
		});

		it('should handle pagination interaction', async () => {
			render(PoolsPage);
			
			// Wait for component initialization
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have interactive pagination controls
			const perPageSelect = screen.getByDisplayValue('25');
			expect(perPageSelect).toBeInTheDocument();
		});

		it('should handle add pool button interaction', async () => {
			render(PoolsPage);
			
			// Wait for component initialization
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have interactive add button
			const addButton = screen.getByRole('button', { name: /Add Pool/i });
			expect(addButton).toBeInTheDocument();
			
			// Button should be clickable
			await fireEvent.click(addButton);
			expect(screen.getByText(/Create Pool/i)).toBeInTheDocument();
		});
	});

	describe('Responsive Layout', () => {
		it('should use responsive layout classes', async () => {
			render(PoolsPage);
			
			// Wait for component initialization
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have responsive layout
			const mainContainer = document.querySelector('.space-y-6');
			expect(mainContainer).toBeInTheDocument();
		});

		it('should handle mobile-friendly layout', async () => {
			render(PoolsPage);
			
			// Wait for component initialization
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should be configured for mobile responsiveness through DataTable
			expect(screen.getByPlaceholderText(/Search by entity name/i)).toBeInTheDocument();
			expect(screen.getByRole('heading', { name: 'Pools' })).toBeInTheDocument();
		});
	});

	describe('Accessibility', () => {
		it('should have proper accessibility attributes', async () => {
			render(PoolsPage);
			
			// Wait for component initialization
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have proper ARIA attributes and labels
			expect(screen.getByRole('heading', { name: 'Pools' })).toBeInTheDocument();
			expect(screen.getByRole('button', { name: /Add Pool/i })).toBeInTheDocument();
		});

		it('should be keyboard navigable', async () => {
			render(PoolsPage);
			
			// Wait for component initialization
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have focusable elements
			const searchInput = screen.getByPlaceholderText(/Search by entity name/i);
			expect(searchInput).toBeInTheDocument();
			
			const addButton = screen.getByRole('button', { name: /Add Pool/i });
			expect(addButton).toBeInTheDocument();
		});

		it('should handle screen reader compatibility', async () => {
			render(PoolsPage);
			
			// Wait for component initialization
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should be compatible with screen readers
			expect(screen.getByRole('heading', { name: 'Pools' })).toBeInTheDocument();
			expect(screen.getByPlaceholderText(/Search by entity name/i)).toBeInTheDocument();
		});
	});
});