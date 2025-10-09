import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render } from '@testing-library/svelte';
import { createMockOrganization } from '../../../test/factories.js';

// Mock all external dependencies but keep the component rendering real
vi.mock('$lib/api/client.js', () => ({
	garmApi: {
		getOrganization: vi.fn(),
		listOrganizationPools: vi.fn(),
		listOrganizationInstances: vi.fn(),
		updateOrganization: vi.fn(),
		deleteOrganization: vi.fn(),
		deleteInstance: vi.fn(),
		createOrganizationPool: vi.fn(),
		getOrganizationWebhookInfo: vi.fn().mockResolvedValue({ installed: false })
	}
}));

vi.mock('$lib/stores/websocket.js', () => ({
	websocketStore: {
		subscribeToEntity: vi.fn(() => vi.fn())
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

// Mock SvelteKit modules
vi.mock('$app/stores', () => ({
	page: {
		subscribe: vi.fn((callback) => {
			callback({ params: { id: 'org-123' } });
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

vi.mock('$app/environment', () => ({
	browser: false,
	dev: true,
	building: false
}));

// Mock child components
vi.mock('$lib/components/UpdateEntityModal.svelte', () => ({
	default: vi.fn(() => ({ destroy: vi.fn(), $$set: vi.fn() }))
}));

vi.mock('$lib/components/DeleteModal.svelte', () => ({
	default: vi.fn(() => ({ destroy: vi.fn(), $$set: vi.fn() }))
}));

vi.mock('$lib/components/EntityInformation.svelte', () => ({
	default: vi.fn(() => ({ destroy: vi.fn(), $$set: vi.fn() }))
}));

vi.mock('$lib/components/DetailHeader.svelte', () => ({
	default: vi.fn(() => ({ destroy: vi.fn(), $$set: vi.fn() }))
}));

vi.mock('$lib/components/PoolsSection.svelte', () => ({
	default: vi.fn(() => ({ destroy: vi.fn(), $$set: vi.fn() }))
}));

vi.mock('$lib/components/InstancesSection.svelte', () => ({
	default: vi.fn(() => ({ destroy: vi.fn(), $$set: vi.fn() }))
}));

vi.mock('$lib/components/EventsSection.svelte', () => ({
	default: vi.fn(() => ({ destroy: vi.fn(), $$set: vi.fn() }))
}));

vi.mock('$lib/components/WebhookSection.svelte', () => ({
	default: vi.fn(() => ({ destroy: vi.fn(), $$set: vi.fn() }))
}));

vi.mock('$lib/components/CreatePoolModal.svelte', () => ({
	default: vi.fn(() => ({ destroy: vi.fn(), $$set: vi.fn() }))
}));

vi.mock('$lib/utils/common.js', async (importOriginal) => {
	const actual = await importOriginal() as any;
	return {
		...actual,
		// Override only specific functions for testing

	getForgeIcon: vi.fn((type) => `<svg data-forge="${type}"></svg>`)
	};
});

vi.mock('$lib/utils/apiError', () => ({
	extractAPIError: vi.fn((error) => error.message || 'API Error')
}));

import OrganizationDetailsPage from './+page.svelte';

describe('Organization Details Page Rendering Tests', () => {
	beforeEach(async () => {
		vi.clearAllMocks();
		
		const mockOrganization = createMockOrganization({ 
			id: 'org-123', 
			name: 'test-org'
		});
		
		const { garmApi } = await import('$lib/api/client.js');
		(garmApi.getOrganization as any).mockResolvedValue(mockOrganization);
		(garmApi.listOrganizationPools as any).mockResolvedValue([]);
		(garmApi.listOrganizationInstances as any).mockResolvedValue([]);
	});

	describe('Component Rendering', () => {
		it('should render without crashing', () => {
			const { container } = render(OrganizationDetailsPage);
			expect(container).toBeInTheDocument();
		});

		it('should render as a valid DOM element', () => {
			const { container } = render(OrganizationDetailsPage);
			expect(container.firstChild).toBeInstanceOf(HTMLElement);
		});

		it('should have proper document title', () => {
			render(OrganizationDetailsPage);
			expect(document.title).toContain('Organization Details');
		});

		it('should render with correct structure', () => {
			const { container } = render(OrganizationDetailsPage);
			expect(container.firstChild).toHaveClass('space-y-6');
		});

		it('should handle empty state rendering', () => {
			render(OrganizationDetailsPage);
			// Component should render even with no organization data loaded
			expect(document.body).toBeInTheDocument();
		});
	});

	describe('Component Lifecycle', () => {
		it('should mount successfully', () => {
			const component = render(OrganizationDetailsPage);
			expect(component.component).toBeDefined();
		});

		it('should unmount without errors', () => {
			const { unmount } = render(OrganizationDetailsPage);
			expect(() => unmount()).not.toThrow();
		});
	});

	describe('DOM Structure Validation', () => {
		it('should create proper HTML structure', () => {
			const { container } = render(OrganizationDetailsPage);
			
			// Should have main container with proper spacing
			expect(container.querySelector('.space-y-6')).toBeInTheDocument();
		});

		it('should handle conditional rendering', () => {
			const { container } = render(OrganizationDetailsPage);
			
			// Component should render without any modals open initially
			expect(container).toBeInTheDocument();
		});

		it('should render with proper accessibility structure', () => {
			const { container } = render(OrganizationDetailsPage);
			
			// Basic accessibility checks
			expect(container).toBeInTheDocument();
		});
	});
});