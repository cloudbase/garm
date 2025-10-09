import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import InstanceDetailsPage from './+page.svelte';
import { createMockInstance } from '../../../test/factories.js';

// Mock all external dependencies
vi.mock('$app/stores', () => ({
	page: {
		subscribe: vi.fn((callback) => {
			callback({
				params: { id: 'test-instance' },
				url: { pathname: '/instances/test-instance' }
			});
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
		getInstance: vi.fn(),
		deleteInstance: vi.fn()
	}
}));

vi.mock('$lib/stores/websocket.js', () => ({
	websocketStore: {
		subscribeToEntity: vi.fn(() => vi.fn())
	}
}));

vi.mock('$lib/utils/apiError', () => ({
	extractAPIError: vi.fn((err) => err.message || 'Unknown error')
}));

vi.mock('$lib/utils/status.js', () => ({
	formatStatusText: vi.fn((status) => {
		if (!status) return 'Unknown';
		return status.charAt(0).toUpperCase() + status.slice(1);
	}),
	getStatusBadgeClass: vi.fn((status) => {
		switch (status) {
			case 'running': return 'bg-green-100 text-green-800 ring-green-200';
			case 'idle': return 'bg-blue-100 text-blue-800 ring-blue-200';
			case 'pending': return 'bg-yellow-100 text-yellow-800 ring-yellow-200';
			case 'error': return 'bg-red-100 text-red-800 ring-red-200';
			default: return 'bg-gray-100 text-gray-800 ring-gray-200';
		}
	})
}));

vi.mock('$lib/utils/common.js', async (importOriginal) => {
	const actual = await importOriginal() as any;
	return {
		...actual,
		// Override only specific functions for testing

	formatDate: vi.fn((date) => new Date(date).toLocaleString()),
	scrollToBottomEvents: vi.fn(),
	getEventLevelBadge: vi.fn((level) => ({
		variant: level === 'error' ? 'danger' : level === 'warning' ? 'warning' : 'info',
		text: level.toUpperCase()
	}))
	};
});

const mockInstance = createMockInstance({
	id: 'inst-123',
	name: 'test-instance',
	provider_id: 'prov-123',
	provider_name: 'test-provider',
	status: 'running',
	runner_status: 'idle',
	pool_id: 'pool-123',
	addresses: [
		{ address: '192.168.1.100', type: 'private' }
	],
	status_messages: [
		{
			message: 'Instance ready',
			event_level: 'info',
			created_at: '2024-01-01T10:00:00Z'
		}
	]
});

// Reset any component mocks that might be set by setup.ts
vi.unmock('$lib/components/DeleteModal.svelte');
vi.unmock('$lib/components/Badge.svelte');

describe('Instance Details Page - Render Tests', () => {
	beforeEach(async () => {
		vi.clearAllMocks();
		
		// Set up default API mocks
		const { garmApi } = await import('$lib/api/client.js');
		(garmApi.getInstance as any).mockResolvedValue(mockInstance);
		(garmApi.deleteInstance as any).mockResolvedValue({});
	});

	describe('Basic Rendering', () => {
		it('should render without crashing', () => {
			const { container } = render(InstanceDetailsPage);
			expect(container).toBeInTheDocument();
		});

		it('should have proper document structure', () => {
			const { container } = render(InstanceDetailsPage);
			expect(container.querySelector('div')).toBeInTheDocument();
		});

		it('should render breadcrumb navigation', async () => {
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have breadcrumb navigation
			expect(screen.getByRole('navigation', { name: 'Breadcrumb' })).toBeInTheDocument();
		});

		it('should render instance information cards', async () => {
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have main content sections
			expect(screen.getByText('Instance Information')).toBeInTheDocument();
			expect(screen.getByText('Status & Network')).toBeInTheDocument();
		});
	});

	describe('Component Lifecycle', () => {
		it('should mount successfully', () => {
			const { component } = render(InstanceDetailsPage);
			expect(component).toBeDefined();
		});

		it('should unmount without errors', () => {
			const { unmount } = render(InstanceDetailsPage);
			expect(() => unmount()).not.toThrow();
		});

		it('should handle component updates', async () => {
			const { component } = render(InstanceDetailsPage);
			
			// Component should handle reactive updates
			expect(component).toBeDefined();
		});

		it('should load instance on mount', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			
			render(InstanceDetailsPage);
			
			// Wait for component mount and data loading
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should call API to load instance
			expect(garmApi.getInstance).toHaveBeenCalledWith('test-instance');
		});

		it('should subscribe to websocket events on mount', async () => {
			const { websocketStore } = await import('$lib/stores/websocket.js');
			
			render(InstanceDetailsPage);
			
			// Wait for component mount
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should subscribe to websocket events
			expect(websocketStore.subscribeToEntity).toHaveBeenCalledWith(
				'instance',
				['update', 'delete'],
				expect.any(Function)
			);
		});
	});

	describe('DOM Structure', () => {
		it('should create proper DOM hierarchy', async () => {
			const { container } = render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have main container with proper spacing
			const mainDiv = container.querySelector('div.space-y-6');
			expect(mainDiv).toBeInTheDocument();
		});

		it('should render svelte:head for page title', async () => {
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should set page title
			expect(document.title).toContain('test-instance - Instance Details - GARM');
		});

		it('should handle error display conditionally', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			(garmApi.getInstance as any).mockRejectedValue(new Error('Test error'));
			
			render(InstanceDetailsPage);
			
			// Wait for error
			await new Promise(resolve => setTimeout(resolve, 100));
			
			// Error display should be conditional
			expect(screen.getByText(/Test error/i)).toBeInTheDocument();
		});

		it('should render loading state initially', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			
			// Mock delayed response
			(garmApi.getInstance as any).mockImplementation(() => 
				new Promise(resolve => setTimeout(() => resolve(mockInstance), 200))
			);
			
			render(InstanceDetailsPage);
			
			// Should show loading initially
			expect(screen.getByText(/Loading instance details/i)).toBeInTheDocument();
		});
	});

	describe('Information Cards Rendering', () => {
		it('should render instance information card', async () => {
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should render instance information card
			expect(screen.getByText('Instance Information')).toBeInTheDocument();
			expect(screen.getByText('ID:')).toBeInTheDocument();
			expect(screen.getByText('Name:')).toBeInTheDocument();
		});

		it('should render status and network card', async () => {
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should render status card
			expect(screen.getByText('Status & Network')).toBeInTheDocument();
			expect(screen.getByText('Instance Status:')).toBeInTheDocument();
			expect(screen.getByText('Runner Status:')).toBeInTheDocument();
		});

		it('should render network addresses section', async () => {
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should render network section
			expect(screen.getByText('Network Addresses:')).toBeInTheDocument();
			expect(screen.getByText('192.168.1.100')).toBeInTheDocument();
		});

		it('should render OS information conditionally', async () => {
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should render OS information when available
			expect(screen.getByText('OS Type:')).toBeInTheDocument();
			expect(screen.getByText('OS Architecture:')).toBeInTheDocument();
		});
	});

	describe('Status Messages Rendering', () => {
		it('should render status messages when available', async () => {
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should render status messages section
			expect(screen.getByText('Status Messages')).toBeInTheDocument();
			expect(screen.getByText('Instance ready')).toBeInTheDocument();
		});

		it('should render empty state when no messages', async () => {
			const instanceWithoutMessages = { ...mockInstance, status_messages: [] };
			const { garmApi } = await import('$lib/api/client.js');
			(garmApi.getInstance as any).mockResolvedValue(instanceWithoutMessages);
			
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should render empty state
			expect(screen.getByText(/No status messages available/i)).toBeInTheDocument();
		});

		it('should render scrollable container for messages', async () => {
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have scrollable container
			const messagesContainer = document.querySelector('.max-h-96.overflow-y-auto');
			expect(messagesContainer).toBeInTheDocument();
		});
	});

	describe('Modal Rendering', () => {
		it('should conditionally render delete modal', async () => {
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Delete modal should not be visible initially (check for modal-specific text)
			expect(screen.queryByText('Are you sure you want to delete this instance? This action cannot be undone.')).not.toBeInTheDocument();
		});

		it('should render delete button', async () => {
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have delete button
			expect(screen.getByRole('button', { name: /Delete Instance/i })).toBeInTheDocument();
		});
	});

	describe('WebSocket Lifecycle', () => {
		it('should clean up websocket subscription on unmount', async () => {
			const mockUnsubscribe = vi.fn();
			const { websocketStore } = await import('$lib/stores/websocket.js');
			(websocketStore.subscribeToEntity as any).mockReturnValue(mockUnsubscribe);
			
			const { unmount } = render(InstanceDetailsPage);
			
			// Wait for mount
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Unmount and verify cleanup
			unmount();
			expect(mockUnsubscribe).toHaveBeenCalled();
		});

		it('should handle websocket subscription errors gracefully', async () => {
			const { websocketStore } = await import('$lib/stores/websocket.js');
			(websocketStore.subscribeToEntity as any).mockReturnValue(null);
			
			// Should render successfully even with websocket issues
			const { container } = render(InstanceDetailsPage);
			expect(container).toBeInTheDocument();
		});
	});

	describe('Navigation Elements', () => {
		it('should render breadcrumb links correctly', async () => {
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have correct breadcrumb structure
			const instancesLink = screen.getByRole('link', { name: /Instances/i });
			expect(instancesLink).toHaveAttribute('href', '/instances');
		});

		it('should render pool/scale set links when available', async () => {
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have pool link
			const poolLink = screen.getByRole('link', { name: 'pool-123' });
			expect(poolLink).toHaveAttribute('href', '/pools/pool-123');
		});
	});

	describe('Conditional Content Rendering', () => {
		it('should render different states based on data availability', async () => {
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should adapt rendering based on available data
			expect(screen.getByText('Instance Information')).toBeInTheDocument();
		});

		it('should handle not found state', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			(garmApi.getInstance as any).mockResolvedValue(null);
			
			render(InstanceDetailsPage);
			
			// Wait for loading to complete
			await new Promise(resolve => setTimeout(resolve, 100));
			
			// Should show not found state
			expect(screen.getByText(/Instance not found/i)).toBeInTheDocument();
		});

		it('should render updated at field conditionally', async () => {
			const instanceWithUpdate = {
				...mockInstance,
				updated_at: '2024-01-02T00:00:00Z'
			};
			const { garmApi } = await import('$lib/api/client.js');
			(garmApi.getInstance as any).mockResolvedValue(instanceWithUpdate);
			
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should show updated at when different from created at
			expect(screen.getByText('Updated At:')).toBeInTheDocument();
		});
	});

	describe('Responsive Layout', () => {
		it('should use responsive grid layout', async () => {
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have responsive grid
			const gridContainer = document.querySelector('.grid.grid-cols-1.lg\\:grid-cols-2');
			expect(gridContainer).toBeInTheDocument();
		});

		it('should handle mobile-friendly layout', async () => {
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have mobile-responsive classes
			expect(document.querySelector('.space-x-1.md\\:space-x-3')).toBeInTheDocument();
		});
	});
});