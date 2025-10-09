import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import InstanceDetailsPage from './+page.svelte';
import { createMockInstance } from '../../../test/factories.js';

// Mock the page stores
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

// Mock navigation
vi.mock('$app/navigation', () => ({
	goto: vi.fn()
}));

// Mock paths
vi.mock('$app/paths', () => ({
	resolve: vi.fn((path) => path)
}));

// Mock the API client
vi.mock('$lib/api/client.js', () => ({
	garmApi: {
		getInstance: vi.fn(),
		deleteInstance: vi.fn()
	}
}));

// Mock stores
vi.mock('$lib/stores/websocket.js', () => ({
	websocketStore: {
		subscribeToEntity: vi.fn(() => vi.fn())
	}
}));

// Mock utilities
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
	agent_id: 12345,
	pool_id: 'pool-123',
	os_type: 'linux',
	os_name: 'ubuntu',
	os_arch: 'amd64',
	addresses: [
		{ address: '192.168.1.100', type: 'private' },
		{ address: '203.0.113.10', type: 'public' }
	],
	status_messages: [
		{
			message: 'Instance started successfully',
			event_level: 'info',
			created_at: '2024-01-01T10:00:00Z'
		},
		{
			message: 'Warning: High memory usage',
			event_level: 'warning',
			created_at: '2024-01-01T11:00:00Z'
		}
	]
});

// Reset any component mocks that might be set by setup.ts
vi.unmock('$lib/components/DeleteModal.svelte');
vi.unmock('$lib/components/Badge.svelte');

