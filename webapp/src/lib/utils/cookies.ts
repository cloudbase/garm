import { browser } from '$app/environment';

/**
 * Read a cookie value by name. Returns null when not found or outside the browser.
 */
export function getCookie(name: string): string | null {
	if (!browser) return null;

	const nameEQ = name + '=';
	const parts = document.cookie.split(';');
	for (let part of parts) {
		part = part.trim();
		if (part.startsWith(nameEQ)) {
			return part.substring(nameEQ.length);
		}
	}
	return null;
}

/**
 * Set a cookie that expires after the given number of days.
 */
export function setCookie(name: string, value: string, days: number = 7): void {
	if (!browser) return;

	const expires = new Date();
	expires.setTime(expires.getTime() + days * 24 * 60 * 60 * 1000);
	document.cookie = `${name}=${value};expires=${expires.toUTCString()};path=/;SameSite=Lax`;
}

/**
 * Delete a cookie by name.
 */
export function deleteCookie(name: string): void {
	if (!browser) return;
	document.cookie = `${name}=;expires=Thu, 01 Jan 1970 00:00:01 GMT;path=/`;
}
