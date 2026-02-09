<script lang="ts">
	import { page } from '$app/stores';
	import { resolve } from '$app/paths';
	import { auth, authStore } from '$lib/stores/auth.js';
	import { websocketStore } from '$lib/stores/websocket.js';
	import { themeStore } from '$lib/stores/theme.js';
	import { eagerCache } from '$lib/stores/eager-cache.js';
	import { onMount } from 'svelte';

	let mobileMenuOpen = false;
	let userMenuOpen = false;

	// WebSocket connection status
	$: wsState = $websocketStore;
	$: darkMode = $themeStore;
	$: serverVersion = $eagerCache.controllerInfo?.version || '';

	// Close mobile menu when route changes  
	$: $page.url.pathname && (mobileMenuOpen = false);

	onMount(() => {
		// Listen for system theme changes - theme store handles initialization
		const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
		mediaQuery.addEventListener('change', handleSystemThemeChange);
	});

	function handleSystemThemeChange(e: MediaQueryListEvent) {
		// Only update if user hasn't set a manual preference
		if (!localStorage.getItem('theme') || localStorage.getItem('theme') === 'system') {
			themeStore.set(e.matches);
		}
	}

	function toggleDarkMode() {
		themeStore.toggle();
	}

	function handleLogout() {
		auth.logout();
		userMenuOpen = false;
	}

	// Navigation items with pools and scale sets grouped together, instances after scale sets
	const mainNavItems = [
		{
			href: resolve('/'),
			label: 'Dashboard',
			icon: 'M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6' // Home icon
		},
		{
			href: resolve('/repositories'),
			label: 'Repositories',
			icon: 'M7 16V4m0 0a2 2 0 100-4 2 2 0 000 4zm0 0a2 2 0 100 4 2 2 0 000-4zm10 12a2 2 0 100-4 2 2 0 000 4zm0 0V9a5 5 0 00-5-5' // Git branch icon
		},
		{
			href: resolve('/organizations'),
			label: 'Organizations',
			icon: 'M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z' // Users/group icon
		},
		{
			href: resolve('/users'),
			label: 'Users',
			icon: 'M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z'
		},
		{
			href: resolve('/enterprises'),
			label: 'Enterprises',
			icon: 'M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4' // Building/office icon
		},
		{
			href: resolve('/pools'),
			label: 'Pools',
			icon: 'M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10' // Server/stack icon
		},
		{
			href: resolve('/scalesets'),
			label: 'Scale Sets',
			icon: 'M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4' // Database layers icon
		},
		{
			href: resolve('/instances'),
			label: 'Runners',
			icon: 'M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z' // CPU/chip icon
		}
	];

	const configNavItems = [
		{
			href: resolve('/credentials'),
			label: 'Credentials',
			icon: 'M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1721 9z'
		},
		{
			href: resolve('/endpoints'),
			label: 'Endpoints',
			icon: 'M13 10V3L4 14h7v7l9-11h-7z'
		},
		{
			href: resolve('/templates'),
			label: 'Runner Install Templates',
			icon: 'M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z'
		},
		{
			href: resolve('/objects'),
			label: 'Object Storage',
			icon: 'M5 8h14M5 8a2 2 0 110-4h14a2 2 0 110 4M5 8v10a2 2 0 002 2h10a2 2 0 002-2V8m-9 4h4'
		}
	];

	$: currentPath = $page.url.pathname;
</script>

