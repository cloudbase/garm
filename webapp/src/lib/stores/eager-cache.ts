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
	ControllerInfo 
} from '../api/generated/api.js';

interface EagerCacheState {
	repositories: Repository[];
	organizations: Organization[];
	enterprises: Enterprise[];
	pools: Pool[];
	scalesets: ScaleSet[];
	credentials: ForgeCredentials[];
	endpoints: ForgeEndpoint[];
	controllerInfo: ControllerInfo | null;
	loading: {
		repositories: boolean;
		organizations: boolean;
		enterprises: boolean;
		pools: boolean;
		scalesets: boolean;
		credentials: boolean;
		endpoints: boolean;
		controllerInfo: boolean;
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
	loading: {
		repositories: false,
		organizations: false,
		enterprises: false,
		pools: false,
		scalesets: false,
		credentials: false,
		endpoints: false,
		controllerInfo: false,
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
	}
};

export const eagerCache = writable<EagerCacheState>(initialState);

class EagerCacheManager {
	private unsubscribers: (() => void)[] = [];
	private loadingPromises: Map<string, Promise<any>> = new Map();
	private retryAttempts: Map<string, number> = new Map();
	private readonly MAX_RETRIES = 3;
	private readonly RETRY_DELAY_MS = 1000;
	private websocketStatusUnsubscriber: (() => void) | null = null;

