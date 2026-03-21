import { writable, get } from 'svelte/store';
import { garmApi } from '../api/client.js';
import { websocketStore, type WebSocketEvent } from './websocket.js';
import type {
	Repository,
	Organization,
	Enterprise,
	Pool,
	ScaleSet,
	ForgeCredentials,
	ForgeEndpoint,
	ControllerInfo,
	Template
} from '../api/generated/api.js';

type CacheResourceKey = keyof Omit<EagerCacheState, 'loading' | 'loaded' | 'errorMessages'>;

interface EagerCacheState {
	repositories: Repository[];
	organizations: Organization[];
	enterprises: Enterprise[];
	pools: Pool[];
	scalesets: ScaleSet[];
	credentials: ForgeCredentials[];
	endpoints: ForgeEndpoint[];
	controllerInfo: ControllerInfo | null;
	templates: Template[];
	loading: {
		repositories: boolean;
		organizations: boolean;
		enterprises: boolean;
		pools: boolean;
		scalesets: boolean;
		credentials: boolean;
		endpoints: boolean;
		controllerInfo: boolean;
		templates: boolean;
	};
	loaded: {
		repositories: boolean;
		organizations: boolean;
		enterprises: boolean;
		pools: boolean;
		scalesets: boolean;
		credentials: boolean;
		endpoints: boolean;
		controllerInfo: boolean;
		templates: boolean;
	};
	errorMessages: {
		repositories: string;
		organizations: string;
		enterprises: string;
		pools: string;
		scalesets: string;
		credentials: string;
		endpoints: string;
		controllerInfo: string;
		templates: string;
	};
}

const initialState: EagerCacheState = {
	repositories: [],
	organizations: [],
	enterprises: [],
	pools: [],
	scalesets: [],
	credentials: [],
	endpoints: [],
	controllerInfo: null,
	templates: [],
	loading: {
		repositories: false,
		organizations: false,
		enterprises: false,
		pools: false,
		scalesets: false,
		credentials: false,
		endpoints: false,
		controllerInfo: false,
		templates: false,
	},
	loaded: {
		repositories: false,
		organizations: false,
		enterprises: false,
		pools: false,
		scalesets: false,
		credentials: false,
		endpoints: false,
		controllerInfo: false,
		templates: false,
	},
	errorMessages: {
		repositories: '',
		organizations: '',
		enterprises: '',
		pools: '',
		scalesets: '',
		credentials: '',
		endpoints: '',
		controllerInfo: '',
		templates: '',
	}
};

export const eagerCache = writable<EagerCacheState>(initialState);

// Maps resource keys to their API fetch functions for attemptLoad and getters
const apiFetchers: Record<CacheResourceKey, () => Promise<any>> = {
	repositories: () => garmApi.listRepositories(),
	organizations: () => garmApi.listOrganizations(),
	enterprises: () => garmApi.listEnterprises(),
	pools: () => garmApi.listAllPools(),
	scalesets: () => garmApi.listScaleSets(),
	credentials: () => garmApi.listAllCredentials(),
	endpoints: () => garmApi.listAllEndpoints(),
	controllerInfo: () => garmApi.getControllerInfo(),
	templates: () => garmApi.listTemplates(),
};

class EagerCacheManager {
	private unsubscribers: (() => void)[] = [];
	private loadingPromises: Map<string, Promise<any>> = new Map();
	private retryAttempts: Map<string, number> = new Map();
	private readonly MAX_RETRIES = 3;
	private readonly RETRY_DELAY_MS = 1000;
	private websocketStatusUnsubscriber: (() => void) | null = null;

	async loadResource(resourceType: CacheResourceKey, priority: boolean = false) {
		// Avoid duplicate loading
		if (this.loadingPromises.has(resourceType)) {
			return this.loadingPromises.get(resourceType);
		}

		// Clear any previous error message and set loading state
		eagerCache.update(state => ({
			...state,
			loading: { ...state.loading, [resourceType]: true },
			errorMessages: { ...state.errorMessages, [resourceType]: '' }
		}));

		const loadPromise = this.attemptLoad(resourceType);
		this.loadingPromises.set(resourceType, loadPromise);

		try {
			const data = await loadPromise;
			eagerCache.update(state => ({
				...state,
				[resourceType]: data,
				loading: { ...state.loading, [resourceType]: false },
				loaded: { ...state.loaded, [resourceType]: true },
				errorMessages: { ...state.errorMessages, [resourceType]: '' }
			}));

			// Reset retry attempts on success
			this.retryAttempts.delete(resourceType);

			// If this is a priority load, start background loading of other resources
			if (priority) {
				this.startBackgroundLoading(resourceType);
			}

			return data;
		} catch (error) {
			const errorMessage = error instanceof Error ? error.message : 'Failed to load data';
			eagerCache.update(state => ({
				...state,
				loading: { ...state.loading, [resourceType]: false },
				errorMessages: { ...state.errorMessages, [resourceType]: errorMessage }
			}));
			console.error(`Failed to load ${resourceType}:`, error);
			throw error;
		} finally {
			this.loadingPromises.delete(resourceType);
		}
	}

