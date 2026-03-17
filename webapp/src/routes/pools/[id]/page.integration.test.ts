import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/svelte';
import { createMockPool, createMockInstance } from '../../../test/factories.js';

// Mock $app/state (pool detail page uses this, not $app/stores)
vi.mock('$app/state', () => ({
	page: {
		params: { id: 'pool-123' },
		url: new URL('http://localhost/pools/pool-123')
	}
}));

vi.mock('$app/navigation', () => ({
	goto: vi.fn()
}));

vi.mock('$app/paths', () => ({
	resolve: vi.fn((path: string) => path)
}));

vi.mock('$lib/api/client.js', () => ({
	garmApi: {
		getPool: vi.fn(),
		updatePool: vi.fn(),
		deletePool: vi.fn(),
		deleteInstance: vi.fn()
	}
}));

vi.mock('$lib/stores/websocket.js', () => ({
	websocketStore: {
		subscribe: vi.fn((callback: any) => {
			callback({ connected: true, connecting: false, error: null });
			return () => {};
		}),
		subscribeToEntity: vi.fn(() => vi.fn())
	}
}));

vi.mock('$lib/stores/eager-cache.js', () => ({
	eagerCache: {
		subscribe: vi.fn((callback: any) => {
			callback({
				repositories: [],
				organizations: [],
				enterprises: [],
				pools: [],
				instances: [],
				loaded: { repositories: false, organizations: false, enterprises: false, pools: false, instances: false },
				loading: { repositories: false, organizations: false, enterprises: false, pools: false, instances: false },
				errorMessages: { repositories: '', organizations: '', enterprises: '', pools: '', instances: '' }
			});
			return () => {};
		})
	},
	eagerCacheManager: {
		getRepositories: vi.fn(),
		getOrganizations: vi.fn(),
		getEnterprises: vi.fn(),
		getPools: vi.fn(),
		getInstances: vi.fn(),
		retryResource: vi.fn()
	}
}));

vi.mock('$lib/stores/toast.js', () => ({
	toastStore: {
		success: vi.fn(),
		error: vi.fn(),
		info: vi.fn(),
		warning: vi.fn(),
		add: vi.fn()
	}
}));

// Reset component mocks from setup.ts so real components render
vi.unmock('$lib/components/DeleteModal.svelte');
vi.unmock('$lib/components/UpdatePoolModal.svelte');
vi.unmock('$lib/components/DetailHeader.svelte');
vi.unmock('$lib/components/InstancesSection.svelte');
vi.unmock('$lib/components/cells');

import PoolDetailPage from './+page.svelte';

const mockPool = createMockPool({
	id: 'pool-123',
	provider_name: 'test-provider',
	image: 'ubuntu:22.04',
	flavor: 'standard',
	enabled: true,
	max_runners: 10,
	min_idle_runners: 2,
	runner_bootstrap_timeout: 20,
	priority: 100,
	runner_prefix: 'garm',
	os_type: 'linux',
	os_arch: 'amd64',
	tags: [{ id: 'tag1', name: 'ubuntu' }, { id: 'tag2', name: 'test' }],
	repo_id: 'repo-123',
	repo_name: 'test-repo',
	instances: [createMockInstance({ name: 'inst-1', status: 'running' })],
	endpoint: {
		name: 'github.com',
		endpoint_type: 'github',
		description: 'GitHub',
		api_base_url: 'https://api.github.com',
		base_url: 'https://github.com',
		upload_base_url: 'https://uploads.github.com',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z'
	}
});

