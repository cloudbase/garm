import type { Repository, Organization, Enterprise, Instance, Pool, ScaleSet, ForgeCredentials, ForgeEndpoint, Tag } from '$lib/api/generated/api.js';

export function createMockRepository(overrides: Partial<Repository> = {}): Repository {
	return {
		id: 'repo-123',
		name: 'test-repo',
		owner: 'test-owner',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z',
		credentials_name: 'test-credentials',
		credentials_id: 1,
		credentials: createMockCredentials(),
		endpoint: {
			name: 'github.com',
			endpoint_type: 'github',
			description: 'GitHub endpoint',
			api_base_url: 'https://api.github.com',
			base_url: 'https://github.com',
			upload_base_url: 'https://uploads.github.com',
			ca_cert_bundle: undefined,
			created_at: '2024-01-01T00:00:00Z',
			updated_at: '2024-01-01T00:00:00Z'
		},
		pool_manager_status: {
			running: true,
			failure_reason: undefined
		},
		...overrides
	};
}

export function createMockCredentials(overrides: Partial<ForgeCredentials> = {}): ForgeCredentials {
	return {
		id: Math.floor(Math.random() * 10000),
		name: 'test-credentials',
		description: 'Test credentials',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z',
		...overrides
	};
}

export function createMockGiteaRepository(overrides: Partial<Repository> = {}): Repository {
	return createMockRepository({
		endpoint: {
			name: 'gitea.example.com',
			endpoint_type: 'gitea',
			description: 'Gitea endpoint',
			api_base_url: 'https://gitea.example.com/api/v1',
			base_url: 'https://gitea.example.com',
			upload_base_url: undefined,
			ca_cert_bundle: undefined,
			created_at: '2024-01-01T00:00:00Z',
			updated_at: '2024-01-01T00:00:00Z'
		},
		...overrides
	});
}

export function createMockOrganization(overrides: Partial<Organization> = {}): Organization {
	return {
		id: 'org-123',
		name: 'test-org',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z',
		credentials_name: 'test-credentials',
		credentials_id: 1,
		credentials: createMockCredentials(),
		endpoint: {
			name: 'github.com',
			endpoint_type: 'github',
			description: 'GitHub endpoint',
			api_base_url: 'https://api.github.com',
			base_url: 'https://github.com',
			upload_base_url: 'https://uploads.github.com',
			ca_cert_bundle: undefined,
			created_at: '2024-01-01T00:00:00Z',
			updated_at: '2024-01-01T00:00:00Z'
		},
		pool_manager_status: {
			running: true,
			failure_reason: undefined
		},
		...overrides
	};
}

export function createMockGiteaOrganization(overrides: Partial<Organization> = {}): Organization {
	return createMockOrganization({
		endpoint: {
			name: 'gitea.example.com',
			endpoint_type: 'gitea',
			description: 'Gitea endpoint',
			api_base_url: 'https://gitea.example.com/api/v1',
			base_url: 'https://gitea.example.com',
			upload_base_url: undefined,
			ca_cert_bundle: undefined,
			created_at: '2024-01-01T00:00:00Z',
			updated_at: '2024-01-01T00:00:00Z'
		},
		...overrides
	});
}

export function createMockEnterprise(overrides: Partial<Enterprise> = {}): Enterprise {
	return {
		id: 'ent-123',
		name: 'test-enterprise',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z',
		credentials_name: 'test-credentials',
		credentials_id: 1,
		credentials: createMockCredentials(),
		endpoint: {
			name: 'github.com',
			endpoint_type: 'github',
			description: 'GitHub endpoint',
			api_base_url: 'https://api.github.com',
			base_url: 'https://github.com',
			upload_base_url: 'https://uploads.github.com',
			ca_cert_bundle: undefined,
			created_at: '2024-01-01T00:00:00Z',
			updated_at: '2024-01-01T00:00:00Z'
		},
		pool_manager_status: {
			running: true,
			failure_reason: undefined
		},
		...overrides
	};
}

