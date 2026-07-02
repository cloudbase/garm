<script lang="ts">
	import { onMount, onDestroy, tick } from 'svelte';
	import { logStreamStore, filteredLogEntries, type LogFilter, type LogRecord } from '$lib/stores/log-stream.js';

	let logContainer: HTMLDivElement;
	let autoScroll = true;
	let expandedEntries = new Set<number>();
	let filterInput = '';
	let showFilters = false;

	const ROW_HEIGHT = 24;
	const EXPANDED_BASE = 24;
	const EXPANDED_ATTR_HEIGHT = 20;
	const OVERSCAN = 20;

	let scrollTop = 0;
	let containerHeight = 0;

	$: state = $logStreamStore;
	$: entries = $filteredLogEntries;
	$: entryCount = state.entries.length;
	$: filteredCount = entries.length;

	$: rowHeights = entries.map(e => {
		if (!expandedEntries.has(e.id)) return ROW_HEIGHT;
		return EXPANDED_BASE + Object.keys(e.attrs).length * EXPANDED_ATTR_HEIGHT + 8;
	});

	$: rowOffsets = (() => {
		const offsets = new Array(rowHeights.length);
		let cumulative = 0;
		for (let i = 0; i < rowHeights.length; i++) {
			offsets[i] = cumulative;
			cumulative += rowHeights[i];
		}
		return offsets;
	})();

	$: totalHeight = rowOffsets.length > 0
		? rowOffsets[rowOffsets.length - 1] + rowHeights[rowHeights.length - 1]
		: 0;

	$: visibleRange = (() => {
		if (entries.length === 0) return { start: 0, end: 0 };
		let start = 0;
		let end = entries.length;
		// Binary search for start
		let lo = 0, hi = entries.length - 1;
		while (lo <= hi) {
			const mid = (lo + hi) >>> 1;
			if (rowOffsets[mid] + rowHeights[mid] < scrollTop) lo = mid + 1;
			else hi = mid - 1;
		}
		start = Math.max(0, lo - OVERSCAN);
		// Binary search for end
		const bottom = scrollTop + containerHeight;
		lo = start; hi = entries.length - 1;
		while (lo <= hi) {
			const mid = (lo + hi) >>> 1;
			if (rowOffsets[mid] < bottom) lo = mid + 1;
			else hi = mid - 1;
		}
		end = Math.min(entries.length, lo + OVERSCAN);
		return { start, end };
	})();

	$: visibleEntries = entries.slice(visibleRange.start, visibleRange.end);
	$: topPad = visibleRange.start > 0 ? rowOffsets[visibleRange.start] : 0;
	$: bottomPad = visibleRange.end < entries.length
		? totalHeight - (rowOffsets[visibleRange.end - 1] + rowHeights[visibleRange.end - 1])
		: 0;

	onMount(() => {
		logStreamStore.connect();
	});

	onDestroy(() => {
		logStreamStore.disconnect();
	});

	let prevEntryCount = 0;
	$: if (entries.length !== prevEntryCount) {
		prevEntryCount = entries.length;
		if (autoScroll && logContainer) {
			tick().then(() => {
				if (logContainer && autoScroll) {
					logContainer.scrollTop = logContainer.scrollHeight;
				}
			});
		}
	}

	function handleScroll() {
		if (!logContainer) return;
		scrollTop = logContainer.scrollTop;
		containerHeight = logContainer.clientHeight;
		const { scrollHeight, clientHeight } = logContainer;
		autoScroll = scrollHeight - scrollTop - clientHeight < 40;
	}

	function scrollToBottom() {
		autoScroll = true;
		if (logContainer) {
			logContainer.scrollTop = logContainer.scrollHeight;
		}
	}

	function toggleEntry(id: number) {
		if (expandedEntries.has(id)) {
			expandedEntries.delete(id);
		} else {
			expandedEntries.add(id);
		}
		expandedEntries = expandedEntries;
	}

	function addFilter() {
		const trimmed = filterInput.trim();
		if (!trimmed) return;
		const eqIdx = trimmed.indexOf('=');
		if (eqIdx < 1) return;
		const key = trimmed.substring(0, eqIdx).trim();
		const value = trimmed.substring(eqIdx + 1).trim();
		if (!key || !value) return;
		logStreamStore.setFilters([...state.filters, { key, value }]);
		filterInput = '';
	}

	function removeFilter(idx: number) {
		const filters = [...state.filters];
		filters.splice(idx, 1);
		logStreamStore.setFilters(filters);
	}

	function addFilterFromAttr(key: string, value: unknown) {
		logStreamStore.setFilters([...state.filters, { key, value: String(value) }]);
	}

	function formatTime(timeStr: string): string {
		try {
			const d = new Date(timeStr);
			const hh = String(d.getHours()).padStart(2, '0');
			const mm = String(d.getMinutes()).padStart(2, '0');
			const ss = String(d.getSeconds()).padStart(2, '0');
			const ms = String(d.getMilliseconds()).padStart(3, '0');
			return `${hh}:${mm}:${ss}.${ms}`;
		} catch {
			return timeStr;
		}
	}

	function levelColor(level: string): string {
		switch (level) {
			case 'ERROR': return 'text-red-500 dark:text-red-400';
			case 'WARN': case 'WARNING': return 'text-yellow-500 dark:text-yellow-400';
			case 'INFO': return 'text-blue-500 dark:text-blue-400';
			case 'DEBUG': return 'text-purple-500 dark:text-purple-400';
			default: return 'text-gray-500 dark:text-gray-400';
		}
	}

	function levelBg(level: string): string {
		switch (level) {
			case 'ERROR': return 'bg-red-500/10';
			case 'WARN': case 'WARNING': return 'bg-yellow-500/5';
			default: return '';
		}
	}

	function highlightSearch(text: string, search: string): string {
		if (!search) return escapeHtml(text);
		const escaped = escapeHtml(text);
		const searchEscaped = escapeHtml(search).replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
		return escaped.replace(
			new RegExp(`(${searchEscaped})`, 'gi'),
			'<mark class="bg-yellow-300 dark:bg-yellow-600 text-gray-900 dark:text-white rounded px-0.5">$1</mark>'
		);
	}

	function escapeHtml(str: string): string {
		return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
	}

	const levels = ['DEBUG', 'INFO', 'WARN', 'ERROR'];
