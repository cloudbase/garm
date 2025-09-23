import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/svelte';
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

// Mock app stores and navigation
vi.mock('$app/navigation', () => ({
	goto: vi.fn()
}));

vi.mock('$app/paths', () => ({
	resolve: vi.fn((path: string) => path)
}));

// Reset any component mocks that might be set by setup.ts
vi.unmock('$lib/components/Button.svelte');

// Only mock the auth store and API
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

// Global setup for each test
let auth: any;
let authStore: any;
let goto: any;
let resolve: any;
let toastStore: any;
let extractAPIError: any;

describe('Comprehensive Integration Tests for Init Page', () => {
	beforeEach(async () => {
		vi.clearAllMocks();
		
		// Set up API mocks with default successful responses
		const authModule = await import('$lib/stores/auth.js');
		auth = authModule.auth;
		authStore = authModule.authStore;
		
		const navigationModule = await import('$app/navigation');
		goto = navigationModule.goto;
		
		const pathsModule = await import('$app/paths');
		resolve = pathsModule.resolve;
		
		const toastModule = await import('$lib/stores/toast.js');
		toastStore = toastModule.toastStore;
		
		const apiErrorModule = await import('$lib/utils/apiError');
		extractAPIError = apiErrorModule.extractAPIError;
		
		(auth.initialize as any).mockResolvedValue({});
		(resolve as any).mockImplementation((path: string) => path);
		(extractAPIError as any).mockImplementation((err: any) => err.message || 'Unknown error');
		
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

	describe('Component Rendering and Integration', () => {
		it('should render init page with real components', async () => {
			render(InitPage);

			await waitFor(() => {
				// Should render all main components
				expect(screen.getByRole('heading', { name: 'Welcome to GARM' })).toBeInTheDocument();
				expect(screen.getByText('Complete the first-run setup to get started')).toBeInTheDocument();
				expect(screen.getByLabelText('Username')).toBeInTheDocument();
				expect(screen.getByLabelText('Email Address')).toBeInTheDocument();
				expect(screen.getByLabelText('Full Name')).toBeInTheDocument();
				expect(screen.getByLabelText('Password')).toBeInTheDocument();
				expect(screen.getByLabelText('Confirm Password')).toBeInTheDocument();
				expect(screen.getByRole('button', { name: /initialize garm/i })).toBeInTheDocument();
			});
		});

		it('should render proper logo integration', async () => {
			render(InitPage);

			await waitFor(() => {
				const logos = screen.getAllByAltText('GARM');
				expect(logos).toHaveLength(2);
				
				// Should have proper src paths resolved
				expect(resolve).toHaveBeenCalledWith('/assets/garm-light.svg');
				expect(resolve).toHaveBeenCalledWith('/assets/garm-dark.svg');
			});
		});

		it('should integrate all form components properly', async () => {
			render(InitPage);

			await waitFor(() => {
				// All form elements should be integrated
				const form = document.querySelector('form');
				const usernameInput = screen.getByLabelText('Username');
				const emailInput = screen.getByLabelText('Email Address');
				const submitButton = screen.getByRole('button', { name: /initialize garm/i });
				
				expect(form).toBeInTheDocument();
				expect(usernameInput).toBeInTheDocument();
				expect(emailInput).toBeInTheDocument();
				expect(submitButton).toBeInTheDocument();
			});
		});

		it('should integrate info banner with proper styling', async () => {
			render(InitPage);

			await waitFor(() => {
				const infoBanner = screen.getByText('First-Run Initialization');
				expect(infoBanner).toBeInTheDocument();
				
				// Should have proper banner styling container
				const bannerContainer = infoBanner.closest('.bg-blue-50');
				expect(bannerContainer).toBeInTheDocument();
			});
		});
	});

	describe('Authentication State Integration', () => {
		it('should handle initialization required state', async () => {
			vi.mocked(authStore.subscribe).mockImplementation((callback: (state: any) => void) => {
				callback(createMockAuthState({ needsInitialization: true, loading: false }));
				return () => {};
			});

			render(InitPage);

			await waitFor(() => {
				// Should stay on page and render form
				expect(screen.getByRole('heading', { name: 'Welcome to GARM' })).toBeInTheDocument();
				expect(goto).not.toHaveBeenCalled();
			});
		});

		it('should handle authentication redirect integration', async () => {
			// Mock already authenticated user
			vi.mocked(authStore.subscribe).mockImplementation((callback: (state: any) => void) => {
				callback(createMockAuthState({ isAuthenticated: true, user: 'testuser' }));
				return () => {};
			});

			render(InitPage);

			await waitFor(() => {
				// Should automatically redirect to dashboard
				expect(goto).toHaveBeenCalledWith('/');
			});
		});

		it('should handle redirect to login when initialization not needed', async () => {
			// Mock state where initialization is not needed
			vi.mocked(authStore.subscribe).mockImplementation((callback: (state: any) => void) => {
				callback(createMockAuthState({ needsInitialization: false, loading: false }));
				return () => {};
			});

			render(InitPage);

			await waitFor(() => {
				// Should redirect to login page
				expect(goto).toHaveBeenCalledWith('/login');
			});
		});

		it('should handle reactive auth state changes', async () => {
			// Mock store that changes state
			let callback: (state: any) => void;
			vi.mocked(authStore.subscribe).mockImplementation((cb: (state: any) => void) => {
				callback = cb;
				cb(createMockAuthState({ needsInitialization: true, loading: false }));
				return () => {};
			});

			render(InitPage);

			await waitFor(() => {
				expect(authStore.subscribe).toHaveBeenCalled();
			});

			// Simulate auth state change to authenticated
			callback!(createMockAuthState({ isAuthenticated: true, user: 'testuser' }));

			await waitFor(() => {
				expect(goto).toHaveBeenCalledWith('/');
			});
		});
	});

	describe('Form Validation Integration', () => {
		it('should integrate real-time validation feedback', async () => {
			render(InitPage);

			await waitFor(() => {
				expect(screen.getByLabelText('Username')).toBeInTheDocument();
			});

			const usernameInput = screen.getByLabelText('Username');
			
			// Make field invalid with whitespace (will be trimmed to empty but has length > 0)
			await fireEvent.input(usernameInput, { target: { value: ' ' } });

			await waitFor(() => {
				expect(screen.getByText('Username is required')).toBeInTheDocument();
			});
		});

		it('should integrate email validation with UI feedback', async () => {
			render(InitPage);

			await waitFor(() => {
				expect(screen.getByLabelText('Email Address')).toBeInTheDocument();
			});

			const emailInput = screen.getByLabelText('Email Address');
			
			// Enter invalid email
			await fireEvent.input(emailInput, { target: { value: 'invalid-email' } });

			await waitFor(() => {
				expect(screen.getByText('Please enter a valid email address')).toBeInTheDocument();
			});
		});

		it('should integrate password validation workflow', async () => {
			render(InitPage);

			await waitFor(() => {
				expect(screen.getByLabelText('Password')).toBeInTheDocument();
			});

			const passwordInput = screen.getByLabelText('Password');
			const confirmPasswordInput = screen.getByLabelText('Confirm Password');
			
			// Test password length validation
			await fireEvent.input(passwordInput, { target: { value: 'short' } });

			await waitFor(() => {
				expect(screen.getByText('Password must be at least 8 characters long')).toBeInTheDocument();
			});

			// Test password confirmation validation
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			await fireEvent.input(confirmPasswordInput, { target: { value: 'different123' } });

			await waitFor(() => {
				expect(screen.getByText('Passwords do not match')).toBeInTheDocument();
			});
		});

		it('should integrate validation summary display', async () => {
			render(InitPage);

			await waitFor(() => {
				expect(screen.getByLabelText('Username')).toBeInTheDocument();
			});

			// Make username invalid with whitespace to trigger validation summary
			const usernameInput = screen.getByLabelText('Username');
			await fireEvent.input(usernameInput, { target: { value: ' ' } });

			await waitFor(() => {
				expect(screen.getByText('Please complete all required fields')).toBeInTheDocument();
				expect(screen.getByText('Enter a username')).toBeInTheDocument();
			});
		});

		it('should integrate form validation with button state', async () => {
			render(InitPage);

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /initialize garm/i })).toBeInTheDocument();
			});

			const submitButton = screen.getByRole('button', { name: /initialize garm/i });
			
			// Button should be disabled initially (no passwords)
			expect(submitButton).toBeDisabled();

			// Fill in valid passwords
			const passwordInput = screen.getByLabelText('Password');
			const confirmPasswordInput = screen.getByLabelText('Confirm Password');
			
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			await fireEvent.input(confirmPasswordInput, { target: { value: 'password123' } });

			await waitFor(() => {
				// Button should now be enabled
				expect(submitButton).not.toBeDisabled();
			});
		});
	});

	describe('Advanced Configuration Integration', () => {
		it('should integrate advanced configuration toggle workflow', async () => {
			render(InitPage);

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /advanced configuration/i })).toBeInTheDocument();
			});

			const toggleButton = screen.getByRole('button', { name: /advanced configuration/i });
			
			// Advanced fields should not be visible initially
			expect(screen.queryByLabelText('Metadata URL')).not.toBeInTheDocument();

			// Toggle to show advanced fields
			await fireEvent.click(toggleButton);

			await waitFor(() => {
				expect(screen.getByLabelText('Metadata URL')).toBeInTheDocument();
				expect(screen.getByLabelText('Callback URL')).toBeInTheDocument();
				expect(screen.getByLabelText('Webhook URL')).toBeInTheDocument();
			});

			// Toggle to hide advanced fields
			await fireEvent.click(toggleButton);

			await waitFor(() => {
				expect(screen.queryByLabelText('Metadata URL')).not.toBeInTheDocument();
			});
		});

		it('should integrate URL auto-population with form fields', async () => {
			render(InitPage);

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /advanced configuration/i })).toBeInTheDocument();
			});

			const toggleButton = screen.getByRole('button', { name: /advanced configuration/i });
			await fireEvent.click(toggleButton);

			await waitFor(() => {
				const metadataInput = screen.getByLabelText('Metadata URL') as HTMLInputElement;
				const callbackInput = screen.getByLabelText('Callback URL') as HTMLInputElement;
				const webhookInput = screen.getByLabelText('Webhook URL') as HTMLInputElement;
				
				expect(metadataInput.value).toBe('https://garm.example.com/api/v1/metadata');
				expect(callbackInput.value).toBe('https://garm.example.com/api/v1/callbacks');
				expect(webhookInput.value).toBe('https://garm.example.com/webhooks');
			});
		});

		it('should integrate custom URL input workflow', async () => {
			render(InitPage);

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /advanced configuration/i })).toBeInTheDocument();
			});

			const toggleButton = screen.getByRole('button', { name: /advanced configuration/i });
			await fireEvent.click(toggleButton);

			await waitFor(() => {
				expect(screen.getByLabelText('Metadata URL')).toBeInTheDocument();
			});

			const metadataInput = screen.getByLabelText('Metadata URL');
			
			// User can override auto-populated URLs
			await fireEvent.input(metadataInput, { target: { value: 'https://custom.example.com/metadata' } });

			expect((metadataInput as HTMLInputElement).value).toBe('https://custom.example.com/metadata');
		});
	});

	describe('Initialization Workflow Integration', () => {
		it('should handle complete initialization workflow', async () => {
			render(InitPage);

			await waitFor(() => {
				expect(screen.getByLabelText('Password')).toBeInTheDocument();
			});

			// Fill in valid form data
			const passwordInput = screen.getByLabelText('Password');
			const confirmPasswordInput = screen.getByLabelText('Confirm Password');
			const submitButton = screen.getByRole('button', { name: /initialize garm/i });

			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			await fireEvent.input(confirmPasswordInput, { target: { value: 'password123' } });

			// Submit form
			await fireEvent.click(submitButton);

			// Should call auth.initialize with correct parameters
			await waitFor(() => {
				expect(auth.initialize).toHaveBeenCalledWith(
					'admin',
					'admin@garm.local',
					'password123',
					'Administrator',
					{
						callbackUrl: 'https://garm.example.com/api/v1/callbacks',
						metadataUrl: 'https://garm.example.com/api/v1/metadata',
						webhookUrl: 'https://garm.example.com/webhooks'
					}
				);
			});
		});

		it('should integrate success workflow with toast and redirect', async () => {
			render(InitPage);

			await waitFor(() => {
				expect(screen.getByLabelText('Password')).toBeInTheDocument();
			});

			// Fill in valid form data
			const passwordInput = screen.getByLabelText('Password');
			const confirmPasswordInput = screen.getByLabelText('Confirm Password');
			const submitButton = screen.getByRole('button', { name: /initialize garm/i });

			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			await fireEvent.input(confirmPasswordInput, { target: { value: 'password123' } });

			// Submit form
			await fireEvent.click(submitButton);

			// Should show toast (redirect happens via layout reactive statements)
			await waitFor(() => {
				expect(toastStore.success).toHaveBeenCalledWith(
					'GARM Initialized',
					'GARM has been successfully initialized. Welcome!'
				);
				// Note: redirect now happens via layout reactive statements, not direct goto() call
			});
		});

		it('should integrate error handling with UI display', async () => {
			const error = new Error('Initialization failed');
			(auth.initialize as any).mockRejectedValue(error);

			render(InitPage);

			await waitFor(() => {
				expect(screen.getByLabelText('Password')).toBeInTheDocument();
			});

			// Fill in valid form data
			const passwordInput = screen.getByLabelText('Password');
			const confirmPasswordInput = screen.getByLabelText('Confirm Password');
			const submitButton = screen.getByRole('button', { name: /initialize garm/i });

			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			await fireEvent.input(confirmPasswordInput, { target: { value: 'password123' } });

			// Submit form
			await fireEvent.click(submitButton);

			// Should display error in UI
			await waitFor(() => {
				expect(screen.getByText('Initialization failed')).toBeInTheDocument();
			});

			// Should extract API error properly
			expect(extractAPIError).toHaveBeenCalledWith(error);
			expect(goto).not.toHaveBeenCalled();
		});

		it('should handle loading state integration', async () => {
			// Mock delayed initialization
			let resolveInitialize: () => void;
			const initializePromise = new Promise<void>((resolve) => {
				resolveInitialize = resolve;
			});
			(auth.initialize as any).mockReturnValue(initializePromise);

			render(InitPage);

			await waitFor(() => {
				expect(screen.getByLabelText('Password')).toBeInTheDocument();
			});

			// Fill in valid form data
			const passwordInput = screen.getByLabelText('Password');
			const confirmPasswordInput = screen.getByLabelText('Confirm Password');
			const submitButton = screen.getByRole('button', { name: /initialize garm/i });

			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			await fireEvent.input(confirmPasswordInput, { target: { value: 'password123' } });

			// Submit form
			await fireEvent.click(submitButton);

			// Should show loading state
			await waitFor(() => {
				expect(screen.getByText('Initializing...')).toBeInTheDocument();
				expect(submitButton).toBeDisabled();
			});

			// Complete initialization
			resolveInitialize!();
			await initializePromise;
		});
	});

	describe('Advanced Configuration Workflow Integration', () => {
		it('should integrate advanced configuration in initialization', async () => {
			render(InitPage);

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /advanced configuration/i })).toBeInTheDocument();
			});

			// Enable advanced configuration
			const toggleButton = screen.getByRole('button', { name: /advanced configuration/i });
			await fireEvent.click(toggleButton);

			await waitFor(() => {
				expect(screen.getByLabelText('Metadata URL')).toBeInTheDocument();
			});

			// Customize URLs
			const metadataInput = screen.getByLabelText('Metadata URL');
			const callbackInput = screen.getByLabelText('Callback URL');
			
			await fireEvent.input(metadataInput, { target: { value: 'https://custom.example.com/metadata' } });
			await fireEvent.input(callbackInput, { target: { value: 'https://custom.example.com/callbacks' } });

			// Fill in required fields
			const passwordInput = screen.getByLabelText('Password');
			const confirmPasswordInput = screen.getByLabelText('Confirm Password');
			
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			await fireEvent.input(confirmPasswordInput, { target: { value: 'password123' } });

			// Submit form
			const submitButton = screen.getByRole('button', { name: /initialize garm/i });
			await fireEvent.click(submitButton);

			// Should use custom URLs in initialization
			await waitFor(() => {
				expect(auth.initialize).toHaveBeenCalledWith(
					'admin',
					'admin@garm.local',
					'password123',
					'Administrator',
					{
						callbackUrl: 'https://custom.example.com/callbacks',
						metadataUrl: 'https://custom.example.com/metadata',
						webhookUrl: 'https://garm.example.com/webhooks'
					}
				);
			});
		});

		it('should integrate empty URL handling in advanced config', async () => {
			render(InitPage);

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /advanced configuration/i })).toBeInTheDocument();
			});

			// Enable advanced configuration
			const toggleButton = screen.getByRole('button', { name: /advanced configuration/i });
			await fireEvent.click(toggleButton);

			await waitFor(() => {
				expect(screen.getByLabelText('Metadata URL')).toBeInTheDocument();
			});

			// URLs are auto-populated, verify they have default values
			const metadataInput = screen.getByLabelText('Metadata URL') as HTMLInputElement;
			const callbackInput = screen.getByLabelText('Callback URL') as HTMLInputElement;
			const webhookInput = screen.getByLabelText('Webhook URL') as HTMLInputElement;
			
			// Verify auto-population works
			expect(metadataInput.value).toBe('https://garm.example.com/api/v1/metadata');
			expect(callbackInput.value).toBe('https://garm.example.com/api/v1/callbacks');
			expect(webhookInput.value).toBe('https://garm.example.com/webhooks');

			// Fill in required fields
			const passwordInput = screen.getByLabelText('Password');
			const confirmPasswordInput = screen.getByLabelText('Confirm Password');
			
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			await fireEvent.input(confirmPasswordInput, { target: { value: 'password123' } });

			// Submit form
			const submitButton = screen.getByRole('button', { name: /initialize garm/i });
			await fireEvent.click(submitButton);

			// Should use auto-populated URLs (component design prevents empty URLs)
			await waitFor(() => {
				expect(auth.initialize).toHaveBeenCalledWith(
					'admin',
					'admin@garm.local',
					'password123',
					'Administrator',
					{
						callbackUrl: 'https://garm.example.com/api/v1/callbacks',
						metadataUrl: 'https://garm.example.com/api/v1/metadata',
						webhookUrl: 'https://garm.example.com/webhooks'
					}
				);
			});
		});
	});

	describe('Form State Management Integration', () => {
		it('should maintain form state during validation interactions', async () => {
			render(InitPage);

			await waitFor(() => {
				expect(screen.getByLabelText('Username')).toBeInTheDocument();
			});

			const usernameInput = screen.getByLabelText('Username') as HTMLInputElement;
			const emailInput = screen.getByLabelText('Email Address') as HTMLInputElement;
			
			// Change values
			await fireEvent.input(usernameInput, { target: { value: 'testuser' } });
			await fireEvent.input(emailInput, { target: { value: 'test@example.com' } });

			// Values should be maintained
			expect(usernameInput.value).toBe('testuser');
			expect(emailInput.value).toBe('test@example.com');

			// Trigger validation with whitespace in username field
			await fireEvent.input(usernameInput, { target: { value: ' ' } });

			// Should show validation but maintain other field values
			await waitFor(() => {
				expect(screen.getByText('Username is required')).toBeInTheDocument();
				expect(emailInput.value).toBe('test@example.com'); // Other field maintained
			});
		});

		it('should integrate form submission prevention when invalid', async () => {
			render(InitPage);

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /initialize garm/i })).toBeInTheDocument();
			});

			const submitButton = screen.getByRole('button', { name: /initialize garm/i });
			
			// Form should be invalid initially (no passwords)
			expect(submitButton).toBeDisabled();

			// Try to submit (should not call API)
			await fireEvent.click(submitButton);

			// Should not call initialize API
			expect(auth.initialize).not.toHaveBeenCalled();
		});

		it('should handle form state persistence during advanced toggle', async () => {
			render(InitPage);

			await waitFor(() => {
				expect(screen.getByLabelText('Username')).toBeInTheDocument();
			});

			// Fill in form data
			const usernameInput = screen.getByLabelText('Username') as HTMLInputElement;
			await fireEvent.input(usernameInput, { target: { value: 'testuser' } });

			// Toggle advanced configuration
			const toggleButton = screen.getByRole('button', { name: /advanced configuration/i });
			await fireEvent.click(toggleButton);

			await waitFor(() => {
				expect(screen.getByLabelText('Metadata URL')).toBeInTheDocument();
			});

			// Toggle back
			await fireEvent.click(toggleButton);

			// Form data should be maintained
			expect(usernameInput.value).toBe('testuser');
		});
	});

	describe('Error Handling Integration', () => {
		it('should integrate API error extraction and display', async () => {
			const error = new Error('Server error occurred');
			(auth.initialize as any).mockRejectedValue(error);
			(extractAPIError as any).mockReturnValue('Server error occurred');

			render(InitPage);

			await waitFor(() => {
				expect(screen.getByLabelText('Password')).toBeInTheDocument();
			});

			// Fill in valid form data
			const passwordInput = screen.getByLabelText('Password');
			const confirmPasswordInput = screen.getByLabelText('Confirm Password');
			const submitButton = screen.getByRole('button', { name: /initialize garm/i });

			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			await fireEvent.input(confirmPasswordInput, { target: { value: 'password123' } });

			// Submit form
			await fireEvent.click(submitButton);

			// Should extract and display error
			await waitFor(() => {
				expect(extractAPIError).toHaveBeenCalledWith(error);
				expect(screen.getByText('Server error occurred')).toBeInTheDocument();
			});
		});

		it('should handle error state recovery', async () => {
			// First cause an error
			const error = new Error('First error');
			(auth.initialize as any).mockRejectedValue(error);

			render(InitPage);

			await waitFor(() => {
				expect(screen.getByLabelText('Password')).toBeInTheDocument();
			});

			// Fill in valid form data
			const passwordInput = screen.getByLabelText('Password');
			const confirmPasswordInput = screen.getByLabelText('Confirm Password');
			const submitButton = screen.getByRole('button', { name: /initialize garm/i });

			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			await fireEvent.input(confirmPasswordInput, { target: { value: 'password123' } });

			// Trigger error
			await fireEvent.click(submitButton);

			await waitFor(() => {
				expect(screen.getByText('First error')).toBeInTheDocument();
			});

			// Now mock success and try again
			(auth.initialize as any).mockResolvedValue({});
			await fireEvent.click(submitButton);

			// Error should be cleared
			await waitFor(() => {
				expect(screen.queryByText('First error')).not.toBeInTheDocument();
			});
		});

		it('should integrate error styling with theme', async () => {
			const error = new Error('Initialization failed');
			(auth.initialize as any).mockRejectedValue(error);

			render(InitPage);

			await waitFor(() => {
				expect(screen.getByLabelText('Password')).toBeInTheDocument();
			});

			// Fill in valid form data and submit
			const passwordInput = screen.getByLabelText('Password');
			const confirmPasswordInput = screen.getByLabelText('Confirm Password');
			const submitButton = screen.getByRole('button', { name: /initialize garm/i });

			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			await fireEvent.input(confirmPasswordInput, { target: { value: 'password123' } });
			await fireEvent.click(submitButton);

			// Should display error with proper styling
			await waitFor(() => {
				const errorMessage = screen.getByText('Initialization failed');
				expect(errorMessage).toBeInTheDocument();
				
				// Should have proper error styling container
				const errorContainer = errorMessage.closest('.bg-red-50');
				expect(errorContainer).toBeInTheDocument();
			});
		});
	});

	describe('Navigation Integration', () => {
		it('should integrate path resolution', async () => {
			render(InitPage);

			await waitFor(() => {
				// Should resolve asset paths
				expect(resolve).toHaveBeenCalledWith('/assets/garm-light.svg');
				expect(resolve).toHaveBeenCalledWith('/assets/garm-dark.svg');
			});
		});

		it('should handle navigation on successful initialization', async () => {
			render(InitPage);

			await waitFor(() => {
				expect(screen.getByLabelText('Password')).toBeInTheDocument();
			});

			// Fill in valid form data
			const passwordInput = screen.getByLabelText('Password');
			const confirmPasswordInput = screen.getByLabelText('Confirm Password');
			const submitButton = screen.getByRole('button', { name: /initialize garm/i });

			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			await fireEvent.input(confirmPasswordInput, { target: { value: 'password123' } });

			// Submit form
			await fireEvent.click(submitButton);

			// Should call auth.initialize successfully (navigation happens via layout reactive statements)
			await waitFor(() => {
				expect(auth.initialize).toHaveBeenCalled();
			});
		});

		it('should integrate automatic redirect for authenticated users', async () => {
			// Mock authenticated user from start
			vi.mocked(authStore.subscribe).mockImplementation((callback: (state: any) => void) => {
				callback(createMockAuthState({ isAuthenticated: true, user: 'existinguser' }));
				return () => {};
			});

			render(InitPage);

			// Should immediately redirect
			await waitFor(() => {
				expect(goto).toHaveBeenCalledWith('/');
			});
		});
	});

	describe('Toast Integration', () => {
		it('should integrate toast notifications with initialization success', async () => {
			render(InitPage);

			await waitFor(() => {
				expect(screen.getByLabelText('Password')).toBeInTheDocument();
			});

			// Fill in valid form data
			const passwordInput = screen.getByLabelText('Password');
			const confirmPasswordInput = screen.getByLabelText('Confirm Password');
			const submitButton = screen.getByRole('button', { name: /initialize garm/i });

			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			await fireEvent.input(confirmPasswordInput, { target: { value: 'password123' } });

			// Submit form
			await fireEvent.click(submitButton);

			// Should show success toast
			await waitFor(() => {
				expect(toastStore.success).toHaveBeenCalledWith(
					'GARM Initialized',
					'GARM has been successfully initialized. Welcome!'
				);
			});
		});

		it('should not show toast on initialization errors', async () => {
			const error = new Error('Initialization failed');
			(auth.initialize as any).mockRejectedValue(error);

			render(InitPage);

			await waitFor(() => {
				expect(screen.getByLabelText('Password')).toBeInTheDocument();
			});

			// Fill in valid form data
			const passwordInput = screen.getByLabelText('Password');
			const confirmPasswordInput = screen.getByLabelText('Confirm Password');
			const submitButton = screen.getByRole('button', { name: /initialize garm/i });

			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			await fireEvent.input(confirmPasswordInput, { target: { value: 'password123' } });

			// Submit form
			await fireEvent.click(submitButton);

			// Wait for error
			await screen.findByText('Initialization failed');

			// Should not show success toast
			expect(toastStore.success).not.toHaveBeenCalled();
		});
	});

	describe('Component Lifecycle Integration', () => {
		it('should handle complete component lifecycle', () => {
			const { unmount } = render(InitPage);

			// Should mount without errors
			expect(screen.getByRole('heading', { name: 'Welcome to GARM' })).toBeInTheDocument();

			// Should unmount without errors
			expect(() => unmount()).not.toThrow();
		});

		it('should integrate auth store subscription lifecycle', async () => {
			render(InitPage);

			await waitFor(() => {
				// Should subscribe to auth store
				expect(authStore.subscribe).toHaveBeenCalled();
			});
		});

		it('should handle reactive state updates', async () => {
			// Mock store with reactive updates
			let callback: (state: any) => void;
			vi.mocked(authStore.subscribe).mockImplementation((cb: (state: any) => void) => {
				callback = cb;
				cb(createMockAuthState({ needsInitialization: true }));
				return () => {};
			});

			render(InitPage);

			await waitFor(() => {
				expect(authStore.subscribe).toHaveBeenCalled();
			});

			// Should handle reactive state change
			callback!(createMockAuthState({ isAuthenticated: true, user: 'newuser' }));

			await waitFor(() => {
				expect(goto).toHaveBeenCalledWith('/');
			});
		});
	});
});