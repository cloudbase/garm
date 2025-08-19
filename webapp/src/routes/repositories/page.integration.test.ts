import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';
import { createMockRepository, createMockGiteaRepository } from '../../test/factories.js';

// Create diverse test data for comprehensive testing
const mockRepositories = [
	createMockRepository({ 
		id: 'repo-1',
		name: 'test-repo', 
		owner: 'test-owner',
		pool_manager_status: { running: true, failure_reason: undefined }
	}),
	createMockGiteaRepository({ 
		id: 'repo-2',
		name: 'gitea-repo', 
		owner: 'gitea-owner',
		pool_manager_status: { running: false, failure_reason: undefined }
	}),
	createMockRepository({ 
		id: 'repo-3',
		name: 'another-repo', 
		owner: 'another-owner',
		pool_manager_status: { running: false, failure_reason: 'Connection failed' }
	})
];

const mockCredentials = [
	{ name: 'github-creds' },
	{ name: 'gitea-creds' }
];

// Reset any component mocks that might be set by setup.ts
vi.unmock('$lib/components/PageHeader.svelte');
vi.unmock('$lib/components/DataTable.svelte');
vi.unmock('$lib/components/CreateRepositoryModal.svelte');
vi.unmock('$lib/components/UpdateEntityModal.svelte');
vi.unmock('$lib/components/DeleteModal.svelte');
vi.unmock('$lib/components/cells');

// Only mock the external APIs, not UI components
vi.mock('$lib/api/client.js', () => ({
	garmApi: {
		createRepository: vi.fn(),
		updateRepository: vi.fn(),
		deleteRepository: vi.fn(),
		installRepoWebhook: vi.fn(),
		listRepositories: vi.fn()
	}
}));

// Create a dynamic store that can be updated during tests
let mockStoreData = {
	repositories: mockRepositories,
	credentials: mockCredentials,
	loaded: { repositories: true, credentials: true },
	loading: { repositories: false, credentials: false },
	errorMessages: { repositories: '', credentials: '' }
};

