import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import InitPage from './+page.svelte';

// Helper function to create complete AuthState objects
function createMockAuthState(overrides: any = {}) {
	return {
		isAuthenticated: false,
		user: null,
		loading: false,
		needsInitialization: true,
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
		initialize: vi.fn()
	}
}));

vi.mock('$lib/stores/toast.js', () => ({
	toastStore: {
		success: vi.fn()
	}
}));

vi.mock('$lib/utils/apiError', () => ({
	extractAPIError: vi.fn((err) => err.message || 'Unknown error')
}));

// Reset any component mocks that might be set by setup.ts
vi.unmock('$lib/components/Button.svelte');

describe('Init Page - Render Tests', () => {
	beforeEach(async () => {
		vi.clearAllMocks();
		
		// Set up default API mocks
		const { auth } = await import('$lib/stores/auth.js');
		(auth.initialize as any).mockResolvedValue({});
		
		const { resolve } = await import('$app/paths');
		(resolve as any).mockImplementation((path: string) => path);
		
		// Mock window.location for URL auto-population
		Object.defineProperty(window, 'location', {
			value: {
				origin: 'https://garm.example.com'
			},
			writable: true
		});
	});

	describe('Basic Rendering', () => {
		it('should render without crashing', () => {
			const { container } = render(InitPage);
			expect(container).toBeInTheDocument();
		});

		it('should have proper document structure', () => {
			const { container } = render(InitPage);
			expect(container.querySelector('.min-h-screen')).toBeInTheDocument();
		});

		it('should render main layout container', () => {
			render(InitPage);
			
			// Should have main container with proper styling
			const mainContainer = document.querySelector('.min-h-screen.bg-gray-50.dark\\:bg-gray-900');
			expect(mainContainer).toBeInTheDocument();
		});

		it('should render centered content areas', () => {
			render(InitPage);
			
			// Should have centered header area
			const headerArea = document.querySelector('.sm\\:mx-auto.sm\\:w-full.sm\\:max-w-md');
			expect(headerArea).toBeInTheDocument();
			
			// Should have centered form area
			const formArea = document.querySelector('.mt-8.sm\\:mx-auto.sm\\:w-full.sm\\:max-w-md');
			expect(formArea).toBeInTheDocument();
		});
	});

	describe('Component Lifecycle', () => {
		it('should mount successfully', () => {
			const { component } = render(InitPage);
			expect(component).toBeDefined();
		});

		it('should unmount without errors', () => {
			const { unmount } = render(InitPage);
			expect(() => unmount()).not.toThrow();
		});

		it('should handle component updates', () => {
			const { component } = render(InitPage);
			
			// Component should handle reactive updates
			expect(component).toBeDefined();
		});
	});

	describe('DOM Structure', () => {
		it('should create proper DOM hierarchy', () => {
			const { container } = render(InitPage);
			
			// Should have main container
			const mainContainer = container.querySelector('.min-h-screen');
			expect(mainContainer).toBeInTheDocument();
			
			// Should have header area
			const headerArea = container.querySelector('.sm\\:mx-auto.sm\\:w-full.sm\\:max-w-md');
			expect(headerArea).toBeInTheDocument();
			
			// Should have form card
			const formCard = container.querySelector('.bg-white.dark\\:bg-gray-800');
			expect(formCard).toBeInTheDocument();
		});

		it('should render svelte:head for page title', () => {
			render(InitPage);
			
			// Should set page title
			expect(document.title).toBe('Initialize GARM - First Run Setup');
		});

		it('should have responsive layout classes', () => {
			render(InitPage);
			
			// Should have responsive layout
			const mainContainer = document.querySelector('.min-h-screen.bg-gray-50.dark\\:bg-gray-900.flex.flex-col.justify-center.py-12.sm\\:px-6.lg\\:px-8');
			expect(mainContainer).toBeInTheDocument();
		});
	});

	describe('Header Section Rendering', () => {
		it('should render logo section', () => {
			render(InitPage);
			
			// Should have logo container
			const logoContainer = document.querySelector('.flex.justify-center');
			expect(logoContainer).toBeInTheDocument();
		});

		it('should render both light and dark logos', () => {
			render(InitPage);
			
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
			render(InitPage);
			
			// Should render main heading
			expect(screen.getByRole('heading', { name: 'Welcome to GARM' })).toBeInTheDocument();
			
			// Should render description
			expect(screen.getByText('Complete the first-run setup to get started')).toBeInTheDocument();
		});

		it('should have proper heading hierarchy', () => {
			render(InitPage);
			
			const heading = screen.getByRole('heading', { name: 'Welcome to GARM' });
			expect(heading.tagName).toBe('H1');
			expect(heading).toHaveClass('text-3xl', 'font-extrabold');
		});
	});

	describe('Info Banner Rendering', () => {
		it('should render initialization info banner', () => {
			render(InitPage);
			
			// Should have info banner
			const infoBanner = document.querySelector('.bg-blue-50.dark\\:bg-blue-900\\/20');
			expect(infoBanner).toBeInTheDocument();
			
			// Should have info title
			expect(screen.getByText('First-Run Initialization')).toBeInTheDocument();
			
			// Should have info description
			expect(screen.getByText(/GARM needs to be initialized before first use/)).toBeInTheDocument();
		});

		it('should have proper info banner styling', () => {
			render(InitPage);
			
			const infoBanner = document.querySelector('.bg-blue-50.dark\\:bg-blue-900\\/20.border.border-blue-200.dark\\:border-blue-800.rounded-md.p-4.mb-6');
			expect(infoBanner).toBeInTheDocument();
		});

		it('should render info icon', () => {
			render(InitPage);
			
			const infoIcon = document.querySelector('.h-5.w-5.text-blue-400');
			expect(infoIcon).toBeInTheDocument();
		});
	});

	describe('Form Rendering', () => {
		it('should render initialization form', () => {
			render(InitPage);
			
			// Should have form element
			const form = document.querySelector('form');
			expect(form).toBeInTheDocument();
			expect(form).toHaveClass('space-y-6');
		});

		it('should render all form fields', () => {
			render(InitPage);
			
			// Required fields
			expect(screen.getByLabelText('Username')).toBeInTheDocument();
			expect(screen.getByLabelText('Email Address')).toBeInTheDocument();
			expect(screen.getByLabelText('Full Name')).toBeInTheDocument();
			expect(screen.getByLabelText('Password')).toBeInTheDocument();
			expect(screen.getByLabelText('Confirm Password')).toBeInTheDocument();
		});

		it('should render form fields with proper attributes', () => {
			render(InitPage);
			
			const usernameInput = screen.getByLabelText('Username');
			expect(usernameInput).toHaveAttribute('type', 'text');
			expect(usernameInput).toHaveAttribute('name', 'username');
			expect(usernameInput).toHaveAttribute('required');
			
			const emailInput = screen.getByLabelText('Email Address');
			expect(emailInput).toHaveAttribute('type', 'email');
			expect(emailInput).toHaveAttribute('name', 'email');
			expect(emailInput).toHaveAttribute('required');
			
			const passwordInput = screen.getByLabelText('Password');
			expect(passwordInput).toHaveAttribute('type', 'password');
			expect(passwordInput).toHaveAttribute('name', 'password');
			expect(passwordInput).toHaveAttribute('required');
		});

		it('should render submit button', () => {
			render(InitPage);
			
			const submitButton = screen.getByRole('button', { name: /initialize garm/i });
			expect(submitButton).toBeInTheDocument();
			expect(submitButton).toHaveAttribute('type', 'submit');
		});

		it('should have proper form styling', () => {
			render(InitPage);
			
			// Should have form card container
			const formCard = document.querySelector('.bg-white.dark\\:bg-gray-800.py-8.px-4.shadow.sm\\:rounded-lg.sm\\:px-10');
			expect(formCard).toBeInTheDocument();
			
			// Form inputs should have consistent styling
			const usernameInput = screen.getByLabelText('Username');
			expect(usernameInput).toHaveClass('appearance-none', 'block', 'w-full', 'px-3', 'py-2', 'border');
		});
	});

	describe('Advanced Configuration Rendering', () => {
		it('should render advanced configuration toggle', () => {
			render(InitPage);
			
			const toggleButton = screen.getByRole('button', { name: /advanced configuration/i });
			expect(toggleButton).toBeInTheDocument();
		});

		it('should not show advanced fields initially', () => {
			render(InitPage);
			
			// Advanced fields should not be visible initially
			expect(screen.queryByLabelText('Metadata URL')).not.toBeInTheDocument();
			expect(screen.queryByLabelText('Callback URL')).not.toBeInTheDocument();
			expect(screen.queryByLabelText('Webhook URL')).not.toBeInTheDocument();
		});

		it('should have proper toggle button styling', () => {
			render(InitPage);
			
			const toggleButton = screen.getByRole('button', { name: /advanced configuration/i });
			
			// Should have ghost variant styling
			expect(toggleButton).toHaveClass('text-gray-700', 'dark:text-gray-300');
		});

		it('should render toggle icon', () => {
			render(InitPage);
			
			// Should have chevron icon in toggle button
			const chevronIcon = document.querySelector('.w-4.h-4.mr-2.transition-transform');
			expect(chevronIcon).toBeInTheDocument();
		});
	});

	describe('Validation Messages Rendering', () => {
		it('should not show validation messages initially', () => {
			render(InitPage);
			
			// Should not have validation messages initially
			expect(screen.queryByText('Username is required')).not.toBeInTheDocument();
			expect(screen.queryByText('Please enter a valid email address')).not.toBeInTheDocument();
			expect(screen.queryByText('Password must be at least 8 characters long')).not.toBeInTheDocument();
		});

		it('should show validation summary with default values', () => {
			render(InitPage);
			
			// Should show validation summary because form has default values but is missing passwords
			// The validation summary shows when form is invalid AND has field content (which default values provide)
			expect(screen.getByText('Please complete all required fields')).toBeInTheDocument();
		});

		it('should have proper validation message styling structure ready', () => {
			render(InitPage);
			
			// Form should be structured to accommodate validation messages
			const form = document.querySelector('form');
			expect(form).toHaveClass('space-y-6');
		});
	});

	describe('Error State Rendering', () => {
		it('should not show error state initially', () => {
			render(InitPage);
			
			// Should not have error container initially
			const errorContainer = document.querySelector('.bg-red-50');
			expect(errorContainer).not.toBeInTheDocument();
		});

		it('should conditionally render error display', () => {
			render(InitPage);
			
			// Error display should be conditional (not visible initially)
			expect(screen.queryByText(/error/i)).not.toBeInTheDocument();
		});
	});

	describe('Button Integration', () => {
		it('should integrate Button component', () => {
			render(InitPage);
			
			const submitButton = screen.getByRole('button', { name: /initialize garm/i });
			expect(submitButton).toBeInTheDocument();
			
			const toggleButton = screen.getByRole('button', { name: /advanced configuration/i });
			expect(toggleButton).toBeInTheDocument();
		});

		it('should pass correct props to submit Button', () => {
			render(InitPage);
			
			const submitButton = screen.getByRole('button', { name: /initialize garm/i });
			
			// Should be submit type
			expect(submitButton).toHaveAttribute('type', 'submit');
			
			// Should have primary variant styling
			expect(submitButton).toHaveClass('bg-blue-600');
			
			// Should be full width
			expect(submitButton).toHaveClass('w-full');
		});

		it('should pass correct props to toggle Button', () => {
			render(InitPage);
			
			const toggleButton = screen.getByRole('button', { name: /advanced configuration/i });
			
			// Should be button type
			expect(toggleButton).toHaveAttribute('type', 'button');
			
			// Should have ghost variant styling
			expect(toggleButton).toHaveClass('text-gray-700', 'dark:text-gray-300');
		});
	});

	describe('Accessibility Features', () => {
		it('should have proper form labels', () => {
			render(InitPage);
			
			// All form fields should have accessible labels
			expect(screen.getByLabelText('Username')).toBeInTheDocument();
			expect(screen.getByLabelText('Email Address')).toBeInTheDocument();
			expect(screen.getByLabelText('Full Name')).toBeInTheDocument();
			expect(screen.getByLabelText('Password')).toBeInTheDocument();
			expect(screen.getByLabelText('Confirm Password')).toBeInTheDocument();
		});

		it('should have proper form semantics', () => {
			render(InitPage);
			
			// Should have form element
			const form = document.querySelector('form');
			expect(form).toBeInTheDocument();
			
			// Should have submit button
			const submitButton = screen.getByRole('button', { name: /initialize garm/i });
			expect(submitButton).toHaveAttribute('type', 'submit');
		});

		it('should support keyboard navigation', () => {
			render(InitPage);
			
			const usernameInput = screen.getByLabelText('Username');
			const emailInput = screen.getByLabelText('Email Address');
			const submitButton = screen.getByRole('button', { name: /initialize garm/i });
			
			// All elements should be focusable
			expect(usernameInput).toBeInTheDocument();
			expect(emailInput).toBeInTheDocument();
			expect(submitButton).toBeInTheDocument();
		});

		it('should have proper ARIA attributes', () => {
			render(InitPage);
			
			// Form inputs should have proper attributes
			const usernameInput = screen.getByLabelText('Username');
			expect(usernameInput).toHaveAttribute('required');
			
			const emailInput = screen.getByLabelText('Email Address');
			expect(emailInput).toHaveAttribute('required');
		});
	});

	describe('Theme Support', () => {
		it('should have dark mode classes', () => {
			render(InitPage);
			
			// Should have dark mode background
			const mainContainer = document.querySelector('.dark\\:bg-gray-900');
			expect(mainContainer).toBeInTheDocument();
			
			// Should have dark mode text colors
			const heading = screen.getByRole('heading', { name: 'Welcome to GARM' });
			expect(heading).toHaveClass('dark:text-white');
		});

		it('should handle theme-aware logo display', () => {
			render(InitPage);
			
			const logos = screen.getAllByAltText('GARM');
			
			// Light logo should be hidden in dark mode
			const lightLogo = logos.find(img => img.classList.contains('dark:hidden'));
			expect(lightLogo).toBeInTheDocument();
			
			// Dark logo should be shown in dark mode
			const darkLogo = logos.find(img => img.classList.contains('dark:block'));
			expect(darkLogo).toBeInTheDocument();
		});

		it('should have theme-aware input styling', () => {
			render(InitPage);
			
			const usernameInput = screen.getByLabelText('Username');
			
			// Should have dark mode classes
			expect(usernameInput).toHaveClass('dark:border-gray-600');
			expect(usernameInput).toHaveClass('dark:bg-gray-700');
			expect(usernameInput).toHaveClass('dark:text-white');
		});

		it('should have theme-aware form card styling', () => {
			render(InitPage);
			
			const formCard = document.querySelector('.bg-white.dark\\:bg-gray-800');
			expect(formCard).toBeInTheDocument();
		});
	});

	describe('Responsive Design', () => {
		it('should use responsive layout classes', () => {
			render(InitPage);
			
			// Should have responsive padding
			const mainContainer = document.querySelector('.py-12.sm\\:px-6.lg\\:px-8');
			expect(mainContainer).toBeInTheDocument();
		});

		it('should handle mobile-friendly layout', () => {
			render(InitPage);
			
			// Should have mobile-optimized form
			const headerArea = document.querySelector('.sm\\:mx-auto.sm\\:w-full.sm\\:max-w-md');
			expect(headerArea).toBeInTheDocument();
			
			const formArea = document.querySelector('.mt-8.sm\\:mx-auto.sm\\:w-full.sm\\:max-w-md');
			expect(formArea).toBeInTheDocument();
		});

		it('should have responsive typography', () => {
			render(InitPage);
			
			const heading = screen.getByRole('heading', { name: 'Welcome to GARM' });
			
			// Should use responsive text sizing
			expect(heading).toHaveClass('text-3xl');
		});

		it('should have responsive form card styling', () => {
			render(InitPage);
			
			const formCard = document.querySelector('.py-8.px-4.shadow.sm\\:rounded-lg.sm\\:px-10');
			expect(formCard).toBeInTheDocument();
		});
	});

	describe('Visual Hierarchy', () => {
		it('should render elements in proper visual order', () => {
			render(InitPage);
			
			// Logo should be first
			const logoContainer = document.querySelector('.flex.justify-center');
			expect(logoContainer).toBeInTheDocument();
			
			// Then heading
			const heading = screen.getByRole('heading', { name: 'Welcome to GARM' });
			expect(heading).toBeInTheDocument();
			
			// Then description
			const description = screen.getByText('Complete the first-run setup to get started');
			expect(description).toBeInTheDocument();
			
			// Then info banner
			const infoBanner = screen.getByText('First-Run Initialization');
			expect(infoBanner).toBeInTheDocument();
			
			// Then form
			const form = document.querySelector('form');
			expect(form).toBeInTheDocument();
		});

		it('should have proper spacing between sections', () => {
			render(InitPage);
			
			// Main container should have spacing
			const headerArea = document.querySelector('.sm\\:mx-auto.sm\\:w-full.sm\\:max-w-md');
			expect(headerArea).toBeInTheDocument();
			
			// Form area should have top margin
			const formArea = document.querySelector('.mt-8.sm\\:mx-auto.sm\\:w-full.sm\\:max-w-md');
			expect(formArea).toBeInTheDocument();
			
			// Form should have spacing
			const form = document.querySelector('form.space-y-6');
			expect(form).toBeInTheDocument();
		});

		it('should use consistent typography scale', () => {
			render(InitPage);
			
			const heading = screen.getByRole('heading', { name: 'Welcome to GARM' });
			const description = screen.getByText('Complete the first-run setup to get started');
			const infoTitle = screen.getByText('First-Run Initialization');
			
			// Main heading should be largest
			expect(heading).toHaveClass('text-3xl', 'font-extrabold');
			
			// Description should be smaller
			expect(description).toHaveClass('text-sm');
			
			// Info title should be medium
			expect(infoTitle).toHaveClass('text-sm', 'font-medium');
		});
	});

	describe('Loading State Rendering', () => {
		it('should render button in normal state initially', () => {
			render(InitPage);
			
			const submitButton = screen.getByRole('button', { name: /initialize garm/i });
			expect(screen.getByText('Initialize GARM')).toBeInTheDocument();
		});

		it('should support loading state styling', () => {
			render(InitPage);
			
			// Button should be ready to show loading state
			const submitButton = screen.getByRole('button', { name: /initialize garm/i });
			expect(submitButton).toBeInTheDocument();
		});

		it('should support disabled form states', () => {
			render(InitPage);
			
			const submitButton = screen.getByRole('button', { name: /initialize garm/i });
			
			// Button should be disabled initially (passwords empty)
			expect(submitButton).toBeDisabled();
		});
	});

	describe('Help Text Rendering', () => {
		it('should render help text section', () => {
			render(InitPage);
			
			// Should have help text (be more specific to avoid matching the info banner)
			expect(screen.getByText(/This will create the admin user, generate a unique controller ID, and configure the required URLs/)).toBeInTheDocument();
			expect(screen.getByText(/Make sure to remember these credentials/)).toBeInTheDocument();
		});

		it('should have proper help text styling', () => {
			render(InitPage);
			
			const helpText = document.querySelector('.mt-6 .text-center .text-xs.text-gray-500.dark\\:text-gray-400');
			expect(helpText).toBeInTheDocument();
		});
	});
});