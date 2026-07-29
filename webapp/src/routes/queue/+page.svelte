<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { garmApi } from '$lib/api/client.js';
	import type { Job, Pool, ScaleSet } from '$lib/api/generated/api.js';
	import PageHeader from '$lib/components/PageHeader.svelte';
	import { websocketStore, type WebSocketEvent } from '$lib/stores/websocket.js';
	import { eagerCacheManager } from '$lib/stores/eager-cache.js';
	import { getEntityName } from '$lib/utils/common.js';

	let jobs: Job[] = [];
	let scaleSets: ScaleSet[] = [];
	let pools: Pool[] = [];
	let loading = true;
	let error = '';
	let searchTerm = '';
	let unsubscribeWebsocket: (() => void) | null = null;

	// Current time, refreshed periodically so "waiting for" durations stay fresh.
	let currentTime = Date.now();
	let clockInterval: ReturnType<typeof setInterval> | null = null;

	interface QueueGroup {
		key: string;
		kind: 'scaleset' | 'pool' | 'unmatched';
		title: string;
		subtitle: string;
		maxRunners?: number;
		queued: Job[];
		inProgress: Job[];
	}

	function isScaleSetJob(job: Job): boolean {
		return !!job.scaleset_job_id;
	}

	function jobMatchesPool(job: Job, pool: Pool): boolean {
		const jobLabels = (job.labels || []).map((l) => l.toLowerCase());
		if (jobLabels.length === 0) return false;
		const poolTags = new Set((pool.tags || []).map((t) => (t.name || '').toLowerCase()));
		return jobLabels.every((l) => poolTags.has(l));
	}

	function matchesSearch(job: Job, term: string): boolean {
		if (!term) return true;
		const t = term.toLowerCase();
		return (
			(job.name || '').toLowerCase().includes(t) ||
			`${job.repository_owner}/${job.repository_name}`.toLowerCase().includes(t) ||
			(job.labels || []).some((l) => l.toLowerCase().includes(t))
		);
	}

	$: activeJobs = jobs.filter(
		(j) => (j.status === 'queued' || j.status === 'in_progress') && matchesSearch(j, searchTerm)
	);

	$: queueGroups = buildGroups(activeJobs, scaleSets, pools);

	function buildGroups(active: Job[], sets: ScaleSet[], poolList: Pool[]): QueueGroup[] {
		const byTime = (a: Job, b: Job) =>
			new Date(a.created_at || 0).getTime() - new Date(b.created_at || 0).getTime();
		const queued = active.filter((j) => j.status === 'queued').sort(byTime);
		const running = active.filter((j) => j.status === 'in_progress');

		const groups: QueueGroup[] = [];
		const attributed = new Set<number>();

		// Scale sets: jobs carry the garm scale set ID directly.
		for (const set of sets) {
			const q = queued.filter((j) => j.scale_set_id === set.id);
			const r = running.filter((j) => j.scale_set_id === set.id);
			q.forEach((j) => attributed.add(j.id!));
			r.forEach((j) => attributed.add(j.id!));
			if (q.length === 0 && r.length === 0) continue;
			groups.push({
				key: `scaleset-${set.id}`,
				kind: 'scaleset',
				title: set.name || `Scale set ${set.id}`,
				subtitle: getEntityName(set),
				maxRunners: set.max_runners,
				queued: q,
				inProgress: r
			});
		}

		// Pools: webhook jobs are matched to pools by labels at runtime, so we
		// mirror that matching here. A job may match more than one pool.
		for (const pool of poolList) {
			const q = queued.filter((j) => !isScaleSetJob(j) && jobMatchesPool(j, pool));
			const r = running.filter((j) => !isScaleSetJob(j) && jobMatchesPool(j, pool));
			q.forEach((j) => attributed.add(j.id!));
			r.forEach((j) => attributed.add(j.id!));
			if (q.length === 0 && r.length === 0) continue;
			groups.push({
				key: `pool-${pool.id}`,
				kind: 'pool',
				title: `Pool ${pool.id?.slice(0, 8)} (${pool.image || pool.flavor || ''})`,
				subtitle: getEntityName(pool),
				maxRunners: (pool as any).max_runners,
				queued: q,
				inProgress: r
			});
		}

		// Anything left over: scale set jobs recorded before garm started
		// tracking scale_set_id, or jobs whose labels match no pool.
		const leftoverQueued = queued.filter((j) => !attributed.has(j.id!));
		const leftoverRunning = running.filter((j) => !attributed.has(j.id!));
		if (leftoverQueued.length > 0 || leftoverRunning.length > 0) {
			groups.push({
				key: 'unmatched',
				kind: 'unmatched',
				title: 'Unattributed jobs',
				subtitle: 'No matching scale set or pool',
				queued: leftoverQueued,
				inProgress: leftoverRunning
			});
		}

		// Busiest queues first.
		groups.sort((a, b) => b.queued.length - a.queued.length);
		return groups;
	}

	function waitingFor(job: Job): string {
		if (!job.created_at) return '-';
		const seconds = Math.max(0, Math.floor((currentTime - new Date(job.created_at).getTime()) / 1000));
		const h = Math.floor(seconds / 3600);
		const m = Math.floor((seconds % 3600) / 60);
		const s = seconds % 60;
		if (h > 0) return `${h}h ${m}m`;
		if (m > 0) return `${m}m ${s}s`;
		return `${s}s`;
	}

	async function loadData() {
		try {
			loading = true;
			error = '';
			const [jobData, scaleSetData, poolData] = await Promise.all([
				garmApi.listJobs(),
				eagerCacheManager.getScaleSets(),
				eagerCacheManager.getPools()
			]);
			jobs = jobData;
			scaleSets = scaleSetData || [];
			pools = poolData || [];
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load job queue';
		} finally {
			loading = false;
		}
	}

	function handleJobEvent(event: WebSocketEvent) {
		if (event.operation === 'create') {
			const newJob = event.payload as Job;
			jobs = [...jobs.filter((j) => j.id !== newJob.id), newJob];
		} else if (event.operation === 'update') {
			const updatedJob = event.payload as Job;
			jobs = jobs.map((j) => (j.id === updatedJob.id ? updatedJob : j));
		} else if (event.operation === 'delete') {
			const jobID = event.payload.id ?? event.payload;
			jobs = jobs.filter((j) => j.id !== jobID);
		}
	}

	onMount(() => {
		loadData();

		unsubscribeWebsocket = websocketStore.subscribeToEntity(
			'job',
			['create', 'update', 'delete'],
			handleJobEvent
		);

		clockInterval = setInterval(() => {
			currentTime = Date.now();
		}, 5000);
	});

	onDestroy(() => {
		if (unsubscribeWebsocket) {
			unsubscribeWebsocket();
			unsubscribeWebsocket = null;
		}
		if (clockInterval) {
			clearInterval(clockInterval);
			clockInterval = null;
		}
	});
