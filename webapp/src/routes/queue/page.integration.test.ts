import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/svelte';

// Unmock real components that setup.ts might mock
vi.unmock('$lib/components/PageHeader.svelte');

vi.mock('$app/navigation', () => ({
	goto: vi.fn()
}));

vi.mock('$app/stores', () => ({}));

vi.mock('$lib/api/client.js', () => ({
	garmApi: {
		listJobs: vi.fn(),
		listInstances: vi.fn()
	}
}));

vi.mock('$lib/stores/eager-cache.js', () => ({
	eagerCache: {
		subscribe: vi.fn()
	},
	eagerCacheManager: {
		getScaleSets: vi.fn(),
		getPools: vi.fn(),
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

vi.mock('$lib/stores/websocket.js', () => ({
	websocketStore: {
		subscribeToEntity: vi.fn(() => vi.fn())
	}
}));

import QueuePage from './+page.svelte';
import { garmApi } from '$lib/api/client.js';

const mockScaleSets = [
	{
		id: 5,
		name: 'cloudstack-ubuntu24-micro',
		org_id: 'org-uuid',
		org_name: 'nexthop',
		max_runners: 100,
		min_idle_runners: 32,
		desired_runner_count: 60,
		statistics: { totalAssignedJobs: 60, totalBusyRunners: 40, totalIdleRunners: 2 },
		enabled: true
	},
	{
		id: 7,
		name: 'cloudstack-ubuntu24-large',
		org_id: 'org-uuid',
		org_name: 'nexthop',
		max_runners: 10,
		enabled: true
	}
];

const mockPools = [
	{
		id: 'aaaabbbb-cccc-dddd-eeee-ffff00001111',
		image: 'ubuntu:22.04',
		flavor: 'default',
		org_id: 'org-uuid',
		org_name: 'nexthop',
		max_runners: 4,
		tags: [{ id: 't1', name: 'self-hosted' }, { id: 't2', name: 'pool-label' }]
	}
];

const mockInstances = [
	{ name: 'r1', scale_set_id: 5, status: 'running', runner_status: 'active' },
	{ name: 'r2', scale_set_id: 5, status: 'running', runner_status: 'idle' },
	{ name: 'r5', scale_set_id: 5, status: 'running', runner_status: 'offline' },
	{ name: 'r3', scale_set_id: 5, status: 'pending_create', runner_status: 'pending' },
	{ name: 'r4', scale_set_id: 5, status: 'creating', runner_status: 'pending' },
	{ name: 'p1', pool_id: 'aaaabbbb-cccc-dddd-eeee-ffff00001111', status: 'running', runner_status: 'active' }
];

const mockJobs = [
	// Two queued scale set jobs (ordering by created_at: job 2 queued first)
	{
		id: 1,
		scaleset_job_id: 'ss-job-1',
		scale_set_id: 5,
		name: 'build',
		status: 'queued',
		repository_owner: 'nexthop',
		repository_name: 'repo-a',
		labels: ['cloudstack-ubuntu24-micro'],
		created_at: '2026-07-29T00:10:00Z'
	},
	{
		id: 2,
		scaleset_job_id: 'ss-job-2',
		scale_set_id: 5,
		name: 'test',
		status: 'queued',
		repository_owner: 'nexthop',
		repository_name: 'repo-b',
		labels: ['cloudstack-ubuntu24-micro'],
		created_at: '2026-07-29T00:05:00Z'
	},
	// A running scale set job
	{
		id: 3,
		scaleset_job_id: 'ss-job-3',
		scale_set_id: 5,
		name: 'lint',
		status: 'in_progress',
		repository_owner: 'nexthop',
		repository_name: 'repo-a',
		labels: ['cloudstack-ubuntu24-micro'],
		created_at: '2026-07-29T00:01:00Z'
	},
	// A queued webhook job that label-matches the pool
	{
		id: 4,
		workflow_job_id: 1234,
		name: 'pool-job',
		status: 'queued',
		repository_owner: 'nexthop',
		repository_name: 'repo-c',
		labels: ['self-hosted', 'pool-label'],
		created_at: '2026-07-29T00:02:00Z'
	},
	// A completed job that must not show up
	{
		id: 5,
		scaleset_job_id: 'ss-job-5',
		scale_set_id: 7,
		name: 'done-job',
		status: 'completed',
		repository_owner: 'nexthop',
		repository_name: 'repo-a',
		labels: ['cloudstack-ubuntu24-large'],
		created_at: '2026-07-29T00:00:00Z'
	},
	// A queued job matching nothing
	{
		id: 6,
		workflow_job_id: 5678,
		name: 'orphan-job',
		status: 'queued',
		repository_owner: 'nexthop',
		repository_name: 'repo-d',
		labels: ['no-such-label'],
		created_at: '2026-07-29T00:03:00Z'
	}
];

describe('Queue Page - Integration Tests', () => {
	beforeEach(async () => {
		vi.clearAllMocks();

		const cacheModule = await import('$lib/stores/eager-cache.js');
		vi.mocked(cacheModule.eagerCacheManager.getScaleSets).mockResolvedValue(mockScaleSets);
		vi.mocked(cacheModule.eagerCacheManager.getPools).mockResolvedValue(mockPools);
		vi.mocked(garmApi.listJobs).mockResolvedValue(mockJobs as any);
		vi.mocked(garmApi.listInstances).mockResolvedValue(mockInstances as any);
	});

	it('renders page title', async () => {
		render(QueuePage);
		await waitFor(() => {
			expect(screen.getByRole('heading', { name: 'Job Queue' })).toBeInTheDocument();
		});
	});

	it('groups queued jobs per scale set with queue ordering and counts', async () => {
		render(QueuePage);

		await waitFor(() => {
			expect(screen.getByRole('heading', { name: 'cloudstack-ubuntu24-micro' })).toBeInTheDocument();
		});

		// Scale set group shows queued and running counts (incl. max runners)
		expect(screen.getByText('2 queued')).toBeInTheDocument();
		// Runner counts come from instances (2 running: 1 busy / 1 idle; 2 provisioning)
		expect(
			screen.getByText(/3 runners \/ 100 max \(1 busy, 1 idle, 1 offline, 2 provisioning\)/)
		).toBeInTheDocument();

		// GitHub's view from the scale set statistics
		expect(screen.getByText(/GitHub: 60 assigned \(40 busy, 2 idle\)/)).toBeInTheDocument();
		expect(
			screen.getByRole('heading', { name: 'cloudstack-ubuntu24-micro' }).querySelector('a')
		).toHaveAttribute('href', '/scalesets/5');

		// Queue order: 'test' (older) before 'build'
		const rows = screen.getAllByRole('row');
		const rowText = rows.map((r) => r.textContent || '');
		const testIdx = rowText.findIndex((t) => t.includes('test'));
		const buildIdx = rowText.findIndex((t) => t.includes('build'));
		expect(testIdx).toBeGreaterThan(0);
		expect(buildIdx).toBeGreaterThan(testIdx);

		// Completed jobs are not shown
		expect(screen.queryByText('done-job')).not.toBeInTheDocument();
	});

	it('matches webhook jobs to pools by labels', async () => {
		render(QueuePage);

		await waitFor(() => {
			expect(screen.getByText('pool-job')).toBeInTheDocument();
		});
		expect(screen.getByText(/Pool aaaabbbb/)).toBeInTheDocument();
	});

	it('shows unattributed jobs in their own group', async () => {
		render(QueuePage);

		await waitFor(() => {
			expect(screen.getByText('Unattributed jobs')).toBeInTheDocument();
		});
		expect(screen.getByText('orphan-job')).toBeInTheDocument();
	});

	it('shows empty state when there are no active jobs', async () => {
		vi.mocked(garmApi.listJobs).mockResolvedValue([]);
		vi.mocked(garmApi.listInstances).mockResolvedValue([]);
		render(QueuePage);

		await waitFor(() => {
			expect(screen.getByText('No queued or running jobs.')).toBeInTheDocument();
		});
	});
});

describe('Queue Page - job links', () => {
	beforeEach(async () => {
		vi.clearAllMocks();
		const cacheModule = await import('$lib/stores/eager-cache.js');
		vi.mocked(cacheModule.eagerCacheManager.getScaleSets).mockResolvedValue(mockScaleSets);
		vi.mocked(cacheModule.eagerCacheManager.getPools).mockResolvedValue(mockPools);
		vi.mocked(garmApi.listInstances).mockResolvedValue([]);
	});

	it('deep-links webhook jobs to the GitHub job page and scale set jobs to the run page', async () => {
		vi.mocked(garmApi.listJobs).mockResolvedValue([
			{
				id: 10,
				workflow_job_id: 90673415740,
				run_id: 30480594764,
				name: 'pool-job',
				status: 'queued',
				repository_owner: 'nexthop-ai',
				repository_name: 'nhsystem-ops',
				labels: ['self-hosted', 'pool-label'],
				created_at: '2026-07-29T00:02:00Z'
			},
			{
				id: 11,
				scaleset_job_id: 'guid-1',
				scale_set_id: 5,
				run_id: 1234,
				workflow_run_url: 'https://github.com/nexthop-ai/repo-a/actions/runs/1234',
				name: 'ss-job',
				status: 'queued',
				repository_owner: 'nexthop-ai',
				repository_name: 'repo-a',
				labels: [],
				created_at: '2026-07-29T00:02:00Z'
			}
		] as any);

		render(QueuePage);

		await waitFor(() => {
			expect(screen.getByText('pool-job')).toBeInTheDocument();
		});
		expect(screen.getByText('pool-job').closest('a')).toHaveAttribute(
			'href',
			'https://github.com/nexthop-ai/nhsystem-ops/actions/runs/30480594764/job/90673415740'
		);
		expect(screen.getByText('ss-job').closest('a')).toHaveAttribute(
			'href',
			'https://github.com/nexthop-ai/repo-a/actions/runs/1234'
		);
	});
});
