<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { base } from '$app/paths';
	import { auth, authStore } from '$lib/stores/auth.js';
	import { toastStore } from '$lib/stores/toast.js';
	import { extractAPIError } from '$lib/utils/apiError';
	import Button from '$lib/components/Button.svelte';

	let username = 'admin';
	let email = 'admin@garm.local';
	let password = '';
	let confirmPassword = '';
	let fullName = 'Administrator';
	let loading = false;
	let error = '';
	
	// Advanced configuration
	let showAdvanced = false;
	let callbackUrl = '';
	let metadataUrl = '';
	let webhookUrl = '';
	
	// Auto-populate URLs with current origin
	$: if (typeof window !== 'undefined') {
		const currentUrl = window.location.origin;
		if (!callbackUrl) callbackUrl = `${currentUrl}/api/v1/callbacks`;
		if (!metadataUrl) metadataUrl = `${currentUrl}/api/v1/metadata`;
		if (!webhookUrl) webhookUrl = `${currentUrl}/webhooks`;
	}

	// Form validation - all mandatory fields must be filled
	$: isValidEmail = email.trim() !== '' && email.includes('@');
	$: isValidPassword = password.length >= 8;
	$: isValidConfirmPassword = confirmPassword.length > 0 && password === confirmPassword;
	$: isValidUsername = username.trim() !== '';
	$: isValidFullName = fullName.trim() !== '';
	
	$: isValid = isValidUsername && 
		isValidEmail && 
		isValidFullName &&
		isValidPassword && 
		isValidConfirmPassword;

	async function handleSubmit() {
		if (!isValid) return;

		try {
			loading = true;
			error = '';
			
			await auth.initialize(
				username.trim(), 
				email.trim(), 
				password, 
				fullName.trim(), 
				{
					callbackUrl: callbackUrl.trim() || undefined,
					metadataUrl: metadataUrl.trim() || undefined,
					webhookUrl: webhookUrl.trim() || undefined
				}
			);
			
			toastStore.success(
				'GARM Initialized',
				'GARM has been successfully initialized. Welcome!'
			);
			
			// Redirect to dashboard
			goto(`${base}/`);
		} catch (err) {
			error = extractAPIError(err);
		} finally {
			loading = false;
		}
	}

	// Check if we should be on this page
	onMount(() => {
		// If already authenticated, redirect to dashboard
		if ($authStore.isAuthenticated) {
			goto(`${base}/`);
			return;
		}
		
		// If doesn't need initialization, redirect to login
		if (!$authStore.needsInitialization && !$authStore.loading) {
			goto(`${base}/login`);
		}
	});

	// Redirect if auth state changes
	$: {
		if ($authStore.isAuthenticated) {
			goto(`${base}/`);
		} else if (!$authStore.needsInitialization && !$authStore.loading) {
			goto(`${base}/login`);
		}
	}
</script>

<svelte:head>
	<title>Initialize GARM - First Run Setup</title>
</svelte:head>

