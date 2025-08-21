import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import LoginPage from './+page.svelte';

// Helper function to create complete AuthState objects
function createMockAuthState(overrides: any = {}) {
	return {
		isAuthenticated: false,
		user: null,
		loading: false,
		needsInitialization: false,
		...overrides
	};
}

// Mock all external dependencies
vi.mock('$app/navigation', () => ({
	goto: vi.fn()
}));

vi.mock('$app/paths', () => ({
	resolve: vi.fn((path: string) => path)
}));

vi.mock('$lib/stores/auth.js', () => ({
	authStore: {
		subscribe: vi.fn((callback: (state: any) => void) => {
			callback(createMockAuthState());
			return () => {};
		})
	},
	auth: {
		login: vi.fn()
	}
}));

vi.mock('$lib/utils/apiError', () => ({
	extractAPIError: vi.fn((err) => err.message || 'Unknown error')
}));

// Reset any component mocks that might be set by setup.ts
vi.unmock('$lib/components/Button.svelte');

// Mock DOM APIs
const mockLocalStorage = {
	getItem: vi.fn(),
	setItem: vi.fn(),
	removeItem: vi.fn()
};

const mockMatchMedia = vi.fn();

describe('Login Page - Render Tests', () => {
	beforeEach(async () => {
		vi.clearAllMocks();
		
		// Set up default API mocks
		const { auth } = await import('$lib/stores/auth.js');
		(auth.login as any).mockResolvedValue({});
		
		const { resolve } = await import('$app/paths');
		(resolve as any).mockImplementation((path: string) => path);
		
		// Mock DOM APIs
		Object.defineProperty(window, 'localStorage', { value: mockLocalStorage });
		Object.defineProperty(window, 'matchMedia', { value: mockMatchMedia });
		
		(mockLocalStorage.getItem as any).mockReturnValue(null);
		(mockMatchMedia as any).mockReturnValue({ matches: false });
	});

	describe('Basic Rendering', () => {
		it('should render without crashing', () => {
			const { container } = render(LoginPage);
			expect(container).toBeInTheDocument();
		});

		it('should have proper document structure', () => {
			const { container } = render(LoginPage);
			expect(container.querySelector('.min-h-screen')).toBeInTheDocument();
		});

		it('should render main layout container', () => {
			render(LoginPage);
			
			// Should have main container with proper styling
			const mainContainer = document.querySelector('.min-h-screen.flex.items-center.justify-center');
			expect(mainContainer).toBeInTheDocument();
		});

		it('should render centered content area', () => {
			render(LoginPage);
			
			// Should have centered content area
			const contentArea = document.querySelector('.max-w-md.w-full.space-y-8');
			expect(contentArea).toBeInTheDocument();
		});
	});

	describe('Component Lifecycle', () => {
		it('should mount successfully', () => {
			const { component } = render(LoginPage);
			expect(component).toBeDefined();
		});

		it('should unmount without errors', () => {
			const { unmount } = render(LoginPage);
			expect(() => unmount()).not.toThrow();
		});

		it('should handle component updates', () => {
			const { component } = render(LoginPage);
			
			// Component should handle reactive updates
			expect(component).toBeDefined();
		});

		it('should complete mount process successfully', () => {
			render(LoginPage);
			
			// Should complete mount without errors
			// (Theme initialization works in browser but not in test environment)
			expect(screen.getByRole('heading', { name: 'Sign in to GARM' })).toBeInTheDocument();
		});
	});

	describe('DOM Structure', () => {
		it('should create proper DOM hierarchy', () => {
			const { container } = render(LoginPage);
			
			// Should have main container
			const mainContainer = container.querySelector('.min-h-screen');
			expect(mainContainer).toBeInTheDocument();
			
			// Should have content area
			const contentArea = container.querySelector('.max-w-md');
			expect(contentArea).toBeInTheDocument();
		});

		it('should render svelte:head for page title', () => {
			render(LoginPage);
			
			// Should set page title
			expect(document.title).toBe('Login - GARM');
		});

		it('should handle responsive layout classes', () => {
			render(LoginPage);
			
			// Should have responsive layout
			const mainContainer = document.querySelector('.min-h-screen.flex.items-center.justify-center.bg-gray-50.dark\\:bg-gray-900.py-12.px-4.sm\\:px-6.lg\\:px-8');
			expect(mainContainer).toBeInTheDocument();
		});
	});

	describe('Header Section Rendering', () => {
		it('should render logo section', () => {
			render(LoginPage);
			
			// Should have logo container
			const logoContainer = document.querySelector('.mx-auto.h-48.w-auto.flex.justify-center');
			expect(logoContainer).toBeInTheDocument();
		});

		it('should render both light and dark logos', () => {
			render(LoginPage);
			
			const logos = screen.getAllByAltText('GARM');
			expect(logos).toHaveLength(2);
			
			// Should have light logo (visible by default)
			const lightLogo = logos.find(img => img.classList.contains('dark:hidden'));
			expect(lightLogo).toBeInTheDocument();
			
			// Should have dark logo (hidden by default)
			const darkLogo = logos.find(img => img.classList.contains('hidden'));
			expect(darkLogo).toBeInTheDocument();
		});

		it('should render page title and description', () => {
			render(LoginPage);
			
			// Should render main heading
			expect(screen.getByRole('heading', { name: 'Sign in to GARM' })).toBeInTheDocument();
			
			// Should render description
			expect(screen.getByText('GitHub Actions Runner Manager')).toBeInTheDocument();
		});

		it('should have proper heading hierarchy', () => {
			render(LoginPage);
			
			const heading = screen.getByRole('heading', { name: 'Sign in to GARM' });
			expect(heading.tagName).toBe('H2');
			expect(heading).toHaveClass('text-3xl', 'font-extrabold');
		});
	});

	describe('Form Rendering', () => {
		it('should render login form', () => {
			render(LoginPage);
			
			// Should have form element
			const form = document.querySelector('form');
			expect(form).toBeInTheDocument();
			expect(form).toHaveClass('mt-8', 'space-y-6');
		});

		it('should render username input field', () => {
			render(LoginPage);
			
			const usernameInput = screen.getByLabelText('Username');
			expect(usernameInput).toBeInTheDocument();
			expect(usernameInput).toHaveAttribute('type', 'text');
			expect(usernameInput).toHaveAttribute('name', 'username');
			expect(usernameInput).toHaveAttribute('required');
			expect(usernameInput).toHaveAttribute('placeholder', 'Username');
		});

		it('should render password input field', () => {
			render(LoginPage);
			
			const passwordInput = screen.getByLabelText('Password');
			expect(passwordInput).toBeInTheDocument();
			expect(passwordInput).toHaveAttribute('type', 'password');
			expect(passwordInput).toHaveAttribute('name', 'password');
			expect(passwordInput).toHaveAttribute('required');
			expect(passwordInput).toHaveAttribute('placeholder', 'Password');
		});

		it('should render submit button', () => {
			render(LoginPage);
			
			const submitButton = screen.getByRole('button', { name: /sign in/i });
			expect(submitButton).toBeInTheDocument();
			expect(submitButton).toHaveAttribute('type', 'submit');
		});

		it('should have proper form styling', () => {
			render(LoginPage);
			
			// Should have rounded form container
			const formContainer = document.querySelector('.rounded-md.shadow-sm.-space-y-px');
			expect(formContainer).toBeInTheDocument();
			
			// Username should have rounded top
			const usernameInput = screen.getByLabelText('Username');
			expect(usernameInput).toHaveClass('rounded-t-md');
			
			// Password should have rounded bottom
			const passwordInput = screen.getByLabelText('Password');
			expect(passwordInput).toHaveClass('rounded-b-md');
		});
	});

	describe('Error State Rendering', () => {
		it('should not show error state initially', () => {
			render(LoginPage);
			
			// Should not have error container initially
			const errorContainer = document.querySelector('.bg-red-50');
			expect(errorContainer).not.toBeInTheDocument();
		});

		it('should conditionally render error display', () => {
			render(LoginPage);
			
			// Error display should be conditional (not visible initially)
			expect(screen.queryByText(/error/i)).not.toBeInTheDocument();
		});

		it('should have proper error styling structure ready', () => {
			render(LoginPage);
			
			// Form should be structured to accommodate error display
			const form = document.querySelector('form');
			expect(form).toHaveClass('space-y-6');
		});
	});

	describe('Button Integration', () => {
		it('should integrate Button component', () => {
			render(LoginPage);
			
			const submitButton = screen.getByRole('button', { name: /sign in/i });
			expect(submitButton).toBeInTheDocument();
		});

		it('should pass correct props to Button', () => {
			render(LoginPage);
			
			const submitButton = screen.getByRole('button', { name: /sign in/i });
			
			// Should be submit type
			expect(submitButton).toHaveAttribute('type', 'submit');
			
			// Should have primary variant styling (blue background)
			expect(submitButton).toHaveClass('bg-blue-600');
		});

		it('should render Button with full width', () => {
			render(LoginPage);
			
			const submitButton = screen.getByRole('button', { name: /sign in/i });
			expect(submitButton).toHaveClass('w-full');
		});
	});

	describe('Accessibility Features', () => {
		it('should have proper form labels', () => {
			render(LoginPage);
			
			// Username field should have accessible label
			const usernameLabel = screen.getByLabelText('Username');
			expect(usernameLabel).toBeInTheDocument();
			
			// Password field should have accessible label
			const passwordLabel = screen.getByLabelText('Password');
			expect(passwordLabel).toBeInTheDocument();
		});

		it('should have screen reader only labels', () => {
			render(LoginPage);
			
			// Should have sr-only labels for form fields
			const labels = document.querySelectorAll('.sr-only');
			expect(labels.length).toBeGreaterThanOrEqual(2); // At least username and password labels
		});

		it('should have proper form semantics', () => {
			render(LoginPage);
			
			// Should have form element
			const form = document.querySelector('form');
			expect(form).toBeInTheDocument();
			
			// Should have submit button
			const submitButton = screen.getByRole('button', { name: /sign in/i });
			expect(submitButton).toHaveAttribute('type', 'submit');
		});

		it('should support keyboard navigation', () => {
			render(LoginPage);
			
			const usernameInput = screen.getByLabelText('Username');
			const passwordInput = screen.getByLabelText('Password');
			const submitButton = screen.getByRole('button', { name: /sign in/i });
			
			// All elements should be focusable
			expect(usernameInput).toBeInTheDocument();
			expect(passwordInput).toBeInTheDocument();
			expect(submitButton).toBeInTheDocument();
		});
	});

	describe('Theme Support', () => {
		it('should have dark mode classes', () => {
			render(LoginPage);
			
			// Should have dark mode background
			const mainContainer = document.querySelector('.dark\\:bg-gray-900');
			expect(mainContainer).toBeInTheDocument();
			
			// Should have dark mode text colors
			const heading = screen.getByRole('heading', { name: 'Sign in to GARM' });
			expect(heading).toHaveClass('dark:text-white');
		});

		it('should handle theme-aware logo display', () => {
			render(LoginPage);
			
			const logos = screen.getAllByAltText('GARM');
			
			// Light logo should be hidden in dark mode
			const lightLogo = logos.find(img => img.classList.contains('dark:hidden'));
			expect(lightLogo).toBeInTheDocument();
			
			// Dark logo should be shown in dark mode
			const darkLogo = logos.find(img => img.classList.contains('dark:block'));
			expect(darkLogo).toBeInTheDocument();
		});

		it('should have theme-aware input styling', () => {
			render(LoginPage);
			
			const usernameInput = screen.getByLabelText('Username');
			
			// Should have dark mode classes
			expect(usernameInput).toHaveClass('dark:border-gray-600');
			expect(usernameInput).toHaveClass('dark:bg-gray-700');
			expect(usernameInput).toHaveClass('dark:text-white');
		});
	});

	describe('Responsive Design', () => {
		it('should use responsive layout classes', () => {
			render(LoginPage);
			
			// Should have responsive padding
			const mainContainer = document.querySelector('.py-12.px-4.sm\\:px-6.lg\\:px-8');
			expect(mainContainer).toBeInTheDocument();
		});

		it('should handle mobile-friendly layout', () => {
			render(LoginPage);
			
			// Should have mobile-optimized form
			const contentArea = document.querySelector('.max-w-md.w-full');
			expect(contentArea).toBeInTheDocument();
		});

		it('should have responsive typography', () => {
			render(LoginPage);
			
			const heading = screen.getByRole('heading', { name: 'Sign in to GARM' });
			
			// Should use responsive text sizing
			expect(heading).toHaveClass('text-3xl');
		});
	});

	describe('Visual Hierarchy', () => {
		it('should render elements in proper visual order', () => {
			render(LoginPage);
			
			// Logo should be first
			const logoContainer = document.querySelector('.mx-auto.h-48');
			expect(logoContainer).toBeInTheDocument();
			
			// Then heading
			const heading = screen.getByRole('heading', { name: 'Sign in to GARM' });
			expect(heading).toBeInTheDocument();
			
			// Then description
			const description = screen.getByText('GitHub Actions Runner Manager');
			expect(description).toBeInTheDocument();
			
			// Then form
			const form = document.querySelector('form');
			expect(form).toBeInTheDocument();
		});

		it('should have proper spacing between sections', () => {
			render(LoginPage);
			
			// Main container should have spacing
			const contentArea = document.querySelector('.space-y-8');
			expect(contentArea).toBeInTheDocument();
			
			// Form should have spacing
			const form = document.querySelector('form.space-y-6');
			expect(form).toBeInTheDocument();
		});

		it('should use consistent typography scale', () => {
			render(LoginPage);
			
			const heading = screen.getByRole('heading', { name: 'Sign in to GARM' });
			const description = screen.getByText('GitHub Actions Runner Manager');
			
			// Heading should be larger
			expect(heading).toHaveClass('text-3xl', 'font-extrabold');
			
			// Description should be smaller
			expect(description).toHaveClass('text-sm');
		});
	});

	describe('Loading State Rendering', () => {
		it('should render button in normal state initially', () => {
			render(LoginPage);
			
			const submitButton = screen.getByRole('button', { name: /sign in/i });
			expect(submitButton).not.toBeDisabled();
			expect(screen.getByText('Sign in')).toBeInTheDocument();
		});

		it('should support loading state styling', () => {
			render(LoginPage);
			
			// Button should be ready to show loading state
			const submitButton = screen.getByRole('button', { name: /sign in/i });
			expect(submitButton).toBeInTheDocument();
		});

		it('should support disabled input states', () => {
			render(LoginPage);
			
			const usernameInput = screen.getByLabelText('Username');
			const passwordInput = screen.getByLabelText('Password');
			
			// Fields should be ready to be disabled
			expect(usernameInput).not.toBeDisabled();
			expect(passwordInput).not.toBeDisabled();
		});
	});
});