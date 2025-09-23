import { browser } from '$app/environment';

export interface JWTClaims {
	is_admin?: boolean;
	username?: string;
	exp?: number;
	iat?: number;
	// Add other claims as needed
}

/**
 * Decode a JWT token and return its claims
 * @param token - The JWT token string
 * @returns The decoded claims object
 */
export function decodeJWT(token: string): JWTClaims | null {
	try {
		// JWT tokens have 3 parts separated by dots: header.payload.signature
		const parts = token.split('.');
		if (parts.length !== 3) {
			console.error('Invalid JWT token format');
			return null;
		}

		// The payload is the second part (index 1)
		const payload = parts[1];
		
		// Add padding if necessary (base64 requires length to be multiple of 4)
		const paddedPayload = payload + '='.repeat((4 - payload.length % 4) % 4);
		
		// Decode the base64-encoded payload
		const decodedPayload = atob(paddedPayload);
		
		// Parse the JSON claims
		const claims = JSON.parse(decodedPayload) as JWTClaims;
		
		return claims;
	} catch (error) {
		console.error('Failed to decode JWT token:', error);
		return null;
	}
}

/**
 * Get the current user's JWT claims from the stored token
 * @returns The current user's JWT claims or null if not available
 */
export function getCurrentUserClaims(): JWTClaims | null {
	if (!browser) return null;
	
	// Get the token from cookies
	const getCookie = (name: string): string | null => {
		const nameEQ = name + "=";
		const ca = document.cookie.split(';');
		for (let i = 0; i < ca.length; i++) {
			let c = ca[i];
			while (c.charAt(0) === ' ') c = c.substring(1, c.length);
			if (c.indexOf(nameEQ) === 0) {
				return c.substring(nameEQ.length, c.length);
			}
		}
		return null;
	};
	
	const token = getCookie('garm_token');
	if (!token) {
		return null;
	}
	
	return decodeJWT(token);
}

/**
 * Check if the current user is an admin
 * @returns true if the current user is an admin, false otherwise
 */
export function isCurrentUserAdmin(): boolean {
	const claims = getCurrentUserClaims();
	return claims?.is_admin === true;
}