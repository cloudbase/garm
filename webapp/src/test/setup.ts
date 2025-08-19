import '@testing-library/jest-dom';

// Mock SvelteKit runtime modules
import { vi } from 'vitest';

// Mock SvelteKit stores
vi.mock('$app/stores', () => ({
	page: {
		subscribe: vi.fn(() => () => {})
	}
}));

// Mock SvelteKit paths
vi.mock('$app/paths', () => ({
	resolve: vi.fn((path: string) => path)
}));

// Mock SvelteKit environment - Set browser to true for client-side rendering
vi.mock('$app/environment', () => ({
	browser: true,
	dev: true,
	building: false,
	version: 'test'
}));

// Simple component mocks that render as basic divs
vi.mock('$lib/components/CreateRepositoryModal.svelte', () => ({
	default: function MockCreateRepositoryModal(options: any) {
		const target = options.target;
		if (target) {
			const div = document.createElement('div');
			div.setAttribute('data-testid', 'create-repository-modal');
			div.textContent = 'Create Repository Modal';
			target.appendChild(div);
		}
		return {
			$destroy: vi.fn(),
			$set: vi.fn(),
			$on: vi.fn()
		};
	}
}));

vi.mock('$lib/components/UpdateEntityModal.svelte', () => ({
	default: function MockUpdateEntityModal(options: any) {
		const target = options.target;
		if (target) {
			const div = document.createElement('div');
			div.setAttribute('data-testid', 'update-entity-modal');
			div.textContent = 'Update Entity Modal';
			target.appendChild(div);
		}
		return {
			$destroy: vi.fn(),
			$set: vi.fn(),
			$on: vi.fn()
		};
	}
}));

vi.mock('$lib/components/DeleteModal.svelte', () => ({
	default: function MockDeleteModal(options: any) {
		const target = options.target;
		if (target) {
			const div = document.createElement('div');
			div.setAttribute('data-testid', 'delete-modal');
			div.textContent = 'Delete Modal';
			target.appendChild(div);
		}
		return {
			$destroy: vi.fn(),
			$set: vi.fn(),
			$on: vi.fn()
		};
	}
}));

vi.mock('$lib/components/PageHeader.svelte', () => ({
	default: function MockPageHeader(options: any) {
		const target = options.target;
		if (target) {
			const div = document.createElement('div');
			// Extract title from props or use generic title
			const props = options.props || {};
			const title = props.title || 'Runner Instances';
			const showAction = props.showAction !== false;
			const actionText = props.actionText || 'Add';
			
			let html = `<h1>${title}</h1>`;
			if (showAction) {
				html += `<button data-testid="add-button">${actionText}</button>`;
			}
			div.innerHTML = html;
			target.appendChild(div);
		}
		return {
			$destroy: vi.fn(),
			$set: vi.fn(),
			$on: vi.fn()
		};
	}
}));

vi.mock('$lib/components/DataTable.svelte', () => ({
	default: function MockDataTable(options: any) {
		const target = options.target;
		if (target) {
			const div = document.createElement('div');
			div.setAttribute('data-testid', 'data-table');
			
			// Extract search placeholder from props
			const props = options.props || {};
			const searchPlaceholder = props.searchPlaceholder || 'Search...';
			
			div.innerHTML = `
				<div>DataTable Component</div>
				<input type="search" placeholder="${searchPlaceholder}" />
			`;
			target.appendChild(div);
		}
		return {
			$destroy: vi.fn(),
			$set: vi.fn(),
			$on: vi.fn()
		};
	}
}));

// Mock cell components
vi.mock('$lib/components/cells', () => ({
	EntityCell: function MockEntityCell(options: any) {
		const target = options.target;
		if (target) {
			const div = document.createElement('div');
			div.setAttribute('data-testid', 'entity-cell');
			div.textContent = 'Entity Cell';
			target.appendChild(div);
		}
		return { $destroy: vi.fn(), $set: vi.fn(), $on: vi.fn() };
	},
	EndpointCell: function MockEndpointCell(options: any) {
		const target = options.target;
		if (target) {
			const div = document.createElement('div');
			div.setAttribute('data-testid', 'endpoint-cell');
			div.textContent = 'Endpoint Cell';
			target.appendChild(div);
		}
		return { $destroy: vi.fn(), $set: vi.fn(), $on: vi.fn() };
	},
	StatusCell: function MockStatusCell(options: any) {
		const target = options.target;
		if (target) {
			const div = document.createElement('div');
			div.setAttribute('data-testid', 'status-cell');
			div.textContent = 'Status Cell';
			target.appendChild(div);
		}
		return { $destroy: vi.fn(), $set: vi.fn(), $on: vi.fn() };
	},
	ActionsCell: function MockActionsCell(options: any) {
		const target = options.target;
		if (target) {
			const div = document.createElement('div');
			div.setAttribute('data-testid', 'actions-cell');
			div.textContent = 'Actions Cell';
			target.appendChild(div);
		}
		return { $destroy: vi.fn(), $set: vi.fn(), $on: vi.fn() };
	},
	GenericCell: function MockGenericCell(options: any) {
		const target = options.target;
		if (target) {
			const div = document.createElement('div');
			div.setAttribute('data-testid', 'generic-cell');
			div.textContent = 'Generic Cell';
			target.appendChild(div);
		}
		return { $destroy: vi.fn(), $set: vi.fn(), $on: vi.fn() };
	},
	InstancePoolCell: function MockInstancePoolCell(options: any) {
		const target = options.target;
		if (target) {
			const div = document.createElement('div');
			div.setAttribute('data-testid', 'instance-pool-cell');
			div.textContent = 'Instance Pool Cell';
			target.appendChild(div);
		}
		return { $destroy: vi.fn(), $set: vi.fn(), $on: vi.fn() };
	}
}));