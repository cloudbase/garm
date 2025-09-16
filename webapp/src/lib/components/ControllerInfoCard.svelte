<script lang="ts">
	import { createEventDispatcher } from 'svelte';
	import Button from './Button.svelte';
	import Modal from './Modal.svelte';
	import Tooltip from './Tooltip.svelte';
	import { toastStore } from '$lib/stores/toast.js';
	import { garmApi } from '$lib/api/client.js';
	import type { ControllerInfo, UpdateControllerParams } from '$lib/api/generated/api.js';

	export let controllerInfo: ControllerInfo;
	
	const dispatch = createEventDispatcher<{
		updated: ControllerInfo;
	}>();

	let showSettingsModal = false;
	let saving = false;

	// Edit form values
	let metadataUrl = '';
	let callbackUrl = '';
	let webhookUrl = '';
	let agentUrl = '';
	let minimumJobAgeBackoff: number | null = null;
	let garmAgentReleasesUrl = '';
	let syncGarmAgentTools = false;


	function openSettingsModal() {
		// Pre-populate form with current values
		metadataUrl = controllerInfo.metadata_url || '';
		callbackUrl = controllerInfo.callback_url || '';
		webhookUrl = controllerInfo.webhook_url || '';
		agentUrl = controllerInfo.agent_url || '';
		minimumJobAgeBackoff = controllerInfo.minimum_job_age_backoff || null;
		garmAgentReleasesUrl = controllerInfo.garm_agent_releases_url || '';
		syncGarmAgentTools = controllerInfo.enable_agent_tools_sync ?? false;

		showSettingsModal = true;
	}

	async function saveSettings() {
		try {
			saving = true;

			// Build update params - only include non-empty values
			const updateParams: UpdateControllerParams = {};

			if (metadataUrl.trim()) {
				updateParams.metadata_url = metadataUrl.trim();
			}
			if (callbackUrl.trim()) {
				updateParams.callback_url = callbackUrl.trim();
			}
			if (webhookUrl.trim()) {
				updateParams.webhook_url = webhookUrl.trim();
			}
			if (agentUrl.trim()) {
				updateParams.agent_url = agentUrl.trim();
			}
			if (minimumJobAgeBackoff !== null && minimumJobAgeBackoff >= 0) {
				updateParams.minimum_job_age_backoff = minimumJobAgeBackoff;
			}
			if (garmAgentReleasesUrl.trim()) {
				updateParams.garm_agent_releases_url = garmAgentReleasesUrl.trim();
			}
			// Always send the boolean value
			updateParams.enable_agent_tools_sync = syncGarmAgentTools;

			// Update controller settings
			const updatedInfo = await garmApi.updateController(updateParams);
			
			toastStore.success(
				'Settings Updated',
				'Controller settings have been updated successfully.'
			);
			
			showSettingsModal = false;
			
			// Update the controllerInfo and notify parent
			controllerInfo = updatedInfo;
			dispatch('updated', updatedInfo);
		} catch (err) {
			toastStore.error(
				'Update Failed',
				err instanceof Error ? err.message : 'Failed to update controller settings'
			);
		} finally {
			saving = false;
		}
	}

	function closeSettingsModal() {
		showSettingsModal = false;
		// Reset form values
		metadataUrl = '';
		callbackUrl = '';
		webhookUrl = '';
		agentUrl = '';
		minimumJobAgeBackoff = null;
		garmAgentReleasesUrl = '';
		syncGarmAgentTools = false;
	}

	// Form validation
	$: isValidUrl = (url: string) => {
		if (!url.trim()) return true; // Empty is allowed
		try {
			new URL(url);
			return true;
		} catch {
			return false;
		}
	};

	$: isFormValid =
		isValidUrl(metadataUrl) &&
		isValidUrl(callbackUrl) &&
		isValidUrl(webhookUrl) &&
		isValidUrl(agentUrl) &&
		isValidUrl(garmAgentReleasesUrl) &&
		(minimumJobAgeBackoff === null || minimumJobAgeBackoff >= 0);
</script>

