/**
 * Generate a cryptographically secure webhook secret.
 * Returns a 64-character hex string (32 random bytes).
 */
export function generateSecureWebhookSecret(): string {
	const array = new Uint8Array(32);
	crypto.getRandomValues(array);
	return Array.from(array, (byte) => byte.toString(16).padStart(2, '0')).join('');
}