	async loadResource(resourceType: keyof Omit<EagerCacheState, 'loading' | 'loaded' | 'errorMessages'>, priority: boolean = false) {
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

	private async attemptLoad(resourceType: keyof Omit<EagerCacheState, 'loading' | 'loaded' | 'errorMessages'>): Promise<any> {
		const currentAttempt = (this.retryAttempts.get(resourceType) || 0) + 1;
		this.retryAttempts.set(resourceType, currentAttempt);

		try {
			let loadPromise: Promise<any>;

			switch (resourceType) {
				case 'repositories':
					loadPromise = garmApi.listRepositories();
					break;
				case 'organizations':
					loadPromise = garmApi.listOrganizations();
					break;
				case 'enterprises':
					loadPromise = garmApi.listEnterprises();
					break;
				case 'pools':
					loadPromise = garmApi.listAllPools();
					break;
				case 'scalesets':
					loadPromise = garmApi.listScaleSets();
					break;
				case 'credentials':
					loadPromise = garmApi.listAllCredentials();
					break;
				case 'endpoints':
					loadPromise = garmApi.listAllEndpoints();
					break;
				case 'controllerInfo':
					loadPromise = garmApi.getControllerInfo();
					break;
				default:
					throw new Error(`Unknown resource type: ${resourceType}`);
			}

			return await loadPromise;
		} catch (error) {
			// If we haven't reached max retries, try again with exponential backoff
			if (currentAttempt < this.MAX_RETRIES) {
				const delay = this.RETRY_DELAY_MS * Math.pow(2, currentAttempt - 1); // Exponential backoff
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
		const resourceTypes = ['repositories', 'organizations', 'enterprises', 'pools', 'scalesets', 'credentials', 'endpoints'];
		const toLoad = resourceTypes.filter(type => type !== excludeResource);

		// Load in background with slight delays to avoid overwhelming the API
		for (const resourceType of toLoad) {
			setTimeout(() => {
				this.loadResource(resourceType as any, false).catch(error => {
					console.warn(`Background loading failed for ${resourceType}:`, error);
					// Background loading failures are not critical, just log them
				});
			}, 100 * toLoad.indexOf(resourceType));
		}
	}

	// Public method to manually retry loading a resource
	retryResource(resourceType: keyof Omit<EagerCacheState, 'loading' | 'loaded' | 'errorMessages'>) {
		// Clear any existing retry attempts to start fresh
		this.retryAttempts.delete(resourceType);
		return this.loadResource(resourceType, true);
	}

	setupWebSocketSubscriptions() {
		// Clean up existing subscriptions
		this.cleanup();

		// Subscribe to all resource types
		const subscriptions = [
			websocketStore.subscribeToEntity('repository', ['create', 'update', 'delete'], this.handleRepositoryEvent.bind(this)),
			websocketStore.subscribeToEntity('organization', ['create', 'update', 'delete'], this.handleOrganizationEvent.bind(this)),
			websocketStore.subscribeToEntity('enterprise', ['create', 'update', 'delete'], this.handleEnterpriseEvent.bind(this)),
			websocketStore.subscribeToEntity('pool', ['create', 'update', 'delete'], this.handlePoolEvent.bind(this)),
			websocketStore.subscribeToEntity('scaleset', ['create', 'update', 'delete'], this.handleScaleSetEvent.bind(this)),
			websocketStore.subscribeToEntity('controller', ['update'], this.handleControllerEvent.bind(this)),
			websocketStore.subscribeToEntity('github_credentials', ['create', 'update', 'delete'], this.handleCredentialsEvent.bind(this)),
			websocketStore.subscribeToEntity('gitea_credentials', ['create', 'update', 'delete'], this.handleCredentialsEvent.bind(this)),
			websocketStore.subscribeToEntity('github_endpoint', ['create', 'update', 'delete'], this.handleEndpointEvent.bind(this))
		];

		this.unsubscribers = subscriptions;

		// Monitor WebSocket connection status
		this.setupWebSocketStatusMonitoring();
	}

	private setupWebSocketStatusMonitoring() {
		if (this.websocketStatusUnsubscriber) {
			this.websocketStatusUnsubscriber();
		}

		let wasConnected = false;
		
		this.websocketStatusUnsubscriber = websocketStore.subscribe(state => {
			// When WebSocket connects for the first time or reconnects after being disconnected
			if (state.connected && !wasConnected) {
				console.log('[EagerCache] WebSocket connected - reinitializing cache');
				// Reload all resources when WebSocket connects
				this.initializeAllResources();
			}
			wasConnected = state.connected;
		});
	}

	// Reinitialize all resources when WebSocket connects
	private async initializeAllResources() {
		const resourceTypes: (keyof Omit<EagerCacheState, 'loading' | 'loaded' | 'errorMessages'>)[] = [
			'repositories', 'organizations', 'enterprises', 'pools', 'scalesets', 
			'credentials', 'endpoints', 'controllerInfo'
		];

		// Load all resources in parallel
		const loadPromises = resourceTypes.map(resourceType => 
			this.loadResource(resourceType, true).catch(error => {
				console.warn(`Failed to reload ${resourceType} on WebSocket reconnect:`, error);
			})
		);

		await Promise.allSettled(loadPromises);
	}

	private handleRepositoryEvent(event: WebSocketEvent) {
		eagerCache.update(state => {
			if (!state.loaded.repositories) return state;

			const repositories = [...state.repositories];
			const repo = event.payload as Repository;

			if (event.operation === 'create') {
				repositories.push(repo);
			} else if (event.operation === 'update') {
				const index = repositories.findIndex(r => r.id === repo.id);
				if (index !== -1) repositories[index] = repo;
			} else if (event.operation === 'delete') {
				const repoId = typeof repo === 'object' ? repo.id : repo;
				const index = repositories.findIndex(r => r.id === repoId);
				if (index !== -1) repositories.splice(index, 1);
			}

			return { ...state, repositories };
		});
	}

	private handleOrganizationEvent(event: WebSocketEvent) {
		eagerCache.update(state => {
			if (!state.loaded.organizations) return state;

			const organizations = [...state.organizations];
			const org = event.payload as Organization;

			if (event.operation === 'create') {
				organizations.push(org);
			} else if (event.operation === 'update') {
				const index = organizations.findIndex(o => o.id === org.id);
				if (index !== -1) organizations[index] = org;
			} else if (event.operation === 'delete') {
				const orgId = typeof org === 'object' ? org.id : org;
				const index = organizations.findIndex(o => o.id === orgId);
				if (index !== -1) organizations.splice(index, 1);
			}

			return { ...state, organizations };
		});
	}

	private handleEnterpriseEvent(event: WebSocketEvent) {
		eagerCache.update(state => {
			if (!state.loaded.enterprises) return state;

			const enterprises = [...state.enterprises];
			const ent = event.payload as Enterprise;

			if (event.operation === 'create') {
				enterprises.push(ent);
			} else if (event.operation === 'update') {
				const index = enterprises.findIndex(e => e.id === ent.id);
				if (index !== -1) enterprises[index] = ent;
			} else if (event.operation === 'delete') {
				const entId = typeof ent === 'object' ? ent.id : ent;
				const index = enterprises.findIndex(e => e.id === entId);
				if (index !== -1) enterprises.splice(index, 1);
			}

			return { ...state, enterprises };
		});
	}

	private handlePoolEvent(event: WebSocketEvent) {
		eagerCache.update(state => {
			if (!state.loaded.pools) return state;

			const pools = [...state.pools];
			const pool = event.payload as Pool;

			if (event.operation === 'create') {
				pools.push(pool);
			} else if (event.operation === 'update') {
				const index = pools.findIndex(p => p.id === pool.id);
				if (index !== -1) pools[index] = pool;
			} else if (event.operation === 'delete') {
				const poolId = typeof pool === 'object' ? pool.id : pool;
				const index = pools.findIndex(p => p.id === poolId);
				if (index !== -1) pools.splice(index, 1);
			}

			return { ...state, pools };
		});
	}

	private handleScaleSetEvent(event: WebSocketEvent) {
		eagerCache.update(state => {
			if (!state.loaded.scalesets) return state;

			const scalesets = [...state.scalesets];
			const scaleset = event.payload as ScaleSet;

			if (event.operation === 'create') {
				scalesets.push(scaleset);
			} else if (event.operation === 'update') {
				const index = scalesets.findIndex(s => s.id === scaleset.id);
				if (index !== -1) scalesets[index] = scaleset;
			} else if (event.operation === 'delete') {
				const scalesetId = typeof scaleset === 'object' ? scaleset.id : scaleset;
				const index = scalesets.findIndex(s => s.id === scalesetId);
				if (index !== -1) scalesets.splice(index, 1);
			}

			return { ...state, scalesets };
		});
	}

	private handleCredentialsEvent(event: WebSocketEvent) {
		eagerCache.update(state => {
			if (!state.loaded.credentials) return state;

			const credentials = [...state.credentials];
			const cred = event.payload as ForgeCredentials;

			if (event.operation === 'create') {
				credentials.push(cred);
			} else if (event.operation === 'update') {
				const index = credentials.findIndex(c => c.id === cred.id);
				if (index !== -1) credentials[index] = cred;
			} else if (event.operation === 'delete') {
				const credId = typeof cred === 'object' ? cred.id : cred;
				const index = credentials.findIndex(c => c.id === credId);
				if (index !== -1) credentials.splice(index, 1);
			}

			return { ...state, credentials };
		});
	}

	private handleEndpointEvent(event: WebSocketEvent) {
		eagerCache.update(state => {
			if (!state.loaded.endpoints) return state;

			const endpoints = [...state.endpoints];
			const endpoint = event.payload as ForgeEndpoint;

			if (event.operation === 'create') {
				endpoints.push(endpoint);
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

	cleanup() {
		this.unsubscribers.forEach(unsubscribe => unsubscribe());
		this.unsubscribers = [];
		
		if (this.websocketStatusUnsubscriber) {
			this.websocketStatusUnsubscriber();
			this.websocketStatusUnsubscriber = null;
		}
	}

	// Helper method to check if we should use cache or direct API
	private shouldUseCache(): boolean {
		const wsState = get(websocketStore);
		return wsState.connected;
	}

	// Helper methods for components - check WebSocket status first
	async getRepositories(): Promise<Repository[]> {
		const wsState = get(websocketStore);
		
		if (!wsState.connected) {
			// WebSocket disconnected - fetch directly from API
			console.log('[EagerCache] WebSocket disconnected - fetching repositories directly from API');
			return await garmApi.listRepositories();
		}

		const state = get(eagerCache);
		if (state.loaded.repositories) {
			return state.repositories;
		}

		return this.loadResource('repositories', true);
	}

	async getOrganizations(): Promise<Organization[]> {
		const wsState = get(websocketStore);
		
		if (!wsState.connected) {
			console.log('[EagerCache] WebSocket disconnected - fetching organizations directly from API');
			return await garmApi.listOrganizations();
		}

		const state = get(eagerCache);
		if (state.loaded.organizations) {
			return state.organizations;
		}

		return this.loadResource('organizations', true);
	}

	async getEnterprises(): Promise<Enterprise[]> {
		const wsState = get(websocketStore);
		
		if (!wsState.connected) {
			console.log('[EagerCache] WebSocket disconnected - fetching enterprises directly from API');
			return await garmApi.listEnterprises();
		}

		const state = get(eagerCache);
		if (state.loaded.enterprises) {
			return state.enterprises;
		}

		return this.loadResource('enterprises', true);
	}

	async getPools(): Promise<Pool[]> {
		const wsState = get(websocketStore);
		
		if (!wsState.connected) {
			console.log('[EagerCache] WebSocket disconnected - fetching pools directly from API');
			return await garmApi.listAllPools();
		}

		const state = get(eagerCache);
		if (state.loaded.pools) {
			return state.pools;
		}

		return this.loadResource('pools', true);
	}

	async getScaleSets(): Promise<ScaleSet[]> {
		const wsState = get(websocketStore);
		
		if (!wsState.connected) {
			console.log('[EagerCache] WebSocket disconnected - fetching scalesets directly from API');
			return await garmApi.listScaleSets();
		}

		const state = get(eagerCache);
		if (state.loaded.scalesets) {
			return state.scalesets;
		}

		return this.loadResource('scalesets', true);
	}

	async getCredentials(): Promise<ForgeCredentials[]> {
		const wsState = get(websocketStore);
		
		if (!wsState.connected) {
			console.log('[EagerCache] WebSocket disconnected - fetching credentials directly from API');
			return await garmApi.listAllCredentials();
		}

		const state = get(eagerCache);
		if (state.loaded.credentials) {
			return state.credentials;
		}

		return this.loadResource('credentials', true);
	}

	async getEndpoints(): Promise<ForgeEndpoint[]> {
		const wsState = get(websocketStore);
		
		if (!wsState.connected) {
			console.log('[EagerCache] WebSocket disconnected - fetching endpoints directly from API');
			return await garmApi.listAllEndpoints();
		}

		const state = get(eagerCache);
		if (state.loaded.endpoints) {
			return state.endpoints;
		}

		return this.loadResource('endpoints', true);
	}

	async getControllerInfo(): Promise<ControllerInfo | null> {
		const wsState = get(websocketStore);
		
		if (!wsState.connected) {
			console.log('[EagerCache] WebSocket disconnected - fetching controller info directly from API');
			return await garmApi.getControllerInfo();
		}

		const state = get(eagerCache);
		if (state.loaded.controllerInfo) {
			return state.controllerInfo;
		}

		return this.loadResource('controllerInfo', true);
	}

	private handleControllerEvent(event: WebSocketEvent) {
		eagerCache.update(state => {
			if (!state.loaded.controllerInfo) return state;

			const controllerInfo = event.payload as ControllerInfo;

			// Controller info is a singleton, so we just replace it
			if (event.operation === 'update') {
				return { ...state, controllerInfo };
			}

			return state;
		});
	}
}

export const eagerCacheManager = new EagerCacheManager();

// Initialize websocket subscriptions when the module is loaded
if (typeof window !== 'undefined') {
	eagerCacheManager.setupWebSocketSubscriptions();
}