import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/svelte';
import {
	createMockRepository,
	createMockOrganization,
	createMockPool,
	createMockInstance
} from '../test/factories.js';
import type { ControllerInfo } from '$lib/api/generated/api.js';

// Create test data
const mockRepositories = [
	createMockRepository({ id: 'repo-1', name: 'repo-one', owner: 'owner-1' }),
	createMockRepository({ id: 'repo-2', name: 'repo-two', owner: 'owner-2' })
];

const mockOrganizations = [
	createMockOrganization({ id: 'org-1', name: 'org-one' }),
	createMockOrganization({ id: 'org-2', name: 'org-two' }),
	createMockOrganization({ id: 'org-3', name: 'org-three' })
];

const mockPools = [
	createMockPool({ id: 'pool-1' })
];

const mockInstances = [
	createMockInstance({ id: 'inst-1', name: 'instance-1', status: 'running' }),
	createMockInstance({ id: 'inst-2', name: 'instance-2', status: 'stopped' }),
	createMockInstance({ id: 'inst-3', name: 'instance-3', status: 'running' }),
	createMockInstance({ id: 'inst-4', name: 'instance-4', status: 'pending' })
];

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

// Mock ONLY the API
vi.mock('$lib/api/client.js', () => ({
	garmApi: {
		listInstances: vi.fn(),
		updateController: vi.fn()
	}
}));