	private async attemptLoad(resourceType: CacheResourceKey): Promise<any> {
		const currentAttempt = (this.retryAttempts.get(resourceType) || 0) + 1;
		this.retryAttempts.set(resourceType, currentAttempt);

		try {
			const fetcher = apiFetchers[resourceType];
			if (!fetcher) {
				throw new Error(`Unknown resource type: ${resourceType}`);
			}
			return await fetcher();
		} catch (error) {
			// If we haven't reached max retries, try again with exponential backoff
			if (currentAttempt < this.MAX_RETRIES) {
				const delay = this.RETRY_DELAY_MS * Math.pow(2, currentAttempt - 1);
				console.warn(`Attempt ${currentAttempt} failed for ${resourceType}, retrying in ${delay}ms...`, error);

				await new Promise(resolve => setTimeout(resolve, delay));
				return this.attemptLoad(resourceType);
			} else {
				console.error(`All ${this.MAX_RETRIES} attempts failed for ${resourceType}:`, error);
				throw error;
			}
		}
	}

	private async startBackgroundLoading(excludeResource: string) {
		const resourceTypes = Object.keys(apiFetchers) as CacheResourceKey[];
		const toLoad = resourceTypes.filter(type => type !== excludeResource);

		// Load in background with slight delays to avoid overwhelming the API
		for (const resourceType of toLoad) {
			setTimeout(() => {
				this.loadResource(resourceType, false).catch(error => {
					console.warn(`Background loading failed for ${resourceType}:`, error);
				});
			}, 100 * toLoad.indexOf(resourceType));
		}
	}

	// Public method to manually retry loading a resource
	retryResource(resourceType: CacheResourceKey) {
		this.retryAttempts.delete(resourceType);
		return this.loadResource(resourceType, true);
	}

	setupWebSocketSubscriptions() {
		this.cleanup();

		const subscriptions = [
			websocketStore.subscribeToEntity('repository', ['create', 'update', 'delete'], (e) => this.handleCrudEvent(e, 'repositories')),
			websocketStore.subscribeToEntity('organization', ['create', 'update', 'delete'], (e) => this.handleCrudEvent(e, 'organizations')),
			websocketStore.subscribeToEntity('enterprise', ['create', 'update', 'delete'], (e) => this.handleCrudEvent(e, 'enterprises')),
			websocketStore.subscribeToEntity('pool', ['create', 'update', 'delete'], (e) => this.handleCrudEvent(e, 'pools')),
			websocketStore.subscribeToEntity('scaleset', ['create', 'update', 'delete'], (e) => this.handleCrudEvent(e, 'scalesets')),
			websocketStore.subscribeToEntity('template', ['create', 'update', 'delete'], (e) => this.handleCrudEvent(e, 'templates')),
			websocketStore.subscribeToEntity('controller', ['update'], this.handleControllerEvent.bind(this)),
			websocketStore.subscribeToEntity('github_credentials', ['create', 'update', 'delete'], this.handleCredentialsEvent.bind(this)),
			websocketStore.subscribeToEntity('gitea_credentials', ['create', 'update', 'delete'], this.handleCredentialsEvent.bind(this)),
			websocketStore.subscribeToEntity('github_endpoint', ['create', 'update', 'delete'], this.handleEndpointEvent.bind(this)),
		];

		this.unsubscribers = subscriptions;
		this.setupWebSocketStatusMonitoring();
	}

	private setupWebSocketStatusMonitoring() {
		if (this.websocketStatusUnsubscriber) {
			this.websocketStatusUnsubscriber();
		}

		let wasConnected = false;

		this.websocketStatusUnsubscriber = websocketStore.subscribe(state => {
			if (state.connected && !wasConnected) {
				console.log('[EagerCache] WebSocket connected - reinitializing cache');
				this.initializeAllResources();
			}
			wasConnected = state.connected;
		});
	}

	private async initializeAllResources() {
		const resourceTypes = Object.keys(apiFetchers) as CacheResourceKey[];
		const loadPromises = resourceTypes.map(resourceType =>
			this.loadResource(resourceType, true).catch(error => {
				console.warn(`Failed to reload ${resourceType} on WebSocket reconnect:`, error);
			})
		);
		await Promise.allSettled(loadPromises);
	}

	// Generic CRUD handler for entities matched by .id
	private handleCrudEvent(event: WebSocketEvent, resourceKey: CacheResourceKey) {
		eagerCache.update(state => {
			if (!state.loaded[resourceKey]) return state;

			const items = [...(state[resourceKey] as any[])];
			const entity = event.payload as any;

			if (event.operation === 'create') {
				const existingIndex = items.findIndex(item => item.id === entity.id);
				if (existingIndex === -1) {
					items.push(entity);
				} else {
					items[existingIndex] = entity;
				}
			} else if (event.operation === 'update') {
				const index = items.findIndex(item => item.id === entity.id);
				if (index !== -1) items[index] = entity;
			} else if (event.operation === 'delete') {
				const entityId = typeof entity === 'object' ? entity.id : entity;
				const index = items.findIndex(item => item.id === entityId);
				if (index !== -1) items.splice(index, 1);
			}

			return { ...state, [resourceKey]: items };
		});
	}

