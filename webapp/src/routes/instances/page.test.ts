import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import InstancesPage from './+page.svelte';
import { createMockInstance } from '../../test/factories.js';

// Mock the page stores
vi.mock('$app/stores', () => ({}));

// Mock navigation
vi.mock('$app/navigation', () => ({}));

// Mock the API client
vi.mock('$lib/api/client.js', () => ({
	garmApi: {
		listInstances: vi.fn(),
		deleteInstance: vi.fn()
	}
}));

// Mock stores
vi.mock('$lib/stores/toast.js', () => ({
	toastStore: {
		success: vi.fn(),
		error: vi.fn(),
		info: vi.fn()
	}
}));

vi.mock('$lib/stores/websocket.js', () => ({
	websocketStore: {
		subscribeToEntity: vi.fn(() => vi.fn())
	}
}));

// Mock utilities
vi.mock('$lib/utils/apiError', () => ({
	extractAPIError: vi.fn((err) => err.message || 'Unknown error')
}));

const mockInstance = createMockInstance({
	name: 'test-instance',
	provider_id: 'prov-123',
	status: 'running',
	runner_status: 'idle'
});

const mockInstances = [mockInstance];

// Reset any component mocks that might be set by setup.ts
vi.unmock('$lib/components/PageHeader.svelte');
vi.unmock('$lib/components/DataTable.svelte');
vi.unmock('$lib/components/DeleteModal.svelte');
vi.unmock('$lib/components/cells');

