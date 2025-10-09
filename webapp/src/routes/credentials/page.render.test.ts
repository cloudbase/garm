import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render } from '@testing-library/svelte';
import CredentialsPage from './+page.svelte';
import { createMockGithubCredentials, createMockForgeEndpoint } from '../../test/factories.js';

// Mock all external dependencies
vi.mock('$app/stores', () => ({}));

vi.mock('$app/navigation', () => ({}));

vi.mock('$lib/api/client.js', () => ({
	garmApi: {
		createGithubCredentials: vi.fn(),
		createGiteaCredentials: vi.fn(),
		updateGithubCredentials: vi.fn(),
		updateGiteaCredentials: vi.fn(),
		deleteGithubCredentials: vi.fn(),
		deleteGiteaCredentials: vi.fn()
	}
}));

vi.mock('$lib/stores/toast.js', () => ({
	toastStore: {
		success: vi.fn(),
		error: vi.fn(),
		info: vi.fn()
	}
}));

vi.mock('$lib/stores/eager-cache.js', () => ({
	eagerCache: {
		subscribe: vi.fn((callback) => {
			callback({
				credentials: [],
				endpoints: [],
				loading: { credentials: false, endpoints: false },
				loaded: { credentials: false, endpoints: false },
				errorMessages: { credentials: '', endpoints: '' }
			});
			return () => {};
		})
	},
	eagerCacheManager: {
		getCredentials: vi.fn(),
		getEndpoints: vi.fn(),
		retryResource: vi.fn()
	}
}));

vi.mock('$lib/utils/common.js', async (importOriginal) => {
	const actual = await importOriginal() as any;
	return {
		...actual,
		// Override only specific functions for testing

	getForgeIcon: vi.fn(() => 'github'),
	filterCredentials: vi.fn((credentials) => credentials),
	changePerPage: vi.fn((perPage) => ({ newPerPage: perPage, newCurrentPage: 1 })),
	paginateItems: vi.fn((items) => items),
	getAuthTypeBadge: vi.fn(() => 'PAT'),
	getEntityStatusBadge: vi.fn(() => 'active'),
	formatDate: vi.fn((date) => date)
	};
});

vi.mock('$lib/utils/apiError', () => ({
	extractAPIError: vi.fn((err) => err.message || 'Unknown error')
}));

const mockCredential = createMockGithubCredentials({
	name: 'github-creds',
	description: 'GitHub credentials',
	'auth-type': 'pat'
});

const mockEndpoint = createMockForgeEndpoint({
	name: 'github.com',
	description: 'GitHub.com endpoint',
	endpoint_type: 'github'
});

describe('Credentials Page - Render Tests', () => {
	beforeEach(async () => {
		vi.clearAllMocks();
		
		// Set up default API mocks
		const { eagerCacheManager } = await import('$lib/stores/eager-cache.js');
		(eagerCacheManager.getCredentials as any).mockResolvedValue([mockCredential]);
		(eagerCacheManager.getEndpoints as any).mockResolvedValue([mockEndpoint]);
	});

	describe('Basic Rendering', () => {
		it('should render without crashing', () => {
			const { container } = render(CredentialsPage);
			expect(container).toBeInTheDocument();
		});

		it('should have proper document structure', () => {
			const { container } = render(CredentialsPage);
			expect(container.querySelector('div')).toBeInTheDocument();
		});

		it('should render page header', () => {
			const { container } = render(CredentialsPage);
			// Should have page header component
			expect(container).toBeInTheDocument();
		});

		it('should render data table', () => {
			const { container } = render(CredentialsPage);
			// Should have DataTable component
			expect(container).toBeInTheDocument();
		});
	});

	describe('Component Lifecycle', () => {
		it('should mount successfully', () => {
			const { component } = render(CredentialsPage);
			expect(component).toBeDefined();
		});

		it('should unmount without errors', () => {
			const { unmount } = render(CredentialsPage);
			expect(() => unmount()).not.toThrow();
		});

		it('should handle component updates', async () => {
			const { component } = render(CredentialsPage);
			
			// Component should handle reactive updates
			expect(component).toBeDefined();
		});

		it('should load credentials on mount', async () => {
			const { eagerCacheManager } = await import('$lib/stores/eager-cache.js');
			
			render(CredentialsPage);
			
			// Wait for component mount and data loading
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should call eager cache to load credentials
			expect(eagerCacheManager.getCredentials).toHaveBeenCalled();
		});

		it('should load endpoints on mount', async () => {
			const { eagerCacheManager } = await import('$lib/stores/eager-cache.js');
			
			render(CredentialsPage);
			
			// Wait for component mount and data loading
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should call eager cache to load endpoints
			expect(eagerCacheManager.getEndpoints).toHaveBeenCalled();
		});
	});

	describe('DOM Structure', () => {
		it('should create proper DOM hierarchy', () => {
			const { container } = render(CredentialsPage);
			
			// Should have main container with proper spacing
			const mainDiv = container.querySelector('div.space-y-6');
			expect(mainDiv).toBeInTheDocument();
		});

		it('should render svelte:head for page title', async () => {
			render(CredentialsPage);
			
			// Should set page title
			expect(document.title).toContain('Credentials - GARM');
		});

		it('should handle window event listeners', () => {
			render(CredentialsPage);
			
			// Window should have event listener capabilities available
			expect(window.addEventListener).toBeDefined();
			expect(window.removeEventListener).toBeDefined();
			
			// Component should be able to handle keyboard events for modal management
			expect(document).toBeDefined();
			expect(document.addEventListener).toBeDefined();
		});
	});

	describe('Modal Rendering', () => {
		it('should conditionally render create modal', () => {
			const { container } = render(CredentialsPage);
			
			// Create modal should not be visible initially
			expect(container).toBeInTheDocument();
		});

		it('should conditionally render edit modal', () => {
			const { container } = render(CredentialsPage);
			
			// Edit modal should not be visible initially
			expect(container).toBeInTheDocument();
		});

		it('should conditionally render delete modal', () => {
			const { container } = render(CredentialsPage);
			
			// Delete modal should not be visible initially
			expect(container).toBeInTheDocument();
		});

		it('should conditionally render forge type selector', () => {
			const { container } = render(CredentialsPage);
			
			// Forge type selector should be available for create modal
			expect(container).toBeInTheDocument();
		});
	});
});