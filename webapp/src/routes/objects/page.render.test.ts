import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/svelte';
import ObjectsPage from './+page.svelte';
import { createMockFileObject } from '../../test/factories.js';

// Only mock the GARM API - nothing else
vi.mock('$lib/api/client.js', () => ({
	garmApi: {
		listFileObjects: vi.fn(),
		deleteFileObject: vi.fn(),
		updateFileObject: vi.fn()
	}
}));

vi.mock('$lib/stores/toast.js', () => ({
	toastStore: {
		add: vi.fn()
	}
}));

vi.mock('$lib/stores/websocket.js', () => ({
	websocketStore: {
		subscribe: vi.fn(() => () => {}),
		subscribeToEntity: vi.fn(() => () => {})
	}
}));

// Mock app stores
vi.mock('$app/stores', () => ({}));
vi.mock('$app/navigation', () => ({
	goto: vi.fn()
}));
vi.mock('$app/paths', () => ({
	resolve: vi.fn((path: string) => path)
}));

const mockObject = createMockFileObject({
	id: 1,
	name: 'test-file.bin',
	size: 1024000,
	tags: ['test', 'binary']
});

describe('Objects Page - Render Tests', () => {
	beforeEach(async () => {
		vi.clearAllMocks();

		const { garmApi } = await import('$lib/api/client.js');
		(garmApi.listFileObjects as any).mockResolvedValue({
			results: [mockObject],
			pages: 1,
			total_count: 1
		});
	});

	describe('Basic Rendering', () => {
		it('should render without crashing', () => {
			const { container } = render(ObjectsPage);
			expect(container).toBeInTheDocument();
		});

		it('should render page header with correct title', async () => {
			const { getByRole, getByText } = render(ObjectsPage);

			await waitFor(() => {
				expect(getByRole('heading', { name: 'Object Storage' })).toBeInTheDocument();
			});
			expect(getByText(/Manage files stored in GARM/i)).toBeInTheDocument();
		});

		it('should render upload button', async () => {
			render(ObjectsPage);

			await waitFor(() => {
				expect(screen.getByText('Upload New Object')).toBeInTheDocument();
			});
		});
	});

	describe('Search Functionality', () => {
		it('should render search input', async () => {
			render(ObjectsPage);

			await waitFor(() => {
				expect(screen.getByPlaceholderText(/Search by name or tags/i)).toBeInTheDocument();
			});
		});
	});

	describe('Data Display', () => {
		it('should call API to load objects on mount', async () => {
			const { garmApi } = await import('$lib/api/client.js');

			render(ObjectsPage);
			await new Promise(resolve => setTimeout(resolve, 0));

			expect(garmApi.listFileObjects).toHaveBeenCalled();
		});

		it('should display object name in table', async () => {
			render(ObjectsPage);

			await waitFor(() => {
				expect(screen.getByText('test-file.bin')).toBeInTheDocument();
			});
		});

		it('should show empty state when no objects', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			(garmApi.listFileObjects as any).mockResolvedValue({
				results: [],
				pages: 0,
				total_count: 0
			});

			render(ObjectsPage);

			await waitFor(() => {
				expect(screen.getByText('No objects found')).toBeInTheDocument();
			});
		});
	});

	describe('Modals', () => {
		it('should not show upload modal initially', async () => {
			render(ObjectsPage);

			await waitFor(() => {
				expect(screen.getByText('Upload New Object')).toBeInTheDocument(); // button
			});
			expect(screen.queryByLabelText('File Name')).not.toBeInTheDocument(); // modal input
		});

		it('should not show update modal initially', () => {
			render(ObjectsPage);

			expect(screen.queryByText(/Update Object/i)).not.toBeInTheDocument();
		});

		it('should not show delete modal initially', () => {
			render(ObjectsPage);

			expect(screen.queryByText(/Delete Object/i)).not.toBeInTheDocument();
		});
	});

	describe('Error Handling', () => {
		it('should handle API errors gracefully', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			(garmApi.listFileObjects as any).mockRejectedValue(new Error('API Error'));

			const { container } = render(ObjectsPage);
			await new Promise(resolve => setTimeout(resolve, 100));

			// Should still render the container without crashing
			expect(container).toBeInTheDocument();
		});
	});

	describe('What\'s This Description', () => {
		it('should render expandable description button', async () => {
			render(ObjectsPage);
			await new Promise(resolve => setTimeout(resolve, 0));

			expect(screen.getByText("What's this?")).toBeInTheDocument();
		});

		it('should not show description text initially', () => {
			render(ObjectsPage);

			expect(screen.queryByText(/primary goal of this is/i)).not.toBeInTheDocument();
		});
	});
});
