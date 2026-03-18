import { writable, readonly } from 'svelte/store';

export type MetricsConnectionState = 'connecting' | 'connected' | 'disconnected';

export interface MetricsEntity {
	id: string;
	name: string;
	type: string;
	endpoint: string;
	pool_count: number;
	scale_set_count: number;
	healthy: boolean;
}

export interface MetricsPool {
	id: string;
	provider_name: string;
	os_type: string;
	max_runners: number;
	enabled: boolean;
	repo_name?: string;
	org_name?: string;
	enterprise_name?: string;
	runner_counts: Record<string, number>;
	runner_status_counts: Record<string, number>;
}

export interface MetricsScaleSet {
	id: number;
	name: string;
	provider_name: string;
	os_type: string;
	max_runners: number;
	enabled: boolean;
	repo_name?: string;
	org_name?: string;
	enterprise_name?: string;
	runner_counts: Record<string, number>;
	runner_status_counts: Record<string, number>;
}

export interface MetricsSnapshot {
	entities: MetricsEntity[];
	pools: MetricsPool[];
	scale_sets: MetricsScaleSet[];
}

function createMetricsStore() {
	const { subscribe, set } = writable<MetricsSnapshot | null>(null);
	const connectionState = writable<MetricsConnectionState>('connecting');

	let ws: WebSocket | null = null;
	let reconnectAttempts = 0;
	let maxReconnectAttempts = 50;
	let baseReconnectInterval = 1000;
	let reconnectInterval = 1000;
	let maxReconnectInterval = 30000;
	let reconnectTimeout: number | null = null;
	let manuallyDisconnected = false;

	function getWebSocketUrl(): string {
		const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
		const host = window.location.host;
		return `${protocol}//${host}/api/v1/ws/metrics`;
	}

	function connect() {
		if (ws && (ws.readyState === WebSocket.CONNECTING || ws.readyState === WebSocket.OPEN)) {
			return;
		}

		manuallyDisconnected = false;
		connectionState.set('connecting');

		try {
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
				connectionState.set('connected');
			};

			ws.onmessage = (event) => {
				try {
					const data: MetricsSnapshot = JSON.parse(event.data);
					set(data);
				} catch (err) {
					console.error('[MetricsWS] Error parsing message:', err);
				}
			};

			ws.onclose = (event) => {
				clearTimeout(connectionTimeout);
				connectionState.set('disconnected');
				const wasManualDisconnect = event.code === 1000 && manuallyDisconnected;
				if (!wasManualDisconnect) {
					scheduleReconnect();
				}
			};

			ws.onerror = () => {
				connectionState.set('disconnected');
				if (!manuallyDisconnected) {
					scheduleReconnect();
				}
			};
		} catch (err) {
			console.error('[MetricsWS] Failed to connect:', err);
		}
	}

	function scheduleReconnect() {
		if (manuallyDisconnected) {
			return;
		}

		if (reconnectTimeout) {
			clearTimeout(reconnectTimeout);
		}

		reconnectAttempts++;

		if (reconnectAttempts > maxReconnectAttempts) {
			reconnectAttempts = 1;
			reconnectInterval = baseReconnectInterval;
		}

		const actualInterval = Math.min(reconnectInterval, maxReconnectInterval);

		reconnectTimeout = window.setTimeout(() => {
			if (!manuallyDisconnected) {
				connect();
				const jitter = Math.random() * 1000;
				reconnectInterval = Math.min(reconnectInterval * 1.5 + jitter, maxReconnectInterval);
			}
		}, actualInterval);
	}

	function disconnect() {
		manuallyDisconnected = true;
		connectionState.set('disconnected');

		if (reconnectTimeout) {
			clearTimeout(reconnectTimeout);
			reconnectTimeout = null;
		}

		if (ws) {
			ws.close(1000, 'Manual disconnect');
			ws = null;
		}

		set(null);
	}

	// Handle network connectivity changes
	if (typeof window !== 'undefined') {
		window.addEventListener('online', () => {
			if (!manuallyDisconnected) {
				setTimeout(() => {
					if (!ws || ws.readyState === WebSocket.CLOSED || ws.readyState === WebSocket.CLOSING) {
						reconnectAttempts = 0;
						reconnectInterval = baseReconnectInterval;
						connect();
					}
				}, 2000);
			}
		});

		// Periodic connection check
		setInterval(() => {
			if (!manuallyDisconnected) {
				if (!ws || ws.readyState === WebSocket.CLOSED || ws.readyState === WebSocket.CLOSING) {
					connect();
				}
			}
		}, 10000);

		// Auto-connect
		connect();
	}

	return {
		subscribe,
		connect,
		disconnect,
		connectionState: readonly(connectionState)
	};
}

export const metricsStore = createMetricsStore();
