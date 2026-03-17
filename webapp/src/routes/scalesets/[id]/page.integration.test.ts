import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/svelte';
import { createMockScaleSet, createMockInstance } from '../../../test/factories.js';

// Mock $app/state (this page uses $app/state, not $app/stores)
vi.mock('$app/state', () => ({
	page: {
		params: { id: '123' },
		url: new URL('http://localhost/scalesets/123')
	}
}));

vi.mock('$app/navigation', () => ({
	goto: vi.fn()
}));

vi.mock('$lib/api/client.js', () => ({
	garmApi: {
		getScaleSet: vi.fn(),
		updateScaleSet: vi.fn(),
		deleteScaleSet: vi.fn(),
		deleteInstance: vi.fn()
	}
}));

vi.mock('$lib/stores/websocket.js', () => ({
	websocketStore: {
		subscribeToEntity: vi.fn(() => vi.fn()),
		subscribe: vi.fn(() => () => {})
	}
}));

vi.mock('$lib/stores/eager-cache.js', () => ({
	eagerCache: {
		subscribe: vi.fn((callback: any) => {
			callback({
				pools: [],
				repositories: [],
				organizations: [],
				enterprises: [],
				scalesets: [],
				credentials: [],
				endpoints: [],
				controllerInfo: null,
				loaded: {},
				loading: {},
				errorMessages: {}
			});
			return () => {};
		})
	},
	eagerCacheManager: {
		getScaleSets: vi.fn(),
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

vi.mock('$lib/utils/apiError', () => ({
	extractAPIError: vi.fn((err) => err.message || 'Unknown error')
}));

// Reset any component mocks that might be set by setup.ts
vi.unmock('$lib/components/UpdateScaleSetModal.svelte');
vi.unmock('$lib/components/DeleteModal.svelte');
vi.unmock('$lib/components/DetailHeader.svelte');
vi.unmock('$lib/components/InstancesSection.svelte');
vi.unmock('$lib/components/cells');

import ScaleSetDetailPage from './+page.svelte';

const mockScaleSet = createMockScaleSet({
	id: 123,
	name: 'test-scaleset',
	provider_name: 'test-provider',
	image: 'ubuntu:22.04',
	flavor: 'default',
	enabled: true,
	max_runners: 10,
	min_idle_runners: 1,
	os_type: 'linux',
	os_arch: 'amd64',
	runner_bootstrap_timeout: 20,
	runner_prefix: 'garm',
	repo_id: 'repo-123',
	repo_name: 'test-repo',
	scale_set_id: 8,
	state: 'active',
	desired_runner_count: 5,
	'github-runner-group': 'default',
	instances: [createMockInstance({ name: 'ss-inst-1', status: 'running' })],
	endpoint: {
		name: 'github.com',
		endpoint_type: 'github',
		description: 'GitHub endpoint',
		api_base_url: 'https://api.github.com',
		base_url: 'https://github.com',
		upload_base_url: 'https://uploads.github.com',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z'
	}
});

let garmApi: any;

describe('Scale Set Detail Page Integration Tests', () => {
	beforeEach(async () => {
		vi.clearAllMocks();
		const apiModule = await import('$lib/api/client.js');
		garmApi = apiModule.garmApi;
		(garmApi.getScaleSet as any).mockResolvedValue(mockScaleSet);
	});

	it('should show loading state initially', async () => {
		(garmApi.getScaleSet as any).mockImplementation(
			() => new Promise((resolve) => setTimeout(() => resolve(mockScaleSet), 200))
		);

		render(ScaleSetDetailPage);

		expect(screen.getByText('Loading scale set...')).toBeInTheDocument();
	});

	it('should render scale set details after loading', async () => {
		render(ScaleSetDetailPage);

		await waitFor(() => {
			expect(screen.getByRole('heading', { name: 'test-scaleset' })).toBeInTheDocument();
		});

		await waitFor(() => {
			expect(screen.getAllByText('test-provider')[0]).toBeInTheDocument();
			expect(screen.getAllByText('ubuntu:22.04')[0]).toBeInTheDocument();
			expect(screen.getAllByText('default')[0]).toBeInTheDocument();
		});
	});

	it('should show breadcrumb navigation with Scale Sets link', async () => {
		render(ScaleSetDetailPage);

		const breadcrumb = screen.getByRole('navigation', { name: 'Breadcrumb' });
		expect(breadcrumb).toBeInTheDocument();

		const scalesetsLink = screen.getByRole('link', { name: /Scale Sets/i });
		expect(scalesetsLink).toBeInTheDocument();
		expect(scalesetsLink).toHaveAttribute('href', '/scalesets');

		await waitFor(() => {
			// test-scaleset appears in breadcrumb, heading, and Name field
			expect(screen.getAllByText('test-scaleset').length).toBeGreaterThanOrEqual(2);
		});
	});

	it('should display configuration details', async () => {
		render(ScaleSetDetailPage);

		await waitFor(() => {
			expect(screen.getByText('Configuration')).toBeInTheDocument();
			expect(screen.getByText('Max Runners')).toBeInTheDocument();
			expect(screen.getAllByText('10')[0]).toBeInTheDocument();
			expect(screen.getByText('Min Idle Runners')).toBeInTheDocument();
			expect(screen.getAllByText('1')[0]).toBeInTheDocument();
			expect(screen.getByText('Bootstrap Timeout')).toBeInTheDocument();
			expect(screen.getByText('20 minutes')).toBeInTheDocument();
			expect(screen.getByText('Runner Prefix')).toBeInTheDocument();
			expect(screen.getAllByText('garm')[0]).toBeInTheDocument();
			expect(screen.getByText('OS Type / Architecture')).toBeInTheDocument();
			expect(screen.getByText('linux / amd64')).toBeInTheDocument();
		});
	});

	it('should show Enabled badge when scale set is enabled', async () => {
		render(ScaleSetDetailPage);

		await waitFor(() => {
			expect(screen.getByText('Enabled')).toBeInTheDocument();
		});
	});

	it('should show Disabled badge when scale set is disabled', async () => {
		const disabledScaleSet = createMockScaleSet({
			...mockScaleSet,
			enabled: false
		});
		(garmApi.getScaleSet as any).mockResolvedValue(disabledScaleSet);

		render(ScaleSetDetailPage);

		await waitFor(() => {
			// The Status field shows 'Disabled' as badge text; shell access also shows 'Disabled'
			const disabledBadges = screen.getAllByText('Disabled');
			expect(disabledBadges.length).toBeGreaterThanOrEqual(1);
		});
	});

	it('should show GitHub Runner Group when present', async () => {
		render(ScaleSetDetailPage);

		await waitFor(() => {
			expect(screen.getByText('GitHub Runner Group')).toBeInTheDocument();
			// 'default' appears in multiple places (flavor, runner group), so use getAllByText
			expect(screen.getAllByText('default').length).toBeGreaterThanOrEqual(1);
		});
	});

	it('should handle error state when API fails', async () => {
		(garmApi.getScaleSet as any).mockRejectedValue(new Error('Failed to load scale set'));

		const { container } = render(ScaleSetDetailPage);

		await waitFor(() => {
			expect(screen.getByText('Failed to load scale set')).toBeInTheDocument();
		});

		// Error is displayed in a red container
		const errorElement = container.querySelector('.bg-red-50');
		expect(errorElement).toBeInTheDocument();

		// The detail header should not be rendered when there is an error
		expect(screen.queryByRole('heading', { name: 'test-scaleset' })).not.toBeInTheDocument();
	});
});
