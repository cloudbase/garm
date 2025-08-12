<script lang="ts">
	export let variant: 'success' | 'error' | 'warning' | 'info' | 'gray' | 'blue' | 'green' | 'red' | 'yellow' | 'secondary' = 'gray';
	export let size: 'sm' | 'md' = 'sm';
	export let text: string;
	export let ring: boolean = false;

	const variants = {
		success: 'bg-green-100 dark:bg-green-900 text-green-800 dark:text-green-200',
		error: 'bg-red-100 dark:bg-red-900 text-red-800 dark:text-red-200',
		warning: 'bg-yellow-100 dark:bg-yellow-900 text-yellow-800 dark:text-yellow-200',
		info: 'bg-blue-100 dark:bg-blue-900 text-blue-800 dark:text-blue-200',
		gray: 'bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-200',
		blue: 'bg-blue-100 dark:bg-blue-900 text-blue-800 dark:text-blue-200',
		green: 'bg-green-100 dark:bg-green-900 text-green-800 dark:text-green-200',
		red: 'bg-red-100 dark:bg-red-900 text-red-800 dark:text-red-200',
		yellow: 'bg-yellow-100 dark:bg-yellow-900 text-yellow-800 dark:text-yellow-200',
		secondary: 'bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-200'
	};

	const ringVariants = {
		success: 'ring-green-600/20 dark:ring-green-400/30',
		error: 'ring-red-600/20 dark:ring-red-400/30',
		warning: 'ring-yellow-600/20 dark:ring-yellow-400/30',
		info: 'ring-blue-600/20 dark:ring-blue-400/30',
		gray: 'ring-gray-500/20 dark:ring-gray-400/30',
		blue: 'ring-blue-600/20 dark:ring-blue-400/30',
		green: 'ring-green-600/20 dark:ring-green-400/30',
		red: 'ring-red-600/20 dark:ring-red-400/30',
		yellow: 'ring-yellow-600/20 dark:ring-yellow-400/30',
		secondary: 'ring-gray-500/20 dark:ring-gray-400/30'
	};

	const sizes = {
		sm: 'px-2 py-1 text-xs',
		md: 'px-2.5 py-0.5 text-xs'
	};

	$: classes = [
		'inline-flex items-center rounded-full font-semibold',
		variants[variant],
		sizes[size],
		ring ? `ring-1 ring-inset ${ringVariants[variant]}` : ''
	].filter(Boolean).join(' ');
</script>

<span class={classes}>
	{text}
</span>