</script>

<svelte:head>
	<title>Job Queue - GARM</title>
</svelte:head>

<div class="space-y-6">
	<PageHeader
		title="Job Queue"
		description="Queued and running GitHub Actions jobs, grouped per scale set / pool"
		showAction={false}
	/>

	{#if error}
		<div class="bg-red-50 dark:bg-red-900/50 border border-red-200 dark:border-red-800 rounded-md p-4">
			<div class="flex">
				<div class="ml-3">
					<h3 class="text-sm font-medium text-red-800 dark:text-red-200">Error</h3>
					<div class="mt-2 text-sm text-red-700 dark:text-red-300">{error}</div>
					<button
						class="mt-2 text-sm font-medium text-red-800 dark:text-red-200 underline"
						on:click={loadData}>Retry</button
					>
				</div>
			</div>
		</div>
	{/if}

	<div class="flex items-center justify-between gap-4">
		<input
			type="text"
			bind:value={searchTerm}
			placeholder="Filter by job name, repository or label..."
			class="block w-full max-w-md rounded-md border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-white shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
		/>
		<button
			on:click={loadData}
			class="inline-flex items-center px-3 py-2 border border-gray-300 dark:border-gray-600 shadow-sm text-sm font-medium rounded-md text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600"
		>
			Refresh
		</button>
	</div>

	{#if loading}
		<div class="flex justify-center py-12">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
		</div>
	{:else if queueGroups.length === 0}
		<div class="text-center py-12 bg-white dark:bg-gray-800 shadow rounded-lg">
			<p class="text-gray-500 dark:text-gray-400">No queued or running jobs.</p>
		</div>
	{:else}
		{#each queueGroups as group (group.key)}
			<div class="bg-white dark:bg-gray-800 shadow rounded-lg overflow-hidden">
				<div class="px-4 py-4 sm:px-6 border-b border-gray-200 dark:border-gray-700 flex flex-wrap items-center justify-between gap-2">
					<div>
						<h2 class="text-lg font-medium text-gray-900 dark:text-white">
							{group.title}
						</h2>
						<p class="text-sm text-gray-500 dark:text-gray-400">{group.subtitle}</p>
					</div>
					<div class="flex items-center gap-2 text-sm">
						<span class="inline-flex items-center px-2.5 py-0.5 rounded-full font-medium bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200">
							{group.queued.length} queued
						</span>
						<span class="inline-flex items-center px-2.5 py-0.5 rounded-full font-medium bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200">
							{group.inProgress.length} running{group.maxRunners ? ` / ${group.maxRunners} max` : ''}
						</span>
					</div>
				</div>

				{#if group.queued.length === 0}
					<div class="px-4 py-4 sm:px-6 text-sm text-gray-500 dark:text-gray-400">
						Queue is empty.
					</div>
				{:else}
					<div class="overflow-x-auto">
						<table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
							<thead class="bg-gray-50 dark:bg-gray-900/50">
								<tr>
									<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider w-12">#</th>
									<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Job</th>
									<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Repository</th>
									<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Labels</th>
									<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Waiting</th>
								</tr>
							</thead>
							<tbody class="divide-y divide-gray-200 dark:divide-gray-700">
								{#each group.queued as job, idx (job.id)}
									<tr class="hover:bg-gray-50 dark:hover:bg-gray-700/50">
										<td class="px-4 py-2 text-sm text-gray-500 dark:text-gray-400">{idx + 1}</td>
										<td class="px-4 py-2 text-sm text-gray-900 dark:text-white">
											{#if job.workflow_run_url}
												<a
													href={job.workflow_run_url}
													target="_blank"
													rel="noopener noreferrer"
													class="text-blue-600 dark:text-blue-400 hover:underline">{job.name || '(unnamed job)'}</a
												>
											{:else}
												{job.name || '(unnamed job)'}
											{/if}
										</td>
										<td class="px-4 py-2 text-sm text-gray-500 dark:text-gray-400">
											{job.repository_owner}/{job.repository_name}
										</td>
										<td class="px-4 py-2 text-sm">
											<div class="flex flex-wrap gap-1">
												{#each job.labels || [] as label}
													<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300">{label}</span>
												{/each}
											</div>
										</td>
										<td class="px-4 py-2 text-sm text-gray-500 dark:text-gray-400 whitespace-nowrap">
											{waitingFor(job)}
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				{/if}
			</div>
		{/each}
	{/if}
</div>
