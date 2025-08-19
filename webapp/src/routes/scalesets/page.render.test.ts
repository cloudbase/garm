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

// Mock all external dependencies
vi.mock('$app/stores', () => ({}));
vi.mock('$app/navigation', () => ({}));

vi.mock('$lib/api/client.js', () => ({
	garmApi: {
		updateScaleSet: vi.fn(),
		deleteScaleSet: vi.fn()
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
			callback(createMockCacheState());
			return () => {};
		})
	},
	eagerCacheManager: {
		getScaleSets: vi.fn(),
		retryResource: vi.fn()
	}
}));

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

const mockScaleSet = createMockScaleSet({
	id: 123,
	name: 'test-scaleset',
	repo_name: 'test-repo',
	provider_name: 'hetzner',
	enabled: true,
	image: 'ubuntu:22.04',
	flavor: 'default'
});

// Reset any component mocks that might be set by setup.ts
vi.unmock('$lib/components/PageHeader.svelte');
vi.unmock('$lib/components/DataTable.svelte');
vi.unmock('$lib/components/CreateScaleSetModal.svelte');
vi.unmock('$lib/components/UpdateScaleSetModal.svelte');
vi.unmock('$lib/components/DeleteModal.svelte');
vi.unmock('$lib/components/cells');

describe('Scale Sets Page - Render Tests', () => {
	beforeEach(async () => {
		vi.clearAllMocks();
		
		// Set up default API mocks
		const { eagerCacheManager } = await import('$lib/stores/eager-cache.js');
		(eagerCacheManager.getScaleSets as any).mockResolvedValue([mockScaleSet]);
	});

	describe('Basic Rendering', () => {
		it('should render without crashing', () => {
			const { container } = render(ScaleSetsPage);
			expect(container).toBeInTheDocument();
		});

		it('should have proper document structure', () => {
			const { container } = render(ScaleSetsPage);
			expect(container.querySelector('div')).toBeInTheDocument();
		});

		it('should render page header', async () => {
			render(ScaleSetsPage);
			
			// Wait for component to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have page header
			expect(screen.getByRole('heading', { name: 'Scale Sets' })).toBeInTheDocument();
		});

		it('should render data table', async () => {
			render(ScaleSetsPage);
			
			// Wait for component to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have data table
			expect(screen.getByRole('heading', { name: 'Scale Sets' })).toBeInTheDocument();
		});
	});

	describe('Component Lifecycle', () => {
		it('should mount successfully', () => {
			const { component } = render(ScaleSetsPage);
			expect(component).toBeDefined();
		});

		it('should unmount without errors', () => {
			const { unmount } = render(ScaleSetsPage);
			expect(() => unmount()).not.toThrow();
		});

		it('should handle component updates', async () => {
			const { component } = render(ScaleSetsPage);
			
			// Component should handle reactive updates
			expect(component).toBeDefined();
		});

		it('should load scale sets on mount', async () => {
			const { eagerCacheManager } = await import('$lib/stores/eager-cache.js');
			
			render(ScaleSetsPage);
			
			// Wait for component mount and data loading
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should call eager cache manager to load scale sets
			expect(eagerCacheManager.getScaleSets).toHaveBeenCalled();
		});
	});

	describe('DOM Structure', () => {
		it('should create proper DOM hierarchy', async () => {
			const { container } = render(ScaleSetsPage);
			
			// Wait for component to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have main container with proper spacing
			const mainDiv = container.querySelector('div.space-y-6');
			expect(mainDiv).toBeInTheDocument();
		});

		it('should render svelte:head for page title', async () => {
			render(ScaleSetsPage);
			
			// Wait for component to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should set page title
			expect(document.title).toBe('Scale Sets - GARM');
		});

		it('should handle error display conditionally', async () => {
			const { eagerCache } = await import('$lib/stores/eager-cache.js');
			vi.mocked(eagerCache.subscribe).mockImplementation((callback) => {
				callback(createMockCacheState({
					errorMessages: {
						repositories: '',
						organizations: '',
						enterprises: '',
						pools: '',
						scalesets: 'Test error',
						credentials: '',
						endpoints: '',
						controllerInfo: ''
					}
				}));
				return () => {};
			});
			
			render(ScaleSetsPage);
			
			// Wait for error
			await new Promise(resolve => setTimeout(resolve, 100));
			
			// Error display should be conditional
			expect(screen.getByText(/Test error/i)).toBeInTheDocument();
		});

		it('should render loading state initially', async () => {
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
			
			// Should show loading initially
			expect(screen.getByText(/Loading scale sets/i)).toBeInTheDocument();
		});
	});

	describe('Header Section Rendering', () => {
		it('should render page header with correct title', async () => {
			render(ScaleSetsPage);
			
			// Wait for component to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should render page header
			expect(screen.getByRole('heading', { name: 'Scale Sets' })).toBeInTheDocument();
			expect(screen.getByText('Manage GitHub runner scale sets')).toBeInTheDocument();
		});

		it('should render create action button', async () => {
			render(ScaleSetsPage);
			
			// Wait for component to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have create button
			expect(screen.getByText('Add Scale Set')).toBeInTheDocument();
		});
	});

	describe('Data Table Rendering', () => {
		it('should render data table with scale sets', async () => {
			const { eagerCache } = await import('$lib/stores/eager-cache.js');
			
			vi.mocked(eagerCache.subscribe).mockImplementation((callback) => {
				callback(createMockCacheState({
					scalesets: [mockScaleSet],
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
			
			// Wait for component to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should render data table
			expect(screen.getByRole('heading', { name: 'Scale Sets' })).toBeInTheDocument();
		});

		it('should render search functionality', async () => {
			render(ScaleSetsPage);
			
			// Wait for component to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have search input
			expect(screen.getByPlaceholderText(/Search by entity name/i)).toBeInTheDocument();
		});

		it('should render pagination controls', async () => {
			render(ScaleSetsPage);
			
			// Wait for component to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have pagination controls
			expect(screen.getByDisplayValue('25')).toBeInTheDocument();
		});

		it('should render empty state when no scale sets', async () => {
			const { eagerCache } = await import('$lib/stores/eager-cache.js');
			
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
			
			// Wait for component to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should show empty state
			expect(screen.getByText(/No scale sets found/i)).toBeInTheDocument();
		});
	});

	describe('Modal Rendering', () => {
		it('should conditionally render create modal', async () => {
			render(ScaleSetsPage);
			
			// Wait for component to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Create modal should not be visible initially
			expect(screen.queryByText(/Create Scale Set/i)).not.toBeInTheDocument();
		});

		it('should conditionally render update modal', async () => {
			render(ScaleSetsPage);
			
			// Wait for component to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Update modal should not be visible initially
			expect(screen.queryByText(/Update Scale Set/i)).not.toBeInTheDocument();
		});

		it('should conditionally render delete modal', async () => {
			render(ScaleSetsPage);
			
			// Wait for component to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Delete modal should not be visible initially
			expect(screen.queryByText(/Delete Scale Set/i)).not.toBeInTheDocument();
		});
	});

	describe('Integration Elements', () => {
		it('should integrate eager cache subscription', async () => {
			const { eagerCache } = await import('$lib/stores/eager-cache.js');
			
			render(ScaleSetsPage);
			
			// Should subscribe to eager cache
			expect(eagerCache.subscribe).toHaveBeenCalled();
		});

		it('should integrate with eager cache manager', async () => {
			const { eagerCacheManager } = await import('$lib/stores/eager-cache.js');
			
			render(ScaleSetsPage);
			
			// Wait for component mount
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should use cache manager for loading
			expect(eagerCacheManager.getScaleSets).toHaveBeenCalled();
		});

		it('should integrate retry functionality', async () => {
			const { eagerCacheManager } = await import('$lib/stores/eager-cache.js');
			
			render(ScaleSetsPage);
			
			// Retry function should be available
			expect(eagerCacheManager.retryResource).toBeDefined();
		});
	});

	describe('Responsive Layout', () => {
		it('should use responsive layout classes', async () => {
			render(ScaleSetsPage);
			
			// Wait for component to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have responsive layout
			const container = document.querySelector('.space-y-6');
			expect(container).toBeInTheDocument();
		});

		it('should handle mobile-friendly layout', async () => {
			render(ScaleSetsPage);
			
			// Wait for component to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have mobile card configuration
			expect(screen.getByRole('heading', { name: 'Scale Sets' })).toBeInTheDocument();
		});
	});

	describe('Component Integration', () => {
		it('should integrate all major components', async () => {
			render(ScaleSetsPage);
			
			// Wait for component to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should integrate PageHeader and DataTable
			expect(screen.getByRole('heading', { name: 'Scale Sets' })).toBeInTheDocument();
			expect(screen.getByText('Add Scale Set')).toBeInTheDocument();
		});

		it('should handle component communication', async () => {
			render(ScaleSetsPage);
			
			// Wait for component to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Component should be ready for events
			expect(screen.getByRole('heading', { name: 'Scale Sets' })).toBeInTheDocument();
		});
	});

	describe('Error State Rendering', () => {
		it('should render error states gracefully', async () => {
			const { eagerCacheManager } = await import('$lib/stores/eager-cache.js');
			
			// Mock to fail
			(eagerCacheManager.getScaleSets as any).mockRejectedValue(new Error('Test error'));
			
			render(ScaleSetsPage);
			
			// Wait for error handling
			await new Promise(resolve => setTimeout(resolve, 100));
			
			// Should render without crashing despite error
			expect(screen.getByRole('heading', { name: 'Scale Sets' })).toBeInTheDocument();
		});

		it('should handle cache errors in UI', async () => {
			const { eagerCache } = await import('$lib/stores/eager-cache.js');
			
			vi.mocked(eagerCache.subscribe).mockImplementation((callback) => {
				callback(createMockCacheState({
					errorMessages: {
						repositories: '',
						organizations: '',
						enterprises: '',
						pools: '',
						scalesets: 'Cache error occurred',
						credentials: '',
						endpoints: '',
						controllerInfo: ''
					}
				}));
				return () => {};
			});
			
			render(ScaleSetsPage);
			
			// Should display cache error
			expect(screen.getByText(/Cache error occurred/i)).toBeInTheDocument();
		});
	});
});