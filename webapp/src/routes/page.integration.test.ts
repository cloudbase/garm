import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/svelte';
import type { ControllerInfo } from '$lib/api/generated/api.js';
import type { MetricsSnapshot } from '$lib/stores/metrics-ws.js';
import { writable } from 'svelte/store';

// Build a metrics snapshot that the dashboard consumes
const mockSnapshot: MetricsSnapshot = {
	entities: [
		{ id: 'repo-1', name: 'owner-1/repo-one', type: 'repository', endpoint: 'github.com', pool_count: 2, scale_set_count: 1, healthy: true },
		{ id: 'repo-2', name: 'owner-2/repo-two', type: 'repository', endpoint: 'github.com', pool_count: 1, scale_set_count: 0, healthy: true },
		{ id: 'org-1', name: 'org-one', type: 'organization', endpoint: 'github.com', pool_count: 1, scale_set_count: 0, healthy: true },
		{ id: 'org-2', name: 'org-two', type: 'organization', endpoint: 'github.com', pool_count: 0, scale_set_count: 0, healthy: true },
		{ id: 'org-3', name: 'org-three', type: 'organization', endpoint: 'github.com', pool_count: 0, scale_set_count: 0, healthy: true },
		{ id: 'ent-1', name: 'enterprise-one', type: 'enterprise', endpoint: 'github.com', pool_count: 0, scale_set_count: 0, healthy: true }
	],
	pools: [
		{ id: 'pool-1-abc', provider_name: 'openstack', os_type: 'linux', max_runners: 10, enabled: true, repo_name: 'my-repo', runner_counts: { running: 1, stopped: 1 }, runner_status_counts: { active: 1, idle: 1 } },
		{ id: 'pool-2-xyz', provider_name: 'azure', os_type: 'windows', max_runners: 5, enabled: true, org_name: 'my-org', runner_counts: { running: 1, error: 1 }, runner_status_counts: { active: 1, offline: 1 } }
	],
	scale_sets: [
		{ id: 1, name: 'scale-1', provider_name: 'test-provider', os_type: 'linux', max_runners: 10, enabled: true, runner_counts: {}, runner_status_counts: {} }
	]
};

const mockControllerInfo: ControllerInfo = {
	controller_id: 'ctrl-abc-123',
	hostname: 'garm-host-01',
	metadata_url: 'https://garm.example.com/api/v1/metadata',
	callback_url: 'https://garm.example.com/api/v1/callbacks',
	webhook_url: 'https://garm.example.com/webhooks',
	controller_webhook_url: 'https://garm.example.com/webhooks/ctrl-abc-123',
	version: 'v0.1.5',
	minimum_job_age_backoff: 30
};

// Mutable store backing for metrics — tests can change this before rendering
let metricsStoreValue = writable<MetricsSnapshot | null>(mockSnapshot);
let connectionStateValue = writable<string>('connected');

vi.mock('$lib/stores/metrics-ws.js', () => ({
	metricsStore: {
		subscribe: (fn: any) => metricsStoreValue.subscribe(fn),
		connect: vi.fn(),
		disconnect: vi.fn(),
		connectionState: {
			subscribe: (fn: any) => connectionStateValue.subscribe(fn)
		}
	}
}));

// Mock ONLY the API (still needed for ControllerInfoCard updateController)
vi.mock('$lib/api/client.js', () => ({
	garmApi: {
		updateController: vi.fn()
	}
}));

// Mock eager cache store (only used for controllerInfo now)
let mockStoreData: {
	controllerInfo: ControllerInfo | null;
	loaded: { controllerInfo: boolean };
} = {
	controllerInfo: mockControllerInfo,
	loaded: { controllerInfo: true }
};

