import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import ObjectsPage from './+page.svelte';
import { createMockFileObject } from '../../test/factories.js';

// Only mock the GARM API
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

vi.mock('$app/stores', () => ({}));
vi.mock('$app/navigation', () => ({
	goto: vi.fn()
}));
vi.mock('$app/paths', () => ({
	resolve: vi.fn((path: string) => path)
}));

vi.mock('$lib/utils/format', () => ({
	formatFileSize: vi.fn((size) => `${(size / 1024).toFixed(1)} KB`),
	formatDateTime: vi.fn((date) => date || 'N/A')
}));

const mockObject1 = createMockFileObject({
	id: 1,
	name: 'file1.bin',
	size: 1024,
	tags: ['binary', 'linux']
});

const mockObject2 = createMockFileObject({
	id: 2,
	name: 'file2.txt',
	size: 2048,
	tags: ['text', 'config']
});

describe('Objects Page - Integration Tests', () => {
	beforeEach(async () => {
		vi.clearAllMocks();

		const { garmApi } = await import('$lib/api/client.js');
		(garmApi.listFileObjects as any).mockResolvedValue({
			results: [mockObject1, mockObject2],
			pages: 1,
			total_count: 2
		});
	});

	describe('Search Functionality', () => {
		it('should search objects when search term is entered', async () => {
			const { garmApi } = await import('$lib/api/client.js');

			render(ObjectsPage);
			await waitFor(() => expect(garmApi.listFileObjects).toHaveBeenCalledTimes(1));

			const searchInput = screen.getByPlaceholderText(/Search by name or tags/i);

			await fireEvent.input(searchInput, { target: { value: 'binary linux' } });

			// Should call API with comma-separated tags after debounce (500ms)
			await waitFor(() => {
				expect(garmApi.listFileObjects).toHaveBeenCalledWith('binary,linux', 1, 25);
			}, { timeout: 1000 });
		});

		it('should search when Enter key is pressed', async () => {
			const { garmApi } = await import('$lib/api/client.js');

			render(ObjectsPage);
			await waitFor(() => expect(garmApi.listFileObjects).toHaveBeenCalledTimes(1));

			const searchInput = screen.getByPlaceholderText(/Search by name or tags/i);

			await fireEvent.input(searchInput, { target: { value: 'test' } });
			await fireEvent.keyDown(searchInput, { key: 'Enter' });

			await waitFor(() => {
				expect(garmApi.listFileObjects).toHaveBeenCalledWith('test', 1, 25);
			}, { timeout: 1000 });
		});

		it('should reset to page 1 when searching', async () => {
			const { garmApi } = await import('$lib/api/client.js');

			render(ObjectsPage);
			await waitFor(() => expect(garmApi.listFileObjects).toHaveBeenCalled());

			const searchInput = screen.getByPlaceholderText(/Search by name or tags/i);

			await fireEvent.input(searchInput, { target: { value: 'test' } });

			// Should call with page 1 after debounce
			await waitFor(() => {
				const calls = (garmApi.listFileObjects as any).mock.calls;
				const lastCall = calls[calls.length - 1];
				expect(lastCall[1]).toBe(1); // page parameter
			}, { timeout: 1000 });
		});
	});

	describe('Upload Modal', () => {
		it('should open upload modal when button is clicked', async () => {
			render(ObjectsPage);
			await waitFor(() => screen.getByText('Upload New Object'));

			const uploadButton = screen.getByRole('button', { name: 'Upload New Object' });
			await fireEvent.click(uploadButton);

			// Modal should open
			await waitFor(() => {
				expect(screen.getByLabelText('File Name')).toBeInTheDocument();
			});
		});

		it('should close upload modal when cancel is clicked', async () => {
			render(ObjectsPage);
			await waitFor(() => screen.getByText('Upload New Object'));

			// Open modal
			const uploadButton = screen.getByRole('button', { name: 'Upload New Object' });
			await fireEvent.click(uploadButton);
			await waitFor(() => screen.getByLabelText('File Name'));

			// Close modal
			const cancelButton = screen.getByRole('button', { name: 'Cancel' });
			await fireEvent.click(cancelButton);

			await waitFor(() => {
				expect(screen.queryByLabelText('File Name')).not.toBeInTheDocument();
			});
		});
	});

	describe('Delete Functionality', () => {
		it('should show delete modal when delete action is clicked', async () => {
			render(ObjectsPage);
			await waitFor(() => screen.getByText('file1.bin'));

			// Find and click delete button
			const deleteButtons = screen.getAllByRole('button', { name: 'Delete object' });
			await fireEvent.click(deleteButtons[0]);

			// Delete modal should appear
			await waitFor(() => {
				expect(screen.getByText(/Are you sure you want to delete/i)).toBeInTheDocument();
			});
		});

		it('should call delete API when confirmed', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			(garmApi.deleteFileObject as any).mockResolvedValue(undefined);

			render(ObjectsPage);
			await waitFor(() => screen.getByText('file1.bin'));

			// Click delete button
			const deleteButtons = screen.getAllByRole('button', { name: 'Delete object' });
			await fireEvent.click(deleteButtons[0]);

			// Confirm deletion
			await waitFor(() => screen.getByText(/Are you sure/i));
			const confirmButton = screen.getByRole('button', { name: 'Confirm' });
			await fireEvent.click(confirmButton);

			// Should call delete API
			await waitFor(() => {
				expect(garmApi.deleteFileObject).toHaveBeenCalledWith('1');
			});
		});

		it('should reload objects after successful deletion', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			(garmApi.deleteFileObject as any).mockResolvedValue(undefined);

			render(ObjectsPage);
			await waitFor(() => screen.getByText('file1.bin'));

			const initialCallCount = (garmApi.listFileObjects as any).mock.calls.length;

			// Delete object
			const deleteButtons = screen.getAllByRole('button', { name: 'Delete object' });
			await fireEvent.click(deleteButtons[0]);
			await waitFor(() => screen.getByText(/Are you sure/i));

			const confirmButton = screen.getByRole('button', { name: 'Confirm' });
			await fireEvent.click(confirmButton);

			// Should reload list
			await waitFor(() => {
				expect((garmApi.listFileObjects as any).mock.calls.length).toBeGreaterThan(initialCallCount);
			});
		});
	});

	describe('Update Functionality', () => {
		it('should open update modal when update button is clicked', async () => {
			render(ObjectsPage);
			await waitFor(() => screen.getByText('file1.bin'));

			// Click update button
			const updateButtons = screen.getAllByRole('button', { name: 'Update object' });
			await fireEvent.click(updateButtons[0]);

			// Update modal should appear
			await waitFor(() => {
				expect(screen.getByText('Update Object')).toBeInTheDocument();
			});
		});

		it('should populate form with current object data', async () => {
			render(ObjectsPage);
			await waitFor(() => screen.getByText('file1.bin'));

			// Click update button for first object
			const updateButtons = screen.getAllByRole('button', { name: 'Update object' });
			await fireEvent.click(updateButtons[0]);

			await waitFor(() => {
				const nameInput = screen.getByLabelText('File Name') as HTMLInputElement;
				expect(nameInput.value).toBe('file1.bin');
			});
		});

		it('should call update API when form is submitted', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			(garmApi.updateFileObject as any).mockResolvedValue(mockObject1);

			render(ObjectsPage);
			await waitFor(() => screen.getByText('file1.bin'));

			// Open update modal
			const updateButtons = screen.getAllByRole('button', { name: 'Update object' });
			await fireEvent.click(updateButtons[0]);
			await waitFor(() => screen.getByText('Update Object'));

			// Submit form
			const submitButton = screen.getByRole('button', { name: 'Update' });
			await fireEvent.click(submitButton);

			// Should call update API
			await waitFor(() => {
				expect(garmApi.updateFileObject).toHaveBeenCalledWith('1', expect.any(Object));
			});
		});
	});

	describe('Pagination', () => {
		it('should pass page size to API', async () => {
			const { garmApi } = await import('$lib/api/client.js');

			render(ObjectsPage);

			await waitFor(() => {
				const calls = (garmApi.listFileObjects as any).mock.calls;
				const lastCall = calls[calls.length - 1];
				expect(lastCall[2]).toBe(25); // pageSize parameter
			});
		});
	});

	describe('What\'s This Description', () => {
		it('should toggle description when clicked', async () => {
			render(ObjectsPage);
			await waitFor(() => screen.getByText("What's this?"));

			const toggleButton = screen.getByText("What's this?");

			// Description should not be visible initially
			expect(screen.queryByText(/primary goal of this is/i)).not.toBeInTheDocument();

			// Click to show
			await fireEvent.click(toggleButton);
			await waitFor(() => {
				expect(screen.getByText(/primary goal of this is/i)).toBeInTheDocument();
			});

			// Click to hide
			await fireEvent.click(toggleButton);
			await waitFor(() => {
				expect(screen.queryByText(/primary goal of this is/i)).not.toBeInTheDocument();
			});
		});
	});

	describe('Error Handling', () => {
		it('should show toast on delete error', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			const { toastStore } = await import('$lib/stores/toast.js');

			(garmApi.deleteFileObject as any).mockRejectedValue(new Error('Delete failed'));

			render(ObjectsPage);
			await waitFor(() => screen.getByText('file1.bin'));

			// Try to delete
			const deleteButtons = screen.getAllByRole('button', { name: 'Delete object' });
			await fireEvent.click(deleteButtons[0]);
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

		it('should show toast on update error', async () => {
			const { garmApi } = await import('$lib/api/client.js');
			const { toastStore } = await import('$lib/stores/toast.js');

			(garmApi.updateFileObject as any).mockRejectedValue(new Error('Update failed'));

			render(ObjectsPage);
			await waitFor(() => screen.getByText('file1.bin'));

			// Open update modal and submit
			const updateButtons = screen.getAllByRole('button', { name: 'Update object' });
			await fireEvent.click(updateButtons[0]);
			await waitFor(() => screen.getByText('Update Object'));

			const submitButton = screen.getByRole('button', { name: 'Update' });
			await fireEvent.click(submitButton);

			// Should show error toast
			await waitFor(() => {
				expect(toastStore.add).toHaveBeenCalledWith(
					expect.objectContaining({
						type: 'error',
						title: 'Failed to update object'
					})
				);
			});
		});
	});
});
