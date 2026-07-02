import { writable, derived, get } from 'svelte/store';

export interface LogRecord {
	id: number;
	time: string;
	level: string;
	msg: string;
	attrs: Record<string, unknown>;
	raw: string;
}

export interface LogFilter {
	key: string;
	value: string;
}

export interface LogStreamState {
	connected: boolean;
	connecting: boolean;
	paused: boolean;
	error: string | null;
	entries: LogRecord[];
	bufferSize: number;
	minLevel: string;
	filters: LogFilter[];
	filterMode: 'any' | 'all';
	search: string;
}

const LEVEL_ORDER: Record<string, number> = {
	DEBUG: 0,
	INFO: 1,
	WARN: 2,
	WARNING: 2,
	ERROR: 3,
};

let nextId = 0;

function parseLogRecord(raw: string): LogRecord | null {
	try {
		const obj = JSON.parse(raw);
		const { time, level, msg, ...attrs } = obj;
		return {
			id: nextId++,
			time: time || '',
			level: (level || '').toUpperCase(),
			msg: msg || '',
			attrs,
			raw,
		};
	} catch {
		return null;
	}
}

const FLUSH_INTERVAL = 150;

function createLogStreamStore() {
	const { subscribe, set, update } = writable<LogStreamState>({
		connected: false,
		connecting: false,
		paused: false,
		error: null,
		entries: [],
		bufferSize: 1000,
		minLevel: 'DEBUG',
		filters: [],
		filterMode: 'any',
		search: '',
	});

	let ws: WebSocket | null = null;
	let reconnectTimeout: number | null = null;
	let reconnectAttempts = 0;
	let baseReconnectInterval = 1000;
	let reconnectInterval = 1000;
	let maxReconnectInterval = 30000;
	let manuallyDisconnected = false;

	let pendingRecords: LogRecord[] = [];
	let flushTimer: number | null = null;

	function flushPending() {
		flushTimer = null;
		if (pendingRecords.length === 0) return;

		const batch = pendingRecords;
		pendingRecords = [];

		update(s => {
			const entries = s.entries.concat(batch);
			const excess = entries.length - s.bufferSize;
			if (excess > 0) entries.splice(0, excess);
			return { ...s, entries };
		});
	}

	function enqueueRecord(record: LogRecord) {
		pendingRecords.push(record);
		if (flushTimer === null) {
			flushTimer = window.setTimeout(flushPending, FLUSH_INTERVAL);
		}
	}

	function getWebSocketUrl(): string {
		const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
		const host = window.location.host;
		return `${protocol}//${host}/api/v1/ws/logs`;
	}

	async function connect() {
		if (ws && (ws.readyState === WebSocket.CONNECTING || ws.readyState === WebSocket.OPEN)) {
			return;
		}

		manuallyDisconnected = false;
		update(s => ({ ...s, connecting: true, error: null }));

		try {
			// Probe the endpoint with a regular HTTP request first.
			// If log streaming is disabled, the server returns 400 before
			// the WebSocket upgrade — the browser WS API swallows that,
			// so we'd otherwise retry forever with no useful error message.
			try {
				const probeRes = await fetch(`/api/v1/ws/logs`, { method: 'GET' });
				if (probeRes.status === 400) {
					const body = await probeRes.text();
					const msg = body.includes('disabled')
						? 'Log streaming is disabled in the GARM configuration'
						: body || 'Log streaming is not available';
					update(s => ({ ...s, connecting: false, error: msg }));
					return;
				}
				if (probeRes.status === 403) {
					update(s => ({ ...s, connecting: false, error: 'Admin access required to view logs' }));
					return;
				}
			} catch {
				// Probe failed (network error) — fall through and try WS anyway
			}

			ws = new WebSocket(getWebSocketUrl());

			const connectionTimeout = setTimeout(() => {
				if (ws && ws.readyState === WebSocket.CONNECTING) {
					ws.close();
				}
			}, 10000);

			ws.onopen = () => {
				clearTimeout(connectionTimeout);
				reconnectAttempts = 0;
				reconnectInterval = baseReconnectInterval;
				update(s => ({ ...s, connected: true, connecting: false, error: null }));
			};

			ws.onmessage = (event) => {
				const state = get({ subscribe });
				if (state.paused) return;

				const record = parseLogRecord(event.data);
				if (!record) return;

				enqueueRecord(record);
			};

			ws.onclose = (event) => {
				clearTimeout(connectionTimeout);
				const wasManual = event.code === 1000 && manuallyDisconnected;
				update(s => ({
					...s,
					connected: false,
					connecting: false,
					error: event.code !== 1000 ? `Connection closed: ${event.reason || 'Unknown reason'}` : null,
				}));
				if (!wasManual) {
					scheduleReconnect();
				}
			};

			ws.onerror = () => {
				clearTimeout(connectionTimeout);
				update(s => ({
					...s,
					connected: false,
					connecting: false,
					error: 'WebSocket connection error',
				}));
			};
		} catch (err) {
			update(s => ({
				...s,
				connected: false,
				connecting: false,
				error: err instanceof Error ? err.message : 'Failed to connect',
			}));
		}
	}

	function scheduleReconnect() {
		if (manuallyDisconnected) return;
		if (reconnectTimeout) clearTimeout(reconnectTimeout);

		reconnectAttempts++;
		if (reconnectAttempts > 50) {
			reconnectAttempts = 1;
			reconnectInterval = baseReconnectInterval;
		}

		const actualInterval = Math.min(reconnectInterval, maxReconnectInterval);
		reconnectTimeout = window.setTimeout(() => {
			if (!manuallyDisconnected) {
				connect();
				reconnectInterval = Math.min(reconnectInterval * 1.5, maxReconnectInterval);
			}
		}, actualInterval + Math.random() * 500);
	}

	function disconnect() {
		manuallyDisconnected = true;
		if (flushTimer !== null) {
			clearTimeout(flushTimer);
			flushTimer = null;
		}
		pendingRecords = [];
		if (reconnectTimeout) {
			clearTimeout(reconnectTimeout);
			reconnectTimeout = null;
		}
		if (ws) {
			ws.close(1000, 'User disconnected');
			ws = null;
		}
		update(s => ({ ...s, connected: false, connecting: false, entries: [] }));
	}

	function pause() {
		update(s => ({ ...s, paused: true }));
	}

	function resume() {
		update(s => ({ ...s, paused: false }));
	}

	function clear() {
		pendingRecords = [];
		update(s => ({ ...s, entries: [] }));
	}

	function setBufferSize(size: number) {
		const clamped = Math.max(1000, Math.min(5000, size));
		update(s => {
			const entries = s.entries.length > clamped
				? s.entries.slice(s.entries.length - clamped)
				: s.entries;
			return { ...s, bufferSize: clamped, entries };
		});
	}

	function setMinLevel(level: string) {
		update(s => ({ ...s, minLevel: level.toUpperCase() }));
	}

	function setFilters(filters: LogFilter[]) {
		update(s => ({ ...s, filters }));
	}

	function setFilterMode(mode: 'any' | 'all') {
		update(s => ({ ...s, filterMode: mode }));
	}

	function setSearch(search: string) {
		update(s => ({ ...s, search }));
	}

	return {
		subscribe,
		connect,
		disconnect,
		pause,
		resume,
		clear,
		setBufferSize,
		setMinLevel,
		setFilters,
		setFilterMode,
		setSearch,
	};
}