export function createMockPool(overrides: Partial<Pool> = {}): Pool {
	return {
		id: 'pool-123',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z',
		enabled: true,
		image: 'ubuntu:22.04',
		flavor: 'default',
		max_runners: 10,
		min_idle_runners: 1,
		os_arch: 'amd64',
		os_type: 'linux',
		priority: 100,
		provider_name: 'test-provider',
		runner_bootstrap_timeout: 20,
		runner_prefix: 'garm',
		tags: [{ id: 'ubuntu', name: 'ubuntu' }, { id: 'test', name: 'test' }] as Tag[],
		repo_id: 'repo-123',
		...overrides
	};
}

export function createMockInstance(overrides: Partial<Instance> = {}): Instance {
	return {
		id: 'inst-123',
		name: 'test-instance',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z',
		agent_id: 12345,
		pool_id: 'pool-123',
		provider_id: 'prov-123',
		os_type: 'linux',
		os_name: 'ubuntu',
		os_arch: 'amd64',
		status: 'running',
		runner_status: 'idle',
		addresses: [
			{ address: '192.168.1.100', type: 'private' }
		],
		...overrides
	};
}

export function createMockForgeEndpoint(overrides: Partial<ForgeEndpoint> = {}): ForgeEndpoint {
	return {
		name: 'github.com',
		description: 'GitHub.com endpoint',
		endpoint_type: 'github',
		base_url: 'https://github.com',
		api_base_url: 'https://api.github.com',
		upload_base_url: 'https://uploads.github.com',
		ca_cert_bundle: undefined,
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z',
		...overrides
	};
}

export function createMockGiteaEndpoint(overrides: Partial<ForgeEndpoint> = {}): ForgeEndpoint {
	return createMockForgeEndpoint({
		name: 'gitea.example.com',
		description: 'Gitea endpoint',
		endpoint_type: 'gitea',
		base_url: 'https://gitea.example.com',
		api_base_url: 'https://gitea.example.com/api/v1',
		upload_base_url: undefined,
		...overrides
	});
}

export function createMockGithubCredentials(overrides: Partial<ForgeCredentials> = {}): ForgeCredentials {
	return createMockCredentials({
		forge_type: 'github',
		'auth-type': 'pat',
		endpoint: createMockForgeEndpoint(),
		...overrides
	});
}

export function createMockGiteaCredentials(overrides: Partial<ForgeCredentials> = {}): ForgeCredentials {
	return createMockCredentials({
		forge_type: 'gitea',
		'auth-type': 'pat',
		endpoint: createMockGiteaEndpoint(),
		...overrides
	});
}

export function createMockScaleSet(overrides: Partial<ScaleSet> = {}): ScaleSet {
	return {
		id: 123,
		name: 'test-scaleset',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z',
		enabled: true,
		image: 'ubuntu:22.04',
		flavor: 'default',
		max_runners: 10,
		min_idle_runners: 1,
		os_arch: 'amd64',
		os_type: 'linux',
		provider_name: 'test-provider',
		runner_bootstrap_timeout: 20,
		runner_prefix: 'garm',
		repo_id: 'repo-123',
		repo_name: 'test-repo',
		scale_set_id: 8,
		state: 'active',
		desired_runner_count: 5,
		disable_update: false,
		'github-runner-group': 'default',
		extra_specs: {},
		endpoint: {
			name: 'github.com',
			endpoint_type: 'github',
			description: 'GitHub endpoint',
			api_base_url: 'https://api.github.com',
			base_url: 'https://github.com',
			upload_base_url: 'https://uploads.github.com',
			ca_cert_bundle: undefined,
			created_at: '2024-01-01T00:00:00Z',
			updated_at: '2024-01-01T00:00:00Z'
		},
		instances: [],
		status_messages: [],
		...overrides
	};
}