vi.mock('$lib/stores/eager-cache.js', () => ({
	eagerCache: {
		subscribe: vi.fn((callback: (value: typeof mockStoreData) => void) => {
			callback(mockStoreData);
			return () => {};
		})
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

vi.mock('$lib/stores/websocket.js', () => ({
	websocketStore: {
		subscribeToEntity: vi.fn(() => vi.fn())
	}
}));

import DashboardPage from './+page.svelte';

describe('Dashboard Page Integration Tests', () => {
	let websocketStore: any;

	beforeEach(async () => {
		vi.clearAllMocks();

		// Reset metrics store to default snapshot
		metricsStoreValue = writable<MetricsSnapshot | null>(mockSnapshot);
		connectionStateValue = writable<string>('connected');

		mockStoreData = {
			controllerInfo: mockControllerInfo,
			loaded: { controllerInfo: true }
		};

		const wsModule = await import('$lib/stores/websocket.js');
		websocketStore = wsModule.websocketStore;
		(websocketStore.subscribeToEntity as any).mockReturnValue(vi.fn());
	});

	describe('Page Structure and Header', () => {
		it('should render dashboard title and welcome message', async () => {
			render(DashboardPage);

			expect(screen.getByText('Dashboard')).toBeInTheDocument();
			expect(
				screen.getByText('Welcome to GARM - GitHub Actions Runner Manager')
			).toBeInTheDocument();
		});

		it('should set the document title', async () => {
			render(DashboardPage);

			await waitFor(() => {
				expect(document.title).toContain('Dashboard');
			});
		});

		it('should render setup wizard button in header', async () => {
			const { container } = render(DashboardPage);

			await waitFor(() => {
				const setupLink = Array.from(container.querySelectorAll('a')).find(
					(a) => a.textContent?.includes('Setup Wizard')
				);
				expect(setupLink).toBeInTheDocument();
				expect(setupLink?.getAttribute('href')).toBe('/setup');
			});
		});
	});

	describe('Stat Cards', () => {
		it('should render all 6 stat cards with correct values from metrics snapshot', async () => {
			render(DashboardPage);

			await waitFor(() => {
				expect(screen.getAllByText('Repositories').length).toBeGreaterThanOrEqual(1);
				expect(screen.getAllByText('Organizations').length).toBeGreaterThanOrEqual(1);
				expect(screen.getAllByText('Enterprises').length).toBeGreaterThanOrEqual(1);
				expect(screen.getByText('Pools')).toBeInTheDocument();
				expect(screen.getByText('Scale Sets')).toBeInTheDocument();
				expect(screen.getByText('Instances')).toBeInTheDocument();
			});

			await waitFor(() => {
				const repoStat = document.querySelector('[data-stat="repositories"]');
				const orgStat = document.querySelector('[data-stat="organizations"]');
				const entStat = document.querySelector('[data-stat="enterprises"]');
				const poolStat = document.querySelector('[data-stat="pools"]');
				const scaleStat = document.querySelector('[data-stat="scalesets"]');
				const instanceStat = document.querySelector('[data-stat="instances"]');

				expect(repoStat?.textContent).toBe('2');
				expect(orgStat?.textContent).toBe('3');
				expect(entStat?.textContent).toBe('1');
				expect(poolStat?.textContent).toBe('2');
				expect(scaleStat?.textContent).toBe('1');
				// 4 instances total: pool-1 has running(1)+stopped(1), pool-2 has running(1)+error(1)
				expect(instanceStat?.textContent).toBe('4');
			});
		});

		it('should render stat cards as links to their respective pages', async () => {
			const { container } = render(DashboardPage);

			await waitFor(() => {
				const repoLink = container.querySelector('a[href="/repositories"]');
				const orgLink = container.querySelector('a[href="/organizations"]');
				const entLink = container.querySelector('a[href="/enterprises"]');
				const poolLink = container.querySelector('a[href="/pools"]');
				const scaleLink = container.querySelector('a[href="/scalesets"]');
				const instanceLink = container.querySelector('a[href="/instances"]');

				expect(repoLink).toBeInTheDocument();
				expect(orgLink).toBeInTheDocument();
				expect(entLink).toBeInTheDocument();
				expect(poolLink).toBeInTheDocument();
				expect(scaleLink).toBeInTheDocument();
				expect(instanceLink).toBeInTheDocument();
			});
		});
	});

	describe('Runner Status Section', () => {
		it('should render runner status section with breakdown', async () => {
			render(DashboardPage);

			await waitFor(() => {
				expect(screen.getByText('Runner Status')).toBeInTheDocument();
				// 2 active (1 from each pool), 1 idle, 1 offline
				expect(screen.getByText('2 Active')).toBeInTheDocument();
				expect(screen.getByText('1 Idle')).toBeInTheDocument();
				expect(screen.getByText('1 Offline')).toBeInTheDocument();
			});
		});

		it('should show empty state when no instances exist', async () => {
			metricsStoreValue = writable<MetricsSnapshot | null>({
				entities: [],
				pools: [{ id: 'p1', provider_name: 'test', os_type: 'linux', max_runners: 5, enabled: true, runner_counts: {}, runner_status_counts: {} }],
				scale_sets: []
			});

			render(DashboardPage);

			await waitFor(() => {
				expect(screen.getByText(/No instances running/)).toBeInTheDocument();
			});
		});
	});

	describe('Entities Section', () => {
		it('should render entities section with entity names', async () => {
			render(DashboardPage);

			await waitFor(() => {
				expect(screen.getByText('Entities')).toBeInTheDocument();
				expect(screen.getByText('owner-1/repo-one')).toBeInTheDocument();
				expect(screen.getByText('owner-2/repo-two')).toBeInTheDocument();
				expect(screen.getByText('org-one')).toBeInTheDocument();
			});
		});

		it('should show pool and scale set counts for entities', async () => {
			render(DashboardPage);

			await waitFor(() => {
				// repo-1 has 2 pools + 1 scale set
				expect(screen.getByText('2 pools, 1 scale set')).toBeInTheDocument();
				// repo-2 and org-1 each have 1 pool
				expect(screen.getAllByText('1 pool').length).toBeGreaterThanOrEqual(1);
			});
		});

		it('should show empty state when no entities exist', async () => {
			metricsStoreValue = writable<MetricsSnapshot | null>({
				entities: [],
				pools: [],
				scale_sets: []
			});

			render(DashboardPage);

			await waitFor(() => {
				expect(screen.getByText('No entities configured yet.')).toBeInTheDocument();
			});
		});
	});

	describe('Pool Capacity Section', () => {
		it('should render pool capacity section with pool data', async () => {
			render(DashboardPage);

			await waitFor(() => {
				expect(screen.getByText('Pool Capacity')).toBeInTheDocument();
				// Pools show entity name / short ID
				expect(screen.getByText('my-repo / pool-1-a')).toBeInTheDocument();
				expect(screen.getByText('my-org / pool-2-x')).toBeInTheDocument();
				expect(screen.getByText('2/10')).toBeInTheDocument();
				expect(screen.getByText('2/5')).toBeInTheDocument();
			});
		});

		it('should show empty state when no active pools exist', async () => {
			metricsStoreValue = writable<MetricsSnapshot | null>({
				entities: [],
				pools: [{ id: 'p1', provider_name: 'test', os_type: 'linux', max_runners: 5, enabled: false, runner_counts: {}, runner_status_counts: {} }],
				scale_sets: []
			});

			render(DashboardPage);

			await waitFor(() => {
				expect(screen.getByText('No active pools.')).toBeInTheDocument();
			});
		});
	});

	describe('Connection Status', () => {
		it('should show connecting banner when metrics WebSocket is connecting', async () => {
			metricsStoreValue = writable<MetricsSnapshot | null>(null);
			connectionStateValue = writable<string>('connecting');

			render(DashboardPage);

			await waitFor(() => {
				expect(screen.getByText('Connecting to live metrics...')).toBeInTheDocument();
			});
		});

		it('should show disconnected banner when metrics WebSocket is lost', async () => {
			metricsStoreValue = writable<MetricsSnapshot | null>(null);
			connectionStateValue = writable<string>('disconnected');

			render(DashboardPage);

			await waitFor(() => {
				expect(screen.getByText('Live metrics unavailable')).toBeInTheDocument();
			});
		});

		it('should gray out live data sections when not connected', async () => {
			metricsStoreValue = writable<MetricsSnapshot | null>(null);
			connectionStateValue = writable<string>('disconnected');

			const { container } = render(DashboardPage);

			await waitFor(() => {
				const statGrid = container.querySelector('.grid.grid-cols-2');
				expect(statGrid?.classList.contains('opacity-50')).toBe(true);
				expect(statGrid?.classList.contains('pointer-events-none')).toBe(true);
			});
		});

		it('should not show connection banner when connected with data', async () => {
			render(DashboardPage);

			await waitFor(() => {
				expect(screen.queryByText('Connecting to live metrics...')).not.toBeInTheDocument();
				expect(screen.queryByText('Live metrics unavailable')).not.toBeInTheDocument();
			});
		});
	});

	describe('Controller Info Card', () => {
		it('should render ControllerInfoCard when controller info is available', async () => {
			render(DashboardPage);

			await waitFor(() => {
				expect(screen.getByText('Controller Information')).toBeInTheDocument();
			});
		});
	});

	describe('WebSocket Subscriptions', () => {
		it('should subscribe to websocket events for all entity types on mount', async () => {
			render(DashboardPage);

			const ops = ['create', 'update', 'delete'];
			await waitFor(() => {
				expect(websocketStore.subscribeToEntity).toHaveBeenCalledWith(
					'repository', ops, expect.any(Function)
				);
				expect(websocketStore.subscribeToEntity).toHaveBeenCalledWith(
					'organization', ops, expect.any(Function)
				);
				expect(websocketStore.subscribeToEntity).toHaveBeenCalledWith(
					'enterprise', ops, expect.any(Function)
				);
				expect(websocketStore.subscribeToEntity).toHaveBeenCalledWith(
					'pool', ops, expect.any(Function)
				);
				expect(websocketStore.subscribeToEntity).toHaveBeenCalledWith(
					'scaleset', ops, expect.any(Function)
				);
				expect(websocketStore.subscribeToEntity).toHaveBeenCalledWith(
					'instance', ops, expect.any(Function)
				);
				expect(websocketStore.subscribeToEntity).toHaveBeenCalledWith(
					'github_credentials', ops, expect.any(Function)
				);
				expect(websocketStore.subscribeToEntity).toHaveBeenCalledWith(
					'gitea_credentials', ops, expect.any(Function)
				);
				expect(websocketStore.subscribeToEntity).toHaveBeenCalledWith(
					'github_endpoint', ops, expect.any(Function)
				);
			});
		});

		it('should clean up websocket subscriptions on unmount', async () => {
			const mockUnsubscribes = Array.from({ length: 9 }, () => vi.fn());
			let callIndex = 0;
			(websocketStore.subscribeToEntity as any).mockImplementation(() => {
				return mockUnsubscribes[callIndex++] || vi.fn();
			});

			const { unmount } = render(DashboardPage);

			await waitFor(() => {
				expect(websocketStore.subscribeToEntity).toHaveBeenCalledTimes(9);
			});

			unmount();

			mockUnsubscribes.forEach((unsub) => {
				expect(unsub).toHaveBeenCalled();
			});
		});
	});
});
