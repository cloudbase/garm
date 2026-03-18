<script lang="ts">
	import { onDestroy } from 'svelte';
	import { resolve } from '$app/paths';
	import { websocketStore, type WebSocketEvent } from '$lib/stores/websocket.js';
	import { eagerCache } from '$lib/stores/eager-cache.js';
	import { metricsStore, type MetricsSnapshot, type MetricsPool, type MetricsScaleSet, type MetricsConnectionState } from '$lib/stores/metrics-ws.js';
	import ControllerInfoCard from '$lib/components/ControllerInfoCard.svelte';
	import Badge from '$lib/components/Badge.svelte';
	import type { ControllerInfo } from '$lib/api/generated/api.js';

	let stats = {
		repositories: 0,
		organizations: 0,
		enterprises: 0,
		pools: 0,
		scalesets: 0,
		instances: 0
	};

	let statusBreakdown = { active: 0, idle: 0, offline: 0, pending: 0, other: 0 };
	let entityList: Array<{ type: string; name: string; id: string; poolCount: number; scaleSetCount: number; healthy: boolean; href: string }> = [];
	let poolCapacity: Array<{ id: string; name: string; provider: string; osType: string; current: number; max: number; utilization: number; href: string }> = [];

	let controllerInfo: ControllerInfo | null = null;
	let unsubscribeWebsockets: (() => void)[] = [];

	let metricsConnection: MetricsConnectionState = 'connecting';
	const unsubscribeConnection = metricsStore.connectionState.subscribe(state => {
		metricsConnection = state;
	});

	$: metricsConnected = $metricsStore !== null;

	$: {
		if (!controllerInfo || $eagerCache.loaded.controllerInfo) {
			controllerInfo = $eagerCache.controllerInfo;
		}
	}

	// === Derive dashboard data from metrics snapshot (updated every 5s) ===

	function sumRunnerCounts(counts: Record<string, number>): number {
		return Object.values(counts).reduce((a, b) => a + b, 0);
	}

	function computeStatusFromSnapshot(snap: MetricsSnapshot) {
		const groups = { active: 0, idle: 0, offline: 0, pending: 0, other: 0 };
		const allItems: Array<MetricsPool | MetricsScaleSet> = [...snap.pools, ...snap.scale_sets];
		for (const item of allItems) {
			for (const [status, count] of Object.entries(item.runner_status_counts || {})) {
				switch (status) {
					case 'active': groups.active += count; break;
					case 'idle': case 'online': groups.idle += count; break;
					case 'offline': case 'terminated': case 'failed': groups.offline += count; break;
					case 'pending': case 'installing': groups.pending += count; break;
					default: groups.other += count; break;
				}
			}
		}
		return groups;
	}

	function getPoolDisplayName(p: MetricsPool): string {
		const entity = p.repo_name || p.org_name || p.enterprise_name || '';
		const shortId = (p.id || '').slice(0, 8);
		return entity ? `${entity} / ${shortId}` : shortId || 'Unknown';
	}

	function entityHref(type: string, id: string): string {
		switch (type) {
			case 'repository': return resolve(`/repositories/${id}`);
			case 'organization': return resolve(`/organizations/${id}`);
			case 'enterprise': return resolve(`/enterprises/${id}`);
			default: return resolve('/');
		}
	}

	function buildEntityListFromSnapshot(snap: MetricsSnapshot) {
		return snap.entities
			.map(e => ({
				type: e.type,
				name: e.name || 'Unknown',
				id: e.id,
				poolCount: e.pool_count,
				scaleSetCount: e.scale_set_count || 0,
				healthy: e.healthy,
				href: entityHref(e.type, e.id)
			}))
			.sort((a, b) => (b.poolCount + b.scaleSetCount) - (a.poolCount + a.scaleSetCount))
			.slice(0, 5);
	}

	function buildPoolCapacityFromSnapshot(snap: MetricsSnapshot) {
		return snap.pools
			.filter(p => p.enabled)
			.map(p => {
				const current = sumRunnerCounts(p.runner_counts);
				const max = p.max_runners || 0;
				return {
					id: p.id || '',
					name: getPoolDisplayName(p),
					provider: p.provider_name || '',
					osType: p.os_type || '',
					current,
					max,
					utilization: max ? current / max : 0,
					href: resolve(`/pools/${p.id}`)
				};
			})
			.sort((a, b) => b.utilization - a.utilization)
			.slice(0, 5);
	}

	// React to metrics snapshot updates (every 5s from server)
	$: if ($metricsStore) {
		lastSnapshot = $metricsStore;
		rederiveFromSnapshot($metricsStore);
	}

	// === Instant updates from WebSocket events (for "live" feel) ===
	// We hold a mutable copy of the last snapshot so event handlers can patch it
	// and trigger reactive re-derivation immediately (without waiting 5s).
	let lastSnapshot: MetricsSnapshot | null = null;

	function patchSnapshot(patcher: (snap: MetricsSnapshot) => MetricsSnapshot) {
		if (!lastSnapshot) return;
		lastSnapshot = patcher(lastSnapshot);
		rederiveFromSnapshot(lastSnapshot);
	}

	function rederiveFromSnapshot(snap: MetricsSnapshot) {
		const newStats = {
			repositories: snap.entities.filter(e => e.type === 'repository').length,
			organizations: snap.entities.filter(e => e.type === 'organization').length,
			enterprises: snap.entities.filter(e => e.type === 'enterprise').length,
			pools: snap.pools.length,
			scalesets: snap.scale_sets.length,
			instances: [...snap.pools, ...snap.scale_sets].reduce((sum, p) => sum + sumRunnerCounts(p.runner_counts), 0)
		};

		for (const key of Object.keys(newStats) as Array<keyof typeof stats>) {
			if (newStats[key] !== stats[key]) {
				const el = document.querySelector(`[data-stat="${key}"]`) as HTMLElement;
				if (el) animateNumber(el, newStats[key], 500);
			}
		}

		stats = newStats;
		statusBreakdown = computeStatusFromSnapshot(snap);
		entityList = buildEntityListFromSnapshot(snap);
		poolCapacity = buildPoolCapacityFromSnapshot(snap);
	}

	function handleCountEvent(key: keyof typeof stats) {
		return (event: WebSocketEvent) => {
			const element = document.querySelector(`[data-stat="${key}"]`) as HTMLElement;
			if (event.operation === 'create') {
				stats[key]++;
				if (element) animateNumber(element, stats[key], 500);
			} else if (event.operation === 'delete') {
				stats[key] = Math.max(0, stats[key] - 1);
				if (element) animateNumber(element, stats[key], 500);
			}
		};
	}

	function handlePoolEvent(event: WebSocketEvent) {
		handleCountEvent('pools')(event);
		if (event.operation === 'update' && event.payload) {
			patchSnapshot(snap => ({
				...snap,
				pools: snap.pools.map(p =>
					p.id === event.payload.id
						? { ...p, enabled: event.payload.enabled, max_runners: event.payload.max_runners, provider_name: event.payload.provider_name, os_type: event.payload.os_type }
						: p
				)
			}));
		}
	}

	function handleInstanceEvent(event: WebSocketEvent) {
		handleCountEvent('instances')(event);
		if (event.payload) {
			const poolId = event.payload.pool_id;
			if (!poolId) return;
			patchSnapshot(snap => ({
				...snap,
				pools: snap.pools.map(p => {
					if (p.id !== poolId) return p;
					const counts = { ...p.runner_counts };
					const status = event.payload.status || 'unknown';
					if (event.operation === 'create') {
						counts[status] = (counts[status] || 0) + 1;
					} else if (event.operation === 'delete') {
						// Find and decrement any status (we may not know the old status)
						const oldStatus = event.payload.status || 'unknown';
						counts[oldStatus] = Math.max(0, (counts[oldStatus] || 0) - 1);
						if (counts[oldStatus] === 0) delete counts[oldStatus];
					} else if (event.operation === 'update') {
						// Can't know old status, let the 5s snapshot correct it
						// But at least ensure the new status is counted
						counts[status] = (counts[status] || 0);
					}
					return { ...p, runner_counts: counts };
				})
			}));
		}
	}

	// Subscribe to all relevant entity types with create/update/delete
	const repoSub = websocketStore.subscribeToEntity('repository', ['create', 'update', 'delete'], handleCountEvent('repositories'));
	const orgSub = websocketStore.subscribeToEntity('organization', ['create', 'update', 'delete'], handleCountEvent('organizations'));
	const entSub = websocketStore.subscribeToEntity('enterprise', ['create', 'update', 'delete'], handleCountEvent('enterprises'));
	const poolSub = websocketStore.subscribeToEntity('pool', ['create', 'update', 'delete'], handlePoolEvent);
	const scaleSub = websocketStore.subscribeToEntity('scaleset', ['create', 'update', 'delete'], handleCountEvent('scalesets'));
	const instSub = websocketStore.subscribeToEntity('instance', ['create', 'update', 'delete'], handleInstanceEvent);
	const credSub = websocketStore.subscribeToEntity('github_credentials', ['create', 'update', 'delete'], () => {});
	const giteaCredSub = websocketStore.subscribeToEntity('gitea_credentials', ['create', 'update', 'delete'], () => {});
	const endpointSub = websocketStore.subscribeToEntity('github_endpoint', ['create', 'update', 'delete'], () => {});
	unsubscribeWebsockets = [repoSub, orgSub, entSub, poolSub, scaleSub, instSub, credSub, giteaCredSub, endpointSub];

	onDestroy(() => {
		unsubscribeWebsockets.forEach(unsubscribe => unsubscribe());
		unsubscribeConnection();
	});

	function animateNumber(element: HTMLElement, targetValue: number, duration: number = 1000) {
		const startValue = parseInt(element.textContent || '0');
		const increment = (targetValue - startValue) / (duration / 16);
		let currentValue = startValue;

		const animate = () => {
			currentValue += increment;
			if ((increment > 0 && currentValue >= targetValue) || (increment < 0 && currentValue <= targetValue)) {
				element.textContent = targetValue.toString();
				return;
			}
			element.textContent = Math.floor(currentValue).toString();
			requestAnimationFrame(animate);
		};

		if (startValue !== targetValue) {
			requestAnimationFrame(animate);
		}
	}

	function handleControllerUpdate(event: CustomEvent<ControllerInfo>) {
		controllerInfo = event.detail;
	}

	function formatEntityCounts(poolCount: number, scaleSetCount: number): string {
		const parts: string[] = [];
		if (poolCount > 0) parts.push(`${poolCount} ${poolCount === 1 ? 'pool' : 'pools'}`);
		if (scaleSetCount > 0) parts.push(`${scaleSetCount} ${scaleSetCount === 1 ? 'scale set' : 'scale sets'}`);
		return parts.length > 0 ? parts.join(', ') : 'no pools';
	}

	function getEntityIcon(type: string): string {
		switch (type) {
			case 'repository': return 'M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z';
			case 'organization': return 'M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z';
			case 'enterprise': return 'M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4';
			default: return '';
		}
	}

	function getEntityTypeBadge(type: string): 'blue' | 'green' | 'purple' {
		switch (type) {
			case 'repository': return 'blue';
			case 'organization': return 'green';
			case 'enterprise': return 'purple';
			default: return 'blue';
		}
	}

	$: totalInstances = statusBreakdown.active + statusBreakdown.idle + statusBreakdown.offline + statusBreakdown.pending + statusBreakdown.other;

	$: statCards = [
		{ title: 'Repositories', value: stats.repositories, key: 'repositories', href: resolve('/repositories'),
		  icon: 'M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z', color: 'blue' },
		{ title: 'Organizations', value: stats.organizations, key: 'organizations', href: resolve('/organizations'),
		  icon: 'M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z', color: 'green' },
		{ title: 'Enterprises', value: stats.enterprises, key: 'enterprises', href: resolve('/enterprises'),
		  icon: 'M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4', color: 'purple' },
		{ title: 'Pools', value: stats.pools, key: 'pools', href: resolve('/pools'),
		  icon: 'M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10', color: 'indigo' },
		{ title: 'Scale Sets', value: stats.scalesets, key: 'scalesets', href: resolve('/scalesets'),
		  icon: 'M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4', color: 'teal' },
		{ title: 'Instances', value: stats.instances, key: 'instances', href: resolve('/instances'),
		  icon: 'M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z', color: 'yellow' }
	];

	function getStatColor(color: string) {
		const map: Record<string, string> = {
			blue: 'bg-blue-100 text-blue-600 dark:bg-blue-900/50 dark:text-blue-400',
			green: 'bg-green-100 text-green-600 dark:bg-green-900/50 dark:text-green-400',
			purple: 'bg-purple-100 text-purple-600 dark:bg-purple-900/50 dark:text-purple-400',
			indigo: 'bg-indigo-100 text-indigo-600 dark:bg-indigo-900/50 dark:text-indigo-400',
			teal: 'bg-teal-100 text-teal-600 dark:bg-teal-900/50 dark:text-teal-400',
			yellow: 'bg-yellow-100 text-yellow-600 dark:bg-yellow-900/50 dark:text-yellow-400'
		};
		return map[color] || map.blue;
	}
