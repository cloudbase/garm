<script lang="ts">
	import { createEventDispatcher } from 'svelte';

	const dispatch = createEventDispatcher<{
		click: void;
	}>();

	export let variant: 'primary' | 'secondary' | 'danger' | 'ghost' = 'primary';
	export let size: 'sm' | 'md' | 'lg' = 'md';
	export let disabled: boolean = false;
	export let loading: boolean = false;
	export let type: 'button' | 'submit' | 'reset' = 'button';
	export let fullWidth: boolean = false;
	export let icon: string | null = null;
	export let iconPosition: 'left' | 'right' = 'left';

	function handleClick() {
		if (!disabled && !loading) {
			dispatch('click');
		}
	}

	$: baseClasses = 'inline-flex items-center justify-center font-medium rounded-md transition-colors focus:outline-none focus:ring-2 focus:ring-offset-2 dark:focus:ring-offset-gray-900 cursor-pointer disabled:cursor-not-allowed';

	$: sizeClasses = {
		'sm': 'px-3 py-2 text-sm',
		'md': 'px-4 py-2 text-sm',
		'lg': 'px-6 py-3 text-base'
	}[size];

	$: variantClasses = {
		'primary': 'text-white bg-blue-600 hover:bg-blue-700 focus:ring-blue-500 disabled:bg-gray-400 disabled:hover:bg-gray-400',
		'secondary': 'text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-600 focus:ring-blue-500',
		'danger': 'text-white bg-red-600 hover:bg-red-700 focus:ring-red-500 disabled:bg-gray-400 disabled:hover:bg-gray-400',
		'ghost': 'text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-800 focus:ring-blue-500'
	}[variant];

	$: widthClasses = fullWidth ? 'w-full' : '';

	$: opacityClasses = disabled ? 'opacity-50' : '';

	$: allClasses = [baseClasses, sizeClasses, variantClasses, widthClasses, opacityClasses].filter(Boolean).join(' ');

	$: iconClasses = {
		'sm': 'h-4 w-4',
		'md': 'h-5 w-5', 
		'lg': 'h-6 w-6'
	}[size];

	$: iconSpacing = {
		'sm': iconPosition === 'left' ? '-ml-0.5 mr-2' : 'ml-2 -mr-0.5',
		'md': iconPosition === 'left' ? '-ml-1 mr-2' : 'ml-2 -mr-1',
		'lg': iconPosition === 'left' ? '-ml-1 mr-3' : 'ml-3 -mr-1'
	}[size];
</script>

<button
	{type}
	{disabled}
	class={allClasses}
	on:click={handleClick}
	{...$$restProps}
>
	{#if loading}
		<svg class="animate-spin {iconClasses} {iconPosition === 'left' ? '-ml-1 mr-2' : 'ml-2 -mr-1'}" fill="none" viewBox="0 0 24 24">
			<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
			<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
		</svg>
	{:else if icon && iconPosition === 'left'}
		<svg class="{iconClasses} {iconSpacing}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
			{@html icon}
		</svg>
	{/if}

	<slot />

	{#if icon && iconPosition === 'right' && !loading}
		<svg class="{iconClasses} {iconSpacing}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
			{@html icon}
		</svg>
	{/if}
</button>