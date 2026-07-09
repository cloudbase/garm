import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/svelte';

// Unmock real components that setup.ts might mock
vi.unmock('$lib/components/PageHeader.svelte');
vi.unmock('$lib/components/DataTable.svelte');
vi.unmock('$lib/components/DeleteModal.svelte');
vi.unmock('$lib/components/Modal.svelte');
vi.unmock('$lib/components/Badge.svelte');
vi.unmock('$lib/components/cells');

vi.mock('$app/navigation', () => ({
	goto: vi.fn()
}));

vi.mock('$app/stores', () => ({}));

vi.mock('$lib/api/client.js', () => ({
	garmApi: {
		createProxy: vi.fn(),
		updateProxy: vi.fn(),
		deleteProxy: vi.fn()
	}
}));

vi.mock('$lib/stores/eager-cache.js', () => ({
	eagerCache: {
		subscribe: vi.fn()
	},
	eagerCacheManager: {
		getProxies: vi.fn(),
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

import ProxiesPage from './+page.svelte';

// Proxy mock data
const mockProxies = [
	{
		id: 1,
		name: 'corporate-proxy',
		description: 'Main corporate proxy',
		http_proxy: 'http://proxy.example.com:3128',
		https_proxy: 'http://proxy.example.com:3128',
		no_proxy: 'localhost,127.0.0.1',
		username: 'proxyuser',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z'
	},
	{
		id: 2,
		name: 'dmz-proxy',
		description: 'DMZ proxy without auth',
		http_proxy: 'http://dmz.example.com:8080',
		https_proxy: '',
		no_proxy: '',
		created_at: '2024-01-01T00:00:00Z',
		updated_at: '2024-01-01T00:00:00Z'
	}
];

function mockCacheState(proxies: any[]) {
	return {
		proxies,
		loaded: { proxies: true },
		loading: { proxies: false },
		errorMessages: { proxies: '' }
	};
}

describe('Proxies Page - Integration Tests', () => {
	let eagerCacheManager: any;

	beforeEach(async () => {
		vi.clearAllMocks();

		const cacheModule = await import('$lib/stores/eager-cache.js');
		eagerCacheManager = cacheModule.eagerCacheManager;
		const eagerCache = cacheModule.eagerCache;

		vi.mocked(eagerCache.subscribe).mockImplementation((callback: any) => {
			callback(mockCacheState(mockProxies));
			return () => {};
		});

		vi.mocked(eagerCacheManager.getProxies).mockResolvedValue(mockProxies);
	});

	it('renders page title and description', async () => {
		render(ProxiesPage);

		await waitFor(() => {
			expect(screen.getByRole('heading', { name: 'Proxies' })).toBeInTheDocument();
		});

		await waitFor(() => {
			expect(
				screen.getByText(/Manage proxy definitions runners can use/i)
			).toBeInTheDocument();
		});
	});

	it('shows "Create Proxy" button', async () => {
		render(ProxiesPage);

		await waitFor(() => {
			expect(screen.getByRole('button', { name: /Create Proxy/i })).toBeInTheDocument();
		});
	});

	it('renders proxy data in the table', async () => {
		render(ProxiesPage);

		await waitFor(() => {
			expect(screen.getAllByText('corporate-proxy').length).toBeGreaterThan(0);
		});

		await waitFor(() => {
			expect(screen.getAllByText('dmz-proxy').length).toBeGreaterThan(0);
			expect(screen.getAllByText('http://proxy.example.com:3128').length).toBeGreaterThan(0);
		});
	});

	it('shows authentication status for proxies', async () => {
		render(ProxiesPage);

		await waitFor(() => {
			// corporate-proxy has a username set, dmz-proxy does not
			expect(screen.getAllByText('Authenticated').length).toBeGreaterThan(0);
			expect(screen.getAllByText('No auth').length).toBeGreaterThan(0);
		});
	});

	it('handles search filtering', async () => {
		render(ProxiesPage);

		await waitFor(() => {
			expect(screen.getAllByText('corporate-proxy').length).toBeGreaterThan(0);
		});

		const searchInput = screen.getByPlaceholderText(/Search proxies by name, description or URL/i);
		await fireEvent.input(searchInput, { target: { value: 'dmz' } });

		await waitFor(() => {
			expect(screen.getAllByText('dmz-proxy').length).toBeGreaterThan(0);
		});

		await waitFor(() => {
			expect(screen.queryByText('corporate-proxy')).not.toBeInTheDocument();
		});
	});

	it('handles empty proxies list', async () => {
		const { eagerCache } = await import('$lib/stores/eager-cache.js');

		vi.mocked(eagerCache.subscribe).mockImplementation((callback: any) => {
			callback(mockCacheState([]));
			return () => {};
		});
		vi.mocked(eagerCacheManager.getProxies).mockResolvedValue([]);

		render(ProxiesPage);

		await waitFor(() => {
			expect(screen.getByText(/No proxies found/i)).toBeInTheDocument();
		});
	});

	it('opens the create modal with form fields when "Create Proxy" is clicked', async () => {
		render(ProxiesPage);

		await waitFor(() => {
			expect(screen.getByRole('button', { name: /Create Proxy/i })).toBeInTheDocument();
		});

		await fireEvent.click(screen.getByRole('button', { name: /Create Proxy/i }));

		await waitFor(() => {
			expect(screen.getByLabelText(/^Name/)).toBeInTheDocument();
			expect(screen.getByLabelText(/Description/i)).toBeInTheDocument();
			expect(screen.getByLabelText('HTTP Proxy')).toBeInTheDocument();
			expect(screen.getByLabelText('HTTPS Proxy')).toBeInTheDocument();
			expect(screen.getByLabelText(/No Proxy/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/Username/i)).toBeInTheDocument();
			expect(screen.getByLabelText(/Password/i)).toBeInTheDocument();
		});
	});

	it('creates a proxy when the create form is submitted', async () => {
		const { garmApi } = await import('$lib/api/client.js');
		vi.mocked(garmApi.createProxy).mockResolvedValue(mockProxies[0] as any);

		render(ProxiesPage);

		await waitFor(() => {
			expect(screen.getByRole('button', { name: /Create Proxy/i })).toBeInTheDocument();
		});

		await fireEvent.click(screen.getByRole('button', { name: /Create Proxy/i }));

		await waitFor(() => {
			expect(screen.getByLabelText(/^Name/)).toBeInTheDocument();
		});

		await fireEvent.input(screen.getByLabelText(/^Name/), { target: { value: 'new-proxy' } });
		await fireEvent.input(screen.getByLabelText('HTTP Proxy'), {
			target: { value: 'http://new.example.com:3128' }
		});

		// The submit button inside the modal form
		const submitButtons = screen.getAllByRole('button', { name: /^Create Proxy$/i });
		await fireEvent.click(submitButtons[submitButtons.length - 1]);

		await waitFor(() => {
			expect(garmApi.createProxy).toHaveBeenCalledWith(
				expect.objectContaining({
					name: 'new-proxy',
					http_proxy: 'http://new.example.com:3128'
				})
			);
		});
	});

	it('opens the edit modal prefilled and with a blank password', async () => {
		render(ProxiesPage);

		await waitFor(() => {
			expect(screen.getAllByText('corporate-proxy').length).toBeGreaterThan(0);
		});

		// Both mobile and desktop views render edit buttons; click the first one
		const editButtons = screen.getAllByRole('button', { name: /Edit/i });
		await fireEvent.click(editButtons[0]);

		await waitFor(() => {
			expect(screen.getByRole('heading', { name: /Edit Proxy corporate-proxy/i })).toBeInTheDocument();
			expect(screen.getByLabelText(/^Name/)).toHaveValue('corporate-proxy');
			expect(screen.getByLabelText(/Username/i)).toHaveValue('proxyuser');
			expect(screen.getByLabelText(/Password/i)).toHaveValue('');
			expect(
				screen.getByText(/Leaving the password blank keeps the current password/i)
			).toBeInTheDocument();
		});
	});

	it('deletes a proxy after confirmation', async () => {
		const { garmApi } = await import('$lib/api/client.js');
		vi.mocked(garmApi.deleteProxy).mockResolvedValue(undefined);

		render(ProxiesPage);

		await waitFor(() => {
			expect(screen.getAllByText('corporate-proxy').length).toBeGreaterThan(0);
		});

		const deleteButtons = screen.getAllByRole('button', { name: /Delete/i });
		await fireEvent.click(deleteButtons[0]);

		await waitFor(() => {
			expect(screen.getByText(/Are you sure you want to delete this proxy/i)).toBeInTheDocument();
		});

		const confirmButton = screen.getByRole('button', { name: /^Delete$/i });
		await fireEvent.click(confirmButton);

		await waitFor(() => {
			expect(garmApi.deleteProxy).toHaveBeenCalledWith(1);
		});
	});

	it('surfaces the API error when deletion is blocked', async () => {
		const { garmApi } = await import('$lib/api/client.js');
		const { toastStore } = await import('$lib/stores/toast.js');
		vi.mocked(garmApi.deleteProxy).mockRejectedValue(
			new Error('proxy is in use by one or more pools or scale sets')
		);

		render(ProxiesPage);

		await waitFor(() => {
			expect(screen.getAllByText('corporate-proxy').length).toBeGreaterThan(0);
		});

		const deleteButtons = screen.getAllByRole('button', { name: /Delete/i });
		await fireEvent.click(deleteButtons[0]);

		await waitFor(() => {
			expect(screen.getByText(/Are you sure you want to delete this proxy/i)).toBeInTheDocument();
		});

		const confirmButton = screen.getByRole('button', { name: /^Delete$/i });
		await fireEvent.click(confirmButton);

		await waitFor(() => {
			expect(garmApi.deleteProxy).toHaveBeenCalledWith(1);
			expect(toastStore.add).toHaveBeenCalledWith(
				expect.objectContaining({
					type: 'error',
					title: 'Failed to delete proxy'
				})
			);
		});
	});
});