</script>

<svelte:head>
	<title>Logs - GARM</title>
</svelte:head>

<div class="flex flex-col h-[calc(100vh-2rem)] max-h-[calc(100vh-2rem)]">
	<!-- Header -->
	<div class="flex-shrink-0 mb-3">
		<div class="flex items-center justify-between">
			<div>
				<h1 class="text-2xl font-bold text-gray-900 dark:text-white">Logs</h1>
				<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
					Real-time GARM log stream
				</p>
			</div>
			<div class="flex items-center space-x-2">
				{#if state.connected}
					<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200">
						<div class="w-1.5 h-1.5 bg-green-500 rounded-full mr-1.5 animate-pulse"></div>
						Connected
					</span>
				{:else if state.connecting}
					<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200">
						<div class="w-1.5 h-1.5 bg-yellow-500 rounded-full mr-1.5 animate-pulse"></div>
						Connecting
					</span>
				{:else if state.error}
					<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200" title={state.error}>
						<div class="w-1.5 h-1.5 bg-red-500 rounded-full mr-1.5"></div>
						Disconnected
					</span>
				{:else}
					<span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200">
						<div class="w-1.5 h-1.5 bg-gray-400 rounded-full mr-1.5"></div>
						Idle
					</span>
				{/if}
			</div>
		</div>
	</div>

	<!-- Toolbar -->
	<div class="flex-shrink-0 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-t-lg p-3 space-y-3">
		<div class="flex flex-wrap items-center gap-3">
			<!-- Level filter buttons -->
			<div class="flex items-center space-x-1">
				<span class="text-xs font-medium text-gray-500 dark:text-gray-400 mr-1">Level:</span>
				{#each levels as level}
					<button
						on:click={() => logStreamStore.setMinLevel(level)}
						class="px-2 py-1 text-xs font-medium rounded transition-colors cursor-pointer
							{state.minLevel === level
								? 'bg-gray-900 text-white dark:bg-white dark:text-gray-900'
								: 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600'}"
					>
						{level}
					</button>
				{/each}
			</div>

			<!-- Search -->
			<div class="flex-1 min-w-[200px]">
				<div class="relative">
					<svg class="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
					</svg>
					<input
						type="text"
						placeholder="Search logs..."
						bind:value={state.search}
						on:input={(e) => logStreamStore.setSearch(e.currentTarget.value)}
						class="w-full pl-8 pr-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 focus:outline-none focus:ring-1 focus:ring-blue-500"
					/>
				</div>
			</div>

			<!-- Actions -->
			<div class="flex items-center space-x-1">
				<button
					on:click={() => showFilters = !showFilters}
					class="relative p-1.5 rounded text-gray-500 hover:text-gray-700 hover:bg-gray-100 dark:text-gray-400 dark:hover:text-gray-200 dark:hover:bg-gray-700 cursor-pointer transition-colors {showFilters ? 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-200' : ''}"
					title="Toggle filters"
				>
					<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z" />
					</svg>
					{#if state.filters.length > 0}
						<span class="absolute -top-1 -right-1 w-3.5 h-3.5 bg-blue-500 text-white text-[9px] font-bold rounded-full flex items-center justify-center">{state.filters.length}</span>
					{/if}
				</button>
				<button
					on:click={() => state.paused ? logStreamStore.resume() : logStreamStore.pause()}
					class="p-1.5 rounded text-gray-500 hover:text-gray-700 hover:bg-gray-100 dark:text-gray-400 dark:hover:text-gray-200 dark:hover:bg-gray-700 cursor-pointer transition-colors"
					title={state.paused ? 'Resume' : 'Pause'}
				>
					{#if state.paused}
						<svg class="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
							<path d="M8 5v14l11-7z" />
						</svg>
					{:else}
						<svg class="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
							<path d="M6 4h4v16H6V4zm8 0h4v16h-4V4z" />
						</svg>
					{/if}
				</button>
				<button
					on:click={() => logStreamStore.clear()}
					class="p-1.5 rounded text-gray-500 hover:text-gray-700 hover:bg-gray-100 dark:text-gray-400 dark:hover:text-gray-200 dark:hover:bg-gray-700 cursor-pointer transition-colors"
					title="Clear logs"
				>
					<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
					</svg>
				</button>
			</div>
		</div>

		<!-- Filters panel -->
		{#if showFilters}
			<div class="border-t border-gray-200 dark:border-gray-700 pt-3 space-y-2">
				<div class="flex items-center gap-2">
					<div class="flex-1">
						<input
							type="text"
							placeholder="Add filter (key=value, e.g. method=GET or msg=access)"
							bind:value={filterInput}
							on:keydown={(e) => { if (e.key === 'Enter') addFilter(); }}
							class="w-full px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 rounded bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-400 focus:outline-none focus:ring-1 focus:ring-blue-500"
						/>
					</div>
					<button
						on:click={addFilter}
						class="px-3 py-1.5 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded cursor-pointer transition-colors"
					>
						Add
					</button>
					<div class="flex items-center space-x-1 ml-2">
						<span class="text-xs text-gray-500 dark:text-gray-400">Match:</span>
						<button
							on:click={() => logStreamStore.setFilterMode('any')}
							class="px-2 py-0.5 text-xs font-medium rounded cursor-pointer transition-colors
								{state.filterMode === 'any' ? 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300' : 'text-gray-500 hover:text-gray-700 dark:text-gray-400'}"
						>
							Any
						</button>
						<button
							on:click={() => logStreamStore.setFilterMode('all')}
							class="px-2 py-0.5 text-xs font-medium rounded cursor-pointer transition-colors
								{state.filterMode === 'all' ? 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300' : 'text-gray-500 hover:text-gray-700 dark:text-gray-400'}"
						>
							All
						</button>
					</div>
				</div>
				{#if state.filters.length > 0}
					<div class="flex flex-wrap gap-1.5">
						{#each state.filters as filter, idx}
							<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200">
								<span class="font-semibold">{filter.key}</span>={filter.value}
								<button on:click={() => removeFilter(idx)} class="ml-1 hover:text-blue-600 dark:hover:text-blue-400 cursor-pointer">&times;</button>
							</span>
						{/each}
					</div>
				{/if}
			</div>
		{/if}
	</div>

	<!-- Log entries (virtualized) -->
	<div
		bind:this={logContainer}
		on:scroll={handleScroll}
		class="flex-1 overflow-y-auto bg-gray-950 border-x border-gray-200 dark:border-gray-700 font-mono text-[13px] leading-6 min-h-0"
	>
		{#if entries.length === 0}
			<div class="flex items-center justify-center h-full text-gray-500 dark:text-gray-400">
				{#if state.connected}
					<div class="text-center">
						<svg class="w-12 h-12 mx-auto mb-3 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
						</svg>
						<p class="text-sm">Waiting for log entries...</p>
						<p class="text-xs mt-1 text-gray-600">Log entries will appear here as they arrive</p>
					</div>
				{:else if state.error}
					<div class="text-center">
						<svg class="w-12 h-12 mx-auto mb-3 text-red-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
						</svg>
						<p class="text-sm text-red-400">{state.error}</p>
						<p class="text-xs mt-1 text-gray-600">Log streaming may be disabled in the GARM configuration</p>
					</div>
				{:else}
					<div class="text-center">
						<svg class="w-12 h-12 mx-auto mb-3 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M13 10V3L4 14h7v7l9-11h-7z" />
						</svg>
						<p class="text-sm">Connecting to log stream...</p>
					</div>
				{/if}
			</div>
		{:else}
			<!-- Top spacer for virtualization -->
			<div style="height:{topPad}px"></div>

			{#each visibleEntries as entry, i (entry.id)}
				{@const hasAttrs = Object.keys(entry.attrs).length > 0}
				{@const isExpanded = expandedEntries.has(entry.id)}
				<div class="group hover:bg-gray-900/50 {levelBg(entry.level)}" style="height:{rowHeights[visibleRange.start + i]}px;overflow:hidden">
					<!-- svelte-ignore a11y-click-events-have-key-events -->
					<!-- svelte-ignore a11y-no-static-element-interactions -->
					<div
						class="flex items-start px-3 h-6 {hasAttrs ? 'cursor-pointer' : ''}"
						on:click={() => hasAttrs && toggleEntry(entry.id)}
					>
						<span class="w-4 flex-shrink-0 text-gray-600 text-[10px] leading-6 select-none">
							{#if hasAttrs}
								{isExpanded ? '▼' : '▶'}
							{/if}
						</span>
						<span class="text-gray-500 flex-shrink-0 mr-3 leading-6 select-all">{formatTime(entry.time)}</span>
						<span class="w-12 flex-shrink-0 font-semibold {levelColor(entry.level)} mr-3 leading-6">{entry.level.padEnd(5)}</span>
						<span class="text-gray-200 flex-shrink-0 leading-6">
							{@html highlightSearch(entry.msg, state.search)}
						</span>
						{#if hasAttrs && !isExpanded}
							<span class="ml-2 text-gray-600 truncate flex-1 min-w-0 hidden sm:inline leading-6">
								{Object.entries(entry.attrs).slice(0, 4).map(([k, v]) => `${k}=${v}`).join(' ')}
								{#if Object.keys(entry.attrs).length > 4}
									 +{Object.keys(entry.attrs).length - 4}
								{/if}
							</span>
						{/if}
					</div>
					{#if isExpanded && hasAttrs}
						<div class="pl-[4.5rem] pr-3 pb-1">
							{#each Object.entries(entry.attrs).sort(([a], [b]) => a.localeCompare(b)) as [key, value]}
								<div class="flex items-center h-5 text-[12px]">
									<button
										class="text-cyan-400 hover:text-cyan-300 cursor-pointer mr-1 flex-shrink-0 opacity-0 group-hover:opacity-100 transition-opacity"
										title="Filter by {key}={value}"
										on:click|stopPropagation={() => addFilterFromAttr(key, value)}
									>
										<svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
											<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z" />
										</svg>
									</button>
									<span class="text-cyan-500 mr-1">{@html highlightSearch(key, state.search)}</span>
									<span class="text-gray-500 mr-1">=</span>
									<span class="text-gray-300 truncate">
										{@html highlightSearch(typeof value === 'object' ? JSON.stringify(value) : String(value), state.search)}
									</span>
								</div>
							{/each}
						</div>
					{/if}
				</div>
			{/each}

			<!-- Bottom spacer for virtualization -->
			<div style="height:{bottomPad}px"></div>
		{/if}
	</div>

	<!-- Footer bar -->
	<div class="flex-shrink-0 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-b-lg px-3 py-2">
		<div class="flex items-center justify-between text-xs text-gray-500 dark:text-gray-400">
			<div class="flex items-center space-x-4">
				<span>
					{filteredCount}
					{#if filteredCount !== entryCount}
						/ {entryCount}
					{/if}
					entries
				</span>
				{#if state.paused}
					<span class="text-yellow-600 dark:text-yellow-400 font-medium">Paused</span>
				{/if}
			</div>
			<div class="flex items-center space-x-4">
				<div class="flex items-center space-x-2">
					<label for="buffer-slider" class="whitespace-nowrap">Buffer:</label>
					<input
						id="buffer-slider"
						type="range"
						min="1000"
						max="5000"
						step="500"
						value={state.bufferSize}
						on:input={(e) => logStreamStore.setBufferSize(parseInt(e.currentTarget.value))}
						class="w-24 h-1 bg-gray-300 dark:bg-gray-600 rounded-lg appearance-none cursor-pointer"
					/>
					<span class="w-10 text-right tabular-nums">{state.bufferSize}</span>
				</div>

				{#if !autoScroll}
					<button
						on:click={scrollToBottom}
						class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-100 text-blue-700 hover:bg-blue-200 dark:bg-blue-900 dark:text-blue-300 dark:hover:bg-blue-800 cursor-pointer transition-colors"
					>
						<svg class="w-3 h-3 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 14l-7 7m0 0l-7-7m7 7V3" />
						</svg>
						Follow
					</button>
				{/if}
			</div>
		</div>
	</div>
</div>
