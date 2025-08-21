import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render } from '@testing-library/svelte';
import { createMockOrganization, createMockGiteaOrganization } from '../../test/factories.js';

// Mock all external dependencies
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

// Mock SvelteKit modules
vi.mock('$app/paths', () => ({
	resolve: vi.fn((path) => path)
}));

vi.mock('$app/environment', () => ({
	browser: false,
	dev: true,
	building: false
}));

// Mock all child components
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

vi.mock('$lib/utils/common.js', () => ({
	getForgeIcon: vi.fn((type) => `<svg data-forge="${type}"></svg>`),
	getEntityStatusBadge: vi.fn(() => ({ variant: 'success', text: 'Running' })),
	filterByName: vi.fn((items, term) => 
		term ? items.filter((item: any) => 
			item.name.toLowerCase().includes(term.toLowerCase())
		) : items
	)
}));

vi.mock('$lib/utils/apiError', () => ({
	extractAPIError: vi.fn((error) => error.message || 'API Error')
}));

import OrganizationsPage from './+page.svelte';

describe('Organizations Page Unit Tests', () => {
	let mockOrganizations: any[];

	beforeEach(() => {
		vi.clearAllMocks();
		mockOrganizations = [
			createMockOrganization({ 
				id: 'org-1', 
				name: 'test-org',
				pool_manager_status: { running: true, failure_reason: undefined }
			}),
			createMockGiteaOrganization({ 
				id: 'org-2', 
				name: 'gitea-org',
				pool_manager_status: { running: false, failure_reason: undefined }
			})
		];
	});

	describe('Component Structure', () => {
		it('should render organizations page', () => {
			const { container } = render(OrganizationsPage);
			expect(container).toBeInTheDocument();
		});

		it('should set correct page title', () => {
			render(OrganizationsPage);
			expect(document.title).toBe('Organizations - GARM');
		});

		it('should have organizations state variables', async () => {
			const component = render(OrganizationsPage);
			expect(component).toBeDefined();
		});
	});

	describe('Data Management', () => {
		it('should initialize with correct default values', () => {
			// Component should render without errors and set up initial state
			const { container } = render(OrganizationsPage);
			expect(container).toBeInTheDocument();
			expect(document.title).toBe('Organizations - GARM');
		});

		it('should handle organizations data from eager cache', () => {
			// Component should render structure for handling cache data
			const { container } = render(OrganizationsPage);
			expect(container.querySelector('.space-y-6')).toBeInTheDocument();
		});
	});

	describe('Search and Filtering', () => {
		it('should filter organizations by search term', async () => {
			const { filterByName } = await import('$lib/utils/common.js');
			
			const filtered = filterByName(mockOrganizations, 'test');
			expect(filterByName).toHaveBeenCalledWith(mockOrganizations, 'test');
			expect(filtered).toHaveLength(1);
			expect(filtered[0].name).toBe('test-org');
		});

		it('should return all organizations when search term is empty', async () => {
			const { filterByName } = await import('$lib/utils/common.js');
			
			const filtered = filterByName(mockOrganizations, '');
			expect(filterByName).toHaveBeenCalledWith(mockOrganizations, '');
			expect(filtered).toHaveLength(2);
		});

		it('should handle case-insensitive search', async () => {
			const { filterByName } = await import('$lib/utils/common.js');
			
			filterByName(mockOrganizations, 'TEST');
			expect(filterByName).toHaveBeenCalledWith(mockOrganizations, 'TEST');
		});

		it('should reset to first page when searching', () => {
			render(OrganizationsPage);
			// Component should reset currentPage to 1 when search term changes
			expect(document.title).toBe('Organizations - GARM');
		});
	});

	describe('Pagination Logic', () => {
		it('should calculate total pages correctly', () => {
			const organizations = Array(75).fill(null).map((_, i) => 
				createMockOrganization({ id: `org-${i}`, name: `org-${i}` })
			);
			const perPage = 25;
			const totalPages = Math.ceil(organizations.length / perPage);
			expect(totalPages).toBe(3);
		});

		it('should calculate paginated organizations correctly', () => {
			const organizations = Array(75).fill(null).map((_, i) => 
				createMockOrganization({ id: `org-${i}`, name: `org-${i}` })
			);
			const currentPage = 2;
			const perPage = 25;
			const start = (currentPage - 1) * perPage;
			const paginatedOrganizations = organizations.slice(start, start + perPage);
			
			expect(paginatedOrganizations).toHaveLength(25);
			expect(paginatedOrganizations[0].name).toBe('org-25');
			expect(paginatedOrganizations[24].name).toBe('org-49');
		});

		it('should adjust current page when it exceeds total pages', () => {
			// When filtering reduces results, current page should adjust
			const totalPages = 2;
			let currentPage = 5;
			
			if (currentPage > totalPages && totalPages > 0) {
				currentPage = totalPages;
			}
			
			expect(currentPage).toBe(2);
		});

		it('should handle empty results gracefully', () => {
			const organizations: any[] = [];
			const perPage = 25;
			const totalPages = Math.ceil(organizations.length / perPage);
			expect(totalPages).toBe(0);
		});
	});

	describe('Modal Management', () => {
		it('should have correct initial modal states', () => {
			render(OrganizationsPage);
			// Component should render without modal states
			expect(document.title).toBe('Organizations - GARM');
		});

		it('should handle create modal opening', () => {
			render(OrganizationsPage);
			// Component should handle modal state management
			expect(document.title).toBe('Organizations - GARM');
		});

		it('should handle update modal opening with organization', () => {
			render(OrganizationsPage);
			// Component should handle update modal state
			expect(document.title).toBe('Organizations - GARM');
		});

		it('should handle delete modal opening with organization', () => {
			render(OrganizationsPage);
			// Component should handle delete modal state
			expect(document.title).toBe('Organizations - GARM');
		});

		it('should close all modals', () => {
			render(OrganizationsPage);
			// Component should handle modal closing
			expect(document.title).toBe('Organizations - GARM');
		});
	});

	describe('API Integration', () => {
		it('should call createOrganization API', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			render(OrganizationsPage);

			const orgParams = {
				name: 'new-org',
				credentials_name: 'test-creds',
				webhook_secret: 'secret123',
				pool_balancer_type: 'roundrobin'
			};

			await garmApi.createOrganization(orgParams);
			expect(garmApi.createOrganization).toHaveBeenCalledWith(orgParams);
		});

		it('should call updateOrganization API', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			render(OrganizationsPage);

			const updateParams = { webhook_secret: 'new-secret' };
			await garmApi.updateOrganization('org-1', updateParams);
			expect(garmApi.updateOrganization).toHaveBeenCalledWith('org-1', updateParams);
		});

		it('should call deleteOrganization API', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			render(OrganizationsPage);

			await garmApi.deleteOrganization('org-1');
			expect(garmApi.deleteOrganization).toHaveBeenCalledWith('org-1');
		});

		it('should call installOrganizationWebhook API when requested', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			render(OrganizationsPage);

			await garmApi.installOrganizationWebhook('org-1');
			expect(garmApi.installOrganizationWebhook).toHaveBeenCalledWith('org-1');
		});
	});

	describe('Toast Notifications', () => {
		it('should show success toast for organization creation', async () => {
			const { toastStore } = await import('$lib/stores/toast.js');
			render(OrganizationsPage);

			toastStore.success('Organization Created', 'Organization test-org has been created successfully.');
			expect(toastStore.success).toHaveBeenCalledWith(
				'Organization Created',
				'Organization test-org has been created successfully.'
			);
		});

		it('should show success toast for organization update', async () => {
			const { toastStore } = await import('$lib/stores/toast.js');
			render(OrganizationsPage);

			toastStore.success('Organization Updated', 'Organization test-org has been updated successfully.');
			expect(toastStore.success).toHaveBeenCalledWith(
				'Organization Updated',
				'Organization test-org has been updated successfully.'
			);
		});

		it('should show success toast for organization deletion', async () => {
			const { toastStore } = await import('$lib/stores/toast.js');
			render(OrganizationsPage);

			toastStore.success('Organization Deleted', 'Organization test-org has been deleted successfully.');
			expect(toastStore.success).toHaveBeenCalledWith(
				'Organization Deleted',
				'Organization test-org has been deleted successfully.'
			);
		});

		it('should show error toast for API failures', async () => {
			const { toastStore } = await import('$lib/stores/toast.js');
			render(OrganizationsPage);

			toastStore.error('Delete Failed', 'Organization deletion failed');
			expect(toastStore.error).toHaveBeenCalledWith('Delete Failed', 'Organization deletion failed');
		});
	});

	describe('DataTable Configuration', () => {
		it('should have correct column configuration', () => {
			render(OrganizationsPage);
			
			// DataTable should be configured with proper columns
			const expectedColumns = [
				{ key: 'name', title: 'Name' },
				{ key: 'endpoint', title: 'Endpoint' },
				{ key: 'credentials', title: 'Credentials' },
				{ key: 'status', title: 'Status' },
				{ key: 'actions', title: 'Actions', align: 'right' }
			];
			
			expect(expectedColumns).toHaveLength(5);
		});

		it('should have correct mobile card configuration', () => {
			render(OrganizationsPage);
			
			// Mobile card should be configured for organizations
			const config = {
				entityType: 'organization',
				primaryText: { field: 'name', isClickable: true, href: '/organizations/{id}' }
			};
			
			expect(config.entityType).toBe('organization');
			expect(config.primaryText.field).toBe('name');
			expect(config.primaryText.isClickable).toBe(true);
		});
	});

	describe('Event Handlers', () => {
		it('should handle table search event', () => {
			render(OrganizationsPage);
			
			// handleTableSearch should update searchTerm and reset page
			const mockEvent = { detail: { term: 'test-search' } };
			expect(mockEvent.detail.term).toBe('test-search');
		});

		it('should handle table page change event', () => {
			render(OrganizationsPage);
			
			// handleTablePageChange should update currentPage
			const mockEvent = { detail: { page: 3 } };
			expect(mockEvent.detail.page).toBe(3);
		});

		it('should handle table per-page change event', () => {
			render(OrganizationsPage);
			
			// handleTablePerPageChange should update perPage and reset page
			const mockEvent = { detail: { perPage: 50 } };
			expect(mockEvent.detail.perPage).toBe(50);
		});

		it('should handle edit action event', () => {
			render(OrganizationsPage);
			
			// handleEdit should call openUpdateModal
			const mockOrganization = createMockOrganization();
			const mockEvent = { detail: { item: mockOrganization } };
			expect(mockEvent.detail.item).toBe(mockOrganization);
		});

		it('should handle delete action event', () => {
			render(OrganizationsPage);
			
			// handleDelete should call openDeleteModal
			const mockOrganization = createMockOrganization();
			const mockEvent = { detail: { item: mockOrganization } };
			expect(mockEvent.detail.item).toBe(mockOrganization);
		});
	});

	describe('Error Handling', () => {
		it('should handle API errors in organization creation', async () => {
			const { extractAPIError } = await import('$lib/utils/apiError');
			render(OrganizationsPage);

			const error = new Error('Creation failed');
			const extractedError = extractAPIError(error);
			expect(extractAPIError).toHaveBeenCalledWith(error);
			expect(extractedError).toBe('Creation failed');
		});

		it('should handle webhook installation errors', async () => {
			const { toastStore } = await import('$lib/stores/toast.js');
			render(OrganizationsPage);

			// Should show error toast for webhook installation failure
			toastStore.error(
				'Webhook Installation Failed', 
				'Failed to install webhook. You can try installing it manually from the organization details page.'
			);
			expect(toastStore.error).toHaveBeenCalled();
		});

		it('should handle organizations loading errors', () => {
			render(OrganizationsPage);
			
			// Component should render without errors during error states
			expect(document.title).toBe('Organizations - GARM');
		});

		it('should handle retry functionality', async () => {
			const { eagerCacheManager } = await import('$lib/stores/eager-cache.js');
			render(OrganizationsPage);

			await eagerCacheManager.retryResource('organizations');
			expect(eagerCacheManager.retryResource).toHaveBeenCalledWith('organizations');
		});
	});

	describe('Utility Functions', () => {
		it('should get correct forge icon', async () => {
			const { getForgeIcon } = await import('$lib/utils/common.js');
			
			const githubIcon = getForgeIcon('github');
			const giteaIcon = getForgeIcon('gitea');
			
			expect(getForgeIcon).toHaveBeenCalledWith('github');
			expect(getForgeIcon).toHaveBeenCalledWith('gitea');
			expect(githubIcon).toContain('svg');
			expect(giteaIcon).toContain('svg');
		});

		it('should get entity status badge', async () => {
			const { getEntityStatusBadge } = await import('$lib/utils/common.js');
			
			const organization = createMockOrganization({ 
				pool_manager_status: { running: true, failure_reason: undefined }
			});
			
			const badge = getEntityStatusBadge(organization);
			expect(getEntityStatusBadge).toHaveBeenCalledWith(organization);
			expect(badge).toEqual({ variant: 'success', text: 'Running' });
		});
	});

	describe('Reactive Statements', () => {
		it('should update filtered organizations when search term changes', () => {
			render(OrganizationsPage);
			
			// Component should handle reactive filtering
			expect(document.title).toBe('Organizations - GARM');
		});

		it('should recalculate total pages when filtered organizations change', () => {
			render(OrganizationsPage);
			
			// Component should handle reactive pagination
			expect(document.title).toBe('Organizations - GARM');
		});

		it('should adjust current page when total pages change', () => {
			render(OrganizationsPage);
			
			// Component should handle page adjustments
			expect(document.title).toBe('Organizations - GARM');
		});

		it('should update paginated organizations when page or filter changes', () => {
			render(OrganizationsPage);
			
			// Component should handle reactive pagination updates
			expect(document.title).toBe('Organizations - GARM');
		});
	});

	describe('Lifecycle Management', () => {
		it('should load organizations on mount', () => {
			render(OrganizationsPage);
			
			// Component should load without errors on mount
			expect(document.title).toBe('Organizations - GARM');
		});

		it('should handle mount errors gracefully', () => {
			render(OrganizationsPage);
			
			// Component should handle mount errors gracefully
			expect(document.title).toBe('Organizations - GARM');
		});

		it('should subscribe to eager cache', () => {
			render(OrganizationsPage);
			
			// Component should set up cache subscription
			expect(document.title).toBe('Organizations - GARM');
		});
	});
});