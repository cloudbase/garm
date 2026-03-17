import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/svelte';

// Mock $app/state (template create page uses this, not $app/stores)
vi.mock('$app/state', () => ({
	page: {
		params: {},
		url: new URL('http://localhost/templates/create')
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
		createTemplate: vi.fn(),
		getTemplate: vi.fn()
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

// Mock CodeEditor since it depends on codemirror which requires a real DOM
vi.mock('$lib/components/CodeEditor.svelte', () => ({
	default: function MockCodeEditor(anchor: any, props: any) {
		const target = anchor?.parentNode;
		if (target) {
			const div = document.createElement('div');
			div.setAttribute('data-testid', 'code-editor');
			div.textContent = 'Code Editor';
			target.appendChild(div);
		}
		return {
			$destroy: vi.fn(),
			$set: vi.fn(),
			$on: vi.fn()
		};
	}
}));

import TemplateCreatePage from './+page.svelte';

describe('Template Create Page', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('renders create template page title "Create New Template"', async () => {
		render(TemplateCreatePage);

		await waitFor(() => {
			expect(screen.getByRole('heading', { name: 'Create New Template' })).toBeInTheDocument();
		});
	});

	it('shows form fields: Name input, Description input, Forge Type select, OS Type select', async () => {
		render(TemplateCreatePage);

		await waitFor(() => {
			// Name field
			const nameInput = screen.getByLabelText(/Name \*/);
			expect(nameInput).toBeInTheDocument();
			expect(nameInput).toHaveAttribute('type', 'text');

			// Description field
			const descriptionInput = screen.getByLabelText('Description');
			expect(descriptionInput).toBeInTheDocument();
			expect(descriptionInput).toHaveAttribute('type', 'text');

			// Forge Type select
			const forgeTypeSelect = screen.getByLabelText(/Forge Type \*/);
			expect(forgeTypeSelect).toBeInTheDocument();
			expect(forgeTypeSelect.tagName).toBe('SELECT');

			// OS Type select
			const osTypeSelect = screen.getByLabelText(/OS Type \*/);
			expect(osTypeSelect).toBeInTheDocument();
			expect(osTypeSelect.tagName).toBe('SELECT');
		});
	});

	it('shows "Template Content" section with CodeEditor', async () => {
		render(TemplateCreatePage);

		await waitFor(() => {
			expect(screen.getByText('Template Content')).toBeInTheDocument();
			expect(screen.getByTestId('code-editor')).toBeInTheDocument();
		});
	});

	it('shows "Upload File" button', async () => {
		render(TemplateCreatePage);

		await waitFor(() => {
			expect(screen.getByRole('button', { name: /Upload File/ })).toBeInTheDocument();
		});
	});

	it('shows "Available Template Variables" help section', async () => {
		render(TemplateCreatePage);

		await waitFor(() => {
			expect(screen.getByText('Available Template Variables')).toBeInTheDocument();
			// Variables appear in both code elements and description text, so use getAllByText
			expect(screen.getAllByText(/\.RunnerName/).length).toBeGreaterThan(0);
			expect(screen.getAllByText(/\.DownloadURL/).length).toBeGreaterThan(0);
			expect(screen.getAllByText(/\.MetadataURL/).length).toBeGreaterThan(0);
		});
	});

	it('has cancel button with "Cancel" label', async () => {
		render(TemplateCreatePage);

		await waitFor(() => {
			expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument();
		});
	});
});
