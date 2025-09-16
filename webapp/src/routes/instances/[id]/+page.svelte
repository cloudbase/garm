<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { garmApi } from '$lib/api/client.js';
	import type { Instance } from '$lib/api/generated/api.js';
	import { resolve } from '$app/paths';
	import DeleteModal from '$lib/components/DeleteModal.svelte';
	import ShellTerminal from '$lib/components/ShellTerminal.svelte';
	import { websocketStore, type WebSocketEvent } from '$lib/stores/websocket.js';
	import { formatStatusText, getStatusBadgeClass } from '$lib/utils/status.js';
	import { formatDate, scrollToBottomEvents, getEventLevelBadge } from '$lib/utils/common.js';
	import { extractAPIError } from '$lib/utils/apiError';
	import Badge from '$lib/components/Badge.svelte';

	let instance: Instance | null = null;
	let loading = true;
	let error = '';
	let showDeleteModal = false;
	let showShellModal = false;
	let unsubscribeWebsocket: (() => void) | null = null;
	let statusMessagesContainer: HTMLElement;


	$: instanceName = decodeURIComponent($page.params.id || '');

	// Current time for heartbeat staleness check - updates every second
	let currentTime = Date.now();
	let heartbeatCheckInterval: ReturnType<typeof setInterval> | null = null;

	// Check if shell should be disabled (heartbeat stale or instance stopped)
	$: isHeartbeatStale = (instance?.agent_id) ?
		(() => {
			// Disable if instance doesn't have shell capability
			if (!instance.capabilities?.has_shell) return true;

			// Disable if instance status is "stopped"
			if (instance.status === 'stopped') return true;

			const lastHeartbeat = instance.heartbeat;
			if (!lastHeartbeat) return true;
			const heartbeatTime = new Date(lastHeartbeat);
			return (currentTime - heartbeatTime.getTime()) > 60000; // 60 seconds
		})() : true;

	async function loadInstance() {
		if (!instanceName) return;
		
		try {
			loading = true;
			error = '';
			instance = await garmApi.getInstance(instanceName);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load instance';
		} finally {
			loading = false;
		}
	}

	async function handleDelete() {
		if (!instance) return;
		try {
			await garmApi.deleteInstance(instance.name!);
			goto(resolve('/instances'));
		} catch (err) {
			error = extractAPIError(err);
		}
		showDeleteModal = false;
	}

	function handleInstanceEvent(event: WebSocketEvent) {
		if (!instance) return;
		
		
		if (event.operation === 'update' && event.payload.id === instance.id) {
			// Check if status messages have been updated
			const oldMessageCount = instance.status_messages?.length || 0;
			const newInstance = { ...instance, ...event.payload };
			const newMessageCount = newInstance.status_messages?.length || 0;
			
			// Update instance
			instance = newInstance;
			
			// Auto-scroll if new messages were added
			if (newMessageCount > oldMessageCount) {
				// Use setTimeout to ensure the DOM has updated
				setTimeout(() => {
					scrollToBottomEvents(statusMessagesContainer);
				}, 100);
			}
		} else if (event.operation === 'delete') {
			// Instance was deleted - redirect to list page
			const instanceId = event.payload.id || event.payload;
			if (instanceId === instance.id) {
				goto(resolve('/instances'));
			}
		}
	}

	onMount(() => {
		loadInstance().then(() => {
			// Scroll to bottom on initial load if there are status messages
			if (instance?.status_messages?.length) {
				setTimeout(() => {
					scrollToBottomEvents(statusMessagesContainer);
				}, 100);
			}
		});

		// Subscribe to real-time instance events
		unsubscribeWebsocket = websocketStore.subscribeToEntity(
			'instance',
			['update', 'delete'],
			handleInstanceEvent
		);

		// Update current time every second for heartbeat staleness check
		heartbeatCheckInterval = setInterval(() => {
			currentTime = Date.now();
		}, 1000);
	});

	onDestroy(() => {
		// Clean up websocket subscription
		if (unsubscribeWebsocket) {
			unsubscribeWebsocket();
			unsubscribeWebsocket = null;
		}

		// Clean up heartbeat check interval
		if (heartbeatCheckInterval) {
			clearInterval(heartbeatCheckInterval);
			heartbeatCheckInterval = null;
		}
	});
