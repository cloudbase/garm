import { writable } from 'svelte/store';
import { browser } from '$app/environment';
import { garmApi } from '../api/client.js';

// Check if we're in development mode (cross-origin setup)
const isDevelopmentMode = () => {
	if (!browser) return false;
	// Development mode: either VITE_GARM_API_URL is set OR we detect cross-origin
	return !!(import.meta.env.VITE_GARM_API_URL) || window.location.port === '5173';
};

interface AuthState {
	isAuthenticated: boolean;
	user: string | null;
	loading: boolean;
	needsInitialization: boolean;
}

const initialState: AuthState = {
	isAuthenticated: false,
	user: null,
	loading: true,
	needsInitialization: false
};

// Keep using writable store for compatibility with existing API calls
// but enhance with Svelte 5 features where possible
export const authStore = writable<AuthState>(initialState);

// Cookie utilities
function setCookie(name: string, value: string, days: number = 7): void {
	if (!browser) return;
	
	const expires = new Date();
	expires.setTime(expires.getTime() + (days * 24 * 60 * 60 * 1000));
	document.cookie = `${name}=${value};expires=${expires.toUTCString()};path=/;SameSite=Lax`;
}

function getCookie(name: string): string | null {
	if (!browser) return null;
	
	const nameEQ = name + "=";
	const ca = document.cookie.split(';');
	for (let i = 0; i < ca.length; i++) {
		let c = ca[i];
		while (c.charAt(0) === ' ') c = c.substring(1, c.length);
		if (c.indexOf(nameEQ) === 0) {
			const value = c.substring(nameEQ.length, c.length);
			return value;
		}
	}
	return null;
}

function deleteCookie(name: string): void {
	if (!browser) return;
	document.cookie = `${name}=;expires=Thu, 01 Jan 1970 00:00:01 GMT;path=/`;
}

// Auth utilities
export const auth = {
	async login(username: string, password: string): Promise<void> {
		try {
			authStore.update(state => ({ ...state, loading: true }));
			
			const response = await garmApi.login({ username, password });
			
			// Store JWT token in cookies for server authentication and set it in the API client
			if (browser) {
				setCookie('garm_token', response.token);
				setCookie('garm_user', username);
			}
			
			// Set the token in the API client for future requests
			garmApi.setToken(response.token);
			
			authStore.set({
				isAuthenticated: true,
				user: username,
				loading: false,
				needsInitialization: false
			});
		} catch (error) {
			authStore.update(state => ({ ...state, loading: false }));
			throw error;
		}
	},

	logout(): void {
		if (browser) {
			deleteCookie('garm_token');
			deleteCookie('garm_user');
		}
		
		authStore.set({
			isAuthenticated: false,
			user: null,
			loading: false,
			needsInitialization: false
		});
	},

	async init(): Promise<void> {
		if (browser) {
			try {
				authStore.update(state => ({ ...state, loading: true }));
				
				// First, always check initialization status by doing GET /api/v1/login
				await auth.checkInitializationStatus();
				
				// If we get here without needsInitialization being set, check for existing auth
				const token = getCookie('garm_token');
				const user = getCookie('garm_user');
				
				if (token && user) {
					// Set the token in the API client for future requests
					garmApi.setToken(token);
					
					// Verify token is still valid
					const isValid = await auth.checkAuth();
					if (isValid) {
						// Token is valid, set authenticated state
						authStore.set({
							isAuthenticated: true,
							user,
							loading: false,
							needsInitialization: false
						});
						return;
					}
				}
				
				// No valid token, user needs to login (but GARM is initialized)
				authStore.update(state => ({ 
					...state, 
					loading: false,
					needsInitialization: false
				}));
				
			} catch (error) {
				// If checkInitializationStatus threw an error, it should have set needsInitialization
				authStore.update(state => ({ ...state, loading: false }));
			}
		} else {
			authStore.update(state => ({ ...state, loading: false }));
		}
	},

	// Check initialization status by calling GET /api/v1/login
	async checkInitializationStatus(): Promise<void> {
		try {
			// Make a GET request to /api/v1/login to check status
			const headers: Record<string, string> = {
				'Accept': 'application/json',
			};
			
			// In development mode, always use Bearer token; in production, prefer cookies
			const token = getCookie('garm_token');
			const isDevMode = isDevelopmentMode();
			
			if (isDevMode && token) {
				headers['Authorization'] = `Bearer ${token}`;
			}
			
			const response = await fetch('/api/v1/login', {
				method: 'GET',
				headers,
				// Only include credentials in production (same-origin)
				credentials: isDevMode ? 'omit' : 'include'
			});
			
			if (!response.ok) {
				if (response.status === 409) {
					const errorData = await response.json();
					if (errorData.error === 'init_required') {
						// GARM needs initialization
						authStore.update(state => ({ 
							...state, 
							needsInitialization: true, 
							loading: false 
						}));
						throw new Error('Initialization required');
					}
				}
				// For other 4xx/5xx errors, assume GARM is initialized
				return;
			}
			
			// GET /api/v1/login succeeded, GARM is initialized
			return;
			
		} catch (error) {
			// If it's our initialization error, re-throw it
			if (error instanceof Error && error.message === 'Initialization required') {
				throw error;
			}
			// For network errors or other issues, assume GARM is initialized
			return;
		}
	},

	// Check if token is still valid by making a test API call
	async checkAuth(): Promise<boolean> {
		try {
			// First check if initialization is still required
			await auth.checkInitializationStatus();
			
			// If we get here, GARM is initialized, now check if token is valid
			await garmApi.getControllerInfo();
			return true;
		} catch (error: any) {
			// If it's initialization required, the checkInitializationStatus already handled it
			if (error instanceof Error && error.message === 'Initialization required') {
				return false;
			}
			
			// Check if it's an initialization required error from the API call
			if (error?.response?.status === 409 && 
				error?.response?.data?.error === 'init_required') {
				authStore.update(state => ({ 
					...state, 
					needsInitialization: true, 
					loading: false 
				}));
				return false;
			}
			
			// Token is invalid, logout
			auth.logout();
			return false;
		}
	},

	// Initialize GARM controller
	async initialize(
		username: string, 
		email: string, 
		password: string, 
		fullName?: string,
		urls?: {
			callbackUrl?: string;
			metadataUrl?: string;
			webhookUrl?: string;
			agentUrl?: string;
		}
	): Promise<void> {
		try {
			authStore.update(state => ({ ...state, loading: true }));
			
			// Step 1: Create the admin user
			const response = await garmApi.firstRun({
				username,
				email,
				password,
				full_name: fullName || username
			});
			
			// Step 2: Login with the new credentials
			await auth.login(username, password);
			
			// Step 3: Set controller URLs (similar to garm-cli init)
			const currentUrl = window.location.origin;
			const finalMetadataUrl = urls?.metadataUrl || `${currentUrl}/api/v1/metadata`;
			const finalCallbackUrl = urls?.callbackUrl || `${currentUrl}/api/v1/callbacks`;
			const finalWebhookUrl = urls?.webhookUrl || `${currentUrl}/webhooks`;
			const finalAgentUrl = urls?.agentUrl || `${currentUrl}/agent`;
			
			await garmApi.updateController({
				metadata_url: finalMetadataUrl,
				callback_url: finalCallbackUrl,
				webhook_url: finalWebhookUrl,
				agent_url: finalAgentUrl
			});
			
			authStore.update(state => ({ 
				...state, 
				needsInitialization: false 
			}));
		} catch (error) {
			authStore.update(state => ({ ...state, loading: false }));
			throw error;
		}
	}
};