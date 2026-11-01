import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

// SvelteKit 3 no longer reads svelte.config.js, and rejects that filename
// outright. The configuration lives here instead and is passed to the
// sveltekit() plugin by both the app and the test config, so the two cannot
// drift apart.

// Use different base paths for development vs production
const isDev = process.env.NODE_ENV === 'development';
const basePath: '' | `/${string}` = isDev ? '' : '/ui';

export const svelteKitConfig = {
	preprocess: vitePreprocess(),
	adapter: adapter({
		pages: 'build',
		assets: 'build',
		fallback: 'index.html',
		precompress: false,
		strict: true
	}),
	paths: {
		base: basePath
	},
	// SvelteKit 3 renamed $lib to #lib. Keep the old specifier rather than
	// rewriting it across the app as part of a dependency bump.
	alias: {
		$lib: 'src/lib'
	}
};
