import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/svelte';
import '@testing-library/jest-dom';
import { createMockGithubCredentials, createMockGiteaCredentials, createMockForgeEndpoint, createMockGiteaEndpoint, createMockRepository, createMockOrganization, createMockEnterprise } from '../../../../test/factories.js';

vi.unmock('$lib/components/DataTable.svelte');
vi.unmock('$lib/components/DeleteModal.svelte');
vi.unmock('$lib/components/cells');

// Mock $app/stores - page is a Svelte store with subscribe
let mockPageParams = { forge_type: 'github', id: '1001' };
vi.mock('$app/stores', () => ({
	page: {
		subscribe: vi.fn((callback) => {
			callback({
				params: mockPageParams,
				url: new URL(`http://localhost/credentials/${mockPageParams.forge_type}/${mockPageParams.id}`)
			});
			return () => {};
		})
	}
}));

vi.mock('$app/navigation', () => ({
	goto: vi.fn()
}));

vi.mock('$app/paths', () => ({
	resolve: vi.fn((path: string) => path)
}));

vi.mock('$lib/api/client.js', () => ({
	garmApi: {
		getGithubCredentials: vi.fn(),
		getGiteaCredentials: vi.fn(),
		deleteGithubCredentials: vi.fn(),
		deleteGiteaCredentials: vi.fn()
	}
}));

vi.mock('$lib/stores/websocket.js', () => ({
	websocketStore: {
		subscribeToEntity: vi.fn(() => vi.fn())
	}
}));

vi.mock('$lib/stores/toast.js', () => ({
	toastStore: {
		success: vi.fn(),
		error: vi.fn(),
		info: vi.fn(),
		warning: vi.fn(),
		add: vi.fn()
	}
}));

import CredentialDetailPage from './+page.svelte';

const mockGithubCredential = createMockGithubCredentials({
	id: 1001,
	name: 'github-test-creds',
	description: 'Test GitHub credentials',
	forge_type: 'github',
	'auth-type': 'pat',
	base_url: 'https://github.com',
	api_base_url: 'https://api.github.com',
	upload_base_url: 'https://uploads.github.com',
	endpoint: createMockForgeEndpoint(),
	repositories: [
		createMockRepository({ id: 'repo-1', name: 'test-repo', owner: 'test-owner' })
	],
	organizations: [
		createMockOrganization({ id: 'org-1', name: 'test-org' })
	],
	enterprises: [
		createMockEnterprise({ id: 'ent-1', name: 'test-enterprise' })
	]
});

