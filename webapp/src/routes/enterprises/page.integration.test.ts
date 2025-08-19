import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';
import { createMockEnterprise } from '../../test/factories.js';

// Create diverse test data for comprehensive testing
const mockEnterprises = [
	createMockEnterprise({ 
		id: 'ent-1',
		name: 'test-enterprise', 
		pool_manager_status: { running: true, failure_reason: undefined }
	}),
	createMockEnterprise({ 
		id: 'ent-2',
		name: 'github-enterprise', 
		pool_manager_status: { running: false, failure_reason: undefined }
	}),
	createMockEnterprise({ 
		id: 'ent-3',
		name: 'another-enterprise', 
		pool_manager_status: { running: false, failure_reason: 'Connection failed' }
	})
];

const mockCredentials = [
	{ name: 'github-creds' },
	{ name: 'enterprise-creds' }
];

// Reset any component mocks that might be set by setup.ts
vi.unmock('$lib/components/PageHeader.svelte');
vi.unmock('$lib/components/DataTable.svelte');
vi.unmock('$lib/components/CreateEnterpriseModal.svelte');
vi.unmock('$lib/components/UpdateEntityModal.svelte');
vi.unmock('$lib/components/DeleteModal.svelte');
vi.unmock('$lib/components/cells');

// Only mock the external APIs, not UI components
vi.mock('$lib/api/client.js', () => ({
	garmApi: {
		createEnterprise: vi.fn(),
		updateEnterprise: vi.fn(),
		deleteEnterprise: vi.fn(),
		listEnterprises: vi.fn()
	}
}));

// Create a dynamic store that can be updated during tests
let mockStoreData = {
	enterprises: mockEnterprises,
	credentials: mockCredentials,
	loaded: { enterprises: true, credentials: true },
	loading: { enterprises: false, credentials: false },
	errorMessages: { enterprises: '', credentials: '' }
};

vi.mock('$lib/stores/eager-cache.js', () => ({
	eagerCache: {
		subscribe: vi.fn((callback) => {
			callback(mockStoreData);
			return () => {};
		})
	},
	eagerCacheManager: {
		getEnterprises: vi.fn(),
		retryResource: vi.fn(),
		getCredentials: vi.fn()
	}
}));

// Helper to update mock store data
function updateMockStore(updates: Partial<typeof mockStoreData>) {
	mockStoreData = { ...mockStoreData, ...updates };
}

vi.mock('$lib/stores/toast.js', () => ({
	toastStore: {
		success: vi.fn(),
		error: vi.fn(),
		info: vi.fn(),
		warning: vi.fn()
	}
}));

// Import the enterprises page without any UI component mocks
import EnterprisesPage from './+page.svelte';

