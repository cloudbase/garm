<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { auth, authStore } from '$lib/stores/auth.js';
	import Button from '$lib/components/Button.svelte';
	import { extractAPIError } from '$lib/utils/apiError';

	let username = '';
	let password = '';
	let loading = false;
	let error = '';

	// Initialize theme on mount
	onMount(() => {
		initializeTheme();
	});

	function initializeTheme() {
		const savedTheme = localStorage.getItem('theme');
		let isDark = false;
		
		if (savedTheme === 'dark') {
			isDark = true;
		} else if (savedTheme === 'light') {
			isDark = false;
		} else {
			// No saved preference or 'system' - use system preference
			isDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
		}
		
		if (isDark) {
			document.documentElement.classList.add('dark');
		} else {
			document.documentElement.classList.remove('dark');
		}
	}

	// Redirect if already authenticated
	$: if ($authStore.isAuthenticated) {
		goto(resolve('/'));
	}

	async function handleLogin() {
		if (!username || !password) {
			error = 'Please enter both username and password';
			return;
		}

		loading = true;
		error = '';

		try {
			await auth.login(username, password);
			goto(resolve('/'));
		} catch (err) {
			error = extractAPIError(err);
		} finally {
			loading = false;
		}
	}

	function handleKeyPress(event: KeyboardEvent) {
		if (event.key === 'Enter') {
			handleLogin();
		}
	}
</script>

<svelte:head>
	<title>Login - GARM</title>
</svelte:head>

<div class="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-gray-900 py-12 px-4 sm:px-6 lg:px-8">
	<div class="max-w-md w-full space-y-8">
		<div>
			<div class="mx-auto h-48 w-auto flex justify-center">
				<img 
					src={resolve('/assets/garm-light.svg')} 
					alt="GARM" 
					class="h-48 w-auto dark:hidden"
				/>
				<img 
					src={resolve('/assets/garm-dark.svg')} 
					alt="GARM" 
					class="h-48 w-auto hidden dark:block"
				/>
			</div>
			<h2 class="mt-6 text-center text-3xl font-extrabold text-gray-900 dark:text-white">
				Sign in to GARM
			</h2>
			<p class="mt-2 text-center text-sm text-gray-600 dark:text-gray-400">
				GitHub Actions Runner Manager
			</p>
		</div>

		<form class="mt-8 space-y-6" on:submit|preventDefault={handleLogin}>
			<div class="rounded-md shadow-sm -space-y-px">
				<div>
					<label for="username" class="sr-only">Username</label>
					<input
						id="username"
						name="username"
						type="text"
						required
						bind:value={username}
						on:keypress={handleKeyPress}
						class="appearance-none rounded-none relative block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 placeholder-gray-500 dark:placeholder-gray-400 text-gray-900 dark:text-white bg-white dark:bg-gray-700 rounded-t-md focus:outline-none focus:ring-blue-500 focus:border-blue-500 focus:z-10 sm:text-sm"
						placeholder="Username"
						disabled={loading}
					/>
				</div>
				<div>
					<label for="password" class="sr-only">Password</label>
					<input
						id="password"
						name="password"
						type="password"
						required
						bind:value={password}
						on:keypress={handleKeyPress}
						class="appearance-none rounded-none relative block w-full px-3 py-2 border border-gray-300 dark:border-gray-600 placeholder-gray-500 dark:placeholder-gray-400 text-gray-900 dark:text-white bg-white dark:bg-gray-700 rounded-b-md focus:outline-none focus:ring-blue-500 focus:border-blue-500 focus:z-10 sm:text-sm"
						placeholder="Password"
						disabled={loading}
					/>
				</div>
			</div>

			{#if error}
				<div class="rounded-md bg-red-50 dark:bg-red-900 p-4">
					<div class="flex">
						<div class="flex-shrink-0">
							<svg class="h-5 w-5 text-red-400" viewBox="0 0 20 20" fill="currentColor">
								<path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
							</svg>
						</div>
						<div class="ml-3">
							<p class="text-sm font-medium text-red-800 dark:text-red-200">
								{error}
							</p>
						</div>
					</div>
				</div>
			{/if}

			<div>
				<Button
					type="submit"
					variant="primary"
					size="md"
					fullWidth
					disabled={loading}
					loading={loading}
				>
					{loading ? 'Signing in...' : 'Sign in'}
				</Button>
			</div>
		</form>
	</div>
</div>