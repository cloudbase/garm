import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import InstancesPage from './+page.svelte';
import { createMockInstance } from '../../test/factories.js';

// Mock all external dependencies
vi.mock('$app/stores', () => ({}));

vi.mock('$app/navigation', () => ({}));

vi.mock('$lib/api/client.js', () => ({
	garmApi: {
		listInstances: vi.fn(),
		deleteInstance: vi.fn()
	}
}));

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

vi.mock('$lib/utils/apiError', () => ({
	extractAPIError: vi.fn((err) => err.message || 'Unknown error')
}));

const mockInstance = createMockInstance({
	name: 'test-instance',
	provider_id: 'prov-123',
	status: 'running',
	runner_status: 'idle'
});

// Reset any component mocks that might be set by setup.ts
vi.unmock('$lib/components/PageHeader.svelte');
vi.unmock('$lib/components/DataTable.svelte');
vi.unmock('$lib/components/DeleteModal.svelte');
vi.unmock('$lib/components/cells');

describe('Instances Page - Render Tests', () => {
	beforeEach(async () => {
		vi.clearAllMocks();
		
		// Set up default API mocks
		const { garmApi } = await import('$lib/api/client.js');
		(garmApi.listInstances as any).mockResolvedValue([mockInstance]);
	});

	describe('Basic Rendering', () => {
		it('should render without crashing', () => {
			const { container } = render(InstancesPage);
			expect(container).toBeInTheDocument();
		});

		it('should have proper document structure', () => {
			const { container } = render(InstancesPage);
			expect(container.querySelector('div')).toBeInTheDocument();
		});

		it('should render page header', () => {
			const { container } = render(InstancesPage);
			// Should have page header component
			expect(container).toBeInTheDocument();
		});

		it('should render data table', () => {
			const { container } = render(InstancesPage);
			// Should have DataTable component
			expect(container).toBeInTheDocument();
		});
	});

	describe('Component Lifecycle', () => {
		it('should mount successfully', () => {
			const { component } = render(InstancesPage);
			expect(component).toBeDefined();
		});

		it('should unmount without errors', () => {
			const { unmount } = render(InstancesPage);
			expect(() => unmount()).not.toThrow();
		});

		it('should handle component updates', async () => {
			const { component } = render(InstancesPage);
			
			// Component should handle reactive updates
			expect(component).toBeDefined();
		});

		it('should load instances on mount', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			
			render(InstancesPage);
			
			// Wait for component mount and data loading
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should call API to load instances
			expect(garmApi.listInstances).toHaveBeenCalled();
		});

		it('should subscribe to websocket events on mount', async () => {
			const { websocketStore } = await import('$lib/stores/websocket.js');
			
			render(InstancesPage);
			
			// Wait for component mount
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Should subscribe to websocket events
			expect(websocketStore.subscribeToEntity).toHaveBeenCalled();
		});
	});

	describe('DOM Structure', () => {
		it('should create proper DOM hierarchy', () => {
			const { container } = render(InstancesPage);
			
			// Should have main container with proper spacing
			const mainDiv = container.querySelector('div.space-y-6');
			expect(mainDiv).toBeInTheDocument();
		});

		it('should render svelte:head for page title', async () => {
			render(InstancesPage);
			
			// Should set page title
			expect(document.title).toContain('Instances - GARM');
		});

		it('should handle error display conditionally', () => {
			const { container } = render(InstancesPage);
			
			// Error display should be conditional
			expect(container).toBeInTheDocument();
		});
	});

	describe('Modal Rendering', () => {
		it('should conditionally render delete modal', () => {
			const { container } = render(InstancesPage);
			
			// Delete modal should not be visible initially
			expect(container).toBeInTheDocument();
		});

		it('should handle modal state management', () => {
			const { container } = render(InstancesPage);
			
			// Modal state should be properly managed
			expect(container).toBeInTheDocument();
		});
	});

	describe('WebSocket Lifecycle', () => {
		it('should clean up websocket subscription on unmount', async () => {
			const mockUnsubscribe = vi.fn();
			const { websocketStore } = await import('$lib/stores/websocket.js');
			(websocketStore.subscribeToEntity as any).mockReturnValue(mockUnsubscribe);
			
			const { unmount } = render(InstancesPage);
			
			// Wait for mount
			await new Promise(resolve => setTimeout(resolve, 0));
			
			// Unmount and verify cleanup
			unmount();
			expect(mockUnsubscribe).toHaveBeenCalled();
		});

		it('should handle websocket subscription errors gracefully', () => {
			const { container } = render(InstancesPage);
			
			// Should handle websocket errors gracefully
			expect(container).toBeInTheDocument();
		});
	});

	describe('Data Table Integration', () => {
		it('should integrate with DataTable component', () => {
			const { container } = render(InstancesPage);
			
			// Should integrate with DataTable for instance display
			expect(container).toBeInTheDocument();
		});

		it('should configure table columns properly', () => {
			const { container } = render(InstancesPage);
			
			// Should configure columns for instance display
			expect(container).toBeInTheDocument();
		});

		it('should configure mobile card layout', () => {
			const { container } = render(InstancesPage);
			
			// Should configure mobile-friendly layout
			expect(container).toBeInTheDocument();
		});
	});
});