describe('Credential Detail Page Integration Tests', () => {
	beforeEach(async () => {
		vi.clearAllMocks();
		mockPageParams = { forge_type: 'github', id: '1001' };
		const { garmApi } = await import('$lib/api/client.js');
		(garmApi.getGithubCredentials as any).mockResolvedValue(mockGithubCredential);
		(garmApi.getGiteaCredentials as any).mockResolvedValue(null);
	});

	it('should show loading state initially before data loads', async () => {
		const { garmApi } = await import('$lib/api/client.js');
		(garmApi.getGithubCredentials as any).mockImplementation(
			() => new Promise((resolve) => setTimeout(() => resolve(mockGithubCredential), 200))
		);

		const { container } = render(CredentialDetailPage);

		// While the API call is pending, the loading spinner should be visible
		const spinner = container.querySelector('.animate-spin');
		expect(spinner).toBeInTheDocument();

		// After loading resolves, the credential name should appear
		await waitFor(() => {
			const allNames = screen.getAllByText('github-test-creds');
			expect(allNames.length).toBeGreaterThan(0);
		}, { timeout: 1000 });
	});

	it('should render GitHub credential details after loading', async () => {
		render(CredentialDetailPage);

		await waitFor(() => {
			// Credential name appears in the header
			const allNames = screen.getAllByText('github-test-creds');
			expect(allNames.length).toBeGreaterThan(0);
		});

		await waitFor(() => {
			// Forge type shown in the subtitle (capitalize CSS, DOM text is lowercase)
			const forgeTexts = screen.getAllByText(/github/i);
			expect(forgeTexts.length).toBeGreaterThan(0);

			// Auth type displayed (uppercase CSS class applied, DOM text is 'pat')
			const authTexts = screen.getAllByText('pat');
			expect(authTexts.length).toBeGreaterThan(0);

			// Delete button is present
			expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
		});
	});

	it('should show breadcrumb with Credentials link', async () => {
		render(CredentialDetailPage);

		// Breadcrumb should be present immediately
		expect(screen.getByText('Credentials')).toBeInTheDocument();

		// The "Credentials" breadcrumb should be a link to /credentials
		const credentialsLink = screen.getByText('Credentials').closest('a');
		expect(credentialsLink).toHaveAttribute('href', '/credentials');
	});

	it('should show Credential Information section with key fields', async () => {
		const { container } = render(CredentialDetailPage);

		await waitFor(() => {
			expect(screen.getByText('Credential Information')).toBeInTheDocument();
		});

		await waitFor(() => {
			// ID field - rendered as number text
			const allIds = screen.getAllByText('1001');
			expect(allIds.length).toBeGreaterThan(0);

			// Name field - appears in header and info section
			const allNames = screen.getAllByText('github-test-creds');
			expect(allNames.length).toBeGreaterThan(0);

			// Description field - appears in header subtitle and info section
			const allDescs = screen.getAllByText('Test GitHub credentials');
			expect(allDescs.length).toBeGreaterThan(0);

			// Base URL field - may appear in both credential and endpoint sections
			const allBaseUrls = screen.getAllByText('https://github.com');
			expect(allBaseUrls.length).toBeGreaterThan(0);

			// API Base URL field - may appear in both credential and endpoint sections
			const allApiUrls = screen.getAllByText('https://api.github.com');
			expect(allApiUrls.length).toBeGreaterThan(0);

			// Upload Base URL (GitHub only) - may appear in both credential and endpoint sections
			const allUploadUrls = screen.getAllByText('https://uploads.github.com');
			expect(allUploadUrls.length).toBeGreaterThan(0);
		});
	});

	it('should show Endpoint Information section when credential has an endpoint', async () => {
		render(CredentialDetailPage);

		await waitFor(() => {
			expect(screen.getByText('Endpoint Information')).toBeInTheDocument();
		});

		await waitFor(() => {
			// Endpoint name from createMockForgeEndpoint defaults
			const endpointNames = screen.getAllByText('github.com');
			expect(endpointNames.length).toBeGreaterThan(0);

			// Endpoint description
			const endpointDescs = screen.getAllByText('GitHub.com endpoint');
			expect(endpointDescs.length).toBeGreaterThan(0);
		});
	});

	it('should show Repositories section when credential has repositories', async () => {
		render(CredentialDetailPage);

		await waitFor(() => {
			expect(screen.getByText('Repositories')).toBeInTheDocument();
			expect(screen.getByText('Repositories using this credential')).toBeInTheDocument();
		});

		await waitFor(() => {
			// The repository name should appear in the DataTable
			// DataTable renders both mobile card and desktop table views
			const repoNames = screen.getAllByText(/test-repo/);
			expect(repoNames.length).toBeGreaterThan(0);
		});
	});

	it('should call the correct API based on forge_type and subscribe to websocket events', async () => {
		render(CredentialDetailPage);

		const { garmApi } = await import('$lib/api/client.js');
		const { websocketStore } = await import('$lib/stores/websocket.js');

		await waitFor(() => {
			// Should call getGithubCredentials with the numeric ID
			expect(garmApi.getGithubCredentials).toHaveBeenCalledWith(1001);
			// Should NOT call getGiteaCredentials for a GitHub credential
			expect(garmApi.getGiteaCredentials).not.toHaveBeenCalled();
		});

		// Should subscribe to both github and gitea credential delete events
		expect(websocketStore.subscribeToEntity).toHaveBeenCalledWith(
			'github_credentials',
			['delete'],
			expect.any(Function)
		);
		expect(websocketStore.subscribeToEntity).toHaveBeenCalledWith(
			'gitea_credentials',
			['delete'],
			expect.any(Function)
		);
	});

	it('should handle error when credential loading fails', async () => {
		const { garmApi } = await import('$lib/api/client.js');
		(garmApi.getGithubCredentials as any).mockRejectedValue(new Error('Credential not found'));

		const { container } = render(CredentialDetailPage);

		await waitFor(() => {
			// Error heading should appear
			expect(screen.getByText('Error loading credential')).toBeInTheDocument();
		});

		await waitFor(() => {
			// The error container should be styled with the red error background
			const errorDiv = container.querySelector('.bg-red-50');
			expect(errorDiv).toBeInTheDocument();
		});
	});
});