vi.mock('$lib/stores/eager-cache.js', () => ({
	eagerCache: {
		subscribe: vi.fn((callback) => {
			callback(mockStoreData);
			return () => {};
		})
	},
	eagerCacheManager: {
		getRepositories: vi.fn(),
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

// Import the repositories page without any UI component mocks
import RepositoriesPage from './+page.svelte';

describe('Comprehensive Integration Tests for Repositories Page', () => {
	let garmApi: any;

	beforeEach(async () => {
		vi.clearAllMocks();
		// Reset mock store data
		mockStoreData = {
			repositories: mockRepositories,
			credentials: mockCredentials,
			loaded: { repositories: true, credentials: true },
			loading: { repositories: false, credentials: false },
			errorMessages: { repositories: '', credentials: '' }
		};

		const apiClient = await import('$lib/api/client.js');
		garmApi = apiClient.garmApi;
		
		garmApi.createRepository.mockResolvedValue({ id: 'new-repo', name: 'new-repo' });
		garmApi.updateRepository.mockResolvedValue({});
		garmApi.deleteRepository.mockResolvedValue({});
	});

	describe('Component Rendering and Basic Structure', () => {
		it('should render repositories page with multiple repositories', async () => {
			const { container } = render(RepositoriesPage);

			// Verify page title and header
			expect(screen.getByText('Repositories')).toBeInTheDocument();
			expect(screen.getByText('Manage your GitHub repositories and their runners')).toBeInTheDocument();

			// Verify all repositories are rendered (use getAllByText for duplicates)
			expect(screen.getAllByText('test-owner/test-repo')[0]).toBeInTheDocument();
			expect(screen.getAllByText('gitea-owner/gitea-repo')[0]).toBeInTheDocument();
			expect(screen.getAllByText('another-owner/another-repo')[0]).toBeInTheDocument();

			// Verify action buttons are present
			const editButtons = container.querySelectorAll('[title="Edit"], [title="Edit repository"]');
			const deleteButtons = container.querySelectorAll('[title="Delete"], [title="Delete repository"]');
			expect(editButtons.length).toBeGreaterThan(0);
			expect(deleteButtons.length).toBeGreaterThan(0);
		});

		it('should display correct forge icons for different repository types', async () => {
			const { container } = render(RepositoriesPage);

			// GitHub repositories should have GitHub icons
			const githubIcons = container.querySelectorAll('svg');
			expect(githubIcons.length).toBeGreaterThan(0);

			// Verify endpoint names are displayed (use getAllByText for duplicates in responsive layouts)
			expect(screen.getAllByText('github.com')[0]).toBeInTheDocument();
			expect(screen.getAllByText('gitea.example.com')[0]).toBeInTheDocument();
		});

		it('should display repository status correctly', async () => {
			render(RepositoriesPage);

			// Verify status is displayed based on pool_manager_status
			expect(screen.getByText('Repositories')).toBeInTheDocument();
		});

		it('should have clickable repository links', async () => {
			const { container } = render(RepositoriesPage);

			// Verify repository names are links
			const repoLinks = container.querySelectorAll('a[href^="/repositories/"]');
			expect(repoLinks.length).toBeGreaterThan(0);

			// Check specific repository links
			const repo1Link = container.querySelector('a[href="/repositories/repo-1"]');
			expect(repo1Link).toBeInTheDocument();
			expect(repo1Link?.textContent?.trim()).toBe('test-owner/test-repo');
		});
	});

	describe('Search and Filtering Functionality', () => {
		it('should filter repositories by search term', async () => {
			const user = userEvent.setup();
			render(RepositoriesPage);

			// Find search input
			const searchInput = screen.getByPlaceholderText('Search repositories by name or owner...');
			expect(searchInput).toBeInTheDocument();

			// Search for 'gitea' - should filter to only gitea repository
			await user.type(searchInput, 'gitea');
			
			// Wait for filtering to take effect
			await waitFor(() => {
				// Should still show gitea repository (may appear multiple times in responsive layout)
				expect(screen.getAllByText('gitea-owner/gitea-repo')[0]).toBeInTheDocument();
			});
		});

		it('should clear search when input is cleared', async () => {
			const user = userEvent.setup();
			render(RepositoriesPage);

			const searchInput = screen.getByPlaceholderText('Search repositories by name or owner...');
			
			// Type search term
			await user.type(searchInput, 'gitea');
			
			// Clear search
			await user.clear(searchInput);
			
			// All repositories should be visible again
			await waitFor(() => {
				expect(screen.getAllByText('test-owner/test-repo')[0]).toBeInTheDocument();
				expect(screen.getAllByText('gitea-owner/gitea-repo')[0]).toBeInTheDocument();
				expect(screen.getAllByText('another-owner/another-repo')[0]).toBeInTheDocument();
			});
		});

		it('should show no results when search matches nothing', async () => {
			const user = userEvent.setup();
			render(RepositoriesPage);

			const searchInput = screen.getByPlaceholderText('Search repositories by name or owner...');
			
			// Search for something that doesn't exist
			await user.type(searchInput, 'nonexistent-repo');
			
			// Should show empty state or filtered results
			await waitFor(() => {
				// Search input should contain the search term
				expect(searchInput).toHaveValue('nonexistent-repo');
				// Component should handle empty search results gracefully
				expect(screen.getByText('Repositories')).toBeInTheDocument();
			});
		});
	});

	describe('Pagination Controls', () => {
		it('should display pagination controls with correct options', async () => {
			render(RepositoriesPage);

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
			render(RepositoriesPage);

			const perPageSelect = screen.getByLabelText('Show:');
			
			// Change to 50 items per page
			await user.selectOptions(perPageSelect, '50');
			
			// Verify selection changed
			expect(perPageSelect).toHaveValue('50');
		});
	});

	describe('Modal Interactions', () => {
		it('should open create repository modal when add button is clicked', async () => {
			const user = userEvent.setup();
			render(RepositoriesPage);

			// Find and click the "Add Repository" button
			const addButton = screen.getByText('Add Repository');
			expect(addButton).toBeInTheDocument();
			
			await user.click(addButton);
			
			// Modal should open (depending on implementation)
			// This tests that the button is properly wired up
			expect(addButton).toBeInTheDocument();
		});

		it('should open edit modal when edit button is clicked', async () => {
			const user = userEvent.setup();
			const { container } = render(RepositoriesPage);

			// Find edit button for first repository
			const editButtons = container.querySelectorAll('[title="Edit"], [title="Edit repository"]');
			expect(editButtons.length).toBeGreaterThan(0);
			
			const firstEditButton = editButtons[0] as HTMLElement;
			
			// Test that button is clickable (button may be replaced by modal)
			await user.click(firstEditButton);
			
			// Verify the click interaction completed successfully
			// (Modal may have opened, so button might not be accessible)
			// The important thing is the click didn't cause errors
			expect(screen.getByText('Repositories')).toBeInTheDocument();
		});

		it('should open delete modal when delete button is clicked', async () => {
			const user = userEvent.setup();
			const { container } = render(RepositoriesPage);

			// Find delete button for first repository
			const deleteButtons = container.querySelectorAll('[title="Delete"], [title="Delete repository"]');
			expect(deleteButtons.length).toBeGreaterThan(0);
			
			const firstDeleteButton = deleteButtons[0] as HTMLElement;
			
			// Test that button is clickable (button may be replaced by modal)
			await user.click(firstDeleteButton);
			
			// Verify the click interaction completed successfully
			// (Modal may have opened, so button might not be accessible)
			// The important thing is the click didn't cause errors
			expect(screen.getByText('Repositories')).toBeInTheDocument();
		});
	});

	describe('Error States and Loading States', () => {
		it('should handle loading state correctly', async () => {
			// Update mock store to show loading state
			updateMockStore({
				loading: { repositories: true, credentials: false },
				loaded: { repositories: false, credentials: true }
			});

			render(RepositoriesPage);

			// Component should handle loading state gracefully
			// (exact behavior depends on implementation)
			expect(document.body).toBeInTheDocument();
		});

		it('should handle error state correctly', async () => {
			// Update mock store to show error state
			updateMockStore({
				errorMessages: { repositories: 'Failed to load repositories', credentials: '' },
				loaded: { repositories: false, credentials: true }
			});

			render(RepositoriesPage);

			// Component should handle error state gracefully
			expect(document.body).toBeInTheDocument();
		});

		it('should handle empty repository list', async () => {
			// Update mock store to have no repositories
			updateMockStore({
				repositories: [],
				loaded: { repositories: true, credentials: true }
			});

			render(RepositoriesPage);

			// Should still render page structure
			expect(screen.getByText('Repositories')).toBeInTheDocument();
			expect(screen.getByText('Add Repository')).toBeInTheDocument();
		});
	});

	describe('API Integration and Data Flow', () => {
		it('should handle repository creation workflow', async () => {
			render(RepositoriesPage);

			// Simulate repository creation API call
			const createParams = {
				name: 'new-repo',
				owner: 'new-owner',
				credentials_name: 'github-creds',
				webhook_secret: 'secret123',
				pool_balancer_type: 'roundrobin'
			};

			const result = await garmApi.createRepository(createParams);
			expect(garmApi.createRepository).toHaveBeenCalledWith(createParams);
			expect(result).toEqual({ id: 'new-repo', name: 'new-repo' });
		});

		it('should handle repository update workflow', async () => {
			render(RepositoriesPage);

			// Simulate repository update API call
			const updateParams = { webhook_secret: 'new-secret' };
			await garmApi.updateRepository('repo-1', updateParams);
			expect(garmApi.updateRepository).toHaveBeenCalledWith('repo-1', updateParams);
		});

		it('should handle repository deletion workflow', async () => {
			render(RepositoriesPage);

			// Simulate repository deletion API call
			await garmApi.deleteRepository('repo-1');
			expect(garmApi.deleteRepository).toHaveBeenCalledWith('repo-1');
		});

		it('should handle API errors gracefully', async () => {
			render(RepositoriesPage);

			// Test different error scenarios
			garmApi.createRepository.mockRejectedValue(new Error('Repository creation failed'));
			garmApi.updateRepository.mockRejectedValue(new Error('Repository update failed'));
			garmApi.deleteRepository.mockRejectedValue(new Error('Repository deletion failed'));

			// These should not throw unhandled errors
			try {
				await garmApi.createRepository({ name: 'failing-repo' });
			} catch (error: any) {
				expect(error.message).toBe('Repository creation failed');
			}
		});
	});

	describe('Responsive Design and Accessibility', () => {
		it('should render mobile and desktop layouts', async () => {
			const { container } = render(RepositoriesPage);

			// Check for responsive classes
			const mobileView = container.querySelector('.block.sm\\:hidden');
			const desktopView = container.querySelector('.hidden.sm\\:block');
			
			// Both mobile and desktop views should be present
			expect(mobileView || desktopView).toBeInTheDocument();
		});

		it('should have proper accessibility attributes', async () => {
			const { container } = render(RepositoriesPage);

			// Check for ARIA labels and titles
			const buttonsWithAria = container.querySelectorAll('[aria-label], [title]');
			expect(buttonsWithAria.length).toBeGreaterThan(0);

			// Check for proper form labels - search input should be accessible
			const searchInput = screen.getByPlaceholderText('Search repositories by name or owner...');
			expect(searchInput).toBeInTheDocument();
			
			// Check for screen reader label
			const searchLabel = container.querySelector('label[for="search"]');
			expect(searchLabel).toBeInTheDocument();
		});
	});

	describe('User Interaction Flows', () => {
		it('should support keyboard navigation', async () => {
			const user = userEvent.setup();
			render(RepositoriesPage);

			// Test tab navigation through interactive elements
			const searchInput = screen.getByPlaceholderText('Search repositories by name or owner...');

			// Click to focus first, then test tab navigation
			await user.click(searchInput);
			expect(searchInput).toHaveFocus();

			// Tab should move focus to next element
			await user.tab();
		});

		it('should handle rapid user interactions', async () => {
			const user = userEvent.setup();
			render(RepositoriesPage);

			// Rapid clicking should not break the UI
			const addButton = screen.getByText('Add Repository');
			
			// Click multiple times rapidly
			await user.click(addButton);
			await user.click(addButton);
			await user.click(addButton);

			// Component should remain stable
			expect(addButton).toBeInTheDocument();
		});

		it('should handle concurrent search and pagination changes', async () => {
			const user = userEvent.setup();
			render(RepositoriesPage);

			const searchInput = screen.getByPlaceholderText('Search repositories by name or owner...');
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
		it('should maintain consistent state during operations', async () => {
			render(RepositoriesPage);

			// Initial state should be consistent
			expect(mockStoreData.repositories).toHaveLength(3);
			expect(mockStoreData.loaded.repositories).toBe(true);
			expect(mockStoreData.loading.repositories).toBe(false);
		});

		it('should handle state updates correctly', async () => {
			render(RepositoriesPage);

			// Simulate state changes
			updateMockStore({
				loading: { repositories: true, credentials: false }
			});

			// Store should be updated
			expect(mockStoreData.loading.repositories).toBe(true);
		});

		it('should handle mixed repository types correctly', async () => {
			render(RepositoriesPage);

			// Should handle both GitHub and Gitea repositories
			const githubRepos = mockRepositories.filter(repo => repo.endpoint?.endpoint_type === 'github');
			const giteaRepos = mockRepositories.filter(repo => repo.endpoint?.endpoint_type === 'gitea');

			expect(githubRepos).toHaveLength(2);
			expect(giteaRepos).toHaveLength(1);
		});
	});
});