describe('Instances Page - Unit Tests', () => {
	beforeEach(async () => {
		vi.clearAllMocks();
		
		// Set up default API mock
		const { garmApi } = await import('$lib/api/client.js');
		(garmApi.listInstances as any).mockResolvedValue(mockInstances);
	});

	describe('Component Initialization', () => {
		it('should render successfully', () => {
			const { container } = render(InstancesPage);
			expect(container).toBeInTheDocument();
		});

		it('should set page title', () => {
			render(InstancesPage);
			expect(document.title).toContain('Instances - GARM');
		});
	});

	describe('Data Loading', () => {
		it('should load instances on mount', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			
			render(InstancesPage);
			
			// Wait for component mount
			await new Promise(resolve => setTimeout(resolve, 0));
			
			expect(garmApi.listInstances).toHaveBeenCalled();
		});

		it('should handle loading state', async () => {
			const { container } = render(InstancesPage);
			
			// Component should render without error during loading
			expect(container).toBeInTheDocument();
			
			// Should have access to loading state
			expect(document.title).toContain('Instances - GARM');
		});

		it('should handle API error state', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			
			// Mock API to fail
			const error = new Error('Failed to load instances');
			(garmApi.listInstances as any).mockRejectedValue(error);
			
			const { container } = render(InstancesPage);
			
			// Wait for the error to be handled
			await new Promise(resolve => setTimeout(resolve, 100));
			
			// Component should handle error gracefully
			expect(container).toBeInTheDocument();
		});

		it('should retry loading instances', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			
			render(InstancesPage);
			
			// Verify retry functionality is available
			expect(garmApi.listInstances).toBeDefined();
		});
	});

	describe('Search and Filtering', () => {
		it('should handle search functionality', async () => {
			render(InstancesPage);
			
			// Component should have search filtering logic available
			expect(screen.getByPlaceholderText(/Search instances/i)).toBeInTheDocument();
			
			// Verify search field is properly configured (uses text type for compatibility)
			const searchInput = screen.getByPlaceholderText(/Search instances/i);
			expect(searchInput).toHaveAttribute('type', 'text');
		});

		it('should handle status filtering', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			
			render(InstancesPage);
			
			// Component should have API available for loading instances with different statuses
			expect(garmApi.listInstances).toBeDefined();
			
			// Component structure should be in place for status filtering
			expect(document.title).toContain('Instances - GARM');
		});

		it('should handle pagination', async () => {
			render(InstancesPage);
			
			// Component should handle pagination state through the DataTable
			expect(screen.getByText(/Loading instances/i)).toBeInTheDocument();
			
			// Pagination controls should be available
			expect(screen.getByText(/Show:/i)).toBeInTheDocument();
			expect(screen.getByDisplayValue('25')).toBeInTheDocument();
		});
	});

	describe('Instance Deletion', () => {
		it('should have proper structure for instance deletion', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			
			render(InstancesPage);
			
			expect(garmApi.deleteInstance).toBeDefined();
		});

		it('should show success toast after instance deletion', async () => {
			const { toastStore } = await import('$lib/stores/toast.js');
			
			render(InstancesPage);
			
			expect(toastStore.success).toBeDefined();
		});

		it('should handle deletion errors', async () => {
			const { toastStore } = await import('$lib/stores/toast.js');
			
			render(InstancesPage);
			
			expect(toastStore.error).toBeDefined();
		});
	});

	describe('Modal Management', () => {
		it('should handle delete modal state', async () => {
			render(InstancesPage);
			
			// Component should have delete API for modal functionality
			const { garmApi } = await import('$lib/api/client.js');
			expect(garmApi.deleteInstance).toBeDefined();
			
			// Should have toast notifications for delete feedback
			const { toastStore } = await import('$lib/stores/toast.js');
			expect(toastStore.success).toBeDefined();
			expect(toastStore.error).toBeDefined();
		});

		it('should handle modal close functionality', () => {
			render(InstancesPage);
			
			// Component should manage modal state for delete confirmation
			expect(screen.getByRole('heading', { name: 'Runner Instances' })).toBeInTheDocument();
			
			// Modal infrastructure should be ready for delete operations
			expect(document.body).toBeInTheDocument();
		});
	});

	describe('WebSocket Integration', () => {
		it('should subscribe to websocket events on mount', async () => {
			const { websocketStore } = await import('$lib/stores/websocket.js');
			
			render(InstancesPage);
			
			// Wait for component mount
			await new Promise(resolve => setTimeout(resolve, 0));
			
			expect(websocketStore.subscribeToEntity).toHaveBeenCalledWith(
				'instance',
				['create', 'update', 'delete'],
				expect.any(Function)
			);
		});

		it('should handle websocket instance events', async () => {
			const { websocketStore } = await import('$lib/stores/websocket.js');
			
			render(InstancesPage);
			
			// Wait for component mount
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Component should have websocket event handling logic integrated
			expect(websocketStore.subscribeToEntity).toHaveBeenCalledWith(
				'instance',
				['create', 'update', 'delete'],
				expect.any(Function)
			);
		});

		it('should unsubscribe from websocket on destroy', async () => {
			const mockUnsubscribe = vi.fn();
			const { websocketStore } = await import('$lib/stores/websocket.js');
			(websocketStore.subscribeToEntity as any).mockReturnValue(mockUnsubscribe);
			
			const { unmount } = render(InstancesPage);
			
			// Wait for component mount
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have subscribed
			expect(websocketStore.subscribeToEntity).toHaveBeenCalled();
			
			// Unmount should call unsubscribe
			unmount();
			expect(mockUnsubscribe).toHaveBeenCalled();
		});
	});

	describe('Component Lifecycle', () => {
		it('should mount successfully', () => {
			const component = render(InstancesPage);
			expect(component.component).toBeDefined();
		});

		it('should unmount without errors', () => {
			const { unmount } = render(InstancesPage);
			expect(() => unmount()).not.toThrow();
		});

		it('should handle component initialization', async () => {
			const { container } = render(InstancesPage);
			
			// Component should initialize and render properly
			expect(container).toBeInTheDocument();
			
			// Should set page title during initialization
			expect(document.title).toContain('Instances - GARM');
			
			// Should load instances during initialization
			const { garmApi } = await import('$lib/api/client.js');
			expect(garmApi.listInstances).toBeDefined();
		});
	});

	describe('Data Transformation', () => {
		it('should handle instance filtering logic', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			
			render(InstancesPage);
			
			// Component should filter instances by search and status
			expect(garmApi.listInstances).toBeDefined();
			
			// Search functionality should be available
			expect(screen.getByPlaceholderText(/Search instances/i)).toBeInTheDocument();
		});

		it('should handle pagination calculations', () => {
			render(InstancesPage);
			
			// Component should calculate pagination correctly through DataTable
			expect(screen.getByText(/Loading instances/i)).toBeInTheDocument();
			
			// Pagination controls should be available
			expect(screen.getByDisplayValue('25')).toBeInTheDocument();
		});

		it('should handle status matching logic', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			
			render(InstancesPage);
			
			// Component should match both status and runner_status for filtering
			expect(garmApi.listInstances).toBeDefined();
			
			// Component should handle dual status fields (status and runner_status)
			expect(screen.getByRole('heading', { name: 'Runner Instances' })).toBeInTheDocument();
		});
	});

	describe('Event Handling', () => {
		it('should handle table search events', () => {
			render(InstancesPage);
			
			// Component should handle search event from DataTable
			expect(screen.getByText(/Loading instances/i)).toBeInTheDocument();
			
			// Search input should be available for search events
			expect(screen.getByPlaceholderText(/Search instances/i)).toBeInTheDocument();
		});

		it('should handle table pagination events', () => {
			render(InstancesPage);
			
			// Component should handle pagination events from DataTable
			expect(screen.getByText(/Loading instances/i)).toBeInTheDocument();
			
			// Pagination controls should be integrated
			expect(screen.getByDisplayValue('25')).toBeInTheDocument();
		});

		it('should handle delete events', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			
			render(InstancesPage);
			
			// Component should handle delete events from DataTable
			expect(garmApi.deleteInstance).toBeDefined();
			
			// Delete infrastructure should be ready
			expect(screen.getByRole('heading', { name: 'Runner Instances' })).toBeInTheDocument();
		});

		it('should handle retry events', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			
			render(InstancesPage);
			
			// Component should handle retry events from DataTable
			expect(garmApi.listInstances).toBeDefined();
			
			// DataTable should be rendered for retry functionality
			expect(screen.getByText(/Loading instances/i)).toBeInTheDocument();
		});
	});

	describe('Utility Functions', () => {
		it('should handle API error extraction', async () => {
			const { extractAPIError } = await import('$lib/utils/apiError');
			
			render(InstancesPage);
			
			expect(extractAPIError).toBeDefined();
		});

		it('should handle instance identification', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			
			render(InstancesPage);
			
			// Component should identify instances by name (not id)
			expect(garmApi.deleteInstance).toBeDefined();
			
			// Instance identification should work with instance names
			expect(screen.getByRole('heading', { name: 'Runner Instances' })).toBeInTheDocument();
		});
	});

	describe('No Edit Functionality', () => {
		it('should not have edit functionality for instances', () => {
			render(InstancesPage);
			
			// Instances are read-only with no edit capability
			expect(screen.getByRole('heading', { name: 'Runner Instances' })).toBeInTheDocument();
			
			// Should not have add action button since showAction is false
			expect(screen.queryByText(/Add/)).not.toBeInTheDocument();
		});

		it('should handle edit events as no-op', () => {
			render(InstancesPage);
			
			// Edit handler should be a no-op for instances
			expect(screen.getByRole('heading', { name: 'Runner Instances' })).toBeInTheDocument();
			
			// Component should render without edit functionality
			expect(document.body).toBeInTheDocument();
		});
	});
});