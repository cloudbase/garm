import { writable } from 'svelte/store';

function createThemeStore() {
	const { subscribe, set, update } = writable(false);

	return {
		subscribe,
		init: () => {
			// Initialize theme from localStorage or system preference
			const savedTheme = localStorage.getItem('theme');
			let darkMode = false;
			
			if (savedTheme === 'dark') {
				darkMode = true;
			} else if (savedTheme === 'light') {
				darkMode = false;
			} else {
				// No saved preference or 'system' - use system preference
				darkMode = window.matchMedia('(prefers-color-scheme: dark)').matches;
			}
			
			set(darkMode);
			updateDocumentClass(darkMode);
		},
		toggle: () => update(dark => {
			const newDarkMode = !dark;
			// Save explicit preference
			localStorage.setItem('theme', newDarkMode ? 'dark' : 'light');
			updateDocumentClass(newDarkMode);
			return newDarkMode;
		}),
		set: (/** @type {boolean} */ isDark) => {
			localStorage.setItem('theme', isDark ? 'dark' : 'light');
			updateDocumentClass(isDark);
			set(isDark);
		}
	};
}

function updateDocumentClass(/** @type {boolean} */ isDark) {
	if (isDark) {
		document.documentElement.classList.add('dark');
	} else {
		document.documentElement.classList.remove('dark');
	}
}

export const themeStore = createThemeStore();