// Mock eager cache store and manager
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
	},
	eagerCacheManager: {
		getRepositories: vi.fn(),
		getOrganizations: vi.fn(),
		getPools: vi.fn(),
		getControllerInfo: vi.fn()
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

vi.mock('$lib/utils/apiError', () => ({
	extractAPIError: vi.fn((err: any) => err.message || 'Unknown error')
}));

import DashboardPage from './+page.svelte';

describe('Dashboard Page Integration Tests', () => {
	let garmApi: any;
	let eagerCacheManager: any;
	let websocketStore: any;

	beforeEach(async () => {
		vi.clearAllMocks();

		// Reset mock store data
		mockStoreData = {
			controllerInfo: mockControllerInfo,
			loaded: { controllerInfo: true }
		};

		const apiModule = await import('$lib/api/client.js');
		garmApi = apiModule.garmApi;

		const cacheModule = await import('$lib/stores/eager-cache.js');
		eagerCacheManager = cacheModule.eagerCacheManager;

		const wsModule = await import('$lib/stores/websocket.js');
		websocketStore = wsModule.websocketStore;

		// Set up default successful responses
		(eagerCacheManager.getRepositories as any).mockResolvedValue(mockRepositories);
		(eagerCacheManager.getOrganizations as any).mockResolvedValue(mockOrganizations);
		(eagerCacheManager.getPools as any).mockResolvedValue(mockPools);
		(garmApi.listInstances as any).mockResolvedValue(mockInstances);
		(eagerCacheManager.getControllerInfo as any).mockResolvedValue(mockControllerInfo);
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
	});

	describe('Stat Cards', () => {
		it('should render stat cards with correct values after data loads', async () => {
			render(DashboardPage);

			await waitFor(() => {
				// Verify all four stat card titles are present
				expect(screen.getByText('Repositories')).toBeInTheDocument();
				expect(screen.getByText('Organizations')).toBeInTheDocument();
				expect(screen.getByText('Pools')).toBeInTheDocument();
				expect(screen.getByText('Instances')).toBeInTheDocument();
			});

			// Wait for the data to load and stats to update
			await waitFor(() => {
				const repoStat = document.querySelector('[data-stat="repositories"]');
				const orgStat = document.querySelector('[data-stat="organizations"]');
				const poolStat = document.querySelector('[data-stat="pools"]');
				const instanceStat = document.querySelector('[data-stat="instances"]');

				expect(repoStat?.textContent).toBe('2');
				expect(orgStat?.textContent).toBe('3');
				expect(poolStat?.textContent).toBe('1');
				expect(instanceStat?.textContent).toBe('4');
			});
		});

		it('should render stat cards as links to their respective pages', async () => {
			const { container } = render(DashboardPage);

			await waitFor(() => {
				const repoLink = container.querySelector('a[href="/repositories"]');
				const orgLink = container.querySelector('a[href="/organizations"]');
				const poolLink = container.querySelector('a[href="/pools"]');
				const instanceLink = container.querySelector('a[href="/instances"]');

				expect(repoLink).toBeInTheDocument();
				expect(orgLink).toBeInTheDocument();
				expect(poolLink).toBeInTheDocument();
				expect(instanceLink).toBeInTheDocument();
			});
		});
	});

	describe('Quick Actions', () => {
		it('should render quick action links', async () => {
			render(DashboardPage);

			await waitFor(() => {
				expect(screen.getByText('Quick Actions')).toBeInTheDocument();
				expect(screen.getByText('Add Repository')).toBeInTheDocument();
				expect(screen.getByText('Create Pool')).toBeInTheDocument();
				expect(screen.getByText('View Instances')).toBeInTheDocument();
			});
		});

		it('should link quick actions to the correct pages', async () => {
			const { container } = render(DashboardPage);

			await waitFor(() => {
				// Add Repository links to /repositories
				const addRepoLink = Array.from(container.querySelectorAll('a')).find(
					(a) => a.textContent?.includes('Add Repository')
				);
				expect(addRepoLink?.getAttribute('href')).toBe('/repositories');

				// Create Pool links to /pools
				const createPoolLink = Array.from(container.querySelectorAll('a')).find(
					(a) => a.textContent?.includes('Create Pool')
				);
				expect(createPoolLink?.getAttribute('href')).toBe('/pools');

				// View Instances links to /instances
				const viewInstancesLink = Array.from(container.querySelectorAll('a')).find(
					(a) => a.textContent?.includes('View Instances')
				);
				expect(viewInstancesLink?.getAttribute('href')).toBe('/instances');
			});
		});
	});

	describe('Error Handling', () => {
		it('should display error message when API calls fail', async () => {
			const errorMessage = 'Failed to connect to GARM server';
			(eagerCacheManager.getRepositories as any).mockRejectedValue(
				new Error(errorMessage)
			);

			render(DashboardPage);

			await waitFor(() => {
				expect(screen.getByText('Error loading dashboard')).toBeInTheDocument();
				expect(screen.getByText(errorMessage)).toBeInTheDocument();
			});
		});

		it('should still render page structure when API errors occur', async () => {
			(eagerCacheManager.getRepositories as any).mockRejectedValue(
				new Error('Network error')
			);

			render(DashboardPage);

			await waitFor(() => {
				// Title and welcome message should always be present
				expect(screen.getByText('Dashboard')).toBeInTheDocument();
				expect(
					screen.getByText('Welcome to GARM - GitHub Actions Runner Manager')
				).toBeInTheDocument();
				// Stat cards should still render (with zero values)
				expect(screen.getByText('Repositories')).toBeInTheDocument();
				expect(screen.getByText('Quick Actions')).toBeInTheDocument();
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

			await waitFor(() => {
				expect(websocketStore.subscribeToEntity).toHaveBeenCalledWith(
					'repository',
					['create', 'delete'],
					expect.any(Function)
				);
				expect(websocketStore.subscribeToEntity).toHaveBeenCalledWith(
					'organization',
					['create', 'delete'],
					expect.any(Function)
				);
				expect(websocketStore.subscribeToEntity).toHaveBeenCalledWith(
					'pool',
					['create', 'delete'],
					expect.any(Function)
				);
				expect(websocketStore.subscribeToEntity).toHaveBeenCalledWith(
					'instance',
					['create', 'delete'],
					expect.any(Function)
				);
			});
		});

		it('should clean up websocket subscriptions on unmount', async () => {
			const mockUnsubscribes = [vi.fn(), vi.fn(), vi.fn(), vi.fn()];
			let callIndex = 0;
			(websocketStore.subscribeToEntity as any).mockImplementation(() => {
				return mockUnsubscribes[callIndex++] || vi.fn();
			});

			const { unmount } = render(DashboardPage);

			await waitFor(() => {
				expect(websocketStore.subscribeToEntity).toHaveBeenCalledTimes(4);
			});

			unmount();

			mockUnsubscribes.forEach((unsub) => {
				expect(unsub).toHaveBeenCalled();
			});
		});
	});

	describe('Data Loading', () => {
		it('should call all data loading functions on mount', async () => {
			render(DashboardPage);

			await waitFor(() => {
				expect(eagerCacheManager.getRepositories).toHaveBeenCalled();
				expect(eagerCacheManager.getOrganizations).toHaveBeenCalled();
				expect(eagerCacheManager.getPools).toHaveBeenCalled();
				expect(garmApi.listInstances).toHaveBeenCalled();
				expect(eagerCacheManager.getControllerInfo).toHaveBeenCalled();
			});
		});
	});
});
