import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/svelte';

// Unmock real components that setup.ts might mock
vi.unmock('$lib/components/PageHeader.svelte');
vi.unmock('$lib/components/DataTable.svelte');
vi.unmock('$lib/components/DeleteModal.svelte');
vi.unmock('$lib/components/cells');

vi.mock('$app/navigation', () => ({
	goto: vi.fn()
}));

vi.mock('$app/stores', () => ({}));

vi.mock('$lib/api/client.js', () => ({
	garmApi: {
		deleteTemplate: vi.fn(),
		restoreTemplates: vi.fn()
	}
}));

// Mock JWT utils to control admin state
vi.mock('$lib/utils/jwt', () => ({
	isCurrentUserAdmin: vi.fn(() => true)
}));

vi.mock('$lib/stores/eager-cache.js', () => ({
	eagerCache: {
		subscribe: vi.fn()
	},
	eagerCacheManager: {
		getTemplates: vi.fn(),
		retryResource: vi.fn()
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

vi.mock('$lib/stores/websocket.js', () => ({
	websocketStore: {
		subscribeToEntity: vi.fn(() => vi.fn())
	}
}));

import TemplatesPage from './+page.svelte';

// Template mock data
const mockTemplates = [
	{
		id: 1,
		name: 'linux-github-default',
		description: 'Default Linux template for GitHub',
		forge_type: 'github',
		os_type: 'linux',
		owner_id: 'system',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z',
		data: Array.from(new TextEncoder().encode('#!/bin/bash\necho hello'))
	},
	{
		id: 2,
		name: 'windows-github-default',
		description: 'Default Windows template for GitHub',
		forge_type: 'github',
		os_type: 'windows',
		owner_id: 'system',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z',
		data: Array.from(new TextEncoder().encode('Write-Host "hello"'))
	},
	{
		id: 3,
		name: 'linux-gitea-custom',
		description: 'Custom Gitea template',
		forge_type: 'gitea',
		os_type: 'linux',
		owner_id: 'user-123',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z',
		data: Array.from(new TextEncoder().encode('#!/bin/bash\necho custom'))
	}
];

describe('Templates Page - Integration Tests', () => {
	let eagerCacheManager: any;

	beforeEach(async () => {
		vi.clearAllMocks();

		const cacheModule = await import('$lib/stores/eager-cache.js');
		eagerCacheManager = cacheModule.eagerCacheManager;
		const eagerCache = cacheModule.eagerCache;

		vi.mocked(eagerCache.subscribe).mockImplementation((callback: any) => {
			callback({
				templates: mockTemplates,
				loaded: { templates: true },
				loading: { templates: false },
				errorMessages: { templates: '' }
			});
			return () => {};
		});

		vi.mocked(eagerCacheManager.getTemplates).mockResolvedValue(mockTemplates);
	});

	it('renders page title and description', async () => {
		render(TemplatesPage);

		await waitFor(() => {
			expect(screen.getByRole('heading', { name: 'Runner Install Templates' })).toBeInTheDocument();
		});

		await waitFor(() => {
			expect(
				screen.getByText(/Manage templates for configuring runner software installation/i)
			).toBeInTheDocument();
		});
	});

	it('shows "Create Template" button', async () => {
		render(TemplatesPage);

		await waitFor(() => {
			expect(screen.getByRole('button', { name: /Create Template/i })).toBeInTheDocument();
		});
	});

	it('renders template data in the table', async () => {
		render(TemplatesPage);

		await waitFor(() => {
			expect(screen.getAllByText('linux-github-default').length).toBeGreaterThan(0);
		});

		await waitFor(() => {
			expect(screen.getAllByText('windows-github-default').length).toBeGreaterThan(0);
			expect(screen.getAllByText('linux-gitea-custom').length).toBeGreaterThan(0);
		});
	});

	it('shows "Restore System Templates" button for admin users', async () => {
		render(TemplatesPage);

		await waitFor(() => {
			expect(screen.getByRole('button', { name: /Restore System Templates/i })).toBeInTheDocument();
		});
	});

	it('hides "Restore System Templates" button for non-admin users', async () => {
		const { isCurrentUserAdmin } = await import('$lib/utils/jwt');
		vi.mocked(isCurrentUserAdmin).mockReturnValue(false);

		render(TemplatesPage);

		await waitFor(() => {
			expect(screen.getByRole('heading', { name: 'Runner Install Templates' })).toBeInTheDocument();
		});

		expect(screen.queryByRole('button', { name: /Restore System Templates/i })).not.toBeInTheDocument();
	});

	it('handles search filtering', async () => {
		render(TemplatesPage);

		await waitFor(() => {
			expect(screen.getAllByText('linux-github-default').length).toBeGreaterThan(0);
		});

		const searchInput = screen.getByPlaceholderText(/Search templates by name, description, type/i);
		await fireEvent.input(searchInput, { target: { value: 'gitea' } });

		await waitFor(() => {
			expect(screen.getAllByText('linux-gitea-custom').length).toBeGreaterThan(0);
		});

		await waitFor(() => {
			expect(screen.queryByText('linux-github-default')).not.toBeInTheDocument();
			expect(screen.queryByText('windows-github-default')).not.toBeInTheDocument();
		});
	});

	it('handles empty templates list', async () => {
		const { eagerCache } = await import('$lib/stores/eager-cache.js');

		vi.mocked(eagerCache.subscribe).mockImplementation((callback: any) => {
			callback({
				templates: [],
				loaded: { templates: true },
				loading: { templates: false },
				errorMessages: { templates: '' }
			});
			return () => {};
		});
		vi.mocked(eagerCacheManager.getTemplates).mockResolvedValue([]);

		render(TemplatesPage);

		await waitFor(() => {
			expect(screen.getByText(/No templates found/i)).toBeInTheDocument();
		});
	});

	it('navigates to create page when "Create Template" is clicked', async () => {
		const { goto } = await import('$app/navigation');

		render(TemplatesPage);

		await waitFor(() => {
			expect(screen.getByRole('button', { name: /Create Template/i })).toBeInTheDocument();
		});

		const createButton = screen.getByRole('button', { name: /Create Template/i });
		await fireEvent.click(createButton);

		await waitFor(() => {
			expect(goto).toHaveBeenCalledWith('/templates/create');
		});
	});
});