	// Credentials match on both id and forge_type
	private handleCredentialsEvent(event: WebSocketEvent) {
		eagerCache.update(state => {
			if (!state.loaded.credentials) return state;

			const credentials = [...state.credentials];
			const cred = event.payload as ForgeCredentials;
			// Derive forge_type from the WebSocket entity type if missing from payload
			// (backend delete events may send sparse payloads without forge_type)
			const forgeType = cred.forge_type || (event['entity-type'] === 'github_credentials' ? 'github' : 'gitea');
			const matchCred = (c: ForgeCredentials) => c.id === cred.id && c.forge_type === forgeType;

			if (event.operation === 'create') {
				const existingIndex = credentials.findIndex(matchCred);
				if (existingIndex === -1) {
					credentials.push(cred);
				} else {
					credentials[existingIndex] = cred;
				}
			} else if (event.operation === 'update') {
				const index = credentials.findIndex(matchCred);
				if (index !== -1) credentials[index] = cred;
			} else if (event.operation === 'delete') {
				const index = credentials.findIndex(matchCred);
				if (index !== -1) credentials.splice(index, 1);
			}

			return { ...state, credentials };
		});
	}

	// Endpoints match on name instead of id
	private handleEndpointEvent(event: WebSocketEvent) {
		eagerCache.update(state => {
			if (!state.loaded.endpoints) return state;

			const endpoints = [...state.endpoints];
			const endpoint = event.payload as ForgeEndpoint;

			if (event.operation === 'create') {
				const existingIndex = endpoints.findIndex(e => e.name === endpoint.name);
				if (existingIndex === -1) {
					endpoints.push(endpoint);
				} else {
					endpoints[existingIndex] = endpoint;
				}
			} else if (event.operation === 'update') {
				const index = endpoints.findIndex(e => e.name === endpoint.name);
				if (index !== -1) endpoints[index] = endpoint;
			} else if (event.operation === 'delete') {
				const endpointName = typeof endpoint === 'object' ? endpoint.name : endpoint;
				const index = endpoints.findIndex(e => e.name === endpointName);
				if (index !== -1) endpoints.splice(index, 1);
			}

			return { ...state, endpoints };
		});
	}

	// Controller is a singleton, update-only
	private handleControllerEvent(event: WebSocketEvent) {
		eagerCache.update(state => {
			if (!state.loaded.controllerInfo) return state;
			if (event.operation === 'update') {
				return { ...state, controllerInfo: event.payload as ControllerInfo };
			}
			return state;
		});
	}

	cleanup() {
		this.unsubscribers.forEach(unsubscribe => unsubscribe());
		this.unsubscribers = [];

		if (this.websocketStatusUnsubscriber) {
			this.websocketStatusUnsubscriber();
			this.websocketStatusUnsubscriber = null;
		}
	}

	// Generic getter: use cache if WS connected and loaded, otherwise fetch from API
	private async getCachedOrFetch<T>(resourceKey: CacheResourceKey, logName: string): Promise<T> {
		const wsState = get(websocketStore);
		if (!wsState.connected) {
			console.log(`[EagerCache] WebSocket disconnected - fetching ${logName} directly from API`);
			return await apiFetchers[resourceKey]() as T;
		}
		const state = get(eagerCache);
		if (state.loaded[resourceKey]) {
			return state[resourceKey] as T;
		}
		return this.loadResource(resourceKey, true) as Promise<T>;
	}

	async getRepositories(): Promise<Repository[]> {
		return this.getCachedOrFetch('repositories', 'repositories');
	}

	async getOrganizations(): Promise<Organization[]> {
		return this.getCachedOrFetch('organizations', 'organizations');
	}

	async getEnterprises(): Promise<Enterprise[]> {
		return this.getCachedOrFetch('enterprises', 'enterprises');
	}

	async getPools(): Promise<Pool[]> {
		return this.getCachedOrFetch('pools', 'pools');
	}

	async getScaleSets(): Promise<ScaleSet[]> {
		return this.getCachedOrFetch('scalesets', 'scalesets');
	}

	async getCredentials(): Promise<ForgeCredentials[]> {
		return this.getCachedOrFetch('credentials', 'credentials');
	}

	async getEndpoints(): Promise<ForgeEndpoint[]> {
		return this.getCachedOrFetch('endpoints', 'endpoints');
	}

	async getControllerInfo(): Promise<ControllerInfo | null> {
		return this.getCachedOrFetch('controllerInfo', 'controller info');
	}

	async getTemplates(): Promise<Template[]> {
		return this.getCachedOrFetch('templates', 'templates');
	}
}

export const eagerCacheManager = new EagerCacheManager();

// Initialize websocket subscriptions when the module is loaded
if (typeof window !== 'undefined') {
	eagerCacheManager.setupWebSocketSubscriptions();
}
