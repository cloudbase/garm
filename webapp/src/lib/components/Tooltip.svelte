<script lang="ts">
	export let title: string;
	export let content: string;
	export let position: 'top' | 'bottom' | 'left' | 'right' = 'top';
	export let width: string = 'w-80';
</script>

<div class="relative group">
	<svg class="w-3 h-3 text-gray-400 cursor-help" fill="none" stroke="currentColor" viewBox="0 0 24 24">
		<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
	</svg>
	
	<!-- Tooltip content -->
	<div class="absolute {position === 'top' ? 'bottom-full' : position === 'bottom' ? 'top-full' : position === 'left' ? 'right-full top-1/2 -translate-y-1/2' : 'left-full top-1/2 -translate-y-1/2'} left-1/2 transform -translate-x-1/2 {position === 'top' ? 'mb-2' : position === 'bottom' ? 'mt-2' : 'mx-2'} {width} p-3 bg-gray-900 text-white text-xs rounded-lg shadow-lg opacity-0 invisible group-hover:opacity-100 group-hover:visible transition-all duration-200 z-50">
		<div class="font-semibold mb-1">{title}</div>
		<div class="text-gray-300">{content}</div>
		
		<!-- Arrow -->
		{#if position === 'top'}
			<div class="absolute top-full left-1/2 transform -translate-x-1/2 border-4 border-transparent border-t-gray-900"></div>
		{:else if position === 'bottom'}
			<div class="absolute bottom-full left-1/2 transform -translate-x-1/2 border-4 border-transparent border-b-gray-900"></div>
		{:else if position === 'left'}
			<div class="absolute right-full top-1/2 transform -translate-y-1/2 border-4 border-transparent border-l-gray-900"></div>
		{:else if position === 'right'}
			<div class="absolute left-full top-1/2 transform -translate-y-1/2 border-4 border-transparent border-r-gray-900"></div>
		{/if}
	</div>
</div>