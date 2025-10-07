import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
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
	size: 2048,
	tags: ['test', 'binary']
});

describe('Object Detail Page - Integration Tests', () => {
	beforeEach(async () => {
		vi.clearAllMocks();

		const { garmApi } = await import('$lib/api/client.js');
		(garmApi.getFileObject as any).mockResolvedValue(mockObject);
	});

	describe('Navigation', () => {
		it('should navigate back when back button is clicked', async () => {
			const { goto } = await import('$app/navigation');

			render(ObjectDetailPage);
			await waitFor(() => screen.getByText('test-file.bin'));

			const backButton = screen.getByText('Back');
			await fireEvent.click(backButton);

			expect(goto).toHaveBeenCalledWith('/objects');
		});
	});

	describe('Delete Functionality', () => {
		it('should open delete modal when delete button is clicked', async () => {
			render(ObjectDetailPage);
			await waitFor(() => screen.getByText('test-file.bin'));

			const deleteButton = screen.getByRole('button', { name: 'Delete' });
			await fireEvent.click(deleteButton);

			// Delete modal should appear
			await waitFor(() => {
				expect(screen.getByText(/Are you sure you want to delete/i)).toBeInTheDocument();
			});
		});

		it('should close delete modal when cancel is clicked', async () => {
			render(ObjectDetailPage);
			await waitFor(() => screen.getByText('test-file.bin'));

			// Open delete modal
			const deleteButton = screen.getByRole('button', { name: 'Delete' });
			await fireEvent.click(deleteButton);
			await waitFor(() => screen.getByText(/Are you sure/i));

			// Cancel deletion
			const cancelButton = screen.getByRole('button', { name: 'Cancel' });
			await fireEvent.click(cancelButton);

			await waitFor(() => {
				expect(screen.queryByText(/Are you sure/i)).not.toBeInTheDocument();
			});
		});

		it('should call delete API when confirmed', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			(garmApi.deleteFileObject as any).mockResolvedValue(undefined);

			render(ObjectDetailPage);
			await waitFor(() => screen.getByText('test-file.bin'));

			// Open delete modal
			const deleteButton = screen.getByRole('button', { name: 'Delete' });
			await fireEvent.click(deleteButton);
			await waitFor(() => screen.getByText(/Are you sure/i));

			// Confirm deletion
			const confirmButton = screen.getByRole('button', { name: 'Confirm' });
			await fireEvent.click(confirmButton);

			// Should call delete API
			await waitFor(() => {
				expect(garmApi.deleteFileObject).toHaveBeenCalledWith('1');
			});
		});

		it('should navigate to list page after successful deletion', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			const { goto } = await import('$app/navigation');

			(garmApi.deleteFileObject as any).mockResolvedValue(undefined);

			render(ObjectDetailPage);
			await waitFor(() => screen.getByText('test-file.bin'));

			// Delete object
			const deleteButton = screen.getByRole('button', { name: 'Delete' });
			await fireEvent.click(deleteButton);
			await waitFor(() => screen.getByText(/Are you sure/i));

			const confirmButton = screen.getByRole('button', { name: 'Confirm' });
			await fireEvent.click(confirmButton);

			// Should navigate to list
			await waitFor(() => {
				expect(goto).toHaveBeenCalledWith('/objects');
			});
		});

		it('should show success toast after deletion', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			const { toastStore } = await import('$lib/stores/toast.js');

			(garmApi.deleteFileObject as any).mockResolvedValue(undefined);

			render(ObjectDetailPage);
			await waitFor(() => screen.getByText('test-file.bin'));

			// Delete object
			const deleteButton = screen.getByRole('button', { name: 'Delete' });
			await fireEvent.click(deleteButton);
			await waitFor(() => screen.getByText(/Are you sure/i));

			const confirmButton = screen.getByRole('button', { name: 'Confirm' });
			await fireEvent.click(confirmButton);

			// Should show success toast
			await waitFor(() => {
				expect(toastStore.add).toHaveBeenCalledWith(
					expect.objectContaining({
						type: 'success',
						title: 'Object deleted'
					})
				);
			});
		});

		it('should show error toast on delete failure', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			const { toastStore } = await import('$lib/stores/toast.js');

			(garmApi.deleteFileObject as any).mockRejectedValue(new Error('Delete failed'));

			render(ObjectDetailPage);
			await waitFor(() => screen.getByText('test-file.bin'));

			// Try to delete
			const deleteButton = screen.getByRole('button', { name: 'Delete' });
			await fireEvent.click(deleteButton);
			await waitFor(() => screen.getByText(/Are you sure/i));

			const confirmButton = screen.getByRole('button', { name: 'Confirm' });
			await fireEvent.click(confirmButton);

			// Should show error toast
			await waitFor(() => {
				expect(toastStore.add).toHaveBeenCalledWith(
					expect.objectContaining({
						type: 'error',
						title: 'Failed to delete object'
					})
				);
			});
		});
	});

	describe('Download Functionality', () => {
		it('should trigger download when download button is clicked', async () => {
			global.fetch = vi.fn().mockResolvedValue({
				ok: true,
				blob: () => Promise.resolve(new Blob(['test content']))
			});

			const createElementSpy = vi.spyOn(document, 'createElement');
			const appendChildSpy = vi.spyOn(document.body, 'appendChild');
			const removeChildSpy = vi.spyOn(document.body, 'removeChild');

			render(ObjectDetailPage);
			await waitFor(() => screen.getByText('test-file.bin'));

			const downloadButton = screen.getByRole('button', { name: 'Download' });
			await fireEvent.click(downloadButton);

			// Should create temporary link and trigger download
			await waitFor(() => {
				expect(createElementSpy).toHaveBeenCalledWith('a');
			});
		});

		it('should show success toast on download start', async () => {
			const { toastStore } = await import('$lib/stores/toast.js');

			global.fetch = vi.fn().mockResolvedValue({
				ok: true,
				blob: () => Promise.resolve(new Blob(['test content']))
			});

			render(ObjectDetailPage);
			await waitFor(() => screen.getByText('test-file.bin'));

			const downloadButton = screen.getByRole('button', { name: 'Download' });
			await fireEvent.click(downloadButton);

			await waitFor(() => {
				expect(toastStore.add).toHaveBeenCalledWith(
					expect.objectContaining({
						type: 'success',
						title: 'Download started'
					})
				);
			});
		});

		it('should show error toast on download failure', async () => {
			const { toastStore } = await import('$lib/stores/toast.js');

			global.fetch = vi.fn().mockResolvedValue({
				ok: false,
				status: 500
			});

			render(ObjectDetailPage);
			await waitFor(() => screen.getByText('test-file.bin'));

			const downloadButton = screen.getByRole('button', { name: 'Download' });
			await fireEvent.click(downloadButton);

			await waitFor(() => {
				expect(toastStore.add).toHaveBeenCalledWith(
					expect.objectContaining({
						type: 'error',
						title: 'Download failed'
					})
				);
			});
		});
	});

	describe('Data Loading', () => {
		it('should reload object data if it changes', async () => {
			const { garmApi } = await import('$lib/api/client.js');

			render(ObjectDetailPage);
			await waitFor(() => screen.getByText('test-file.bin'));

			// Initially loaded once
			expect(garmApi.getFileObject).toHaveBeenCalledTimes(1);
		});
	});
});
