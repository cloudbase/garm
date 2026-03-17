import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/svelte';
import type { Template } from '$lib/api/generated/api.js';

// Mock $app/state (template detail page uses this, not $app/stores)
vi.mock('$app/state', () => ({
	page: {
		params: { id: '1' },
		url: new URL('http://localhost/templates/1')
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
		getTemplate: vi.fn(),
		updateTemplate: vi.fn(),
		deleteTemplate: vi.fn()
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

// Mock JWT utils
vi.mock('$lib/utils/jwt', () => ({
	isCurrentUserAdmin: vi.fn(() => true)
}));

import TemplateDetailPage from './+page.svelte';

const mockTemplate: Template = {
	id: 1,
	name: 'linux-github-default',
	description: 'Default Linux runner install template for GitHub',
	forge_type: 'github',
	os_type: 'linux',
	owner_id: 'system',
	created_at: '2024-01-01T00:00:00Z',
	updated_at: '2024-01-01T00:00:00Z',
	data: Array.from(new TextEncoder().encode('#!/bin/bash\necho "Installing runner..."'))
};

describe('Template Detail Page Integration Tests', () => {
	beforeEach(async () => {
		vi.clearAllMocks();
		const { garmApi } = await import('$lib/api/client.js');
		(garmApi.getTemplate as any).mockResolvedValue(mockTemplate);
	});

	it('should show loading state initially', async () => {
		const { garmApi } = await import('$lib/api/client.js');
		// Delay the API response so loading state is visible
		(garmApi.getTemplate as any).mockImplementation(
			() => new Promise((resolve) => setTimeout(() => resolve(mockTemplate), 100))
		);

		render(TemplateDetailPage);

		// Loading text should appear while the API call is pending
		expect(screen.getByText('Loading template...')).toBeInTheDocument();

		// After API resolves, loading should go away and template data should appear
		await waitFor(() => {
			expect(screen.queryByText('Loading template...')).not.toBeInTheDocument();
			expect(screen.getByText('Template Information')).toBeInTheDocument();
		});
	});

	it('should render template details after loading', async () => {
		render(TemplateDetailPage);

		await waitFor(() => {
			// Template name displayed in the information section
			expect(screen.getAllByText('linux-github-default').length).toBeGreaterThan(0);
			// Description
			expect(screen.getAllByText('Default Linux runner install template for GitHub').length).toBeGreaterThan(0);
		});
	});

	it('should show forge type and OS type badges', async () => {
		render(TemplateDetailPage);

		await waitFor(() => {
			// Forge type badge should display "GitHub"
			expect(screen.getByText('GitHub')).toBeInTheDocument();
			// OS type badge should display "Linux"
			expect(screen.getByText('Linux')).toBeInTheDocument();
		});
	});

	it('should show Template Information section with owner and template ID', async () => {
		render(TemplateDetailPage);

		await waitFor(() => {
			// Section header
			expect(screen.getByText('Template Information')).toBeInTheDocument();
			// Owner field - 'system' owner is displayed as 'System'
			expect(screen.getByText('Owner')).toBeInTheDocument();
			expect(screen.getByText('System')).toBeInTheDocument();
			// Template ID field - use getAllByText because CodeMirror gutter also renders "1"
			expect(screen.getByText('Template ID')).toBeInTheDocument();
			expect(screen.getAllByText('1').length).toBeGreaterThan(0);
		});
	});

	it('should show Template Content section', async () => {
		render(TemplateDetailPage);

		await waitFor(() => {
			expect(screen.getByText('Template Content')).toBeInTheDocument();
		});
	});

	it('should show created and updated dates', async () => {
		render(TemplateDetailPage);

		await waitFor(() => {
			expect(screen.getByText('Created')).toBeInTheDocument();
			expect(screen.getByText('Last Updated')).toBeInTheDocument();
			// Dates are rendered via toLocaleDateString, just verify the labels exist
			const formattedDate = new Date('2024-01-01T00:00:00Z').toLocaleDateString();
			expect(screen.getAllByText(formattedDate).length).toBeGreaterThan(0);
		});
	});

	it('should handle error state when API fails', async () => {
		const { garmApi } = await import('$lib/api/client.js');
		(garmApi.getTemplate as any).mockRejectedValue(new Error('Template not found'));

		render(TemplateDetailPage);

		await waitFor(() => {
			// Error heading should be displayed
			expect(screen.getByText('Error loading template')).toBeInTheDocument();
			// The error message text should be shown
			expect(screen.getByText('Template not found')).toBeInTheDocument();
			// Template content sections should not be rendered
			expect(screen.queryByText('Template Information')).not.toBeInTheDocument();
			expect(screen.queryByText('Template Content')).not.toBeInTheDocument();
		});
	});
});
