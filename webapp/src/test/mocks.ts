import { vi } from 'vitest';
import type { Repository, CreateRepoParams, UpdateEntityParams } from '$lib/api/generated/api.js';

// Mock the API client
export const mockGarmApi = {
	createRepository: vi.fn(),
	updateRepository: vi.fn(),
	deleteRepository: vi.fn(),
	installRepoWebhook: vi.fn(),
	listRepositories: vi.fn()
};

// Mock the eager cache
export const mockEagerCache = {
	repositories: [] as any[],
	loaded: {
		repositories: false
	},
	loading: {
		repositories: false
	},
	errorMessages: {
		repositories: ''
	}
};

export const mockEagerCacheManager = {
	getRepositories: vi.fn(),
	retryResource: vi.fn()
};

// Mock the toast store
export const mockToastStore = {
	success: vi.fn(),
	error: vi.fn(),
	info: vi.fn(),
	warning: vi.fn()
};

// Setup common mocks
export function setupMocks() {
	vi.clearAllMocks();
	
	// Reset mock implementations
	mockGarmApi.createRepository.mockResolvedValue({ id: 'new-repo', name: 'new-repo', owner: 'test-owner' });
	mockGarmApi.updateRepository.mockResolvedValue({});
	mockGarmApi.deleteRepository.mockResolvedValue({});
	mockGarmApi.installRepoWebhook.mockResolvedValue({});
	mockEagerCacheManager.getRepositories.mockResolvedValue([]);
	mockEagerCacheManager.retryResource.mockResolvedValue({});
}