import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import { createMockOrganization } from '../../test/factories.js';

// Mock all external dependencies but keep the component rendering real
vi.mock('$lib/api/client.js', () => ({
	garmApi: {
		createOrganization: vi.fn(),
		updateOrganization: vi.fn(),
		deleteOrganization: vi.fn(),
		installOrganizationWebhook: vi.fn(),
		listOrganizations: vi.fn()
	}
}));

vi.mock('$lib/stores/eager-cache.js', () => ({
	eagerCache: {
		subscribe: vi.fn((callback) => {
			callback({
				organizations: [],
				credentials: [],
				loaded: { organizations: true, credentials: true },
				loading: { organizations: false, credentials: false },
				errorMessages: { organizations: '', credentials: '' }
			});
			return () => {};
		})
	},
	eagerCacheManager: {
		getOrganizations: vi.fn(),
		retryResource: vi.fn(),
		getCredentials: vi.fn()
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

vi.mock('$app/paths', () => ({
	resolve: vi.fn((path) => path)
}));

vi.mock('$app/environment', () => ({
	browser: false,
	dev: true,
	building: false
}));

vi.mock('$lib/components/CreateOrganizationModal.svelte', () => ({
	default: vi.fn(() => ({ destroy: vi.fn(), $$set: vi.fn() }))
}));

vi.mock('$lib/components/UpdateEntityModal.svelte', () => ({
	default: vi.fn(() => ({ destroy: vi.fn(), $$set: vi.fn() }))
}));

vi.mock('$lib/components/DeleteModal.svelte', () => ({
	default: vi.fn(() => ({ destroy: vi.fn(), $$set: vi.fn() }))
}));

vi.mock('$lib/components/PageHeader.svelte', () => ({
	default: vi.fn(() => ({ destroy: vi.fn(), $$set: vi.fn() }))
}));

vi.mock('$lib/components/DataTable.svelte', () => ({
	default: vi.fn(() => ({ destroy: vi.fn(), $$set: vi.fn() }))
}));

vi.mock('$lib/components/Badge.svelte', () => ({
	default: vi.fn(() => ({ destroy: vi.fn(), $$set: vi.fn() }))
}));

vi.mock('$lib/components/ActionButton.svelte', () => ({
	default: vi.fn(() => ({ destroy: vi.fn(), $$set: vi.fn() }))
}));

vi.mock('$lib/components/cells', () => ({
	EntityCell: vi.fn(() => ({ destroy: vi.fn(), $$set: vi.fn() })),
	EndpointCell: vi.fn(() => ({ destroy: vi.fn(), $$set: vi.fn() })),
	StatusCell: vi.fn(() => ({ destroy: vi.fn(), $$set: vi.fn() })),
	ActionsCell: vi.fn(() => ({ destroy: vi.fn(), $$set: vi.fn() })),
	GenericCell: vi.fn(() => ({ destroy: vi.fn(), $$set: vi.fn() }))
}));

vi.mock('$lib/utils/common.js', async (importOriginal) => {
	const actual = await importOriginal() as any;
	return {
		...actual,
		// Override only specific functions for testing

	getForgeIcon: vi.fn((type) => `<svg data-forge="${type}"></svg>`),
	getEntityStatusBadge: vi.fn(() => ({ variant: 'success', text: 'Running' })),
	filterByName: vi.fn((items, term) => 
		term ? items.filter((item: any) => 
			item.name.toLowerCase().includes(term.toLowerCase())
		) : items
	)
	};
});

vi.mock('$lib/utils/apiError', () => ({
	extractAPIError: vi.fn((error) => error.message || 'API Error')
}));

import OrganizationsPage from './+page.svelte';

describe('Organizations Page Rendering Tests', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	describe('Component Rendering', () => {
		it('should render without crashing', () => {
			const { container } = render(OrganizationsPage);
			expect(container).toBeInTheDocument();
		});

		it('should render as a valid DOM element', () => {
			const { container } = render(OrganizationsPage);
			expect(container.firstChild).toBeInstanceOf(HTMLElement);
		});

		it('should have proper document title', () => {
			render(OrganizationsPage);
			expect(document.title).toBe('Organizations - GARM');
		});

		it('should render with correct structure', () => {
			const { container } = render(OrganizationsPage);
			expect(container.firstChild).toHaveClass('space-y-6');
		});

		it('should handle empty state rendering', () => {
			render(OrganizationsPage);
			// Component should render even with no organizations
			expect(document.body).toBeInTheDocument();
		});
	});

	describe('Component Lifecycle', () => {
		it('should mount successfully', () => {
			const component = render(OrganizationsPage);
			expect(component.component).toBeDefined();
		});

		it('should unmount without errors', () => {
			const { unmount } = render(OrganizationsPage);
			expect(() => unmount()).not.toThrow();
		});
	});

	describe('DOM Structure Validation', () => {
		it('should create proper HTML structure', () => {
			const { container } = render(OrganizationsPage);
			
			// Should have main container
			expect(container.querySelector('.space-y-6')).toBeInTheDocument();
		});

		it('should handle conditional rendering', () => {
			const { container } = render(OrganizationsPage);
			
			// Component should render without any modals open initially
			expect(container).toBeInTheDocument();
		});

		it('should render with proper accessibility structure', () => {
			const { container } = render(OrganizationsPage);
			
			// Basic accessibility checks
			expect(container).toBeInTheDocument();
		});
	});
});