export const logStreamStore = createLogStreamStore();

function matchesFilter(filter: LogFilter, attrs: Record<string, unknown>, msg: string): boolean {
	if (filter.key === 'msg') {
		return msg.toLowerCase().includes(filter.value.toLowerCase());
	}
	const val = attrs[filter.key];
	if (val === undefined) return false;
	return String(val).toLowerCase().includes(filter.value.toLowerCase());
}

export const filteredLogEntries = derived(
	logStreamStore,
	($state) => {
		const minLevelNum = LEVEL_ORDER[$state.minLevel] ?? 0;

		return $state.entries.filter(entry => {
			const entryLevelNum = LEVEL_ORDER[entry.level] ?? 0;
			if (entryLevelNum < minLevelNum) return false;

			if ($state.filters.length > 0) {
				if ($state.filterMode === 'all') {
					if (!$state.filters.every(f => matchesFilter(f, entry.attrs, entry.msg))) return false;
				} else {
					if (!$state.filters.some(f => matchesFilter(f, entry.attrs, entry.msg))) return false;
				}
			}

			if ($state.search) {
				const term = $state.search.toLowerCase();
				const inMsg = entry.msg.toLowerCase().includes(term);
				const inAttrs = Object.entries(entry.attrs).some(
					([k, v]) => k.toLowerCase().includes(term) || String(v).toLowerCase().includes(term)
				);
				if (!inMsg && !inAttrs) return false;
			}

			return true;
		});
	}
);
