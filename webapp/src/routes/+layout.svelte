<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { auth, authStore } from '$lib/stores/auth.js';
	import { themeStore } from '$lib/stores/theme.js';
	import Navigation from '$lib/components/Navigation.svelte';
	import Toast from '$lib/components/Toast.svelte';
	
	onMount(() => {
		auth.init();
		themeStore.init();
	});

	// Reactive redirect logic - handles all redirects
	$: {
		if (!$authStore.loading) {
			const isLoginPage = page.url.pathname === resolve('/login');
			const isInitPage = page.url.pathname === resolve('/init');
			
			if (!isLoginPage && !isInitPage && !$authStore.isAuthenticated) {
				if ($authStore.needsInitialization) {
					goto(resolve('/init'), { replaceState: true });
				} else {
					goto(resolve('/login'), { replaceState: true });
				}
			}
		}
	}

	$: isLoginPage = page.url.pathname === resolve('/login');
	$: isInitPage = page.url.pathname === resolve('/init');
	$: shouldShowInitPage = $authStore.needsInitialization && !$authStore.isAuthenticated && !$authStore.loading;
	$: shouldShowLoginPage = !$authStore.needsInitialization && !$authStore.isAuthenticated && !$authStore.loading;
	$: shouldShowMainLayout = $authStore.isAuthenticated && !$authStore.loading;
	$: shouldShowRedirect = !shouldShowInitPage && !shouldShowLoginPage && !shouldShowMainLayout;
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
{:else if shouldShowLoginPage || shouldShowInitPage}
	<!-- Login/Init page - no navigation -->
	<slot />
{:else if shouldShowMainLayout}
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
{:else if shouldShowRedirect}
	<!-- Redirect to login/init handled by reactive logic -->
	<div class="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-gray-900">
		<div class="text-center">
			<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600 mx-auto mb-4"></div>
			<p class="text-gray-600 dark:text-gray-400">
				{#if $authStore.needsInitialization}
					Redirecting to initialization...
				{:else}
					Redirecting to login...
				{/if}
			</p>
		</div>
	</div>
{/if}

<!-- Toast notifications (rendered globally) -->
<Toast />
