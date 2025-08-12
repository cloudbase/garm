<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { base } from '$app/paths';
	import { auth, authStore } from '$lib/stores/auth.js';
	import Navigation from '$lib/components/Navigation.svelte';
	import Toast from '$lib/components/Toast.svelte';
	
	onMount(() => {
		auth.init();
		
		// Check for redirect after auth state settles
		setTimeout(() => {
			const isLoginPage = $page.url.pathname === `${base}/login`;
			const isInitPage = $page.url.pathname === `${base}/init`;
			
			if (!isLoginPage && !isInitPage && !$authStore.isAuthenticated && !$authStore.loading) {
				if ($authStore.needsInitialization) {
					goto(`${base}/init`);
				} else {
					goto(`${base}/login`);
				}
			}
		}, 200);
	});

	// Reactive redirect logic
	$: {
		if (!$authStore.loading) {
			const isLoginPage = $page.url.pathname === `${base}/login`;
			const isInitPage = $page.url.pathname === `${base}/init`;
			
			if (!isLoginPage && !isInitPage && !$authStore.isAuthenticated) {
				if ($authStore.needsInitialization) {
					goto(`${base}/init`);
				} else {
					goto(`${base}/login`);
				}
			}
		}
	}

	$: isLoginPage = $page.url.pathname === `${base}/login`;
	$: isInitPage = $page.url.pathname === `${base}/init`;
	$: requiresAuth = !isLoginPage && !isInitPage;
</script>

<svelte:head>
	<title>GARM - GitHub Actions Runner Manager</title>
</svelte:head>

{#if $authStore.loading}
	<div class="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-gray-900">
		<div class="text-center">
			<div class="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mx-auto"></div>
			<p class="mt-4 text-gray-500 dark:text-gray-400">Loading...</p>
		</div>
	</div>
{:else if requiresAuth && !$authStore.isAuthenticated}
	<!-- Redirect to login handled by load function -->
	<div class="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-gray-900">
		<div class="text-center">
			<p class="text-gray-600 dark:text-gray-400">Redirecting to login...</p>
		</div>
	</div>
{:else if isLoginPage || isInitPage}
	<!-- Login/Init page - no navigation -->
	<slot />
{:else}
	<!-- Main app layout with sidebar -->
	<div class="min-h-screen bg-gray-100 dark:bg-gray-900">
		<Navigation />
		<!-- Main content -->
		<div class="lg:pl-64">
			<main class="py-6 pt-20 lg:pt-6">
				<div class="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
					<slot />
				</div>
			</main>
		</div>
	</div>
{/if}

<!-- Toast notifications (rendered globally) -->
<Toast />
