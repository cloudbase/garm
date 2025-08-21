import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/svelte';
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
		login: vi.fn()
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
let extractAPIError: any;

// Mock DOM APIs
const mockLocalStorage = {
	getItem: vi.fn(),
	setItem: vi.fn(),
	removeItem: vi.fn()
};

const mockMatchMedia = vi.fn();

describe('Comprehensive Integration Tests for Login Page', () => {
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
		
		const apiErrorModule = await import('$lib/utils/apiError');
		extractAPIError = apiErrorModule.extractAPIError;
		
		// Mock DOM APIs
		Object.defineProperty(window, 'localStorage', { value: mockLocalStorage });
		Object.defineProperty(window, 'matchMedia', { value: mockMatchMedia });
		
		(auth.login as any).mockResolvedValue({});
		(resolve as any).mockImplementation((path: string) => path);
		(mockLocalStorage.getItem as any).mockReturnValue(null);
		(mockMatchMedia as any).mockReturnValue({ matches: false });
		(extractAPIError as any).mockImplementation((err: any) => err.message || 'Unknown error');
	});

	afterEach(() => {
		// Clean up DOM changes
		document.documentElement.classList.remove('dark');
		vi.restoreAllMocks();
	});

	describe('Component Rendering and Integration', () => {
		it('should render login page with real components', async () => {
			render(LoginPage);

			await waitFor(() => {
				// Should render all main components
				expect(screen.getByRole('heading', { name: 'Sign in to GARM' })).toBeInTheDocument();
				expect(screen.getByText('GitHub Actions Runner Manager')).toBeInTheDocument();
				expect(screen.getByLabelText('Username')).toBeInTheDocument();
				expect(screen.getByLabelText('Password')).toBeInTheDocument();
				expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument();
			});
		});

		it('should integrate theme initialization with DOM', async () => {
			render(LoginPage);
			
			await waitFor(() => {
				// Should call localStorage to check theme
				expect(mockLocalStorage.getItem).toHaveBeenCalledWith('theme');
			});

			// Should not have dark class initially (light theme)
			expect(document.documentElement.classList.contains('dark')).toBe(false);
		});

		it('should render proper logo integration', async () => {
			render(LoginPage);

			await waitFor(() => {
				const logos = screen.getAllByAltText('GARM');
				expect(logos).toHaveLength(2);
				
				// Should have proper src paths resolved
				expect(resolve).toHaveBeenCalledWith('/assets/garm-light.svg');
				expect(resolve).toHaveBeenCalledWith('/assets/garm-dark.svg');
			});
		});

		it('should integrate all form components properly', async () => {
			render(LoginPage);

			await waitFor(() => {
				// All form elements should be integrated
				const form = document.querySelector('form');
				const usernameInput = screen.getByLabelText('Username');
				const passwordInput = screen.getByLabelText('Password');
				const submitButton = screen.getByRole('button', { name: /sign in/i });
				
				expect(form).toBeInTheDocument();
				expect(usernameInput).toBeInTheDocument();
				expect(passwordInput).toBeInTheDocument();
				expect(submitButton).toBeInTheDocument();
			});
		});
	});

	describe('Authentication Workflow Integration', () => {
		it('should handle complete login workflow', async () => {
			render(LoginPage);

			await waitFor(() => {
				expect(screen.getByLabelText('Username')).toBeInTheDocument();
			});

			// Complete login workflow
			const usernameInput = screen.getByLabelText('Username');
			const passwordInput = screen.getByLabelText('Password');
			const submitButton = screen.getByRole('button', { name: /sign in/i });

			// User enters credentials
			await fireEvent.input(usernameInput, { target: { value: 'testuser' } });
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });

			// User submits form
			await fireEvent.click(submitButton);

			// Should call auth API
			expect(auth.login).toHaveBeenCalledWith('testuser', 'password123');
			
			// Should redirect on success
			expect(goto).toHaveBeenCalledWith('/');
		});

		it('should handle authentication redirect integration', async () => {
			// Mock already authenticated user
			vi.mocked(authStore.subscribe).mockImplementation((callback: (state: any) => void) => {
				callback(createMockAuthState({ isAuthenticated: true, user: 'testuser' }));
				return () => {};
			});

			render(LoginPage);

			await waitFor(() => {
				// Should automatically redirect
				expect(goto).toHaveBeenCalledWith('/');
			});
		});

		it('should integrate error handling with UI display', async () => {
			const error = new Error('Invalid credentials');
			(auth.login as any).mockRejectedValue(error);

			render(LoginPage);

			await waitFor(() => {
				expect(screen.getByLabelText('Username')).toBeInTheDocument();
			});

			const usernameInput = screen.getByLabelText('Username');
			const passwordInput = screen.getByLabelText('Password');
			const submitButton = screen.getByRole('button', { name: /sign in/i });

			// Enter credentials and submit
			await fireEvent.input(usernameInput, { target: { value: 'testuser' } });
			await fireEvent.input(passwordInput, { target: { value: 'wrongpassword' } });
			await fireEvent.click(submitButton);

			// Should display error in UI
			await waitFor(() => {
				expect(screen.getByText('Invalid credentials')).toBeInTheDocument();
			});

			// Should extract API error properly
			expect(extractAPIError).toHaveBeenCalledWith(error);
		});

		it('should handle loading state integration', async () => {
			// Mock delayed login
			let resolveLogin: () => void;
			const loginPromise = new Promise<void>((resolve) => {
				resolveLogin = resolve;
			});
			(auth.login as any).mockReturnValue(loginPromise);

			render(LoginPage);

			await waitFor(() => {
				expect(screen.getByLabelText('Username')).toBeInTheDocument();
			});

			const usernameInput = screen.getByLabelText('Username');
			const passwordInput = screen.getByLabelText('Password');
			const submitButton = screen.getByRole('button', { name: /sign in/i });

			// Enter credentials and submit
			await fireEvent.input(usernameInput, { target: { value: 'testuser' } });
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			await fireEvent.click(submitButton);

			// Should show loading state
			await waitFor(() => {
				expect(screen.getByText('Signing in...')).toBeInTheDocument();
				expect(usernameInput).toBeDisabled();
				expect(passwordInput).toBeDisabled();
			});

			// Complete login
			resolveLogin!();
			await loginPromise;
		});
	});

	describe('Theme Integration Workflows', () => {
		it('should apply dark theme from localStorage', async () => {
			(mockLocalStorage.getItem as any).mockReturnValue('dark');

			render(LoginPage);

			await waitFor(() => {
				expect(mockLocalStorage.getItem).toHaveBeenCalledWith('theme');
			});

			// Should apply dark theme to document
			expect(document.documentElement.classList.contains('dark')).toBe(true);
		});

		it('should apply light theme from localStorage', async () => {
			(mockLocalStorage.getItem as any).mockReturnValue('light');

			render(LoginPage);

			await waitFor(() => {
				expect(mockLocalStorage.getItem).toHaveBeenCalledWith('theme');
			});

			// Should remove dark theme from document
			expect(document.documentElement.classList.contains('dark')).toBe(false);
		});

		it('should use system preference when no saved theme', async () => {
			(mockLocalStorage.getItem as any).mockReturnValue(null);
			(mockMatchMedia as any).mockReturnValue({ matches: true }); // Dark system preference

			render(LoginPage);

			await waitFor(() => {
				expect(mockMatchMedia).toHaveBeenCalledWith('(prefers-color-scheme: dark)');
			});

			// Should apply dark theme based on system preference
			expect(document.documentElement.classList.contains('dark')).toBe(true);
		});

		it('should handle system preference for light theme', async () => {
			(mockLocalStorage.getItem as any).mockReturnValue(null);
			(mockMatchMedia as any).mockReturnValue({ matches: false }); // Light system preference

			render(LoginPage);

			await waitFor(() => {
				expect(mockMatchMedia).toHaveBeenCalledWith('(prefers-color-scheme: dark)');
			});

			// Should not apply dark theme
			expect(document.documentElement.classList.contains('dark')).toBe(false);
		});

		it('should handle theme integration with logo display', async () => {
			render(LoginPage);

			await waitFor(() => {
				const logos = screen.getAllByAltText('GARM');
				expect(logos).toHaveLength(2);
			});

			// Should have proper theme-aware classes
			const logos = screen.getAllByAltText('GARM');
			const lightLogo = logos.find(img => img.classList.contains('dark:hidden'));
			const darkLogo = logos.find(img => img.classList.contains('hidden'));
			
			expect(lightLogo).toBeInTheDocument();
			expect(darkLogo).toBeInTheDocument();
		});
	});

	describe('Form Interaction Integration', () => {
		it('should handle keyboard interaction workflows', async () => {
			render(LoginPage);

			await waitFor(() => {
				expect(screen.getByLabelText('Username')).toBeInTheDocument();
			});

			const usernameInput = screen.getByLabelText('Username');
			const passwordInput = screen.getByLabelText('Password');

			// Enter credentials
			await fireEvent.input(usernameInput, { target: { value: 'testuser' } });
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });

			// Press Enter in username field
			await fireEvent.keyPress(usernameInput, { key: 'Enter', code: 'Enter' });

			// Should trigger login
			expect(auth.login).toHaveBeenCalledWith('testuser', 'password123');
		});

		it('should handle form submission prevention', async () => {
			render(LoginPage);

			await waitFor(() => {
				expect(document.querySelector('form')).toBeInTheDocument();
			});

			const form = document.querySelector('form')!
			
			// Form should have proper structure for preventing default submission
			expect(form).toBeInTheDocument();
		});

		it('should integrate form validation with UI feedback', async () => {
			render(LoginPage);

			await waitFor(() => {
				expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument();
			});

			const form = document.querySelector('form')!;

			// Submit empty form via form submission
			await fireEvent.submit(form);

			// Should show validation error
			await waitFor(() => {
				expect(screen.getByText('Please enter both username and password')).toBeInTheDocument();
			});

			// Should not call auth API
			expect(auth.login).not.toHaveBeenCalled();
		});

		it('should handle partial validation scenarios', async () => {
			render(LoginPage);

			await waitFor(() => {
				expect(screen.getByLabelText('Username')).toBeInTheDocument();
			});

			const usernameInput = screen.getByLabelText('Username');
			const form = document.querySelector('form')!;

			// Enter only username
			await fireEvent.input(usernameInput, { target: { value: 'testuser' } });
			await fireEvent.submit(form);

			// Should show validation error
			await waitFor(() => {
				expect(screen.getByText('Please enter both username and password')).toBeInTheDocument();
			});

			// Should not call auth API
			expect(auth.login).not.toHaveBeenCalled();
		});
	});

	describe('Error Handling Integration', () => {
		it('should integrate API error extraction and display', async () => {
			const error = new Error('Server error occurred');
			(auth.login as any).mockRejectedValue(error);
			(extractAPIError as any).mockReturnValue('Server error occurred');

			render(LoginPage);

			await waitFor(() => {
				expect(screen.getByLabelText('Username')).toBeInTheDocument();
			});

			const usernameInput = screen.getByLabelText('Username');
			const passwordInput = screen.getByLabelText('Password');
			const submitButton = screen.getByRole('button', { name: /sign in/i });

			// Enter credentials and submit
			await fireEvent.input(usernameInput, { target: { value: 'testuser' } });
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
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
			(auth.login as any).mockRejectedValue(error);

			render(LoginPage);

			await waitFor(() => {
				expect(screen.getByLabelText('Username')).toBeInTheDocument();
			});

			const usernameInput = screen.getByLabelText('Username');
			const passwordInput = screen.getByLabelText('Password');
			const submitButton = screen.getByRole('button', { name: /sign in/i });

			// Trigger error
			await fireEvent.input(usernameInput, { target: { value: 'testuser' } });
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			await fireEvent.click(submitButton);

			await waitFor(() => {
				expect(screen.getByText('First error')).toBeInTheDocument();
			});

			// Now mock success and try again
			(auth.login as any).mockResolvedValue({});
			await fireEvent.click(submitButton);

			// Error should be cleared
			await waitFor(() => {
				expect(screen.queryByText('First error')).not.toBeInTheDocument();
			});
		});

		it('should integrate error styling with theme', async () => {
			const error = new Error('Authentication failed');
			(auth.login as any).mockRejectedValue(error);

			render(LoginPage);

			await waitFor(() => {
				expect(screen.getByLabelText('Username')).toBeInTheDocument();
			});

			const usernameInput = screen.getByLabelText('Username');
			const passwordInput = screen.getByLabelText('Password');
			const submitButton = screen.getByRole('button', { name: /sign in/i });

			// Trigger error
			await fireEvent.input(usernameInput, { target: { value: 'testuser' } });
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			await fireEvent.click(submitButton);

			// Should display error with proper styling
			await waitFor(() => {
				const errorMessage = screen.getByText('Authentication failed');
				expect(errorMessage).toBeInTheDocument();
				
				// Should have proper error styling container
				const errorContainer = errorMessage.closest('.bg-red-50');
				expect(errorContainer).toBeInTheDocument();
			});
		});
	});

	describe('State Management Integration', () => {
		it('should integrate auth store subscription', async () => {
			render(LoginPage);

			await waitFor(() => {
				// Should subscribe to auth store
				expect(authStore.subscribe).toHaveBeenCalled();
			});
		});

		it('should handle auth store state changes', async () => {
			// Mock store that changes state
			let callback: (state: any) => void;
			vi.mocked(authStore.subscribe).mockImplementation((cb: (state: any) => void) => {
				callback = cb;
				cb(createMockAuthState({ isAuthenticated: false }));
				return () => {};
			});

			render(LoginPage);

			await waitFor(() => {
				expect(authStore.subscribe).toHaveBeenCalled();
			});

			// Simulate auth state change
			callback!(createMockAuthState({ isAuthenticated: true, user: 'testuser' }));

			// Should trigger redirect
			await waitFor(() => {
				expect(goto).toHaveBeenCalledWith('/');
			});
		});

		it('should maintain component state during interactions', async () => {
			render(LoginPage);

			await waitFor(() => {
				expect(screen.getByLabelText('Username')).toBeInTheDocument();
			});

			const usernameInput = screen.getByLabelText('Username') as HTMLInputElement;
			const passwordInput = screen.getByLabelText('Password') as HTMLInputElement;

			// Enter values
			await fireEvent.input(usernameInput, { target: { value: 'testuser' } });
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });

			// Values should be maintained
			expect(usernameInput.value).toBe('testuser');
			expect(passwordInput.value).toBe('password123');
		});

		it('should handle loading state transitions', async () => {
			// Mock login that resolves after delay
			let resolveLogin: () => void;
			const loginPromise = new Promise<void>((resolve) => {
				resolveLogin = resolve;
			});
			(auth.login as any).mockReturnValue(loginPromise);

			render(LoginPage);

			await waitFor(() => {
				expect(screen.getByLabelText('Username')).toBeInTheDocument();
			});

			const usernameInput = screen.getByLabelText('Username');
			const passwordInput = screen.getByLabelText('Password');
			const submitButton = screen.getByRole('button', { name: /sign in/i });

			// Initial state - not loading
			expect(screen.getByText('Sign in')).toBeInTheDocument();
			expect(usernameInput).not.toBeDisabled();
			expect(passwordInput).not.toBeDisabled();

			// Enter credentials and submit
			await fireEvent.input(usernameInput, { target: { value: 'testuser' } });
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			await fireEvent.click(submitButton);

			// Should transition to loading state
			await waitFor(() => {
				expect(screen.getByText('Signing in...')).toBeInTheDocument();
				expect(usernameInput).toBeDisabled();
				expect(passwordInput).toBeDisabled();
			});

			// Complete login
			resolveLogin!();
			await loginPromise;

			// Should redirect
			await waitFor(() => {
				expect(goto).toHaveBeenCalledWith('/');
			});
		});
	});

	describe('Navigation Integration', () => {
		it('should integrate path resolution', async () => {
			render(LoginPage);

			await waitFor(() => {
				// Should resolve asset paths
				expect(resolve).toHaveBeenCalledWith('/assets/garm-light.svg');
				expect(resolve).toHaveBeenCalledWith('/assets/garm-dark.svg');
			});
		});

		it('should handle navigation on successful login', async () => {
			render(LoginPage);

			await waitFor(() => {
				expect(screen.getByLabelText('Username')).toBeInTheDocument();
			});

			const usernameInput = screen.getByLabelText('Username');
			const passwordInput = screen.getByLabelText('Password');
			const submitButton = screen.getByRole('button', { name: /sign in/i });

			// Successful login flow
			await fireEvent.input(usernameInput, { target: { value: 'testuser' } });
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			await fireEvent.click(submitButton);

			// Should navigate to home with resolved path
			await waitFor(() => {
				expect(goto).toHaveBeenCalledWith('/');
			});
		});

		it('should integrate automatic redirect for authenticated users', async () => {
			// Mock authenticated user from start
			vi.mocked(authStore.subscribe).mockImplementation((callback: (state: any) => void) => {
				callback(createMockAuthState({ isAuthenticated: true, user: 'existinguser' }));
				return () => {};
			});

			render(LoginPage);

			// Should immediately redirect
			await waitFor(() => {
				expect(goto).toHaveBeenCalledWith('/');
			});
		});
	});

	describe('Accessibility Integration', () => {
		it('should integrate keyboard navigation flow', async () => {
			render(LoginPage);

			await waitFor(() => {
				expect(screen.getByLabelText('Username')).toBeInTheDocument();
			});

			const usernameInput = screen.getByLabelText('Username');
			const passwordInput = screen.getByLabelText('Password');

			// Should support tab navigation
			usernameInput.focus();
			expect(document.activeElement).toBe(usernameInput);

			// Should support keyboard form submission
			await fireEvent.input(usernameInput, { target: { value: 'testuser' } });
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			await fireEvent.keyPress(passwordInput, { key: 'Enter', code: 'Enter' });

			// Should submit form
			expect(auth.login).toHaveBeenCalledWith('testuser', 'password123');
		});

		it('should maintain accessibility during loading states', async () => {
			// Mock delayed login
			let resolveLogin: () => void;
			const loginPromise = new Promise<void>((resolve) => {
				resolveLogin = resolve;
			});
			(auth.login as any).mockReturnValue(loginPromise);

			render(LoginPage);

			await waitFor(() => {
				expect(screen.getByLabelText('Username')).toBeInTheDocument();
			});

			const usernameInput = screen.getByLabelText('Username');
			const passwordInput = screen.getByLabelText('Password');
			const submitButton = screen.getByRole('button', { name: /sign in/i });

			// Submit form
			await fireEvent.input(usernameInput, { target: { value: 'testuser' } });
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			await fireEvent.click(submitButton);

			// Should maintain proper labels during loading
			await waitFor(() => {
				expect(screen.getByLabelText('Username')).toBeInTheDocument();
				expect(screen.getByLabelText('Password')).toBeInTheDocument();
				expect(screen.getByRole('button', { name: /signing in/i })).toBeInTheDocument();
			});

			// Complete login
			resolveLogin!();
			await loginPromise;
		});
	});

	describe('Component Lifecycle Integration', () => {
		it('should handle complete component lifecycle', () => {
			const { unmount } = render(LoginPage);

			// Should mount without errors
			expect(screen.getByRole('heading', { name: 'Sign in to GARM' })).toBeInTheDocument();

			// Should unmount without errors
			expect(() => unmount()).not.toThrow();
		});

		it('should integrate properly with Svelte lifecycle', async () => {
			render(LoginPage);

			// Should complete mount phase
			await waitFor(() => {
				expect(screen.getByRole('heading', { name: 'Sign in to GARM' })).toBeInTheDocument();
				expect(mockLocalStorage.getItem).toHaveBeenCalledWith('theme');
			});
		});

		it('should handle reactive updates', async () => {
			// Mock store with reactive updates
			let callback: (state: any) => void;
			vi.mocked(authStore.subscribe).mockImplementation((cb: (state: any) => void) => {
				callback = cb;
				cb(createMockAuthState({ isAuthenticated: false }));
				return () => {};
			});

			render(LoginPage);

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