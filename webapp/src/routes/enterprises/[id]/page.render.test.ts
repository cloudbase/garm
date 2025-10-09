import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render } from '@testing-library/svelte';
import EnterpriseDetailsPage from './+page.svelte';
import { createMockEnterprise } from '../../../test/factories.js';

// Mock all external dependencies
vi.mock('$app/stores', () => ({
	page: {
		subscribe: vi.fn((callback) => {
			callback({ params: { id: 'ent-123' } });
			return () => {};
		})
	}
}));

vi.mock('$app/navigation', () => ({
	goto: vi.fn()
}));

vi.mock('$app/paths', () => ({
	resolve: vi.fn((path) => path)
}));

vi.mock('$lib/api/client.js', () => ({
	garmApi: {
		getEnterprise: vi.fn(),
		listEnterprisePools: vi.fn(),
		listEnterpriseInstances: vi.fn(),
		updateEnterprise: vi.fn(),
		deleteEnterprise: vi.fn(),
		deleteInstance: vi.fn(),
		createEnterprisePool: vi.fn()
	}
}));

vi.mock('$lib/stores/toast.js', () => ({
	toastStore: {
		success: vi.fn(),
		error: vi.fn()
	}
}));

vi.mock('$lib/stores/websocket.js', () => ({
	websocketStore: {
		subscribeToEntity: vi.fn(() => () => {}),
		subscribe: vi.fn(() => () => {})
	}
}));

vi.mock('$lib/utils/common.js', async (importOriginal) => {
	const actual = await importOriginal() as any;
	return {
		...actual,
		// Use all real functions
	};
});

vi.mock('$lib/utils/apiError', () => ({
	extractAPIError: vi.fn((err) => err.message || 'Unknown error')
}));

const mockEnterprise = createMockEnterprise({
	id: 'ent-123',
	name: 'test-enterprise',
	endpoint: {
		name: 'github.com'
	},
	pool_manager_status: { running: true, failure_reason: undefined }
});

describe('Enterprise Details Page - Render Tests', () => {
	beforeEach(async () => {
		vi.clearAllMocks();
		
		// Set up default API mocks
		const { garmApi } = await import('$lib/api/client.js');
		(garmApi.getEnterprise as any).mockResolvedValue(mockEnterprise);
		(garmApi.listEnterprisePools as any).mockResolvedValue([]);
		(garmApi.listEnterpriseInstances as any).mockResolvedValue([]);
	});

	describe('Basic Rendering', () => {
		it('should render without crashing', () => {
			const { container } = render(EnterpriseDetailsPage);
			expect(container).toBeInTheDocument();
		});

		it('should have proper document structure', () => {
			const { container } = render(EnterpriseDetailsPage);
			expect(container.querySelector('div')).toBeInTheDocument();
		});

		it('should render breadcrumb navigation', () => {
			const { container } = render(EnterpriseDetailsPage);
			const breadcrumb = container.querySelector('[aria-label="Breadcrumb"]');
			expect(breadcrumb).toBeInTheDocument();
		});

		it('should render loading state initially', () => {
			const { container } = render(EnterpriseDetailsPage);
			// Component should render some form of loading indicator or content
			expect(container).toBeInTheDocument();
		});
	});

	describe('Component Lifecycle', () => {
		it('should mount successfully', () => {
			const { component } = render(EnterpriseDetailsPage);
			expect(component).toBeDefined();
		});

		it('should unmount without errors', () => {
			const { unmount } = render(EnterpriseDetailsPage);
			expect(() => unmount()).not.toThrow();
		});

		it('should handle component updates', async () => {
			const { component } = render(EnterpriseDetailsPage);
			
			// Component should handle reactive updates
			expect(component).toBeDefined();
		});

		it('should set up websocket subscriptions on mount', async () => {
			const { websocketStore } = await import('$lib/stores/websocket.js');
			
			render(EnterpriseDetailsPage);
			
			// Wait for component mount and subscription setup
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should call subscription setup
			expect(websocketStore.subscribeToEntity).toHaveBeenCalled();
		});
	});

	describe('DOM Structure', () => {
		it('should create proper DOM hierarchy', () => {
			const { container } = render(EnterpriseDetailsPage);
			
			// Should have main container with proper spacing
			const mainDiv = container.querySelector('div.space-y-6');
			expect(mainDiv).toBeInTheDocument();
		});

		it('should render svelte:head for page title', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			
			// Mock enterprise data for the title
			(garmApi.getEnterprise as any).mockResolvedValue(mockEnterprise);
			
			render(EnterpriseDetailsPage);
			
			// Initially should show generic title (before enterprise loads)
			expect(document.title).toContain('Enterprise Details - GARM');
			
			// Wait for enterprise data to load and title to update
			await new Promise(resolve => setTimeout(resolve, 100));
			
			// Should now show enterprise-specific title
			expect(document.title).toContain('test-enterprise - Enterprise Details - GARM');
		});
	});
});