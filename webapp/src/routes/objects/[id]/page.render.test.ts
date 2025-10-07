import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import ObjectDetailPage from './+page.svelte';
import { createMockFileObject } from '../../../test/factories.js';

// Only mock the GARM API
vi.mock('$lib/api/client.js', () => ({
	garmApi: {
		getFileObject: vi.fn(),
		deleteFileObject: vi.fn()
	}
}));

vi.mock('$lib/stores/toast.js', () => ({
	toastStore: {
		add: vi.fn()
	}
}));

vi.mock('$app/stores', () => ({
	page: {
		subscribe: vi.fn((callback) => {
			callback({ params: { id: '1' } });
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

const mockObject = createMockFileObject({
	id: 1,
	name: 'test-file.bin',
	size: 1024000,
	sha256: 'a'.repeat(64),
	file_type: 'application/octet-stream',
	tags: ['test', 'binary', 'linux']
});

describe('Object Detail Page - Render Tests', () => {
	beforeEach(async () => {
		vi.clearAllMocks();

		const { garmApi } = await import('$lib/api/client.js');
		(garmApi.getFileObject as any).mockResolvedValue(mockObject);
	});

	describe('Basic Rendering', () => {
		it('should render without crashing', () => {
			const { container } = render(ObjectDetailPage);
			expect(container).toBeInTheDocument();
		});

		it('should set page title', async () => {
			render(ObjectDetailPage);
			await new Promise(resolve => setTimeout(resolve, 100));

			expect(document.title).toBe('test-file.bin - GARM');
		});

		it('should render page header', async () => {
			render(ObjectDetailPage);
			await new Promise(resolve => setTimeout(resolve, 100));

			expect(screen.getByRole('heading', { name: 'test-file.bin' })).toBeInTheDocument();
		});
	});

	describe('Object Details Display', () => {
		it('should call API to load object on mount', async () => {
			const { garmApi } = await import('$lib/api/client.js');

			render(ObjectDetailPage);
			await new Promise(resolve => setTimeout(resolve, 0));

			expect(garmApi.getFileObject).toHaveBeenCalledWith('1');
		});

		it('should display object ID', async () => {
			render(ObjectDetailPage);
			await new Promise(resolve => setTimeout(resolve, 100));

			expect(screen.getByText('1')).toBeInTheDocument();
		});

		it('should display object name', async () => {
			render(ObjectDetailPage);
			await new Promise(resolve => setTimeout(resolve, 100));

			expect(screen.getByText('test-file.bin')).toBeInTheDocument();
		});

		it('should display formatted file size', async () => {
			render(ObjectDetailPage);
			await new Promise(resolve => setTimeout(resolve, 100));

			// 1024000 bytes = 1000.0 KB
			expect(screen.getByText(/1000\.0 KB/i)).toBeInTheDocument();
		});

		it('should display SHA256 hash', async () => {
			render(ObjectDetailPage);
			await new Promise(resolve => setTimeout(resolve, 100));

			expect(screen.getByText('a'.repeat(64))).toBeInTheDocument();
		});

		it('should display file type', async () => {
			render(ObjectDetailPage);
			await new Promise(resolve => setTimeout(resolve, 100));

			expect(screen.getByText('application/octet-stream')).toBeInTheDocument();
		});

		it('should display tags', async () => {
			render(ObjectDetailPage);
			await new Promise(resolve => setTimeout(resolve, 100));

			expect(screen.getByText('test')).toBeInTheDocument();
			expect(screen.getByText('binary')).toBeInTheDocument();
			expect(screen.getByText('linux')).toBeInTheDocument();
		});
	});

	describe('Action Buttons', () => {
		it('should render back button', async () => {
			render(ObjectDetailPage);
			await new Promise(resolve => setTimeout(resolve, 100));

			expect(screen.getByText('Back')).toBeInTheDocument();
		});

		it('should render download button', async () => {
			render(ObjectDetailPage);
			await new Promise(resolve => setTimeout(resolve, 100));

			expect(screen.getByRole('button', { name: 'Download' })).toBeInTheDocument();
		});

		it('should render delete button', async () => {
			render(ObjectDetailPage);
			await new Promise(resolve => setTimeout(resolve, 100));

			expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument();
		});
	});

	describe('Loading State', () => {
		it('should show loading state initially', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			(garmApi.getFileObject as any).mockImplementation(() => new Promise(() => {})); // Never resolves

			render(ObjectDetailPage);

			expect(screen.getByText(/Loading/i)).toBeInTheDocument();
		});
	});

	describe('Error Handling', () => {
		it('should handle API errors gracefully', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			(garmApi.getFileObject as any).mockRejectedValue(new Error('API Error'));

			render(ObjectDetailPage);
			await new Promise(resolve => setTimeout(resolve, 100));

			// Should show error message
			expect(screen.getByText(/Failed to load object/i)).toBeInTheDocument();
		});

		it('should handle invalid object ID', async () => {
			const { page } = await import('$app/stores');
			vi.mocked(page.subscribe).mockImplementation((callback) => {
				callback({ params: { id: '' } } as any);
				return () => {};
			});

			render(ObjectDetailPage);
			await new Promise(resolve => setTimeout(resolve, 100));

			// Should show error
			expect(screen.getByText(/Invalid object ID/i)).toBeInTheDocument();
		});
	});

	describe('Delete Modal', () => {
		it('should not show delete modal initially', async () => {
			render(ObjectDetailPage);
			await new Promise(resolve => setTimeout(resolve, 100));

			expect(screen.queryByText(/Are you sure you want to delete/i)).not.toBeInTheDocument();
		});
	});
});
