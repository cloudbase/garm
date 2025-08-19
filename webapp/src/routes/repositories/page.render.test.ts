import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render } from '@testing-library/svelte';
import { createMockRepository, createMockGiteaRepository } from '../../test/factories.js';

// Mock all the dependencies first
vi.mock('$lib/api/client.js', () => ({
	garmApi: {
		createRepository: vi.fn(),
		updateRepository: vi.fn(),
		deleteRepository: vi.fn(),
		installRepoWebhook: vi.fn(),
		listRepositories: vi.fn()
	}
}));

vi.mock('$lib/stores/eager-cache.js', () => ({
	eagerCache: {
		subscribe: vi.fn((callback) => {
			callback({
				repositories: [
					createMockRepository({ name: 'test-repo-1', owner: 'owner-1' }),
					createMockGiteaRepository({ name: 'gitea-repo', owner: 'owner-2' })
				],
				loaded: { repositories: true },
				loading: { repositories: false },
				errorMessages: { repositories: '' }
			});
			return () => {};
		})
	},
	eagerCacheManager: {
		getRepositories: vi.fn(),
		retryResource: vi.fn()
	}
}));

vi.mock('$lib/stores/toast.js', () => ({
	toastStore: {
		success: vi.fn(),
		error: vi.fn(),
		info: vi.fn(),
		warning: vi.fn()
	}
}));

vi.mock('$lib/utils/common.js', () => ({
	getForgeIcon: vi.fn((endpointType: string) => {
		if (endpointType === 'github') {
			return '<div class="github-icon">GitHub Icon</div>';
		} else if (endpointType === 'gitea') {
			return '<svg class="gitea-icon">Gitea Icon</svg>';
		}
		return '<svg class="unknown-icon">Unknown Icon</svg>';
	}),
	changePerPage: vi.fn((newPerPage: number) => ({
		newPerPage,
		newCurrentPage: 1
	})),
	getEntityStatusBadge: vi.fn((entity: any) => ({
		text: entity?.pool_manager_status?.running ? 'Running' : 'Stopped',
		variant: entity?.pool_manager_status?.running ? 'success' : 'error'
	})),
	filterRepositories: vi.fn((repositories: any[], searchTerm: string) => {
		if (!searchTerm) return repositories;
		return repositories.filter((repo: any) => 
			repo.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
			repo.owner.toLowerCase().includes(searchTerm.toLowerCase())
		);
	}),
	paginateItems: vi.fn((items: any[], currentPage: number, perPage: number) => {
		const start = (currentPage - 1) * perPage;
		return items.slice(start, start + perPage);
	})
}));

vi.mock('$lib/utils/apiError', () => ({
	extractAPIError: vi.fn((error: any) => {
		return error?.message || 'An error occurred';
	})
}));

// Import the actual repositories page component after mocks
import RepositoriesPage from './+page.svelte';

describe('Repositories Page Rendering Tests', () => {
	let eagerCacheManager: any;

	beforeEach(async () => {
		vi.clearAllMocks();
		
		// Setup default mock implementations
		const cache = await import('$lib/stores/eager-cache.js');
		eagerCacheManager = cache.eagerCacheManager;
		
		eagerCacheManager.getRepositories.mockResolvedValue([]);
		eagerCacheManager.retryResource.mockResolvedValue({});
	});

	it('should render the repositories page component using testing library', () => {
		// Test that render() doesn't throw errors and returns valid container
		const result = render(RepositoriesPage);
		
		expect(result).toBeDefined();
		expect(result.container).toBeDefined();
		expect(result.component).toBeDefined();
	});

	it('should render the page structure correctly', () => {
		const { container } = render(RepositoriesPage);
		
		// Test that the main page structure is rendered
		const spaceYDiv = container.querySelector('.space-y-6');
		expect(spaceYDiv).toBeTruthy();
		expect(spaceYDiv).toBeInTheDocument();
	});

	it('should have correct page title in document head', () => {
		render(RepositoriesPage);
		
		// Test that the document title is set correctly
		expect(document.title).toBe('Repositories - GARM');
	});

	it('should render without throwing errors', () => {
		// Test that rendering doesn't throw any errors
		expect(() => render(RepositoriesPage)).not.toThrow();
	});

	it('should have proper component structure in DOM', () => {
		const { container } = render(RepositoriesPage);
		
		// Test that the component creates actual DOM elements
		expect(container.innerHTML).toContain('space-y-6');
		expect(container.firstChild).toBeTruthy();
	});

	it('should successfully mount and render component in DOM', () => {
		// Test that the component can be successfully mounted and rendered
		const { container } = render(RepositoriesPage);
		
		// Verify the component is actually in the DOM
		expect(container).toBeInTheDocument();
		expect(container.children.length).toBeGreaterThan(0);
	});

	it('should handle component lifecycle correctly', () => {
		const { unmount } = render(RepositoriesPage);
		
		// Test that unmounting doesn't throw errors
		expect(() => unmount()).not.toThrow();
	});
});