<div class="min-h-screen bg-gray-50 dark:bg-gray-900 flex flex-col justify-center py-12 sm:px-6 lg:px-8">
	<div class="sm:mx-auto sm:w-full sm:max-w-md">
		<div class="flex justify-center">
			<img 
				src="{base}/assets/garm-light.svg" 
				alt="GARM" 
				class="h-16 w-auto dark:hidden"
			/>
			<img 
				src="{base}/assets/garm-dark.svg" 
				alt="GARM" 
				class="h-16 w-auto hidden dark:block"
			/>
		</div>
		<h1 class="mt-6 text-center text-3xl font-extrabold text-gray-900 dark:text-white">
			Welcome to GARM
		</h1>
		<p class="mt-2 text-center text-sm text-gray-600 dark:text-gray-400">
			Complete the first-run setup to get started
		</p>
	</div>

	<div class="mt-8 sm:mx-auto sm:w-full sm:max-w-md">
		<!-- Initialization info -->
		<div class="bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-md p-4 mb-6">
			<div class="flex">
				<div class="flex-shrink-0">
					<svg class="h-5 w-5 text-blue-400" fill="currentColor" viewBox="0 0 20 20">
						<path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z" clip-rule="evenodd"></path>
					</svg>
				</div>
				<div class="ml-3">
					<h3 class="text-sm font-medium text-blue-800 dark:text-blue-200">
						First-Run Initialization
					</h3>
					<div class="mt-2 text-sm text-blue-700 dark:text-blue-300">
						<p>GARM needs to be initialized before first use. This will create the admin user and generate a unique controller ID for this installation.</p>
					</div>
				</div>
			</div>
		</div>

		<!-- Form Card -->
		<div class="bg-white dark:bg-gray-800 py-8 px-4 shadow sm:rounded-lg sm:px-10">
			<form class="space-y-6" on:submit|preventDefault={handleSubmit}>
				<!-- Username -->
				<div>
					<label for="username" class="block text-sm font-medium text-gray-700 dark:text-gray-300">
						Username
					</label>
					<div class="mt-1">
						<input
							id="username"
							name="username"
							type="text"
							required
							bind:value={username}
							class="appearance-none block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white sm:text-sm {!isValidUsername && username.length > 0 ? 'border-red-300 dark:border-red-600' : ''}"
							placeholder="Enter admin username"
						/>
						{#if !isValidUsername && username.length > 0}
							<p class="mt-2 text-sm text-red-600 dark:text-red-400">
								Username is required
							</p>
						{/if}
					</div>
				</div>

				<!-- Email -->
				<div>
					<label for="email" class="block text-sm font-medium text-gray-700 dark:text-gray-300">
						Email Address
					</label>
					<div class="mt-1">
						<input
							id="email"
							name="email"
							type="email"
							required
							bind:value={email}
							class="appearance-none block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white sm:text-sm {!isValidEmail && email.length > 0 ? 'border-red-300 dark:border-red-600' : ''}"
							placeholder="admin@example.com"
						/>
						{#if !isValidEmail && email.length > 0}
							<p class="mt-2 text-sm text-red-600 dark:text-red-400">
								Please enter a valid email address
							</p>
						{/if}
					</div>
				</div>

				<!-- Full Name -->
				<div>
					<label for="fullName" class="block text-sm font-medium text-gray-700 dark:text-gray-300">
						Full Name
					</label>
					<div class="mt-1">
						<input
							id="fullName"
							name="fullName"
							type="text"
							required
							bind:value={fullName}
							class="appearance-none block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white sm:text-sm {!isValidFullName && fullName.length > 0 ? 'border-red-300 dark:border-red-600' : ''}"
							placeholder="Enter full name"
						/>
						{#if !isValidFullName && fullName.length > 0}
							<p class="mt-2 text-sm text-red-600 dark:text-red-400">
								Full name is required
							</p>
						{/if}
					</div>
				</div>

				<!-- Password -->
				<div>
					<label for="password" class="block text-sm font-medium text-gray-700 dark:text-gray-300">
						Password
					</label>
					<div class="mt-1">
						<input
							id="password"
							name="password"
							type="password"
							required
							bind:value={password}
							class="appearance-none block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white sm:text-sm {!isValidPassword && password.length > 0 ? 'border-red-300 dark:border-red-600' : ''}"
							placeholder="Choose a strong password"
						/>
						{#if !isValidPassword && password.length > 0}
							<p class="mt-2 text-sm text-red-600 dark:text-red-400">
								Password must be at least 8 characters long
							</p>
						{/if}
					</div>
				</div>

				<!-- Confirm Password -->
				<div>
					<label for="confirmPassword" class="block text-sm font-medium text-gray-700 dark:text-gray-300">
						Confirm Password
					</label>
					<div class="mt-1">
						<input
							id="confirmPassword"
							name="confirmPassword"
							type="password"
							required
							bind:value={confirmPassword}
							class="appearance-none block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white sm:text-sm {!isValidConfirmPassword && confirmPassword.length > 0 ? 'border-red-300 dark:border-red-600' : ''}"
							placeholder="Confirm your password"
						/>
						{#if !isValidConfirmPassword && confirmPassword.length > 0}
							<p class="mt-2 text-sm text-red-600 dark:text-red-400">
								Passwords do not match
							</p>
						{/if}
					</div>
				</div>

				<!-- Advanced Configuration -->
				<div class="pt-4">
					<Button
						type="button"
						variant="ghost"
						size="sm"
						on:click={() => showAdvanced = !showAdvanced}
					>
						<svg 
							class="w-4 h-4 mr-2 transition-transform {showAdvanced ? 'rotate-90' : ''}"
							fill="none" 
							stroke="currentColor" 
							viewBox="0 0 24 24"
						>
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"></path>
						</svg>
						Advanced Configuration (Optional)
					</Button>

					{#if showAdvanced}
						<div class="mt-4 p-4 bg-gray-50 dark:bg-gray-800 rounded-md border border-gray-200 dark:border-gray-600">
							<div class="space-y-4">
								<!-- Metadata URL -->
								<div>
									<label for="metadataUrl" class="block text-sm font-medium text-gray-700 dark:text-gray-300">
										Metadata URL
									</label>
									<div class="mt-1">
										<input
											id="metadataUrl"
											name="metadataUrl"
											type="url"
											bind:value={metadataUrl}
											class="appearance-none block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white sm:text-sm"
											placeholder="https://garm.example.com/api/v1/metadata"
										/>
										<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
											URL where runners can fetch metadata and setup information.
										</p>
									</div>
								</div>

								<!-- Callback URL -->
								<div>
									<label for="callbackUrl" class="block text-sm font-medium text-gray-700 dark:text-gray-300">
										Callback URL
									</label>
									<div class="mt-1">
										<input
											id="callbackUrl"
											name="callbackUrl"
											type="url"
											bind:value={callbackUrl}
											class="appearance-none block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white sm:text-sm"
											placeholder="https://garm.example.com/api/v1/callbacks"
										/>
										<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
											URL where runners send status updates and lifecycle events.
										</p>
									</div>
								</div>

								<!-- Webhook URL -->
								<div>
									<label for="webhookUrl" class="block text-sm font-medium text-gray-700 dark:text-gray-300">
										Webhook URL
									</label>
									<div class="mt-1">
										<input
											id="webhookUrl"
											name="webhookUrl"
											type="url"
											bind:value={webhookUrl}
											class="appearance-none block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:ring-blue-500 focus:border-blue-500 dark:bg-gray-700 dark:text-white sm:text-sm"
											placeholder="https://garm.example.com/webhooks"
										/>
										<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
											URL where GitHub/Gitea will send webhook events for job notifications.
										</p>
									</div>
								</div>
							</div>
						</div>
					{/if}
				</div>

				<!-- Validation Summary -->
				{#if !isValid && (username.length > 0 || email.length > 0 || fullName.length > 0 || password.length > 0 || confirmPassword.length > 0)}
					<div class="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-md p-4">
						<div class="flex">
							<div class="flex-shrink-0">
								<svg class="h-5 w-5 text-yellow-400" fill="currentColor" viewBox="0 0 20 20">
									<path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd"></path>
								</svg>
							</div>
							<div class="ml-3">
								<h3 class="text-sm font-medium text-yellow-800 dark:text-yellow-200">
									Please complete all required fields
								</h3>
								<div class="mt-2 text-sm text-yellow-700 dark:text-yellow-300">
									<ul class="list-disc list-inside space-y-1">
										{#if !isValidUsername}
											<li>Enter a username</li>
										{/if}
										{#if !isValidEmail}
											<li>Enter a valid email address</li>
										{/if}
										{#if !isValidFullName}
											<li>Enter your full name</li>
										{/if}
										{#if !isValidPassword}
											<li>Enter a password with at least 8 characters</li>
										{/if}
										{#if !isValidConfirmPassword}
											<li>Confirm your password</li>
										{/if}
									</ul>
								</div>
							</div>
						</div>
					</div>
				{/if}

				<!-- Error Message -->
				{#if error}
					<div class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-md p-4">
						<div class="flex">
							<div class="flex-shrink-0">
								<svg class="h-5 w-5 text-red-400" fill="currentColor" viewBox="0 0 20 20">
									<path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd"></path>
								</svg>
							</div>
							<div class="ml-3">
								<p class="text-sm text-red-800 dark:text-red-200">{error}</p>
							</div>
						</div>
					</div>
				{/if}

				<!-- Submit Button -->
				<div>
					<Button
						type="submit"
						variant="primary"
						size="lg"
						fullWidth={true}
						{loading}
						disabled={!isValid || loading}
					>
						{loading ? 'Initializing...' : 'Initialize GARM'}
					</Button>
				</div>
			</form>

			<!-- Help text -->
			<div class="mt-6">
				<div class="text-center">
					<p class="text-xs text-gray-500 dark:text-gray-400">
						This will create the admin user, generate a unique controller ID, and configure the required URLs for your GARM installation.
						<br />
						Make sure to remember these credentials as they cannot be recovered.
					</p>
				</div>
			</div>
		</div>
	</div>
</div>