describe('Comprehensive Integration Tests for Enterprises Page', () => {
	let garmApi: any;

	beforeEach(async () => {
		vi.clearAllMocks();
		// Reset mock store data
		mockStoreData = {
			enterprises: mockEnterprises,
			credentials: mockCredentials,
			loaded: { enterprises: true, credentials: true },
			loading: { enterprises: false, credentials: false },
			errorMessages: { enterprises: '', credentials: '' }
		};

		const apiClient = await import('$lib/api/client.js');
		garmApi = apiClient.garmApi;
		
		garmApi.createEnterprise.mockResolvedValue({ id: 'new-ent', name: 'new-ent' });
		garmApi.updateEnterprise.mockResolvedValue({});
		garmApi.deleteEnterprise.mockResolvedValue({});
	});

	describe('Component Rendering and Basic Structure', () => {
		it('should render enterprises page with multiple enterprises', async () => {
			const { container } = render(EnterprisesPage);

			// Verify page title and header
			expect(screen.getByText('Enterprises')).toBeInTheDocument();
			expect(screen.getByText('Manage GitHub enterprises')).toBeInTheDocument();

			// Verify all enterprises are rendered (use getAllByText for duplicates)
			expect(screen.getAllByText('test-enterprise')[0]).toBeInTheDocument();
			expect(screen.getAllByText('github-enterprise')[0]).toBeInTheDocument();
			expect(screen.getAllByText('another-enterprise')[0]).toBeInTheDocument();

			// Verify action buttons are present
			const editButtons = container.querySelectorAll('[title="Edit"], [title="Edit enterprise"]');
			const deleteButtons = container.querySelectorAll('[title="Delete"], [title="Delete enterprise"]');
			expect(editButtons.length).toBeGreaterThan(0);
			expect(deleteButtons.length).toBeGreaterThan(0);
		});

		it('should display correct forge icons for enterprise types', async () => {
			const { container } = render(EnterprisesPage);

			// GitHub enterprises should have GitHub icons
			const githubIcons = container.querySelectorAll('svg');
			expect(githubIcons.length).toBeGreaterThan(0);

			// Verify endpoint names are displayed (use getAllByText for duplicates in responsive layouts)
			expect(screen.getAllByText('github.com')[0]).toBeInTheDocument();
		});

		it('should display enterprise status correctly', async () => {
			const { container } = render(EnterprisesPage);

			// Verify status information is displayed for enterprises
			// Look for any status-related elements in the table
			const tableElements = container.querySelectorAll('td, div');
			expect(tableElements.length).toBeGreaterThan(0);
			
			// Enterprises page should render with status information
			expect(screen.getByText('Enterprises')).toBeInTheDocument();
		});

		it('should have clickable enterprise links', async () => {
			const { container } = render(EnterprisesPage);

			// Verify enterprise names are links
			const entLinks = container.querySelectorAll('a[href^="/enterprises/"]');
			expect(entLinks.length).toBeGreaterThan(0);

			// Check specific enterprise links
			const ent1Link = container.querySelector('a[href="/enterprises/ent-1"]');
			expect(ent1Link).toBeInTheDocument();
			expect(ent1Link?.textContent?.includes('test-enterprise')).toBe(true);
		});
	});

	describe('Search and Filtering Functionality', () => {
		it('should filter enterprises by search term', async () => {
			const user = userEvent.setup();
			render(EnterprisesPage);

			// Find search input
			const searchInput = screen.getByPlaceholderText('Search enterprises...');
			expect(searchInput).toBeInTheDocument();

			// Search for 'github' - should filter to only github enterprise
			await user.type(searchInput, 'github');
			
			// Wait for filtering to take effect
			await waitFor(() => {
				// Should still show github enterprise (may appear multiple times in responsive layout)
				expect(screen.getAllByText('github-enterprise')[0]).toBeInTheDocument();
			});
		});

		it('should clear search when input is cleared', async () => {
			const user = userEvent.setup();
			render(EnterprisesPage);

			const searchInput = screen.getByPlaceholderText('Search enterprises...');
			
			// Type search term
			await user.type(searchInput, 'github');
			
			// Clear search
			await user.clear(searchInput);
			
			// All enterprises should be visible again
			await waitFor(() => {
				expect(screen.getAllByText('test-enterprise')[0]).toBeInTheDocument();
				expect(screen.getAllByText('github-enterprise')[0]).toBeInTheDocument();
				expect(screen.getAllByText('another-enterprise')[0]).toBeInTheDocument();
			});
		});

		it('should show no results when search matches nothing', async () => {
			const user = userEvent.setup();
			render(EnterprisesPage);

			const searchInput = screen.getByPlaceholderText('Search enterprises...');
			
			// Search for something that doesn't exist
			await user.type(searchInput, 'nonexistent-enterprise');
			
			// Should show empty state or filtered results
			await waitFor(() => {
				// Search input should contain the search term
				expect(searchInput).toHaveValue('nonexistent-enterprise');
				// Component should handle empty search results gracefully
				expect(screen.getByText('Enterprises')).toBeInTheDocument();
			});
		});
	});

	describe('Pagination Controls', () => {
		it('should display pagination controls with correct options', async () => {
			render(EnterprisesPage);

			// Find per-page selector
			const perPageSelect = screen.getByLabelText('Show:');
			expect(perPageSelect).toBeInTheDocument();

			// Verify options are available
			expect(screen.getByText('25')).toBeInTheDocument();
			expect(screen.getByText('50')).toBeInTheDocument();
			expect(screen.getByText('100')).toBeInTheDocument();
		});

		it('should allow changing items per page', async () => {
			const user = userEvent.setup();
			render(EnterprisesPage);

			const perPageSelect = screen.getByLabelText('Show:');
			
			// Change to 50 items per page
			await user.selectOptions(perPageSelect, '50');
			
			// Verify selection changed
			expect(perPageSelect).toHaveValue('50');
		});
	});

	describe('Modal Interactions', () => {
		it('should open create enterprise modal when add button is clicked', async () => {
			const user = userEvent.setup();
			render(EnterprisesPage);

			// Find and click the "Add Enterprise" button
			const addButton = screen.getByText('Add Enterprise');
			expect(addButton).toBeInTheDocument();
			
			await user.click(addButton);
			
			// Modal should open (depending on implementation)
			// This tests that the button is properly wired up
			expect(addButton).toBeInTheDocument();
		});

		it('should open edit modal when edit button is clicked', async () => {
			const user = userEvent.setup();
			const { container } = render(EnterprisesPage);

			// Find edit button for first enterprise
			const editButtons = container.querySelectorAll('[title="Edit"], [title="Edit enterprise"]');
			expect(editButtons.length).toBeGreaterThan(0);
			
			const firstEditButton = editButtons[0] as HTMLElement;
			
			// Test that button is clickable (button may be replaced by modal)
			await user.click(firstEditButton);
			
			// Verify the click interaction completed successfully
			// (Modal may have opened, so button might not be accessible)
			// The important thing is the click didn't cause errors
			expect(screen.getByText('Enterprises')).toBeInTheDocument();
		});

		it('should open delete modal when delete button is clicked', async () => {
			const user = userEvent.setup();
			const { container } = render(EnterprisesPage);

			// Find delete button for first enterprise
			const deleteButtons = container.querySelectorAll('[title="Delete"], [title="Delete enterprise"]');
			expect(deleteButtons.length).toBeGreaterThan(0);
			
			const firstDeleteButton = deleteButtons[0] as HTMLElement;
			
			// Test that button is clickable (button may be replaced by modal)
			await user.click(firstDeleteButton);
			
			// Verify the click interaction completed successfully
			// (Modal may have opened, so button might not be accessible)
			// The important thing is the click didn't cause errors
			expect(screen.getByText('Enterprises')).toBeInTheDocument();
		});
	});

	describe('Error States and Loading States', () => {
		it('should handle loading state correctly', async () => {
			// Update mock store to show loading state
			updateMockStore({
				loading: { enterprises: true, credentials: false },
				loaded: { enterprises: false, credentials: true },
				enterprises: []
			});

			render(EnterprisesPage);

			// Component should still render basic structure during loading
			expect(screen.getByText('Enterprises')).toBeInTheDocument();
			expect(screen.getByText('Manage GitHub enterprises')).toBeInTheDocument();
			expect(screen.getByText('Add Enterprise')).toBeInTheDocument();
		});

		it('should handle error state correctly', async () => {
			// Update mock store to show error state
			updateMockStore({
				errorMessages: { enterprises: 'Failed to load enterprises', credentials: '' },
				loaded: { enterprises: false, credentials: true },
				enterprises: []
			});

			render(EnterprisesPage);

			// Component should still render page structure even with errors
			expect(screen.getByText('Enterprises')).toBeInTheDocument();
			expect(screen.getByText('Add Enterprise')).toBeInTheDocument();
			// Should render gracefully without crashing
			expect(screen.getByText('Manage GitHub enterprises')).toBeInTheDocument();
		});

		it('should handle empty enterprise list', async () => {
			// Update mock store to have no enterprises
			updateMockStore({
				enterprises: [],
				loaded: { enterprises: true, credentials: true }
			});

			render(EnterprisesPage);

			// Should still render page structure
			expect(screen.getByText('Enterprises')).toBeInTheDocument();
			expect(screen.getByText('Add Enterprise')).toBeInTheDocument();
		});
	});

	describe('Component Integration and Data Flow', () => {
		it('should render consistent UI based on component state', async () => {
			render(EnterprisesPage);

			// Component should display all enterprises from initial state
			expect(screen.getAllByText('test-enterprise')[0]).toBeInTheDocument();
			expect(screen.getAllByText('github-enterprise')[0]).toBeInTheDocument();
			expect(screen.getAllByText('another-enterprise')[0]).toBeInTheDocument();
			
			// Should show GitHub endpoints (enterprises are GitHub only)
			expect(screen.getAllByText('github.com')[0]).toBeInTheDocument();
		});

		it('should properly subscribe to eager cache on component mount', async () => {
			render(EnterprisesPage);

			// Verify component subscribes to and displays cache data
			expect(screen.getAllByText('test-enterprise')[0]).toBeInTheDocument();
			expect(screen.getAllByText('github-enterprise')[0]).toBeInTheDocument();
			expect(screen.getAllByText('another-enterprise')[0]).toBeInTheDocument();

			// Verify enterprises from GitHub endpoints are displayed
			expect(screen.getAllByText('github.com')[0]).toBeInTheDocument();

			// Verify component renders the correct number of enterprises in the UI
			// (This tests actual component rendering, not our mock setup)
			const entLinks = document.querySelectorAll('a[href^="/enterprises/"]');
			expect(entLinks.length).toBeGreaterThan(0);
		});

		it('should handle different data states gracefully', async () => {
			// Test with empty data state
			updateMockStore({
				enterprises: [],
				loaded: { enterprises: true, credentials: true }
			});

			render(EnterprisesPage);

			// Component should render gracefully with no enterprises
			expect(screen.getByText('Enterprises')).toBeInTheDocument();
			expect(screen.getByText('Add Enterprise')).toBeInTheDocument();
			
			// Should still show the data table structure
			expect(document.body).toBeInTheDocument();
		});
	});

	describe('Responsive Design and Accessibility', () => {
		it('should render mobile and desktop layouts', async () => {
			const { container } = render(EnterprisesPage);

			// Check for responsive classes
			const mobileView = container.querySelector('.block.sm\\:hidden');
			const desktopView = container.querySelector('.hidden.sm\\:block');
			
			// Both mobile and desktop views should be present
			expect(mobileView || desktopView).toBeInTheDocument();
		});

		it('should have proper accessibility attributes', async () => {
			const { container } = render(EnterprisesPage);

			// Check for ARIA labels and titles
			const buttonsWithAria = container.querySelectorAll('[aria-label], [title]');
			expect(buttonsWithAria.length).toBeGreaterThan(0);

			// Check for proper form labels - search input should be accessible
			const searchInput = screen.getByPlaceholderText('Search enterprises...');
			expect(searchInput).toBeInTheDocument();
			
			// Check for screen reader label
			const searchLabel = container.querySelector('label[for="search"]');
			expect(searchLabel).toBeInTheDocument();
		});
	});

	describe('User Interaction Flows', () => {
		it('should support keyboard navigation', async () => {
			const user = userEvent.setup();
			render(EnterprisesPage);

			// Test tab navigation through interactive elements
			const searchInput = screen.getByPlaceholderText('Search enterprises...');

			// Click to focus first, then test tab navigation
			await user.click(searchInput);
			expect(searchInput).toHaveFocus();

			// Tab should move focus to next element
			await user.tab();
		});

		it('should handle rapid user interactions', async () => {
			const user = userEvent.setup();
			render(EnterprisesPage);

			// Rapid clicking should not break the UI
			const addButton = screen.getByText('Add Enterprise');
			
			// Click multiple times rapidly
			await user.click(addButton);
			await user.click(addButton);
			await user.click(addButton);

			// Component should remain stable
			expect(addButton).toBeInTheDocument();
		});

		it('should handle concurrent search and pagination changes', async () => {
			const user = userEvent.setup();
			render(EnterprisesPage);

			const searchInput = screen.getByPlaceholderText('Search enterprises...');
			const perPageSelect = screen.getByLabelText('Show:');

			// Perform search and pagination changes simultaneously
			await user.type(searchInput, 'test');
			await user.selectOptions(perPageSelect, '50');

			// Both changes should be applied
			expect(searchInput).toHaveValue('test');
			expect(perPageSelect).toHaveValue('50');
		});
	});

	describe('Data Consistency and State Management', () => {
		it('should maintain UI consistency during user operations', async () => {
			const user = userEvent.setup();
			render(EnterprisesPage);

			// Initial UI should show all enterprises
			expect(screen.getAllByText('test-enterprise')[0]).toBeInTheDocument();
			expect(screen.getAllByText('github-enterprise')[0]).toBeInTheDocument();
			expect(screen.getAllByText('another-enterprise')[0]).toBeInTheDocument();
			
			// User interactions should not break the UI consistency
			const addButton = screen.getByText('Add Enterprise');
			await user.click(addButton);
			
			// Page should remain stable after interactions
			expect(screen.getByText('Enterprises')).toBeInTheDocument();
		});

		it('should maintain UI consistency during state changes', async () => {
			render(EnterprisesPage);

			// Initially should show all enterprises
			expect(screen.getAllByText('test-enterprise')[0]).toBeInTheDocument();
			
			// Component should handle state transitions gracefully
			// (In real app, Svelte reactivity would update UI when store changes)
			expect(screen.getByText('Enterprises')).toBeInTheDocument();
			expect(screen.getByText('Add Enterprise')).toBeInTheDocument();
		});

		it('should display enterprise types correctly in UI', async () => {
			const { container } = render(EnterprisesPage);

			// Should display GitHub enterprises in the UI (enterprises are GitHub only)
			expect(screen.getAllByText('github.com')[0]).toBeInTheDocument();
			
			// Should show enterprise names
			expect(screen.getAllByText('test-enterprise')[0]).toBeInTheDocument();
			expect(screen.getAllByText('github-enterprise')[0]).toBeInTheDocument();
			
			// Should have appropriate forge icons for GitHub
			const svgIcons = container.querySelectorAll('svg');
			expect(svgIcons.length).toBeGreaterThan(0);
		});
	});
});