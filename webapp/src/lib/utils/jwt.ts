import { getCookie } from './cookies';

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

		// The payload is the second part (index 1). It is base64url-encoded,
		// so convert to standard base64 before decoding with atob.
		const payload = parts[1].replace(/-/g, '+').replace(/_/g, '/');

		// Add padding if necessary (base64 requires length to be multiple of 4)
		const paddedPayload = payload + '='.repeat((4 - (payload.length % 4)) % 4);

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
 * Check whether a JWT token is expired (or undecodable).
 * Tokens without an exp claim are treated as non-expiring.
 */
export function isTokenExpired(token: string): boolean {
	const claims = decodeJWT(token);
	if (!claims) return true;
	if (typeof claims.exp !== 'number') return false;
	return claims.exp <= Math.floor(Date.now() / 1000);
}

/**
 * Get the current user's JWT claims from the stored token
 * @returns The current user's JWT claims or null if not available
 */
export function getCurrentUserClaims(): JWTClaims | null {
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
