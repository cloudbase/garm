import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
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

// Mock the page stores
vi.mock('$app/navigation', () => ({
	goto: vi.fn()
}));

vi.mock('$app/paths', () => ({
	resolve: vi.fn((path: string) => path)
}));

// Mock the auth store
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

// Mock toast store
vi.mock('$lib/stores/toast.js', () => ({
	toastStore: {
		success: vi.fn()
	}
}));

// Mock utilities
vi.mock('$lib/utils/apiError', () => ({
	extractAPIError: vi.fn((err) => err.message || 'Unknown error')
}));

// Reset any component mocks that might be set by setup.ts
vi.unmock('$lib/components/Button.svelte');

// Global setup for each test
let auth: any;
let authStore: any;
let goto: any;
let resolve: any;
let toastStore: any;

describe('Init Page - Unit Tests', () => {
	beforeEach(async () => {
		vi.clearAllMocks();
		
		// Set up mocks
		const authModule = await import('$lib/stores/auth.js');
		auth = authModule.auth;
		authStore = authModule.authStore;
		
		const navigationModule = await import('$app/navigation');
		goto = navigationModule.goto;
		
		const pathsModule = await import('$app/paths');
		resolve = pathsModule.resolve;
		
		const toastModule = await import('$lib/stores/toast.js');
		toastStore = toastModule.toastStore;
		
		// Set up default API mocks
		(auth.initialize as any).mockResolvedValue({});
		(resolve as any).mockImplementation((path: string) => path);
		
		// Mock window.location for URL auto-population
		Object.defineProperty(window, 'location', {
			value: {
				origin: 'https://garm.example.com'
			},
			writable: true
		});
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	describe('Component Initialization', () => {
		it('should render successfully', () => {
			const { container } = render(InitPage);
			expect(container).toBeInTheDocument();
		});

		it('should set page title', () => {
			render(InitPage);
			expect(document.title).toBe('Initialize GARM - First Run Setup');
		});

		it('should render init form elements', () => {
			render(InitPage);
			
			expect(screen.getByLabelText('Username')).toBeInTheDocument();
			expect(screen.getByLabelText('Email Address')).toBeInTheDocument();
			expect(screen.getByLabelText('Full Name')).toBeInTheDocument();
			expect(screen.getByLabelText('Password')).toBeInTheDocument();
			expect(screen.getByLabelText('Confirm Password')).toBeInTheDocument();
			expect(screen.getByRole('button', { name: /initialize garm/i })).toBeInTheDocument();
		});

		it('should render GARM logo and branding', () => {
			render(InitPage);
			
			expect(screen.getByText('Welcome to GARM')).toBeInTheDocument();
			expect(screen.getByText('Complete the first-run setup to get started')).toBeInTheDocument();
			expect(screen.getAllByAltText('GARM')).toHaveLength(2); // Light and dark logos
		});

		it('should render initialization info banner', () => {
			render(InitPage);
			
			expect(screen.getByText('First-Run Initialization')).toBeInTheDocument();
			expect(screen.getByText(/GARM needs to be initialized before first use/)).toBeInTheDocument();
		});
	});

	describe('Default Form Values', () => {
		it('should have default values populated', () => {
			render(InitPage);
			
			const usernameInput = screen.getByLabelText('Username') as HTMLInputElement;
			const emailInput = screen.getByLabelText('Email Address') as HTMLInputElement;
			const fullNameInput = screen.getByLabelText('Full Name') as HTMLInputElement;
			
			expect(usernameInput.value).toBe('admin');
			expect(emailInput.value).toBe('admin@garm.local');
			expect(fullNameInput.value).toBe('Administrator');
		});

		it('should have empty password fields by default', () => {
			render(InitPage);
			
			const passwordInput = screen.getByLabelText('Password') as HTMLInputElement;
			const confirmPasswordInput = screen.getByLabelText('Confirm Password') as HTMLInputElement;
			
			expect(passwordInput.value).toBe('');
			expect(confirmPasswordInput.value).toBe('');
		});
	});

	describe('Authentication Redirect Logic', () => {
		it('should redirect to dashboard when user is already authenticated', () => {
			vi.mocked(authStore.subscribe).mockImplementation((callback: (state: any) => void) => {
				callback(createMockAuthState({ isAuthenticated: true, user: 'testuser' }));
				return () => {};
			});
			
			render(InitPage);
			
			expect(goto).toHaveBeenCalledWith('/');
		});

		it('should redirect to login when initialization not needed', () => {
			vi.mocked(authStore.subscribe).mockImplementation((callback: (state: any) => void) => {
				callback(createMockAuthState({ needsInitialization: false, loading: false }));
				return () => {};
			});
			
			render(InitPage);
			
			expect(goto).toHaveBeenCalledWith('/login');
		});

		it('should stay on page when initialization is needed', () => {
			vi.mocked(authStore.subscribe).mockImplementation((callback: (state: any) => void) => {
				callback(createMockAuthState({ needsInitialization: true, loading: false }));
				return () => {};
			});
			
			render(InitPage);
			
			expect(goto).not.toHaveBeenCalled();
		});
	});

	describe('Form Validation', () => {
		it('should validate username field', async () => {
			render(InitPage);
			
			const usernameInput = screen.getByLabelText('Username');
			
			// Make field invalid with whitespace (will be trimmed to empty but has length > 0)
			await fireEvent.input(usernameInput, { target: { value: ' ' } });
			
			expect(screen.getByText('Username is required')).toBeInTheDocument();
		});

		it('should validate email field', async () => {
			render(InitPage);
			
			const emailInput = screen.getByLabelText('Email Address');
			
			// Enter invalid email
			await fireEvent.input(emailInput, { target: { value: 'invalid-email' } });
			
			expect(screen.getByText('Please enter a valid email address')).toBeInTheDocument();
		});

		it('should validate full name field', async () => {
			render(InitPage);
			
			const fullNameInput = screen.getByLabelText('Full Name');
			
			// Make field invalid with whitespace (will be trimmed to empty but has length > 0)
			await fireEvent.input(fullNameInput, { target: { value: ' ' } });
			
			expect(screen.getByText('Full name is required')).toBeInTheDocument();
		});

		it('should validate password length', async () => {
			render(InitPage);
			
			const passwordInput = screen.getByLabelText('Password');
			
			// Enter short password
			await fireEvent.input(passwordInput, { target: { value: '123' } });
			
			expect(screen.getByText('Password must be at least 8 characters long')).toBeInTheDocument();
		});

		it('should validate password confirmation', async () => {
			render(InitPage);
			
			const passwordInput = screen.getByLabelText('Password');
			const confirmPasswordInput = screen.getByLabelText('Confirm Password');
			
			// Enter mismatching passwords
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			await fireEvent.input(confirmPasswordInput, { target: { value: 'different123' } });
			
			expect(screen.getByText('Passwords do not match')).toBeInTheDocument();
		});

		it('should show validation summary when form is invalid', async () => {
			render(InitPage);
			
			// Make username invalid with whitespace to trigger validation summary
			const usernameInput = screen.getByLabelText('Username');
			await fireEvent.input(usernameInput, { target: { value: ' ' } });
			
			expect(screen.getByText('Please complete all required fields')).toBeInTheDocument();
		});
	});

	describe('Advanced Configuration', () => {
		it('should toggle advanced configuration panel', async () => {
			render(InitPage);
			
			const toggleButton = screen.getByRole('button', { name: /advanced configuration/i });
			
			// Advanced section should not be visible initially
			expect(screen.queryByLabelText('Metadata URL')).not.toBeInTheDocument();
			
			// Click to show advanced section
			await fireEvent.click(toggleButton);
			
			expect(screen.getByLabelText('Metadata URL')).toBeInTheDocument();
			expect(screen.getByLabelText('Callback URL')).toBeInTheDocument();
			expect(screen.getByLabelText('Webhook URL')).toBeInTheDocument();
		});

		it('should auto-populate URL fields', async () => {
			render(InitPage);
			
			const toggleButton = screen.getByRole('button', { name: /advanced configuration/i });
			await fireEvent.click(toggleButton);
			
			const metadataInput = screen.getByLabelText('Metadata URL') as HTMLInputElement;
			const callbackInput = screen.getByLabelText('Callback URL') as HTMLInputElement;
			const webhookInput = screen.getByLabelText('Webhook URL') as HTMLInputElement;
			
			expect(metadataInput.value).toBe('https://garm.example.com/api/v1/metadata');
			expect(callbackInput.value).toBe('https://garm.example.com/api/v1/callbacks');
			expect(webhookInput.value).toBe('https://garm.example.com/webhooks');
		});
	});

	describe('Form Submission', () => {
		it('should call auth.initialize with correct parameters on successful submission', async () => {
			render(InitPage);
			
			// Fill in valid form data
			const passwordInput = screen.getByLabelText('Password');
			const confirmPasswordInput = screen.getByLabelText('Confirm Password');
			
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			await fireEvent.input(confirmPasswordInput, { target: { value: 'password123' } });
			
			const submitButton = screen.getByRole('button', { name: /initialize garm/i });
			await fireEvent.click(submitButton);
			
			expect(auth.initialize).toHaveBeenCalledWith(
				'admin',
				'admin@garm.local',
				'password123',
				'Administrator',
				{
					callbackUrl: 'https://garm.example.com/api/v1/callbacks',
					metadataUrl: 'https://garm.example.com/api/v1/metadata',
					webhookUrl: 'https://garm.example.com/webhooks',
					agentUrl: 'https://garm.example.com/agent'
				}
			);
		});

		it('should show success toast and redirect on successful initialization', async () => {
			render(InitPage);
			
			// Fill in valid form data
			const passwordInput = screen.getByLabelText('Password');
			const confirmPasswordInput = screen.getByLabelText('Confirm Password');
			
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			await fireEvent.input(confirmPasswordInput, { target: { value: 'password123' } });
			
			const submitButton = screen.getByRole('button', { name: /initialize garm/i });
			await fireEvent.click(submitButton);
			
			// Wait for async operations
			await new Promise(resolve => setTimeout(resolve, 0));
			
			expect(toastStore.success).toHaveBeenCalledWith(
				'GARM Initialized',
				'GARM has been successfully initialized. Welcome!'
			);
			// Note: redirect now happens via layout reactive statements, not direct goto() call
		});

		it('should handle initialization errors', async () => {
			const error = new Error('Initialization failed');
			(auth.initialize as any).mockRejectedValue(error);
			
			render(InitPage);
			
			// Fill in valid form data
			const passwordInput = screen.getByLabelText('Password');
			const confirmPasswordInput = screen.getByLabelText('Confirm Password');
			
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			await fireEvent.input(confirmPasswordInput, { target: { value: 'password123' } });
			
			const submitButton = screen.getByRole('button', { name: /initialize garm/i });
			await fireEvent.click(submitButton);
			
			// Wait for error to appear
			await screen.findByText('Initialization failed');
			expect(goto).not.toHaveBeenCalled();
		});

		it('should not submit if form is invalid', async () => {
			render(InitPage);
			
			// Leave passwords empty to make form invalid
			const submitButton = screen.getByRole('button', { name: /initialize garm/i });
			await fireEvent.click(submitButton);
			
			expect(auth.initialize).not.toHaveBeenCalled();
		});
	});

	describe('Loading States', () => {
		it('should show loading state during initialization', async () => {
			// Mock initialize to return a promise that doesn't resolve immediately
			let resolveInitialize: () => void;
			const initializePromise = new Promise<void>((resolve) => {
				resolveInitialize = resolve;
			});
			(auth.initialize as any).mockReturnValue(initializePromise);
			
			render(InitPage);
			
			// Fill in valid form data
			const passwordInput = screen.getByLabelText('Password');
			const confirmPasswordInput = screen.getByLabelText('Confirm Password');
			
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			await fireEvent.input(confirmPasswordInput, { target: { value: 'password123' } });
			
			const submitButton = screen.getByRole('button', { name: /initialize garm/i });
			await fireEvent.click(submitButton);
			
			// Should show loading state
			await screen.findByText('Initializing...');
			expect(submitButton).toBeDisabled();
			
			// Complete the initialization
			resolveInitialize!();
			await initializePromise;
		});

		it('should clear loading state after initialization failure', async () => {
			const error = new Error('Initialization failed');
			(auth.initialize as any).mockRejectedValue(error);
			
			render(InitPage);
			
			// Fill in valid form data
			const passwordInput = screen.getByLabelText('Password');
			const confirmPasswordInput = screen.getByLabelText('Confirm Password');
			
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			await fireEvent.input(confirmPasswordInput, { target: { value: 'password123' } });
			
			const submitButton = screen.getByRole('button', { name: /initialize garm/i });
			await fireEvent.click(submitButton);
			
			// Wait for error handling
			await screen.findByText('Initialization failed');
			
			// Should not be in loading state anymore
			expect(screen.queryByText('Initializing...')).not.toBeInTheDocument();
			expect(screen.getByText('Initialize GARM')).toBeInTheDocument();
			expect(submitButton).not.toBeDisabled();
		});
	});

	describe('Error Display', () => {
		it('should clear error when starting new initialization attempt', async () => {
			// First, cause an error
			const error = new Error('Initialization failed');
			(auth.initialize as any).mockRejectedValue(error);
			
			render(InitPage);
			
			// Fill in valid form data
			const passwordInput = screen.getByLabelText('Password');
			const confirmPasswordInput = screen.getByLabelText('Confirm Password');
			const submitButton = screen.getByRole('button', { name: /initialize garm/i });
			
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			await fireEvent.input(confirmPasswordInput, { target: { value: 'password123' } });
			
			// Trigger error
			await fireEvent.click(submitButton);
			await screen.findByText('Initialization failed');
			
			// Now mock success and try again
			(auth.initialize as any).mockResolvedValue({});
			await fireEvent.click(submitButton);
			
			// Wait for async operations and error should be cleared
			await new Promise(resolve => setTimeout(resolve, 0));
			expect(screen.queryByText('Initialization failed')).not.toBeInTheDocument();
		});

		it('should display API errors with proper formatting', async () => {
			const error = new Error('Server temporarily unavailable');
			(auth.initialize as any).mockRejectedValue(error);
			
			render(InitPage);
			
			// Fill in valid form data
			const passwordInput = screen.getByLabelText('Password');
			const confirmPasswordInput = screen.getByLabelText('Confirm Password');
			const submitButton = screen.getByRole('button', { name: /initialize garm/i });
			
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			await fireEvent.input(confirmPasswordInput, { target: { value: 'password123' } });
			
			// Enter credentials and submit
			await fireEvent.click(submitButton);
			
			// Should display error message
			const errorElement = await screen.findByText('Server temporarily unavailable');
			expect(errorElement).toBeInTheDocument();
			
			// Should have proper error styling
			const errorContainer = errorElement.closest('.bg-red-50');
			expect(errorContainer).toBeInTheDocument();
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

		it('should subscribe to auth store on mount', () => {
			render(InitPage);
			expect(authStore.subscribe).toHaveBeenCalled();
		});
	});

	describe('Form State Management', () => {
		it('should maintain form state during interactions', async () => {
			render(InitPage);
			
			const usernameInput = screen.getByLabelText('Username') as HTMLInputElement;
			const emailInput = screen.getByLabelText('Email Address') as HTMLInputElement;
			
			// Enter values
			await fireEvent.input(usernameInput, { target: { value: 'testuser' } });
			await fireEvent.input(emailInput, { target: { value: 'test@example.com' } });
			
			// Values should be maintained
			expect(usernameInput.value).toBe('testuser');
			expect(emailInput.value).toBe('test@example.com');
		});

		it('should update button state based on form validity', async () => {
			render(InitPage);
			
			const submitButton = screen.getByRole('button', { name: /initialize garm/i });
			
			// Button should be disabled initially (no passwords)
			expect(submitButton).toBeDisabled();
			
			// Fill in passwords to make form valid
			const passwordInput = screen.getByLabelText('Password');
			const confirmPasswordInput = screen.getByLabelText('Confirm Password');
			
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			await fireEvent.input(confirmPasswordInput, { target: { value: 'password123' } });
			
			// Button should now be enabled
			expect(submitButton).not.toBeDisabled();
		});
	});

	describe('URL Auto-population', () => {
		it('should update URLs when window.location changes', async () => {
			const { unmount } = render(InitPage);
			
			const toggleButton = screen.getByRole('button', { name: /advanced configuration/i });
			await fireEvent.click(toggleButton);
			
			// Check initial URLs
			const metadataInput = screen.getByLabelText('Metadata URL') as HTMLInputElement;
			expect(metadataInput.value).toBe('https://garm.example.com/api/v1/metadata');
			
			// Clean up first render
			unmount();
			
			// Simulate location change (this would happen in real browser)
			Object.defineProperty(window, 'location', {
				value: {
					origin: 'https://new-garm.example.com'
				},
				writable: true
			});
			
			// Re-render component to trigger reactive updates
			render(InitPage);
			
			const toggleButton2 = screen.getByRole('button', { name: /advanced configuration/i });
			await fireEvent.click(toggleButton2);
			
			const metadataInput2 = screen.getByLabelText('Metadata URL') as HTMLInputElement;
			expect(metadataInput2.value).toBe('https://new-garm.example.com/api/v1/metadata');
		});
	});
});