describe('Pool Detail Page Integration Tests', () => {
	beforeEach(async () => {
		vi.clearAllMocks();
		const { garmApi } = await import('$lib/api/client.js');
		(garmApi.getPool as any).mockResolvedValue(mockPool);
	});

	it('should show loading state initially', async () => {
		const { garmApi } = await import('$lib/api/client.js');
		// Delay the API response so loading state is visible
		(garmApi.getPool as any).mockImplementation(
			() => new Promise((resolve) => setTimeout(() => resolve(mockPool), 100))
		);

		render(PoolDetailPage);

		// Loading text should appear while the API call is pending
		expect(screen.getByText('Loading pool...')).toBeInTheDocument();

		// After API resolves, loading should go away and pool data should appear
		await waitFor(() => {
			expect(screen.queryByText('Loading pool...')).not.toBeInTheDocument();
			expect(screen.getByText('Basic Information')).toBeInTheDocument();
		});
	});

	it('should render pool details after loading', async () => {
		render(PoolDetailPage);

		await waitFor(() => {
			// Pool ID shown in Basic Information section
			expect(screen.getAllByText('pool-123').length).toBeGreaterThan(0);
			// Provider name
			expect(screen.getAllByText('test-provider').length).toBeGreaterThan(0);
			// Image displayed inside a code element
			expect(screen.getAllByText('ubuntu:22.04').length).toBeGreaterThan(0);
			// Flavor
			expect(screen.getAllByText('standard').length).toBeGreaterThan(0);
		});
	});

	it('should show breadcrumb navigation with Pools link', async () => {
		render(PoolDetailPage);

		// Breadcrumb nav is rendered immediately (before data loads)
		expect(screen.getByRole('navigation', { name: 'Breadcrumb' })).toBeInTheDocument();

		// The "Pools" breadcrumb link should point to /pools
		const poolsLink = screen.getByRole('link', { name: /Pools/i });
		expect(poolsLink).toBeInTheDocument();
		expect(poolsLink).toHaveAttribute('href', '/pools');

		// After data loads, the breadcrumb should show the pool ID
		await waitFor(() => {
			expect(screen.getAllByText('pool-123').length).toBeGreaterThan(0);
		});
	});

	it('should display pool configuration values', async () => {
		render(PoolDetailPage);

		await waitFor(() => {
			// Configuration section header
			expect(screen.getByText('Configuration')).toBeInTheDocument();

			// Max runners
			expect(screen.getByText('Max Runners')).toBeInTheDocument();
			expect(screen.getAllByText('10').length).toBeGreaterThan(0);

			// Min idle runners
			expect(screen.getByText('Min Idle Runners')).toBeInTheDocument();
			expect(screen.getAllByText('2').length).toBeGreaterThan(0);

			// Bootstrap timeout (displayed as "20 minutes")
			expect(screen.getByText('Bootstrap Timeout')).toBeInTheDocument();
			expect(screen.getAllByText('20 minutes').length).toBeGreaterThan(0);

			// Priority
			expect(screen.getByText('Priority')).toBeInTheDocument();
			expect(screen.getAllByText('100').length).toBeGreaterThan(0);

			// Runner prefix
			expect(screen.getByText('Runner Prefix')).toBeInTheDocument();
			expect(screen.getAllByText('garm').length).toBeGreaterThan(0);

			// OS type / architecture
			expect(screen.getByText('OS Type / Architecture')).toBeInTheDocument();
			expect(screen.getAllByText('linux / amd64').length).toBeGreaterThan(0);
		});
	});

	it('should show Enabled status badge', async () => {
		render(PoolDetailPage);

		await waitFor(() => {
			expect(screen.getAllByText('Status').length).toBeGreaterThan(0);
			expect(screen.getAllByText('Enabled').length).toBeGreaterThan(0);
		});
	});

	it('should show Disabled status badge when pool is disabled', async () => {
		const { garmApi } = await import('$lib/api/client.js');
		const disabledPool = createMockPool({
			...mockPool,
			enabled: false
		});
		(garmApi.getPool as any).mockResolvedValue(disabledPool);

		render(PoolDetailPage);

		await waitFor(() => {
			expect(screen.getAllByText('Disabled').length).toBeGreaterThan(0);
		});
	});

	it('should display tags', async () => {
		render(PoolDetailPage);

		await waitFor(() => {
			expect(screen.getByText('Tags')).toBeInTheDocument();
			expect(screen.getAllByText('ubuntu').length).toBeGreaterThan(0);
			expect(screen.getAllByText('test').length).toBeGreaterThan(0);
		});
	});

	it('should handle error state when API fails', async () => {
		const { garmApi } = await import('$lib/api/client.js');
		(garmApi.getPool as any).mockRejectedValue({
			message: 'Pool not found'
		});

		render(PoolDetailPage);

		await waitFor(() => {
			// The page should not show pool content
			expect(screen.queryByText('Basic Information')).not.toBeInTheDocument();
			expect(screen.queryByText('Configuration')).not.toBeInTheDocument();
		});

		// Breadcrumb navigation should still be present
		expect(screen.getByRole('navigation', { name: 'Breadcrumb' })).toBeInTheDocument();
	});
});