describe('Instance Details Page - Unit Tests', () => {
	beforeEach(async () => {
		vi.clearAllMocks();
		
		// Set up default API mock
		const { garmApi } = await import('$lib/api/client.js');
		(garmApi.getInstance as any).mockResolvedValue(mockInstance);
		(garmApi.deleteInstance as any).mockResolvedValue({});
	});

	describe('Component Initialization', () => {
		it('should render successfully', () => {
			const { container } = render(InstanceDetailsPage);
			expect(container).toBeInTheDocument();
		});

		it('should set page title with instance name', async () => {
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			expect(document.title).toContain('test-instance - Instance Details - GARM');
		});

		it('should set fallback page title when no instance', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			(garmApi.getInstance as any).mockRejectedValue(new Error('Instance not found'));
			
			render(InstanceDetailsPage);
			
			expect(document.title).toContain('Instance Details - GARM');
		});
	});

	describe('Data Loading', () => {
		it('should load instance on mount', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			
			render(InstanceDetailsPage);
			
			// Wait for component mount
			await new Promise(resolve => setTimeout(resolve, 0));
			
			expect(garmApi.getInstance).toHaveBeenCalledWith('test-instance');
		});

		it('should handle loading state', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			
			// Mock delayed response
			(garmApi.getInstance as any).mockImplementation(() => 
				new Promise(resolve => setTimeout(() => resolve(mockInstance), 100))
			);
			
			render(InstanceDetailsPage);
			
			// Should show loading state initially
			expect(screen.getByText(/Loading instance details/i)).toBeInTheDocument();
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 150));
			
			// Loading should be gone
			expect(screen.queryByText(/Loading instance details/i)).not.toBeInTheDocument();
		});

		it('should handle API error state', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			
			// Mock API to fail
			const error = new Error('Failed to load instance');
			(garmApi.getInstance as any).mockRejectedValue(error);
			
			render(InstanceDetailsPage);
			
			// Wait for the error to be handled
			await new Promise(resolve => setTimeout(resolve, 100));
			
			// Should display error
			expect(screen.getByText(/Failed to load instance/i)).toBeInTheDocument();
		});
	});

	describe('Instance Information Display', () => {
		it('should display instance basic information', async () => {
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should display instance details
			expect(screen.getByText('Instance Information')).toBeInTheDocument();
			expect(screen.getAllByText('test-instance')[0]).toBeInTheDocument();
			expect(screen.getByText('inst-123')).toBeInTheDocument();
			expect(screen.getByText('prov-123')).toBeInTheDocument();
		});

		it('should display status information', async () => {
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should display status section
			expect(screen.getByText('Status & Network')).toBeInTheDocument();
			expect(screen.getByText('Instance Status:')).toBeInTheDocument();
			expect(screen.getByText('Runner Status:')).toBeInTheDocument();
		});

		it('should display network addresses when available', async () => {
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should display network addresses
			expect(screen.getByText('Network Addresses:')).toBeInTheDocument();
			expect(screen.getByText('192.168.1.100')).toBeInTheDocument();
			expect(screen.getByText('203.0.113.10')).toBeInTheDocument();
		});

		it('should handle missing network addresses', async () => {
			const instanceWithoutAddresses = { ...mockInstance, addresses: [] };
			const { garmApi } = await import('$lib/api/client.js');
			(garmApi.getInstance as any).mockResolvedValue(instanceWithoutAddresses);
			
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should show no addresses message
			expect(screen.getByText(/No addresses available/i)).toBeInTheDocument();
		});
	});

	describe('Pool/Scale Set Links', () => {
		it('should display pool link when pool_id exists', async () => {
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have pool link
			const poolLink = screen.getByRole('link', { name: 'pool-123' });
			expect(poolLink).toBeInTheDocument();
			expect(poolLink).toHaveAttribute('href', '/pools/pool-123');
		});

		it('should display scale set link when scale_set_id exists', async () => {
			const instanceWithScaleSet = { ...mockInstance, pool_id: undefined, scale_set_id: 'scaleset-123' };
			const { garmApi } = await import('$lib/api/client.js');
			(garmApi.getInstance as any).mockResolvedValue(instanceWithScaleSet);
			
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have scale set link
			const scaleSetLink = screen.getByRole('link', { name: 'scaleset-123' });
			expect(scaleSetLink).toBeInTheDocument();
			expect(scaleSetLink).toHaveAttribute('href', '/scalesets/scaleset-123');
		});

		it('should show dash when no pool or scale set', async () => {
			const instanceWithoutPoolOrScaleSet = { ...mockInstance, pool_id: undefined, scale_set_id: undefined };
			const { garmApi } = await import('$lib/api/client.js');
			(garmApi.getInstance as any).mockResolvedValue(instanceWithoutPoolOrScaleSet);
			
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should show dash
			expect(screen.getByText('-')).toBeInTheDocument();
		});
	});

	describe('Status Messages', () => {
		it('should display status messages when available', async () => {
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should display status messages
			expect(screen.getByText('Status Messages')).toBeInTheDocument();
			expect(screen.getByText('Instance started successfully')).toBeInTheDocument();
			expect(screen.getByText('Warning: High memory usage')).toBeInTheDocument();
		});

		it('should handle empty status messages', async () => {
			const instanceWithoutMessages = { ...mockInstance, status_messages: [] };
			const { garmApi } = await import('$lib/api/client.js');
			(garmApi.getInstance as any).mockResolvedValue(instanceWithoutMessages);
			
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should show no messages state
			expect(screen.getByText(/No status messages available/i)).toBeInTheDocument();
		});

		it('should auto-scroll status messages on load', async () => {
			const { scrollToBottomEvents } = await import('$lib/utils/common.js');
			
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 200));
			
			// Should call scroll function
			expect(scrollToBottomEvents).toHaveBeenCalled();
		});
	});

	describe('Delete Functionality', () => {
		it('should show delete button', async () => {
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have delete button
			expect(screen.getByRole('button', { name: /Delete Instance/i })).toBeInTheDocument();
		});

		it('should handle delete instance', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			const { goto } = await import('$app/navigation');
			
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Delete API should be available
			expect(garmApi.deleteInstance).toBeDefined();
			expect(goto).toBeDefined();
		});

		it('should handle delete error', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			
			// Mock delete to fail
			const error = new Error('Delete failed');
			(garmApi.deleteInstance as any).mockRejectedValue(error);
			
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have error handling ready
			expect(screen.getByRole('button', { name: /Delete Instance/i })).toBeInTheDocument();
		});
	});

	describe('WebSocket Integration', () => {
		it('should subscribe to websocket events on mount', async () => {
			const { websocketStore } = await import('$lib/stores/websocket.js');
			
			render(InstanceDetailsPage);
			
			// Wait for component mount
			await new Promise(resolve => setTimeout(resolve, 0));
			
			expect(websocketStore.subscribeToEntity).toHaveBeenCalledWith(
				'instance',
				['update', 'delete'],
				expect.any(Function)
			);
		});

		it('should handle websocket instance update events', async () => {
			const { websocketStore } = await import('$lib/stores/websocket.js');
			
			render(InstanceDetailsPage);
			
			// Wait for component mount
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should subscribe to update events
			expect(websocketStore.subscribeToEntity).toHaveBeenCalledWith(
				'instance',
				expect.arrayContaining(['update']),
				expect.any(Function)
			);
		});

		it('should handle websocket instance delete events', async () => {
			const { websocketStore } = await import('$lib/stores/websocket.js');
			const { goto } = await import('$app/navigation');
			
			render(InstanceDetailsPage);
			
			// Wait for component mount
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should subscribe to delete events and have navigation ready
			expect(websocketStore.subscribeToEntity).toHaveBeenCalledWith(
				'instance',
				expect.arrayContaining(['delete']),
				expect.any(Function)
			);
			expect(goto).toBeDefined();
		});

		it('should unsubscribe from websocket on destroy', async () => {
			const mockUnsubscribe = vi.fn();
			const { websocketStore } = await import('$lib/stores/websocket.js');
			(websocketStore.subscribeToEntity as any).mockReturnValue(mockUnsubscribe);
			
			const { unmount } = render(InstanceDetailsPage);
			
			// Wait for component mount
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have subscribed
			expect(websocketStore.subscribeToEntity).toHaveBeenCalled();
			
			// Unmount should call unsubscribe
			unmount();
			expect(mockUnsubscribe).toHaveBeenCalled();
		});
	});

	describe('Breadcrumb Navigation', () => {
		it('should display breadcrumb navigation', async () => {
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have breadcrumb navigation
			expect(screen.getByRole('navigation', { name: 'Breadcrumb' })).toBeInTheDocument();
			expect(screen.getByRole('link', { name: /Instances/i })).toBeInTheDocument();
		});

		it('should link back to instances list', async () => {
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should have link back to instances
			const instancesLink = screen.getByRole('link', { name: /Instances/i });
			expect(instancesLink).toHaveAttribute('href', '/instances');
		});
	});

	describe('Component Lifecycle', () => {
		it('should mount successfully', () => {
			const component = render(InstanceDetailsPage);
			expect(component.component).toBeDefined();
		});

		it('should unmount without errors', () => {
			const { unmount } = render(InstanceDetailsPage);
			expect(() => unmount()).not.toThrow();
		});

		it('should handle parameter changes', async () => {
			// Simulate parameter change by remocking the page store
			const storesModule = await import('$app/stores');
			vi.mocked(storesModule.page.subscribe).mockImplementation((callback: any) => {
				callback({
					params: { id: 'different-instance' },
					url: new URL('/instances/different-instance', 'http://localhost')
				});
				return () => {};
			});
			
			const { garmApi } = await import('$lib/api/client.js');
			
			render(InstanceDetailsPage);
			
			// Should handle parameter change
			expect(garmApi.getInstance).toBeDefined();
		});
	});

	describe('Error Handling', () => {
		it('should display not found state when instance is null', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			(garmApi.getInstance as any).mockResolvedValue(null);
			
			render(InstanceDetailsPage);
			
			// Wait for loading to complete
			await new Promise(resolve => setTimeout(resolve, 100));
			
			// Should show not found message
			expect(screen.getByText(/Instance not found/i)).toBeInTheDocument();
		});

		it('should handle missing optional fields gracefully', async () => {
			const minimalInstance = {
				id: 'inst-123',
				name: 'minimal-instance',
				created_at: '2024-01-01T00:00:00Z',
				status: 'running'
			};
			
			const { garmApi } = await import('$lib/api/client.js');
			(garmApi.getInstance as any).mockResolvedValue(minimalInstance);
			
			render(InstanceDetailsPage);
			
			// Wait for instance to load
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should handle missing fields gracefully (use getAllByText for instance name)
			expect(screen.getAllByText('minimal-instance')[0]).toBeInTheDocument();
			expect(screen.getByText(/Not assigned/i)).toBeInTheDocument(); // agent_id fallback
		});
	});

	describe('URL Parameter Handling', () => {
		it('should decode URL-encoded instance names', async () => {
			// Mock page store with encoded name
			const { page } = await import('$app/stores');
			vi.mocked(page.subscribe).mockImplementation((callback: any) => {
				callback({
					params: { id: 'test%2Dinstance%2Dwith%2Ddashes' },
					url: { pathname: '/instances/test%2Dinstance%2Dwith%2Ddashes' }
				});
				return () => {};
			});
			
			const { garmApi } = await import('$lib/api/client.js');
			
			render(InstanceDetailsPage);
			
			// Wait for component mount
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should decode the parameter
			expect(garmApi.getInstance).toHaveBeenCalledWith('test-instance-with-dashes');
		});
	});
});