</script>

<svelte:head>
	<title>{instance ? `${instance.name} - Instance Details` : 'Instance Details'} - GARM</title>
</svelte:head>

<div class="space-y-6">
	<!-- Breadcrumbs -->
	<nav class="flex" aria-label="Breadcrumb">
		<ol class="inline-flex items-center space-x-1 md:space-x-3">
			<li class="inline-flex items-center">
				<a href={resolve('/instances')} class="inline-flex items-center text-sm font-medium text-gray-700 hover:text-blue-600 dark:text-gray-400 dark:hover:text-white">
					<svg class="w-3 h-3 mr-2.5" fill="currentColor" viewBox="0 0 20 20">
						<path d="M10.707 2.293a1 1 0 00-1.414 0l-7 7a1 1 0 001.414 1.414L4 10.414V17a1 1 0 001 1h2a1 1 0 001-1v-2a1 1 0 011-1h2a1 1 0 011 1v2a1 1 0 001 1h2a1 1 0 001-1v-6.586l.293.293a1 1 0 001.414-1.414l-7-7z"/>
					</svg>
					Instances
				</a>
			</li>
			<li>
				<div class="flex items-center">
					<svg class="w-6 h-6 text-gray-400" fill="currentColor" viewBox="0 0 20 20">
						<path fill-rule="evenodd" d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z" clip-rule="evenodd"/>
					</svg>
					<span class="text-sm font-medium text-gray-500 dark:text-gray-400 ml-1 md:ml-2">
						{instance ? instance.name : 'Instance Details'}
					</span>
				</div>
			</li>
		</ol>
	</nav>

	{#if error}
		<div class="bg-red-50 dark:bg-red-900/50 border border-red-200 dark:border-red-800 rounded-md p-4">
			<div class="flex">
				<div class="ml-3">
					<h3 class="text-sm font-medium text-red-800 dark:text-red-200">Error</h3>
					<div class="mt-2 text-sm text-red-700 dark:text-red-300">{error}</div>
				</div>
			</div>
		</div>
	{/if}

	{#if loading}
		<div class="bg-white dark:bg-gray-800 shadow rounded-lg">
			<div class="px-6 py-4 text-center">
				<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600 mx-auto"></div>
				<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">Loading instance details...</p>
			</div>
		</div>
	{:else if instance}
		<!-- Instance Information Cards -->
		<div class="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-6">
			<!-- Basic Information -->
			<div class="bg-white dark:bg-gray-800 shadow rounded-lg p-6">
				<div class="flex items-center justify-between mb-4">
					<h3 class="text-lg font-medium text-gray-900 dark:text-white">Instance Information</h3>
					<div class="flex items-center space-x-3">
						<button
							on:click={() => showShellModal = true}
							disabled={isHeartbeatStale}
							class="px-4 py-2 {isHeartbeatStale ? 'bg-gray-400 cursor-not-allowed' : 'bg-blue-600 hover:bg-blue-700 dark:bg-blue-700 dark:hover:bg-blue-800 cursor-pointer'} text-white rounded-lg font-medium text-sm flex items-center space-x-2"
							title={isHeartbeatStale ?
								(!instance?.capabilities?.has_shell ? "Shell unavailable - Agent does not support shell" :
								instance?.status === 'stopped' ? "Shell unavailable - Instance is stopped" :
								"Shell unavailable - Agent heartbeat is stale") :
								"Open Shell"}
						>
							<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v14a2 2 0 002 2z"></path>
							</svg>
							<span>Shell</span>
						</button>
						<button
							on:click={() => showDeleteModal = true}
							class="px-4 py-2 bg-red-600 hover:bg-red-700 dark:bg-red-700 dark:hover:bg-red-800 text-white rounded-lg font-medium text-sm cursor-pointer"
						>
							Delete Instance
						</button>
					</div>
				</div>
				<dl class="space-y-3">
					<div class="flex justify-between">
						<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">ID:</dt>
						<dd class="text-sm font-mono text-gray-900 dark:text-white break-all">{instance.id}</dd>
					</div>
					<div class="flex justify-between">
						<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Name:</dt>
						<dd class="text-sm text-gray-900 dark:text-white">{instance.name}</dd>
					</div>
					<div class="flex justify-between">
						<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Provider ID:</dt>
						<dd class="text-sm font-mono text-gray-900 dark:text-white break-all">{instance.provider_id}</dd>
					</div>
					<div class="flex justify-between">
						<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Provider:</dt>
						<dd class="text-sm text-gray-900 dark:text-white">{instance.provider_name || 'Unknown'}</dd>
					</div>
					<div class="flex justify-between">
						<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Pool/Scale Set:</dt>
						<dd class="text-sm font-mono text-gray-900 dark:text-white break-all">
							{#if instance.pool_id}
								<a href={resolve(`/pools/${instance.pool_id}`)} class="text-blue-600 dark:text-blue-400 hover:text-blue-500 dark:hover:text-blue-300 hover:underline">
									{instance.pool_id}
								</a>
							{:else if instance.scale_set_id}
								<a href={resolve(`/scalesets/${instance.scale_set_id}`)} class="text-blue-600 dark:text-blue-400 hover:text-blue-500 dark:hover:text-blue-300 hover:underline">
									{instance.scale_set_id}
								</a>
							{:else}
								<span class="text-gray-400 dark:text-gray-500">-</span>
							{/if}
						</dd>
					</div>
					<div class="flex justify-between">
						<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Agent ID:</dt>
						<dd class="text-sm font-mono text-gray-900 dark:text-white">{instance.agent_id || 'Not assigned'}</dd>
					</div>
					<div class="flex justify-between">
						<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Created At:</dt>
						<dd class="text-sm text-gray-900 dark:text-white">{formatDate(instance.created_at!)}</dd>
					</div>
					{#if instance.updated_at && instance.updated_at !== instance.created_at}
					<div class="flex justify-between">
						<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Updated At:</dt>
						<dd class="text-sm text-gray-900 dark:text-white">{formatDate(instance.updated_at)}</dd>
					</div>
					{/if}
				</dl>
			</div>

			<!-- Status & Network Information -->
			<div class="bg-white dark:bg-gray-800 shadow rounded-lg p-6">
				<h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">Status & Network</h3>
				<dl class="space-y-3">
					<div class="flex justify-between items-center">
						<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Instance Status:</dt>
						<dd class="text-sm">
							<span class="inline-flex px-2 py-1 text-xs font-semibold rounded-full ring-1 ring-inset {getStatusBadgeClass(instance.status || 'unknown')}">
								{formatStatusText(instance.status || 'unknown')}
							</span>
						</dd>
					</div>
					<div class="flex justify-between items-center">
						<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Runner Status:</dt>
						<dd class="text-sm">
							<span class="inline-flex px-2 py-1 text-xs font-semibold rounded-full ring-1 ring-inset {getStatusBadgeClass(instance.runner_status || 'unknown')}">
								{formatStatusText(instance.runner_status || 'unknown')}
							</span>
						</dd>
					</div>
					{#if instance.addresses && instance.addresses.length > 0}
					<div>
						<dt class="text-sm font-medium text-gray-500 dark:text-gray-400 mb-2">Network Addresses:</dt>
						<dd class="text-sm space-y-1">
							{#each instance.addresses as address}
							<div class="flex justify-between items-center bg-gray-50 dark:bg-gray-700 p-2 rounded">
								<span class="font-mono text-gray-900 dark:text-white">{address.address}</span>
								<Badge variant="info" text={address.type || 'Unknown'} />
							</div>
							{/each}
						</dd>
					</div>
					{:else}
					<div>
						<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">Network Addresses:</dt>
						<dd class="text-sm text-gray-500 dark:text-gray-400 italic">No addresses available</dd>
					</div>
					{/if}
					{#if instance.os_type}
					<div class="flex justify-between">
						<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">OS Type:</dt>
						<dd class="text-sm text-gray-900 dark:text-white">{instance.os_type}</dd>
					</div>
					{/if}
					{#if instance.os_name}
					<div class="flex justify-between">
						<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">OS Name:</dt>
						<dd class="text-sm text-gray-900 dark:text-white">{instance.os_name}</dd>
					</div>
					{/if}
					{#if instance.os_version}
					<div class="flex justify-between">
						<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">OS Version:</dt>
						<dd class="text-sm text-gray-900 dark:text-white">{instance.os_version}</dd>
					</div>
					{/if}
					{#if instance.os_arch}
					<div class="flex justify-between">
						<dt class="text-sm font-medium text-gray-500 dark:text-gray-400">OS Architecture:</dt>
						<dd class="text-sm text-gray-900 dark:text-white">{instance.os_arch}</dd>
					</div>
					{/if}
				</dl>
			</div>
		</div>

		{#if instance.status_messages && instance.status_messages.length > 0}
		<!-- Status Messages -->
		<div class="bg-white dark:bg-gray-800 shadow rounded-lg p-6">
			<h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">Status Messages</h3>
			<div bind:this={statusMessagesContainer} class="space-y-3 max-h-96 overflow-y-auto scroll-smooth">
				{#each instance.status_messages as message}
				<div class="bg-gray-50 dark:bg-gray-700 p-4 rounded-lg">
					<div class="flex justify-between items-start">
						<p class="text-sm text-gray-900 dark:text-white flex-1 mr-4">{message.message}</p>
						<div class="flex items-center space-x-2 flex-shrink-0">
							{#if message.event_level}
								{@const levelBadge = getEventLevelBadge(message.event_level)}
								<Badge variant={levelBadge.variant} text={levelBadge.text} />
							{/if}
							<span class="text-xs text-gray-500 dark:text-gray-400">
								{#if message.created_at}
									{formatDate(message.created_at)}
								{:else}
									Unknown date
								{/if}
							</span>
						</div>
					</div>
				</div>
				{/each}
			</div>
		</div>
		{:else}
		<!-- No Status Messages -->
		<div class="bg-white dark:bg-gray-800 shadow rounded-lg p-6">
			<h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">Status Messages</h3>
			<div class="text-center py-8">
				<svg class="w-12 h-12 text-gray-400 dark:text-gray-500 mx-auto mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
				</svg>
				<p class="text-sm text-gray-500 dark:text-gray-400">No status messages available</p>
			</div>
		</div>
		{/if}
	{:else}
		<div class="bg-white dark:bg-gray-800 shadow rounded-lg">
			<div class="px-6 py-8 text-center text-gray-500 dark:text-gray-400">
				Instance not found.
			</div>
		</div>
	{/if}
</div>

<!-- Shell Modal -->
{#if showShellModal && instance && !isHeartbeatStale}
	<div class="fixed inset-0 bg-black/30 dark:bg-black/50 overflow-hidden h-full w-full z-50">
		<div class="relative w-full h-full flex items-center justify-center p-4">
			<ShellTerminal
				runnerName={instance.name!}
				onClose={() => showShellModal = false}
			/>
		</div>
	</div>
{/if}

<!-- Delete Modal -->
{#if showDeleteModal && instance}
	<DeleteModal
		title="Delete Instance"
		message="Are you sure you want to delete this instance? This action cannot be undone."
		itemName={instance.name}
		on:close={() => showDeleteModal = false}
		on:confirm={handleDelete}
	/>
{/if}