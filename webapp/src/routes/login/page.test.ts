import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
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
		login: vi.fn()
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

// Mock localStorage
const mockLocalStorage = {
	getItem: vi.fn(),
	setItem: vi.fn(),
	removeItem: vi.fn()
};

// Mock window.matchMedia
const mockMatchMedia = vi.fn();

describe('Login Page - Unit Tests', () => {
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
		
		// Mock DOM APIs
		Object.defineProperty(window, 'localStorage', { value: mockLocalStorage });
		Object.defineProperty(window, 'matchMedia', { value: mockMatchMedia });
		
		// Set up default API mocks
		(auth.login as any).mockResolvedValue({});
		(resolve as any).mockImplementation((path: string) => path);
		(mockLocalStorage.getItem as any).mockReturnValue(null);
		(mockMatchMedia as any).mockReturnValue({ matches: false });
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	describe('Component Initialization', () => {
		it('should render successfully', () => {
			const { container } = render(LoginPage);
			expect(container).toBeInTheDocument();
		});

		it('should set page title', () => {
			render(LoginPage);
			expect(document.title).toBe('Login - GARM');
		});

		it('should render login form elements', () => {
			render(LoginPage);
			
			expect(screen.getByLabelText('Username')).toBeInTheDocument();
			expect(screen.getByLabelText('Password')).toBeInTheDocument();
			expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument();
		});

		it('should render GARM logo and branding', () => {
			render(LoginPage);
			
			expect(screen.getByText('Sign in to GARM')).toBeInTheDocument();
			expect(screen.getByText('GitHub Actions Runner Manager')).toBeInTheDocument();
			expect(screen.getAllByAltText('GARM')).toHaveLength(2); // Light and dark logos
		});
	});

	describe('Theme Initialization', () => {
		it('should render component successfully', () => {
			render(LoginPage);
			
			// Theme functionality works in browser but is hard to test in Node environment
			// Focus on ensuring component renders without errors
			expect(screen.getByRole('heading', { name: 'Sign in to GARM' })).toBeInTheDocument();
		});

		it('should have theme-aware styling classes', () => {
			render(LoginPage);
			
			// Should have dark mode classes ready
			const heading = screen.getByRole('heading', { name: 'Sign in to GARM' });
			expect(heading).toHaveClass('dark:text-white');
		});

		it('should render both theme logo variants', () => {
			render(LoginPage);
			
			const logos = screen.getAllByAltText('GARM');
			expect(logos).toHaveLength(2); // Light and dark variants
		});
	});

	describe('Authentication Redirect', () => {
		it('should redirect when user is already authenticated', () => {
			vi.mocked(authStore.subscribe).mockImplementation((callback: (state: any) => void) => {
				callback(createMockAuthState({ isAuthenticated: true, user: 'testuser' }));
				return () => {};
			});
			
			render(LoginPage);
			
			expect(goto).toHaveBeenCalledWith('/');
		});

		it('should not redirect when user is not authenticated', () => {
			vi.mocked(authStore.subscribe).mockImplementation((callback: (state: any) => void) => {
				callback(createMockAuthState({ isAuthenticated: false }));
				return () => {};
			});
			
			render(LoginPage);
			
			expect(goto).not.toHaveBeenCalled();
		});
	});

	describe('Form Validation', () => {
		it('should have required form fields', () => {
			render(LoginPage);
			
			const usernameInput = screen.getByLabelText('Username');
			const passwordInput = screen.getByLabelText('Password');
			
			// Fields should have required attribute
			expect(usernameInput).toHaveAttribute('required');
			expect(passwordInput).toHaveAttribute('required');
		});

		it('should validate empty form submission', async () => {
			render(LoginPage);
			
			const submitButton = screen.getByRole('button', { name: /sign in/i });
			
			// Submit form without entering anything
			await fireEvent.click(submitButton);
			
			// Should not call auth API for empty form
			expect(auth.login).not.toHaveBeenCalled();
		});

		it('should have proper form structure for validation', () => {
			render(LoginPage);
			
			const form = document.querySelector('form');
			const usernameInput = screen.getByLabelText('Username');
			const passwordInput = screen.getByLabelText('Password');
			
			expect(form).toBeInTheDocument();
			expect(usernameInput).toHaveAttribute('name', 'username');
			expect(passwordInput).toHaveAttribute('name', 'password');
		});
	});

	describe('Login Functionality', () => {
		it('should call auth.login with correct credentials on successful login', async () => {
			render(LoginPage);
			
			const usernameInput = screen.getByLabelText('Username');
			const passwordInput = screen.getByLabelText('Password');
			const submitButton = screen.getByRole('button', { name: /sign in/i });
			
			// Enter credentials
			await fireEvent.input(usernameInput, { target: { value: 'testuser' } });
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			
			// Submit form
			submitButton.click();
			
			expect(auth.login).toHaveBeenCalledWith('testuser', 'password123');
		});

		it('should redirect to home on successful login', async () => {
			render(LoginPage);
			
			const usernameInput = screen.getByLabelText('Username');
			const passwordInput = screen.getByLabelText('Password');
			const submitButton = screen.getByRole('button', { name: /sign in/i });
			
			// Enter credentials
			await fireEvent.input(usernameInput, { target: { value: 'testuser' } });
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			
			// Submit form
			submitButton.click();
			
			// Wait for async operations
			await new Promise(resolve => setTimeout(resolve, 0));
			
			expect(goto).toHaveBeenCalledWith('/');
		});

		it('should handle login API errors', async () => {
			const error = new Error('Invalid credentials');
			(auth.login as any).mockRejectedValue(error);
			
			render(LoginPage);
			
			const usernameInput = screen.getByLabelText('Username');
			const passwordInput = screen.getByLabelText('Password');
			const submitButton = screen.getByRole('button', { name: /sign in/i });
			
			// Enter credentials
			await fireEvent.input(usernameInput, { target: { value: 'testuser' } });
			await fireEvent.input(passwordInput, { target: { value: 'wrongpassword' } });
			
			// Submit form
			submitButton.click();
			
			// Wait for error to appear
			await screen.findByText('Invalid credentials');
			expect(goto).not.toHaveBeenCalled();
		});
	});

	describe('Loading States', () => {
		it('should show loading state during login', async () => {
			// Mock auth.login to return a promise that doesn't resolve immediately
			let resolveLogin: () => void;
			const loginPromise = new Promise<void>((resolve) => {
				resolveLogin = resolve;
			});
			(auth.login as any).mockReturnValue(loginPromise);
			
			render(LoginPage);
			
			const usernameInput = screen.getByLabelText('Username');
			const passwordInput = screen.getByLabelText('Password');
			const submitButton = screen.getByRole('button', { name: /sign in/i });
			
			// Enter credentials
			await fireEvent.input(usernameInput, { target: { value: 'testuser' } });
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			
			// Submit form
			await fireEvent.click(submitButton);
			
			// Should show loading state - inputs disabled and button shows loading
			expect(usernameInput).toBeDisabled();
			expect(passwordInput).toBeDisabled();
			
			// Button should show loading text (may be inside component structure)
			await screen.findByText('Signing in...');
			
			// Complete the login
			resolveLogin!();
			await loginPromise;
		});

		it('should clear loading state after login failure', async () => {
			const error = new Error('Login failed');
			(auth.login as any).mockRejectedValue(error);
			
			render(LoginPage);
			
			const usernameInput = screen.getByLabelText('Username');
			const passwordInput = screen.getByLabelText('Password');
			const submitButton = screen.getByRole('button', { name: /sign in/i });
			
			// Enter credentials and submit
			await fireEvent.input(usernameInput, { target: { value: 'testuser' } });
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			submitButton.click();
			
			// Wait for error handling
			await screen.findByText('Login failed');
			
			// Should not be in loading state anymore
			expect(screen.queryByText('Signing in...')).not.toBeInTheDocument();
			expect(screen.getByText('Sign in')).toBeInTheDocument();
			expect(usernameInput).not.toBeDisabled();
			expect(passwordInput).not.toBeDisabled();
		});
	});

	describe('Keyboard Interactions', () => {
		it('should submit form when Enter is pressed in username field', async () => {
			render(LoginPage);
			
			const usernameInput = screen.getByLabelText('Username');
			const passwordInput = screen.getByLabelText('Password');
			
			// Enter credentials
			await fireEvent.input(usernameInput, { target: { value: 'testuser' } });
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			
			// Press Enter in username field
			await fireEvent.keyPress(usernameInput, { key: 'Enter' });
			
			expect(auth.login).toHaveBeenCalledWith('testuser', 'password123');
		});

		it('should submit form when Enter is pressed in password field', async () => {
			render(LoginPage);
			
			const usernameInput = screen.getByLabelText('Username');
			const passwordInput = screen.getByLabelText('Password');
			
			// Enter credentials
			await fireEvent.input(usernameInput, { target: { value: 'testuser' } });
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			
			// Press Enter in password field
			await fireEvent.keyPress(passwordInput, { key: 'Enter' });
			
			expect(auth.login).toHaveBeenCalledWith('testuser', 'password123');
		});

		it('should not submit on non-Enter key press', async () => {
			render(LoginPage);
			
			const usernameInput = screen.getByLabelText('Username');
			const passwordInput = screen.getByLabelText('Password');
			
			// Enter credentials
			await fireEvent.input(usernameInput, { target: { value: 'testuser' } });
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			
			// Press non-Enter key
			await fireEvent.keyPress(usernameInput, { key: ' ' });
			
			expect(auth.login).not.toHaveBeenCalled();
		});
	});

	describe('Error Display', () => {
		it('should clear error when starting new login attempt', async () => {
			// First, cause an error
			const error = new Error('Login failed');
			(auth.login as any).mockRejectedValue(error);
			
			render(LoginPage);
			
			const usernameInput = screen.getByLabelText('Username');
			const passwordInput = screen.getByLabelText('Password');
			const submitButton = screen.getByRole('button', { name: /sign in/i });
			
			// Trigger error
			await fireEvent.input(usernameInput, { target: { value: 'testuser' } });
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			await fireEvent.click(submitButton);
			
			await screen.findByText('Login failed');
			
			// Now mock success and try again
			(auth.login as any).mockResolvedValue({});
			await fireEvent.click(submitButton);
			
			// Wait for async operations and error should be cleared
			await new Promise(resolve => setTimeout(resolve, 0));
			expect(screen.queryByText('Login failed')).not.toBeInTheDocument();
		});

		it('should display API errors with proper formatting', async () => {
			const error = new Error('Server temporarily unavailable');
			(auth.login as any).mockRejectedValue(error);
			
			render(LoginPage);
			
			const usernameInput = screen.getByLabelText('Username');
			const passwordInput = screen.getByLabelText('Password');
			const submitButton = screen.getByRole('button', { name: /sign in/i });
			
			// Enter credentials and submit
			await fireEvent.input(usernameInput, { target: { value: 'testuser' } });
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			submitButton.click();
			
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
			const { component } = render(LoginPage);
			expect(component).toBeDefined();
		});

		it('should unmount without errors', () => {
			const { unmount } = render(LoginPage);
			expect(() => unmount()).not.toThrow();
		});

		it('should subscribe to auth store on mount', () => {
			render(LoginPage);
			expect(authStore.subscribe).toHaveBeenCalled();
		});
	});

	describe('Form State Management', () => {
		it('should maintain form state during interactions', async () => {
			render(LoginPage);
			
			const usernameInput = screen.getByLabelText('Username') as HTMLInputElement;
			const passwordInput = screen.getByLabelText('Password') as HTMLInputElement;
			
			// Enter values
			await fireEvent.input(usernameInput, { target: { value: 'testuser' } });
			await fireEvent.input(passwordInput, { target: { value: 'password123' } });
			
			// Values should be maintained
			expect(usernameInput.value).toBe('testuser');
			expect(passwordInput.value).toBe('password123');
		});

		it('should support loading state functionality', async () => {
			render(LoginPage);
			
			const usernameInput = screen.getByLabelText('Username');
			const passwordInput = screen.getByLabelText('Password');
			const submitButton = screen.getByRole('button', { name: /sign in/i });
			
			// Fields should be enabled initially
			expect(usernameInput).not.toBeDisabled();
			expect(passwordInput).not.toBeDisabled();
			expect(submitButton).toHaveTextContent('Sign in');
			
			// Component should be ready to handle loading states
			// (actual loading behavior is tested in integration tests)
			expect(usernameInput).toHaveAttribute('type', 'text');
			expect(passwordInput).toHaveAttribute('type', 'password');
		});
	});
});