</script>

<svelte:head>
	<title>Dashboard - GARM</title>
</svelte:head>

<div class="space-y-6">
	<!-- Header -->
	<div class="flex items-start justify-between">
		<div>
			<h1 class="text-2xl font-bold text-gray-900 dark:text-white">Dashboard</h1>
			<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
				Welcome to GARM - GitHub Actions Runner Manager
			</p>
		</div>
		<a
			href={resolve('/setup')}
			class="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 dark:focus:ring-offset-gray-900 transition-colors"
		>
			<svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
			</svg>
			Setup Wizard
		</a>
	</div>

	<!-- Connection Status Banner -->
	{#if !metricsConnected}
		<div class="rounded-lg border px-4 py-3 flex items-center space-x-3 {metricsConnection === 'connecting' ? 'bg-yellow-50 border-yellow-200 dark:bg-yellow-900/20 dark:border-yellow-700' : 'bg-red-50 border-red-200 dark:bg-red-900/20 dark:border-red-700'}">
			{#if metricsConnection === 'connecting'}
				<svg class="w-5 h-5 text-yellow-500 dark:text-yellow-400 animate-spin flex-shrink-0" fill="none" viewBox="0 0 24 24">
					<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
					<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
				</svg>
				<p class="text-sm text-yellow-700 dark:text-yellow-300">Connecting to live metrics...</p>
			{:else}
				<svg class="w-5 h-5 text-red-500 dark:text-red-400 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4.5c-.77-.833-2.694-.833-3.464 0L3.34 16.5c-.77.833.192 2.5 1.732 2.5z" />
				</svg>
				<div>
					<p class="text-sm font-medium text-red-700 dark:text-red-300">Live metrics unavailable</p>
					<p class="text-xs text-red-600 dark:text-red-400">WebSocket connection lost. Dashboard data may be stale. Reconnecting automatically...</p>
				</div>
			{/if}
		</div>
	{/if}

	<!-- Stats Grid -->
	<div class="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-6 {!metricsConnected ? 'opacity-50 pointer-events-none' : ''}" class:transition-opacity={true}>
		{#each statCards as card}
			<a
				href={card.href}
				class="bg-white dark:bg-gray-800 overflow-hidden shadow rounded-lg hover:shadow-md transition-shadow duration-200 p-4"
			>
				<div class="flex items-center space-x-3">
					<div class="flex-shrink-0 w-9 h-9 rounded-lg {getStatColor(card.color)} flex items-center justify-center">
						<svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={card.icon}></path>
						</svg>
					</div>
					<div class="min-w-0">
						<p class="text-xs font-medium text-gray-500 dark:text-gray-400 truncate">{card.title}</p>
						<p class="text-lg font-semibold text-gray-900 dark:text-white" data-stat={card.key}>{card.value}</p>
					</div>
				</div>
			</a>
		{/each}
	</div>

	<!-- Runner Status -->
	<div class="bg-white dark:bg-gray-800 shadow rounded-lg p-5 {!metricsConnected ? 'opacity-50 pointer-events-none' : ''}" class:transition-opacity={true}>
		<h3 class="text-sm font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider mb-4">Runner Status</h3>

		{#if totalInstances > 0}
			<div class="w-full h-4 rounded-full bg-gray-100 dark:bg-gray-700 overflow-hidden flex">
				{#if statusBreakdown.active > 0}
					<div
						class="bg-green-500 dark:bg-green-400 transition-all duration-500"
						style="width: {(statusBreakdown.active / totalInstances) * 100}%"
						title="{statusBreakdown.active} active"
					></div>
				{/if}
				{#if statusBreakdown.idle > 0}
					<div
						class="bg-blue-400 dark:bg-blue-400 transition-all duration-500"
						style="width: {(statusBreakdown.idle / totalInstances) * 100}%"
						title="{statusBreakdown.idle} idle"
					></div>
				{/if}
				{#if statusBreakdown.pending > 0}
					<div
						class="bg-purple-400 dark:bg-purple-400 transition-all duration-500"
						style="width: {(statusBreakdown.pending / totalInstances) * 100}%"
						title="{statusBreakdown.pending} pending"
					></div>
				{/if}
				{#if statusBreakdown.offline > 0}
					<div
						class="bg-red-500 dark:bg-red-400 transition-all duration-500"
						style="width: {(statusBreakdown.offline / totalInstances) * 100}%"
						title="{statusBreakdown.offline} offline"
					></div>
				{/if}
				{#if statusBreakdown.other > 0}
					<div
						class="bg-gray-400 dark:bg-gray-500 transition-all duration-500"
						style="width: {(statusBreakdown.other / totalInstances) * 100}%"
						title="{statusBreakdown.other} other"
					></div>
				{/if}
			</div>

			<div class="flex flex-wrap gap-x-5 gap-y-1 mt-3 text-sm">
				{#if statusBreakdown.active > 0}
					<span class="flex items-center">
						<span class="w-2.5 h-2.5 rounded-full bg-green-500 dark:bg-green-400 mr-1.5"></span>
						<span class="text-gray-600 dark:text-gray-300">{statusBreakdown.active} Active</span>
					</span>
				{/if}
				{#if statusBreakdown.idle > 0}
					<span class="flex items-center">
						<span class="w-2.5 h-2.5 rounded-full bg-blue-400 dark:bg-blue-400 mr-1.5"></span>
						<span class="text-gray-600 dark:text-gray-300">{statusBreakdown.idle} Idle</span>
					</span>
				{/if}
				{#if statusBreakdown.pending > 0}
					<span class="flex items-center">
						<span class="w-2.5 h-2.5 rounded-full bg-purple-400 dark:bg-purple-400 mr-1.5"></span>
						<span class="text-gray-600 dark:text-gray-300">{statusBreakdown.pending} Pending</span>
					</span>
				{/if}
				{#if statusBreakdown.offline > 0}
					<span class="flex items-center">
						<span class="w-2.5 h-2.5 rounded-full bg-red-500 dark:bg-red-400 mr-1.5"></span>
						<span class="text-gray-600 dark:text-gray-300">{statusBreakdown.offline} Offline</span>
					</span>
				{/if}
				{#if statusBreakdown.other > 0}
					<span class="flex items-center">
						<span class="w-2.5 h-2.5 rounded-full bg-gray-400 dark:bg-gray-500 mr-1.5"></span>
						<span class="text-gray-600 dark:text-gray-300">{statusBreakdown.other} Other</span>
					</span>
				{/if}
			</div>
		{:else}
			<p class="text-sm text-gray-500 dark:text-gray-400">No instances running. Use the Setup Wizard to create your first runner pool.</p>
		{/if}
	</div>

	<!-- Entities + Pool Capacity -->
	<div class="grid grid-cols-1 lg:grid-cols-2 gap-6 {!metricsConnected ? 'opacity-50 pointer-events-none' : ''}" class:transition-opacity={true}>
		<!-- Entities -->
		<div class="bg-white dark:bg-gray-800 shadow rounded-lg p-5">
			<div class="flex items-center justify-between mb-4">
				<h3 class="text-sm font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Entities</h3>
				<div class="flex space-x-2 text-xs">
					<a href={resolve('/repositories')} class="text-blue-600 dark:text-blue-400 hover:underline">Repos</a>
					<span class="text-gray-300 dark:text-gray-600">|</span>
					<a href={resolve('/organizations')} class="text-blue-600 dark:text-blue-400 hover:underline">Orgs</a>
					<span class="text-gray-300 dark:text-gray-600">|</span>
					<a href={resolve('/enterprises')} class="text-blue-600 dark:text-blue-400 hover:underline">Enterprises</a>
				</div>
			</div>

			{#if entityList.length > 0}
				<div class="space-y-2">
					{#each entityList as entity}
						<a
							href={entity.href}
							class="flex items-center justify-between p-2.5 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors group"
						>
							<div class="flex items-center space-x-3 min-w-0">
								<svg class="w-4 h-4 flex-shrink-0 text-gray-400 dark:text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={getEntityIcon(entity.type)}></path>
								</svg>
								<span class="text-sm font-medium text-gray-900 dark:text-white truncate group-hover:text-blue-600 dark:group-hover:text-blue-400">{entity.name}</span>
								<Badge variant={getEntityTypeBadge(entity.type)} size="sm" text={{ repository: 'repo', organization: 'org', enterprise: 'ent' }[entity.type] || entity.type} />
							</div>
							<div class="flex items-center space-x-2 flex-shrink-0 ml-2">
								<span class="text-xs text-gray-500 dark:text-gray-400">{formatEntityCounts(entity.poolCount, entity.scaleSetCount)}</span>
								<span class="w-2 h-2 rounded-full {entity.healthy ? 'bg-green-500' : 'bg-red-500'}" title={entity.healthy ? 'Pool manager running' : 'Pool manager error'}></span>
							</div>
						</a>
					{/each}
				</div>
			{:else}
				<p class="text-sm text-gray-500 dark:text-gray-400 py-4 text-center">No entities configured yet.</p>
			{/if}
		</div>

		<!-- Pool Capacity -->
		<div class="bg-white dark:bg-gray-800 shadow rounded-lg p-5">
			<div class="flex items-center justify-between mb-4">
				<h3 class="text-sm font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Pool Capacity</h3>
				<a href={resolve('/pools')} class="text-xs text-blue-600 dark:text-blue-400 hover:underline">View All</a>
			</div>

			{#if poolCapacity.length > 0}
				<div class="space-y-3">
					{#each poolCapacity as pool}
						<a href={pool.href} class="block p-2.5 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors group">
							<div class="flex items-center justify-between mb-1.5">
								<span class="text-sm font-medium text-gray-900 dark:text-white truncate group-hover:text-blue-600 dark:group-hover:text-blue-400">{pool.name}</span>
								<span class="text-xs text-gray-500 dark:text-gray-400 flex-shrink-0 ml-2">{pool.current}/{pool.max}</span>
							</div>
							<div class="w-full h-2 rounded-full bg-gray-100 dark:bg-gray-700 overflow-hidden">
								<div
									class="h-full rounded-full transition-all duration-300 {pool.utilization > 0.9 ? 'bg-red-500' : pool.utilization > 0.7 ? 'bg-yellow-500' : 'bg-green-500'}"
									style="width: {Math.min(pool.utilization * 100, 100)}%"
								></div>
							</div>
							<div class="flex items-center justify-between mt-1">
								<span class="text-xs text-gray-400 dark:text-gray-500">{pool.provider}{pool.osType ? ` / ${pool.osType}` : ''}</span>
								<span class="text-xs text-gray-400 dark:text-gray-500">{Math.round(pool.utilization * 100)}%</span>
							</div>
						</a>
					{/each}
				</div>
			{:else}
				<p class="text-sm text-gray-500 dark:text-gray-400 py-4 text-center">No active pools.</p>
			{/if}
		</div>
	</div>

	<!-- Controller Information -->
	{#if controllerInfo}
		<ControllerInfoCard
			{controllerInfo}
			on:updated={handleControllerUpdate}
		/>
	{/if}
</div>