<div class="bg-white dark:bg-gray-800 shadow rounded-lg">
	<div class="p-6">
		<!-- Header with inline edit action -->
		<div class="flex items-center justify-between mb-6">
			<div class="flex items-center space-x-3">
				<div class="flex items-center justify-center w-10 h-10 rounded-lg bg-blue-100 dark:bg-blue-900">
					<svg class="w-6 h-6 text-blue-600 dark:text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
					</svg>
				</div>
				<div>
					<h3 class="text-lg font-semibold text-gray-900 dark:text-white">Controller Information</h3>
					<div class="mt-1">
						<span class="text-sm text-gray-500 dark:text-gray-400">
							v{controllerInfo.version?.replace(/^v/, '') || 'Unknown'}
						</span>
					</div>
				</div>
			</div>
			
			<Button
				variant="secondary"
				size="sm"
				on:click={openSettingsModal}
			>
				<svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"></path>
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"></path>
				</svg>
				Settings
			</Button>
		</div>

		<!-- Main content in clean grid layout -->
		<div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
			<!-- Left column - Identity & Config -->
			<div class="space-y-4">
				<div>
					<h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">Identity</h4>
					<div class="space-y-3">
						<!-- Controller ID -->
						<div>
							<div class="text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wide font-medium">Controller ID</div>
							<div class="mt-1 p-2 bg-gray-50 dark:bg-gray-700 rounded text-sm font-mono text-gray-600 dark:text-gray-300 break-all min-h-[38px] flex items-center">
								{controllerInfo.controller_id}
							</div>
						</div>

						<!-- Hostname -->
						<div>
							<div class="text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wide font-medium">Hostname</div>
							<div class="mt-1 p-2 bg-gray-50 dark:bg-gray-700 rounded text-sm font-mono text-gray-600 dark:text-gray-300 break-all min-h-[38px] flex items-center">
								{controllerInfo.hostname || 'Unknown'}
							</div>
						</div>

						<!-- Job Age Backoff -->
						<div>
							<div class="flex items-center">
								<div class="text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wide font-medium">Job Age Backoff</div>
								<div class="ml-2">
									<Tooltip
										title="Job Age Backoff"
										content="Time in seconds GARM waits after receiving a new job before spinning up a runner. This delay allows existing idle runners to pick up jobs first, preventing unnecessary runner creation. Set to 0 for immediate response."
									/>
								</div>
							</div>
							<div class="mt-1 p-2 bg-gray-50 dark:bg-gray-700 rounded text-sm font-mono text-gray-600 dark:text-gray-300 min-h-[38px] flex items-center">
								{controllerInfo.minimum_job_age_backoff || 30}s
							</div>
						</div>

						<!-- Agent Tools Sync -->
						<div>
							<div class="flex items-center">
								<div class="text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wide font-medium">Agent Tools Sync</div>
								<div class="ml-2">
									<Tooltip
										title="Agent Tools Sync"
										content="When enabled, GARM will automatically synchronize garm-agent tools from the configured releases URL. This ensures agents are up-to-date with the latest versions."
									/>
								</div>
							</div>
							<div class="mt-1 p-2 bg-gray-50 dark:bg-gray-700 rounded text-sm font-mono text-gray-600 dark:text-gray-300 min-h-[38px] flex items-center">
								{#if controllerInfo.enable_agent_tools_sync}
									<span class="inline-flex items-center px-2 py-1 rounded-full text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200">
										Enabled
									</span>
								{:else}
									<span class="inline-flex items-center px-2 py-1 rounded-full text-xs font-medium bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300">
										Disabled
									</span>
								{/if}
							</div>
						</div>

						<!-- Agent Releases URL -->
						{#if controllerInfo.garm_agent_releases_url}
							<div>
								<div class="flex items-center">
									<div class="text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wide font-medium">Agent Releases URL</div>
									<div class="ml-2">
										<Tooltip
											title="Agent Releases URL"
											content="URL from where GARM fetches garm-agent binaries. Must be compatible with the GitHub releases API format. Defaults to the official garm-agent releases repository."
										/>
									</div>
								</div>
								<div class="mt-1 p-2 bg-gray-50 dark:bg-gray-700 rounded text-sm font-mono text-gray-600 dark:text-gray-300 break-all min-h-[38px] flex items-center">
									{controllerInfo.garm_agent_releases_url}
								</div>
							</div>
						{/if}
					</div>
				</div>
			</div>

			<!-- Right column - URLs & Integration -->
			<div class="space-y-4">
				<div>
					<h4 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">Integration URLs</h4>
					<div class="space-y-3">
						<!-- Metadata URL -->
						{#if controllerInfo.metadata_url}
							<div>
								<div class="flex items-center">
									<div class="text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wide font-medium">Metadata</div>
									<div class="ml-2">
										<Tooltip
											title="Metadata URL"
											content="URL where runners retrieve setup information and metadata. Runners must be able to connect to this URL during their initialization process. Usually accessible at /api/v1/metadata endpoint."
										/>
									</div>
								</div>
								<div class="mt-1 p-2 bg-gray-50 dark:bg-gray-700 rounded text-sm font-mono text-gray-600 dark:text-gray-300 break-all min-h-[38px] flex items-center">
									{controllerInfo.metadata_url}
								</div>
							</div>
						{/if}

						<!-- Callback URL -->
						{#if controllerInfo.callback_url}
							<div>
								<div class="flex items-center">
									<div class="text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wide font-medium">Callback</div>
									<div class="ml-2">
										<Tooltip
											title="Callback URL"
											content="URL where runners send status updates and system information (OS version, runner agent ID, etc.) to the controller. Runners must be able to connect to this URL. Usually accessible at /api/v1/callbacks endpoint."
										/>
									</div>
								</div>
								<div class="mt-1 p-2 bg-gray-50 dark:bg-gray-700 rounded text-sm font-mono text-gray-600 dark:text-gray-300 break-all min-h-[38px] flex items-center">
									{controllerInfo.callback_url}
								</div>
							</div>
						{/if}

						<!-- Webhook URL -->
						{#if controllerInfo.webhook_url}
							<div>
								<div class="flex items-center">
									<div class="text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wide font-medium">Webhook</div>
									<div class="ml-2">
										<Tooltip
											title="Webhook Base URL"
											content="Base URL for webhooks where GitHub sends job notifications. GARM needs to receive these webhooks to know when to create new runners for jobs. GitHub must be able to connect to this URL. Usually accessible at /webhooks endpoint."
										/>
									</div>
								</div>
								<div class="mt-1 p-2 bg-gray-50 dark:bg-gray-700 rounded text-sm font-mono text-gray-600 dark:text-gray-300 break-all min-h-[38px] flex items-center">
									{controllerInfo.webhook_url}
								</div>
							</div>
						{/if}

						<!-- Agent URL -->
						{#if controllerInfo.agent_url}
							<div>
								<div class="flex items-center">
									<div class="text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wide font-medium">Agent</div>
									<div class="ml-2">
										<Tooltip
											title="Agent URL"
											content="URL where GARM agents connect for communication. This URL must support websocket connections for real-time communication between the controller and agent instances. Usually accessible at /agent endpoint."
										/>
									</div>
								</div>
								<div class="mt-1 p-2 bg-gray-50 dark:bg-gray-700 rounded text-sm font-mono text-gray-600 dark:text-gray-300 break-all min-h-[38px] flex items-center">
									{controllerInfo.agent_url}
								</div>
							</div>
						{/if}

						<!-- If no URLs configured -->
						{#if !controllerInfo.metadata_url && !controllerInfo.callback_url && !controllerInfo.webhook_url && !controllerInfo.agent_url}
							<div class="text-center py-4">
								<svg class="mx-auto h-8 w-8 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"></path>
								</svg>
								<p class="mt-2 text-sm text-gray-500 dark:text-gray-400">No URLs configured</p>
								<button
									on:click={openSettingsModal}
									class="mt-1 text-xs text-blue-600 dark:text-blue-400 hover:text-blue-500 dark:hover:text-blue-300 font-medium cursor-pointer"
								>
									Configure now
								</button>
							</div>
						{/if}
					</div>
				</div>
			</div>
		</div>

		<!-- Controller webhook URL at the bottom if available -->
		{#if controllerInfo.controller_webhook_url}
			<div class="mt-6 pt-4 border-t border-gray-200 dark:border-gray-600">
				<div class="flex items-center mb-2">
					<div class="text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wide font-medium">Controller Webhook URL</div>
					<div class="ml-2">
						<Tooltip
							title="Controller Webhook URL"
							content="Unique webhook URL for this GARM controller. This is the preferred URL to use in GitHub webhook settings as it's controller-specific and allows multiple GARM controllers to work with the same repository. Automatically combines the webhook base URL with the controller ID."
						/>
					</div>
				</div>
				<div class="p-3 bg-blue-50 dark:bg-blue-900/20 rounded-md border border-blue-200 dark:border-blue-800">
					<div class="flex items-start space-x-3">
						<div class="flex-shrink-0 mt-0.5">
							<svg class="w-4 h-4 text-blue-600 dark:text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"></path>
							</svg>
						</div>
						<div class="min-w-0 flex-1">
							<code class="text-sm font-mono text-blue-800 dark:text-blue-300 break-all">
								{controllerInfo.controller_webhook_url}
							</code>
							<p class="mt-1 text-xs text-blue-700 dark:text-blue-400">
								Use this URL in your GitHub organization/repository webhook settings
							</p>
						</div>
					</div>
				</div>
			</div>
		{/if}
	</div>
</div>

<!-- Settings Modal -->
{#if showSettingsModal}
	<Modal on:close={closeSettingsModal}>
		<div class="max-w-2xl w-full p-6">
			<h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">Controller Settings</h3>
			
			<form on:submit|preventDefault={saveSettings} class="space-y-4">
				<!-- Metadata URL -->
				<div>
					<label for="metadataUrl" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						Metadata URL
					</label>
					<input
						id="metadataUrl"
						type="url"
						bind:value={metadataUrl}
						placeholder="https://garm.example.com/api/v1/metadata"
						class="block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white sm:text-sm"
						class:border-red-300={!isValidUrl(metadataUrl)}
					/>
					{#if !isValidUrl(metadataUrl)}
						<p class="mt-1 text-sm text-red-600">Please enter a valid URL</p>
					{/if}
					<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
						URL where runners can fetch metadata and setup information
					</p>
				</div>

				<!-- Callback URL -->
				<div>
					<label for="callbackUrl" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						Callback URL
					</label>
					<input
						id="callbackUrl"
						type="url"
						bind:value={callbackUrl}
						placeholder="https://garm.example.com/api/v1/callbacks"
						class="block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white sm:text-sm"
						class:border-red-300={!isValidUrl(callbackUrl)}
					/>
					{#if !isValidUrl(callbackUrl)}
						<p class="mt-1 text-sm text-red-600">Please enter a valid URL</p>
					{/if}
					<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
						URL where runners send status updates and lifecycle events
					</p>
				</div>

				<!-- Webhook URL -->
				<div>
					<label for="webhookUrl" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						Webhook Base URL
					</label>
					<input
						id="webhookUrl"
						type="url"
						bind:value={webhookUrl}
						placeholder="https://garm.example.com/webhooks"
						class="block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white sm:text-sm"
						class:border-red-300={!isValidUrl(webhookUrl)}
					/>
					{#if !isValidUrl(webhookUrl)}
						<p class="mt-1 text-sm text-red-600">Please enter a valid URL</p>
					{/if}
					<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
						URL where GitHub/Gitea will send webhook events for job notifications
					</p>
				</div>

				<!-- Agent URL -->
				<div>
					<label for="agentUrl" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						Agent URL
					</label>
					<input
						id="agentUrl"
						type="url"
						bind:value={agentUrl}
						placeholder="https://garm.example.com/agent"
						class="block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white sm:text-sm"
						class:border-red-300={!isValidUrl(agentUrl)}
					/>
					{#if !isValidUrl(agentUrl)}
						<p class="mt-1 text-sm text-red-600">Please enter a valid URL</p>
					{/if}
					<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
						URL where GARM agents connect. Must support websocket connections
					</p>
				</div>

				<!-- Minimum Job Age Backoff -->
				<div>
					<label for="minimumJobAgeBackoff" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						Minimum Job Age Backoff (seconds)
					</label>
					<input
						id="minimumJobAgeBackoff"
						type="number"
						min="0"
						bind:value={minimumJobAgeBackoff}
						placeholder="30"
						class="block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white sm:text-sm"
					/>
					<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
						Time to wait before spinning up a runner for a new job (0 = immediate)
					</p>
				</div>

				<!-- Agent Releases URL -->
				<div>
					<label for="garmAgentReleasesUrl" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
						Agent Releases URL
					</label>
					<input
						id="garmAgentReleasesUrl"
						type="url"
						bind:value={garmAgentReleasesUrl}
						placeholder="https://api.github.com/repos/cloudbase/garm-agent/releases"
						class="block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white sm:text-sm"
						class:border-red-300={!isValidUrl(garmAgentReleasesUrl)}
					/>
					{#if !isValidUrl(garmAgentReleasesUrl)}
						<p class="mt-1 text-sm text-red-600">Please enter a valid URL</p>
					{/if}
					<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
						URL where GARM fetches garm-agent binaries (must be compatible with GitHub releases API)
					</p>
				</div>

				<!-- Sync Agent Tools -->
				<div>
					<label class="flex items-center cursor-pointer">
						<input
							id="syncGarmAgentTools"
							type="checkbox"
							bind:checked={syncGarmAgentTools}
							class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded cursor-pointer"
						/>
						<span class="ml-2 text-sm font-medium text-gray-700 dark:text-gray-300">
							Enable Agent Tools Sync
						</span>
					</label>
					<p class="mt-1 text-xs text-gray-500 dark:text-gray-400 ml-6">
						Automatically synchronize garm-agent tools from the configured releases URL
					</p>
				</div>

				<!-- Form Actions -->
				<div class="flex justify-end space-x-3 pt-4">
					<button
						type="button"
						disabled={saving}
						on:click={closeSettingsModal}
						class="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 dark:bg-gray-700 dark:text-gray-300 dark:border-gray-600 dark:hover:bg-gray-600 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
					>
						Cancel
					</button>
					<button
						type="submit"
						disabled={!isFormValid || saving}
						class="px-4 py-2 text-sm font-medium text-white bg-blue-600 border border-transparent rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 dark:focus:ring-offset-gray-900 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer"
					>
						{saving ? 'Saving...' : 'Save Changes'}
					</button>
				</div>
			</form>
		</div>
	</Modal>
{/if}