<!-- Fixed sidebar for desktop -->
<div class="hidden lg:fixed lg:inset-y-0 lg:flex lg:w-64 lg:flex-col">
	<div class="flex min-h-0 flex-1 flex-col bg-white dark:bg-gray-800 border-r border-gray-200 dark:border-gray-700">
		<!-- Logo and Status Section -->
		<div class="flex-shrink-0 border-b border-gray-200 dark:border-gray-700">
			<!-- Logo Area - Generous padding and larger size -->
			<div class="px-6 py-3 bg-gradient-to-r from-gray-50 to-white dark:from-gray-800 dark:to-gray-700">
				<a href={resolve('/')} class="flex justify-center">
					<img 
						src={resolve('/assets/garm-light.svg' as any)} 
						alt="GARM" 
						class="h-24 w-auto dark:hidden transition-transform hover:scale-105"
					/>
					<img 
						src={resolve('/assets/garm-dark.svg' as any)} 
						alt="GARM" 
						class="h-24 w-auto hidden dark:block transition-transform hover:scale-105"
					/>
				</a>
			</div>

			<!-- Status and Controls Row -->
			<div class="px-4 py-3 bg-gray-50 dark:bg-gray-800/50 border-t border-gray-100 dark:border-gray-700">
				<div class="flex items-center justify-between">
					<!-- WebSocket Status -->
					<div class="flex items-center space-x-2">
						{#if wsState.connected}
							<div class="flex items-center text-green-600 dark:text-green-400">
								<div class="w-2.5 h-2.5 bg-green-500 rounded-full mr-2 animate-pulse shadow-sm"></div>
								<span class="text-xs font-medium">Live Updates</span>
							</div>
						{:else if wsState.connecting}
							<div class="flex items-center text-yellow-600 dark:text-yellow-400">
								<div class="w-2.5 h-2.5 bg-yellow-500 rounded-full mr-2 animate-pulse shadow-sm"></div>
								<span class="text-xs font-medium">Connecting</span>
							</div>
						{:else if wsState.error}
							<div class="flex items-center text-red-600 dark:text-red-400">
								<div class="w-2.5 h-2.5 bg-red-500 rounded-full mr-2 shadow-sm"></div>
								<span class="text-xs font-medium">Updates Unavailable</span>
							</div>
						{:else}
							<div class="flex items-center text-gray-500 dark:text-gray-400">
								<div class="w-2.5 h-2.5 bg-gray-400 rounded-full mr-2 shadow-sm"></div>
								<span class="text-xs font-medium">Manual Refresh</span>
							</div>
						{/if}
					</div>

					<!-- Theme Toggle -->
					<button
						on:click={toggleDarkMode}
						class="p-2 rounded-lg bg-white dark:bg-gray-700 shadow-sm border border-gray-200 dark:border-gray-600 text-gray-600 hover:bg-gray-50 hover:text-gray-900 dark:text-gray-300 dark:hover:bg-gray-600 dark:hover:text-white transition-all duration-200 hover:shadow-md cursor-pointer"
						title={darkMode ? 'Switch to Light Mode' : 'Switch to Dark Mode'}
					>
						{#if darkMode}
							<svg class="h-4 w-4 text-yellow-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z"></path>
							</svg>
						{:else}
							<svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z"></path>
							</svg>
						{/if}
					</button>
				</div>
			</div>
		</div>

		<!-- Navigation -->
		<nav class="mt-5 flex-1 px-2 space-y-1">
			<!-- Main navigation items -->
			{#each mainNavItems as item}
				<a
					href={item.href}
					class="group flex items-center px-2 py-2 text-sm font-medium rounded-md transition-colors duration-200
						{currentPath === item.href 
							? 'bg-gray-100 text-gray-900 dark:bg-gray-700 dark:text-white' 
							: 'text-gray-600 hover:bg-gray-50 hover:text-gray-900 dark:text-gray-300 dark:hover:bg-gray-700 dark:hover:text-white'}"
				>
					<svg class="mr-3 h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						{#if Array.isArray(item.icon)}
							{#each item.icon as pathData}
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={pathData}></path>
							{/each}
						{:else}
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={item.icon}></path>
						{/if}
					</svg>
					{item.label}
				</a>
			{/each}

			<!-- Configuration section -->
			<div class="border-t border-gray-200 dark:border-gray-600 mt-4 pt-4">
				{#each configNavItems as item}
					<a
						href={item.href}
						class="group flex items-center px-2 py-2 text-sm font-medium rounded-md transition-colors duration-200
							{currentPath === item.href 
								? 'bg-gray-100 text-gray-900 dark:bg-gray-700 dark:text-white' 
								: 'text-gray-600 hover:bg-gray-50 hover:text-gray-900 dark:text-gray-300 dark:hover:bg-gray-700 dark:hover:text-white'}"
					>
						<svg class="mr-3 h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={item.icon}></path>
						</svg>
						{item.label}
					</a>
				{/each}
			</div>


			<!-- Logout section -->
			<div class="border-t border-gray-200 dark:border-gray-600 mt-4 pt-4">
				<button
					on:click={handleLogout}
					class="group flex items-center px-2 py-2 text-sm font-medium rounded-md w-full text-left text-gray-600 hover:bg-gray-50 hover:text-gray-900 dark:text-gray-300 dark:hover:bg-gray-700 dark:hover:text-white cursor-pointer"
				>
					<svg class="mr-3 h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
					</svg>
					Logout
				</button>
			</div>

			<!-- Version section -->
			{#if serverVersion}
				<div class="border-t border-gray-200 dark:border-gray-600 mt-4 pt-4 px-2">
					<div class="text-xs text-gray-500 dark:text-gray-400">
						<span class="font-medium">GARM</span> {serverVersion}
					</div>
				</div>
			{/if}
		</nav>

	</div>
</div>

<!-- Mobile menu -->
<div class="lg:hidden">
	<div class="fixed top-0 left-0 right-0 z-40 flex items-center justify-between h-16 bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 px-4">
		<!-- Mobile menu button -->
		<button
			on:click={() => mobileMenuOpen = !mobileMenuOpen}
			aria-label="Open main menu"
			class="text-gray-500 hover:text-gray-600 dark:text-gray-400 dark:hover:text-gray-300 focus:outline-none focus:ring-2 focus:ring-inset focus:ring-indigo-500 cursor-pointer"
		>
			<svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
			</svg>
		</button>
		
		<!-- Mobile logo and status -->
		<div class="flex items-center space-x-3">
			<img 
				src={resolve('/assets/garm-light.svg' as any)} 
				alt="GARM" 
				class="{darkMode ? 'hidden' : 'block'} h-8 w-8"
			/>
			<img 
				src={resolve('/assets/garm-dark.svg' as any)} 
				alt="GARM" 
				class="{darkMode ? 'block' : 'hidden'} h-8 w-8"
			/>
			<h1 class="text-xl font-bold text-gray-900 dark:text-white">GARM</h1>
			
			<!-- Mobile WebSocket Status -->
			<div class="flex items-center ml-2">
				{#if wsState.connected}
					<div class="flex items-center">
						<div class="w-2 h-2 bg-green-500 rounded-full animate-pulse shadow-sm" title="Live updates enabled"></div>
					</div>
				{:else if wsState.connecting}
					<div class="flex items-center">
						<div class="w-2 h-2 bg-yellow-500 rounded-full animate-pulse shadow-sm" title="Connecting to live updates"></div>
					</div>
				{:else if wsState.error}
					<div class="flex items-center">
						<div class="w-2 h-2 bg-red-500 rounded-full shadow-sm" title="Live updates unavailable"></div>
					</div>
				{:else}
					<div class="flex items-center">
						<div class="w-2 h-2 bg-gray-400 rounded-full shadow-sm" title="Manual refresh mode"></div>
					</div>
				{/if}
			</div>
		</div>

		<!-- Mobile theme toggle -->
		<button
			on:click={toggleDarkMode}
			class="p-2 rounded-lg bg-white dark:bg-gray-800 shadow-sm border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors cursor-pointer"
			title="Toggle theme"
		>
			{#if darkMode}
				<svg class="w-5 h-5 text-yellow-400 hover:text-yellow-300" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z"></path>
				</svg>
			{:else}
				<svg class="w-5 h-5 text-black hover:text-gray-800" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z"></path>
				</svg>
			{/if}
		</button>
	</div>

	<!-- Mobile menu overlay -->
	{#if mobileMenuOpen}
		<div class="fixed inset-0 flex z-50 lg:hidden">
			<div class="fixed inset-0 bg-black/30" on:click={() => mobileMenuOpen = false} on:keydown={(e) => { if (e.key === 'Escape') mobileMenuOpen = false; }} role="button" tabindex="0" aria-label="Close menu"></div>
			<div class="relative flex-1 flex flex-col max-w-xs w-full bg-white dark:bg-gray-800">
				<div class="absolute top-0 right-0 -mr-12 pt-2">
					<button
						on:click={() => mobileMenuOpen = false}
						aria-label="Close menu"
						class="ml-1 flex items-center justify-center h-10 w-10 rounded-full focus:outline-none focus:ring-2 focus:ring-inset focus:ring-white cursor-pointer"
					>
						<svg class="h-6 w-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
						</svg>
					</button>
				</div>

				<div class="flex-1 h-0 pt-5 pb-4 overflow-y-auto">
					<nav class="px-2 space-y-1">
						<!-- Main navigation items -->
						{#each mainNavItems as item}
							<a
								href={item.href}
								on:click={() => mobileMenuOpen = false}
								class="group flex items-center px-2 py-2 text-base font-medium rounded-md transition-colors duration-200
									{currentPath === item.href 
										? 'bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white' 
										: 'text-gray-600 hover:bg-gray-50 hover:text-gray-900 dark:text-gray-300 dark:hover:bg-gray-700 dark:hover:text-white'}"
							>
								<svg class="mr-4 flex-shrink-0 h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									{#if Array.isArray(item.icon)}
										{#each item.icon as pathData}
											<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={pathData}></path>
										{/each}
									{:else}
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={item.icon}></path>
									{/if}
								</svg>
								{item.label}
							</a>
						{/each}

						<!-- Configuration section -->
						<div class="border-t border-gray-200 dark:border-gray-600 mt-4 pt-4">
							{#each configNavItems as item}
								<a
									href={item.href}
									on:click={() => mobileMenuOpen = false}
									class="group flex items-center px-2 py-2 text-base font-medium rounded-md transition-colors duration-200
										{currentPath === item.href 
											? 'bg-gray-100 dark:bg-gray-700 text-gray-900 dark:text-white' 
											: 'text-gray-600 hover:bg-gray-50 hover:text-gray-900 dark:text-gray-300 dark:hover:bg-gray-700 dark:hover:text-white'}"
								>
									<svg class="mr-4 flex-shrink-0 h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
										<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={item.icon}></path>
									</svg>
									{item.label}
								</a>
							{/each}
						</div>

						<!-- Logout section -->
						<div class="border-t border-gray-200 dark:border-gray-600 mt-4 pt-4">
							<button
								on:click={handleLogout}
								class="group flex items-center px-2 py-2 text-base font-medium rounded-md w-full text-left text-gray-600 hover:bg-gray-50 hover:text-gray-900 dark:text-gray-300 dark:hover:bg-gray-700 dark:hover:text-white cursor-pointer"
							>
								<svg class="mr-4 flex-shrink-0 h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
								</svg>
								Logout
							</button>
						</div>

						<!-- Version section -->
						{#if serverVersion}
							<div class="border-t border-gray-200 dark:border-gray-600 mt-4 pt-4 px-2">
								<div class="text-xs text-gray-500 dark:text-gray-400">
									<span class="font-medium">GARM</span> {serverVersion}
								</div>
							</div>
						{/if}
					</nav>
				</div>

			</div>
		</div>
	{/if}
</div>

<!-- Close user menu when clicking outside -->
{#if userMenuOpen}
	<div class="fixed inset-0 z-10" on:click={() => userMenuOpen = false} on:keydown={(e) => { if (e.key === 'Escape') userMenuOpen = false; }} role="button" tabindex="0" aria-label="Close user menu"></div>
{/if}
